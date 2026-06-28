package service

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/util"

	"go.uber.org/zap"
)

type liveStreamManager struct {
	cfg      *config.Config
	logger   *zap.Logger
	mu       sync.Mutex
	sessions map[string]*liveStreamSession
}

type liveStreamSession struct {
	key           string
	sourceType    string
	sourceID      uint
	streamProfile string
	sourceRTSP    string
	redactedRTSP  string
	playURL       string
	outputDir     string
	playlistPath  string
	cmd           *exec.Cmd
	stderr        *bytes.Buffer
	startedAt     time.Time
	expiresAt     time.Time
	stopped       bool
}

type liveStreamSource struct {
	sourceType  string
	sourceID    uint
	cameraID    any
	channelID   any
	deviceName  string
	rtspURL     string
	redactedURL string
}

func newLiveStreamManager(cfg *config.Config, logger *zap.Logger) *liveStreamManager {
	return &liveStreamManager{
		cfg:      cfg,
		logger:   logger,
		sessions: make(map[string]*liveStreamSession),
	}
}

func (m *liveStreamManager) startHLS(source liveStreamSource, streamProfile string) (*liveStreamSession, error) {
	key := liveStreamKey(source.sourceType, source.sourceID, streamProfile)
	now := time.Now()
	ttl := time.Duration(defaultPositiveInt(m.cfg.LiveHLSSessionTTL, 300)) * time.Second

	m.mu.Lock()
	if session := m.sessions[key]; session != nil && session.isProcessActive() && fileExists(session.playlistPath) {
		session.expiresAt = now.Add(ttl)
		m.mu.Unlock()
		return session, nil
	}
	m.reapExpiredLocked(now)
	m.stopLocked(key, false)
	if maxSessions := defaultPositiveInt(m.cfg.LiveHLSMaxSessions, 16); len(m.sessions) >= maxSessions {
		m.stopOldestLocked()
	}
	m.mu.Unlock()

	outputDir := filepath.Join(m.cfg.MediaRootDir, "live", source.sourceType, strconv.FormatUint(uint64(source.sourceID), 10), sanitizePathSegment(streamProfile))
	if err := os.RemoveAll(outputDir); err != nil {
		return nil, fmt.Errorf("cleanup hls output: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create hls output: %w", err)
	}

	playlistPath := filepath.Join(outputDir, "index.m3u8")
	playURL := m.publicMediaURL(filepath.ToSlash(filepath.Join("live", source.sourceType, strconv.FormatUint(uint64(source.sourceID), 10), sanitizePathSegment(streamProfile), "index.m3u8")))
	stderr := &bytes.Buffer{}
	cmd := exec.Command(m.cfg.FFmpegPath, buildHLSFFmpegArgs(m.cfg, source.rtspURL, outputDir, playlistPath)...)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	session := &liveStreamSession{
		key:           key,
		sourceType:    source.sourceType,
		sourceID:      source.sourceID,
		streamProfile: streamProfile,
		sourceRTSP:    source.rtspURL,
		redactedRTSP:  source.redactedURL,
		playURL:       playURL,
		outputDir:     outputDir,
		playlistPath:  playlistPath,
		cmd:           cmd,
		stderr:        stderr,
		startedAt:     now,
		expiresAt:     now.Add(ttl),
	}

	m.mu.Lock()
	m.sessions[key] = session
	m.mu.Unlock()

	go m.waitForSession(session)
	go m.expireSession(session)
	startTimeout := time.Duration(defaultPositiveInt(m.cfg.LiveHLSStartTimeout, 30)) * time.Second
	if err := waitForPlaylist(playlistPath, startTimeout); err != nil {
		m.mu.Lock()
		if m.sessions[key] == session {
			m.stopLocked(key, true)
		} else {
			m.stopSession(session, true)
		}
		m.mu.Unlock()
		return nil, fmt.Errorf("hls playlist not ready: %w%s", err, formatFFmpegStderr(stderr.String()))
	}
	return session, nil
}

