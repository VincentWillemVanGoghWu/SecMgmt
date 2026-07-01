package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"secmgmt_go/internal/domain/entity"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	deviceCheckNotifyOfflineChanged = "offline_changed"
	deviceCheckNotifyOfflineEachRun = "offline_each_run"
)

func (s *PlatformService) ListDeviceCheckSchedules() ([]map[string]any, error) {
	var items []entity.DeviceCheckSchedule
	if err := s.db().Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, deviceCheckScheduleToMap(item))
	}
	return result, nil
}

func (s *PlatformService) CreateDeviceCheckSchedule(payload DeviceCheckSchedulePayload) (map[string]any, error) {
	if err := normalizeDeviceCheckSchedulePayload(&payload); err != nil {
		return nil, err
	}
	now := time.Now()
	nextRunAt := nextDeviceCheckRunAt(now, payload.FrequencyPerDay)
	item := entity.DeviceCheckSchedule{
		Name:              payload.Name,
		Enabled:           payload.Enabled,
		FrequencyPerDay:   payload.FrequencyPerDay,
		NotifyEnabled:     payload.NotifyEnabled,
		PushConfigIDsJSON: encodeJSON(payload.PushConfigIDs),
		NotifyMode:        payload.NotifyMode,
		NextRunAt:         &nextRunAt,
	}
	if !item.Enabled {
		item.NextRunAt = nil
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return deviceCheckScheduleToMap(item), nil
}

func (s *PlatformService) UpdateDeviceCheckSchedule(id uint, payload DeviceCheckSchedulePayload) (map[string]any, error) {
	if err := normalizeDeviceCheckSchedulePayload(&payload); err != nil {
		return nil, err
	}
	var item entity.DeviceCheckSchedule
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	item.Name = payload.Name
	item.Enabled = payload.Enabled
	item.FrequencyPerDay = payload.FrequencyPerDay
	item.NotifyEnabled = payload.NotifyEnabled
	item.PushConfigIDsJSON = encodeJSON(payload.PushConfigIDs)
	item.NotifyMode = payload.NotifyMode
	nextRunAt := nextDeviceCheckRunAt(time.Now(), payload.FrequencyPerDay)
	item.NextRunAt = &nextRunAt
	if !item.Enabled {
		item.NextRunAt = nil
	}
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return deviceCheckScheduleToMap(item), nil
}

func (s *PlatformService) UpdateDeviceCheckScheduleStatus(id uint, enabled bool) (map[string]any, error) {
	var item entity.DeviceCheckSchedule
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	item.Enabled = enabled
	if enabled {
		nextRunAt := nextDeviceCheckRunAt(time.Now(), item.FrequencyPerDay)
		item.NextRunAt = &nextRunAt
		item.LastError = ""
	} else {
		item.NextRunAt = nil
	}
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return deviceCheckScheduleToMap(item), nil
}

func (s *PlatformService) DeleteDeviceCheckSchedule(id uint) error {
	return s.db().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("schedule_id = ?", id).Delete(&entity.DeviceCheckPushLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("schedule_id = ?", id).Delete(&entity.DeviceCheckRun{}).Error; err != nil {
			return err
		}
		return tx.Delete(&entity.DeviceCheckSchedule{}, id).Error
	})
}

func (s *PlatformService) RunDeviceCheckScheduleNow(id uint) (map[string]any, error) {
	return s.executeDeviceCheckSchedule(id, "manual")
}

