package service

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"secmgmt_go/internal/config"
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

type emailAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
}

func dispatchAlarmPushes(db *gorm.DB, cfg *config.Config, logger *zap.Logger, alarm entity.AlarmRecord, allowedChannels []string, triggeredBy string) {
	if db == nil {
		return
	}

	var configs []entity.PushConfig
	if err := db.Where("enabled = ? AND provider_type IN ?", true, []string{"wechat", "email"}).Find(&configs).Error; err != nil {
		if logger != nil {
			logger.Warn("load push configs failed", zap.Error(err), zap.Uint("alarmID", alarm.ID))
		}
		return
	}

	now := time.Now()
	ctx := resolveAlarmPushContext(db, alarm)
	for _, config := range configs {
		if len(allowedChannels) > 0 && !pushChannelAllowed(allowedChannels, config.ProviderType) {
			continue
		}
		if !pushConfigMatchesAlarm(config, alarm, now) {
			continue
		}
		if isPushRateLimited(db, config, now) {
			channelName := "推送"
			switch strings.ToLower(strings.TrimSpace(config.ProviderType)) {
			case "wechat":
				channelName = "微信推送"
			case "email":
				channelName = "邮件推送"
			}
			createAlarmPushLog(db, alarm, config, triggeredBy, pushDeliveryResult{
				Status:  "rate_limited",
				Message: "触发限流，已跳过" + channelName,
			})
			continue
		}

		var result pushDeliveryResult
		switch strings.ToLower(strings.TrimSpace(config.ProviderType)) {
		case "wechat":
			result = deliverAlarmWechatPush(config, alarm, ctx)
		case "email":
			result = deliverAlarmEmailPush(cfg, config, alarm, ctx)
		default:
			continue
		}
		createAlarmPushLog(db, alarm, config, triggeredBy, result)
	}
}

func deliverAlarmWechatPush(config entity.PushConfig, alarm entity.AlarmRecord, ctx alarmPushContext) pushDeliveryResult {
	return deliverWechatTemplatePush(config, buildAlarmWechatTemplateData(alarm, ctx))
}

func deliverTestWechatPush(config entity.PushConfig, now time.Time) pushDeliveryResult {
	return deliverWechatTemplatePush(config, map[string]wechatTemplateField{
		"time1":  {Value: now.Format("2006-01-02 15:04:05")},
		"const2": {Value: formatWechatAlarmType("移动侦测")},
		"thing5": {Value: trimWechatThingValue("测试通道")},
		"const3": {Value: "中级告警"},
	})
}

func deliverAlarmEmailPush(cfg *config.Config, config entity.PushConfig, alarm entity.AlarmRecord, ctx alarmPushContext) pushDeliveryResult {
	subject := buildAlarmEmailSubject(alarm)
	textBody := buildAlarmEmailTextBody(alarm, ctx)
	htmlBody := buildAlarmEmailHTMLBody(alarm, ctx)
	attachments, err := resolveAlarmEmailAttachments(cfg, alarm)
	if err != nil {
		detail := err.Error()
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("邮件附件准备失败", detail),
			ErrorMessage: detail,
		}
	}
	return deliverEmailPush(cfg, config, subject, textBody, htmlBody, attachments)
}

func deliverTestEmailPush(cfg *config.Config, config entity.PushConfig, now time.Time) pushDeliveryResult {
	subject := "[测试推送] 安全管理平台邮件通道联通性验证"
	textBody := strings.Join([]string{
		"这是一封测试邮件，用于验证邮件推送配置是否可用。",
		"",
		"测试时间: " + now.Format("2006-01-02 15:04:05"),
		"测试类型: 移动侦测",
		"测试通道: 测试通道",
		"测试等级: 中级告警",
	}, "\n")
	htmlBody := strings.Join([]string{
		"<div style=\"font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2d3d;line-height:1.7\">",
		"<h3 style=\"margin:0 0 12px\">邮件推送测试</h3>",
		"<p>这是一封测试邮件，用于验证邮件推送配置是否可用。</p>",
		"<table style=\"border-collapse:collapse\">",
		fmt.Sprintf("<tr><td style=\"padding:4px 12px 4px 0;color:#6b7280\">测试时间</td><td>%s</td></tr>", html.EscapeString(now.Format("2006-01-02 15:04:05"))),
		"<tr><td style=\"padding:4px 12px 4px 0;color:#6b7280\">测试类型</td><td>移动侦测</td></tr>",
		"<tr><td style=\"padding:4px 12px 4px 0;color:#6b7280\">测试通道</td><td>测试通道</td></tr>",
		"<tr><td style=\"padding:4px 12px 4px 0;color:#6b7280\">测试等级</td><td>中级告警</td></tr>",
		"</table>",
		"</div>",
	}, "")
	return deliverEmailPush(cfg, config, subject, textBody, htmlBody, nil)
}