func (m *liveStreamManager) startPlaybackHLS(source liveStreamSource, streamProfile string, startTime, endTime time.Time) (*liveStreamSession, error) {
	key := playbackStreamKey(source.sourceID, streamProfile, startTime, endTime)
	now := time.Now()
	ttl := time.Duration(defaultPositiveInt(m.cfg.LiveHLSSessionTTL, 300)) * time.Second

	m.mu.Lock()
	if session := m.sessions[key]; session != nil && fileExists(session.playlistPath) {
		session.expiresAt = now.Add(ttl)
		m.mu.Unlock()
		return session, nil
	}
	m.reapExpiredLocked(now)
	m.stopLocked(key, false)
	if maxSessions := defaultPositiveInt(m.cfg.LiveHLSMaxSessions, 16); len(m.sessions) >= maxSessions {
		m.stopOldestLocked()
	}
	m.mu.Unlock()

	keyHash := playbackStreamHash(source.sourceID, streamProfile, startTime, endTime)
	outputDir := filepath.Join(m.cfg.MediaRootDir, "playback", "channel", strconv.FormatUint(uint64(source.sourceID), 10), keyHash)
	if err := os.RemoveAll(outputDir); err != nil {
		return nil, fmt.Errorf("cleanup playback hls output: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create playback hls output: %w", err)
	}

	playlistPath := filepath.Join(outputDir, "index.m3u8")
	playURL := m.publicMediaURL(filepath.ToSlash(filepath.Join("playback", "channel", strconv.FormatUint(uint64(source.sourceID), 10), keyHash, "index.m3u8")))
	stderr := &bytes.Buffer{}
	cmd := exec.Command(m.cfg.FFmpegPath, buildPlaybackHLSFFmpegArgs(m.cfg, source.rtspURL, outputDir, playlistPath)...)
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	session := &liveStreamSession{
		key:           key,
		sourceType:    "playback",
		sourceID:      source.sourceID,
		streamProfile: streamProfile,
		sourceRTSP:    source.rtspURL,
		redactedRTSP:  source.redactedURL,
		playURL:       playURL,
		outputDir:     outputDir,
		playlistPath:  playlistPath,
		cmd:           cmd,
		stderr:        stderr,
		startedAt:     now,
		expiresAt:     now.Add(ttl),
	}

	m.mu.Lock()
	m.sessions[key] = session
	m.mu.Unlock()

	go m.waitForSession(session)
	go m.expireSession(session)
	startTimeout := time.Duration(defaultPositiveInt(m.cfg.LiveHLSStartTimeout, 30)) * time.Second
	if err := waitForPlaylist(playlistPath, startTimeout); err != nil {
		m.mu.Lock()
		if m.sessions[key] == session {
			m.stopLocked(key, true)
		} else {
			m.stopSession(session, true)
		}
		m.mu.Unlock()
		return nil, fmt.Errorf("playback hls playlist not ready: %w%s", err, formatFFmpegStderr(stderr.String()))
	}
	return session, nil
}

func (m *liveStreamManager) stop(sourceType string, sourceID uint, streamProfile string) bool {
	key := liveStreamKey(sourceType, sourceID, defaultString(streamProfile, "main"))
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked(key, true)
}

func (m *liveStreamManager) stopPlayback(channelID uint) bool {
	prefix := fmt.Sprintf("playback:channel:%d:", channelID)
	m.mu.Lock()
	defer m.mu.Unlock()
	stopped := false
	for key := range m.sessions {
		if strings.HasPrefix(key, prefix) {
			if m.stopLocked(key, true) {
				stopped = true
			}
		}
	}
	return stopped
}

func (m *liveStreamManager) stopAll() {
	m.mu.Lock()
	keys := make([]string, 0, len(m.sessions))
	for key := range m.sessions {
		keys = append(keys, key)
	}
	for _, key := range keys {
		m.stopLocked(key, true)
	}
	m.mu.Unlock()
}

func (m *liveStreamManager) waitForSession(session *liveStreamSession) {
	err := session.cmd.Wait()
	m.mu.Lock()
	if m.sessions[session.key] == session && session.sourceType != "playback" {
		delete(m.sessions, session.key)
	}
	stopped := session.stopped
	m.mu.Unlock()
	if err != nil && !stopped {
		m.logger.Warn("live hls ffmpeg exited", zap.String("key", session.key), zap.Error(err), zap.String("stderr", trimLog(session.stderr.String(), 2048)))
	}
}

