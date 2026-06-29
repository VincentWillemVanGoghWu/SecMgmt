package service

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
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
	SessionKey string
	DeviceType string
	DeviceID   uint
	DeviceName string
	DeviceIP   string
	SDKPort    int
	Username   string
	Password   string
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

	sdk, err := hikvision.NewSDK(s.cfg.HikvisionSDKPath)
	if err != nil {
		s.lastError = err.Error()
		return err
	}
	if err := sdk.SetAlarmHandler(s.handleAlarm); err != nil {
		s.lastError = err.Error()
		return err
	}
	s.sdk = sdk

	targets, err := s.collectTargets()
	if err != nil {
		s.lastError = err.Error()
		return err
	}
	s.logger.Info("hikvision sdk bridge targets collected",
		zap.Int("bindingCount", s.bindingCount),
		zap.Int("targetCount", len(targets)),
		zap.Int("skippedBindingCount", s.skippedBindingCount),
		zap.Int("mergedBindingCount", s.mergedBindingCount),
	)

	var startupErrors []string
	for _, target := range targets {
		if err := s.openSessionLocked(target); err != nil {
			startupErrors = append(startupErrors, fmt.Sprintf("%s: %v", target.SessionKey, err))
			s.logger.Warn("hikvision sdk bridge open session failed",
				zap.String("sessionKey", target.SessionKey),
				zap.Error(err),
			)
		}
	}
	s.running = len(s.sessions) > 0
	if len(startupErrors) > 0 {
		s.lastError = strings.Join(startupErrors, "; ")
	}
	s.logger.Info("hikvision sdk bridge startup finished",
		zap.Bool("running", s.running),
		zap.Int("sessionCount", len(s.sessions)),
		zap.String("lastError", s.lastError),
	)
	return nil
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
			"sessionKey": session.SessionKey,
			"deviceType": session.DeviceType,
			"deviceId":   session.DeviceID,
			"deviceName": session.DeviceName,
			"deviceIp":   session.DeviceIP,
		})
	}
	sort.Slice(sessionItems, func(i, j int) bool {
		return fmt.Sprint(sessionItems[i]["sessionKey"]) < fmt.Sprint(sessionItems[j]["sessionKey"])
	})
	return map[string]any{
		"running":             s.running,
		"sessionCount":        len(s.sessions),
		"bindingCount":        s.bindingCount,
		"skippedBindingCount": s.skippedBindingCount,
		"mergedBindingCount":  s.mergedBindingCount,
		"lastError":           s.lastError,
		"sessions":            sessionItems,
	}
}

func (s *HikvisionAlarmBridgeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *HikvisionAlarmBridgeService) stopLocked() {
	s.flushAllMotionWindows()
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
	s.motionCooldown = make(map[string]time.Time)
	s.running = false
}

