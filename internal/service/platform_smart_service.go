package service

import (
	"fmt"
	"strings"
	"time"

	"secmgmt_go/internal/domain/entity"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *PlatformService) ListSmartProviders() ([]map[string]any, error) {
	var rows []smartProviderListRow
	if err := s.db().Table("smart_interface_provider AS p").
		Joins("LEFT JOIN smart_device_binding b ON b.provider_id = p.id").
		Joins("LEFT JOIN smart_interface_capability c ON c.id = b.capability_id").
		Select(`p.id, p.provider_code, p.provider_name, p.provider_type, p.auth_type, p.base_url,
			p.callback_path, p.secret_encrypted, p.config_schema_json, p.enabled, p.remark, p.created_at, p.updated_at,
			COALESCE(GROUP_CONCAT(DISTINCT c.capability_code ORDER BY c.id SEPARATOR ','), '') AS capability_codes,
			COALESCE(GROUP_CONCAT(DISTINCT c.capability_name ORDER BY c.id SEPARATOR ','), '') AS capability_names`).
		Group(`p.id, p.provider_code, p.provider_name, p.provider_type, p.auth_type, p.base_url,
			p.callback_path, p.secret_encrypted, p.config_schema_json, p.enabled, p.remark, p.created_at, p.updated_at`).
		Order("p.id DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, smartProviderListMap(row))
	}
	return result, nil
}

type smartProviderListRow struct {
	ID               uint
	ProviderCode     string
	ProviderName     string
	ProviderType     string
	AuthType         string
	BaseURL          string
	CallbackPath     string
	SecretEncrypted  string
	ConfigSchemaJSON string
	Enabled          bool
	Remark           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CapabilityCodes  string
	CapabilityNames  string
}

func smartProviderListMap(item smartProviderListRow) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"providerCode":     item.ProviderCode,
		"providerName":     item.ProviderName,
		"providerType":     item.ProviderType,
		"authType":         item.AuthType,
		"baseUrl":          nullableString(item.BaseURL),
		"callbackPath":     nullableString(item.CallbackPath),
		"enabled":          item.Enabled,
		"remark":           nullableString(item.Remark),
		"configSchema":     decodeJSONAny(item.ConfigSchemaJSON),
		"secretConfigured": item.SecretEncrypted != "",
		"capabilityCodes":  splitCommaValues(item.CapabilityCodes),
		"capabilityNames":  splitCommaValues(item.CapabilityNames),
		"updatedAt":        item.UpdatedAt.Format(time.RFC3339),
		"createdAt":        item.CreatedAt.Format(time.RFC3339),
	}
}