func (m *liveStreamManager) expireSession(session *liveStreamSession) {
	for {
		m.mu.Lock()
		if m.sessions[session.key] != session {
			m.mu.Unlock()
			return
		}
		delay := time.Until(session.expiresAt)
		if delay <= 0 {
			m.stopLocked(session.key, true)
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
		timer := time.NewTimer(delay)
		<-timer.C
	}
}

func (m *liveStreamManager) reapExpiredLocked(now time.Time) {
	for key, session := range m.sessions {
		if !session.expiresAt.After(now) {
			m.stopLocked(key, true)
		}
	}
}

func (m *liveStreamManager) stopOldestLocked() {
	var oldest *liveStreamSession
	for _, session := range m.sessions {
		if oldest == nil || session.startedAt.Before(oldest.startedAt) {
			oldest = session
		}
	}
	if oldest != nil {
		m.stopLocked(oldest.key, true)
	}
}

func (m *liveStreamManager) stopLocked(key string, cleanup bool) bool {
	session := m.sessions[key]
	if session == nil {
		return false
	}
	delete(m.sessions, key)
	m.stopSession(session, cleanup)
	return true
}

func (m *liveStreamManager) stopSession(session *liveStreamSession, cleanup bool) {
	session.stopped = true
	if session.cmd != nil && session.cmd.Process != nil && session.cmd.ProcessState == nil {
		_ = session.cmd.Process.Kill()
	}
	if cleanup {
		_ = os.RemoveAll(session.outputDir)
	}
}

func (m *liveStreamManager) publicMediaURL(relativePath string) string {
	base := strings.TrimRight(m.cfg.BackendPublicBaseURL, "/")
	mountPath := strings.TrimRight(m.cfg.MediaMountPath, "/")
	return base + mountPath + "/" + strings.TrimLeft(relativePath, "/")
}

func (s *liveStreamSession) isProcessActive() bool {
	return s.cmd != nil && s.cmd.Process != nil && s.cmd.ProcessState == nil
}

func (s *PlatformService) getLiveHLSVideo(sourceType string, id uint, streamProfile string) (map[string]any, error) {
	profile := defaultString(streamProfile, "main")
	source, err := s.resolveLiveStreamSource(sourceType, id, profile)
	if err != nil {
		return nil, err
	}
	session, err := s.liveStreams.startHLS(source, profile)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"cameraId":          source.cameraID,
		"channelId":         source.channelID,
		"streamType":        "hls",
		"connectionMode":    "standard",
		"playUrl":           session.playURL,
		"expiresIn":         defaultPositiveInt(s.cfg.LiveHLSSessionTTL, 300),
		"isMock":            false,
		"playableInBrowser": true,
		"diagnosticMessage": fmt.Sprintf("HLS 会话已启动：%s", source.deviceName),
		"sourceRtsp":        source.redactedURL,
	}, nil
}

func (s *PlatformService) resolveLiveStreamSource(sourceType string, id uint, streamProfile string) (liveStreamSource, error) {
	if sourceType == "camera" {
		var camera entity.CameraDevice
		if err := s.db().First(&camera, id).Error; err != nil {
			return liveStreamSource{}, err
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), camera.PasswordEncrypted)
		if err != nil {
			return liveStreamSource{}, fmt.Errorf("resolve camera password: %w", err)
		}
		rtspURL, redactedURL := buildHikRTSPURL(camera.IP, defaultPositiveInt(camera.RTSPPort, 554), camera.Username, password, 1, streamProfile)
		return liveStreamSource{
			sourceType:  "camera",
			sourceID:    camera.ID,
			cameraID:    camera.ID,
			channelID:   nil,
			deviceName:  camera.Name,
			rtspURL:     rtspURL,
			redactedURL: redactedURL,
		}, nil
	}

	var channel entity.RecorderChannel
	var recorder entity.RecorderDevice
	if err := s.db().First(&channel, id).Error; err != nil {
		return liveStreamSource{}, err
	}
	if err := s.db().First(&recorder, channel.RecorderID).Error; err != nil {
		return liveStreamSource{}, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return liveStreamSource{}, fmt.Errorf("resolve recorder password: %w", err)
	}
	rtspURL, redactedURL := buildHikRTSPURL(recorder.IP, 554, recorder.Username, password, channel.ChannelNo, streamProfile)
	return liveStreamSource{
		sourceType:  "channel",
		sourceID:    channel.ID,
		cameraID:    channel.CameraID,
		channelID:   channel.ID,
		deviceName:  recorder.Name + " / " + channel.Name,
		rtspURL:     rtspURL,
		redactedURL: redactedURL,
	}, nil
}

