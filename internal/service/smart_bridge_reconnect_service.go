package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"secmgmt_go/internal/domain/entity"

	"go.uber.org/zap"
)

const (
	smartBridgeReconnectMaxAttempts = 5
	smartBridgeReconnectInterval    = time.Minute
	smartBridgeReconnectScanTick    = 10 * time.Second
)

type smartBridgeReconnectTask struct {
	TaskKey       string
	CycleKey      string
	TriggerReason string
	DeviceType    string
	DeviceID      uint
	SessionKey    string
	BindingIDs    []uint
	Target        hikvisionBridgeTarget
	Attempts      int
	MaxAttempts   int
	NextRunAt     time.Time
	Status        string
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	FinishedAt    *time.Time
}

type SmartBridgeReconnectService struct {
	bridge *HikvisionAlarmBridgeService
	logger *zap.Logger

	mu                sync.Mutex
	tasks             map[string]*smartBridgeReconnectTask
	lastFinishedCycle map[string]string
	stopCh            chan struct{}
	stopOnce          sync.Once
}

func NewSmartBridgeReconnectService(bridge *HikvisionAlarmBridgeService, logger *zap.Logger) *SmartBridgeReconnectService {
	return &SmartBridgeReconnectService{
		bridge:            bridge,
		logger:            logger,
		tasks:             make(map[string]*smartBridgeReconnectTask),
		lastFinishedCycle: make(map[string]string),
		stopCh:            make(chan struct{}),
	}
}

func (s *SmartBridgeReconnectService) Start() func() {
	if s == nil || s.bridge == nil {
		return func() {}
	}
	go func() {
		ticker := time.NewTicker(smartBridgeReconnectScanTick)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runDueTasks(time.Now())
			case <-s.stopCh:
				return
			}
		}
	}()
	return func() {
		s.stopOnce.Do(func() {
			close(s.stopCh)
		})
	}
}

func (s *SmartBridgeReconnectService) HandleDeviceStatus(deviceType string, deviceID uint, oldStatus, newStatus, cycleKey string) {
	if s == nil || s.bridge == nil || deviceID == 0 {
		return
	}
	oldStatus = normalizeReconnectStatus(oldStatus)
	newStatus = normalizeReconnectStatus(newStatus)
	cycleKey = strings.TrimSpace(cycleKey)
	if cycleKey == "" {
		cycleKey = time.Now().Format("20060102150405")
	}

	switch {
	case oldStatus == "offline" && newStatus == "online":
		s.enqueueDevice(deviceType, deviceID, cycleKey, "offline_to_online", true)
	case oldStatus == "online" && newStatus == "offline":
		closed := s.bridge.CloseDeviceSessions(deviceType, deviceID)
		s.logStatusChange(deviceType, deviceID, cycleKey, "online_to_offline", "closed", closed, "")
	case oldStatus == "offline" && newStatus == "offline":
		s.enqueueDevice(deviceType, deviceID, cycleKey, "offline_still", false)
	}
}

func (s *SmartBridgeReconnectService) ReconnectBindingNow(bindingID uint) (map[string]any, error) {
	if s == nil || s.bridge == nil {
		return nil, fmt.Errorf("smart bridge reconnect service is not attached")
	}
	target, ok, err := s.bridge.ResolveReconnectTargetForBinding(bindingID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("当前绑定不是可重连的海康移动侦测绑定，或绑定/提供方/能力未启用")
	}
	cycleKey := buildDeviceCheckCycleKey("manual-smart-reconnect", bindingID, time.Now())
	s.enqueueTarget(target.Target.DeviceType, target.Target.DeviceID, cycleKey, "manual_binding_reconnect", target, true, time.Now())
	return map[string]any{
		"queued":     true,
		"bindingIds": target.BindingIDs,
		"sessionKey": target.Target.SessionKey,
		"deviceType": target.Target.DeviceType,
		"deviceId":   target.Target.DeviceID,
		"cycleKey":   cycleKey,
		"message":    "智能接口重连任务已提交",
	}, nil
}

func (s *SmartBridgeReconnectService) RuntimeStatus() map[string]any {
	if s == nil {
		return map[string]any{"taskCount": 0, "tasks": []map[string]any{}}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]map[string]any, 0, len(s.tasks))
	pending := 0
	running := 0
	success := 0
	failed := 0
	for _, task := range s.tasks {
		switch task.Status {
		case "pending":
			pending++
		case "running":
			running++
		case "success":
			success++
		case "failed":
			failed++
		}
		tasks = append(tasks, map[string]any{
			"taskKey":     task.TaskKey,
			"cycleKey":    task.CycleKey,
			"reason":      task.TriggerReason,
			"deviceType":  task.DeviceType,
			"deviceId":    task.DeviceID,
			"sessionKey":  task.SessionKey,
			"bindingIds":  append([]uint(nil), task.BindingIDs...),
			"attempts":    task.Attempts,
			"maxAttempts": task.MaxAttempts,
			"nextRunAt":   task.NextRunAt.Format(time.RFC3339),
			"status":      task.Status,
			"lastError":   task.LastError,
			"createdAt":   task.CreatedAt.Format(time.RFC3339),
			"updatedAt":   task.UpdatedAt.Format(time.RFC3339),
			"finishedAt":  timePtrToRFC3339(task.FinishedAt),
		})
	}
	return map[string]any{
		"taskCount":    len(s.tasks),
		"pendingCount": pending,
		"runningCount": running,
		"successCount": success,
		"failedCount":  failed,
		"tasks":        tasks,
	}
}