func (s *PlatformService) runDueDeviceCheckSchedules(now time.Time) {
	var items []entity.DeviceCheckSchedule
	if err := s.db().
		Where("enabled = ?", true).
		Where("next_run_at IS NULL OR next_run_at <= ?", now).
		Order("id ASC").
		Find(&items).Error; err != nil {
		if s.logger != nil {
			s.logger.Warn("load due device check schedules failed", zap.Error(err))
		}
		return
	}
	for _, item := range items {
		if item.NextRunAt == nil {
			nextRunAt := nextDeviceCheckRunAt(now, item.FrequencyPerDay)
			_ = s.db().Model(&item).Update("next_run_at", &nextRunAt).Error
			continue
		}
		if _, err := s.executeDeviceCheckSchedule(item.ID, "schedule"); err != nil && s.logger != nil {
			s.logger.Warn("execute device check schedule failed", zap.Uint("scheduleID", item.ID), zap.Error(err))
		}
	}
}

func (s *PlatformService) executeDeviceCheckSchedule(id uint, triggeredBy string) (map[string]any, error) {
	var schedule entity.DeviceCheckSchedule
	if err := s.db().First(&schedule, id).Error; err != nil {
		return nil, err
	}

	startedAt := time.Now()
	scheduleID := schedule.ID
	run := entity.DeviceCheckRun{
		ScheduleID: &scheduleID,
		StartedAt:  startedAt,
		Status:     "running",
		CreatedAt:  startedAt,
	}
	if err := s.db().Create(&run).Error; err != nil {
		return nil, err
	}

	result, err := s.runDeviceStatusCheck("定时状态检查", buildDeviceCheckCycleKey("device-check", run.ID, startedAt))
	finishedAt := time.Now()
	nextRunAt := nextDeviceCheckRunAt(finishedAt, schedule.FrequencyPerDay)
	updates := map[string]any{
		"last_run_at": &finishedAt,
		"next_run_at": &nextRunAt,
	}
	runUpdates := map[string]any{
		"finished_at": &finishedAt,
	}
	if err != nil {
		runUpdates["status"] = "failed"
		runUpdates["error_message"] = err.Error()
		updates["last_error"] = err.Error()
		_ = s.db().Model(&run).Updates(runUpdates).Error
		_ = s.db().Model(&schedule).Updates(updates).Error
		return nil, err
	}

	notifyItems := filterDeviceCheckNotifyItems(result.OfflineDevices, schedule.NotifyMode)
	notified := false
	if schedule.NotifyEnabled && len(notifyItems) > 0 {
		if sent, notifyErr := s.notifyDeviceOffline(schedule, run, result, notifyItems, triggeredBy); notifyErr != nil {
			if s.logger != nil {
				s.logger.Warn("device offline notification failed", zap.Uint("scheduleID", schedule.ID), zap.Error(notifyErr))
			}
		} else {
			notified = sent
		}
	}

	runUpdates["status"] = "success"
	runUpdates["checked_total"] = result.CheckedTotal
	runUpdates["online_total"] = result.OnlineTotal
	runUpdates["offline_total"] = result.OfflineTotal
	runUpdates["disabled_total"] = result.DisabledTotal
	runUpdates["changed_total"] = result.ChangedTotal
	runUpdates["notified"] = notified
	runUpdates["error_message"] = ""
	updates["last_success_at"] = &finishedAt
	updates["last_error"] = ""
	if !schedule.Enabled {
		updates["next_run_at"] = nil
	}
	if err := s.db().Model(&run).Updates(runUpdates).Error; err != nil {
		return nil, err
	}
	if err := s.db().Model(&schedule).Updates(updates).Error; err != nil {
		return nil, err
	}

	run.Status = "success"
	run.FinishedAt = &finishedAt
	run.CheckedTotal = result.CheckedTotal
	run.OnlineTotal = result.OnlineTotal
	run.OfflineTotal = result.OfflineTotal
	run.DisabledTotal = result.DisabledTotal
	run.ChangedTotal = result.ChangedTotal
	run.Notified = notified
	return map[string]any{
		"schedule":       deviceCheckScheduleToMap(schedule),
		"run":            deviceCheckRunToMap(run),
		"checkedDevices": result.CheckedTotal,
		"offlineDevices": result.OfflineTotal,
		"changedDevices": result.ChangedTotal,
		"notified":       notified,
		"message":        "设备巡检执行完成",
	}, nil
}