func splitCommaValues(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (s *PlatformService) CreateSmartProvider(payload SmartProviderPayload) (map[string]any, error) {
	item := entity.SmartInterfaceProvider{
		ProviderCode:     payload.ProviderCode,
		ProviderName:     payload.ProviderName,
		ProviderType:     payload.ProviderType,
		AuthType:         payload.AuthType,
		BaseURL:          valueOrEmpty(payload.BaseURL),
		CallbackPath:     valueOrEmpty(payload.CallbackPath),
		SecretEncrypted:  valueOrEmpty(payload.Secret),
		ConfigSchemaJSON: encodeJSON(payload.ConfigSchema),
		Enabled:          payload.Enabled,
		Remark:           valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	s.reloadHikvisionBridgeForProvider(item.ProviderCode, "create-smart-provider")
	return s.smartProviderMap(item), nil
}

func (s *PlatformService) UpdateSmartProvider(id uint, payload SmartProviderPayload) (map[string]any, error) {
	var item entity.SmartInterfaceProvider
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	previousProviderCode := item.ProviderCode
	item.ProviderCode = payload.ProviderCode
	item.ProviderName = payload.ProviderName
	item.ProviderType = payload.ProviderType
	item.AuthType = payload.AuthType
	item.BaseURL = valueOrEmpty(payload.BaseURL)
	item.CallbackPath = valueOrEmpty(payload.CallbackPath)
	if payload.Secret != nil {
		item.SecretEncrypted = valueOrEmpty(payload.Secret)
	}
	item.ConfigSchemaJSON = encodeJSON(payload.ConfigSchema)
	item.Enabled = payload.Enabled
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	switch {
	case shouldReloadHikvisionProvider(previousProviderCode) && shouldReloadHikvisionProvider(item.ProviderCode):
		s.reloadHikvisionBridgeForProvider(item.ProviderCode, "update-smart-provider")
	case shouldReloadHikvisionProvider(previousProviderCode):
		s.reloadHikvisionBridgeForProvider(previousProviderCode, "update-smart-provider-old")
	case shouldReloadHikvisionProvider(item.ProviderCode):
		s.reloadHikvisionBridgeForProvider(item.ProviderCode, "update-smart-provider-new")
	}
	return s.smartProviderMap(item), nil
}

func (s *PlatformService) TestSmartProvider(id uint) (map[string]any, error) {
	var provider entity.SmartInterfaceProvider
	if err := s.db().First(&provider, id).Error; err != nil {
		return nil, err
	}
	checkedAt := time.Now().Format(time.RFC3339)
	if shouldReloadHikvisionProvider(provider.ProviderCode) {
		status := map[string]any{
			"running":             false,
			"sessionCount":        0,
			"bindingCount":        0,
			"skippedBindingCount": 0,
			"mergedBindingCount":  0,
			"lastError":           "hikvision bridge is not attached",
			"sessions":            []map[string]any{},
		}
		if s.hikvisionBridge != nil {
			status = s.hikvisionBridge.RuntimeStatus()
		}
		success, message := hikvisionProviderRuntimeState(provider.ProviderCode, status)
		if s.logger != nil {
			s.logger.Info("smart provider test",
				zap.String("providerCode", provider.ProviderCode),
				zap.Bool("success", success),
				zap.Any("hikvisionBridgeStatus", status),
			)
		}
		return map[string]any{
			"success":   success,
			"message":   message,
			"checkedAt": checkedAt,
			"status":    status,
		}, nil
	}
	if s.logger != nil {
		s.logger.Info("smart provider test",
			zap.String("providerCode", provider.ProviderCode),
			zap.Bool("success", true),
		)
	}
	return map[string]any{"success": true, "message": "Provider test succeeded", "checkedAt": checkedAt}, nil
}

func (s *PlatformService) GetSmartBridgeStatus() (map[string]any, error) {
	bridgeStatus := map[string]any{
		"running":             false,
		"sessionCount":        0,
		"bindingCount":        0,
		"skippedBindingCount": 0,
		"mergedBindingCount":  0,
		"lastError":           "hikvision bridge is not attached",
		"sessions":            []map[string]any{},
	}
	if s.hikvisionBridge != nil {
		bridgeStatus = s.hikvisionBridge.RuntimeStatus()
	}
	reconnectStatus := map[string]any{"taskCount": 0, "tasks": []map[string]any{}}
	if s.smartBridgeReconnect != nil {
		reconnectStatus = s.smartBridgeReconnect.RuntimeStatus()
	}
	return map[string]any{
		"bridge":    bridgeStatus,
		"reconnect": reconnectStatus,
	}, nil
}

func (s *PlatformService) ReconnectSmartBinding(id uint) (map[string]any, error) {
	if s.smartBridgeReconnect == nil {
		return nil, fmt.Errorf("smart bridge reconnect service is not attached")
	}
	return s.smartBridgeReconnect.ReconnectBindingNow(id)
}

func (s *PlatformService) ReloadSmartBinding(id uint) (map[string]any, error) {
	var binding entity.SmartDeviceBinding
	if err := s.db().First(&binding, id).Error; err != nil {
		return nil, err
	}
	var provider entity.SmartInterfaceProvider
	if err := s.db().First(&provider, binding.ProviderID).Error; err != nil {
		return nil, err
	}
	var capability entity.SmartInterfaceCapability
	if err := s.db().First(&capability, binding.CapabilityID).Error; err != nil {
		return nil, err
	}
	if !shouldReloadHikvisionBinding(provider.ProviderCode, capability.CapabilityCode) {
		return nil, fmt.Errorf("当前绑定不是海康移动侦测绑定，无法重启移动侦测接口")
	}
	if !binding.Enabled || !provider.Enabled || !capability.Enabled {
		return nil, fmt.Errorf("当前绑定/提供方/能力未启用，无法重启移动侦测接口")
	}
	s.reloadHikvisionBridgeForBinding(provider.ProviderCode, capability.CapabilityCode, "manual-smart-binding-reload")
	return map[string]any{
		"reloaded":       true,
		"bindingId":      binding.ID,
		"providerCode":   provider.ProviderCode,
		"capabilityCode": capability.CapabilityCode,
		"message":        "移动侦测接口已提交重启",
	}, nil
}

func (s *PlatformService) TestSmartBinding(id uint) (map[string]any, error) {
	var binding entity.SmartDeviceBinding
	if err := s.db().First(&binding, id).Error; err != nil {
		return nil, err
	}

	var provider entity.SmartInterfaceProvider
	if err := s.db().First(&provider, binding.ProviderID).Error; err != nil {
		return nil, err
	}

	var capability entity.SmartInterfaceCapability
	if err := s.db().First(&capability, binding.CapabilityID).Error; err != nil {
		return nil, err
	}

	sourceName, sourcePath := s.resolveSourceName(binding.SourceType, binding.SourceID)
	checkedAt := time.Now().Format(time.RFC3339)

	providerResult, err := s.TestSmartProvider(provider.ID)
	if err != nil {
		return nil, err
	}
	providerSuccess, _ := providerResult["success"].(bool)
	providerMessage := fmt.Sprint(providerResult["message"])

	deviceResult, err := s.testSmartBindingDevice(binding)
	if err != nil {
		return nil, err
	}
	deviceSuccess, _ := deviceResult["success"].(bool)
	deviceMessage := fmt.Sprint(deviceResult["message"])
	runtimeResult := s.inspectSmartBindingRuntime(binding, provider, capability)
	runtimeSuccess, _ := runtimeResult["success"].(bool)
	ruleSummary := s.summarizeSmartBindingRules(binding.ID)
	ruleSuccess, _ := ruleSummary["success"].(bool)
	latestEvent := s.latestSmartBindingEvent(binding.ID)
	latestAlarm := s.latestSmartBindingAlarm(binding.ID)
	latestEventFound, _ := latestEvent["found"].(bool)
	latestAlarmFound, _ := latestAlarm["found"].(bool)
	alarmEnabledRuleCount, _ := ruleSummary["alarmEnabledRuleCount"].(int)

	issues := make([]string, 0, 6)
	if !binding.Enabled {
		issues = append(issues, "绑定已停用")
	}
	if !provider.Enabled {
		issues = append(issues, "接口提供方已停用")
	}
	if !capability.Enabled {
		issues = append(issues, "能力已停用")
	}
	if !deviceSuccess {
		issues = append(issues, "绑定设备检测异常")
	}
	if !providerSuccess {
		issues = append(issues, "接口检测异常")
	}
	if !runtimeSuccess {
		issues = append(issues, "运行链路未就绪")
	}
	if !ruleSuccess {
		issues = append(issues, "未配置启用规则")
	}

	observation := make([]string, 0, 2)
	if !latestEventFound {
		observation = append(observation, "暂未发现历史事件")
	}
	if alarmEnabledRuleCount > 0 && !latestAlarmFound {
		observation = append(observation, "暂未发现历史告警")
	}

	message := fmt.Sprintf("绑定自检通过：%s 与 %s 链路正常", sourceName, provider.ProviderName)
	if len(issues) > 0 {
		message = "绑定自检未通过：" + strings.Join(issues, "，")
	} else if len(observation) > 0 {
		message = message + "，" + strings.Join(observation, "，")
	}

	return map[string]any{
		"success":         len(issues) == 0,
		"message":         message,
		"checkedAt":       checkedAt,
		"bindingEnabled":  binding.Enabled,
		"providerEnabled": provider.Enabled,
		"capabilityCode":  capability.CapabilityCode,
		"capabilityName":  capability.CapabilityName,
		"provider": map[string]any{
			"id":           provider.ID,
			"providerCode": provider.ProviderCode,
			"providerName": provider.ProviderName,
			"success":      providerSuccess,
			"message":      providerMessage,
		},
		"device": map[string]any{
			"sourceType": binding.SourceType,
			"sourceId":   binding.SourceID,
			"sourceName": sourceName,
			"sourcePath": sourcePath,
			"success":    deviceSuccess,
			"message":    deviceMessage,
			"detail":     deviceResult,
		},
		"runtime":     runtimeResult,
		"rules":       ruleSummary,
		"latestEvent": latestEvent,
		"latestAlarm": latestAlarm,
	}, nil
}

func (s *PlatformService) ListSmartCapabilities() ([]map[string]any, error) {
	var items []entity.SmartInterfaceCapability
	if err := s.db().Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":               item.ID,
			"capabilityCode":   item.CapabilityCode,
			"capabilityName":   item.CapabilityName,
			"eventCategory":    item.EventCategory,
			"supportsPush":     item.SupportsPush,
			"supportsPull":     item.SupportsPull,
			"supportsAiReview": item.SupportsAIReview,
			"payloadSchema":    decodeJSONAny(item.PayloadSchemaJSON),
			"defaultRule":      decodeJSONAny(item.DefaultRuleJSON),
			"enabled":          item.Enabled,
			"createdAt":        item.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (s *PlatformService) testSmartBindingDevice(binding entity.SmartDeviceBinding) (map[string]any, error) {
	sourceName, sourcePath := s.resolveSourceName(binding.SourceType, binding.SourceID)

	switch binding.SourceType {
	case "camera":
		result, err := s.TestCameraConnection(binding.SourceID, nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"targetDeviceType": "camera",
			"targetDeviceId":   binding.SourceID,
			"sourceName":       sourceName,
			"sourcePath":       sourcePath,
			"success":          result["success"],
			"status":           result["status"],
			"message":          result["message"],
		}, nil
	case "recorder":
		result, err := s.TestRecorderConnection(binding.SourceID, nil)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"targetDeviceType": "recorder",
			"targetDeviceId":   binding.SourceID,
			"sourceName":       sourceName,
			"sourcePath":       sourcePath,
			"success":          result["success"],
			"status":           result["status"],
			"message":          result["message"],
		}, nil
	case "channel":
		var channel entity.RecorderChannel
		if err := s.db().First(&channel, binding.SourceID).Error; err != nil {
			return nil, err
		}
		result, err := s.TestRecorderConnection(channel.RecorderID, nil)
		if err != nil {
			return nil, err
		}
		recorderSuccess, _ := result["success"].(bool)
		success := recorderSuccess && channel.Enabled
		message := "通道所属录像机连接测试成功"
		if !channel.Enabled {
			message = "通道已停用"
		} else if text := fmt.Sprint(result["message"]); text != "" {
			message = text
		}
		return map[string]any{
			"targetDeviceType": "recorder",
			"targetDeviceId":   channel.RecorderID,
			"channelId":        channel.ID,
			"channelNo":        channel.ChannelNo,
			"channelEnabled":   channel.Enabled,
			"sourceName":       sourceName,
			"sourcePath":       sourcePath,
			"success":          success,
			"status":           result["status"],
			"message":          message,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported binding source type %q", binding.SourceType)
	}
}

func (s *PlatformService) inspectSmartBindingRuntime(binding entity.SmartDeviceBinding, provider entity.SmartInterfaceProvider, capability entity.SmartInterfaceCapability) map[string]any {
	if !shouldReloadHikvisionBinding(provider.ProviderCode, capability.CapabilityCode) {
		return map[string]any{
			"supported": false,
			"success":   true,
			"message":   "当前绑定无需执行 SDK bridge 运行态检查",
		}
	}

	status := map[string]any{
		"running":             false,
		"sessionCount":        0,
		"bindingCount":        0,
		"skippedBindingCount": 0,
		"mergedBindingCount":  0,
		"lastError":           "hikvision bridge is not attached",
		"sessions":            []map[string]any{},
	}
	if s.hikvisionBridge != nil {
		status = s.hikvisionBridge.RuntimeStatus()
	}

	running, _ := status["running"].(bool)
	bindingIncluded := binding.Enabled && provider.Enabled && capability.Enabled
	result := map[string]any{
		"supported":       true,
		"success":         false,
		"message":         "Hikvision SDK bridge is not attached",
		"bindingIncluded": bindingIncluded,
		"running":         running,
		"sessionFound":    false,
		"sessionKey":      nil,
		"deviceType":      nil,
		"deviceId":        nil,
		"deviceIp":        nil,
		"lastError":       fmt.Sprint(status["lastError"]),
		"status":          status,
	}
	if s.hikvisionBridge == nil {
		return result
	}
	target, ok, err := s.hikvisionBridge.bindingToTargetForProvider(provider.ProviderCode, binding)
	if err != nil {
		result["message"] = fmt.Sprintf("bridge 目标解析失败：%v", err)
		return result
	}
	if !ok {
		result["message"] = "当前绑定未能解析为有效的 bridge 监听目标"
		return result
	}

	result["sessionKey"] = target.SessionKey
	result["deviceType"] = target.DeviceType
	result["deviceId"] = target.DeviceID
	result["deviceIp"] = target.DeviceIP

	session, found := findBridgeSession(status["sessions"], target.SessionKey)
	result["sessionFound"] = found
	if found {
		result["session"] = session
	}
	sessionConnected := provider.ProviderCode != "hikvision-isapi"
	if provider.ProviderCode == "hikvision-isapi" && found {
		sessionConnected, _ = session["connected"].(bool)
	}
	switch {
	case !bindingIncluded:
		result["message"] = "当前绑定未处于可监听状态"
	case !running:
		result["message"] = "Hikvision SDK bridge 未运行"
	case !found:
		result["message"] = fmt.Sprintf("bridge 已运行，但未命中当前绑定会话 %s", target.SessionKey)
	case !sessionConnected:
		result["message"] = fmt.Sprintf("ISAPI alertStream 会话已创建但未连接成功：%s", fmt.Sprint(session["lastError"]))
	default:
		result["success"] = true
		result["message"] = fmt.Sprintf("bridge 已命中当前绑定会话 %s", target.SessionKey)
	}
	return result
}

func (s *PlatformService) summarizeSmartBindingRules(bindingID uint) map[string]any {
	var rules []entity.SmartBindingRule
	_ = s.db().Where("binding_id = ?", bindingID).Find(&rules).Error

	enabledRuleCount := 0
	alarmEnabledRuleCount := 0
	directAlarmRuleCount := 0
	sendToAIRuleCount := 0
	for _, item := range rules {
		if item.Enabled {
			enabledRuleCount++
		}
		if item.Enabled && item.AlarmEnabled {
			alarmEnabledRuleCount++
		}
		if item.Enabled && item.AlarmEnabled && item.GenerateAlarmDirectly {
			directAlarmRuleCount++
		}
		if item.Enabled && item.SendToAI {
			sendToAIRuleCount++
		}
	}

	message := fmt.Sprintf("共 %d 条规则，已启用 %d 条", len(rules), enabledRuleCount)
	if enabledRuleCount == 0 {
		message = "未配置启用规则"
	}
	return map[string]any{
		"success":               enabledRuleCount > 0,
		"message":               message,
		"ruleCount":             len(rules),
		"enabledRuleCount":      enabledRuleCount,
		"alarmEnabledRuleCount": alarmEnabledRuleCount,
		"directAlarmRuleCount":  directAlarmRuleCount,
		"sendToAiRuleCount":     sendToAIRuleCount,
	}
}

func (s *PlatformService) latestSmartBindingEvent(bindingID uint) map[string]any {
	var item entity.SmartEvent
	if err := s.db().Where("binding_id = ?", bindingID).Order("event_time DESC").First(&item).Error; err != nil {
		return map[string]any{
			"found":   false,
			"message": "暂无历史事件",
		}
	}
	return map[string]any{
		"found":       true,
		"id":          item.ID,
		"code":        item.EventCode,
		"time":        item.EventTime.Format(time.RFC3339),
		"eventType":   item.EventType,
		"eventLevel":  item.EventLevel,
		"sourceStage": item.SourceStage,
		"status":      item.Status,
		"ageSeconds":  ageSeconds(item.EventTime),
		"message":     fmt.Sprintf("最近事件 %s", item.EventCode),
	}
}

func (s *PlatformService) latestSmartBindingAlarm(bindingID uint) map[string]any {
	var item entity.AlarmRecord
	err := s.db().
		Table("alarm_record AS a").
		Select("a.*").
		Joins("JOIN smart_event e ON e.id = a.smart_event_id").
		Where("e.binding_id = ?", bindingID).
		Order("a.alarm_time DESC").
		First(&item).Error
	if err != nil {
		return map[string]any{
			"found":   false,
			"message": "暂无历史告警",
		}
	}
	return map[string]any{
		"found":      true,
		"id":         item.ID,
		"code":       item.AlarmNo,
		"time":       item.AlarmTime.Format(time.RFC3339),
		"alarmType":  item.AlarmType,
		"alarmLevel": item.AlarmLevel,
		"status":     item.Status,
		"ageSeconds": ageSeconds(item.AlarmTime),
		"message":    fmt.Sprintf("最近告警 %s", item.AlarmNo),
	}
}

func hikvisionProviderRuntimeState(providerCode string, status map[string]any) (bool, string) {
	switch providerCode {
	case "hikvision-isapi":
		connected := intFromAny(status["isapiConnectedSessionCount"])
		receiving := intFromAny(status["isapiReceivingSessionCount"])
		total := intFromAny(status["isapiSessionCount"])
		if connected > 0 {
			if receiving == 0 {
				return true, fmt.Sprintf("Hikvision ISAPI alertStream is connected (%d/%d), but no stream payload has been received yet", connected, total)
			}
			return true, fmt.Sprintf("Hikvision ISAPI alertStream is connected (%d/%d)", connected, total)
		}
		if total > 0 {
			return false, fmt.Sprintf("Hikvision ISAPI alertStream is not connected (0/%d): %s", total, fmt.Sprint(status["lastError"]))
		}
		return false, "Hikvision ISAPI alertStream has no listening sessions"
	case "hikvision-sdk":
		count := intFromAny(status["sdkSessionCount"])
		if count > 0 {
			return true, fmt.Sprintf("Hikvision SDK bridge is running (%d sessions)", count)
		}
		return false, fmt.Sprintf("Hikvision SDK bridge is not running: %s", fmt.Sprint(status["lastError"]))
	default:
		running, _ := status["running"].(bool)
		if running {
			return true, "Hikvision bridge is running"
		}
		return false, fmt.Sprintf("Hikvision bridge is not running: %s", fmt.Sprint(status["lastError"]))
	}
}

func intFromAny(value any) int {
	switch item := value.(type) {
	case int:
		return item
	case int64:
		return int(item)
	case float64:
		return int(item)
	case uint:
		return int(item)
	case uint64:
		return int(item)
	default:
		return 0
	}
}

func findBridgeSession(raw any, sessionKey string) (map[string]any, bool) {
	switch typed := raw.(type) {
	case []map[string]any:
		for _, item := range typed {
			if fmt.Sprint(item["sessionKey"]) == sessionKey {
				return item, true
			}
		}
	case []any:
		for _, item := range typed {
			if session, ok := item.(map[string]any); ok && fmt.Sprint(session["sessionKey"]) == sessionKey {
				return session, true
			}
		}
	}
	return nil, false
}

type smartBindingListRow struct {
	ID                   uint
	ProviderID           uint
	CapabilityID         uint
	SourceType           string
	SourceID             uint
	Enabled              bool
	Priority             int
	ConnectionConfigJSON string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	ProviderCode         string
	ProviderName         string
	CapabilityCode       string
	CapabilityName       string
	RuleCount            int
	SourceName           string
	SourcePath           string
	LastEventTime        *time.Time
}

func (s *PlatformService) ListSmartBindings(page, pageSize int, filter SmartBindingListFilter) (map[string]any, error) {
	page = maxInt(page, 1)
	pageSize = maxInt(pageSize, 20)
	query := s.db().Table("smart_device_binding AS b").
		Joins("LEFT JOIN smart_interface_provider p ON p.id = b.provider_id").
		Joins("LEFT JOIN smart_interface_capability c ON c.id = b.capability_id")
	if filter.SourceType != "" {
		query = query.Where("b.source_type = ?", filter.SourceType)
	}
	if filter.ProviderCode != "" {
		query = query.Where("p.provider_code = ?", filter.ProviderCode)
	}
	if filter.CapabilityCode != "" {
		query = query.Where("c.capability_code = ?", filter.CapabilityCode)
	}
	if filter.Enabled != nil {
		query = query.Where("b.enabled = ?", *filter.Enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []smartBindingListRow
	if err := query.
		Joins("LEFT JOIN (SELECT binding_id, COUNT(*) AS rule_count FROM smart_binding_rule GROUP BY binding_id) r ON r.binding_id = b.id").
		Joins("LEFT JOIN (SELECT binding_id, MAX(event_time) AS last_event_time FROM smart_event GROUP BY binding_id) le ON le.binding_id = b.id").
		Joins("LEFT JOIN camera_device cam ON b.source_type = ? AND cam.id = b.source_id", "camera").
		Joins("LEFT JOIN recorder_device rec ON b.source_type = ? AND rec.id = b.source_id", "recorder").
		Joins("LEFT JOIN recorder_channel ch ON b.source_type = ? AND ch.id = b.source_id", "channel").
		Select(`b.id, b.provider_id, b.capability_id, b.source_type, b.source_id, b.enabled, b.priority,
			b.connection_config_json, b.created_at, b.updated_at,
			p.provider_code, p.provider_name, c.capability_code, c.capability_name,
			COALESCE(r.rule_count, 0) AS rule_count,
			COALESCE(cam.name, rec.name, ch.name, CONCAT(b.source_type, '-', b.source_id)) AS source_name,
			CONCAT(b.source_type, '/', b.source_id) AS source_path,
			le.last_event_time`).
		Order("b.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		result = append(result, smartBindingListMap(row))
	}
	return map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize}, nil
}

func smartBindingListMap(item smartBindingListRow) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"providerId":            item.ProviderID,
		"providerCode":          item.ProviderCode,
		"providerName":          item.ProviderName,
		"capabilityId":          item.CapabilityID,
		"capabilityCode":        item.CapabilityCode,
		"capabilityName":        item.CapabilityName,
		"sourceType":            item.SourceType,
		"sourceId":              item.SourceID,
		"sourceName":            item.SourceName,
		"sourcePath":            item.SourcePath,
		"enabled":               item.Enabled,
		"priority":              item.Priority,
		"connectionConfig":      decodeJSONAny(item.ConnectionConfigJSON),
		"sendToAi":              false,
		"generateAlarmDirectly": true,
		"ruleCount":             item.RuleCount,
		"lastEventTime":         timePtrToRFC3339(item.LastEventTime),
		"updatedAt":             item.UpdatedAt.Format(time.RFC3339),
		"createdAt":             item.CreatedAt.Format(time.RFC3339),
	}
}

func (s *PlatformService) CreateSmartBinding(payload SmartBindingPayload) (map[string]any, error) {
	providerID, capabilityID, err := s.resolveProviderCapabilityIDs(payload.ProviderCode, payload.CapabilityCode)
	if err != nil {
		return nil, err
	}
	item := entity.SmartDeviceBinding{
		ProviderID:           providerID,
		CapabilityID:         capabilityID,
		SourceType:           payload.SourceType,
		SourceID:             payload.SourceID,
		Enabled:              payload.Enabled,
		Priority:             payload.Priority,
		ConnectionConfigJSON: encodeJSON(payload.ConnectionConfig),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	s.reloadHikvisionBridgeForBinding(payload.ProviderCode, payload.CapabilityCode, "create-smart-binding")
	return s.GetSmartBindingDetail(item.ID)
}

func (s *PlatformService) UpdateSmartBinding(id uint, payload SmartBindingPayload) (map[string]any, error) {
	providerID, capabilityID, err := s.resolveProviderCapabilityIDs(payload.ProviderCode, payload.CapabilityCode)
	if err != nil {
		return nil, err
	}
	var item entity.SmartDeviceBinding
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	var previousProvider entity.SmartInterfaceProvider
	var previousCapability entity.SmartInterfaceCapability
	_ = s.db().First(&previousProvider, item.ProviderID).Error
	_ = s.db().First(&previousCapability, item.CapabilityID).Error
	item.ProviderID = providerID
	item.CapabilityID = capabilityID
	item.SourceType = payload.SourceType
	item.SourceID = payload.SourceID
	item.Enabled = payload.Enabled
	item.Priority = payload.Priority
	item.ConnectionConfigJSON = encodeJSON(payload.ConnectionConfig)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	switch {
	case shouldReloadHikvisionBinding(previousProvider.ProviderCode, previousCapability.CapabilityCode) &&
		shouldReloadHikvisionBinding(payload.ProviderCode, payload.CapabilityCode):
		s.reloadHikvisionBridgeForBinding(payload.ProviderCode, payload.CapabilityCode, "update-smart-binding")
	case shouldReloadHikvisionBinding(previousProvider.ProviderCode, previousCapability.CapabilityCode):
		s.reloadHikvisionBridgeForBinding(previousProvider.ProviderCode, previousCapability.CapabilityCode, "update-smart-binding-old")
	case shouldReloadHikvisionBinding(payload.ProviderCode, payload.CapabilityCode):
		s.reloadHikvisionBridgeForBinding(payload.ProviderCode, payload.CapabilityCode, "update-smart-binding-new")
	}
	return s.GetSmartBindingDetail(id)
}

func (s *PlatformService) DeleteSmartBinding(id uint) error {
	var item entity.SmartDeviceBinding
	if err := s.db().First(&item, id).Error; err != nil {
		return err
	}
	var provider entity.SmartInterfaceProvider
	var capability entity.SmartInterfaceCapability
	_ = s.db().First(&provider, item.ProviderID).Error
	_ = s.db().First(&capability, item.CapabilityID).Error
	_ = s.db().Where("binding_id = ?", id).Delete(&entity.SmartBindingRule{}).Error
	if err := s.db().Delete(&entity.SmartDeviceBinding{}, id).Error; err != nil {
		return err
	}
	s.reloadHikvisionBridgeForBinding(provider.ProviderCode, capability.CapabilityCode, "delete-smart-binding")
	return nil
}

func (s *PlatformService) GetSmartBindingDetail(id uint) (map[string]any, error) {
	var binding entity.SmartDeviceBinding
	if err := s.db().First(&binding, id).Error; err != nil {
		return nil, err
	}
	var provider entity.SmartInterfaceProvider
	var capability entity.SmartInterfaceCapability
	var rules []entity.SmartBindingRule
	var events []entity.SmartEvent
	_ = s.db().First(&provider, binding.ProviderID).Error
	_ = s.db().First(&capability, binding.CapabilityID).Error
	_ = s.db().Where("binding_id = ?", id).Order("id DESC").Find(&rules).Error
	_ = s.db().Where("binding_id = ?", id).Order("event_time DESC").Limit(10).Find(&events).Error
	base := s.smartBindingMap(binding, provider, capability, len(rules))
	base["rules"] = buildBindingRules(rules)
	base["recentEvents"] = s.buildSmartEvents(events)
	base["recentAlarms"] = []map[string]any{}
	return base, nil
}

func (s *PlatformService) CreateSmartBindingRule(bindingID uint, payload SmartBindingRulePayload) (map[string]any, error) {
	pushChannels := normalizePushConfigSelectors(payload.PushChannels)
	item := entity.SmartBindingRule{
		BindingID:             bindingID,
		RuleName:              payload.RuleName,
		Enabled:               payload.Enabled,
		AlarmEnabled:          payload.AlarmEnabled,
		AlarmLevel:            payload.AlarmLevel,
		DedupWindowSeconds:    payload.DedupWindowSeconds,
		CooldownSeconds:       payload.CooldownSeconds,
		MinConfidence:         payload.MinConfidence,
		ActiveTimePlanJSON:    encodeJSON(payload.ActiveTimePlan),
		SnapshotEnabled:       payload.SnapshotEnabled,
		RecordClipEnabled:     payload.RecordClipEnabled,
		RecordPreSeconds:      payload.RecordPreSeconds,
		RecordPostSeconds:     payload.RecordPostSeconds,
		PushEnabled:           payload.PushEnabled,
		PushChannelsJSON:      encodeJSON(pushChannels),
		SendToAI:              payload.SendToAI,
		AIFlowCode:            valueOrEmpty(payload.AIFlowCode),
		GenerateAlarmDirectly: payload.GenerateAlarmDirectly,
		Remark:                valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return bindingRuleMap(item), nil
}

func (s *PlatformService) UpdateSmartBindingRule(ruleID uint, payload SmartBindingRulePayload) (map[string]any, error) {
	var item entity.SmartBindingRule
	if err := s.db().First(&item, ruleID).Error; err != nil {
		return nil, err
	}
	pushChannels := normalizePushConfigSelectors(payload.PushChannels)
	item.RuleName = payload.RuleName
	item.Enabled = payload.Enabled
	item.AlarmEnabled = payload.AlarmEnabled
	item.AlarmLevel = payload.AlarmLevel
	item.DedupWindowSeconds = payload.DedupWindowSeconds
	item.CooldownSeconds = payload.CooldownSeconds
	item.MinConfidence = payload.MinConfidence
	item.ActiveTimePlanJSON = encodeJSON(payload.ActiveTimePlan)
	item.SnapshotEnabled = payload.SnapshotEnabled
	item.RecordClipEnabled = payload.RecordClipEnabled
	item.RecordPreSeconds = payload.RecordPreSeconds
	item.RecordPostSeconds = payload.RecordPostSeconds
	item.PushEnabled = payload.PushEnabled
	item.PushChannelsJSON = encodeJSON(pushChannels)
	item.SendToAI = payload.SendToAI
	item.AIFlowCode = valueOrEmpty(payload.AIFlowCode)
	item.GenerateAlarmDirectly = payload.GenerateAlarmDirectly
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return bindingRuleMap(item), nil
}

func (s *PlatformService) DeleteSmartBindingRule(ruleID uint) error {
	return s.db().Delete(&entity.SmartBindingRule{}, ruleID).Error
}

func (s *PlatformService) IngestSmartProviderEvent(providerCode string, payload any, headers map[string]string) (map[string]any, error) {
	var provider entity.SmartInterfaceProvider
	if err := s.db().Where("provider_code = ?", providerCode).First(&provider).Error; err != nil {
		return nil, err
	}
	if providerCode == "hikvision-isapi" {
		if s.hikvisionBridge == nil {
			return nil, fmt.Errorf("hikvision bridge is not attached")
		}
		return s.hikvisionBridge.IngestISAPIMotionEvent(providerCode, payload, headers)
	}
	raw := entity.SmartRawEvent{
		ProviderID:     provider.ID,
		EventNo:        fmt.Sprintf("RAW-%s", strings.ToUpper(uuid.NewString())),
		EventTime:      time.Now(),
		HeadersJSON:    encodeJSON(headers),
		RawPayloadJSON: encodeJSON(payload),
		ParseStatus:    "success",
	}
	if err := s.db().Create(&raw).Error; err != nil {
		return nil, err
	}
	event := entity.SmartEvent{
		RawEventID:            &raw.ID,
		ProviderID:            provider.ID,
		EventCode:             fmt.Sprintf("SE-%s", strings.ToUpper(uuid.NewString())),
		EventType:             "motion_detected",
		EventLevel:            "medium",
		SourceStage:           "raw",
		EventTime:             raw.EventTime,
		DedupKey:              fmt.Sprintf("smart:%d:%s", provider.ID, raw.EventNo),
		NormalizedPayloadJSON: raw.RawPayloadJSON,
		Status:                "stored",
	}
	_ = s.db().Create(&event).Error
	return map[string]any{
		"accepted":     true,
		"rawEventId":   raw.ID,
		"smartEventId": event.ID,
		"reason":       "事件已入库",
	}, nil
}

type smartRawEventListRow struct {
	ID             uint
	ProviderID     uint
	CapabilityID   *uint
	BindingID      *uint
	SourceType     string
	SourceID       *uint
	SourceEventID  string
	EventNo        string
	EventTime      time.Time
	SignatureValid *bool
	HeadersJSON    string
	RawPayloadJSON string
	ParseStatus    string
	ParseError     string
	CreatedAt      time.Time
	ProviderCode   string
	ProviderName   string
	CapabilityCode string
	CapabilityName string
}

func (s *PlatformService) ListSmartRawEvents(page, pageSize int, filter SmartRawEventListFilter) (map[string]any, error) {
	page = maxInt(page, 1)
	pageSize = maxInt(pageSize, 20)
	query := s.db().Table("smart_raw_event AS r").
		Joins("LEFT JOIN smart_interface_provider p ON p.id = r.provider_id").
		Joins("LEFT JOIN smart_interface_capability c ON c.id = r.capability_id")
	if filter.ProviderCode != "" {
		query = query.Where("p.provider_code = ?", filter.ProviderCode)
	}
	if filter.CapabilityCode != "" {
		query = query.Where("c.capability_code = ?", filter.CapabilityCode)
	}
	if filter.ParseStatus != "" {
		query = query.Where("r.parse_status = ?", filter.ParseStatus)
	}
	if filter.SourceType != "" {
		query = query.Where("r.source_type = ?", filter.SourceType)
	}
	if filter.RecentDays > 0 {
		query = query.Where("r.event_time >= ?", time.Now().AddDate(0, 0, -filter.RecentDays))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []smartRawEventListRow
	if err := query.
		Select(`r.id, r.provider_id, r.capability_id, r.binding_id, r.source_type, r.source_id, r.source_event_id,
			r.event_no, r.event_time, r.signature_valid, r.headers_json, r.raw_payload_json,
			r.parse_status, r.parse_error, r.created_at,
			p.provider_code, p.provider_name, c.capability_code, c.capability_name`).
		Order("r.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		result = append(result, map[string]any{
			"id":             item.ID,
			"providerCode":   item.ProviderCode,
			"providerName":   item.ProviderName,
			"capabilityCode": nullableString(item.CapabilityCode),
			"capabilityName": nullableString(item.CapabilityName),
			"bindingId":      item.BindingID,
			"sourceType":     nullableString(item.SourceType),
			"sourceId":       item.SourceID,
			"sourceEventId":  nullableString(item.SourceEventID),
			"eventNo":        item.EventNo,
			"eventTime":      item.EventTime.Format(time.RFC3339),
			"signatureValid": item.SignatureValid,
			"parseStatus":    item.ParseStatus,
			"parseError":     nullableString(item.ParseError),
			"headersJson":    nullableString(item.HeadersJSON),
			"rawPayloadJson": item.RawPayloadJSON,
			"createdAt":      item.CreatedAt.Format(time.RFC3339),
		})
	}
	return map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize}, nil
}

func (s *PlatformService) ListSmartEvents(page, pageSize int, filter SmartEventListFilter) (map[string]any, error) {
	query := s.db().Table("smart_event AS e").
		Joins("LEFT JOIN smart_interface_provider p ON p.id = e.provider_id").
		Joins("LEFT JOIN smart_interface_capability c ON c.id = e.capability_id").
		Joins("LEFT JOIN camera_device cam ON cam.id = e.camera_id").
		Joins("LEFT JOIN recorder_device rec ON rec.id = e.recorder_id").
		Joins("LEFT JOIN recorder_channel ch ON ch.id = e.channel_id")
	query = s.applySmartEventAccessScopeQuery(query, "e", filter.AccessScope)
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("(e.event_code LIKE ? OR e.event_type LIKE ? OR e.dedup_key LIKE ?)", likeKeyword, likeKeyword, likeKeyword)
	}
	if filter.ProviderCode != "" {
		query = query.Where("p.provider_code = ?", filter.ProviderCode)
	}
	if filter.CapabilityCode != "" {
		query = query.Where("c.capability_code = ?", filter.CapabilityCode)
	}
	if filter.Status != "" {
		query = query.Where("e.status = ?", filter.Status)
	}
	if filter.SourceStage != "" {
		query = query.Where("e.source_stage = ?", filter.SourceStage)
	}
	if filter.RecentDays > 0 {
		query = query.Where("e.event_time >= ?", time.Now().AddDate(0, 0, -filter.RecentDays))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []smartEventListRow
	if err := query.Select(`e.id, e.raw_event_id, e.binding_id, e.provider_id, e.capability_id, e.event_code,
			e.event_type, e.event_level, e.source_stage, e.event_time, e.camera_id, e.recorder_id, e.channel_id,
			e.factory_id, e.zone_id, e.image_url, e.video_url, e.confidence, e.dedup_key,
			e.normalized_payload_json, e.status, e.created_at,
			p.provider_code, p.provider_name, c.capability_code, c.capability_name,
			COALESCE(ch.name, cam.name, rec.name) AS source_name,
			CASE
				WHEN e.channel_id IS NOT NULL THEN CONCAT('channel/', e.channel_id)
				WHEN e.camera_id IS NOT NULL THEN CONCAT('camera/', e.camera_id)
				WHEN e.recorder_id IS NOT NULL THEN CONCAT('recorder/', e.recorder_id)
				ELSE NULL
			END AS source_path`).
		Order("e.event_time DESC, e.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&items).Error; err != nil {
		return nil, err
	}
	eventItems := buildSmartEventListItems(items)
	return map[string]any{"items": eventItems, "total": total, "page": page, "pageSize": pageSize}, nil
}

type smartEventListRow struct {
	ID                    uint
	RawEventID            *uint
	BindingID             *uint
	ProviderID            uint
	CapabilityID          *uint
	EventCode             string
	EventType             string
	EventLevel            string
	SourceStage           string
	EventTime             time.Time
	CameraID              *uint
	RecorderID            *uint
	ChannelID             *uint
	FactoryID             *uint
	ZoneID                *uint
	ImageURL              string
	VideoURL              string
	Confidence            *float64
	DedupKey              string
	NormalizedPayloadJSON string
	Status                string
	CreatedAt             time.Time
	ProviderCode          string
	ProviderName          string
	CapabilityCode        string
	CapabilityName        string
	SourceName            *string
	SourcePath            *string
}

func buildSmartEventListItems(items []smartEventListRow) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, smartEventListMap(item))
	}
	return result
}

