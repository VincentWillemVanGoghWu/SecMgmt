package service

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/integration/hikvision"
	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/util"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type hikvisionBridgeSession struct {
	SessionKey  string
	DeviceType  string
	DeviceID    uint
	DeviceName  string
	DeviceIP    string
	UserID      int32
	AlarmHandle int32
	DeviceInfo  hikvision.DeviceInfo
}

type hikvisionBridgeTarget struct {
	ProviderCode string
	SessionKey   string
	DeviceType   string
	DeviceID     uint
	DeviceName   string
	DeviceIP     string
	SDKPort      int
	HTTPPort     int
	Username     string
	Password     string
}

type hikvisionISAPIStreamSession struct {
	SessionKey  string
	DeviceType  string
	DeviceID    uint
	DeviceName  string
	DeviceIP    string
	HTTPPort    int
	Username    string
	Password    string
	Connected   bool
	LastError   string
	LastEventAt *time.Time
	LastByteAt  *time.Time
	cancel      context.CancelFunc
}

type idleReadCloser struct {
	source  io.ReadCloser
	timeout time.Duration
	onIdle  func()
	timer   *time.Timer
	mu      sync.Mutex
	idle    bool
	closed  bool
}

func newIdleReadCloser(source io.ReadCloser, timeout time.Duration, onIdle func()) *idleReadCloser {
	reader := &idleReadCloser{source: source, timeout: timeout, onIdle: onIdle}
	reader.timer = time.AfterFunc(timeout, func() {
		var callback func()
		reader.mu.Lock()
		if !reader.closed {
			reader.idle = true
			callback = reader.onIdle
			_ = reader.source.Close()
		}
		reader.mu.Unlock()
		if callback != nil {
			callback()
		}
	})
	return reader
}

func (r *idleReadCloser) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	if n > 0 {
		r.mu.Lock()
		if !r.closed && r.timer != nil {
			r.timer.Reset(r.timeout)
		}
		r.mu.Unlock()
	}
	if err != nil && r.isIdle() {
		return n, errHikvisionISAPIStreamIdle
	}
	return n, err
}

func (r *idleReadCloser) Close() error {
	r.mu.Lock()
	r.closed = true
	if r.timer != nil {
		r.timer.Stop()
	}
	r.mu.Unlock()
	return r.source.Close()
}

func (r *idleReadCloser) isIdle() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.idle
}

type smartBridgeReconnectTarget struct {
	Target     hikvisionBridgeTarget
	BindingIDs []uint
}

type SmartIngestFile struct {
	Filename    string
	ContentType string
	Data        []byte
}

type motionAggregateWindow struct {
	Key                string
	DedupKey           string
	SessionKey         string
	DeviceType         string
	DeviceID           uint
	DeviceIP           string
	RawChannelNo       int
	ChannelNo          int
	Command            int
	FirstEventTime     time.Time
	LastEventTime      time.Time
	Count              int
	ProviderID         uint
	CapabilityID       uint
	BindingID          uint
	SourceType         string
	SourceID           uint
	EventLevel         string
	CameraID           *uint
	RecorderID         *uint
	ChannelID          *uint
	FactoryID          *uint
	ZoneID             *uint
	Rule               *entity.SmartBindingRule
	SnapshotEntityID   uint
	SnapshotURL        string
	DedupWindowSeconds int
	CooldownSeconds    int
	Timer              *time.Timer
}

const smartEventMergeWindow = 2 * time.Second

var regexpXMLTag = regexp.MustCompile(`(?s)<([A-Za-z0-9_:.-]+)[^>]*>\s*([^<>]+?)\s*</[A-Za-z0-9_:.-]+>`)

var errHikvisionMotionBindingNotMatched = errors.New("hikvision motion binding not matched")
var errHikvisionISAPIStreamIdle = errors.New("hikvision isapi alert stream idle timeout")

const hikvisionISAPIStreamIdleTimeout = 45 * time.Second

type HikvisionAlarmBridgeService struct {
	cfg    *config.Config
	repo   *repository.Repository
	logger *zap.Logger

	mu                  sync.RWMutex
	alarmDedupMu        sync.Mutex
	motionMu            sync.Mutex
	sdk                 *hikvision.SDK
	sessions            map[string]*hikvisionBridgeSession
	sessionsByUserID    map[int32]*hikvisionBridgeSession
	isapiSessions       map[string]*hikvisionISAPIStreamSession
	alarmDedupLocks     map[string]*sync.Mutex
	motionWindows       map[string]*motionAggregateWindow
	motionCooldown      map[string]time.Time
	running             bool
	lastError           string
	bindingCount        int
	skippedBindingCount int
	mergedBindingCount  int
}

func NewHikvisionAlarmBridgeService(cfg *config.Config, repo *repository.Repository, logger *zap.Logger) *HikvisionAlarmBridgeService {
	return &HikvisionAlarmBridgeService{
		cfg:              cfg,
		repo:             repo,
		logger:           logger,
		sessions:         make(map[string]*hikvisionBridgeSession),
		sessionsByUserID: make(map[int32]*hikvisionBridgeSession),
		isapiSessions:    make(map[string]*hikvisionISAPIStreamSession),
		alarmDedupLocks:  make(map[string]*sync.Mutex),
		motionWindows:    make(map[string]*motionAggregateWindow),
		motionCooldown:   make(map[string]time.Time),
	}
}

func (s *HikvisionAlarmBridgeService) Start() error {
	if strings.EqualFold(strings.TrimSpace(s.cfg.AppEnv), "test") {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
	s.lastError = ""

	targets, err := s.collectTargets("hikvision-sdk", "hikvision-isapi")
	if err != nil {
		s.lastError = err.Error()
		return err
	}
	s.logger.Info("hikvision alarm bridge targets collected",
		zap.Int("bindingCount", s.bindingCount),
		zap.Int("targetCount", len(targets)),
		zap.Int("skippedBindingCount", s.skippedBindingCount),
		zap.Int("mergedBindingCount", s.mergedBindingCount),
	)

	var sdkTargets []hikvisionBridgeTarget
	var isapiTargets []hikvisionBridgeTarget
	for _, target := range targets {
		switch target.ProviderCode {
		case "hikvision-isapi":
			isapiTargets = append(isapiTargets, target)
		default:
			sdkTargets = append(sdkTargets, target)
		}
	}
	var startupErrors []string
	if len(sdkTargets) > 0 {
		sdk, err := hikvision.NewSDK(s.cfg.HikvisionSDKPath)
		if err != nil {
			startupErrors = append(startupErrors, fmt.Sprintf("hikvision-sdk: %v", err))
			s.logger.Warn("hikvision sdk bridge init failed", zap.Error(err))
		} else if err := sdk.SetAlarmHandler(s.handleAlarm); err != nil {
			startupErrors = append(startupErrors, fmt.Sprintf("hikvision-sdk: %v", err))
			s.logger.Warn("hikvision sdk bridge alarm handler setup failed", zap.Error(err))
			_ = sdk.Cleanup()
		} else {
			s.sdk = sdk
		}
	}

	if s.sdk != nil {
		for _, target := range sdkTargets {
			if err := s.openSessionLocked(target); err != nil {
				startupErrors = append(startupErrors, fmt.Sprintf("%s: %v", target.SessionKey, err))
				s.logger.Warn("hikvision sdk bridge open session failed",
					zap.String("sessionKey", target.SessionKey),
					zap.Error(err),
				)
			}
		}
	}
	for _, target := range isapiTargets {
		if err := s.openISAPIStreamLocked(target); err != nil {
			startupErrors = append(startupErrors, fmt.Sprintf("%s: %v", target.SessionKey, err))
			s.logger.Warn("hikvision isapi stream open failed",
				zap.String("sessionKey", target.SessionKey),
				zap.Error(err),
			)
		}
	}
	s.running = len(s.sessions) > 0 || len(s.isapiSessions) > 0
	if len(startupErrors) > 0 {
		s.lastError = strings.Join(startupErrors, "; ")
	}
	s.logger.Info("hikvision alarm bridge startup finished",
		zap.Bool("running", s.running),
		zap.Int("sessionCount", len(s.sessions)),
		zap.Int("isapiSessionCount", len(s.isapiSessions)),
		zap.String("lastError", s.lastError),
	)
	return nil
}

func (s *HikvisionAlarmBridgeService) ReconnectTarget(target hikvisionBridgeTarget) error {
	if strings.EqualFold(strings.TrimSpace(s.cfg.AppEnv), "test") {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if target.ProviderCode == "hikvision-isapi" {
		s.closeISAPIStreamLocked(target.SessionKey)
		if err := s.openISAPIStreamLocked(target); err != nil {
			s.lastError = err.Error()
			return err
		}
	} else {
		if err := s.ensureSDKLocked(); err != nil {
			s.lastError = err.Error()
			return err
		}
		s.closeSessionLocked(target.SessionKey)
		if err := s.openSessionLocked(target); err != nil {
			s.lastError = err.Error()
			return err
		}
	}
	s.running = len(s.sessions) > 0 || len(s.isapiSessions) > 0
	s.lastError = ""
	if s.logger != nil {
		s.logger.Info("hikvision alarm bridge target reconnected",
			zap.String("provider", target.ProviderCode),
			zap.String("sessionKey", target.SessionKey),
			zap.String("deviceType", target.DeviceType),
			zap.Uint("deviceID", target.DeviceID),
			zap.String("deviceIP", target.DeviceIP),
		)
	}
	return nil
}

func (s *HikvisionAlarmBridgeService) HasSession(sessionKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.sessions[sessionKey]; ok {
		return true
	}
	_, ok := s.isapiSessions[sessionKey]
	return ok
}

func (s *HikvisionAlarmBridgeService) CloseDeviceSessions(deviceType string, deviceID uint) int {
	if deviceID == 0 {
		return 0
	}
	deviceType = strings.TrimSpace(strings.ToLower(deviceType))
	if deviceType == "channel" {
		if s.logger != nil {
			s.logger.Info("skip closing hikvision bridge session for channel status change",
				zap.Uint("channelID", deviceID),
			)
		}
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	closed := 0
	for sessionKey, session := range s.sessions {
		if session.DeviceType == deviceType && session.DeviceID == deviceID {
			s.closeSessionLocked(sessionKey)
			closed++
		}
	}
	for sessionKey, session := range s.isapiSessions {
		if session.DeviceType == deviceType && session.DeviceID == deviceID {
			s.closeISAPIStreamLocked(sessionKey)
			closed++
		}
	}
	s.running = len(s.sessions) > 0 || len(s.isapiSessions) > 0
	if s.logger != nil && closed > 0 {
		s.logger.Info("hikvision sdk bridge device sessions closed",
			zap.String("deviceType", deviceType),
			zap.Uint("deviceID", deviceID),
			zap.Int("closed", closed),
		)
	}
	return closed
}

func (s *HikvisionAlarmBridgeService) ResolveReconnectTargetsForDevice(deviceType string, deviceID uint) ([]smartBridgeReconnectTarget, error) {
	deviceType = strings.TrimSpace(strings.ToLower(deviceType))
	if deviceID == 0 {
		return nil, nil
	}

	type bindingRow struct {
		entity.SmartDeviceBinding
		ProviderCode string `gorm:"column:provider_code"`
	}
	query := s.repo.DB().
		Table("smart_device_binding AS b").
		Select("b.*, p.provider_code").
		Joins("JOIN smart_interface_provider p ON p.id = b.provider_id").
		Joins("JOIN smart_interface_capability c ON c.id = b.capability_id").
		Where("b.enabled = ? AND p.enabled = ? AND c.enabled = ? AND p.provider_code IN ? AND c.capability_code = ?",
			true, true, true, []string{"hikvision-sdk", "hikvision-isapi"}, "motion_detect")

	switch deviceType {
	case "camera":
		query = query.Joins("LEFT JOIN recorder_channel ch ON b.source_type = ? AND ch.id = b.source_id", "channel").
			Where("(b.source_type = ? AND b.source_id = ?) OR (b.source_type = ? AND ch.camera_id = ?)",
				"camera", deviceID, "channel", deviceID)
	case "recorder":
		query = query.Joins("LEFT JOIN recorder_channel ch ON b.source_type = ? AND ch.id = b.source_id", "channel").
			Where("(b.source_type = ? AND b.source_id = ?) OR (b.source_type = ? AND ch.recorder_id = ?)",
				"recorder", deviceID, "channel", deviceID)
	case "channel":
		query = query.Where("b.source_type = ? AND b.source_id = ?", "channel", deviceID)
	default:
		return nil, nil
	}

	var rows []bindingRow
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	targetsBySession := make(map[string]*smartBridgeReconnectTarget)
	for _, row := range rows {
		target, ok, err := s.bindingToTargetForProvider(row.ProviderCode, row.SmartDeviceBinding)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		item := targetsBySession[target.SessionKey]
		if item == nil {
			item = &smartBridgeReconnectTarget{Target: target}
			targetsBySession[target.SessionKey] = item
		}
		item.BindingIDs = append(item.BindingIDs, row.ID)
	}

	result := make([]smartBridgeReconnectTarget, 0, len(targetsBySession))
	for _, item := range targetsBySession {
		sort.Slice(item.BindingIDs, func(i, j int) bool { return item.BindingIDs[i] < item.BindingIDs[j] })
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Target.SessionKey < result[j].Target.SessionKey })
	return result, nil
}

func (s *HikvisionAlarmBridgeService) ResolveReconnectTargetForBinding(bindingID uint) (smartBridgeReconnectTarget, bool, error) {
	if bindingID == 0 {
		return smartBridgeReconnectTarget{}, false, nil
	}
	type bindingRow struct {
		entity.SmartDeviceBinding
		ProviderCode string `gorm:"column:provider_code"`
	}
	var row bindingRow
	err := s.repo.DB().
		Table("smart_device_binding AS b").
		Select("b.*, p.provider_code").
		Joins("JOIN smart_interface_provider p ON p.id = b.provider_id").
		Joins("JOIN smart_interface_capability c ON c.id = b.capability_id").
		Where("b.id = ? AND b.enabled = ? AND p.enabled = ? AND c.enabled = ? AND p.provider_code IN ? AND c.capability_code = ?",
			bindingID, true, true, true, []string{"hikvision-sdk", "hikvision-isapi"}, "motion_detect").
		Scan(&row).Error
	if err != nil {
		return smartBridgeReconnectTarget{}, false, err
	}
	if row.ID == 0 {
		return smartBridgeReconnectTarget{}, false, nil
	}
	target, ok, err := s.bindingToTargetForProvider(row.ProviderCode, row.SmartDeviceBinding)
	if err != nil || !ok {
		return smartBridgeReconnectTarget{}, ok, err
	}
	return smartBridgeReconnectTarget{Target: target, BindingIDs: []uint{row.ID}}, true, nil
}

func (s *HikvisionAlarmBridgeService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopLocked()
}

func (s *HikvisionAlarmBridgeService) RuntimeStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionItems := make([]map[string]any, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessionItems = append(sessionItems, map[string]any{
			"provider":   "hikvision-sdk",
			"sessionKey": session.SessionKey,
			"deviceType": session.DeviceType,
			"deviceId":   session.DeviceID,
			"deviceName": session.DeviceName,
			"deviceIp":   session.DeviceIP,
		})
	}
	for _, session := range s.isapiSessions {
		lastEventAt := ""
		if session.LastEventAt != nil {
			lastEventAt = session.LastEventAt.Format(time.RFC3339)
		}
		lastByteAt := ""
		if session.LastByteAt != nil {
			lastByteAt = session.LastByteAt.Format(time.RFC3339)
		}
		sessionItems = append(sessionItems, map[string]any{
			"provider":    "hikvision-isapi",
			"sessionKey":  session.SessionKey,
			"deviceType":  session.DeviceType,
			"deviceId":    session.DeviceID,
			"deviceName":  session.DeviceName,
			"deviceIp":    session.DeviceIP,
			"httpPort":    session.HTTPPort,
			"connected":   session.Connected,
			"lastError":   session.LastError,
			"lastEventAt": lastEventAt,
			"lastByteAt":  lastByteAt,
		})
	}
	connectedISAPISessionCount := 0
	receivingISAPISessionCount := 0
	for _, session := range s.isapiSessions {
		if session.Connected {
			connectedISAPISessionCount++
		}
		if session.LastByteAt != nil {
			receivingISAPISessionCount++
		}
	}
	sort.Slice(sessionItems, func(i, j int) bool {
		return fmt.Sprint(sessionItems[i]["sessionKey"]) < fmt.Sprint(sessionItems[j]["sessionKey"])
	})
	return map[string]any{
		"running":                    s.running,
		"sessionCount":               len(s.sessions) + len(s.isapiSessions),
		"sdkSessionCount":            len(s.sessions),
		"isapiSessionCount":          len(s.isapiSessions),
		"isapiConnectedSessionCount": connectedISAPISessionCount,
		"isapiReceivingSessionCount": receivingISAPISessionCount,
		"bindingCount":               s.bindingCount,
		"skippedBindingCount":        s.skippedBindingCount,
		"mergedBindingCount":         s.mergedBindingCount,
		"lastError":                  s.lastError,
		"sessions":                   sessionItems,
	}
}

func (s *HikvisionAlarmBridgeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *HikvisionAlarmBridgeService) stopLocked() {
	s.flushAllMotionWindows()
	for sessionKey := range s.isapiSessions {
		s.closeISAPIStreamLocked(sessionKey)
	}
	for _, session := range s.sessions {
		if s.sdk != nil {
			if err := s.sdk.CloseAlarm(session.AlarmHandle); err != nil {
				s.logger.Warn("close hikvision alarm failed", zap.String("sessionKey", session.SessionKey), zap.Error(err))
			}
			if err := s.sdk.Logout(session.UserID); err != nil {
				s.logger.Warn("hikvision logout failed", zap.String("sessionKey", session.SessionKey), zap.Error(err))
			}
		}
	}
	if s.sdk != nil {
		_ = s.sdk.Cleanup()
	}
	s.sessions = make(map[string]*hikvisionBridgeSession)
	s.sessionsByUserID = make(map[int32]*hikvisionBridgeSession)
	s.isapiSessions = make(map[string]*hikvisionISAPIStreamSession)
	s.motionCooldown = make(map[string]time.Time)
	s.running = false
}

func (s *HikvisionAlarmBridgeService) ensureSDKLocked() error {
	if s.sdk != nil {
		return nil
	}
	sdk, err := hikvision.NewSDK(s.cfg.HikvisionSDKPath)
	if err != nil {
		return err
	}
	if err := sdk.SetAlarmHandler(s.handleAlarm); err != nil {
		_ = sdk.Cleanup()
		return err
	}
	s.sdk = sdk
	return nil
}

func (s *HikvisionAlarmBridgeService) closeSessionLocked(sessionKey string) {
	session := s.sessions[sessionKey]
	if session == nil {
		return
	}
	if s.sdk != nil {
		if err := s.sdk.CloseAlarm(session.AlarmHandle); err != nil && s.logger != nil {
			s.logger.Warn("close hikvision alarm failed", zap.String("sessionKey", session.SessionKey), zap.Error(err))
		}
		if err := s.sdk.Logout(session.UserID); err != nil && s.logger != nil {
			s.logger.Warn("hikvision logout failed", zap.String("sessionKey", session.SessionKey), zap.Error(err))
		}
	}
	delete(s.sessionsByUserID, session.UserID)
	delete(s.sessions, sessionKey)
}

func (s *HikvisionAlarmBridgeService) closeISAPIStreamLocked(sessionKey string) {
	session := s.isapiSessions[sessionKey]
	if session == nil {
		return
	}
	if session.cancel != nil {
		session.cancel()
	}
	delete(s.isapiSessions, sessionKey)
}

func (s *HikvisionAlarmBridgeService) markISAPIStreamConnected(sessionKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.isapiSessions[sessionKey]; session != nil {
		session.Connected = true
		session.LastError = ""
	}
}

func (s *HikvisionAlarmBridgeService) markISAPIStreamError(sessionKey string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.isapiSessions[sessionKey]; session != nil {
		session.Connected = false
		if err != nil {
			session.LastError = err.Error()
		}
	}
}

func (s *HikvisionAlarmBridgeService) markISAPIStreamEvent(sessionKey string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.isapiSessions[sessionKey]; session != nil {
		session.LastEventAt = &now
	}
}

func (s *HikvisionAlarmBridgeService) markISAPIStreamByte(sessionKey string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if session := s.isapiSessions[sessionKey]; session != nil {
		session.LastByteAt = &now
	}
}

func (s *HikvisionAlarmBridgeService) isapiLastByteAt(sessionKey string) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if session := s.isapiSessions[sessionKey]; session != nil && session.LastByteAt != nil {
		last := *session.LastByteAt
		return &last
	}
	return nil
}

