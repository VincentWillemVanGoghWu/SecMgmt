package service

import (
	"sort"
	"strings"
	"time"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"

	"gorm.io/gorm"
)

func (s *PlatformService) GetAlarmDetail(alarmID uint, accessScope *AccessScope) (map[string]any, error) {
	alarm, err := s.ensureAlarmAccessible(accessScope, alarmID)
	if err != nil {
		return nil, err
	}
	items, err := NewQueryService(s.repo).ListAlarms(1, 1000, dto.AlarmListFilter{}, accessScope)
	if err != nil {
		return nil, err
	}
	var base map[string]any
	for _, item := range items.Items {
		if item.ID == alarmID {
			base = dtoAlarmToMap(item)
			break
		}
	}
	if base == nil {
		base = map[string]any{"id": alarm.ID}
	}

	var pushLogs []entity.AlarmPushLog
	var processLogs []entity.AlarmProcessLog
	_ = s.db().Where("alarm_id = ?", alarmID).Order("id DESC").Find(&pushLogs).Error
	_ = s.db().Where("alarm_id = ?", alarmID).Order("id DESC").Find(&processLogs).Error

	base["pushRecords"] = buildPushRecords(pushLogs)
	base["processLogs"] = buildProcessLogs(processLogs)
	base["aiEvent"] = nil
	base["cameraInfo"] = nil
	base["areaInfo"] = nil
	return base, nil
}

func (s *PlatformService) ProcessAlarm(alarmID uint, payload AlarmProcessPayload, operatorName string, operatorID uint, accessScope *AccessScope) (map[string]any, error) {
	alarm, err := s.ensureAlarmAccessible(accessScope, alarmID)
	if err != nil {
		return nil, err
	}
	fromStatus := alarm.Status
	alarm.Status = normalizedStatus(payload.Status, alarm.Status)
	if err := s.db().Save(alarm).Error; err != nil {
		return nil, err
	}
	remark := valueOrEmpty(payload.Remark)
	_ = s.db().Create(&entity.AlarmProcessLog{
		AlarmID:      alarm.ID,
		Action:       "process",
		FromStatus:   fromStatus,
		ToStatus:     alarm.Status,
		OperatorID:   &operatorID,
		OperatorName: operatorName,
		Remark:       remark,
	}).Error
	detail, err := s.GetAlarmDetail(alarmID, accessScope)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *PlatformService) FalseAlarm(alarmID uint, remark string, operatorName string, operatorID uint, accessScope *AccessScope) (map[string]any, error) {
	return s.ProcessAlarm(alarmID, AlarmProcessPayload{Status: "false_alarm", Remark: &remark}, operatorName, operatorID, accessScope)
}

func (s *PlatformService) RePushAlarm(alarmID uint, accessScope *AccessScope) (map[string]any, error) {
	alarm, err := s.ensureAlarmAccessible(accessScope, alarmID)
	if err != nil {
		return nil, err
	}
	logItem := entity.AlarmPushLog{
		AlarmID:      &alarm.ID,
		Channel:      "manual",
		ProviderType: "manual",
		Status:       "success",
		ConfigName:   "manual-repush",
		AlarmNo:      alarm.AlarmNo,
		AlarmType:    alarm.AlarmType,
		AlarmLevel:   alarm.AlarmLevel,
		FactoryID:    alarm.FactoryID,
		ZoneID:       alarm.ZoneID,
		TriggeredBy:  "manual",
		RetryCount:   0,
		Message:      "手动重新推送",
		PushedAt:     time.Now(),
	}
	_ = s.db().Create(&logItem).Error
	detail, err := s.GetAlarmDetail(alarmID, accessScope)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *PlatformService) GetDashboardAlarmTrend(startAt, endAt *time.Time, accessScope *AccessScope) map[string]any {
	type trendRow struct {
		Day   string `gorm:"column:day"`
		Value int64  `gorm:"column:value"`
	}
	rangeStart, rangeEnd := normalizeDashboardRange(startAt, endAt, 7)
	startDay := truncateToDay(rangeStart)
	endDay := truncateToDay(rangeEnd)
	var rows []trendRow
	query := s.db().Table("alarm_record").
		Select("DATE_FORMAT(alarm_time, '%Y-%m-%d') AS day, count(*) AS value").
		Where("alarm_time >= ? AND alarm_time < ?", startDay, endDay.Add(24*time.Hour))
	query = s.applyAlarmAccessScopeQuery(query, "alarm_record", accessScope)
	query = applyOptionalTimeRange(query, "alarm_time", startAt, endAt)
	_ = query.Group("day").Scan(&rows).Error
	countByDay := make(map[string]int, len(rows))
	for _, row := range rows {
		countByDay[row.Day] = int(row.Value)
	}

	categories := []string{}
	seriesData := []int{}
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		categories = append(categories, day.Format("01-02"))
		seriesData = append(seriesData, countByDay[day.Format("2006-01-02")])
	}
	return map[string]any{
		"categories": categories,
		"series":     []map[string]any{{"name": "告警数", "data": seriesData}},
	}
}