func smartEventListMap(item smartEventListRow) map[string]any {
	base := smartEventMap(entity.SmartEvent{
		ID:                    item.ID,
		RawEventID:            item.RawEventID,
		BindingID:             item.BindingID,
		ProviderID:            item.ProviderID,
		CapabilityID:          item.CapabilityID,
		EventCode:             item.EventCode,
		EventType:             item.EventType,
		EventLevel:            item.EventLevel,
		SourceStage:           item.SourceStage,
		EventTime:             item.EventTime,
		CameraID:              item.CameraID,
		RecorderID:            item.RecorderID,
		ChannelID:             item.ChannelID,
		FactoryID:             item.FactoryID,
		ZoneID:                item.ZoneID,
		ImageURL:              item.ImageURL,
		VideoURL:              item.VideoURL,
		Confidence:            item.Confidence,
		DedupKey:              item.DedupKey,
		NormalizedPayloadJSON: item.NormalizedPayloadJSON,
		Status:                item.Status,
		CreatedAt:             item.CreatedAt,
	})
	base["providerCode"] = item.ProviderCode
	base["providerName"] = item.ProviderName
	base["capabilityCode"] = item.CapabilityCode
	base["capabilityName"] = item.CapabilityName
	if item.SourceName != nil {
		base["sourceName"] = *item.SourceName
	}
	if item.SourcePath != nil {
		base["sourcePath"] = *item.SourcePath
	}
	return base
}