func deliverEmailPush(cfg *config.Config, config entity.PushConfig, subject, textBody, htmlBody string, attachments []emailAttachment) pushDeliveryResult {
	smtpHost, smtpPort, smtpUsername, smtpPassword, fromAddress, fromName, err := resolveEmailSenderConfig(cfg)
	if err != nil {
		detail := err.Error()
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("邮件推送配置不完整", detail),
			ErrorMessage: detail,
		}
	}

	recipients, err := resolveEmailRecipients(config.ReceiverOpenIDsJSON)
	if err != nil {
		detail := err.Error()
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("邮件接收人配置无效", detail),
			ErrorMessage: detail,
		}
	}

	requestBody := encodeJSON(map[string]any{
		"from":        fromAddress,
		"to":          recipients,
		"subject":     subject,
		"html":        htmlBody,
		"text":        textBody,
		"attachments": summarizeEmailAttachments(attachments),
	})
	if strings.HasPrefix(strings.ToLower(smtpHost), "mock://email/") {
		return pushDeliveryResult{
			Status:       "success",
			Message:      fmt.Sprintf("邮件模拟推送成功，共 %d 人", len(recipients)),
			RequestBody:  requestBody,
			ResponseBody: `{"mock":true,"message":"ok"}`,
		}
	}

	rawMessage := buildEmailMessage(fromAddress, fromName, recipients, subject, textBody, htmlBody, attachments)
	if err := sendSMTPMessage(smtpHost, smtpPort, smtpUsername, smtpPassword, fromAddress, recipients, rawMessage, smtpTimeout(cfg)); err != nil {
		detail := err.Error()
		return pushDeliveryResult{
			Status:       "failed",
			Message:      buildPushFailureMessage("邮件推送失败", detail),
			RequestBody:  requestBody,
			ErrorMessage: detail,
		}
	}

	return pushDeliveryResult{
		Status:       "success",
		Message:      fmt.Sprintf("邮件推送成功，共 %d 人", len(recipients)),
		RequestBody:  requestBody,
		ResponseBody: `{"message":"ok"}`,
	}
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
			errorMessages = append(errorMessages, fmt.Sprintf("%s: %s", payload.ToUser, humanizeWechatPushError(err.Error(), payload)))
			responses = append(responses, map[string]any{
				"touser": payload.ToUser,
				"error":  humanizeWechatPushError(err.Error(), payload),
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
		"const2": {Value: formatWechatAlarmType(alarm.AlarmType)},
		"thing5": {Value: trimWechatThingValue(ctx.ChannelName)},
		"const3": {Value: formatWechatAlarmLevel(alarm.AlarmLevel)},
	}
}