func (s *PlatformService) getPlaybackHLSVideo(channelID uint, streamProfile string, startTime, endTime time.Time) (map[string]any, error) {
	source, err := s.resolvePlaybackStreamSource(channelID, defaultString(streamProfile, "main"), startTime, endTime)
	if err != nil {
		return nil, err
	}
	session, err := s.liveStreams.startPlaybackHLS(source, defaultString(streamProfile, "main"), startTime, endTime)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"streamType":        "hls",
		"streamProfile":     defaultString(streamProfile, "main"),
		"playbackMode":      "hls",
		"playUrl":           session.playURL,
		"startTime":         startTime.Format(time.RFC3339),
		"endTime":           endTime.Format(time.RFC3339),
		"expiresIn":         defaultPositiveInt(s.cfg.LiveHLSSessionTTL, 300),
		"isMock":            false,
		"playableInBrowser": true,
		"diagnosticMessage": fmt.Sprintf("HLS 回放会话已启动：%s", source.deviceName),
		"sourceRtsp":        source.redactedURL,
	}, nil
}

func (s *PlatformService) resolvePlaybackStreamSource(channelID uint, streamProfile string, startTime, endTime time.Time) (liveStreamSource, error) {
	if !endTime.After(startTime) {
		return liveStreamSource{}, fmt.Errorf("invalid playback time range")
	}

	var channel entity.RecorderChannel
	var recorder entity.RecorderDevice
	if err := s.db().First(&channel, channelID).Error; err != nil {
		return liveStreamSource{}, err
	}
	if err := s.db().First(&recorder, channel.RecorderID).Error; err != nil {
		return liveStreamSource{}, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return liveStreamSource{}, fmt.Errorf("resolve recorder password: %w", err)
	}
	rtspURL, redactedURL := buildHikPlaybackRTSPURL(recorder.IP, 554, recorder.Username, password, channel.ChannelNo, streamProfile, startTime, endTime)
	return liveStreamSource{
		sourceType:  "playback",
		sourceID:    channel.ID,
		cameraID:    channel.CameraID,
		channelID:   channel.ID,
		deviceName:  recorder.Name + " / " + channel.Name,
		rtspURL:     rtspURL,
		redactedURL: redactedURL,
	}, nil
}

func buildHLSFFmpegArgs(cfg *config.Config, rtspURL, outputDir, playlistPath string) []string {
	segmentSeconds := defaultPositiveInt(cfg.LiveHLSSegmentSeconds, 2)
	listSize := defaultPositiveInt(cfg.LiveHLSListSize, 6)
	segmentPattern := filepath.Join(outputDir, "segment_%05d.ts")
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-fflags", "+genpts+discardcorrupt",
		"-err_detect", "ignore_err",
		"-analyzeduration", "10000000",
		"-probesize", "10000000",
		"-use_wallclock_as_timestamps", "1",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-an",
	}
	if cfg.LiveHLSTranscode {
		args = append(
			args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-tune", "zerolatency",
			"-profile:v", "baseline",
			"-level:v", "4.0",
			"-pix_fmt", "yuv420p",
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentSeconds),
			"-sc_threshold", "0",
		)
	} else {
		args = append(args, "-c:v", "copy")
	}
	args = append(args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentSeconds),
		"-hls_list_size", strconv.Itoa(listSize),
		"-hls_flags", "delete_segments+append_list+omit_endlist+independent_segments",
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	)
	return args
}

