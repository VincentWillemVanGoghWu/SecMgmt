package service

import (
	"bytes"
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/util"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *PlatformService) GetCamera(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListCameras(CameraListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == cameraID {
			return cameraDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) CreateCamera(payload CameraPayload, accessScope *AccessScope) (map[string]any, error) {
	if err := validateCameraTargetAccess(accessScope, payload.FactoryID, payload.ZoneID); err != nil {
		return nil, err
	}
	encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt camera password: %w", err)
	}
	item := entity.CameraDevice{
		DeviceCode:        payload.DeviceCode,
		Name:              payload.Name,
		IP:                payload.IP,
		SDKPort:           defaultInt(payload.SDKPort, 8000),
		HTTPPort:          defaultInt(payload.HTTPPort, 80),
		RTSPPort:          defaultInt(payload.RTSPPort, 554),
		Username:          payload.Username,
		PasswordEncrypted: encryptedPassword,
		FactoryID:         payload.FactoryID,
		ZoneID:            payload.ZoneID,
		InstallLocation:   valueOrEmpty(payload.InstallLocation),
		SupportAI:         payload.SupportAI,
		Status:            normalizedStatus(payload.Status, "offline"),
		Remark:            valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.GetCamera(item.ID, accessScope)
}

func (s *PlatformService) UpdateCamera(cameraID uint, payload CameraPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	if err := validateCameraTargetAccess(accessScope, payload.FactoryID, payload.ZoneID); err != nil {
		return nil, err
	}
	item.DeviceCode = payload.DeviceCode
	item.Name = payload.Name
	item.IP = payload.IP
	item.SDKPort = defaultInt(payload.SDKPort, item.SDKPort)
	item.HTTPPort = defaultInt(payload.HTTPPort, item.HTTPPort)
	item.RTSPPort = defaultInt(payload.RTSPPort, item.RTSPPort)
	item.Username = payload.Username
	if payload.Password != "" {
		encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt camera password: %w", err)
		}
		item.PasswordEncrypted = encryptedPassword
	}
	item.FactoryID = payload.FactoryID
	item.ZoneID = payload.ZoneID
	item.InstallLocation = valueOrEmpty(payload.InstallLocation)
	item.SupportAI = payload.SupportAI
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.GetCamera(cameraID, accessScope)
}

func (s *PlatformService) UpdateCameraStatus(cameraID uint, status string, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(status, item.Status)
	now := time.Now()
	item.LastOnlineAt = &now
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.GetCamera(cameraID, accessScope)
}

func (s *PlatformService) DeleteCamera(cameraID uint, accessScope *AccessScope) error {
	if _, err := s.ensureCameraAccessible(accessScope, cameraID); err != nil {
		return err
	}
	if err := s.db().Delete(&entity.CameraDevice{}, cameraID).Error; err != nil {
		return wrapDeviceDeleteError(err)
	}
	return nil
}

func (s *PlatformService) TestCameraConnection(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	status := probeDeviceTCPStatus(item.IP, item.HTTPPort, item.SDKPort, item.RTSPPort)
	if _, err := updateDeviceStatusFromTest(s.db(), "camera", item.ID, "camera_device", item.Status, status, "连接测试"); err != nil {
		return nil, err
	}
	success := status == "online"
	message := "设备连接测试失败"
	if success {
		message = "设备连接测试成功"
	}
	return map[string]any{
		"success": success,
		"status":  status,
		"message": message,
		"rtspUrl": fmt.Sprintf("rtsp://%s:%d/Streaming/Channels/101", item.IP, item.RTSPPort),
	}, nil
}

func (s *PlatformService) CheckCameraStatus(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	return s.TestCameraConnection(cameraID, accessScope)
}

func (s *PlatformService) ControlCameraPTZZoom(cameraID uint, action string, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), item.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve camera password: %w", err)
	}

	zoom := 0
	switch action {
	case "in":
		zoom = 60
	case "out":
		zoom = -60
	default:
		return nil, fmt.Errorf("unsupported PTZ zoom action %q", action)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	baseURL := fmt.Sprintf("%s://%s", resolveDeviceProtocol(item.HTTPPort), item.IP)
	if item.HTTPPort > 0 {
		baseURL = fmt.Sprintf("%s:%d", baseURL, item.HTTPPort)
	}
	targetURL := baseURL + "/ISAPI/PTZCtrl/channels/1/continuous"
	startPayload := buildHikPTZContinuousPayload(0, 0, zoom)
	stopPayload := buildHikPTZContinuousPayload(0, 0, 0)

	if err := hikHTTPPutXML(client, targetURL, item.Username, password, startPayload); err != nil {
		return nil, fmt.Errorf("start PTZ zoom: %w", err)
	}
	time.Sleep(350 * time.Millisecond)
	if err := hikHTTPPutXML(client, targetURL, item.Username, password, stopPayload); err != nil {
		return nil, fmt.Errorf("stop PTZ zoom: %w", err)
	}
	return map[string]any{
		"success": true,
		"message": "镜头动作已执行",
		"action":  action,
	}, nil
}

func (s *PlatformService) GetCameraBrowserLogin(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), item.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve camera password: %w", err)
	}
	return map[string]any{
		"ip":       item.IP,
		"port":     item.HTTPPort,
		"protocol": "http",
		"username": item.Username,
		"password": password,
		"loginUrl": fmt.Sprintf("http://%s:%d", item.IP, item.HTTPPort),
	}, nil
}