func (s *PlatformService) GetDashboardAlarmTypes(startAt, endAt *time.Time, accessScope *AccessScope) map[string]any {
	type row struct {
		Name  string `gorm:"column:alarm_type"`
		Value int64  `gorm:"column:value"`
	}
	var rows []row
	query := s.db().Table("alarm_record").Select("alarm_type, count(*) AS value")
	query = s.applyAlarmAccessScopeQuery(query, "alarm_record", accessScope)
	query = applyOptionalTimeRange(query, "alarm_time", startAt, endAt)
	_ = query.Group("alarm_type").Order("value DESC").Scan(&rows).Error
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"name": row.Name, "value": row.Value})
	}
	return map[string]any{"items": items}
}

func (s *PlatformService) GetDashboardZoneRanking(accessScope *AccessScope) map[string]any {
	return map[string]any{"items": s.GetDashboardZoneRankingPage(nil, nil, 1, 10, accessScope)["items"]}
}

func (s *PlatformService) GetDashboardZoneRankingPage(startAt, endAt *time.Time, page, pageSize int, accessScope *AccessScope) map[string]any {
	type row struct {
		FactoryID     *uint   `gorm:"column:factory_id"`
		FactoryName   *string `gorm:"column:factory_name"`
		ZoneID        *uint   `gorm:"column:zone_id"`
		ZoneName      *string `gorm:"column:zone_name"`
		AlarmCount    int64   `gorm:"column:alarm_count"`
		PendingCount  int64   `gorm:"column:pending_count"`
		CriticalCount int64   `gorm:"column:critical_count"`
	}
	page = maxInt(page, 1)
	pageSize = maxInt(pageSize, 30)
	buildQuery := func() *gorm.DB {
		query := s.db().Table("alarm_record AS a").
			Joins("LEFT JOIN factory_area f ON f.id = a.factory_id").
			Joins("LEFT JOIN factory_zone z ON z.id = a.zone_id")
		query = s.applyAlarmAccessScopeQuery(query, "a", accessScope)
		query = applyOptionalTimeRange(query, "a.alarm_time", startAt, endAt)
		return query.Group("a.factory_id, f.factory_name, a.zone_id, z.zone_name")
	}
	var total int64
	var rows []row
	_ = buildQuery().Select("a.factory_id, f.factory_name, a.zone_id, z.zone_name").Count(&total).Error
	_ = buildQuery().
		Select("a.factory_id, f.factory_name, a.zone_id, z.zone_name, count(*) AS alarm_count, sum(case when a.status = 'pending' then 1 else 0 end) AS pending_count, sum(case when a.alarm_level = 'critical' then 1 else 0 end) AS critical_count").
		Order("alarm_count DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"factoryId":     row.FactoryID,
			"factoryName":   row.FactoryName,
			"zoneId":        row.ZoneID,
			"zoneName":      row.ZoneName,
			"alarmCount":    row.AlarmCount,
			"pendingCount":  row.PendingCount,
			"criticalCount": row.CriticalCount,
		})
	}
	return map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize}
}

func (s *PlatformService) GetDashboardDeviceStatus(accessScope *AccessScope) map[string]any {
	return map[string]any{
		"camera":   s.deviceStatusBlock("camera_device", "camera", accessScope),
		"recorder": s.deviceStatusBlock("recorder_device", "recorder", accessScope),
		"channel":  s.deviceStatusBlock("recorder_channel", "channel", accessScope),
	}
}