func (s *HikvisionAlarmBridgeService) collectTargets(providerCodes ...string) ([]hikvisionBridgeTarget, error) {
	if len(providerCodes) == 0 {
		providerCodes = []string{"hikvision-sdk"}
	}
	type bindingRow struct {
		entity.SmartDeviceBinding
		ProviderCode   string `gorm:"column:provider_code"`
		CapabilityCode string `gorm:"column:capability_code"`
	}
	var rows []bindingRow
	if err := s.repo.DB().
		Table("smart_device_binding AS b").
		Select("b.*, p.provider_code, c.capability_code").
		Joins("JOIN smart_interface_provider p ON p.id = b.provider_id").
		Joins("JOIN smart_interface_capability c ON c.id = b.capability_id").
		Where("b.enabled = ? AND p.enabled = ? AND c.enabled = ? AND p.provider_code IN ? AND c.capability_code = ?", true, true, true, providerCodes, "motion_detect").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	targetMap := make(map[string]hikvisionBridgeTarget)
	s.bindingCount = len(rows)
	s.skippedBindingCount = 0
	s.mergedBindingCount = 0
	for _, row := range rows {
		target, ok, err := s.bindingToTargetForProvider(row.ProviderCode, row.SmartDeviceBinding)
		if err != nil {
			return nil, err
		}
		if !ok {
			s.skippedBindingCount++
			continue
		}
		if _, exists := targetMap[target.SessionKey]; exists {
			s.mergedBindingCount++
		}
		targetMap[target.SessionKey] = target
	}

	targets := make([]hikvisionBridgeTarget, 0, len(targetMap))
	for _, item := range targetMap {
		targets = append(targets, item)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].SessionKey < targets[j].SessionKey })
	return targets, nil
}

func (s *HikvisionAlarmBridgeService) bindingToTarget(binding entity.SmartDeviceBinding) (hikvisionBridgeTarget, bool, error) {
	return s.bindingToTargetForProvider("hikvision-sdk", binding)
}

func (s *HikvisionAlarmBridgeService) bindingToTargetForProvider(providerCode string, binding entity.SmartDeviceBinding) (hikvisionBridgeTarget, bool, error) {
	db := s.repo.DB()
	providerCode = strings.TrimSpace(providerCode)
	if providerCode == "" {
		providerCode = "hikvision-sdk"
	}
	switch binding.SourceType {
	case "recorder":
		var recorder entity.RecorderDevice
		if err := db.First(&recorder, binding.SourceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return hikvisionBridgeTarget{}, false, nil
			}
			return hikvisionBridgeTarget{}, false, err
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
		if err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return hikvisionBridgeTarget{
			ProviderCode: providerCode,
			SessionKey:   buildHikvisionTargetSessionKey(providerCode, "recorder", recorder.ID),
			DeviceType:   "recorder",
			DeviceID:     recorder.ID,
			DeviceName:   recorder.Name,
			DeviceIP:     recorder.IP,
			SDKPort:      recorder.SDKPort,
			HTTPPort:     recorder.HTTPPort,
			Username:     recorder.Username,
			Password:     password,
		}, true, nil
	case "channel":
		var channel entity.RecorderChannel
		if err := db.First(&channel, binding.SourceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return hikvisionBridgeTarget{}, false, nil
			}
			return hikvisionBridgeTarget{}, false, err
		}
		var recorder entity.RecorderDevice
		if err := db.First(&recorder, channel.RecorderID).Error; err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
		if err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return hikvisionBridgeTarget{
			ProviderCode: providerCode,
			SessionKey:   buildHikvisionTargetSessionKey(providerCode, "recorder", recorder.ID),
			DeviceType:   "recorder",
			DeviceID:     recorder.ID,
			DeviceName:   recorder.Name,
			DeviceIP:     recorder.IP,
			SDKPort:      recorder.SDKPort,
			HTTPPort:     recorder.HTTPPort,
			Username:     recorder.Username,
			Password:     password,
		}, true, nil
	case "camera":
		var camera entity.CameraDevice
		if err := db.First(&camera, binding.SourceID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return hikvisionBridgeTarget{}, false, nil
			}
			return hikvisionBridgeTarget{}, false, err
		}
		var linkedChannel entity.RecorderChannel
		if err := db.Where("camera_id = ?", camera.ID).Order("id ASC").First(&linkedChannel).Error; err == nil {
			var recorder entity.RecorderDevice
			if db.First(&recorder, linkedChannel.RecorderID).Error == nil {
				password, decryptErr := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
				if decryptErr == nil {
					return hikvisionBridgeTarget{
						ProviderCode: providerCode,
						SessionKey:   buildHikvisionTargetSessionKey(providerCode, "recorder", recorder.ID),
						DeviceType:   "recorder",
						DeviceID:     recorder.ID,
						DeviceName:   recorder.Name,
						DeviceIP:     recorder.IP,
						SDKPort:      recorder.SDKPort,
						HTTPPort:     recorder.HTTPPort,
						Username:     recorder.Username,
						Password:     password,
					}, true, nil
				}
			}
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), camera.PasswordEncrypted)
		if err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return hikvisionBridgeTarget{
			ProviderCode: providerCode,
			SessionKey:   buildHikvisionTargetSessionKey(providerCode, "camera", camera.ID),
			DeviceType:   "camera",
			DeviceID:     camera.ID,
			DeviceName:   camera.Name,
			DeviceIP:     camera.IP,
			SDKPort:      camera.SDKPort,
			HTTPPort:     camera.HTTPPort,
			Username:     camera.Username,
			Password:     password,
		}, true, nil
	default:
		return hikvisionBridgeTarget{}, false, nil
	}
}