func (s *PlatformService) FetchCameraDeviceIdentity(payload CameraPayload) map[string]any {
	return map[string]any{
		"deviceName":     fmt.Sprintf("%s-%s", payload.Name, payload.DeviceCode),
		"deviceModel":    "Hikvision Camera",
		"deviceSerialNo": fmt.Sprintf("CAM-%s", strings.ToUpper(payload.DeviceCode)),
	}
}

func (s *PlatformService) GetCameraSDKConfig(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	return defaultCameraSDKConfig(*item), nil
}

func (s *PlatformService) UpdateCameraSDKSubConfig(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	return s.GetCameraSDKConfig(cameraID, accessScope)
}

func (s *PlatformService) GetRecorder(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListRecorders(RecorderListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == recorderID {
			return recorderDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) CreateRecorder(payload RecorderPayload, accessScope *AccessScope) (map[string]any, error) {
	if err := validateRecorderTargetAccess(accessScope, payload.FactoryID); err != nil {
		return nil, err
	}
	encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt recorder password: %w", err)
	}
	item := entity.RecorderDevice{
		DeviceCode:        payload.DeviceCode,
		Name:              payload.Name,
		IP:                payload.IP,
		SDKPort:           defaultInt(payload.SDKPort, 8000),
		HTTPPort:          defaultInt(payload.HTTPPort, 80),
		Username:          payload.Username,
		PasswordEncrypted: encryptedPassword,
		ChannelCount:      payload.ChannelCount,
		FactoryID:         payload.FactoryID,
		Status:            normalizedStatus(payload.Status, "offline"),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.GetRecorder(item.ID, accessScope)
}

func (s *PlatformService) UpdateRecorder(recorderID uint, payload RecorderPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureRecorderAccessible(accessScope, recorderID)
	if err != nil {
		return nil, err
	}
	if err := validateRecorderTargetAccess(accessScope, payload.FactoryID); err != nil {
		return nil, err
	}
	item.DeviceCode = payload.DeviceCode
	item.Name = payload.Name
	item.IP = payload.IP
	item.SDKPort = defaultInt(payload.SDKPort, item.SDKPort)
	item.HTTPPort = defaultInt(payload.HTTPPort, item.HTTPPort)
	item.Username = payload.Username
	if payload.Password != "" {
		encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt recorder password: %w", err)
		}
		item.PasswordEncrypted = encryptedPassword
	}
	item.ChannelCount = payload.ChannelCount
	item.FactoryID = payload.FactoryID
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return s.GetRecorder(recorderID, accessScope)
}

func (s *PlatformService) DeleteRecorder(recorderID uint, accessScope *AccessScope) error {
	if _, err := s.ensureRecorderAccessible(accessScope, recorderID); err != nil {
		return err
	}
	if err := s.db().Where("recorder_id = ?", recorderID).Delete(&entity.RecorderChannel{}).Error; err != nil {
		return wrapDeviceDeleteError(err)
	}
	if err := s.db().Delete(&entity.RecorderDevice{}, recorderID).Error; err != nil {
		return wrapDeviceDeleteError(err)
	}
	return nil
}

func (s *PlatformService) TestRecorderConnection(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureRecorderAccessible(accessScope, recorderID)
	if err != nil {
		return nil, err
	}
	status := probeDeviceTCPStatus(item.IP, item.HTTPPort, item.SDKPort)
	if _, err := updateDeviceStatusFromTest(s.db(), "recorder", item.ID, "recorder_device", item.Status, status, "连接测试"); err != nil {
		return nil, err
	}
	success := status == "online"
	message := "录像机连接测试失败"
	if success {
		message = "录像机连接测试成功"
	}
	return map[string]any{"success": success, "status": status, "message": message}, nil
}

func (s *PlatformService) CheckRecorderStatus(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	return s.TestRecorderConnection(recorderID, accessScope)
}

func (s *PlatformService) SyncRecorderChannels(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	recorder, err := s.ensureRecorderAccessible(accessScope, recorderID)
	if err != nil {
		return nil, err
	}
	deviceChannels, err := s.fetchRecorderChannels(*recorder)
	if err != nil {
		if recorder.ChannelCount <= 0 {
			return nil, err
		}
		if s.logger != nil {
			s.logger.Warn("fetch recorder channels failed, fallback to configured channel count",
				zap.Uint("recorderId", recorder.ID),
				zap.String("recorderIP", recorder.IP),
				zap.Error(err),
			)
		}
		deviceChannels = buildDefaultRecorderSyncChannels(recorder.ChannelCount)
	}
	if len(deviceChannels) == 0 {
		if recorder.ChannelCount <= 0 {
			return nil, fmt.Errorf("recorder returned empty channel list")
		}
		deviceChannels = buildDefaultRecorderSyncChannels(recorder.ChannelCount)
	}
	deviceChannels = completeRecorderSyncChannels(deviceChannels, recorder.ChannelCount)

	now := time.Now()
	if err := s.db().Transaction(func(tx *gorm.DB) error {
		var existing []entity.RecorderChannel
		if err := tx.Where("recorder_id = ?", recorderID).Find(&existing).Error; err != nil {
			return err
		}
		existingByNo := make(map[int]entity.RecorderChannel, len(existing))
		for _, channel := range existing {
			existingByNo[channel.ChannelNo] = channel
		}

		for _, deviceChannel := range deviceChannels {
			channel, exists := existingByNo[deviceChannel.ChannelNo]
			if exists {
				channel.Name = defaultString(deviceChannel.Name, channel.Name)
				channel.FactoryID = recorder.FactoryID
				channel.Enabled = deviceChannel.Enabled
				channel.Status = normalizedStatus(deviceChannel.Status, channel.Status)
				if err := tx.Save(&channel).Error; err != nil {
					return err
				}
				continue
			}
			channel = entity.RecorderChannel{
				RecorderID:      recorderID,
				ChannelNo:       deviceChannel.ChannelNo,
				Name:            defaultString(deviceChannel.Name, fmt.Sprintf("通道 %02d", deviceChannel.ChannelNo)),
				FactoryID:       recorder.FactoryID,
				Enabled:         deviceChannel.Enabled,
				SupportPlayback: true,
				Status:          normalizedStatus(deviceChannel.Status, "online"),
			}
			if err := tx.Create(&channel).Error; err != nil {
				return err
			}
		}

		recorder.ChannelCount = len(deviceChannels)
		recorder.Status = "online"
		recorder.LastOnlineAt = &now
		return tx.Save(recorder).Error
	}); err != nil {
		return nil, err
	}
	channels, err := NewQueryService(s.repo).ListChannels(ChannelListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	filtered := make([]map[string]any, 0)
	for _, item := range channels {
		if item.RecorderID == recorderID {
			filtered = append(filtered, channelDTOToMap(item))
		}
	}
	return map[string]any{
		"recorderId":   recorder.ID,
		"recorderName": recorder.Name,
		"channelCount": len(filtered),
		"channels":     filtered,
	}, nil
}

type recorderSyncChannel struct {
	ChannelNo    int
	Name         string
	Enabled      bool
	Status       string
	NamePriority int
}

type hikVideoInputChannelList struct {
	Channels []hikVideoInputChannel `xml:"VideoInputChannel"`
}

type hikVideoInputChannel struct {
	ID      int    `xml:"id"`
	Name    string `xml:"name"`
	Enabled string `xml:"enabled"`
}

type hikInputProxyChannelStatusList struct {
	Channels []hikInputProxyChannelStatus `xml:"InputProxyChannelStatus"`
}

type hikInputProxyChannelStatus struct {
	ID                        int                          `xml:"id"`
	Name                      string                       `xml:"name"`
	DeviceName                string                       `xml:"deviceName"`
	SourceInputPortDescriptor hikSourceInputPortDescriptor `xml:"sourceInputPortDescriptor"`
	IPAddress                 string                       `xml:"ipAddress"`
	Online                    string                       `xml:"online"`
	Enabled                   string                       `xml:"enabled"`
}

type hikInputProxyChannelList struct {
	Channels []hikInputProxyChannel `xml:"InputProxyChannel"`
}

type hikInputProxyChannel struct {
	ID                        int                          `xml:"id"`
	Name                      string                       `xml:"name"`
	DeviceName                string                       `xml:"deviceName"`
	SourceInputPortDescriptor hikSourceInputPortDescriptor `xml:"sourceInputPortDescriptor"`
	IPAddress                 string                       `xml:"ipAddress"`
	Online                    string                       `xml:"online"`
	Enabled                   string                       `xml:"enabled"`
}

type hikSourceInputPortDescriptor struct {
	Text         string `xml:",chardata"`
	IPAddress    string `xml:"ipAddress"`
	Online       string `xml:"online"`
	SrcInputPort string `xml:"srcInputPort"`
}

func (s *PlatformService) fetchRecorderChannels(recorder entity.RecorderDevice) ([]recorderSyncChannel, error) {
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve recorder password: %w", err)
	}
	if strings.TrimSpace(recorder.IP) == "" {
		return nil, fmt.Errorf("recorder ip is empty")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	baseURL := fmt.Sprintf("%s://%s", resolveDeviceProtocol(recorder.HTTPPort), recorder.IP)
	if recorder.HTTPPort > 0 {
		baseURL = fmt.Sprintf("%s:%d", baseURL, recorder.HTTPPort)
	}

	merged := make(map[int]recorderSyncChannel)
	for _, endpoint := range []struct {
		path string
		kind string
	}{
		{path: "/ISAPI/System/Video/inputs/channels", kind: "analog"},
		{path: "/ISAPI/ContentMgmt/InputProxy/channels/status", kind: "digital"},
		{path: "/ISAPI/ContentMgmt/InputProxy/channels", kind: "digital-config"},
	} {
		body, requestErr := hikHTTPGet(client, baseURL+endpoint.path, recorder.Username, password)
		if requestErr != nil {
			if s.logger != nil {
				s.logger.Debug("fetch recorder channel endpoint failed",
					zap.Uint("recorderId", recorder.ID),
					zap.String("endpoint", endpoint.path),
					zap.Error(requestErr),
				)
			}
			continue
		}
		channels, parseErr := parseHikRecorderChannels(body, endpoint.kind)
		if parseErr != nil {
			if s.logger != nil {
				s.logger.Debug("parse recorder channel endpoint failed",
					zap.Uint("recorderId", recorder.ID),
					zap.String("endpoint", endpoint.path),
					zap.Error(parseErr),
				)
			}
			continue
		}
		for _, channel := range channels {
			if channel.ChannelNo <= 0 {
				continue
			}
			if existing, ok := merged[channel.ChannelNo]; ok {
				merged[channel.ChannelNo] = mergeRecorderSyncChannel(existing, channel)
				continue
			}
			merged[channel.ChannelNo] = channel
		}
	}
	if len(merged) == 0 {
		return nil, fmt.Errorf("recorder channel list is empty")
	}

	channels := make([]recorderSyncChannel, 0, len(merged))
	for _, channel := range merged {
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ChannelNo < channels[j].ChannelNo
	})
	return channels, nil
}

func parseHikRecorderChannels(body []byte, kind string) ([]recorderSyncChannel, error) {
	switch kind {
	case "analog":
		var parsed hikVideoInputChannelList
		if err := xml.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		channels := make([]recorderSyncChannel, 0, len(parsed.Channels))
		for index, item := range parsed.Channels {
			channelNo := item.ID
			if channelNo <= 0 {
				channelNo = index + 1
			}
			enabled := parseOptionalBool(item.Enabled, true)
			channels = append(channels, recorderSyncChannel{
				ChannelNo:    channelNo,
				Name:         strings.TrimSpace(item.Name),
				Enabled:      enabled,
				Status:       chooseString(enabled, "online", "offline"),
				NamePriority: 10,
			})
		}
		return channels, nil
	case "digital":
		var parsed hikInputProxyChannelStatusList
		if err := xml.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		channels := make([]recorderSyncChannel, 0, len(parsed.Channels))
		for index, item := range parsed.Channels {
			channelNo := item.ID
			if channelNo <= 0 {
				channelNo = index + 1
			}
			enabled := parseOptionalBool(item.Enabled, true)
			online := parseOptionalBool(defaultString(item.Online, item.SourceInputPortDescriptor.Online), enabled)
			name, namePriority := resolveHikChannelDeviceName(
				item.DeviceName,
				item.Name,
				item.IPAddress,
				item.SourceInputPortDescriptor.IPAddress,
				item.SourceInputPortDescriptor.Text,
			)
			channels = append(channels, recorderSyncChannel{
				ChannelNo:    channelNo,
				Name:         name,
				Enabled:      enabled,
				Status:       chooseString(online, "online", "offline"),
				NamePriority: namePriority,
			})
		}
		return channels, nil
	case "digital-config":
		var parsed hikInputProxyChannelList
		if err := xml.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		channels := make([]recorderSyncChannel, 0, len(parsed.Channels))
		for index, item := range parsed.Channels {
			channelNo := item.ID
			if channelNo <= 0 {
				channelNo = index + 1
			}
			enabled := parseOptionalBool(item.Enabled, true)
			online := parseOptionalBool(defaultString(item.Online, item.SourceInputPortDescriptor.Online), enabled)
			name, namePriority := resolveHikChannelDeviceName(
				item.DeviceName,
				item.Name,
				item.IPAddress,
				item.SourceInputPortDescriptor.IPAddress,
				item.SourceInputPortDescriptor.Text,
			)
			channels = append(channels, recorderSyncChannel{
				ChannelNo:    channelNo,
				Name:         name,
				Enabled:      enabled,
				Status:       chooseString(online, "online", "offline"),
				NamePriority: namePriority,
			})
		}
		return channels, nil
	default:
		return nil, fmt.Errorf("unsupported channel kind %q", kind)
	}
}

func buildDefaultRecorderSyncChannels(count int) []recorderSyncChannel {
	channels := make([]recorderSyncChannel, 0, maxInt(count, 0))
	for i := 1; i <= count; i++ {
		channels = append(channels, recorderSyncChannel{
			ChannelNo: i,
			Name:      fmt.Sprintf("通道 %02d", i),
			Enabled:   true,
			Status:    "online",
		})
	}
	return channels
}

func completeRecorderSyncChannels(channels []recorderSyncChannel, expectedCount int) []recorderSyncChannel {
	if expectedCount <= 0 || len(channels) >= expectedCount {
		return channels
	}
	merged := make(map[int]recorderSyncChannel, expectedCount)
	for _, channel := range channels {
		if channel.ChannelNo <= 0 {
			continue
		}
		if existing, ok := merged[channel.ChannelNo]; ok {
			merged[channel.ChannelNo] = mergeRecorderSyncChannel(existing, channel)
			continue
		}
		merged[channel.ChannelNo] = channel
	}
	for _, channel := range buildDefaultRecorderSyncChannels(expectedCount) {
		if _, exists := merged[channel.ChannelNo]; !exists {
			merged[channel.ChannelNo] = channel
		}
	}
	completed := make([]recorderSyncChannel, 0, len(merged))
	for _, channel := range merged {
		completed = append(completed, channel)
	}
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].ChannelNo < completed[j].ChannelNo
	})
	return completed
}