func (s *PlatformService) GetAlarmReport(startAt, endAt *time.Time, zonePage, zonePageSize int, accessScope *AccessScope) (map[string]any, error) {
	summary, err := NewQueryService(s.repo).GetDashboardSummary(startAt, endAt, accessScope)
	if err != nil {
		return nil, err
	}
	type statusRow struct {
		Name  string `gorm:"column:status"`
		Value int64  `gorm:"column:value"`
	}
	var statusRows []statusRow
	statusQuery := applyOptionalTimeRange(s.applyAlarmAccessScopeQuery(s.db().Table("alarm_record"), "alarm_record", accessScope), "alarm_time", startAt, endAt)
	_ = statusQuery.Select("status, count(*) AS value").Group("status").Scan(&statusRows).Error
	statusSummary := make([]map[string]any, 0, len(statusRows))
	for _, row := range statusRows {
		statusSummary = append(statusSummary, map[string]any{"name": row.Name, "value": row.Value})
	}
	zoneRanking := s.GetDashboardZoneRankingPage(startAt, endAt, zonePage, zonePageSize, accessScope)
	return map[string]any{
		"summary":       summary,
		"trend":         s.GetDashboardAlarmTrend(startAt, endAt, accessScope),
		"alarmTypes":    s.GetDashboardAlarmTypes(startAt, endAt, accessScope),
		"statusSummary": statusSummary,
		"zoneRanking":   zoneRanking,
	}, nil
}

func (s *PlatformService) GetDeviceReport(startAt, endAt *time.Time, factoryPage, factoryPageSize int, accessScope *AccessScope) map[string]any {
	camera := s.deviceStatusBlock("camera_device", "camera", accessScope)
	recorder := s.deviceStatusBlock("recorder_device", "recorder", accessScope)
	channel := s.deviceStatusBlock("recorder_channel", "channel", accessScope)
	type factoryRow struct {
		FactoryID      uint   `gorm:"column:factory_id"`
		FactoryName    string `gorm:"column:factory_name"`
		CameraTotal    int64  `gorm:"column:camera_total"`
		CameraOnline   int64  `gorm:"column:camera_online"`
		RecorderTotal  int64  `gorm:"column:recorder_total"`
		RecorderOnline int64  `gorm:"column:recorder_online"`
	}
	cameras, _ := NewQueryService(s.repo).ListCameras(CameraListFilter{AccessScope: accessScope})
	recorders, _ := NewQueryService(s.repo).ListRecorders(RecorderListFilter{AccessScope: accessScope})
	factoryMap := make(map[uint]*factoryRow)
	for _, item := range cameras {
		row := factoryMap[item.FactoryID]
		if row == nil {
			row = &factoryRow{FactoryID: item.FactoryID, FactoryName: item.FactoryName}
			factoryMap[item.FactoryID] = row
		}
		row.CameraTotal++
		if item.Status == "online" {
			row.CameraOnline++
		}
	}
	for _, item := range recorders {
		row := factoryMap[item.FactoryID]
		if row == nil {
			row = &factoryRow{FactoryID: item.FactoryID, FactoryName: item.FactoryName}
			factoryMap[item.FactoryID] = row
		}
		row.RecorderTotal++
		if item.Status == "online" {
			row.RecorderOnline++
		}
	}
	rows := make([]factoryRow, 0, len(factoryMap))
	for _, row := range factoryMap {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].FactoryID < rows[j].FactoryID
	})
	stats := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, map[string]any{
			"factoryId":      row.FactoryID,
			"factoryName":    row.FactoryName,
			"cameraTotal":    row.CameraTotal,
			"cameraOnline":   row.CameraOnline,
			"recorderTotal":  row.RecorderTotal,
			"recorderOnline": row.RecorderOnline,
		})
	}
	factoryPage = maxInt(factoryPage, 1)
	factoryPageSize = maxInt(factoryPageSize, 30)
	total := len(stats)
	startIndex := (factoryPage - 1) * factoryPageSize
	if startIndex > total {
		startIndex = total
	}
	endIndex := startIndex + factoryPageSize
	if endIndex > total {
		endIndex = total
	}
	return map[string]any{
		"cameraStatus":   camera,
		"recorderStatus": recorder,
		"channelStatus":  channel,
		"statusTrend":    s.GetDashboardAlarmTrend(startAt, endAt, accessScope),
		"factoryStats":   map[string]any{"items": stats[startIndex:endIndex], "total": total, "page": factoryPage, "pageSize": factoryPageSize},
	}
}

