package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/integration/hikvision"
	"secmgmt_go/internal/util"
)

func (s *PlatformService) GetLiveVideo(sourceType string, id uint, streamType, streamProfile string, accessScope *AccessScope) (map[string]any, error) {
	switch sourceType {
	case "camera":
		if _, err := s.ensureCameraAccessible(accessScope, id); err != nil {
			return nil, err
		}
	case "channel":
		if _, err := s.ensureChannelAccessible(accessScope, id); err != nil {
			return nil, err
		}
	}
	playURL := fmt.Sprintf("%s/mock/live/%s/%d.m3u8?profile=%s", strings.TrimRight(s.cfg.BackendPublicBaseURL, "/"), sourceType, id, streamProfile)
	return map[string]any{
		"cameraId":          chooseID(sourceType == "camera", id),
		"channelId":         chooseID(sourceType == "channel", id),
		"streamType":        defaultString(streamType, "hik-sdk"),
		"connectionMode":    "hik-sdk",
		"playUrl":           playURL,
		"expiresIn":         300,
		"isMock":            true,
		"playableInBrowser": false,
		"diagnosticMessage": "当前为后端开发阶段的模拟播放地址",
		"sourceRtsp":        fmt.Sprintf("rtsp://mock/%s/%d", sourceType, id),
	}, nil
}

func (s *PlatformService) StopLiveVideo(sourceType string, id uint, accessScope *AccessScope) (map[string]any, error) {
	switch sourceType {
	case "camera":
		if _, err := s.ensureCameraAccessible(accessScope, id); err != nil {
			return nil, err
		}
	case "channel":
		if _, err := s.ensureChannelAccessible(accessScope, id); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"cameraId":  chooseID(sourceType == "camera", id),
		"channelId": chooseID(sourceType == "channel", id),
		"stopped":   true,
		"message":   "已停止实时预览",
	}, nil
}

func (s *PlatformService) GetLiveWebControlConfig(sourceType string, id uint, streamProfile string, accessScope *AccessScope) (map[string]any, error) {
	if sourceType == "camera" {
		camera, err := s.ensureCameraAccessible(accessScope, id)
		if err != nil {
			return nil, err
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), camera.PasswordEncrypted)
		if err != nil {
			return nil, fmt.Errorf("resolve camera password: %w", err)
		}
		return map[string]any{
			"sourceType":    "camera",
			"cameraId":      camera.ID,
			"deviceName":    camera.Name,
			"host":          camera.IP,
			"port":          camera.HTTPPort,
			"protocol":      resolveDeviceProtocol(camera.HTTPPort),
			"username":      camera.Username,
			"password":      password,
			"channelNo":     1,
			"streamType":    mapStreamProfileToInt(streamProfile),
			"streamProfile": defaultString(streamProfile, "main"),
			"zeroChannel":   false,
			"useProxy":      true,
			"webSocketPort": nil,
			"rtspPort":      camera.RTSPPort,
			"supported":     true,
			"message":       "当前为海康客户端预览，默认通过前端同源代理转发，请确保已部署 WebSDK_noPlugin codebase 静态资源。",
		}, nil
	}
	channel, err := s.ensureChannelAccessible(accessScope, id)
	if err != nil {
		return nil, err
	}
	var recorder entity.RecorderDevice
	if err := s.db().First(&recorder, channel.RecorderID).Error; err != nil {
		return nil, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve recorder password: %w", err)
	}
	return map[string]any{
		"sourceType":    "channel",
		"cameraId":      channel.CameraID,
		"channelId":     channel.ID,
		"deviceName":    recorder.Name,
		"host":          recorder.IP,
		"port":          recorder.HTTPPort,
		"protocol":      resolveDeviceProtocol(recorder.HTTPPort),
		"username":      recorder.Username,
		"password":      password,
		"channelNo":     channel.ChannelNo,
		"streamType":    mapStreamProfileToInt(streamProfile),
		"streamProfile": defaultString(streamProfile, "main"),
		"zeroChannel":   false,
		"useProxy":      true,
		"webSocketPort": nil,
		"rtspPort":      nil,
		"supported":     true,
		"message":       "当前为海康客户端预览，默认通过前端同源代理转发，请确保已部署 WebSDK_noPlugin codebase 静态资源。",
	}, nil
}

func (s *PlatformService) CreateSnapshot(cameraID, channelID *uint, accessScope *AccessScope) (map[string]any, error) {
	if cameraID != nil {
		if _, err := s.ensureCameraAccessible(accessScope, *cameraID); err != nil {
			return nil, err
		}
	}
	if channelID != nil {
		if _, err := s.ensureChannelAccessible(accessScope, *channelID); err != nil {
			return nil, err
		}
	}
	snapshotURL := buildSnapshotDataURL(cameraID, channelID)
	return map[string]any{"cameraId": cameraID, "channelId": channelID, "snapshotUrl": snapshotURL, "expiresIn": 300}, nil
}