func buildPlaybackHLSFFmpegArgs(cfg *config.Config, rtspURL, outputDir, playlistPath string) []string {
	segmentSeconds := defaultPositiveInt(cfg.LiveHLSSegmentSeconds, 2)
	segmentPattern := filepath.Join(outputDir, "segment_%05d.ts")
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-fflags", "+genpts+discardcorrupt",
		"-err_detect", "ignore_err",
		"-analyzeduration", "10000000",
		"-probesize", "10000000",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-an",
	}
	if cfg.LiveHLSTranscode {
		args = append(
			args,
			"-c:v", "libx264",
			"-preset", "veryfast",
			"-tune", "zerolatency",
			"-profile:v", "baseline",
			"-level:v", "4.0",
			"-pix_fmt", "yuv420p",
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", segmentSeconds),
			"-sc_threshold", "0",
		)
	} else {
		args = append(args, "-c:v", "copy")
	}
	args = append(args,
		"-f", "hls",
		"-hls_time", strconv.Itoa(segmentSeconds),
		"-hls_list_size", "0",
		"-hls_playlist_type", "event",
		"-hls_flags", "append_list+independent_segments",
		"-hls_segment_filename", segmentPattern,
		playlistPath,
	)
	return args
}

func buildHikRTSPURL(host string, port int, username string, password string, channelNo int, streamProfile string) (string, string) {
	streamNo := 1
	if streamProfile == "sub" {
		streamNo = 2
	}
	channelCode := defaultPositiveInt(channelNo, 1)*100 + streamNo
	base := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(username, password),
		Host:   net.JoinHostPort(host, strconv.Itoa(defaultPositiveInt(port, 554))),
		Path:   fmt.Sprintf("/Streaming/Channels/%d", channelCode),
	}
	redacted := base
	if username != "" {
		redacted.User = url.UserPassword(username, "******")
	}
	return base.String(), redacted.String()
}

func buildHikPlaybackRTSPURL(host string, port int, username string, password string, channelNo int, streamProfile string, startTime, endTime time.Time) (string, string) {
	streamNo := 1
	if streamProfile == "sub" {
		streamNo = 2
	}
	channelCode := defaultPositiveInt(channelNo, 1)*100 + streamNo
	base := url.URL{
		Scheme: "rtsp",
		User:   url.UserPassword(username, password),
		Host:   net.JoinHostPort(host, strconv.Itoa(defaultPositiveInt(port, 554))),
		Path:   fmt.Sprintf("/Streaming/tracks/%d", channelCode),
	}
	query := base.Query()
	query.Set("starttime", formatHikPlaybackRTSPTime(startTime))
	query.Set("endtime", formatHikPlaybackRTSPTime(endTime))
	base.RawQuery = query.Encode()
	redacted := base
	if username != "" {
		redacted.User = url.UserPassword(username, "******")
	}
	return base.String(), redacted.String()
}

func formatHikPlaybackRTSPTime(value time.Time) string {
	return value.Local().Format("20060102T150405Z")
}

func waitForPlaylist(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil && info.Size() > 0 {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("%s not generated within %s", path, timeout)
}

func liveStreamKey(sourceType string, sourceID uint, streamProfile string) string {
	return fmt.Sprintf("%s:%d:%s", sourceType, sourceID, sanitizePathSegment(defaultString(streamProfile, "main")))
}

func playbackStreamKey(channelID uint, streamProfile string, startTime, endTime time.Time) string {
	return fmt.Sprintf("playback:channel:%d:%s:%s", channelID, sanitizePathSegment(defaultString(streamProfile, "main")), playbackStreamHash(channelID, streamProfile, startTime, endTime))
}

func playbackStreamHash(channelID uint, streamProfile string, startTime, endTime time.Time) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%d|%s|%s|%s", channelID, defaultString(streamProfile, "main"), startTime.Format(time.RFC3339Nano), endTime.Format(time.RFC3339Nano))))
	return hex.EncodeToString(sum[:])[:16]
}

func sanitizePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "default"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "default"
	}
	return builder.String()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func defaultPositiveInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func formatFFmpegStderr(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return ""
	}
	return ": " + trimLog(stderr, 1024)
}

func trimLog(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}
