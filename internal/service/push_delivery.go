package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"secmgmt_go/internal/domain/entity"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type alarmPushContext struct {
	ChannelName string
}

type pushActiveTimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type wechatTemplateField struct {
	Value string `json:"value"`
}

type wechatTemplateMessagePayload struct {
	ToUser     string                         `json:"touser"`
	TemplateID string                         `json:"template_id"`
	Data       map[string]wechatTemplateField `json:"data"`
}

type wechatAccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wechatTemplateSendResponse struct {
	ErrCode int64  `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MsgID   int64  `json:"msgid"`
}

type pushDeliveryResult struct {
	Status       string
	Message      string
	RequestBody  string
	ResponseBody string
	ErrorMessage string
}

func dispatchAlarmPushes(db *gorm.DB, logger *zap.Logger, alarm entity.AlarmRecord, allowedChannels []string, triggeredBy string) {
	if db == nil {
		return
	}
	if len(allowedChannels) > 0 && !containsStringFold(allowedChannels, "wechat") {
		return
	}

	var configs []entity.PushConfig
	if err := db.Where("enabled = ? AND provider_type = ?", true, "wechat").Find(&configs).Error; err != nil {
		if logger != nil {
			logger.Warn("load push configs failed", zap.Error(err), zap.Uint("alarmID", alarm.ID))
		}
		return
	}

	now := time.Now()
	ctx := resolveAlarmPushContext(db, alarm)
	for _, config := range configs {
		if !pushConfigMatchesAlarm(config, alarm, now) {
			continue
		}
		if isPushRateLimited(db, config, now) {
			createAlarmPushLog(db, alarm, config, triggeredBy, pushDeliveryResult{
				Status:  "rate_limited",
				Message: "触发限流，已跳过微信推送",
			})
			continue
		}

		result := deliverAlarmWechatPush(config, alarm, ctx)
		createAlarmPushLog(db, alarm, config, triggeredBy, result)
	}
}

func deliverAlarmWechatPush(config entity.PushConfig, alarm entity.AlarmRecord, ctx alarmPushContext) pushDeliveryResult {
	return deliverWechatTemplatePush(config, buildAlarmWechatTemplateData(alarm, ctx))
}

func deliverTestWechatPush(config entity.PushConfig, now time.Time) pushDeliveryResult {
	return deliverWechatTemplatePush(config, map[string]wechatTemplateField{
		"time1":  {Value: now.Format("2006-01-02 15:04:05")},
		"const2": {Value: "测试告警"},
		"thing5": {Value: trimWechatThingValue("测试通道")},
		"const3": {Value: "中级告警"},
	})
}

func deliverWechatTemplatePush(config entity.PushConfig, data map[string]wechatTemplateField) pushDeliveryResult {
	appID := strings.TrimSpace(config.AppID)
	appSecret := strings.TrimSpace(config.AppSecretEncrypted)
	templateID := strings.TrimSpace(config.TemplateID)
	receiverOpenIDs := decodeJSONStringSlice(config.ReceiverOpenIDsJSON)
	if appID == "" || appSecret == "" || templateID == "" {
		detail := "缺少 AppID、AppSecret 或模板ID"
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("微信推送配置不完整", detail),
			ErrorMessage: detail,
		}
	}
	if len(receiverOpenIDs) == 0 {
		detail := "未配置接收人 OpenID"
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("未配置微信接收人 OpenID", detail),
			ErrorMessage: detail,
		}
	}

	requestPayloads := make([]wechatTemplateMessagePayload, 0, len(receiverOpenIDs))
	for _, openID := range receiverOpenIDs {
		trimmedOpenID := strings.TrimSpace(openID)
		if trimmedOpenID == "" {
			continue
		}
		requestPayloads = append(requestPayloads, wechatTemplateMessagePayload{
			ToUser:     trimmedOpenID,
			TemplateID: templateID,
			Data:       data,
		})
	}
	if len(requestPayloads) == 0 {
		detail := "接收人 OpenID 全为空"
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("未配置有效的微信接收人 OpenID", detail),
			ErrorMessage: detail,
		}
	}

	requestBody := encodeJSON(requestPayloads)
	if strings.HasPrefix(strings.ToLower(appID), "mock://wechat/") {
		return pushDeliveryResult{
			Status:       "success",
			Message:      fmt.Sprintf("微信模板消息模拟推送成功，共 %d 人", len(requestPayloads)),
			RequestBody:  requestBody,
			ResponseBody: `{"mock":true,"errcode":0,"errmsg":"ok"}`,
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	accessToken, tokenErr := fetchWechatAccessToken(client, appID, appSecret)
	if tokenErr != nil {
		detail := tokenErr.Error()
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("获取微信 access_token 失败", detail),
			RequestBody:  requestBody,
			ErrorMessage: detail,
		}
	}

	responses := make([]map[string]any, 0, len(requestPayloads))
	errorMessages := make([]string, 0)
	successCount := 0
	for _, payload := range requestPayloads {
		response, err := sendWechatTemplateMessage(client, accessToken, payload)
		if err != nil {
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %v", payload.ToUser, err))
			responses = append(responses, map[string]any{
				"touser": payload.ToUser,
				"error":  err.Error(),
			})
			continue
		}
		responses = append(responses, map[string]any{
			"touser":  payload.ToUser,
			"errcode": response.ErrCode,
			"errmsg":  response.ErrMsg,
			"msgid":   response.MsgID,
		})
		if response.ErrCode == 0 {
			successCount++
			continue
		}
		errorMessages = append(errorMessages, fmt.Sprintf("%s: %s", payload.ToUser, response.ErrMsg))
	}

	result := pushDeliveryResult{
		RequestBody:  requestBody,
		ResponseBody: encodeJSON(responses),
	}
	switch {
	case successCount == len(requestPayloads):
		result.Status = "success"
		result.Message = fmt.Sprintf("微信模板消息推送成功，共 %d 人", successCount)
	case successCount > 0:
		result.Status = "failed"
		result.ErrorMessage = strings.Join(errorMessages, "; ")
		result.Message = buildPushFailureMessage(
			fmt.Sprintf("微信模板消息部分成功，成功 %d/%d", successCount, len(requestPayloads)),
			result.ErrorMessage,
		)
	default:
		result.Status = "failed"
		result.ErrorMessage = strings.Join(errorMessages, "; ")
		result.Message = buildPushFailureMessage("微信模板消息推送失败", result.ErrorMessage)
	}
	return result
}

func fetchWechatAccessToken(client *http.Client, appID, appSecret string) (string, error) {
	tokenURL := fmt.Sprintf(
		"https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		url.QueryEscape(appID),
		url.QueryEscape(appSecret),
	)
	resp, err := client.Get(tokenURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed wechatAccessTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("解析微信 access_token 响应失败: %w, body=%s", err, compactPushErrorDetail(string(body)))
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		if parsed.ErrMsg == "" {
			parsed.ErrMsg = string(body)
		}
		return "", fmt.Errorf("wechat token error %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}
	return parsed.AccessToken, nil
}

func sendWechatTemplateMessage(client *http.Client, accessToken string, payload wechatTemplateMessagePayload) (wechatTemplateSendResponse, error) {
	requestBody, err := json.Marshal(payload)
	if err != nil {
		return wechatTemplateSendResponse{}, err
	}

	sendURL := "https://api.weixin.qq.com/cgi-bin/message/template/send?access_token=" + url.QueryEscape(accessToken)
	req, err := http.NewRequest(http.MethodPost, sendURL, bytes.NewReader(requestBody))
	if err != nil {
		return wechatTemplateSendResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")

	resp, err := client.Do(req)
	if err != nil {
		return wechatTemplateSendResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wechatTemplateSendResponse{}, err
	}

	var parsed wechatTemplateSendResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return wechatTemplateSendResponse{}, fmt.Errorf("解析微信模板消息响应失败: %w, body=%s", err, compactPushErrorDetail(string(body)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parsed, fmt.Errorf("wechat template send returned status %d: %s", resp.StatusCode, compactPushErrorDetail(string(body)))
	}
	if parsed.ErrCode != 0 {
		return parsed, fmt.Errorf("wechat template send error %d: %s", parsed.ErrCode, parsed.ErrMsg)
	}
	return parsed, nil
}

func createAlarmPushLog(db *gorm.DB, alarm entity.AlarmRecord, config entity.PushConfig, triggeredBy string, result pushDeliveryResult) {
	if db == nil {
		return
	}
	pushConfigID := config.ID
	alarmID := alarm.ID
	logItem := entity.AlarmPushLog{
		AlarmID:      &alarmID,
		PushConfigID: &pushConfigID,
		Channel:      config.ProviderType,
		ProviderType: config.ProviderType,
		Status:       result.Status,
		ConfigName:   config.ConfigName,
		AlarmNo:      alarm.AlarmNo,
		AlarmType:    alarm.AlarmType,
		AlarmLevel:   alarm.AlarmLevel,
		FactoryID:    alarm.FactoryID,
		ZoneID:       alarm.ZoneID,
		TriggeredBy:  triggeredBy,
		RetryCount:   0,
		Message:      result.Message,
		RequestBody:  result.RequestBody,
		ResponseBody: result.ResponseBody,
		ErrorMessage: result.ErrorMessage,
		PushedAt:     time.Now(),
	}
	_ = db.Create(&logItem).Error
}

func pushConfigMatchesAlarm(config entity.PushConfig, alarm entity.AlarmRecord, now time.Time) bool {
	factoryIDs := decodeJSONUintSlice(config.FactoryIDsJSON)
	if len(factoryIDs) > 0 {
		if alarm.FactoryID == nil || !containsUint(factoryIDs, *alarm.FactoryID) {
			return false
		}
	}

	zoneIDs := decodeJSONUintSlice(config.ZoneIDsJSON)
	if len(zoneIDs) > 0 {
		if alarm.ZoneID == nil || !containsUint(zoneIDs, *alarm.ZoneID) {
			return false
		}
	}

	alarmTypes := decodeJSONStringSlice(config.AlarmTypesJSON)
	if len(alarmTypes) > 0 && !containsString(alarmTypes, alarm.AlarmType) {
		return false
	}

	alarmLevels := decodeJSONStringSlice(config.AlarmLevelsJSON)
	if len(alarmLevels) > 0 && !containsString(alarmLevels, alarm.AlarmLevel) {
		return false
	}

	activeRanges := decodePushActiveTimeRanges(config.ActiveTimeRangesJSON)
	if len(activeRanges) > 0 && !isWithinAnyActiveTimeRange(now, activeRanges) {
		return false
	}
	return true
}

func isPushRateLimited(db *gorm.DB, config entity.PushConfig, now time.Time) bool {
	if db == nil || config.RateLimitWindowSeconds <= 0 || config.RateLimitMaxCount <= 0 {
		return false
	}
	var count int64
	cutoff := now.Add(-time.Duration(config.RateLimitWindowSeconds) * time.Second)
	_ = db.Model(&entity.AlarmPushLog{}).
		Where("push_config_id = ? AND pushed_at >= ?", config.ID, cutoff).
		Count(&count).Error
	return count >= int64(config.RateLimitMaxCount)
}

func resolveAlarmPushContext(db *gorm.DB, alarm entity.AlarmRecord) alarmPushContext {
	ctx := alarmPushContext{ChannelName: "未知通道"}
	if db == nil {
		return ctx
	}
	if alarm.ChannelID != nil {
		var channel entity.RecorderChannel
		if err := db.First(&channel, *alarm.ChannelID).Error; err == nil && strings.TrimSpace(channel.Name) != "" {
			ctx.ChannelName = channel.Name
			return ctx
		}
	}
	if alarm.CameraID != nil {
		var camera entity.CameraDevice
		if err := db.First(&camera, *alarm.CameraID).Error; err == nil && strings.TrimSpace(camera.Name) != "" {
			ctx.ChannelName = camera.Name
			return ctx
		}
	}
	if alarm.RecorderID != nil {
		var recorder entity.RecorderDevice
		if err := db.First(&recorder, *alarm.RecorderID).Error; err == nil && strings.TrimSpace(recorder.Name) != "" {
			ctx.ChannelName = recorder.Name
		}
	}
	return ctx
}

func buildAlarmWechatTemplateData(alarm entity.AlarmRecord, ctx alarmPushContext) map[string]wechatTemplateField {
	return map[string]wechatTemplateField{
		"time1":  {Value: alarm.AlarmTime.Format("2006-01-02 15:04:05")},
		"const2": {Value: alarm.AlarmType},
		"thing5": {Value: trimWechatThingValue(ctx.ChannelName)},
		"const3": {Value: formatWechatAlarmLevel(alarm.AlarmLevel)},
	}
}

func formatWechatAlarmLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical", "严重":
		return "严重告警"
	case "medium", "中":
		return "中级告警"
	case "high", "高":
		return "高级告警"
	case "low", "低":
		return "低级告警"
	default:
		return level
	}
}

func trimWechatThingValue(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= 20 {
		return string(runes)
	}
	return string(runes[:20])
}

func decodePushActiveTimeRanges(raw string) []pushActiveTimeRange {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var ranges []pushActiveTimeRange
	if err := json.Unmarshal([]byte(raw), &ranges); err != nil {
		return nil
	}
	return ranges
}

func isWithinAnyActiveTimeRange(now time.Time, ranges []pushActiveTimeRange) bool {
	if len(ranges) == 0 {
		return true
	}
	currentMinute := now.Hour()*60 + now.Minute()
	for _, item := range ranges {
		startMinute, startOK := parseClockMinute(item.Start)
		endMinute, endOK := parseClockMinute(item.End)
		if !startOK || !endOK {
			continue
		}
		if startMinute <= endMinute {
			if currentMinute >= startMinute && currentMinute <= endMinute {
				return true
			}
			continue
		}
		if currentMinute >= startMinute || currentMinute <= endMinute {
			return true
		}
	}
	return false
}

func parseClockMinute(value string) (int, bool) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	return parsed.Hour()*60 + parsed.Minute(), true
}

func containsUint(values []uint, target uint) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func buildPushFailureMessage(summary, detail string) string {
	detail = compactPushErrorDetail(detail)
	if detail == "" {
		return summary
	}
	return summary + "： " + detail
}

func compactPushErrorDetail(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return ""
	}
	detail = strings.Join(strings.Fields(detail), " ")
	runes := []rune(detail)
	if len(runes) <= 180 {
		return detail
	}
	return string(runes[:180]) + "..."
}