func (s *PlatformService) ListDeviceCheckRuns(page, pageSize int, scheduleID uint) (map[string]any, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	query := s.db().Model(&entity.DeviceCheckRun{})
	if scheduleID > 0 {
		query = query.Where("schedule_id = ?", scheduleID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []entity.DeviceCheckRun
	if err := query.Order("started_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, deviceCheckRunToMap(item))
	}
	return map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize}, nil
}

func (s *PlatformService) notifyDeviceOffline(schedule entity.DeviceCheckSchedule, run entity.DeviceCheckRun, result *deviceStatusCheckResult, items []deviceOfflineItem, triggeredBy string) (bool, error) {
	pushConfigIDs := decodeJSONUintSlice(schedule.PushConfigIDsJSON)
	if len(pushConfigIDs) == 0 {
		return false, nil
	}
	var configs []entity.PushConfig
	if err := s.db().
		Where("id IN ? AND enabled = ? AND provider_type = ?", pushConfigIDs, true, "email").
		Find(&configs).Error; err != nil {
		return false, err
	}
	configsByID := make(map[uint]entity.PushConfig, len(configs))
	for _, item := range configs {
		configsByID[item.ID] = item
	}

	sent := false
	now := time.Now()
	for _, configID := range pushConfigIDs {
		config, ok := configsByID[configID]
		if !ok {
			continue
		}
		var delivery pushDeliveryResult
		if isDeviceCheckPushRateLimited(s.db(), config, now) {
			delivery = pushDeliveryResult{Status: "rate_limited", Message: "触发限流，已跳过设备离线邮件推送"}
		} else {
			delivery = deliverDeviceOfflineEmailPush(s.cfg, config, deviceOfflineEmailContext{
				ScheduleName: schedule.Name,
				CheckedAt:    now,
				CheckedTotal: result.CheckedTotal,
				OfflineTotal: result.OfflineTotal,
				ChangedTotal: result.ChangedTotal,
				TriggeredBy:  triggeredBy,
				Items:        deviceOfflineItemsToEmail(items),
			})
		}
		if delivery.Status == "success" {
			sent = true
		}
		scheduleID := schedule.ID
		runID := run.ID
		pushID := config.ID
		logItem := entity.DeviceCheckPushLog{
			ScheduleID:   &scheduleID,
			RunID:        &runID,
			PushConfigID: &pushID,
			Status:       delivery.Status,
			ConfigName:   config.ConfigName,
			OfflineCount: len(items),
			Message:      delivery.Message,
			RequestBody:  delivery.RequestBody,
			ResponseBody: delivery.ResponseBody,
			ErrorMessage: delivery.ErrorMessage,
			PushedAt:     time.Now(),
		}
		if err := s.db().Create(&logItem).Error; err != nil {
			return sent, err
		}
	}
	return sent, nil
}

func isDeviceCheckPushRateLimited(db *gorm.DB, config entity.PushConfig, now time.Time) bool {
	if db == nil || config.RateLimitWindowSeconds <= 0 || config.RateLimitMaxCount <= 0 {
		return false
	}
	cutoff := now.Add(-time.Duration(config.RateLimitWindowSeconds) * time.Second)
	var count int64
	_ = db.Model(&entity.DeviceCheckPushLog{}).
		Where("push_config_id = ? AND pushed_at >= ? AND status IN ?", config.ID, cutoff, []string{"success", "failed"}).
		Count(&count).Error
	return count >= int64(config.RateLimitMaxCount)
}

func filterDeviceCheckNotifyItems(items []deviceOfflineItem, mode string) []deviceOfflineItem {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = deviceCheckNotifyOfflineChanged
	}
	if mode == deviceCheckNotifyOfflineEachRun {
		return items
	}
	result := make([]deviceOfflineItem, 0, len(items))
	for _, item := range items {
		if item.ChangedToOffline {
			result = append(result, item)
		}
	}
	return result
}