func (s *HikvisionAlarmBridgeService) openSessionLocked(target hikvisionBridgeTarget) error {
	if s.sdk == nil {
		return fmt.Errorf("hikvision sdk not initialized")
	}
	var (
		userID     int32
		deviceInfo hikvision.DeviceInfo
		err        error
	)
	if target.DeviceType == "recorder" {
		userID, deviceInfo, err = s.sdk.LoginRecorder(target.DeviceIP, target.SDKPort, target.Username, target.Password)
	} else {
		userID, deviceInfo, err = s.sdk.LoginCamera(target.DeviceIP, target.SDKPort, target.Username, target.Password)
	}
	if err != nil {
		return err
	}
	alarmHandle, err := s.sdk.SetupMotionAlarm(userID)
	if err != nil {
		_ = s.sdk.Logout(userID)
		return err
	}
	session := &hikvisionBridgeSession{
		SessionKey:  target.SessionKey,
		DeviceType:  target.DeviceType,
		DeviceID:    target.DeviceID,
		DeviceName:  target.DeviceName,
		DeviceIP:    target.DeviceIP,
		UserID:      userID,
		AlarmHandle: alarmHandle,
		DeviceInfo:  deviceInfo,
	}
	s.sessions[session.SessionKey] = session
	s.sessionsByUserID[userID] = session
	s.sdk.RegisterSession(hikvision.SessionInfo{
		UserID:     userID,
		SessionKey: session.SessionKey,
		DeviceType: session.DeviceType,
		DeviceID:   session.DeviceID,
		DeviceName: session.DeviceName,
		DeviceIP:   session.DeviceIP,
		DeviceInfo: session.DeviceInfo,
	})
	return nil
}

func (s *HikvisionAlarmBridgeService) openISAPIStreamLocked(target hikvisionBridgeTarget) error {
	if strings.TrimSpace(target.DeviceIP) == "" {
		return fmt.Errorf("hikvision isapi device ip is empty")
	}
	httpPort := target.HTTPPort
	if httpPort <= 0 {
		httpPort = 80
	}
	ctx, cancel := context.WithCancel(context.Background())
	session := &hikvisionISAPIStreamSession{
		SessionKey: target.SessionKey,
		DeviceType: target.DeviceType,
		DeviceID:   target.DeviceID,
		DeviceName: target.DeviceName,
		DeviceIP:   target.DeviceIP,
		HTTPPort:   httpPort,
		Username:   target.Username,
		Password:   target.Password,
		cancel:     cancel,
	}
	s.isapiSessions[session.SessionKey] = session
	if s.logger != nil {
		s.logger.Info("hikvision isapi alert stream starting",
			zap.String("sessionKey", session.SessionKey),
			zap.String("deviceType", session.DeviceType),
			zap.Uint("deviceID", session.DeviceID),
			zap.String("deviceIp", session.DeviceIP),
			zap.Int("httpPort", session.HTTPPort),
		)
	}
	go s.runISAPIAlertStream(ctx, session)
	return nil
}

func (s *HikvisionAlarmBridgeService) runISAPIAlertStream(ctx context.Context, session *hikvisionISAPIStreamSession) {
	backoff := 2 * time.Second
	s.ensureISAPIHTTPCallback(ctx, session)
	for {
		if err := s.consumeISAPIAlertStream(ctx, session); err != nil && ctx.Err() == nil {
			s.markISAPIStreamError(session.SessionKey, err)
			if s.logger != nil {
				s.logger.Warn("hikvision isapi alert stream interrupted",
					zap.String("sessionKey", session.SessionKey),
					zap.String("deviceIp", session.DeviceIP),
					zap.Error(err),
				)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (s *HikvisionAlarmBridgeService) ensureISAPIHTTPCallback(ctx context.Context, session *hikvisionISAPIStreamSession) {
	callbackURL, err := s.buildISAPICallbackURL(session.DeviceIP, session.HTTPPort)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("hikvision isapi callback url resolve failed",
				zap.String("sessionKey", session.SessionKey),
				zap.String("deviceIp", session.DeviceIP),
				zap.Error(err),
			)
		}
		return
	}
	if err := configureHikvisionISAPIHTTPHost(ctx, session, callbackURL); err != nil {
		if s.logger != nil {
			s.logger.Warn("hikvision isapi http callback configure failed",
				zap.String("sessionKey", session.SessionKey),
				zap.String("deviceIp", session.DeviceIP),
				zap.String("callbackUrl", callbackURL),
				zap.Error(err),
			)
		}
		return
	}
	if s.logger != nil {
		s.logger.Info("hikvision isapi http callback configured",
			zap.String("sessionKey", session.SessionKey),
			zap.String("deviceIp", session.DeviceIP),
			zap.String("callbackUrl", callbackURL),
		)
	}
}

func (s *HikvisionAlarmBridgeService) consumeISAPIAlertStream(ctx context.Context, session *hikvisionISAPIStreamSession) error {
	streamURL := buildHikvisionISAPIAlertStreamURL(session.DeviceIP, session.HTTPPort)
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	response, err := doHikvisionISAPIStreamRequest(streamCtx, streamURL, session.Username, session.Password)
	if err != nil {
		return err
	}
	body := newIdleReadCloser(response.Body, hikvisionISAPIStreamIdleTimeout, func() {
		if s.logger != nil {
			s.logger.Warn("hikvision isapi alert stream idle timeout",
				zap.String("sessionKey", session.SessionKey),
				zap.String("deviceIp", session.DeviceIP),
				zap.Duration("idleTimeout", hikvisionISAPIStreamIdleTimeout),
			)
		}
		cancel()
	})
	defer body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("hikvision isapi alert stream status %d", response.StatusCode)
	}
	if s.logger != nil {
		s.logger.Info("hikvision isapi alert stream connected",
			zap.String("sessionKey", session.SessionKey),
			zap.String("deviceIp", session.DeviceIP),
			zap.Int("httpPort", session.HTTPPort),
			zap.String("contentType", response.Header.Get("Content-Type")),
			zap.Duration("idleTimeout", hikvisionISAPIStreamIdleTimeout),
		)
	}
	s.markISAPIStreamConnected(session.SessionKey)
	connectedAt := time.Now()
	watchDone := make(chan struct{})
	defer close(watchDone)
	go s.watchISAPIStreamIdle(streamCtx, watchDone, session, connectedAt, cancel, body.Close)
	contentType := response.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		return s.consumeISAPIMultipartStream(streamCtx, session, body, contentType)
	}
	return s.consumeISAPITextStream(streamCtx, session, body)
}

func (s *HikvisionAlarmBridgeService) watchISAPIStreamIdle(ctx context.Context, done <-chan struct{}, session *hikvisionISAPIStreamSession, connectedAt time.Time, cancel context.CancelFunc, closeBody func() error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			reference := connectedAt
			if lastByteAt := s.isapiLastByteAt(session.SessionKey); lastByteAt != nil {
				reference = *lastByteAt
			}
			idleFor := time.Since(reference)
			if idleFor < hikvisionISAPIStreamIdleTimeout {
				continue
			}
			if s.logger != nil {
				s.logger.Warn("hikvision isapi alert stream idle timeout",
					zap.String("sessionKey", session.SessionKey),
					zap.String("deviceIp", session.DeviceIP),
					zap.Duration("idleFor", idleFor),
					zap.Duration("idleTimeout", hikvisionISAPIStreamIdleTimeout),
				)
			}
			cancel()
			if closeBody != nil {
				_ = closeBody()
			}
			return
		}
	}
}

func (s *HikvisionAlarmBridgeService) consumeISAPIMultipartStream(ctx context.Context, session *hikvisionISAPIStreamSession, body io.Reader, contentType string) error {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return err
	}
	boundary := params["boundary"]
	if boundary == "" {
		return fmt.Errorf("hikvision isapi multipart boundary is empty")
	}
	reader := multipart.NewReader(body, boundary)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(part, 20<<20))
		if closeErr := part.Close(); readErr == nil {
			readErr = closeErr
		}
		if readErr != nil {
			return readErr
		}
		s.markISAPIStreamByte(session.SessionKey)
		partType := part.Header.Get("Content-Type")
		payload := buildISAPIStreamPartPayload(data, part.FileName(), partType)
		if payload == nil {
			continue
		}
		s.markISAPIStreamEvent(session.SessionKey)
		if s.logger != nil {
			s.logger.Info("hikvision isapi alert stream part received",
				zap.String("sessionKey", session.SessionKey),
				zap.String("contentType", partType),
				zap.String("filename", part.FileName()),
				zap.Int("size", len(data)),
			)
		}
		headers := map[string]string{
			"X-Request-Client-IP": session.DeviceIP,
			"X-Hikvision-Session": session.SessionKey,
			"Content-Type":        partType,
		}
		result, err := s.IngestISAPIMotionEvent("hikvision-isapi", payload, headers)
		if err != nil {
			s.logger.Error("hikvision isapi stream event ingest failed",
				zap.String("sessionKey", session.SessionKey),
				zap.Error(err),
			)
		} else if s.logger != nil {
			s.logger.Info("hikvision isapi stream event ingest result",
				zap.String("sessionKey", session.SessionKey),
				zap.Any("result", result),
			)
		}
	}
}