func formatWechatAlarmType(alarmType string) string {
	normalized := strings.ToLower(strings.TrimSpace(alarmType))
	switch normalized {
	case "", "unknown":
		return "设备告警"
	case "motion_detect", "移动侦测":
		return "移动侦测告警"
	case "helmet_missing", "未戴安全帽":
		return "未戴安全帽告警"
	case "intrusion", "区域入侵":
		return "区域入侵告警"
	case "smoke", "烟雾":
		return "烟雾告警"
	case "fire", "明火":
		return "明火告警"
	case "person_fall", "人员跌倒":
		return "人员跌倒告警"
	case "crowd", "人群聚集":
		return "人群聚集告警"
	}

	value := strings.TrimSpace(alarmType)
	if value == "" {
		return "设备告警"
	}
	if strings.HasSuffix(value, "告警") {
		return value
	}
	return value + "告警"
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

func pushChannelAllowed(values []string, target string) bool {
	normalizedTarget := normalizePushChannel(target)
	for _, value := range values {
		if normalizePushChannel(value) == normalizedTarget {
			return true
		}
	}
	return false
}

func normalizePushChannel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "email", "mail", "smtp", "邮件", "邮箱":
		return "email"
	case "wechat", "weixin", "微信":
		return "wechat"
	case "dingtalk", "dingding", "钉钉":
		return "dingtalk"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func buildPushFailureMessage(summary, detail string) string {
	detail = compactPushErrorDetail(detail)
	if detail == "" {
		return summary
	}
	return summary + "： " + detail
}

func humanizeWechatPushError(detail string, payload wechatTemplateMessagePayload) string {
	detail = strings.TrimSpace(detail)
	switch {
	case strings.Contains(detail, "data.const2.value invalid"):
		return detail + fmt.Sprintf("；请确认微信模板“告警原因”枚举值已包含“%s”", payload.Data["const2"].Value)
	case strings.Contains(detail, "data.const3.value invalid"):
		return detail + fmt.Sprintf("；请确认微信模板“告警系统”枚举值已包含“%s”", payload.Data["const3"].Value)
	case strings.Contains(detail, "47003"):
		return detail + "；这通常表示模板字段值与微信模板类型或枚举配置不匹配"
	default:
		return detail
	}
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

func resolveEmailSenderConfig(cfg *config.Config) (string, int, string, string, string, string, error) {
	if cfg == nil {
		return "", 0, "", "", "", "", fmt.Errorf("未加载 .env 邮件配置")
	}
	smtpHost := strings.TrimSpace(cfg.PushEmailSMTPHost)
	smtpPort := cfg.PushEmailSMTPPort
	smtpUsername := strings.TrimSpace(cfg.PushEmailUsername)
	smtpPassword := strings.TrimSpace(cfg.PushEmailPassword)
	fromAddress := strings.TrimSpace(cfg.PushEmailFrom)
	fromName := strings.TrimSpace(cfg.PushEmailFromName)
	if smtpHost == "" {
		smtpHost = config.DefaultPushEmailSMTPHost
	}
	if smtpPort <= 0 {
		smtpPort = config.DefaultPushEmailSMTPPort
	}
	if smtpUsername == "" {
		smtpUsername = config.DefaultPushEmailUsername
	}
	if smtpPassword == "" {
		smtpPassword = config.DefaultPushEmailPassword
	}
	if fromAddress == "" {
		fromAddress = config.DefaultPushEmailFrom
	}
	if fromName == "" {
		fromName = config.DefaultPushEmailFromName
	}
	if fromAddress == "" {
		fromAddress = smtpUsername
	}
	if smtpHost == "" {
		return "", 0, "", "", "", "", fmt.Errorf("缺少 PUSH_EMAIL_SMTP_HOST")
	}
	if smtpPort <= 0 {
		return "", 0, "", "", "", "", fmt.Errorf("缺少有效的 PUSH_EMAIL_SMTP_PORT")
	}
	if smtpUsername == "" {
		return "", 0, "", "", "", "", fmt.Errorf("缺少 PUSH_EMAIL_SMTP_USERNAME")
	}
	if smtpPassword == "" {
		return "", 0, "", "", "", "", fmt.Errorf("缺少 PUSH_EMAIL_SMTP_PASSWORD")
	}
	if fromAddress == "" {
		return "", 0, "", "", "", "", fmt.Errorf("缺少 PUSH_EMAIL_FROM")
	}
	if _, err := mail.ParseAddress(fromAddress); err != nil {
		return "", 0, "", "", "", "", fmt.Errorf("PUSH_EMAIL_FROM 格式无效: %w", err)
	}
	return smtpHost, smtpPort, smtpUsername, smtpPassword, fromAddress, fromName, nil
}

func resolveEmailRecipients(raw string) ([]string, error) {
	values := decodeJSONStringSlice(raw)
	if len(values) == 0 {
		return nil, fmt.Errorf("未配置接收邮箱")
	}
	recipients := make([]string, 0, len(values))
	for _, item := range values {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		address, err := mail.ParseAddress(trimmed)
		if err != nil {
			return nil, fmt.Errorf("邮箱地址 %q 格式无效", trimmed)
		}
		recipients = append(recipients, address.Address)
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("未配置有效的接收邮箱")
	}
	return recipients, nil
}

func buildAlarmEmailSubject(alarm entity.AlarmRecord) string {
	level := strings.TrimSpace(formatWechatAlarmLevel(alarm.AlarmLevel))
	if level == "" {
		level = "告警"
	}
	alarmType := strings.TrimSpace(formatWechatAlarmType(alarm.AlarmType))
	if alarmType == "" {
		alarmType = "设备告警"
	}
	return fmt.Sprintf("[%s] %s", level, alarmType)
}

func buildAlarmEmailTextBody(alarm entity.AlarmRecord, ctx alarmPushContext) string {
	lines := []string{
		"安全管理平台检测到新的告警，请及时处理。",
		"",
		"告警编号: " + fallbackText(alarm.AlarmNo),
		"告警时间: " + alarm.AlarmTime.Format("2006-01-02 15:04:05"),
		"告警类型: " + formatWechatAlarmType(alarm.AlarmType),
		"告警等级: " + fallbackText(formatWechatAlarmLevel(alarm.AlarmLevel)),
		"关联通道: " + fallbackText(ctx.ChannelName),
		"告警说明: " + fallbackText(alarm.Message),
	}
	if imageURL := strings.TrimSpace(alarm.ImageURL); imageURL != "" {
		lines = append(lines, "告警图片: "+imageURL)
	}
	return strings.Join(lines, "\n")
}

func buildAlarmEmailHTMLBody(alarm entity.AlarmRecord, ctx alarmPushContext) string {
	rows := []string{
		buildEmailTableRow("告警编号", html.EscapeString(fallbackText(alarm.AlarmNo))),
		buildEmailTableRow("告警时间", html.EscapeString(alarm.AlarmTime.Format("2006-01-02 15:04:05"))),
		buildEmailTableRow("告警类型", html.EscapeString(formatWechatAlarmType(alarm.AlarmType))),
		buildEmailTableRow("告警等级", html.EscapeString(fallbackText(formatWechatAlarmLevel(alarm.AlarmLevel)))),
		buildEmailTableRow("关联通道", html.EscapeString(fallbackText(ctx.ChannelName))),
		buildEmailTableRow("告警说明", html.EscapeString(fallbackText(alarm.Message))),
	}
	if imageURL := strings.TrimSpace(alarm.ImageURL); imageURL != "" {
		escapedURL := html.EscapeString(imageURL)
		rows = append(rows, buildEmailTableRow("告警图片", fmt.Sprintf("<a href=\"%s\" target=\"_blank\">%s</a>", escapedURL, escapedURL)))
		rows = append(rows, fmt.Sprintf(
			"<div style=\"margin-top:16px\"><img src=\"%s\" alt=\"alarm-image\" style=\"max-width:100%%;border-radius:10px;border:1px solid #dbe4ee\" /></div>",
			escapedURL,
		))
	}
	return strings.Join([]string{
		"<div style=\"font-family:Arial,'Microsoft YaHei',sans-serif;color:#1f2d3d;line-height:1.75\">",
		"<h3 style=\"margin:0 0 12px;color:#17375e\">安全管理平台告警通知</h3>",
		"<p style=\"margin:0 0 16px;color:#4b5f76\">系统检测到新的告警，请及时核查。</p>",
		"<table style=\"border-collapse:collapse\">",
		strings.Join(rows, ""),
		"</table>",
		"</div>",
	}, "")
}

func buildEmailTableRow(label, value string) string {
	return fmt.Sprintf(
		"<tr><td style=\"padding:6px 14px 6px 0;color:#6b7280;vertical-align:top\">%s</td><td style=\"padding:6px 0\">%s</td></tr>",
		html.EscapeString(label),
		value,
	)
}

func fallbackText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func buildEmailMessage(fromAddress, fromName string, recipients []string, subject, textBody, htmlBody string, attachments []emailAttachment) []byte {
	fromHeader := fromAddress
	if strings.TrimSpace(fromName) != "" {
		fromHeader = (&mail.Address{Name: fromName, Address: fromAddress}).String()
	}
	var message bytes.Buffer
	headers := []string{
		"From: " + fromHeader,
		"To: " + strings.Join(recipients, ", "),
		"Subject: " + mime.BEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Date: " + time.Now().Format(time.RFC1123Z),
	}
	if len(attachments) == 0 {
		headers = append(headers, "Content-Type: multipart/alternative; boundary=\"secmgmt-alt\"")
		headers = append(headers, "", "--secmgmt-alt", "Content-Type: text/plain; charset=UTF-8", "Content-Transfer-Encoding: 8bit", "", textBody, "", "--secmgmt-alt", "Content-Type: text/html; charset=UTF-8", "Content-Transfer-Encoding: 8bit", "", htmlBody, "", "--secmgmt-alt--", "")
		return []byte(strings.Join(headers, "\r\n"))
	}
	message.WriteString(strings.Join(headers, "\r\n"))
	message.WriteString("\r\n")

	mixedWriter := multipart.NewWriter(&message)
	_, _ = fmt.Fprintf(&message, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mixedWriter.Boundary())

	altHeader := textproto.MIMEHeader{}
	altHeader.Set("Content-Type", "multipart/alternative; boundary=\"secmgmt-alt\"")
	altPart, _ := mixedWriter.CreatePart(altHeader)
	_, _ = altPart.Write([]byte(strings.Join([]string{
		"--secmgmt-alt",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		textBody,
		"",
		"--secmgmt-alt",
		"Content-Type: text/html; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		htmlBody,
		"",
		"--secmgmt-alt--",
		"",
	}, "\r\n")))

	for _, attachment := range attachments {
		partHeader := textproto.MIMEHeader{}
		partHeader.Set("Content-Type", attachment.ContentType)
		partHeader.Set("Content-Transfer-Encoding", "base64")
		partHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", attachment.Filename))
		part, err := mixedWriter.CreatePart(partHeader)
		if err != nil {
			continue
		}
		_, _ = part.Write([]byte(wrapBase64(base64.StdEncoding.EncodeToString(attachment.Content))))
	}
	_ = mixedWriter.Close()
	return message.Bytes()
}

func sendSMTPMessage(host string, port int, username, password, from string, recipients []string, message []byte, timeout time.Duration) error {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: timeout}

	var conn net.Conn
	var err error
	if port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: host})
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer client.Close()

	if port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return err
			}
		}
	}

	if strings.TrimSpace(username) != "" {
		auth := smtp.PlainAuth("", username, password, host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}

	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func smtpTimeout(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.PushHTTPTimeoutSeconds <= 0 {
		return 10 * time.Second
	}
	return time.Duration(cfg.PushHTTPTimeoutSeconds) * time.Second
}

func resolveAlarmEmailAttachments(cfg *config.Config, alarm entity.AlarmRecord) ([]emailAttachment, error) {
	imageURL := strings.TrimSpace(alarm.ImageURL)
	if imageURL == "" {
		return nil, nil
	}
	attachment, err := resolveAlarmImageAttachment(cfg, imageURL, alarm.AlarmNo)
	if err != nil {
		return nil, err
	}
	if attachment == nil {
		return nil, nil
	}
	return []emailAttachment{*attachment}, nil
}

func resolveAlarmImageAttachment(cfg *config.Config, imageURL, alarmNo string) (*emailAttachment, error) {
	if cfg == nil {
		return nil, fmt.Errorf("未加载系统配置，无法解析截图附件")
	}
	imagePath, err := resolveMediaFilePath(cfg, imageURL)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("读取告警截图失败: %w", err)
	}
	filename := buildAlarmImageAttachmentName(alarmNo, imagePath)
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(imagePath)))
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(content)
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	return &emailAttachment{
		Filename:    filename,
		ContentType: contentType,
		Content:     content,
	}, nil
}