func mergeRecorderSyncChannel(current, next recorderSyncChannel) recorderSyncChannel {
	merged := current
	if next.Enabled != current.Enabled {
		merged.Enabled = next.Enabled
	}
	if strings.TrimSpace(next.Status) != "" {
		merged.Status = next.Status
	}
	if strings.TrimSpace(next.Name) != "" && next.NamePriority >= current.NamePriority {
		merged.Name = next.Name
		merged.NamePriority = next.NamePriority
	}
	return merged
}

func resolveHikChannelDeviceName(values ...string) (string, int) {
	for index, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		return normalized, 40 - index*10
	}
	return "", 0
}

func parseOptionalBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func chooseString(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func buildHikPTZContinuousPayload(pan, tilt, zoom int) []byte {
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?><PTZData><pan>%d</pan><tilt>%d</tilt><zoom>%d</zoom></PTZData>`,
		pan,
		tilt,
		zoom,
	))
}

func hikHTTPGet(client *http.Client, targetURL, username, password string) ([]byte, error) {
	body, status, wwwAuthenticate, err := hikHTTPGetOnce(client, targetURL, username, password, "")
	if err == nil && status >= 200 && status < 300 {
		return body, nil
	}
	if status != http.StatusUnauthorized || !strings.Contains(strings.ToLower(wwwAuthenticate), "digest") {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("GET %s returned status %d", targetURL, status)
	}

	authorization, digestErr := buildDigestAuthorization(wwwAuthenticate, "GET", targetURL, username, password)
	if digestErr != nil {
		return nil, digestErr
	}
	body, status, _, err = hikHTTPGetOnce(client, targetURL, username, password, authorization)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("GET %s returned status %d", targetURL, status)
	}
	return body, nil
}

func hikHTTPPutXML(client *http.Client, targetURL, username, password string, payload []byte) error {
	_, status, wwwAuthenticate, err := hikHTTPPutXMLOnce(client, targetURL, username, password, payload, "")
	if err == nil && status >= 200 && status < 300 {
		return nil
	}
	if status != http.StatusUnauthorized || !strings.Contains(strings.ToLower(wwwAuthenticate), "digest") {
		if err != nil {
			return err
		}
		return fmt.Errorf("PUT %s returned status %d", targetURL, status)
	}

	authorization, digestErr := buildDigestAuthorization(wwwAuthenticate, "PUT", targetURL, username, password)
	if digestErr != nil {
		return digestErr
	}
	_, status, _, err = hikHTTPPutXMLOnce(client, targetURL, username, password, payload, authorization)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("PUT %s returned status %d", targetURL, status)
	}
	return nil
}

func hikHTTPGetOnce(client *http.Client, targetURL, username, password, authorization string) ([]byte, int, string, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, "", err
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	} else if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), err
	}
	return body, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), nil
}

func hikHTTPPutXMLOnce(client *http.Client, targetURL, username, password string, payload []byte, authorization string) ([]byte, int, string, error) {
	req, err := http.NewRequest(http.MethodPut, targetURL, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Content-Type", "application/xml")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	} else if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), err
	}
	return body, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), nil
}

func buildDigestAuthorization(challenge, method, targetURL, username, password string) (string, error) {
	values := parseDigestChallenge(challenge)
	realm := values["realm"]
	nonce := values["nonce"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("invalid digest challenge")
	}
	parsedURL, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return "", err
	}
	uri := parsedURL.URL.RequestURI()
	qop := values["qop"]
	if strings.Contains(qop, ",") {
		for _, item := range strings.Split(qop, ",") {
			if strings.TrimSpace(item) == "auth" {
				qop = "auth"
				break
			}
		}
	}
	if qop != "" && qop != "auth" {
		qop = "auth"
	}
	nc := "00000001"
	cnonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	ha1 := md5Hex(fmt.Sprintf("%s:%s:%s", username, realm, password))
	ha2 := md5Hex(fmt.Sprintf("%s:%s", method, uri))
	response := ""
	if qop == "" {
		response = md5Hex(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
	} else {
		response = md5Hex(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))
	}

	parts := []string{
		fmt.Sprintf(`username="%s"`, digestQuote(username)),
		fmt.Sprintf(`realm="%s"`, digestQuote(realm)),
		fmt.Sprintf(`nonce="%s"`, digestQuote(nonce)),
		fmt.Sprintf(`uri="%s"`, digestQuote(uri)),
		fmt.Sprintf(`response="%s"`, response),
	}
	if values["opaque"] != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, digestQuote(values["opaque"])))
	}
	if qop != "" {
		parts = append(parts,
			fmt.Sprintf(`qop=%s`, qop),
			fmt.Sprintf(`nc=%s`, nc),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
		)
	}
	if values["algorithm"] != "" {
		parts = append(parts, fmt.Sprintf(`algorithm=%s`, values["algorithm"]))
	}
	return "Digest " + strings.Join(parts, ", "), nil
}

func parseDigestChallenge(challenge string) map[string]string {
	challenge = strings.TrimSpace(challenge)
	challenge = strings.TrimPrefix(challenge, "Digest")
	challenge = strings.TrimSpace(challenge)
	values := make(map[string]string)
	for _, part := range strings.Split(challenge, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return values
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func digestQuote(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func (s *PlatformService) ListRecorderChannels(recorderID uint, accessScope *AccessScope) ([]map[string]any, error) {
	if _, err := s.ensureRecorderAccessible(accessScope, recorderID); err != nil {
		return nil, err
	}
	channels, err := NewQueryService(s.repo).ListChannels(ChannelListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	filtered := make([]map[string]any, 0)
	for _, item := range channels {
		if item.RecorderID == recorderID {
			filtered = append(filtered, channelDTOToMap(item))
		}
	}
	return filtered, nil
}

func (s *PlatformService) UpdateChannel(channelID uint, payload ChannelPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureChannelAccessible(accessScope, channelID)
	if err != nil {
		return nil, err
	}
	if err := validateChannelTargetAccess(accessScope, payload.FactoryID, payload.ZoneID, payload.CameraID, item.RecorderID, channelID); err != nil {
		return nil, err
	}
	item.Name = payload.Name
	item.CameraID = payload.CameraID
	item.FactoryID = payload.FactoryID
	item.ZoneID = payload.ZoneID
	item.Enabled = payload.Enabled
	item.SupportPlayback = payload.SupportPlayback
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	channels, err := NewQueryService(s.repo).ListChannels(ChannelListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	for _, record := range channels {
		if record.ID == channelID {
			return channelDTOToMap(record), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) ListDeviceStatusLogs(page, pageSize int, filter DeviceStatusLogListFilter, accessScope *AccessScope) (map[string]any, error) {
	page = maxInt(page, 1)
	pageSize = maxInt(pageSize, 20)
	buildQuery := func() *gorm.DB {
		query := s.db().Table("device_status_log AS l").
			Joins("LEFT JOIN camera_device c ON l.device_type = ? AND c.id = l.device_id", "camera").
			Joins("LEFT JOIN recorder_device r ON l.device_type = ? AND r.id = l.device_id", "recorder").
			Joins("LEFT JOIN recorder_channel ch ON l.device_type = ? AND ch.id = l.device_id", "channel")
		query = applyDeviceStatusLogAccessScopeQuery(query, accessScope)
		if filter.DeviceType != "" {
			query = query.Where("l.device_type = ?", filter.DeviceType)
		}
		if filter.Status != "" {
			query = query.Where("l.new_status = ?", filter.Status)
		}
		if filter.StartAt != nil {
			query = query.Where("l.checked_at >= ?", *filter.StartAt)
		}
		if filter.EndAt != nil {
			query = query.Where("l.checked_at <= ?", *filter.EndAt)
		}
		if keyword := strings.TrimSpace(filter.DeviceName); keyword != "" {
			likeKeyword := "%" + keyword + "%"
			query = query.Where("COALESCE(c.name, r.name, ch.name, CONCAT(l.device_type, '-', l.device_id)) LIKE ?", likeKeyword)
		}
		return query
	}

	var total int64
	if err := buildQuery().Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []deviceStatusLogRow
	if err := buildQuery().
		Select(`l.id, l.device_type, l.device_id,
			COALESCE(c.name, r.name, ch.name, CONCAT(l.device_type, '-', l.device_id)) AS device_name,
			l.old_status, l.new_status, l.message, l.checked_at`).
		Order("l.checked_at DESC, l.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		result = append(result, map[string]any{
			"id":         item.ID,
			"deviceType": item.DeviceType,
			"deviceId":   item.DeviceID,
			"deviceName": item.DeviceName,
			"oldStatus":  nullableString(item.OldStatus),
			"newStatus":  item.NewStatus,
			"message":    nullableString(item.Message),
			"checkedAt":  item.CheckedAt.Format(time.RFC3339),
		})
	}

	return map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize}, nil
}

type deviceStatusLogRow struct {
	ID         uint      `gorm:"column:id"`
	DeviceType string    `gorm:"column:device_type"`
	DeviceID   uint      `gorm:"column:device_id"`
	DeviceName string    `gorm:"column:device_name"`
	OldStatus  string    `gorm:"column:old_status"`
	NewStatus  string    `gorm:"column:new_status"`
	Message    string    `gorm:"column:message"`
	CheckedAt  time.Time `gorm:"column:checked_at"`
}

func applyDeviceStatusLogAccessScopeQuery(query *gorm.DB, accessScope *AccessScope) *gorm.DB {
	if accessScope == nil || accessScope.All {
		return query
	}
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 6)

	switch {
	case len(accessScope.CameraIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'camera' AND c.id IN ?)")
		args = append(args, accessScope.CameraIDs)
	case len(accessScope.RecorderIDs) > 0 || len(accessScope.ChannelIDs) > 0:
	case len(accessScope.ZoneIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'camera' AND c.zone_id IN ?)")
		args = append(args, accessScope.ZoneIDs)
	case len(accessScope.FactoryIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'camera' AND c.factory_id IN ?)")
		args = append(args, accessScope.FactoryIDs)
	}

	switch {
	case len(accessScope.RecorderIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'recorder' AND r.id IN ?)")
		args = append(args, accessScope.RecorderIDs)
	case len(accessScope.ChannelIDs) > 0:
	case len(accessScope.FactoryIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'recorder' AND r.factory_id IN ?)")
		args = append(args, accessScope.FactoryIDs)
	}

	switch {
	case len(accessScope.ChannelIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'channel' AND ch.id IN ?)")
		args = append(args, accessScope.ChannelIDs)
	case len(accessScope.RecorderIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'channel' AND ch.recorder_id IN ?)")
		args = append(args, accessScope.RecorderIDs)
	case len(accessScope.CameraIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'channel' AND ch.camera_id IN ?)")
		args = append(args, accessScope.CameraIDs)
	case len(accessScope.ZoneIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'channel' AND ch.zone_id IN ?)")
		args = append(args, accessScope.ZoneIDs)
	case len(accessScope.FactoryIDs) > 0:
		conditions = append(conditions, "(l.device_type = 'channel' AND ch.factory_id IN ?)")
		args = append(args, accessScope.FactoryIDs)
	}

	if len(conditions) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(strings.Join(conditions, " OR "), args...)
}

func (s *PlatformService) CheckAllDevicesStatus() (map[string]any, error) {
	result, err := s.runDeviceStatusCheck("批量状态检查", buildDeviceCheckCycleKey("manual-device-check", 0, time.Now()))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"checkedDevices":   result.CheckedTotal,
		"changedDevices":   result.ChangedTotal,
		"checkedCameras":   result.CheckedCameras,
		"checkedRecorders": result.CheckedRecorders,
		"checkedChannels":  result.CheckedChannels,
		"onlineDevices":    result.OnlineTotal,
		"offlineDevices":   result.OfflineTotal,
		"disabledDevices":  result.DisabledTotal,
		"message":          "全部设备状态检查完成，仅记录状态变化",
	}, nil
}

type deviceStatusCheckResult struct {
	CheckedTotal     int
	CheckedCameras   int
	CheckedRecorders int
	CheckedChannels  int
	OnlineTotal      int
	OfflineTotal     int
	DisabledTotal    int
	ChangedTotal     int
	OfflineDevices   []deviceOfflineItem
	StatusChanges    []deviceStatusChangeItem
}

type deviceOfflineItem struct {
	DeviceType       string
	DeviceID         uint
	DeviceName       string
	IP               string
	FactoryID        *uint
	ZoneID           *uint
	Location         string
	OldStatus        string
	NewStatus        string
	ChangedToOffline bool
}

type deviceStatusChangeItem struct {
	DeviceType string
	DeviceID   uint
	OldStatus  string
	NewStatus  string
}

func (s *PlatformService) runDeviceStatusCheck(message string, cycleKey string) (*deviceStatusCheckResult, error) {
	s.deviceCheckMu.Lock()
	defer s.deviceCheckMu.Unlock()

	const batchSize = 200
	result := &deviceStatusCheckResult{}

	var cameras []entity.CameraDevice
	cameraStatuses := make(map[uint]string)
	if err := s.db().FindInBatches(&cameras, batchSize, func(tx *gorm.DB, batch int) error {
		result.CheckedCameras += len(cameras)
		for _, camera := range cameras {
			oldStatus := normalizedStatus(camera.Status, "offline")
			nextStatus := probeDeviceTCPStatus(camera.IP, camera.HTTPPort, camera.SDKPort, camera.RTSPPort)
			cameraStatuses[camera.ID] = nextStatus
			result.addDeviceStatus(nextStatus)
			if nextStatus == "offline" {
				factoryID := camera.FactoryID
				zoneID := camera.ZoneID
				result.OfflineDevices = append(result.OfflineDevices, deviceOfflineItem{
					DeviceType:       "camera",
					DeviceID:         camera.ID,
					DeviceName:       camera.Name,
					IP:               camera.IP,
					FactoryID:        &factoryID,
					ZoneID:           &zoneID,
					Location:         camera.InstallLocation,
					OldStatus:        oldStatus,
					NewStatus:        nextStatus,
					ChangedToOffline: oldStatus != "offline",
				})
			}
			if changedNow, err := updateDeviceStatusIfChanged(tx, "camera", camera.ID, "camera_device", camera.Status, nextStatus, message); err != nil {
				return err
			} else if changedNow {
				result.ChangedTotal++
				result.StatusChanges = append(result.StatusChanges, deviceStatusChangeItem{
					DeviceType: "camera",
					DeviceID:   camera.ID,
					OldStatus:  oldStatus,
					NewStatus:  nextStatus,
				})
			}
		}
		return nil
	}).Error; err != nil {
		return nil, err
	}

	var recorders []entity.RecorderDevice
	recorderStatuses := make(map[uint]string)
	if err := s.db().FindInBatches(&recorders, batchSize, func(tx *gorm.DB, batch int) error {
		result.CheckedRecorders += len(recorders)
		for _, recorder := range recorders {
			oldStatus := normalizedStatus(recorder.Status, "offline")
			nextStatus := probeDeviceTCPStatus(recorder.IP, recorder.HTTPPort, recorder.SDKPort)
			recorderStatuses[recorder.ID] = nextStatus
			result.addDeviceStatus(nextStatus)
			if nextStatus == "offline" {
				factoryID := recorder.FactoryID
				result.OfflineDevices = append(result.OfflineDevices, deviceOfflineItem{
					DeviceType:       "recorder",
					DeviceID:         recorder.ID,
					DeviceName:       recorder.Name,
					IP:               recorder.IP,
					FactoryID:        &factoryID,
					OldStatus:        oldStatus,
					NewStatus:        nextStatus,
					ChangedToOffline: oldStatus != "offline",
				})
			}
			if changedNow, err := updateDeviceStatusIfChanged(tx, "recorder", recorder.ID, "recorder_device", recorder.Status, nextStatus, message); err != nil {
				return err
			} else if changedNow {
				result.ChangedTotal++
				result.StatusChanges = append(result.StatusChanges, deviceStatusChangeItem{
					DeviceType: "recorder",
					DeviceID:   recorder.ID,
					OldStatus:  oldStatus,
					NewStatus:  nextStatus,
				})
			}
		}
		return nil
	}).Error; err != nil {
		return nil, err
	}

	var channels []entity.RecorderChannel
	if err := s.db().FindInBatches(&channels, batchSize, func(tx *gorm.DB, batch int) error {
		result.CheckedChannels += len(channels)
		for _, channel := range channels {
			oldStatus := normalizedStatus(channel.Status, "offline")
			nextStatus := deriveChannelStatus(channel, recorderStatuses, cameraStatuses)
			result.addDeviceStatus(nextStatus)
			if nextStatus == "offline" {
				factoryID := channel.FactoryID
				result.OfflineDevices = append(result.OfflineDevices, deviceOfflineItem{
					DeviceType:       "channel",
					DeviceID:         channel.ID,
					DeviceName:       channel.Name,
					FactoryID:        &factoryID,
					ZoneID:           channel.ZoneID,
					OldStatus:        oldStatus,
					NewStatus:        nextStatus,
					ChangedToOffline: oldStatus != "offline",
				})
			}
			if changedNow, err := updateDeviceStatusIfChanged(tx, "channel", channel.ID, "recorder_channel", channel.Status, nextStatus, message); err != nil {
				return err
			} else if changedNow {
				result.ChangedTotal++
				result.StatusChanges = append(result.StatusChanges, deviceStatusChangeItem{
					DeviceType: "channel",
					DeviceID:   channel.ID,
					OldStatus:  oldStatus,
					NewStatus:  nextStatus,
				})
			}
		}
		return nil
	}).Error; err != nil {
		return nil, err
	}

	result.CheckedTotal = result.CheckedCameras + result.CheckedRecorders + result.CheckedChannels
	s.dispatchSmartBridgeReconnects(result, cycleKey)
	return result, nil
}

func (s *PlatformService) dispatchSmartBridgeReconnects(result *deviceStatusCheckResult, cycleKey string) {
	if s.smartBridgeReconnect == nil || result == nil {
		return
	}
	for _, item := range result.StatusChanges {
		s.smartBridgeReconnect.HandleDeviceStatus(item.DeviceType, item.DeviceID, item.OldStatus, item.NewStatus, cycleKey)
	}
	for _, item := range result.OfflineDevices {
		if normalizeReconnectStatus(item.OldStatus) == "offline" && normalizeReconnectStatus(item.NewStatus) == "offline" {
			s.smartBridgeReconnect.HandleDeviceStatus(item.DeviceType, item.DeviceID, item.OldStatus, item.NewStatus, cycleKey)
		}
	}
}

func (r *deviceStatusCheckResult) addDeviceStatus(status string) {
	switch status {
	case "online":
		r.OnlineTotal++
	case "disabled":
		r.DisabledTotal++
	default:
		r.OfflineTotal++
	}
}

func probeDeviceTCPStatus(ip string, ports ...int) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "offline"
	}
	for _, port := range ports {
		if port <= 0 {
			continue
		}
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, strconv.Itoa(port)), 800*time.Millisecond)
		if err != nil {
			continue
		}
		_ = conn.Close()
		return "online"
	}
	return "offline"
}

func deriveChannelStatus(channel entity.RecorderChannel, recorderStatuses, cameraStatuses map[uint]string) string {
	if !channel.Enabled {
		return "disabled"
	}
	if channel.CameraID != nil {
		if status, ok := cameraStatuses[*channel.CameraID]; ok {
			return status
		}
	}
	if status, ok := recorderStatuses[channel.RecorderID]; ok {
		return status
	}
	return "offline"
}

func updateDeviceStatusFromTest(tx *gorm.DB, deviceType string, deviceID uint, tableName, oldStatus, newStatus, message string) (bool, error) {
	changed, err := updateDeviceStatusIfChanged(tx, deviceType, deviceID, tableName, oldStatus, newStatus, message)
	if err != nil || changed || newStatus != "online" || tableName == "recorder_channel" {
		return changed, err
	}
	now := time.Now()
	if err := tx.Table(tableName).Where("id = ?", deviceID).Update("last_online_at", &now).Error; err != nil {
		return false, err
	}
	return false, nil
}

func updateDeviceStatusIfChanged(tx *gorm.DB, deviceType string, deviceID uint, tableName, oldStatus, newStatus, message string) (bool, error) {
	oldStatus = normalizedStatus(oldStatus, "offline")
	newStatus = normalizedStatus(newStatus, oldStatus)
	if oldStatus == newStatus {
		return false, nil
	}
	updates := map[string]any{"status": newStatus}
	if newStatus == "online" && tableName != "recorder_channel" {
		now := time.Now()
		updates["last_online_at"] = &now
	}
	if err := tx.Table(tableName).Where("id = ?", deviceID).Updates(updates).Error; err != nil {
		return false, err
	}
	if err := tx.Create(&entity.DeviceStatusLog{
		DeviceType: deviceType,
		DeviceID:   deviceID,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Message:    message,
		CheckedAt:  time.Now(),
	}).Error; err != nil {
		return false, err
	}
	return true, nil
}