func (s *HikvisionAlarmBridgeService) collectTargets() ([]hikvisionBridgeTarget, error) {
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
		Where("b.enabled = ? AND p.enabled = ? AND c.enabled = ? AND p.provider_code = ? AND c.capability_code = ?", true, true, true, "hikvision-sdk", "motion_detect").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	targetMap := make(map[string]hikvisionBridgeTarget)
	s.bindingCount = len(rows)
	s.skippedBindingCount = 0
	s.mergedBindingCount = 0
	for _, row := range rows {
		target, ok, err := s.bindingToTarget(row.SmartDeviceBinding)
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
	db := s.repo.DB()
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
			SessionKey: fmt.Sprintf("recorder:%d", recorder.ID),
			DeviceType: "recorder",
			DeviceID:   recorder.ID,
			DeviceName: recorder.Name,
			DeviceIP:   recorder.IP,
			SDKPort:    recorder.SDKPort,
			Username:   recorder.Username,
			Password:   password,
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
			SessionKey: fmt.Sprintf("recorder:%d", recorder.ID),
			DeviceType: "recorder",
			DeviceID:   recorder.ID,
			DeviceName: recorder.Name,
			DeviceIP:   recorder.IP,
			SDKPort:    recorder.SDKPort,
			Username:   recorder.Username,
			Password:   password,
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
						SessionKey: fmt.Sprintf("recorder:%d", recorder.ID),
						DeviceType: "recorder",
						DeviceID:   recorder.ID,
						DeviceName: recorder.Name,
						DeviceIP:   recorder.IP,
						SDKPort:    recorder.SDKPort,
						Username:   recorder.Username,
						Password:   password,
					}, true, nil
				}
			}
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), camera.PasswordEncrypted)
		if err != nil {
			return hikvisionBridgeTarget{}, false, err
		}
		return hikvisionBridgeTarget{
			SessionKey: fmt.Sprintf("camera:%d", camera.ID),
			DeviceType: "camera",
			DeviceID:   camera.ID,
			DeviceName: camera.Name,
			DeviceIP:   camera.IP,
			SDKPort:    camera.SDKPort,
			Username:   camera.Username,
			Password:   password,
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

func (s *HikvisionAlarmBridgeService) handleAlarm(alarm hikvision.MotionAlarm) {
	s.mu.RLock()
	session := s.sessionsByUserID[alarm.UserID]
	s.mu.RUnlock()
	if session == nil {
		return
	}

	for _, rawChannelNo := range alarm.Channels {
		channelNo := hikvision.NormalizeAlarmChannelNo(session.DeviceInfo, rawChannelNo)
		if err := s.aggregateMotionEvent(session, alarm.DeviceIP, rawChannelNo, channelNo, int(alarm.Command)); err != nil {
			s.logger.Error("persist hikvision motion event failed",
				zap.String("sessionKey", session.SessionKey),
				zap.Int("channelNo", channelNo),
				zap.Error(err),
			)
		}
	}
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
			return nil
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
			action, alarm, err := s.createOrMergeMotionAlarm(tx, smartEvent, rawEvent, window.FirstEventTime, window.ChannelNo, window.DeviceIP, window.Rule)
			if err != nil {
				return err
			}
			if alarm != nil && action != "cooldown_suppressed" {
				alarm.OccurrenceCount += maxInt(window.Count-1, 0)
				alarm.LastEventTime = timePtr(window.LastEventTime)
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
) error {
	eventTime := time.Now()
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

		provider, capability, binding, rule, err := s.matchMotionBinding(tx, camera, recorder, channel)
		if err != nil {
			return err
		}
		if provider == nil || capability == nil || binding == nil {
			return nil
		}
		payload := map[string]any{
			"capabilityCode": "motion_detect",
			"eventType":      "motion_detect",
			"eventTime":      eventTime.Format(time.RFC3339),
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
			"imageUrl":       nil,
		}
		rawEventID := fmt.Sprintf("hikvision-motion:%s:%d:%d:%s", binding.SourceType, binding.SourceID, channelNo, uuid.NewString()[:12])
		headers := map[string]string{"x-hikvision-bridge": "sdk-callback"}

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
			Confidence:            floatPtr(1),
			DedupKey:              buildSmartDedupKey(binding.SourceType, binding.SourceID, channelNo, firstZoneID(camera, channel)),
			NormalizedPayloadJSON: rawEvent.RawPayloadJSON,
			Status:                "stored",
		}
		smartEvent, err = s.createOrMergeSmartEvent(tx, smartEvent)
		if err != nil {
			return err
		}
		if shouldCaptureMotionSnapshot(rule, smartEvent) {
			snapshotEntityID := resolveSnapshotEntityID(camera, channel, recorder, session.DeviceID)
			if snapshotURL := s.captureSnapshot(session, snapshotEntityID, channelNo, eventTime); snapshotURL != "" {
				payload["imageUrl"] = nullableStringValue(snapshotURL)
				rawEvent.RawPayloadJSON = marshalJSON(payload)
				if err := tx.Save(&rawEvent).Error; err != nil {
					return err
				}
				smartEvent.ImageURL = snapshotURL
				smartEvent.NormalizedPayloadJSON = rawEvent.RawPayloadJSON
				if err := tx.Save(&smartEvent).Error; err != nil {
					return err
				}
			}
		}

		if rule != nil && rule.AlarmEnabled && rule.GenerateAlarmDirectly {
			smartEvent.Status = "alarm_generated"
			smartEvent.EventLevel = eventLevel
			if err := tx.Save(&smartEvent).Error; err != nil {
				return err
			}
			action, alarm, err := s.createOrMergeMotionAlarm(tx, smartEvent, rawEvent, eventTime, channelNo, deviceIP, rule)
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
	provider := &entity.SmartInterfaceProvider{}
	if err := tx.Where("provider_code = ? AND enabled = ?", "hikvision-sdk", true).First(provider).Error; err != nil {
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
	for _, candidate := range candidates {
		if candidate.sourceID == 0 {
			continue
		}
		binding := &entity.SmartDeviceBinding{}
		err := tx.Where(
			"provider_id = ? AND capability_id = ? AND source_type = ? AND source_id = ? AND enabled = ?",
			provider.ID, capability.ID, candidate.sourceType, candidate.sourceID, true,
		).Order("priority DESC, id ASC").First(binding).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil {
			return nil, nil, nil, nil, err
		}
		var rule entity.SmartBindingRule
		err = tx.Where("binding_id = ? AND enabled = ?", binding.ID, true).
			Order("generate_alarm_directly DESC, alarm_enabled DESC, id ASC").
			First(&rule).Error
		if err == gorm.ErrRecordNotFound {
			return provider, capability, binding, nil, nil
		}
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return provider, capability, binding, &rule, nil
	}
	return provider, capability, nil, nil, nil
}

func (s *HikvisionAlarmBridgeService) createOrMergeMotionAlarm(
	tx *gorm.DB,
	smartEvent entity.SmartEvent,
	rawEvent entity.SmartRawEvent,
	eventTime time.Time,
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
			existing.LastEventTime = timePtr(eventTime)
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
		RecordEndTime:   timePtr(eventTime.Add(90 * time.Second)),
		PushRecordsJSON: "[]",
		DedupKey:        dedupKey,
		OccurrenceCount: 1,
		LastEventTime:   timePtr(eventTime),
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
		CreatedAt:    eventTime,
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