func (s *HikvisionAlarmBridgeService) consumeISAPITextStream(ctx context.Context, session *hikvisionISAPIStreamSession, body io.Reader) error {
	reader := bufio.NewReader(body)
	var buffer strings.Builder
	chunk := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, err := reader.Read(chunk)
		if n > 0 {
			s.markISAPIStreamByte(session.SessionKey)
			buffer.Write(chunk[:n])
			for {
				payload, ok := nextISAPITextPayload(&buffer)
				if !ok {
					break
				}
				s.ingestISAPIStreamPayload(session, payload, "application/xml")
			}
			if buffer.Len() > 4*1024*1024 {
				return fmt.Errorf("hikvision isapi text stream buffer exceeded 4MB without complete event")
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func nextISAPITextPayload(buffer *strings.Builder) (string, bool) {
	text := buffer.String()
	lower := strings.ToLower(text)
	endTag := "</eventnotificationalert>"
	endIndex := strings.Index(lower, endTag)
	if endIndex < 0 {
		return "", false
	}
	eventEnd := endIndex + len(endTag)
	startIndex := strings.LastIndex(strings.ToLower(text[:eventEnd]), "<eventnotificationalert")
	if startIndex < 0 {
		startIndex = 0
	}
	payload := strings.TrimSpace(text[startIndex:eventEnd])
	remaining := text[eventEnd:]
	buffer.Reset()
	buffer.WriteString(remaining)
	return payload, payload != ""
}

func (s *HikvisionAlarmBridgeService) ingestISAPIStreamPayload(session *hikvisionISAPIStreamSession, payload string, contentType string) {
	s.markISAPIStreamEvent(session.SessionKey)
	if s.logger != nil {
		s.logger.Info("hikvision isapi alert stream payload received",
			zap.String("sessionKey", session.SessionKey),
			zap.String("contentType", contentType),
			zap.Int("size", len(payload)),
			zap.String("preview", truncateText(payload, 300)),
		)
	}
	headers := map[string]string{
		"X-Request-Client-IP": session.DeviceIP,
		"X-Hikvision-Session": session.SessionKey,
		"Content-Type":        contentType,
	}
	result, err := s.IngestISAPIMotionEvent("hikvision-isapi", payload, headers)
	if err != nil {
		s.logger.Error("hikvision isapi stream event ingest failed",
			zap.String("sessionKey", session.SessionKey),
			zap.Error(err),
		)
	} else if s.logger != nil {
		s.logger.Info("hikvision isapi stream event ingest result",
			zap.String("sessionKey", session.SessionKey),
			zap.Any("result", result),
		)
	}
}

func buildHikvisionTargetSessionKey(providerCode, deviceType string, deviceID uint) string {
	base := fmt.Sprintf("%s:%d", deviceType, deviceID)
	if providerCode == "hikvision-isapi" {
		return "isapi-" + base
	}
	return base
}

func buildHikvisionISAPIAlertStreamURL(deviceIP string, httpPort int) string {
	if httpPort <= 0 {
		httpPort = 80
	}
	scheme := "http"
	if httpPort == 443 {
		scheme = "https"
	}
	return (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(deviceIP, strconv.Itoa(httpPort)),
		Path:   "/ISAPI/Event/notification/alertStream",
	}).String()
}

func (s *HikvisionAlarmBridgeService) buildISAPICallbackURL(deviceIP string, httpPort int) (string, error) {
	base := strings.TrimSpace(s.cfg.BackendPublicBaseURL)
	if base != "" {
		if parsed, err := url.Parse(base); err == nil {
			host := parsed.Hostname()
			if host != "" && host != "127.0.0.1" && host != "localhost" && host != "::1" {
				parsed.Path = "/smart/events/ingest/hikvision-isapi"
				parsed.RawQuery = ""
				parsed.Fragment = ""
				return parsed.String(), nil
			}
		}
	}
	localIP, err := localOutboundIP(deviceIP, httpPort)
	if err != nil {
		return "", err
	}
	port := s.cfg.HTTPPort
	if port <= 0 {
		port = 8000
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(localIP, strconv.Itoa(port)),
		Path:   "/smart/events/ingest/hikvision-isapi",
	}).String(), nil
}

func localOutboundIP(remoteIP string, remotePort int) (string, error) {
	if remotePort <= 0 {
		remotePort = 80
	}
	conn, err := net.DialTimeout("udp", net.JoinHostPort(remoteIP, strconv.Itoa(remotePort)), 3*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok && localAddr.IP != nil {
		return localAddr.IP.String(), nil
	}
	return "", fmt.Errorf("resolve local outbound ip failed")
}

func configureHikvisionISAPIHTTPHost(ctx context.Context, session *hikvisionISAPIStreamSession, callbackURL string) error {
	parsed, err := url.Parse(callbackURL)
	if err != nil {
		return err
	}
	host := parsed.Hostname()
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	addressingType := "hostname"
	addressTag := fmt.Sprintf("<hostName>%s</hostName>", xmlEscape(host))
	if ip := net.ParseIP(host); ip != nil {
		addressingType = "ipaddress"
		addressTag = fmt.Sprintf("<ipAddress>%s</ipAddress>", xmlEscape(host))
	}
	payload := []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<HttpHostNotification version="1.0" xmlns="urn:psialliance-org">
<id>1</id>
<url>%s</url>
<protocolType>%s</protocolType>
<parameterFormatType>XML</parameterFormatType>
<addressingFormatType>%s</addressingFormatType>
%s
<portNo>%s</portNo>
<httpAuthenticationMethod>none</httpAuthenticationMethod>
<Extensions xmlns="http://www.hikvision.com/ver20/XMLSchema">
<intervalBetweenEvents>0</intervalBetweenEvents>
</Extensions>
</HttpHostNotification>`,
		xmlEscape(path),
		strings.ToUpper(firstNonEmpty(parsed.Scheme, "http")),
		addressingType,
		addressTag,
		xmlEscape(port),
	))
	targetURL := buildHikvisionISAPIHTTPHostURL(session.DeviceIP, session.HTTPPort, 1)
	return doHikvisionISAPIPutXML(ctx, targetURL, session.Username, session.Password, payload)
}

func buildHikvisionISAPIHTTPHostURL(deviceIP string, httpPort int, hostID int) string {
	if httpPort <= 0 {
		httpPort = 80
	}
	scheme := "http"
	if httpPort == 443 {
		scheme = "https"
	}
	return (&url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(deviceIP, strconv.Itoa(httpPort)),
		Path:   fmt.Sprintf("/ISAPI/Event/notification/httpHosts/%d", hostID),
	}).String()
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func buildISAPIStreamPartPayload(data []byte, filename, contentType string) any {
	if len(data) == 0 {
		return nil
	}
	normalizedType := strings.ToLower(contentType)
	if strings.HasPrefix(normalizedType, "image/") || looksLikeJPEG(data) {
		return map[string]any{
			"files": []SmartIngestFile{{
				Filename:    filename,
				ContentType: contentType,
				Data:        data,
			}},
		}
	}
	text := string(data)
	return map[string]any{
		"fields": map[string][]string{"body": {text}},
	}
}

func doHikvisionISAPIStreamRequest(ctx context.Context, streamURL, username, password string) (*http.Response, error) {
	client := &http.Client{}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return nil, err
	}
	setHikvisionISAPIStreamHeaders(request)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusUnauthorized {
		return response, nil
	}
	challenge := response.Header.Get("WWW-Authenticate")
	_ = response.Body.Close()
	authHeader, err := buildHikvisionAuthHeader(http.MethodGet, request.URL.RequestURI(), challenge, username, password)
	if err != nil {
		return nil, err
	}
	request, err = http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return nil, err
	}
	setHikvisionISAPIStreamHeaders(request)
	request.Header.Set("Authorization", authHeader)
	return client.Do(request)
}

func doHikvisionISAPIPutXML(ctx context.Context, targetURL, username, password string, payload []byte) error {
	client := &http.Client{Timeout: 8 * time.Second}
	body, statusCode, challenge, err := doHikvisionISAPIPutXMLOnce(ctx, client, targetURL, username, password, payload, "")
	if err == nil && statusCode >= 200 && statusCode < 300 {
		return nil
	}
	if statusCode != http.StatusUnauthorized {
		if err != nil {
			return err
		}
		return fmt.Errorf("PUT %s returned status %d: %s", targetURL, statusCode, truncateText(string(body), 300))
	}
	authHeader, err := buildHikvisionAuthHeader(http.MethodPut, mustRequestURI(targetURL), challenge, username, password)
	if err != nil {
		return err
	}
	body, statusCode, _, err = doHikvisionISAPIPutXMLOnce(ctx, client, targetURL, username, password, payload, authHeader)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("PUT %s returned status %d: %s", targetURL, statusCode, truncateText(string(body), 300))
	}
	return nil
}

func doHikvisionISAPIPutXMLOnce(ctx context.Context, client *http.Client, targetURL, username, password string, payload []byte, authorization string) ([]byte, int, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, targetURL, strings.NewReader(string(payload)))
	if err != nil {
		return nil, 0, "", err
	}
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Accept", "application/xml, text/xml, */*")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	} else if username != "" || password != "" {
		request.SetBasicAuth(username, password)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, 0, "", err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(response.Body)
	return body, response.StatusCode, response.Header.Get("WWW-Authenticate"), readErr
}

func mustRequestURI(targetURL string) string {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return targetURL
	}
	return parsed.RequestURI()
}

func setHikvisionISAPIStreamHeaders(request *http.Request) {
	request.Header.Set("Accept", "multipart/x-mixed-replace, application/xml, text/xml, application/json, */*")
	request.Header.Set("Cache-Control", "no-cache")
	request.Header.Set("Connection", "keep-alive")
	request.Header.Set("User-Agent", "SecmgmtV2-Hikvision-ISAPI/1.0")
}

func buildHikvisionAuthHeader(method, uri, challenge, username, password string) (string, error) {
	challenge = strings.TrimSpace(challenge)
	if strings.HasPrefix(strings.ToLower(challenge), "basic") {
		token := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
		return "Basic " + token, nil
	}
	if !strings.HasPrefix(strings.ToLower(challenge), "digest") {
		return "", fmt.Errorf("unsupported hikvision auth challenge: %s", challenge)
	}
	params := parseHikvisionDigestChallenge(strings.TrimSpace(challenge[len("Digest"):]))
	realm := params["realm"]
	nonce := params["nonce"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("invalid hikvision digest challenge")
	}
	qop := firstDigestQOP(params["qop"])
	algorithm := strings.ToUpper(firstNonEmpty(params["algorithm"], "MD5"))
	if algorithm != "MD5" {
		return "", fmt.Errorf("unsupported hikvision digest algorithm: %s", algorithm)
	}
	cnonce := randomHex(8)
	nc := "00000001"
	ha1 := hikvisionMD5Hex(username + ":" + realm + ":" + password)
	ha2 := hikvisionMD5Hex(method + ":" + uri)
	response := ""
	if qop != "" {
		response = hikvisionMD5Hex(strings.Join([]string{ha1, nonce, nc, cnonce, qop, ha2}, ":"))
	} else {
		response = hikvisionMD5Hex(strings.Join([]string{ha1, nonce, ha2}, ":"))
	}
	parts := []string{
		fmt.Sprintf(`username="%s"`, username),
		fmt.Sprintf(`realm="%s"`, realm),
		fmt.Sprintf(`nonce="%s"`, nonce),
		fmt.Sprintf(`uri="%s"`, uri),
		fmt.Sprintf(`response="%s"`, response),
	}
	if params["opaque"] != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, params["opaque"]))
	}
	if qop != "" {
		parts = append(parts,
			fmt.Sprintf(`qop=%s`, qop),
			fmt.Sprintf(`nc=%s`, nc),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
		)
	}
	return "Digest " + strings.Join(parts, ", "), nil
}

func parseHikvisionDigestChallenge(value string) map[string]string {
	result := make(map[string]string)
	for _, part := range splitAuthHeader(value) {
		key, rawValue, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		rawValue = strings.TrimSpace(rawValue)
		rawValue = strings.Trim(rawValue, `"`)
		result[key] = rawValue
	}
	return result
}

func splitAuthHeader(value string) []string {
	parts := make([]string, 0)
	var builder strings.Builder
	inQuote := false
	for _, item := range value {
		switch item {
		case '"':
			inQuote = !inQuote
			builder.WriteRune(item)
		case ',':
			if inQuote {
				builder.WriteRune(item)
			} else if strings.TrimSpace(builder.String()) != "" {
				parts = append(parts, strings.TrimSpace(builder.String()))
				builder.Reset()
			}
		default:
			builder.WriteRune(item)
		}
	}
	if strings.TrimSpace(builder.String()) != "" {
		parts = append(parts, strings.TrimSpace(builder.String()))
	}
	return parts
}

func firstDigestQOP(value string) string {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "auth" {
			return item
		}
	}
	return ""
}

func hikvisionMD5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomHex(size int) string {
	if size <= 0 {
		size = 8
	}
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data)
}

func (s *HikvisionAlarmBridgeService) handleAlarm(alarm hikvision.MotionAlarm) {
	s.mu.RLock()
	session := s.sessionsByUserID[alarm.UserID]
	s.mu.RUnlock()
	if session == nil {
		return
	}

	for _, rawChannelNo := range alarm.Channels {
		channelNo := hikvision.NormalizeAlarmChannelNo(session.DeviceInfo, rawChannelNo)
		if err := s.persistMotionEventForProvider("hikvision-sdk", session, alarm.DeviceIP, rawChannelNo, channelNo, int(alarm.Command), alarm.EventTime, alarm.ImageData, alarm.ImageType); err != nil {
			if errors.Is(err, errHikvisionMotionBindingNotMatched) {
				continue
			}
			s.logger.Error("persist hikvision motion event failed",
				zap.String("sessionKey", session.SessionKey),
				zap.Int("channelNo", channelNo),
				zap.Error(err),
			)
		}
	}
}

func (s *HikvisionAlarmBridgeService) IngestISAPIMotionEvent(providerCode string, payload any, headers map[string]string) (map[string]any, error) {
	event, ok := parseHikvisionISAPIMotionPayload(payload, headers)
	if !ok {
		s.logger.Warn("hikvision isapi event ignored",
			zap.String("reason", "not a hikvision motion event"),
			zap.Any("headers", headers),
			zap.Any("payloadSummary", summarizeISAPIPayload(payload)),
		)
		return map[string]any{"accepted": false, "reason": "not a hikvision motion event"}, nil
	}
	session, ok, err := s.resolveISAPISession(event.DeviceIP, event.ChannelNo)
	if err != nil {
		return nil, err
	}
	if !ok {
		s.logger.Warn("hikvision isapi event ignored",
			zap.String("reason", "device or channel not matched"),
			zap.String("deviceIp", event.DeviceIP),
			zap.Int("channelNo", event.ChannelNo),
			zap.Any("headers", headers),
			zap.Any("payloadSummary", summarizeISAPIPayload(payload)),
		)
		return map[string]any{"accepted": false, "reason": "device or channel not matched"}, nil
	}
	if event.RawChannelNo <= 0 {
		event.RawChannelNo = event.ChannelNo
	}
	if err := s.persistMotionEventForProvider(providerCode, session, event.DeviceIP, event.RawChannelNo, event.ChannelNo, 0x6009, event.EventTime, event.ImageData, event.ImageType); err != nil {
		if errors.Is(err, errHikvisionMotionBindingNotMatched) {
			s.logger.Warn("hikvision isapi event ignored",
				zap.String("reason", "smart binding not matched"),
				zap.String("provider", providerCode),
				zap.String("deviceIp", event.DeviceIP),
				zap.Int("channelNo", event.ChannelNo),
				zap.String("sessionKey", session.SessionKey),
			)
			return map[string]any{
				"accepted":  false,
				"reason":    "smart binding not matched",
				"provider":  providerCode,
				"deviceIp":  event.DeviceIP,
				"channelNo": event.ChannelNo,
			}, nil
		}
		return nil, err
	}
	return map[string]any{
		"accepted":  true,
		"provider":  providerCode,
		"deviceIp":  event.DeviceIP,
		"channelNo": event.ChannelNo,
		"hasImage":  len(event.ImageData) > 0,
	}, nil
}

type hikvisionISAPIMotionEvent struct {
	DeviceIP     string
	RawChannelNo int
	ChannelNo    int
	EventTime    *time.Time
	ImageData    []byte
	ImageType    byte
}

func parseHikvisionISAPIMotionPayload(payload any, headers map[string]string) (hikvisionISAPIMotionEvent, bool) {
	var event hikvisionISAPIMotionEvent
	values := flattenPayloadValues(payload)
	bodyText := payloadText(payload)
	for key, value := range headers {
		values = append(values, flattenedPayloadValue{Key: key, Value: value})
	}
	event.DeviceIP = firstPayloadText(values, "ipAddress", "ipv4Address", "deviceIP", "sourceIP", "remoteHost")
	if event.DeviceIP == "" {
		event.DeviceIP = firstClientIP(headers)
	}
	event.ChannelNo = firstPayloadInt(values, "channelID", "channelNo", "channel", "dynChannelID", "inputProxyChannelID")
	event.RawChannelNo = event.ChannelNo
	event.EventTime = firstPayloadTime(values, "dateTime", "eventTime", "alarmTime", "triggerTime", "time")
	if event.ChannelNo <= 0 {
		event.ChannelNo = extractChannelNoFromText(bodyText)
		event.RawChannelNo = event.ChannelNo
	}
	state := strings.ToLower(strings.TrimSpace(firstPayloadText(values, "eventState", "state", "status")))
	if state != "" && state != "active" && state != "start" && state != "true" && state != "1" {
		return event, false
	}
	eventType := firstPayloadText(values, "eventType", "eventTypeCode", "eventDescription", "event")
	if eventType == "" {
		eventType = bodyText
	}
	if !isHikvisionMotionEventText(eventType) && !isHikvisionMotionEventText(bodyText) {
		return event, false
	}
	event.ImageData, event.ImageType = firstPayloadImage(payload)
	if event.ChannelNo <= 0 {
		event.ChannelNo = 1
		event.RawChannelNo = 1
	}
	return event, true
}