func (s *PlatformService) GetSmartEventDetail(id uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureSmartEventAccessible(id, accessScope)
	if err != nil {
		return nil, err
	}
	base := smartEventMap(*item)
	if item.ProviderID != 0 {
		var provider entity.SmartInterfaceProvider
		if err := s.db().First(&provider, item.ProviderID).Error; err == nil {
			base["providerCode"] = provider.ProviderCode
			base["providerName"] = provider.ProviderName
		}
	}
	if item.CapabilityID != nil {
		var capability entity.SmartInterfaceCapability
		if err := s.db().First(&capability, *item.CapabilityID).Error; err == nil {
			base["capabilityCode"] = capability.CapabilityCode
			base["capabilityName"] = capability.CapabilityName
		}
	}
	if item.CameraID != nil {
		var camera entity.CameraDevice
		if err := s.db().First(&camera, *item.CameraID).Error; err == nil {
			base["cameraName"] = camera.Name
			if base["sourceName"] == nil {
				base["sourceName"] = camera.Name
			}
		}
	}
	if item.RecorderID != nil {
		var recorder entity.RecorderDevice
		if err := s.db().First(&recorder, *item.RecorderID).Error; err == nil {
			base["recorderName"] = recorder.Name
			if base["sourceName"] == nil {
				base["sourceName"] = recorder.Name
			}
		}
	}
	if item.ChannelID != nil {
		var channel entity.RecorderChannel
		if err := s.db().First(&channel, *item.ChannelID).Error; err == nil {
			base["channelName"] = channel.Name
			base["sourceName"] = channel.Name
		}
	}
	if item.FactoryID != nil {
		var factory entity.FactoryArea
		if err := s.db().First(&factory, *item.FactoryID).Error; err == nil {
			base["factoryName"] = factory.FactoryName
		}
	}
	if item.ZoneID != nil {
		var zone entity.FactoryZone
		if err := s.db().First(&zone, *item.ZoneID).Error; err == nil {
			base["zoneName"] = zone.ZoneName
		}
	}
	var linkedAlarm entity.AlarmRecord
	linkedAlarmFound := false
	if err := s.db().Where("smart_event_id = ?", item.ID).Order("id DESC").First(&linkedAlarm).Error; err == nil {
		linkedAlarmFound = true
	} else if err == gorm.ErrRecordNotFound && item.RawEventID != nil {
		if rawErr := s.db().Where("raw_event_id = ?", *item.RawEventID).Order("id DESC").First(&linkedAlarm).Error; rawErr == nil {
			linkedAlarmFound = true
		}
	}
	if linkedAlarmFound {
		base["linkedAlarm"] = map[string]any{
			"id":              linkedAlarm.ID,
			"code":            linkedAlarm.AlarmNo,
			"level":           linkedAlarm.AlarmLevel,
			"status":          linkedAlarm.Status,
			"time":            linkedAlarm.AlarmTime.Format(time.RFC3339),
			"message":         nullableString(linkedAlarm.Message),
			"imageUrl":        nullableString(linkedAlarm.ImageURL),
			"videoUrl":        nullableString(linkedAlarm.VideoURL),
			"recordStartTime": timePtrToRFC3339(linkedAlarm.RecordStartTime),
			"recordEndTime":   timePtrToRFC3339(linkedAlarm.RecordEndTime),
		}
		if base["imageUrl"] == nil {
			base["imageUrl"] = nullableString(linkedAlarm.ImageURL)
		}
		if base["videoUrl"] == nil {
			base["videoUrl"] = nullableString(linkedAlarm.VideoURL)
		}
	}
	if item.RawEventID != nil {
		var raw entity.SmartRawEvent
		if err := s.db().First(&raw, *item.RawEventID).Error; err == nil {
			base["rawEvent"] = map[string]any{
				"id":             raw.ID,
				"eventNo":        raw.EventNo,
				"eventTime":      raw.EventTime.Format(time.RFC3339),
				"parseStatus":    raw.ParseStatus,
				"rawPayloadJson": raw.RawPayloadJSON,
				"createdAt":      raw.CreatedAt.Format(time.RFC3339),
			}
		}
	}
	var tasks []entity.AiReviewTask
	var results []entity.AiReviewResult
	_ = s.db().Where("smart_event_id = ?", id).Order("id DESC").Find(&tasks).Error
	taskIDs := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	if len(taskIDs) > 0 {
		_ = s.db().Where("task_id IN ?", taskIDs).Order("id DESC").Find(&results).Error
	}
	base["aiTasks"] = buildAITasks(tasks, results)
	base["aiResults"] = buildAIResults(results)
	return base, nil
}