func (s *PlatformService) GetPushReport(startAt, endAt *time.Time, accessScope *AccessScope) map[string]any {
	type overviewRow struct {
		Total       int64 `gorm:"column:total"`
		Success     int64 `gorm:"column:success"`
		Failed      int64 `gorm:"column:failed"`
		RateLimited int64 `gorm:"column:rate_limited"`
	}
	var statusRows []nameValueRow
	var channelRows []nameValueRow
	var overview overviewRow
	pushLogTable := func() *gorm.DB {
		return applyOptionalTimeRange(s.applyPushLogAccessScopeQuery(s.db().Table("alarm_push_log"), "alarm_push_log", accessScope), "pushed_at", startAt, endAt)
	}
	_ = pushLogTable().
		Select("count(*) AS total, coalesce(sum(case when status = 'success' then 1 else 0 end), 0) AS success, coalesce(sum(case when status = 'failed' then 1 else 0 end), 0) AS failed, coalesce(sum(case when status = 'rate_limited' then 1 else 0 end), 0) AS rate_limited").
		Scan(&overview).Error
	_ = pushLogTable().Select("status AS name, count(*) AS value").Group("status").Scan(&statusRows).Error
	_ = pushLogTable().Select("channel AS name, count(*) AS value").Group("channel").Scan(&channelRows).Error
	return map[string]any{
		"overview": map[string]any{
			"total": overview.Total, "success": overview.Success, "failed": overview.Failed, "rateLimited": overview.RateLimited,
			"successRate": percent(overview.Success, overview.Total),
		},
		"channelDistribution": map[string]any{"items": rowsToItems(channelRows)},
		"statusDistribution":  map[string]any{"items": rowsToItems(statusRows)},
		"trend":               s.GetDashboardAlarmTrend(startAt, endAt, accessScope),
	}
}

func (s *PlatformService) ListPushConfigs(filter PushConfigListFilter) ([]map[string]any, error) {
	var items []entity.PushConfig
	query := s.db().Order("id DESC")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("(config_name LIKE ? OR webhook LIKE ? OR app_id LIKE ? OR template_id LIKE ?)", likeKeyword, likeKeyword, likeKeyword, likeKeyword)
	}
	if filter.ProviderType != "" {
		query = query.Where("provider_type = ?", filter.ProviderType)
	}
	if filter.Enabled != nil {
		query = query.Where("enabled = ?", *filter.Enabled)
	}
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if !s.canAccessPushConfig(item, filter.AccessScope) {
			continue
		}
		result = append(result, pushConfigToMap(item))
	}
	return result, nil
}