func (s *SmartBridgeReconnectService) enqueueDevice(deviceType string, deviceID uint, cycleKey, reason string, immediate bool) {
	targets, err := s.bridge.ResolveReconnectTargetsForDevice(deviceType, deviceID)
	if err != nil {
		s.logStatusChange(deviceType, deviceID, cycleKey, reason, "resolve_failed", 0, err.Error())
		return
	}
	if len(targets) == 0 {
		s.logStatusChange(deviceType, deviceID, cycleKey, reason, "no_target", 0, "")
		return
	}
	now := time.Now()
	for _, target := range targets {
		s.enqueueTarget(deviceType, deviceID, cycleKey, reason, target, immediate, now)
	}
}

func (s *SmartBridgeReconnectService) enqueueTarget(deviceType string, deviceID uint, cycleKey, reason string, target smartBridgeReconnectTarget, immediate bool, now time.Time) {
	taskKey := buildSmartBridgeReconnectTaskKey(target.Target.SessionKey)
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing := s.tasks[taskKey]; existing != nil && (existing.Status == "pending" || existing.Status == "running") {
		s.logReconnectTaskLocked(existing, "skip_active", "")
		return
	}
	if !immediate && s.lastFinishedCycle[taskKey] == cycleKey {
		if existing := s.tasks[taskKey]; existing != nil {
			s.logReconnectTaskLocked(existing, "skip_same_cycle", "")
		}
		return
	}
	if s.bridge.HasSession(target.Target.SessionKey) {
		s.lastFinishedCycle[taskKey] = cycleKey
		task := &smartBridgeReconnectTask{
			TaskKey:       taskKey,
			CycleKey:      cycleKey,
			TriggerReason: reason,
			DeviceType:    strings.ToLower(strings.TrimSpace(deviceType)),
			DeviceID:      deviceID,
			SessionKey:    target.Target.SessionKey,
			BindingIDs:    append([]uint(nil), target.BindingIDs...),
			Target:        target.Target,
			MaxAttempts:   smartBridgeReconnectMaxAttempts,
			Status:        "success",
			CreatedAt:     now,
			UpdatedAt:     now,
			FinishedAt:    &now,
		}
		s.tasks[taskKey] = task
		s.logReconnectTaskLocked(task, "already_connected", "")
		return
	}

	nextRunAt := now.Add(smartBridgeReconnectInterval)
	if immediate {
		nextRunAt = now
	}
	task := &smartBridgeReconnectTask{
		TaskKey:       taskKey,
		CycleKey:      cycleKey,
		TriggerReason: reason,
		DeviceType:    strings.ToLower(strings.TrimSpace(deviceType)),
		DeviceID:      deviceID,
		SessionKey:    target.Target.SessionKey,
		BindingIDs:    append([]uint(nil), target.BindingIDs...),
		Target:        target.Target,
		MaxAttempts:   smartBridgeReconnectMaxAttempts,
		NextRunAt:     nextRunAt,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	s.tasks[taskKey] = task
	s.logReconnectTaskLocked(task, "queued", "")
}

func (s *SmartBridgeReconnectService) runDueTasks(now time.Time) {
	dueTasks := make([]*smartBridgeReconnectTask, 0)
	s.mu.Lock()
	for _, task := range s.tasks {
		if task.Status != "pending" {
			continue
		}
		if task.NextRunAt.After(now) {
			continue
		}
		task.Status = "running"
		task.UpdatedAt = now
		dueTasks = append(dueTasks, cloneSmartBridgeReconnectTask(task))
	}
	s.mu.Unlock()

	for _, task := range dueTasks {
		s.runTaskAttempt(task, time.Now())
	}
}

func (s *SmartBridgeReconnectService) runTaskAttempt(task *smartBridgeReconnectTask, now time.Time) {
	err := s.bridge.ReconnectTarget(task.Target)
	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.tasks[task.TaskKey]
	if current == nil {
		return
	}
	current.Attempts++
	current.UpdatedAt = now
	if err == nil {
		current.Status = "success"
		current.LastError = ""
		current.FinishedAt = &now
		s.lastFinishedCycle[current.TaskKey] = current.CycleKey
		s.logReconnectTaskLocked(current, "success", "")
		return
	}

	current.LastError = err.Error()
	if current.Attempts >= current.MaxAttempts {
		current.Status = "failed"
		current.FinishedAt = &now
		s.lastFinishedCycle[current.TaskKey] = current.CycleKey
		s.logReconnectTaskLocked(current, "failed", err.Error())
		return
	}
	current.Status = "pending"
	current.NextRunAt = now.Add(smartBridgeReconnectInterval)
	s.logReconnectTaskLocked(current, "retry_scheduled", err.Error())
}

func (s *SmartBridgeReconnectService) logStatusChange(deviceType string, deviceID uint, cycleKey, reason, status string, count int, detail string) {
	s.persistLog(entity.SmartBridgeReconnectLog{
		TaskKey:       fmt.Sprintf("device:%s:%d", strings.ToLower(strings.TrimSpace(deviceType)), deviceID),
		CycleKey:      cycleKey,
		TriggerReason: reason,
		Action:        status,
		Status:        status,
		DeviceType:    strings.ToLower(strings.TrimSpace(deviceType)),
		DeviceID:      deviceID,
		Attempt:       count,
		Detail:        detail,
		CreatedAt:     time.Now(),
	})
	if s.logger == nil {
		return
	}
	fields := []zap.Field{
		zap.String("event", "smart_bridge_reconnect"),
		zap.String("cycleKey", cycleKey),
		zap.String("reason", reason),
		zap.String("status", status),
		zap.String("deviceType", strings.ToLower(strings.TrimSpace(deviceType))),
		zap.Uint("deviceID", deviceID),
		zap.Int("count", count),
	}
	if detail != "" {
		fields = append(fields, zap.String("detail", detail))
	}
	s.logger.Info("smart bridge reconnect status change handled", fields...)
}

func (s *SmartBridgeReconnectService) logReconnectTaskLocked(task *smartBridgeReconnectTask, action, detail string) {
	if task != nil {
		s.persistLog(entity.SmartBridgeReconnectLog{
			TaskKey:        task.TaskKey,
			CycleKey:       task.CycleKey,
			TriggerReason:  task.TriggerReason,
			Action:         action,
			Status:         task.Status,
			DeviceType:     task.DeviceType,
			DeviceID:       task.DeviceID,
			SessionKey:     task.SessionKey,
			BindingIDsJSON: encodeJSON(task.BindingIDs),
			Attempt:        task.Attempts,
			MaxAttempts:    task.MaxAttempts,
			NextRunAt:      nullableFutureTime(task.NextRunAt),
			Detail:         detail,
			LastError:      task.LastError,
			CreatedAt:      time.Now(),
		})
	}
	if s.logger == nil || task == nil {
		return
	}
	fields := []zap.Field{
		zap.String("event", "smart_bridge_reconnect"),
		zap.String("action", action),
		zap.String("taskKey", task.TaskKey),
		zap.String("cycleKey", task.CycleKey),
		zap.String("reason", task.TriggerReason),
		zap.String("deviceType", task.DeviceType),
		zap.Uint("deviceID", task.DeviceID),
		zap.String("sessionKey", task.SessionKey),
		zap.Uints("bindingIDs", task.BindingIDs),
		zap.Int("attempt", task.Attempts),
		zap.Int("maxAttempts", task.MaxAttempts),
		zap.String("status", task.Status),
	}
	if detail != "" {
		fields = append(fields, zap.String("detail", detail))
	}
	if task.LastError != "" {
		fields = append(fields, zap.String("lastError", task.LastError))
	}
	s.logger.Info("smart bridge reconnect task", fields...)
}

func (s *SmartBridgeReconnectService) persistLog(item entity.SmartBridgeReconnectLog) {
	if s == nil || s.bridge == nil || s.bridge.repo == nil {
		return
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	if item.MaxAttempts == 0 {
		item.MaxAttempts = smartBridgeReconnectMaxAttempts
	}
	if err := s.bridge.repo.DB().Create(&item).Error; err != nil && s.logger != nil {
		s.logger.Warn("persist smart bridge reconnect log failed",
			zap.String("taskKey", item.TaskKey),
			zap.String("action", item.Action),
			zap.Error(err),
		)
	}
}

func nullableFutureTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copyValue := value
	return &copyValue
}

func buildSmartBridgeReconnectTaskKey(sessionKey string) string {
	return "hikvision:" + strings.TrimSpace(sessionKey)
}

func cloneSmartBridgeReconnectTask(task *smartBridgeReconnectTask) *smartBridgeReconnectTask {
	if task == nil {
		return nil
	}
	copyTask := *task
	copyTask.BindingIDs = append([]uint(nil), task.BindingIDs...)
	return &copyTask
}

func normalizeReconnectStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "offline"
	}
	switch value {
	case "online", "disabled":
		return value
	default:
		return "offline"
	}
}

func buildDeviceCheckCycleKey(prefix string, id uint, at time.Time) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "device-check"
	}
	return fmt.Sprintf("%s:%d:%s", prefix, id, at.Format("20060102150405"))
}