type flattenedPayloadValue struct {
	Key   string
	Value string
}

func flattenPayloadValues(payload any) []flattenedPayloadValue {
	values := make([]flattenedPayloadValue, 0)
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch item := value.(type) {
		case map[string]any:
			for key, child := range item {
				walk(key, child)
			}
		case map[string][]string:
			for key, list := range item {
				for _, child := range list {
					values = append(values, flattenedPayloadValue{Key: key, Value: child})
					values = append(values, extractStructuredTextValues(child)...)
				}
			}
		case []any:
			for _, child := range item {
				walk(prefix, child)
			}
		case []string:
			for _, child := range item {
				values = append(values, flattenedPayloadValue{Key: prefix, Value: child})
				values = append(values, extractStructuredTextValues(child)...)
			}
		case string:
			values = append(values, flattenedPayloadValue{Key: prefix, Value: item})
			values = append(values, extractStructuredTextValues(item)...)
		case []byte:
			text := string(item)
			values = append(values, flattenedPayloadValue{Key: prefix, Value: text})
			values = append(values, extractStructuredTextValues(text)...)
		case float64:
			values = append(values, flattenedPayloadValue{Key: prefix, Value: fmt.Sprintf("%.0f", item)})
		case int:
			values = append(values, flattenedPayloadValue{Key: prefix, Value: fmt.Sprintf("%d", item)})
		case int64:
			values = append(values, flattenedPayloadValue{Key: prefix, Value: fmt.Sprintf("%d", item)})
		case json.Number:
			values = append(values, flattenedPayloadValue{Key: prefix, Value: item.String()})
		case bool:
			values = append(values, flattenedPayloadValue{Key: prefix, Value: fmt.Sprintf("%t", item)})
		}
	}
	walk("", payload)
	return values
}

func extractStructuredTextValues(text string) []flattenedPayloadValue {
	values := extractXMLValues(text)
	values = append(values, extractJSONValues(text)...)
	return values
}

func extractXMLValues(text string) []flattenedPayloadValue {
	result := make([]flattenedPayloadValue, 0)
	matches := regexpXMLTag.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			result = append(result, flattenedPayloadValue{Key: match[1], Value: strings.TrimSpace(match[2])})
		}
	}
	return result
}

func extractJSONValues(text string) []flattenedPayloadValue {
	text = strings.TrimSpace(text)
	if text == "" || (!strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[")) {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var parsed any
	if err := decoder.Decode(&parsed); err != nil {
		return nil
	}
	result := make([]flattenedPayloadValue, 0)
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch item := value.(type) {
		case map[string]any:
			for key, child := range item {
				walk(key, child)
			}
		case []any:
			for _, child := range item {
				walk(prefix, child)
			}
		case string:
			result = append(result, flattenedPayloadValue{Key: prefix, Value: item})
		case json.Number:
			result = append(result, flattenedPayloadValue{Key: prefix, Value: item.String()})
		case float64:
			result = append(result, flattenedPayloadValue{Key: prefix, Value: fmt.Sprintf("%.0f", item)})
		case bool:
			result = append(result, flattenedPayloadValue{Key: prefix, Value: fmt.Sprintf("%t", item)})
		case nil:
		default:
			result = append(result, flattenedPayloadValue{Key: prefix, Value: fmt.Sprint(item)})
		}
	}
	walk("", parsed)
	return result
}

func payloadText(payload any) string {
	switch item := payload.(type) {
	case string:
		return item
	case []byte:
		return string(item)
	case map[string]any:
		if body, ok := item["body"].(string); ok {
			return body
		}
		if fields, ok := item["fields"].(map[string][]string); ok {
			for _, key := range []string{"body", "event", "EventNotificationAlert", "xml"} {
				if values := fields[key]; len(values) > 0 {
					return strings.Join(values, "\n")
				}
			}
		}
	}
	return ""
}

func firstPayloadText(values []flattenedPayloadValue, keys ...string) string {
	for _, key := range keys {
		for _, item := range values {
			if strings.EqualFold(strings.TrimSpace(item.Key), key) && strings.TrimSpace(item.Value) != "" {
				return strings.TrimSpace(item.Value)
			}
		}
	}
	return ""
}

func firstPayloadTime(values []flattenedPayloadValue, keys ...string) *time.Time {
	for _, key := range keys {
		for _, item := range values {
			if !strings.EqualFold(strings.TrimSpace(item.Key), key) {
				continue
			}
			if parsed := parseHikvisionCallbackTime(item.Value); parsed != nil {
				return parsed
			}
		}
	}
	return nil
}

func parseHikvisionCallbackTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		local := parsed.Local()
		return &local
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"20060102150405",
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &parsed
		}
	}
	return nil
}

func firstPayloadInt(values []flattenedPayloadValue, keys ...string) int {
	for _, key := range keys {
		for _, item := range values {
			if !strings.EqualFold(strings.TrimSpace(item.Key), key) {
				continue
			}
			var parsed int
			if _, err := fmt.Sscanf(strings.TrimSpace(item.Value), "%d", &parsed); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return 0
}

func firstPayloadImage(payload any) ([]byte, byte) {
	if item, ok := payload.(map[string]any); ok {
		if files, ok := item["files"].([]SmartIngestFile); ok {
			for _, file := range files {
				if len(file.Data) == 0 {
					continue
				}
				contentType := strings.ToLower(file.ContentType)
				if strings.Contains(contentType, "jpeg") || strings.Contains(contentType, "jpg") || looksLikeJPEG(file.Data) {
					image := make([]byte, len(file.Data))
					copy(image, file.Data)
					return image, 1
				}
			}
		}
		for _, key := range []string{"imageData", "image", "pictureData", "picture", "picData"} {
			if text, ok := item[key].(string); ok {
				if image := decodeBase64Image(text); len(image) > 0 {
					return image, 1
				}
			}
		}
	}
	return nil, 0
}

func decodeBase64Image(value string) []byte {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if comma := strings.Index(value, ","); comma >= 0 {
		value = value[comma+1:]
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}

func firstClientIP(headers map[string]string) string {
	for _, key := range []string{"X-Forwarded-For", "X-Real-IP", "X-Request-Client-IP"} {
		value := strings.TrimSpace(headers[key])
		if value == "" {
			continue
		}
		if comma := strings.Index(value, ","); comma >= 0 {
			value = strings.TrimSpace(value[:comma])
		}
		if host, _, err := net.SplitHostPort(value); err == nil {
			value = host
		}
		if net.ParseIP(value) != nil {
			return value
		}
	}
	return ""
}

func extractChannelNoFromText(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	for _, pattern := range []string{"channelID", "channelNo", "channel"} {
		re := regexp.MustCompile(`(?i)` + pattern + `[^0-9]{0,16}([0-9]+)`)
		if match := re.FindStringSubmatch(text); len(match) >= 2 {
			var parsed int
			if _, err := fmt.Sscanf(match[1], "%d", &parsed); err == nil {
				return parsed
			}
		}
	}
	return 0
}

func isHikvisionMotionEventText(value string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", ""))
	normalized = strings.ReplaceAll(normalized, "-", "")
	switch {
	case strings.Contains(value, "移动侦测"):
		return true
	case strings.Contains(normalized, "vmd"):
		return true
	case strings.Contains(normalized, "motion"):
		return true
	case strings.Contains(normalized, "fielddetection"):
		return true
	case strings.Contains(normalized, "videomotion"):
		return true
	default:
		return false
	}
}

func summarizeISAPIPayload(payload any) any {
	switch item := payload.(type) {
	case string:
		return truncateText(item, 1000)
	case []byte:
		return map[string]any{
			"type": "bytes",
			"size": len(item),
			"text": truncateText(string(item), 1000),
		}
	case map[string]any:
		summary := make(map[string]any, len(item))
		for key, value := range item {
			switch typed := value.(type) {
			case []SmartIngestFile:
				files := make([]map[string]any, 0, len(typed))
				for _, file := range typed {
					files = append(files, map[string]any{
						"filename":    file.Filename,
						"contentType": file.ContentType,
						"size":        len(file.Data),
					})
				}
				summary[key] = files
			case map[string][]string:
				fields := make(map[string][]string, len(typed))
				for fieldKey, values := range typed {
					copied := make([]string, 0, len(values))
					for _, value := range values {
						copied = append(copied, truncateText(value, 1000))
					}
					fields[fieldKey] = copied
				}
				summary[key] = fields
			case string:
				summary[key] = truncateText(typed, 1000)
			default:
				summary[key] = typed
			}
		}
		return summary
	default:
		return truncateText(fmt.Sprintf("%v", payload), 1000)
	}
}

func truncateText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
}

func (s *HikvisionAlarmBridgeService) resolveISAPISession(deviceIP string, channelNo int) (*hikvisionBridgeSession, bool, error) {
	deviceIP = strings.TrimSpace(deviceIP)
	db := s.repo.DB()
	if deviceIP != "" {
		var recorder entity.RecorderDevice
		if err := db.Where("ip = ?", deviceIP).First(&recorder).Error; err == nil {
			return &hikvisionBridgeSession{
				SessionKey: fmt.Sprintf("isapi-recorder:%d", recorder.ID),
				DeviceType: "recorder",
				DeviceID:   recorder.ID,
				DeviceName: recorder.Name,
				DeviceIP:   recorder.IP,
			}, true, nil
		} else if err != gorm.ErrRecordNotFound {
			return nil, false, err
		}
		var camera entity.CameraDevice
		if err := db.Where("ip = ?", deviceIP).First(&camera).Error; err == nil {
			return &hikvisionBridgeSession{
				SessionKey: fmt.Sprintf("isapi-camera:%d", camera.ID),
				DeviceType: "camera",
				DeviceID:   camera.ID,
				DeviceName: camera.Name,
				DeviceIP:   camera.IP,
			}, true, nil
		} else if err != gorm.ErrRecordNotFound {
			return nil, false, err
		}
	}
	if channelNo > 0 {
		var channel entity.RecorderChannel
		if err := db.Where("channel_no = ?", channelNo).Order("id ASC").First(&channel).Error; err == nil {
			var recorder entity.RecorderDevice
			if err := db.First(&recorder, channel.RecorderID).Error; err != nil {
				return nil, false, err
			}
			return &hikvisionBridgeSession{
				SessionKey: fmt.Sprintf("isapi-recorder:%d", recorder.ID),
				DeviceType: "recorder",
				DeviceID:   recorder.ID,
				DeviceName: recorder.Name,
				DeviceIP:   firstNonEmpty(deviceIP, recorder.IP),
			}, true, nil
		} else if err != gorm.ErrRecordNotFound {
			return nil, false, err
		}
	}
	return nil, false, nil
}

func (s *HikvisionAlarmBridgeService) aggregateMotionEvent(
	session *hikvisionBridgeSession,
	deviceIP string,
	rawChannelNo int,
	channelNo int,
	command int,
) error {
	eventTime := time.Now()
	key := buildMotionAggregateKey(session.SessionKey, channelNo)

	s.motionMu.Lock()
	if s.motionWindows == nil {
		s.motionWindows = make(map[string]*motionAggregateWindow)
	}
	if s.motionCooldown == nil {
		s.motionCooldown = make(map[string]time.Time)
	}
	if cooldownUntil, ok := s.motionCooldown[key]; ok {
		if eventTime.Before(cooldownUntil) {
			s.motionMu.Unlock()
			return nil
		}
		delete(s.motionCooldown, key)
	}
	if window := s.motionWindows[key]; window != nil {
		window.Count++
		window.LastEventTime = eventTime
		window.RawChannelNo = rawChannelNo
		window.Command = command
		if strings.TrimSpace(deviceIP) != "" {
			window.DeviceIP = deviceIP
		}
		s.motionMu.Unlock()
		return nil
	}
	s.motionMu.Unlock()

	window, err := s.buildMotionAggregateWindow(session, deviceIP, rawChannelNo, channelNo, command, eventTime, key)
	if err != nil || window == nil {
		return err
	}

	s.motionMu.Lock()
	if cooldownUntil, ok := s.motionCooldown[key]; ok {
		if eventTime.Before(cooldownUntil) {
			s.motionMu.Unlock()
			return nil
		}
		delete(s.motionCooldown, key)
	}
	if existing := s.motionWindows[key]; existing != nil {
		existing.Count++
		existing.LastEventTime = eventTime
		existing.RawChannelNo = rawChannelNo
		existing.Command = command
		if strings.TrimSpace(deviceIP) != "" {
			existing.DeviceIP = deviceIP
		}
		s.motionMu.Unlock()
		return nil
	}
	s.motionWindows[key] = window
	s.motionMu.Unlock()

	snapshotURL := ""
	if shouldCaptureMotionSnapshot(window.Rule, entity.SmartEvent{}) && window.SnapshotEntityID != 0 {
		snapshotURL = s.captureSnapshot(session, window.SnapshotEntityID, channelNo, eventTime)
	}

	s.motionMu.Lock()
	if current := s.motionWindows[key]; current == window {
		window.SnapshotURL = snapshotURL
		if window.DedupWindowSeconds <= 0 {
			delete(s.motionWindows, key)
			s.setMotionCooldownLocked(window)
			s.motionMu.Unlock()
			return s.persistMotionAggregate(window)
		}
		window.Timer = time.AfterFunc(time.Duration(window.DedupWindowSeconds)*time.Second, func() {
			s.flushMotionWindow(key)
		})
	}
	s.motionMu.Unlock()
	return nil
}

func (s *HikvisionAlarmBridgeService) buildMotionAggregateWindow(
	session *hikvisionBridgeSession,
	deviceIP string,
	rawChannelNo int,
	channelNo int,
	command int,
	eventTime time.Time,
	key string,
) (*motionAggregateWindow, error) {
	var window *motionAggregateWindow
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var (
			camera   *entity.CameraDevice
			recorder *entity.RecorderDevice
			channel  *entity.RecorderChannel
		)

		if session.DeviceType == "recorder" {
			var recorderValue entity.RecorderDevice
			if err := tx.First(&recorderValue, session.DeviceID).Error; err != nil {
				return err
			}
			recorder = &recorderValue
			channelValue, err := s.ensureRecorderChannel(tx, recorderValue, channelNo)
			if err != nil {
				return err
			}
			channel = &channelValue
			if channel.CameraID != nil {
				var cameraValue entity.CameraDevice
				if err := tx.First(&cameraValue, *channel.CameraID).Error; err == nil {
					camera = &cameraValue
				}
			}
		} else {
			var cameraValue entity.CameraDevice
			if err := tx.First(&cameraValue, session.DeviceID).Error; err != nil {
				return err
			}
			camera = &cameraValue
			var channelValue entity.RecorderChannel
			if err := tx.Where("camera_id = ?", cameraValue.ID).Order("id ASC").First(&channelValue).Error; err == nil {
				channel = &channelValue
				var recorderValue entity.RecorderDevice
				if err := tx.First(&recorderValue, channelValue.RecorderID).Error; err == nil {
					recorder = &recorderValue
				}
			}
		}

		provider, capability, binding, rule, err := s.matchMotionBinding(tx, camera, recorder, channel)
		if err != nil {
			return err
		}
		if provider == nil || capability == nil || binding == nil {
			return errHikvisionMotionBindingNotMatched
		}

		eventLevel := "medium"
		dedupWindow := 60
		cooldownWindow := 0
		var ruleCopy *entity.SmartBindingRule
		if rule != nil {
			copied := *rule
			ruleCopy = &copied
			if strings.TrimSpace(rule.AlarmLevel) != "" {
				eventLevel = rule.AlarmLevel
			}
			if rule.DedupWindowSeconds > 0 {
				dedupWindow = rule.DedupWindowSeconds
			}
			if rule.CooldownSeconds > 0 {
				cooldownWindow = rule.CooldownSeconds
			}
		}

		window = &motionAggregateWindow{
			Key:                key,
			DedupKey:           buildSmartDedupKey(binding.SourceType, binding.SourceID, channelNo, firstZoneID(camera, channel)),
			SessionKey:         session.SessionKey,
			DeviceType:         session.DeviceType,
			DeviceID:           session.DeviceID,
			DeviceIP:           deviceIP,
			RawChannelNo:       rawChannelNo,
			ChannelNo:          channelNo,
			Command:            command,
			FirstEventTime:     eventTime,
			LastEventTime:      eventTime,
			Count:              1,
			ProviderID:         provider.ID,
			CapabilityID:       capability.ID,
			BindingID:          binding.ID,
			SourceType:         binding.SourceType,
			SourceID:           binding.SourceID,
			EventLevel:         eventLevel,
			CameraID:           nullableEntityID(camera),
			RecorderID:         nullableEntityID(recorder),
			ChannelID:          nullableEntityID(channel),
			FactoryID:          firstFactoryID(camera, recorder, channel),
			ZoneID:             firstZoneID(camera, channel),
			Rule:               ruleCopy,
			SnapshotEntityID:   resolveSnapshotEntityID(camera, channel, recorder, session.DeviceID),
			DedupWindowSeconds: dedupWindow,
			CooldownSeconds:    cooldownWindow,
		}
		return nil
	})
	return window, err
}