func (s *PlatformService) ListSmartBridgeReconnectLogs(page, pageSize int, filter SmartBridgeReconnectLogListFilter) (map[string]any, error) {
	page = maxInt(page, 1)
	pageSize = maxInt(pageSize, 20)
	if pageSize > 100 {
		pageSize = 100
	}
	query := s.db().Model(&entity.SmartBridgeReconnectLog{})
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.TriggerReason != "" {
		query = query.Where("trigger_reason = ?", filter.TriggerReason)
	}
	if filter.DeviceType != "" {
		query = query.Where("device_type = ?", filter.DeviceType)
	}
	if filter.DeviceID > 0 {
		query = query.Where("device_id = ?", filter.DeviceID)
	}
	if filter.SessionKey != "" {
		query = query.Where("session_key LIKE ?", "%"+filter.SessionKey+"%")
	}
	if filter.StartAt != nil {
		query = query.Where("created_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("created_at <= ?", *filter.EndAt)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []entity.SmartBridgeReconnectLog
	if err := query.Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, smartBridgeReconnectLogMap(item))
	}
	return map[string]any{"items": result, "total": total, "page": page, "pageSize": pageSize}, nil
}

func (s *PlatformService) SubmitSmartAIReview(eventID uint, payload SmartAIReviewPayload, accessScope *AccessScope) (map[string]any, error) {
	if _, err := s.ensureSmartEventAccessible(eventID, accessScope); err != nil {
		return nil, err
	}
	task := entity.AiReviewTask{
		SmartEventID:       eventID,
		TaskNo:             fmt.Sprintf("AIT-%s", strings.ToUpper(uuid.NewString())),
		AIFlowCode:         payload.AIFlowCode,
		ModelCode:          valueOrEmpty(payload.ModelCode),
		RequestPayloadJSON: encodeJSON(payload),
		Status:             "pending",
		RetryCount:         0,
		MaxRetryCount:      3,
		SubmittedAt:        time.Now(),
	}
	if err := s.db().Create(&task).Error; err != nil {
		return nil, err
	}
	return aiTaskMap(task, nil), nil
}