func (s *PlatformService) CreatePushConfig(payload PushConfigPayload, accessScope *AccessScope) (map[string]any, error) {
	if err := normalizePushConfigPayload(&payload); err != nil {
		return nil, err
	}
	if err := s.validatePushConfigScope(payload.FactoryIDs, payload.ZoneIDs, accessScope); err != nil {
		return nil, err
	}
	item := entity.PushConfig{
		ConfigName:             payload.ConfigName,
		ProviderType:           payload.ProviderType,
		Webhook:                valueOrEmpty(payload.Webhook),
		SecretEncrypted:        valueOrEmpty(payload.Secret),
		AppID:                  valueOrEmpty(payload.AppID),
		AppSecretEncrypted:     valueOrEmpty(payload.AppSecret),
		TemplateID:             valueOrEmpty(payload.TemplateID),
		ReceiverOpenIDsJSON:    encodeJSON(payload.ReceiverOpenIDs),
		FactoryIDsJSON:         encodeJSON(payload.FactoryIDs),
		ZoneIDsJSON:            encodeJSON(payload.ZoneIDs),
		AlarmTypesJSON:         encodeJSON(payload.AlarmTypes),
		AlarmLevelsJSON:        encodeJSON(payload.AlarmLevels),
		ActiveTimeRangesJSON:   encodeJSON(payload.ActiveTimeRanges),
		Enabled:                payload.Enabled,
		RateLimitWindowSeconds: payload.RateLimitWindowSeconds,
		RateLimitMaxCount:      payload.RateLimitMaxCount,
		RetryMaxCount:          payload.RetryMaxCount,
		RetryIntervalSeconds:   payload.RetryIntervalSeconds,
		Remark:                 valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return pushConfigToMap(item), nil
}

func (s *PlatformService) UpdatePushConfig(id uint, payload PushConfigPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensurePushConfigAccessible(id, accessScope)
	if err != nil {
		return nil, err
	}
	if err := normalizePushConfigPayload(&payload); err != nil {
		return nil, err
	}
	if err := s.validatePushConfigScope(payload.FactoryIDs, payload.ZoneIDs, accessScope); err != nil {
		return nil, err
	}
	item.ConfigName = payload.ConfigName
	item.ProviderType = payload.ProviderType
	item.Webhook = valueOrEmpty(payload.Webhook)
	if payload.Secret != nil {
		item.SecretEncrypted = valueOrEmpty(payload.Secret)
	}
	item.AppID = valueOrEmpty(payload.AppID)
	if payload.AppSecret != nil {
		item.AppSecretEncrypted = valueOrEmpty(payload.AppSecret)
	}
	item.TemplateID = valueOrEmpty(payload.TemplateID)
	item.ReceiverOpenIDsJSON = encodeJSON(payload.ReceiverOpenIDs)
	item.FactoryIDsJSON = encodeJSON(payload.FactoryIDs)
	item.ZoneIDsJSON = encodeJSON(payload.ZoneIDs)
	item.AlarmTypesJSON = encodeJSON(payload.AlarmTypes)
	item.AlarmLevelsJSON = encodeJSON(payload.AlarmLevels)
	item.ActiveTimeRangesJSON = encodeJSON(payload.ActiveTimeRanges)
	item.Enabled = payload.Enabled
	item.RateLimitWindowSeconds = payload.RateLimitWindowSeconds
	item.RateLimitMaxCount = payload.RateLimitMaxCount
	item.RetryMaxCount = payload.RetryMaxCount
	item.RetryIntervalSeconds = payload.RetryIntervalSeconds
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return pushConfigToMap(*item), nil
}

func (s *PlatformService) UpdatePushConfigStatus(id uint, enabled bool, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensurePushConfigAccessible(id, accessScope)
	if err != nil {
		return nil, err
	}
	item.Enabled = enabled
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return pushConfigToMap(*item), nil
}

func (s *PlatformService) DeletePushConfig(id uint, accessScope *AccessScope) error {
	if _, err := s.ensurePushConfigAccessible(id, accessScope); err != nil {
		return err
	}
	return s.db().Delete(&entity.PushConfig{}, id).Error
}

func (s *PlatformService) TestPushConfig(id uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensurePushConfigAccessible(id, accessScope)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	switch item.ProviderType {
	case "dingtalk", "wechat", "email":
		var result pushDeliveryResult
		if item.ProviderType == "dingtalk" {
			result = deliverTestDingtalkPush(*item, now)
		} else if item.ProviderType == "wechat" {
			result = deliverTestWechatPush(*item, now)
		} else {
			result = deliverTestEmailPush(s.cfg, *item, now)
		}
		logItem := entity.AlarmPushLog{
			PushConfigID: &item.ID,
			Channel:      item.ProviderType,
			ProviderType: item.ProviderType,
			Status:       result.Status,
			ConfigName:   item.ConfigName,
			TriggeredBy:  "test",
			RetryCount:   0,
			Message:      result.Message,
			RequestBody:  result.RequestBody,
			ResponseBody: result.ResponseBody,
			ErrorMessage: result.ErrorMessage,
			PushedAt:     now,
		}
		_ = s.db().Create(&logItem).Error
		return map[string]any{
			"success":  result.Status == "success",
			"status":   result.Status,
			"message":  result.Message,
			"pushedAt": now.Format(time.RFC3339),
		}, nil
	}
	return map[string]any{"success": true, "status": "success", "message": "测试推送成功", "pushedAt": now.Format(time.RFC3339)}, nil
}

func (s *PlatformService) ListPushLogs(page, pageSize int, filter PushLogListFilter) (map[string]any, error) {
	query := s.applyPushLogAccessScopeQuery(s.db().Model(&entity.AlarmPushLog{}), "alarm_push_log", filter.AccessScope)
	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.AlarmType != "" {
		query = query.Where("alarm_type = ?", filter.AlarmType)
	}
	if filter.StartAt != nil {
		query = query.Where("pushed_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("pushed_at <= ?", *filter.EndAt)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []entity.AlarmPushLog
	if err := query.Order("pushed_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, pushLogToMap(item))
	}
	return map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize}, nil
}

func (s *PlatformService) RetryPushLog(id uint, accessScope *AccessScope) (map[string]any, error) {
	var item entity.AlarmPushLog
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	if accessScope != nil && !s.canAccessPushLog(item, accessScope) {
		return nil, ErrAccessDenied
	}
	item.Status = "success"
	item.RetryCount += 1
	item.Message = "重试后推送成功"
	item.PushedAt = time.Now()
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return pushLogToMap(item), nil
}