func (s *HikvisionAlarmBridgeService) flushMotionWindow(key string) {
	s.motionMu.Lock()
	window := s.motionWindows[key]
	if window == nil {
		s.motionMu.Unlock()
		return
	}
	delete(s.motionWindows, key)
	s.setMotionCooldownLocked(window)
	s.motionMu.Unlock()

	if err := s.persistMotionAggregate(window); err != nil {
		s.logger.Error("flush hikvision motion aggregate failed",
			zap.String("key", key),
			zap.String("dedupKey", window.DedupKey),
			zap.Error(err),
		)
	}
}

func (s *HikvisionAlarmBridgeService) flushAllMotionWindows() {
	s.motionMu.Lock()
	windows := make([]*motionAggregateWindow, 0, len(s.motionWindows))
	for key, window := range s.motionWindows {
		if window.Timer != nil {
			window.Timer.Stop()
		}
		windows = append(windows, window)
		delete(s.motionWindows, key)
	}
	s.motionMu.Unlock()

	for _, window := range windows {
		if err := s.persistMotionAggregate(window); err != nil {
			s.logger.Error("flush hikvision motion aggregate failed",
				zap.String("key", window.Key),
				zap.String("dedupKey", window.DedupKey),
				zap.Error(err),
			)
		}
	}
}

func (s *HikvisionAlarmBridgeService) setMotionCooldownLocked(window *motionAggregateWindow) {
	if window == nil || window.CooldownSeconds <= 0 {
		return
	}
	s.motionCooldown[window.Key] = time.Now().Add(time.Duration(window.CooldownSeconds) * time.Second)
}