func normalizeDeviceCheckSchedulePayload(payload *DeviceCheckSchedulePayload) error {
	if payload == nil {
		return fmt.Errorf("巡检计划参数无效")
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return fmt.Errorf("计划名称不能为空")
	}
	if payload.FrequencyPerDay <= 0 {
		payload.FrequencyPerDay = 1
	}
	if payload.FrequencyPerDay > 24 {
		return fmt.Errorf("每天检测次数不能超过 24 次")
	}
	payload.NotifyMode = strings.TrimSpace(payload.NotifyMode)
	if payload.NotifyMode == "" {
		payload.NotifyMode = deviceCheckNotifyOfflineChanged
	}
	switch payload.NotifyMode {
	case deviceCheckNotifyOfflineChanged, deviceCheckNotifyOfflineEachRun:
	default:
		return fmt.Errorf("不支持的离线推送策略")
	}
	payload.PushConfigIDs = uniqueUintValues(payload.PushConfigIDs)
	if payload.NotifyEnabled && len(payload.PushConfigIDs) == 0 {
		return fmt.Errorf("启用邮件推送时必须选择邮件推送配置")
	}
	return nil
}

func uniqueUintValues(values []uint) []uint {
	result := make([]uint, 0, len(values))
	seen := make(map[uint]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func nextDeviceCheckRunAt(base time.Time, frequencyPerDay int) time.Time {
	if frequencyPerDay <= 0 {
		frequencyPerDay = 1
	}
	if frequencyPerDay > 24 {
		frequencyPerDay = 24
	}
	return base.Add(24 * time.Hour / time.Duration(frequencyPerDay))
}

func deviceCheckScheduleToMap(item entity.DeviceCheckSchedule) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"name":            item.Name,
		"enabled":         item.Enabled,
		"frequencyPerDay": item.FrequencyPerDay,
		"notifyEnabled":   item.NotifyEnabled,
		"pushConfigIds":   decodeJSONUintSlice(item.PushConfigIDsJSON),
		"notifyMode":      item.NotifyMode,
		"lastRunAt":       timePtrToRFC3339(item.LastRunAt),
		"nextRunAt":       timePtrToRFC3339(item.NextRunAt),
		"lastSuccessAt":   timePtrToRFC3339(item.LastSuccessAt),
		"lastError":       item.LastError,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}

func deviceCheckRunToMap(item entity.DeviceCheckRun) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"scheduleId":    item.ScheduleID,
		"startedAt":     item.StartedAt,
		"finishedAt":    timePtrToRFC3339(item.FinishedAt),
		"status":        item.Status,
		"checkedTotal":  item.CheckedTotal,
		"onlineTotal":   item.OnlineTotal,
		"offlineTotal":  item.OfflineTotal,
		"disabledTotal": item.DisabledTotal,
		"changedTotal":  item.ChangedTotal,
		"notified":      item.Notified,
		"errorMessage":  item.ErrorMessage,
		"createdAt":     item.CreatedAt,
	}
}

func deviceOfflineItemsToEmail(items []deviceOfflineItem) []deviceOfflineEmailItem {
	result := make([]deviceOfflineEmailItem, 0, len(items))
	for _, item := range items {
		location := strings.TrimSpace(item.Location)
		result = append(result, deviceOfflineEmailItem{
			DeviceType: deviceTypeDisplayName(item.DeviceType),
			DeviceID:   item.DeviceID,
			DeviceName: item.DeviceName,
			IP:         item.IP,
			Location:   location,
			OldStatus:  item.OldStatus,
			NewStatus:  item.NewStatus,
		})
	}
	return result
}

func deviceTypeDisplayName(value string) string {
	switch value {
	case "camera":
		return "摄像机"
	case "recorder":
		return "录像机"
	case "channel":
		return "通道"
	default:
		return value
	}
}