func resolveMediaFilePath(cfg *config.Config, imageURL string) (string, error) {
	mediaRoot := strings.TrimSpace(cfg.MediaRootDir)
	mediaMountPath := strings.TrimSpace(cfg.MediaMountPath)
	if mediaRoot == "" || mediaMountPath == "" {
		return "", fmt.Errorf("媒体目录配置不完整")
	}
	parsedURL, err := url.Parse(imageURL)
	if err != nil {
		return "", fmt.Errorf("截图地址无效: %w", err)
	}
	relativeURLPath := strings.TrimPrefix(parsedURL.Path, strings.TrimRight(mediaMountPath, "/")+"/")
	if relativeURLPath == parsedURL.Path {
		return "", fmt.Errorf("截图地址不在媒体目录下: %s", imageURL)
	}
	mediaRootAbs, err := filepath.Abs(mediaRoot)
	if err != nil {
		mediaRootAbs = mediaRoot
	}
	filePath := filepath.Join(mediaRootAbs, filepath.FromSlash(relativeURLPath))
	filePathAbs, err := filepath.Abs(filePath)
	if err != nil {
		filePathAbs = filePath
	}
	relativePath, err := filepath.Rel(mediaRootAbs, filePathAbs)
	if err != nil || strings.HasPrefix(relativePath, "..") {
		return "", fmt.Errorf("截图路径越界: %s", imageURL)
	}
	return filePathAbs, nil
}

func buildAlarmImageAttachmentName(alarmNo, imagePath string) string {
	ext := strings.ToLower(filepath.Ext(imagePath))
	if ext == "" {
		ext = ".jpg"
	}
	baseName := strings.TrimSpace(alarmNo)
	if baseName == "" {
		baseName = "alarm-snapshot"
	}
	baseName = strings.NewReplacer("\\", "-", "/", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-").Replace(baseName)
	return baseName + ext
}

func summarizeEmailAttachments(attachments []emailAttachment) []map[string]any {
	if len(attachments) == 0 {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(attachments))
	for _, item := range attachments {
		result = append(result, map[string]any{
			"filename":    item.Filename,
			"contentType": item.ContentType,
			"size":        len(item.Content),
		})
	}
	return result
}

func wrapBase64(value string) string {
	if value == "" {
		return ""
	}
	const lineLength = 76
	var builder strings.Builder
	for start := 0; start < len(value); start += lineLength {
		end := start + lineLength
		if end > len(value) {
			end = len(value)
		}
		builder.WriteString(value[start:end])
		builder.WriteString("\r\n")
	}
	return builder.String()
}