func (s *HikvisionAlarmBridgeService) persistMotionAggregate(window *motionAggregateWindow) error {
	if window == nil || window.Count <= 0 {
		return nil
	}
	var alarmToPush *entity.AlarmRecord
	var pushChannels []string
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		payload := map[string]any{
			"capabilityCode":  "motion_detect",
			"eventType":       "motion_detect",
			"eventTime":       window.FirstEventTime.Format(time.RFC3339),
			"firstEventTime":  window.FirstEventTime.Format(time.RFC3339),
			"lastEventTime":   window.LastEventTime.Format(time.RFC3339),
			"occurrenceCount": window.Count,
			"sourceType":      window.SourceType,
			"sourceId":        window.SourceID,
			"cameraId":        window.CameraID,
			"recorderId":      window.RecorderID,
			"channelId":       window.ChannelID,
			"factoryId":       window.FactoryID,
			"zoneId":          window.ZoneID,
			"channelNo":       window.ChannelNo,
			"rawChannelNo":    window.RawChannelNo,
			"sourceIp":        window.DeviceIP,
			"command":         window.Command,
			"confidence":      1,
			"imageUrl":        nullableStringValue(window.SnapshotURL),
		}
		rawEventID := fmt.Sprintf("hikvision-motion:%s:%d:%d:%s", window.SourceType, window.SourceID, window.ChannelNo, uuid.NewString()[:12])
		headers := map[string]string{"x-hikvision-bridge": "sdk-callback", "x-aggregate-mode": "memory-window"}

		rawEvent := entity.SmartRawEvent{
			ProviderID:     window.ProviderID,
			CapabilityID:   uintPtr(window.CapabilityID),
			BindingID:      uintPtr(window.BindingID),
			SourceType:     window.SourceType,
			SourceID:       uintPtr(window.SourceID),
			SourceEventID:  rawEventID,
			EventNo:        buildSmartEventCode("RAW"),
			EventTime:      window.FirstEventTime,
			HeadersJSON:    marshalJSON(headers),
			RawPayloadJSON: marshalJSON(payload),
			ParseStatus:    "success",
		}
		if err := tx.Create(&rawEvent).Error; err != nil {
			return err
		}

		smartEvent := entity.SmartEvent{
			RawEventID:            uintPtr(rawEvent.ID),
			BindingID:             uintPtr(window.BindingID),
			ProviderID:            window.ProviderID,
			CapabilityID:          uintPtr(window.CapabilityID),
			EventCode:             buildSmartEventCode("SE"),
			EventType:             "motion_detect",
			EventLevel:            window.EventLevel,
			SourceStage:           "raw",
			EventTime:             window.FirstEventTime,
			CameraID:              window.CameraID,
			RecorderID:            window.RecorderID,
			ChannelID:             window.ChannelID,
			FactoryID:             window.FactoryID,
			ZoneID:                window.ZoneID,
			ImageURL:              window.SnapshotURL,
			Confidence:            floatPtr(1),
			DedupKey:              window.DedupKey,
			NormalizedPayloadJSON: rawEvent.RawPayloadJSON,
			Status:                "stored",
		}
		smartEvent, err := s.createOrMergeSmartEvent(tx, smartEvent)
		if err != nil {
			return err
		}

		if window.Rule != nil && window.Rule.AlarmEnabled && window.Rule.GenerateAlarmDirectly {
			smartEvent.Status = "alarm_generated"
			smartEvent.EventLevel = window.EventLevel
			if err := tx.Save(&smartEvent).Error; err != nil {
				return err
			}
			action, alarm, err := s.createOrMergeMotionAlarm(tx, smartEvent, rawEvent, window.FirstEventTime, window.LastEventTime, time.Now(), window.ChannelNo, window.DeviceIP, window.Rule)
			if err != nil {
				return err
			}
			if alarm != nil && action != "cooldown_suppressed" {
				alarm.OccurrenceCount += maxInt(window.Count-1, 0)
				alarm.LastEventTime = timePtr(window.LastEventTime)
				alarm.RecordEndTime = timePtr(window.LastEventTime.Add(90 * time.Second))
				if err := tx.Save(alarm).Error; err != nil {
					return err
				}
				if window.Rule.PushEnabled && action == "created" {
					alarmCopy := *alarm
					alarmToPush = &alarmCopy
					pushChannels = decodeJSONStringSlice(window.Rule.PushChannelsJSON)
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if alarmToPush != nil {
		dispatchAlarmPushes(s.repo.DB(), s.cfg, s.logger, *alarmToPush, pushChannels, "auto")
	}
	return nil
}

func (s *HikvisionAlarmBridgeService) persistMotionEvent(
	session *hikvisionBridgeSession,
	deviceIP string,
	rawChannelNo int,
	channelNo int,
	command int,
	alarmImage []byte,
	alarmImageType byte,
) error {
	return s.persistMotionEventForProvider("hikvision-sdk", session, deviceIP, rawChannelNo, channelNo, command, nil, alarmImage, alarmImageType)
}

func (s *HikvisionAlarmBridgeService) persistMotionEventForProvider(
	providerCode string,
	session *hikvisionBridgeSession,
	deviceIP string,
	rawChannelNo int,
	channelNo int,
	command int,
	callbackEventTime *time.Time,
	_ []byte,
	_ byte,
) error {
	receiveTime := time.Now()
	eventTime := receiveTime
	if callbackEventTime != nil && !callbackEventTime.IsZero() {
		eventTime = callbackEventTime.Local()
	}
	unlockPersist := s.lockAlarmDedupKey(fmt.Sprintf("motion-persist:%s:%d", session.SessionKey, channelNo))
	defer unlockPersist()
	var alarmToPush *entity.AlarmRecord
	var pushChannels []string
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var (
			camera   *entity.CameraDevice
			recorder *entity.RecorderDevice
			channel  *entity.RecorderChannel
		)

		if session.DeviceType == "recorder" {
			var recorderValue entity.RecorderDevice
			if err := tx.First(&recorderValue, session.DeviceID).Error; err != nil {
				return err
			}
			recorder = &recorderValue
			channelValue, err := s.ensureRecorderChannel(tx, recorderValue, channelNo)
			if err != nil {
				return err
			}
			channel = &channelValue
			if channel.CameraID != nil {
				var cameraValue entity.CameraDevice
				if err := tx.First(&cameraValue, *channel.CameraID).Error; err == nil {
					camera = &cameraValue
				}
			}
		} else {
			var cameraValue entity.CameraDevice
			if err := tx.First(&cameraValue, session.DeviceID).Error; err != nil {
				return err
			}
			camera = &cameraValue
			var channelValue entity.RecorderChannel
			if err := tx.Where("camera_id = ?", cameraValue.ID).Order("id ASC").First(&channelValue).Error; err == nil {
				channel = &channelValue
				var recorderValue entity.RecorderDevice
				if err := tx.First(&recorderValue, channelValue.RecorderID).Error; err == nil {
					recorder = &recorderValue
				}
			}
		}

		provider, capability, binding, rule, err := s.matchMotionBindingForProvider(tx, providerCode, camera, recorder, channel)
		if err != nil {
			return err
		}
		if provider == nil || capability == nil || binding == nil {
			return errHikvisionMotionBindingNotMatched
		}
		snapshotURL := ""
		if shouldCaptureMotionSnapshot(rule, entity.SmartEvent{}) {
			snapshotURL = s.captureMotionSnapshot(session, camera, recorder, channel, eventTime, channelNo)
		}
		payload := map[string]any{
			"capabilityCode": "motion_detect",
			"eventType":      "motion_detect",
			"eventTime":      eventTime.Format(time.RFC3339),
			"receiveTime":    receiveTime.Format(time.RFC3339),
			"sourceType":     binding.SourceType,
			"sourceId":       binding.SourceID,
			"cameraId":       nullableUintValue(camera),
			"recorderId":     nullableUintValue(recorder),
			"channelId":      nullableUintValue(channel),
			"factoryId":      firstFactoryID(camera, recorder, channel),
			"zoneId":         firstZoneID(camera, channel),
			"channelNo":      channelNo,
			"rawChannelNo":   rawChannelNo,
			"sourceIp":       deviceIP,
			"command":        command,
			"confidence":     1,
			"imageUrl":       nullableStringValue(snapshotURL),
		}
		if snapshotURL != "" {
			payload["imageSource"] = "snapshot-api"
		}
		rawEventID := fmt.Sprintf("hikvision-motion:%s:%d:%d:%s", binding.SourceType, binding.SourceID, channelNo, uuid.NewString()[:12])
		headers := map[string]string{"x-hikvision-bridge": providerCode}

		rawEvent := entity.SmartRawEvent{
			ProviderID:     provider.ID,
			CapabilityID:   uintPtr(capability.ID),
			BindingID:      uintPtr(binding.ID),
			SourceType:     binding.SourceType,
			SourceID:       uintPtr(binding.SourceID),
			SourceEventID:  rawEventID,
			EventNo:        buildSmartEventCode("RAW"),
			EventTime:      eventTime,
			HeadersJSON:    marshalJSON(headers),
			RawPayloadJSON: marshalJSON(payload),
			ParseStatus:    "success",
		}
		if err := tx.Create(&rawEvent).Error; err != nil {
			return err
		}

		eventLevel := "medium"
		if rule != nil && strings.TrimSpace(rule.AlarmLevel) != "" {
			eventLevel = rule.AlarmLevel
		}
		smartEvent := entity.SmartEvent{
			RawEventID:            uintPtr(rawEvent.ID),
			BindingID:             uintPtr(binding.ID),
			ProviderID:            provider.ID,
			CapabilityID:          uintPtr(capability.ID),
			EventCode:             buildSmartEventCode("SE"),
			EventType:             "motion_detect",
			EventLevel:            eventLevel,
			SourceStage:           "raw",
			EventTime:             eventTime,
			CameraID:              nullableEntityID(camera),
			RecorderID:            nullableEntityID(recorder),
			ChannelID:             nullableEntityID(channel),
			FactoryID:             firstFactoryID(camera, recorder, channel),
			ZoneID:                firstZoneID(camera, channel),
			ImageURL:              snapshotURL,
			Confidence:            floatPtr(1),
			DedupKey:              buildSmartDedupKey(binding.SourceType, binding.SourceID, channelNo, firstZoneID(camera, channel)),
			NormalizedPayloadJSON: rawEvent.RawPayloadJSON,
			Status:                "stored",
		}
		if err := tx.Create(&smartEvent).Error; err != nil {
			return err
		}

		if rule != nil && rule.AlarmEnabled && rule.GenerateAlarmDirectly {
			smartEvent.Status = "alarm_generated"
			smartEvent.EventLevel = eventLevel
			if err := tx.Save(&smartEvent).Error; err != nil {
				return err
			}
			action, alarm, err := s.createOrMergeMotionAlarm(tx, smartEvent, rawEvent, eventTime, eventTime, receiveTime, channelNo, deviceIP, rule)
			if err != nil {
				return err
			}
			if alarm != nil && rule.PushEnabled && action == "created" {
				alarmCopy := *alarm
				alarmToPush = &alarmCopy
				pushChannels = decodeJSONStringSlice(rule.PushChannelsJSON)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if alarmToPush != nil {
		dispatchAlarmPushes(s.repo.DB(), s.cfg, s.logger, *alarmToPush, pushChannels, "auto")
	}
	return nil
}

func (s *HikvisionAlarmBridgeService) createOrMergeSmartEvent(tx *gorm.DB, smartEvent entity.SmartEvent) (entity.SmartEvent, error) {
	unlockDedup := s.lockAlarmDedupKey("smart-event:" + smartEvent.DedupKey)
	defer unlockDedup()

	var existing entity.SmartEvent
	err := tx.Where(
		"dedup_key = ? AND event_time >= ? AND event_time <= ?",
		smartEvent.DedupKey,
		smartEvent.EventTime.Add(-smartEventMergeWindow),
		smartEvent.EventTime.Add(smartEventMergeWindow),
	).Order("event_time DESC, id DESC").First(&existing).Error
	if err == nil {
		existing.RawEventID = smartEvent.RawEventID
		existing.BindingID = smartEvent.BindingID
		existing.ProviderID = smartEvent.ProviderID
		existing.CapabilityID = smartEvent.CapabilityID
		existing.EventType = smartEvent.EventType
		existing.EventLevel = smartEvent.EventLevel
		existing.SourceStage = smartEvent.SourceStage
		if smartEvent.EventTime.After(existing.EventTime) {
			existing.EventTime = smartEvent.EventTime
		}
		existing.CameraID = smartEvent.CameraID
		existing.RecorderID = smartEvent.RecorderID
		existing.ChannelID = smartEvent.ChannelID
		existing.FactoryID = smartEvent.FactoryID
		existing.ZoneID = smartEvent.ZoneID
		if strings.TrimSpace(existing.ImageURL) == "" && strings.TrimSpace(smartEvent.ImageURL) != "" {
			existing.ImageURL = smartEvent.ImageURL
		}
		if strings.TrimSpace(smartEvent.VideoURL) != "" {
			existing.VideoURL = smartEvent.VideoURL
		}
		existing.Confidence = smartEvent.Confidence
		existing.NormalizedPayloadJSON = smartEvent.NormalizedPayloadJSON
		if err := tx.Save(&existing).Error; err != nil {
			return entity.SmartEvent{}, err
		}
		return existing, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return entity.SmartEvent{}, err
	}
	if err := tx.Create(&smartEvent).Error; err != nil {
		return entity.SmartEvent{}, err
	}
	return smartEvent, nil
}

func (s *HikvisionAlarmBridgeService) ensureRecorderChannel(tx *gorm.DB, recorder entity.RecorderDevice, channelNo int) (entity.RecorderChannel, error) {
	var channel entity.RecorderChannel
	err := tx.Where("recorder_id = ? AND channel_no = ?", recorder.ID, channelNo).First(&channel).Error
	if err == nil {
		return channel, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return entity.RecorderChannel{}, err
	}

	channel = entity.RecorderChannel{
		RecorderID:      recorder.ID,
		ChannelNo:       channelNo,
		Name:            fmt.Sprintf("%s-通道%02d", recorder.Name, channelNo),
		FactoryID:       recorder.FactoryID,
		Enabled:         true,
		SupportPlayback: true,
		Status:          recorder.Status,
	}
	if err := tx.Create(&channel).Error; err != nil {
		return entity.RecorderChannel{}, err
	}
	if recorder.ChannelCount < channelNo {
		recorder.ChannelCount = channelNo
		if err := tx.Model(&entity.RecorderDevice{}).Where("id = ?", recorder.ID).Update("channel_count", recorder.ChannelCount).Error; err != nil {
			return entity.RecorderChannel{}, err
		}
	}
	return channel, nil
}

func (s *HikvisionAlarmBridgeService) matchMotionBinding(
	tx *gorm.DB,
	camera *entity.CameraDevice,
	recorder *entity.RecorderDevice,
	channel *entity.RecorderChannel,
) (*entity.SmartInterfaceProvider, *entity.SmartInterfaceCapability, *entity.SmartDeviceBinding, *entity.SmartBindingRule, error) {
	return s.matchMotionBindingForProvider(tx, "hikvision-sdk", camera, recorder, channel)
}

func (s *HikvisionAlarmBridgeService) matchMotionBindingForProvider(
	tx *gorm.DB,
	providerCode string,
	camera *entity.CameraDevice,
	recorder *entity.RecorderDevice,
	channel *entity.RecorderChannel,
) (*entity.SmartInterfaceProvider, *entity.SmartInterfaceCapability, *entity.SmartDeviceBinding, *entity.SmartBindingRule, error) {
	provider := &entity.SmartInterfaceProvider{}
	if err := tx.Where("provider_code = ? AND enabled = ?", providerCode, true).First(provider).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil, nil, nil
		}
		return nil, nil, nil, nil, err
	}
	capability := &entity.SmartInterfaceCapability{}
	if err := tx.Where("capability_code = ? AND enabled = ?", "motion_detect", true).First(capability).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, nil, nil, nil
		}
		return nil, nil, nil, nil, err
	}

	candidates := []struct {
		sourceType string
		sourceID   uint
	}{
		{sourceType: "channel", sourceID: entityID(channel)},
		{sourceType: "camera", sourceID: entityID(camera)},
		{sourceType: "recorder", sourceID: entityID(recorder)},
	}
	findBinding := func(providerID uint) (*entity.SmartDeviceBinding, *entity.SmartBindingRule, error) {
		for _, candidate := range candidates {
			if candidate.sourceID == 0 {
				continue
			}
			binding := &entity.SmartDeviceBinding{}
			err := tx.Where(
				"provider_id = ? AND capability_id = ? AND source_type = ? AND source_id = ? AND enabled = ?",
				providerID, capability.ID, candidate.sourceType, candidate.sourceID, true,
			).Order("priority DESC, id ASC").First(binding).Error
			if err == gorm.ErrRecordNotFound {
				continue
			}
			if err != nil {
				return nil, nil, err
			}
			var rule entity.SmartBindingRule
			err = tx.Where("binding_id = ? AND enabled = ?", binding.ID, true).
				Order("generate_alarm_directly DESC, alarm_enabled DESC, id ASC").
				First(&rule).Error
			if err == gorm.ErrRecordNotFound {
				return binding, nil, nil
			}
			if err != nil {
				return nil, nil, err
			}
			return binding, &rule, nil
		}
		return nil, nil, nil
	}
	binding, rule, err := findBinding(provider.ID)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if binding != nil {
		return provider, capability, binding, rule, nil
	}
	return provider, capability, nil, nil, nil
}

func (s *HikvisionAlarmBridgeService) createOrMergeMotionAlarm(
	tx *gorm.DB,
	smartEvent entity.SmartEvent,
	rawEvent entity.SmartRawEvent,
	eventTime time.Time,
	lastEventTime time.Time,
	receiveTime time.Time,
	channelNo int,
	deviceIP string,
	rule *entity.SmartBindingRule,
) (string, *entity.AlarmRecord, error) {
	dedupWindow := 60
	if rule != nil && rule.DedupWindowSeconds > 0 {
		dedupWindow = rule.DedupWindowSeconds
	}
	cooldownWindow := 0
	if rule != nil && rule.CooldownSeconds > 0 {
		cooldownWindow = rule.CooldownSeconds
	}
	message := fmt.Sprintf("海康移动侦测报警，通道 %d，设备 %s", channelNo, firstNonEmpty(deviceIP, "未知设备"))
	dedupKey := buildSmartDedupKey("alarm", firstUintValue(smartEvent.CameraID, smartEvent.ChannelID, smartEvent.RecorderID), channelNo, smartEvent.ZoneID)
	unlockDedup := s.lockAlarmDedupKey(dedupKey)
	defer unlockDedup()

	var existing entity.AlarmRecord
	if dedupWindow > 0 {
		cutoff := eventTime.Add(-time.Duration(dedupWindow) * time.Second)
		err := tx.Where(
			"dedup_key = ? AND alarm_time <= ? AND alarm_time >= ?",
			dedupKey,
			eventTime,
			cutoff,
		).Order("alarm_time DESC, id DESC").First(&existing).Error
		if err == nil {
			existing.SmartEventID = uintPtr(smartEvent.ID)
			existing.RawEventID = uintPtr(rawEvent.ID)
			existing.AlarmLevel = smartEvent.EventLevel
			existing.SourceStage = "raw"
			existing.AlarmOrigin = "device"
			existing.Message = message
			if strings.TrimSpace(existing.ImageURL) == "" && strings.TrimSpace(smartEvent.ImageURL) != "" {
				existing.ImageURL = smartEvent.ImageURL
			}
			existing.OccurrenceCount++
			existing.LastEventTime = timePtr(lastEventTime)
			existing.RecordEndTime = timePtr(lastEventTime.Add(90 * time.Second))
			if err := tx.Save(&existing).Error; err != nil {
				return "", nil, err
			}
			return "deduped", &existing, nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return "", nil, err
		}
	}

	if cooldownWindow > 0 {
		var cooldown entity.AlarmRecord
		cutoff := eventTime.Add(-time.Duration(cooldownWindow) * time.Second)
		err := tx.Where("dedup_key = ? AND last_event_time >= ?", dedupKey, cutoff).Order("last_event_time DESC, id DESC").First(&cooldown).Error
		if err == nil {
			return "cooldown_suppressed", &cooldown, nil
		}
		if err != nil && err != gorm.ErrRecordNotFound {
			return "", nil, err
		}
	}

	alarm := entity.AlarmRecord{
		AlarmNo:         buildAlarmNo(),
		SmartEventID:    uintPtr(smartEvent.ID),
		RawEventID:      uintPtr(rawEvent.ID),
		AlarmType:       "移动侦测",
		AlarmLevel:      smartEvent.EventLevel,
		AlarmTime:       eventTime,
		Status:          "pending",
		SourceStage:     "raw",
		AlarmOrigin:     "device",
		CameraID:        smartEvent.CameraID,
		RecorderID:      smartEvent.RecorderID,
		ChannelID:       smartEvent.ChannelID,
		FactoryID:       smartEvent.FactoryID,
		ZoneID:          smartEvent.ZoneID,
		Message:         message,
		ImageURL:        smartEvent.ImageURL,
		VideoURL:        smartEvent.VideoURL,
		RecordStartTime: timePtr(eventTime.Add(-30 * time.Second)),
		RecordEndTime:   timePtr(lastEventTime.Add(90 * time.Second)),
		PushRecordsJSON: "[]",
		DedupKey:        dedupKey,
		OccurrenceCount: 1,
		LastEventTime:   timePtr(lastEventTime),
	}
	if err := tx.Create(&alarm).Error; err != nil {
		return "", nil, err
	}
	processLog := entity.AlarmProcessLog{
		AlarmID:      alarm.ID,
		Action:       "create",
		FromStatus:   "",
		ToStatus:     "pending",
		OperatorName: "system",
		Remark:       fmt.Sprintf("海康移动侦测报警自动生成，通道 %d", channelNo),
		CreatedAt:    receiveTime,
	}
	if err := tx.Create(&processLog).Error; err != nil {
		return "", nil, err
	}
	return "created", &alarm, nil
}

func (s *HikvisionAlarmBridgeService) deviceSecretKey() string {
	if strings.TrimSpace(s.cfg.DeviceSecretKey) != "" {
		return s.cfg.DeviceSecretKey
	}
	return s.cfg.JWTSecretKey
}

func shouldCaptureMotionSnapshot(rule *entity.SmartBindingRule, smartEvent entity.SmartEvent) bool {
	if rule != nil && !rule.SnapshotEnabled {
		return false
	}
	return strings.TrimSpace(smartEvent.ImageURL) == ""
}

func (s *HikvisionAlarmBridgeService) captureSnapshot(session *hikvisionBridgeSession, entityID uint, channelNo int, eventTime time.Time) string {
	if s.sdk == nil || session == nil || entityID == 0 {
		return ""
	}
	snapshotPath := s.buildSnapshotPath(entityID, eventTime)
	if _, err := s.sdk.CaptureJPEG(session.UserID, channelNo, snapshotPath, session.DeviceInfo); err != nil {
		s.logger.Warn("hikvision motion snapshot capture failed",
			zap.String("sessionKey", session.SessionKey),
			zap.Uint("entityId", entityID),
			zap.Int("channelNo", channelNo),
			zap.Error(err),
		)
		return ""
	}
	return s.buildSnapshotURL(snapshotPath)
}

func (s *HikvisionAlarmBridgeService) captureMotionSnapshot(
	session *hikvisionBridgeSession,
	camera *entity.CameraDevice,
	recorder *entity.RecorderDevice,
	channel *entity.RecorderChannel,
	eventTime time.Time,
	channelNo int,
) string {
	snapshotEntityID := resolveSnapshotEntityID(camera, channel, recorder, session.DeviceID)
	if snapshotEntityID == 0 {
		return ""
	}
	if session != nil && session.UserID > 0 {
		return s.captureSnapshot(session, snapshotEntityID, channelNo, eventTime)
	}

	target, ok, err := s.snapshotLoginTarget(session, camera, recorder, channel)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("resolve hikvision motion snapshot target failed",
				zap.String("sessionKey", session.SessionKey),
				zap.Error(err),
			)
		}
		return ""
	}
	if !ok {
		return ""
	}

	s.mu.Lock()
	if err := s.ensureSDKLocked(); err != nil {
		s.mu.Unlock()
		if s.logger != nil {
			s.logger.Warn("hikvision snapshot sdk init failed",
				zap.String("sessionKey", session.SessionKey),
				zap.Error(err),
			)
		}
		return ""
	}
	sdk := s.sdk
	s.mu.Unlock()

	var (
		userID     int32
		deviceInfo hikvision.DeviceInfo
	)
	if target.DeviceType == "recorder" {
		userID, deviceInfo, err = sdk.LoginRecorder(target.DeviceIP, target.SDKPort, target.Username, target.Password)
	} else {
		userID, deviceInfo, err = sdk.LoginCamera(target.DeviceIP, target.SDKPort, target.Username, target.Password)
	}
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("hikvision snapshot login failed",
				zap.String("sessionKey", session.SessionKey),
				zap.String("deviceType", target.DeviceType),
				zap.Uint("deviceId", target.DeviceID),
				zap.Error(err),
			)
		}
		return ""
	}
	defer func() {
		if err := sdk.Logout(userID); err != nil && s.logger != nil {
			s.logger.Warn("hikvision snapshot logout failed",
				zap.String("sessionKey", session.SessionKey),
				zap.Error(err),
			)
		}
	}()

	snapshotSession := &hikvisionBridgeSession{
		SessionKey: session.SessionKey,
		DeviceType: target.DeviceType,
		DeviceID:   target.DeviceID,
		DeviceName: target.DeviceName,
		DeviceIP:   target.DeviceIP,
		UserID:     userID,
		DeviceInfo: deviceInfo,
	}
	return s.captureSnapshot(snapshotSession, snapshotEntityID, channelNo, eventTime)
}