func (s *PlatformService) ListSmartAITasks(page, pageSize int, filter SmartAITaskListFilter, accessScope *AccessScope) (map[string]any, error) {
	page = maxInt(page, 1)
	pageSize = maxInt(pageSize, 20)
	var tasks []entity.AiReviewTask
	var results []entity.AiReviewResult
	query := s.db().Table("ai_review_task AS t").Joins("JOIN smart_event e ON e.id = t.smart_event_id")
	query = s.applySmartEventAccessScopeQuery(query, "e", accessScope)
	if filter.Status != "" {
		query = query.Where("t.status = ?", filter.Status)
	}
	if filter.AIFlowCode != "" {
		query = query.Where("t.ai_flow_code = ?", filter.AIFlowCode)
	}
	if filter.RecentDays > 0 {
		query = query.Where("t.submitted_at >= ?", time.Now().AddDate(0, 0, -filter.RecentDays))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	if err := query.Select("t.*").
		Order("t.submitted_at DESC, t.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&tasks).Error; err != nil {
		return nil, err
	}
	taskIDs := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	if len(taskIDs) > 0 {
		_ = s.db().Where("task_id IN ?", taskIDs).Order("id DESC").Find(&results).Error
	}
	return map[string]any{"items": buildAITasks(tasks, results), "total": total, "page": page, "pageSize": pageSize}, nil
}

func (s *PlatformService) GetSmartAITask(taskID uint, accessScope *AccessScope) (map[string]any, error) {
	var task entity.AiReviewTask
	if err := s.db().First(&task, taskID).Error; err != nil {
		return nil, err
	}
	if _, err := s.ensureSmartEventAccessible(task.SmartEventID, accessScope); err != nil {
		return nil, err
	}
	var result entity.AiReviewResult
	if err := s.db().Where("task_id = ?", taskID).Order("id DESC").First(&result).Error; err == nil {
		return aiTaskMap(task, &result), nil
	}
	return aiTaskMap(task, nil), nil
}

func (s *PlatformService) RetrySmartAITask(taskID uint, accessScope *AccessScope) (map[string]any, error) {
	var task entity.AiReviewTask
	if err := s.db().First(&task, taskID).Error; err != nil {
		return nil, err
	}
	if _, err := s.ensureSmartEventAccessible(task.SmartEventID, accessScope); err != nil {
		return nil, err
	}
	task.Status = "pending"
	task.RetryCount += 1
	task.ErrorMessage = ""
	task.FinishedAt = nil
	if err := s.db().Save(&task).Error; err != nil {
		return nil, err
	}
	return aiTaskMap(task, nil), nil
}

func (s *PlatformService) HandleSmartAICallback(payload SmartAICallbackPayload) (map[string]any, error) {
	var task entity.AiReviewTask
	if err := s.db().Where("task_no = ?", payload.TaskNo).First(&task).Error; err != nil {
		return nil, err
	}
	result := entity.AiReviewResult{
		TaskID:            task.ID,
		Decision:          payload.Decision,
		LabelsJSON:        encodeJSON(payload.Labels),
		Confidence:        payload.Confidence,
		Reason:            valueOrEmpty(payload.Reason),
		EvidenceJSON:      encodeJSON(payload.Evidence),
		ResultPayloadJSON: encodeJSON(payload.Raw),
	}
	now := time.Now()
	task.Status = "done"
	task.FinishedAt = &now
	task.ErrorMessage = ""
	if err := s.db().Create(&result).Error; err != nil {
		return nil, err
	}
	if err := s.db().Save(&task).Error; err != nil {
		return nil, err
	}
	return aiTaskMap(task, &result), nil
}