func (s *PlatformService) SearchPlaybackSegments(channelID uint, startAt, endAt *time.Time, accessScope *AccessScope) ([]map[string]any, error) {
	channel, err := s.ensureChannelAccessible(accessScope, channelID)
	var recorder entity.RecorderDevice
	var camera entity.CameraDevice
	if err != nil {
		return nil, err
	}
	_ = s.db().First(&recorder, channel.RecorderID).Error
	if channel.CameraID != nil {
		_ = s.db().First(&camera, *channel.CameraID).Error
	}
	start := time.Now().Add(-30 * time.Minute)
	end := time.Now()
	if startAt != nil {
		start = *startAt
	}
	if endAt != nil {
		end = *endAt
	}
	if !end.After(start) {
		end = start.Add(2 * time.Minute)
	}
	return []map[string]any{{
		"startTime":    start.Format(time.RFC3339),
		"endTime":      end.Format(time.RFC3339),
		"channelId":    channel.ID,
		"channelName":  channel.Name,
		"recorderId":   recorder.ID,
		"recorderName": recorder.Name,
		"cameraId":     channel.CameraID,
		"cameraName":   nullableString(camera.Name),
		"recordType":   "alarm",
		"available":    channel.SupportPlayback,
	}}, nil
}

func (s *PlatformService) GetPlaybackURL(channelID uint, streamType, streamProfile, playbackMode string, accessScope *AccessScope) (map[string]any, error) {
	if _, err := s.ensureChannelAccessible(accessScope, channelID); err != nil {
		return nil, err
	}
	start := time.Now().Add(-30 * time.Minute)
	end := time.Now()
	return map[string]any{
		"streamType":        defaultString(streamType, "hik-sdk"),
		"streamProfile":     defaultString(streamProfile, "main"),
		"playbackMode":      defaultString(playbackMode, "hik"),
		"playUrl":           fmt.Sprintf("%s/mock/playback/channel/%d.m3u8", strings.TrimRight(s.cfg.BackendPublicBaseURL, "/"), channelID),
		"startTime":         start.Format(time.RFC3339),
		"endTime":           end.Format(time.RFC3339),
		"expiresIn":         300,
		"isMock":            true,
		"playableInBrowser": false,
		"diagnosticMessage": "当前返回模拟回放地址",
		"sourceRtsp":        fmt.Sprintf("rtsp://mock/playback/%d", channelID),
	}, nil
}

func (s *PlatformService) DownloadPlaybackFile(channelID uint, startTime, endTime time.Time, alarmNo string, accessScope *AccessScope) (string, string, error) {
	if !endTime.After(startTime) {
		return "", "", fmt.Errorf("invalid playback time range")
	}

	channel, err := s.ensureChannelAccessible(accessScope, channelID)
	if err != nil {
		return "", "", err
	}

	var recorder entity.RecorderDevice
	if err := s.db().First(&recorder, channel.RecorderID).Error; err != nil {
		return "", "", err
	}

	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return "", "", fmt.Errorf("resolve recorder password: %w", err)
	}

	sdk, err := hikvision.NewSDK(s.cfg.HikvisionSDKPath)
	if err != nil {
		return "", "", fmt.Errorf("create hikvision sdk: %w", err)
	}
	defer func() {
		_ = sdk.Cleanup()
	}()

	userID, deviceInfo, err := sdk.LoginRecorder(recorder.IP, recorder.SDKPort, recorder.Username, password)
	if err != nil {
		return "", "", fmt.Errorf("login recorder: %w", err)
	}
	defer func() {
		_ = sdk.Logout(userID)
	}()

	fileName := buildPlaybackDownloadFilename(alarmNo, recorder.Name, channel.Name, startTime, endTime)
	outputDir := filepath.Join(s.cfg.MediaRootDir, "playback-downloads", time.Now().Format("20060102"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create playback output dir: %w", err)
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileName))
	if err := sdk.DownloadRecordByTime(userID, channel.ChannelNo, startTime, endTime, outputPath, deviceInfo); err != nil {
		return "", "", fmt.Errorf("download playback by time: %w", err)
	}
	return outputPath, fileName, nil
}

func (s *PlatformService) StopPlayback(channelID uint, accessScope *AccessScope) (map[string]any, error) {
	if _, err := s.ensureChannelAccessible(accessScope, channelID); err != nil {
		return nil, err
	}
	return map[string]any{"channelId": channelID, "stopped": true, "message": "已停止回放"}, nil
}