func (s *HikvisionAlarmBridgeService) snapshotLoginTarget(
	session *hikvisionBridgeSession,
	camera *entity.CameraDevice,
	recorder *entity.RecorderDevice,
	channel *entity.RecorderChannel,
) (hikvisionBridgeTarget, bool, error) {
	if recorder != nil {
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
		if err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return hikvisionBridgeTarget{
			SessionKey: fmt.Sprintf("snapshot-recorder:%d", recorder.ID),
			DeviceType: "recorder",
			DeviceID:   recorder.ID,
			DeviceName: recorder.Name,
			DeviceIP:   recorder.IP,
			SDKPort:    recorder.SDKPort,
			Username:   recorder.Username,
			Password:   password,
		}, true, nil
	}
	if camera != nil {
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), camera.PasswordEncrypted)
		if err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return hikvisionBridgeTarget{
			SessionKey: fmt.Sprintf("snapshot-camera:%d", camera.ID),
			DeviceType: "camera",
			DeviceID:   camera.ID,
			DeviceName: camera.Name,
			DeviceIP:   camera.IP,
			SDKPort:    camera.SDKPort,
			Username:   camera.Username,
			Password:   password,
		}, true, nil
	}
	if session != nil && session.DeviceType == "recorder" && session.DeviceID != 0 {
		var recorderValue entity.RecorderDevice
		if err := s.repo.DB().First(&recorderValue, session.DeviceID).Error; err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return s.snapshotLoginTarget(session, camera, &recorderValue, channel)
	}
	if channel != nil && channel.RecorderID != 0 {
		var recorderValue entity.RecorderDevice
		if err := s.repo.DB().First(&recorderValue, channel.RecorderID).Error; err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return s.snapshotLoginTarget(session, camera, &recorderValue, channel)
	}
	return hikvisionBridgeTarget{}, false, nil
}

func (s *HikvisionAlarmBridgeService) saveAlarmPacketImage(session *hikvisionBridgeSession, entityID uint, eventTime time.Time, image []byte, imageType byte) string {
	if session == nil || entityID == 0 || len(image) == 0 {
		return ""
	}
	if imageType != 0 && imageType != 1 {
		return ""
	}
	if !looksLikeJPEG(image) {
		return ""
	}
	snapshotPath := s.buildSnapshotPath(entityID, eventTime)
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		s.logger.Warn("create hikvision alarm packet image dir failed",
			zap.String("sessionKey", session.SessionKey),
			zap.Uint("entityId", entityID),
			zap.Error(err),
		)
		return ""
	}
	if err := os.WriteFile(snapshotPath, image, 0o644); err != nil {
		s.logger.Warn("write hikvision alarm packet image failed",
			zap.String("sessionKey", session.SessionKey),
			zap.Uint("entityId", entityID),
			zap.Error(err),
		)
		return ""
	}
	return s.buildSnapshotURL(snapshotPath)
}

func looksLikeJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
}

func (s *HikvisionAlarmBridgeService) buildSnapshotPath(entityID uint, eventTime time.Time) string {
	baseDir := filepath.Join(s.cfg.MediaRootDir, "hikvision-smart-alarm", fmt.Sprintf("entity-%d", entityID))
	filename := fmt.Sprintf("motion_%s_%06d.jpg", eventTime.Format("20060102_150405"), eventTime.Nanosecond()/1000)
	return filepath.Join(baseDir, filename)
}

func (s *HikvisionAlarmBridgeService) buildSnapshotURL(snapshotPath string) string {
	mediaRoot, err := filepath.Abs(s.cfg.MediaRootDir)
	if err != nil {
		mediaRoot = s.cfg.MediaRootDir
	}
	absoluteSnapshotPath, err := filepath.Abs(snapshotPath)
	if err != nil {
		absoluteSnapshotPath = snapshotPath
	}
	relativePath, err := filepath.Rel(mediaRoot, absoluteSnapshotPath)
	if err != nil {
		return ""
	}
	return strings.TrimRight(s.cfg.BackendPublicBaseURL, "/") + s.cfg.MediaMountPath + "/" + filepath.ToSlash(relativePath)
}

func buildSmartEventCode(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, strings.ToUpper(uuid.NewString()))
}

func buildAlarmNo() string {
	return fmt.Sprintf("ALM%s%s", time.Now().Format("20060102150405"), strings.ToUpper(uuid.NewString()[:8]))
}

func buildMotionAggregateKey(sessionKey string, channelNo int) string {
	return fmt.Sprintf("%s:%d", sessionKey, channelNo)
}

func buildSmartDedupKey(sourceType string, sourceID uint, channelNo int, zoneID *uint) string {
	zone := uint(0)
	if zoneID != nil {
		zone = *zoneID
	}
	return fmt.Sprintf("hikvision-motion:%s:%d:%d:%d", sourceType, sourceID, channelNo, zone)
}

func (s *HikvisionAlarmBridgeService) lockAlarmDedupKey(dedupKey string) func() {
	s.alarmDedupMu.Lock()
	locker, ok := s.alarmDedupLocks[dedupKey]
	if !ok {
		locker = &sync.Mutex{}
		s.alarmDedupLocks[dedupKey] = locker
	}
	s.alarmDedupMu.Unlock()

	locker.Lock()
	return func() {
		locker.Unlock()
	}
}

func marshalJSON(value any) string {
	if value == nil {
		return "{}"
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func entityID(value interface{}) uint {
	switch item := value.(type) {
	case *entity.CameraDevice:
		if item == nil {
			return 0
		}
		return item.ID
	case *entity.RecorderDevice:
		if item == nil {
			return 0
		}
		return item.ID
	case *entity.RecorderChannel:
		if item == nil {
			return 0
		}
		return item.ID
	default:
		return 0
	}
}

func nullableEntityID(value interface{}) *uint {
	switch item := value.(type) {
	case *entity.CameraDevice:
		if item == nil {
			return nil
		}
		return uintPtr(item.ID)
	case *entity.RecorderDevice:
		if item == nil {
			return nil
		}
		return uintPtr(item.ID)
	case *entity.RecorderChannel:
		if item == nil {
			return nil
		}
		return uintPtr(item.ID)
	default:
		return nil
	}
}

func resolveSnapshotEntityID(camera *entity.CameraDevice, channel *entity.RecorderChannel, recorder *entity.RecorderDevice, fallback uint) uint {
	if camera != nil {
		return camera.ID
	}
	if channel != nil {
		return channel.ID
	}
	if recorder != nil {
		return recorder.ID
	}
	return fallback
}

func nullableStringValue(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func firstFactoryID(camera *entity.CameraDevice, recorder *entity.RecorderDevice, channel *entity.RecorderChannel) *uint {
	if camera != nil {
		return uintPtr(camera.FactoryID)
	}
	if channel != nil {
		return uintPtr(channel.FactoryID)
	}
	if recorder != nil {
		return uintPtr(recorder.FactoryID)
	}
	return nil
}

func firstZoneID(camera *entity.CameraDevice, channel *entity.RecorderChannel) *uint {
	if camera != nil {
		return uintPtr(camera.ZoneID)
	}
	if channel != nil && channel.ZoneID != nil {
		return channel.ZoneID
	}
	return nil
}

func nullableUintValue(value interface{}) *uint {
	switch item := value.(type) {
	case *entity.CameraDevice:
		if item == nil {
			return nil
		}
		return uintPtr(item.ID)
	case *entity.RecorderDevice:
		if item == nil {
			return nil
		}
		return uintPtr(item.ID)
	case *entity.RecorderChannel:
		if item == nil {
			return nil
		}
		return uintPtr(item.ID)
	default:
		return nil
	}
}

func firstUintValue(values ...*uint) uint {
	for _, value := range values {
		if value != nil {
			return *value
		}
	}
	return 0
}

func ruleID(rule *entity.SmartBindingRule) *uint {
	if rule == nil {
		return nil
	}
	return uintPtr(rule.ID)
}

func uintPtr(value uint) *uint {
	copyValue := value
	return &copyValue
}

func floatPtr(value float64) *float64 {
	copyValue := value
	return &copyValue
}

func timePtr(value time.Time) *time.Time {
	copyValue := value
	return &copyValue
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func sessionKeyValue(session *hikvisionBridgeSession) string {
	if session == nil {
		return ""
	}
	return session.SessionKey
}
