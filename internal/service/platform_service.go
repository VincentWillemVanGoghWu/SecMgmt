package service

import (
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/repository"

	driverMysql "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PlatformService struct {
	cfg                  *config.Config
	repo                 *repository.Repository
	logger               *zap.Logger
	hikvisionBridge      *HikvisionAlarmBridgeService
	smartBridgeReconnect *SmartBridgeReconnectService
	deviceCheckMu        sync.Mutex
}

var hiddenMenuCodeSet = map[string]struct{}{
	"safety-data-items": {},
	"monitor-config":    {},
}

var hiddenPermissionCodeSet = map[string]struct{}{
	"ai:event:callback": {},
}

var ErrDeviceDeleteForbidden = errors.New("device delete forbidden")
var ErrAccessDenied = errors.New("access denied")

func NewPlatformService(cfg *config.Config, repo *repository.Repository, logger *zap.Logger) *PlatformService {
	return &PlatformService{cfg: cfg, repo: repo, logger: logger}
}

func (s *PlatformService) SetHikvisionAlarmBridge(bridge *HikvisionAlarmBridgeService) {
	s.hikvisionBridge = bridge
}

func (s *PlatformService) SetSmartBridgeReconnectService(reconnect *SmartBridgeReconnectService) {
	s.smartBridgeReconnect = reconnect
}

func (s *PlatformService) tryReloadHikvisionBridge(trigger string) {
	if s.hikvisionBridge == nil {
		return
	}
	if err := s.hikvisionBridge.Start(); err != nil && s.logger != nil {
		s.logger.Warn("reload hikvision alarm bridge",
			zap.String("trigger", trigger),
			zap.Error(err),
		)
	}
}

func (s *PlatformService) reloadHikvisionBridgeForProvider(providerCode, trigger string) {
	if providerCode != "hikvision-sdk" {
		return
	}
	s.tryReloadHikvisionBridge(trigger)
}

func (s *PlatformService) reloadHikvisionBridgeForBinding(providerCode, capabilityCode, trigger string) {
	if providerCode != "hikvision-sdk" || capabilityCode != "motion_detect" {
		return
	}
	s.tryReloadHikvisionBridge(trigger)
}

func shouldReloadHikvisionProvider(providerCode string) bool {
	return providerCode == "hikvision-sdk"
}

func shouldReloadHikvisionBinding(providerCode, capabilityCode string) bool {
	return providerCode == "hikvision-sdk" && capabilityCode == "motion_detect"
}

type UserPayload struct {
	Username string `json:"username"`
	RealName string `json:"realName"`
	DeptID   *uint  `json:"deptId"`
	Status   string `json:"status"`
	RoleIDs  []uint `json:"roleIds"`
	Password string `json:"password"`
}

type UserListFilter struct {
	Keyword string
	Status  string
	DeptID  uint
	RoleID  uint
}

type RolePayload struct {
	RoleCode string  `json:"roleCode"`
	RoleName string  `json:"roleName"`
	Status   string  `json:"status"`
	Remark   *string `json:"remark"`
}

type RoleListFilter struct {
	Keyword string
	Status  string
}

type RoleStatusPayload struct {
	Status string `json:"status"`
}

type RoleDataScopePayload struct {
	DataScopeType  string `json:"dataScopeType"`
	DataScopeValue any    `json:"dataScopeValue"`
}

type RoleMenuPayload struct {
	MenuIDs []uint `json:"menuIds"`
}

type RolePermissionPayload struct {
	PermissionIDs []uint `json:"permissionIds"`
}

type StatusPayload struct {
	Status string `json:"status"`
}

type PushConfigListFilter struct {
	Keyword      string
	ProviderType string
	Enabled      *bool
	AccessScope  *AccessScope
}

type PushLogListFilter struct {
	Channel     string
	Status      string
	AlarmType   string
	StartAt     *time.Time
	EndAt       *time.Time
	AccessScope *AccessScope
}

type DeviceStatusLogListFilter struct {
	DeviceType string
	DeviceName string
	Status     string
	StartAt    *time.Time
	EndAt      *time.Time
}

type DeviceCheckSchedulePayload struct {
	Name            string `json:"name"`
	Enabled         bool   `json:"enabled"`
	FrequencyPerDay int    `json:"frequencyPerDay"`
	NotifyEnabled   bool   `json:"notifyEnabled"`
	PushConfigIDs   []uint `json:"pushConfigIds"`
	NotifyMode      string `json:"notifyMode"`
}

type DeviceCheckScheduleStatusPayload struct {
	Enabled bool `json:"enabled"`
}

type FactoryPayload struct {
	FactoryCode string  `json:"factoryCode"`
	FactoryName string  `json:"factoryName"`
	Status      string  `json:"status"`
	Remark      *string `json:"remark"`
}

type ZonePayload struct {
	FactoryID uint    `json:"factoryId"`
	ZoneCode  string  `json:"zoneCode"`
	ZoneName  string  `json:"zoneName"`
	Status    string  `json:"status"`
	Remark    *string `json:"remark"`
}

type DeptPayload struct {
	DeptCode  string  `json:"deptCode"`
	DeptName  string  `json:"deptName"`
	ParentID  *uint   `json:"parentId"`
	FactoryID *uint   `json:"factoryId"`
	ZoneID    *uint   `json:"zoneId"`
	Leader    *string `json:"leader"`
	Phone     *string `json:"phone"`
	Sort      int     `json:"sort"`
	Status    string  `json:"status"`
	Remark    *string `json:"remark"`
}

type DictTypePayload struct {
	DictCode string  `json:"dictCode"`
	DictName string  `json:"dictName"`
	Status   string  `json:"status"`
	Remark   *string `json:"remark"`
}

type DictItemPayload struct {
	DictTypeID uint    `json:"dictTypeId"`
	ItemLabel  string  `json:"itemLabel"`
	ItemValue  string  `json:"itemValue"`
	ItemSort   int     `json:"itemSort"`
	IsDefault  bool    `json:"isDefault"`
	Status     string  `json:"status"`
	Remark     *string `json:"remark"`
}

type CameraPayload struct {
	DeviceCode      string  `json:"deviceCode"`
	Name            string  `json:"name"`
	IP              string  `json:"ip"`
	SDKPort         int     `json:"sdkPort"`
	HTTPPort        int     `json:"httpPort"`
	RTSPPort        int     `json:"rtspPort"`
	Username        string  `json:"username"`
	Password        string  `json:"password"`
	FactoryID       uint    `json:"factoryId"`
	ZoneID          uint    `json:"zoneId"`
	InstallLocation *string `json:"installLocation"`
	SupportAI       bool    `json:"supportAi"`
	Status          string  `json:"status"`
	Remark          *string `json:"remark"`
}

type RecorderPayload struct {
	DeviceCode   string `json:"deviceCode"`
	Name         string `json:"name"`
	IP           string `json:"ip"`
	SDKPort      int    `json:"sdkPort"`
	HTTPPort     int    `json:"httpPort"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	ChannelCount int    `json:"channelCount"`
	FactoryID    uint   `json:"factoryId"`
	Status       string `json:"status"`
}

type ChannelPayload struct {
	Name            string `json:"name"`
	CameraID        *uint  `json:"cameraId"`
	FactoryID       uint   `json:"factoryId"`
	ZoneID          *uint  `json:"zoneId"`
	Enabled         bool   `json:"enabled"`
	SupportPlayback bool   `json:"supportPlayback"`
	Status          string `json:"status"`
}

type AlarmProcessPayload struct {
	Status string  `json:"status"`
	Remark *string `json:"remark"`
}

type PushConfigPayload struct {
	ConfigName             string              `json:"configName"`
	ProviderType           string              `json:"providerType"`
	Webhook                *string             `json:"webhook"`
	AppID                  *string             `json:"appId"`
	TemplateID             *string             `json:"templateId"`
	ReceiverOpenIDs        []string            `json:"receiverOpenIds"`
	FactoryIDs             []uint              `json:"factoryIds"`
	ZoneIDs                []uint              `json:"zoneIds"`
	AlarmTypes             []string            `json:"alarmTypes"`
	AlarmLevels            []string            `json:"alarmLevels"`
	ActiveTimeRanges       []map[string]string `json:"activeTimeRanges"`
	Enabled                bool                `json:"enabled"`
	RateLimitWindowSeconds int                 `json:"rateLimitWindowSeconds"`
	RateLimitMaxCount      int                 `json:"rateLimitMaxCount"`
	RetryMaxCount          int                 `json:"retryMaxCount"`
	RetryIntervalSeconds   int                 `json:"retryIntervalSeconds"`
	Remark                 *string             `json:"remark"`
	Secret                 *string             `json:"secret"`
	AppSecret              *string             `json:"appSecret"`
}

type PushConfigStatusPayload struct {
	Enabled bool `json:"enabled"`
}

type SmartProviderPayload struct {
	ProviderCode string  `json:"providerCode"`
	ProviderName string  `json:"providerName"`
	ProviderType string  `json:"providerType"`
	AuthType     string  `json:"authType"`
	BaseURL      *string `json:"baseUrl"`
	CallbackPath *string `json:"callbackPath"`
	Secret       *string `json:"secret"`
	ConfigSchema any     `json:"configSchema"`
	Enabled      bool    `json:"enabled"`
	Remark       *string `json:"remark"`
}

type SmartBindingPayload struct {
	ProviderCode     string `json:"providerCode"`
	CapabilityCode   string `json:"capabilityCode"`
	SourceType       string `json:"sourceType"`
	SourceID         uint   `json:"sourceId"`
	Enabled          bool   `json:"enabled"`
	Priority         int    `json:"priority"`
	ConnectionConfig any    `json:"connectionConfig"`
}

type SmartBindingRulePayload struct {
	RuleName              string   `json:"ruleName"`
	Enabled               bool     `json:"enabled"`
	AlarmEnabled          bool     `json:"alarmEnabled"`
	AlarmLevel            string   `json:"alarmLevel"`
	DedupWindowSeconds    int      `json:"dedupWindowSeconds"`
	CooldownSeconds       int      `json:"cooldownSeconds"`
	MinConfidence         *float64 `json:"minConfidence"`
	ActiveTimePlan        any      `json:"activeTimePlan"`
	SnapshotEnabled       bool     `json:"snapshotEnabled"`
	RecordClipEnabled     bool     `json:"recordClipEnabled"`
	RecordPreSeconds      int      `json:"recordPreSeconds"`
	RecordPostSeconds     int      `json:"recordPostSeconds"`
	PushEnabled           bool     `json:"pushEnabled"`
	PushChannels          []string `json:"pushChannels"`
	SendToAI              bool     `json:"sendToAi"`
	AIFlowCode            *string  `json:"aiFlowCode"`
	GenerateAlarmDirectly bool     `json:"generateAlarmDirectly"`
	Remark                *string  `json:"remark"`
}

type SmartBindingListFilter struct {
	SourceType     string
	ProviderCode   string
	CapabilityCode string
	Enabled        *bool
}

type SmartRawEventListFilter struct {
	ProviderCode   string
	CapabilityCode string
	ParseStatus    string
	SourceType     string
	RecentDays     int
}

type SmartEventListFilter struct {
	Keyword        string
	ProviderCode   string
	CapabilityCode string
	Status         string
	SourceStage    string
	RecentDays     int
	AccessScope    *AccessScope
}

type SmartBridgeReconnectLogListFilter struct {
	Status        string
	Action        string
	TriggerReason string
	DeviceType    string
	DeviceID      uint
	SessionKey    string
	StartAt       *time.Time
	EndAt         *time.Time
}

type SmartAITaskListFilter struct {
	Status     string
	AIFlowCode string
	RecentDays int
}

type SmartAIReviewPayload struct {
	AIFlowCode string  `json:"aiFlowCode"`
	ModelCode  *string `json:"modelCode"`
	Force      bool    `json:"force"`
}

type SmartAICallbackPayload struct {
	TaskNo     string   `json:"taskNo"`
	Decision   string   `json:"decision"`
	Labels     []string `json:"labels"`
	Confidence *float64 `json:"confidence"`
	Reason     *string  `json:"reason"`
	Evidence   any      `json:"evidence"`
	Raw        any      `json:"raw"`
}

type nameValueRow struct {
	Name  string `gorm:"column:name"`
	Value int64  `gorm:"column:value"`
}

func (s *PlatformService) db() *gorm.DB {
	return s.repo.DB()
}

func (s *PlatformService) ensureCameraAccessible(accessScope *AccessScope, cameraID uint) (*entity.CameraDevice, error) {
	var item entity.CameraDevice
	if err := s.db().First(&item, cameraID).Error; err != nil {
		return nil, err
	}
	if accessScope != nil && !accessScope.AllowsCamera(item.FactoryID, item.ZoneID, item.ID) {
		return nil, ErrAccessDenied
	}
	return &item, nil
}

func (s *PlatformService) ensureRecorderAccessible(accessScope *AccessScope, recorderID uint) (*entity.RecorderDevice, error) {
	var item entity.RecorderDevice
	if err := s.db().First(&item, recorderID).Error; err != nil {
		return nil, err
	}
	if accessScope != nil && !accessScope.AllowsRecorder(item.FactoryID, item.ID) {
		return nil, ErrAccessDenied
	}
	return &item, nil
}

func (s *PlatformService) ensureChannelAccessible(accessScope *AccessScope, channelID uint) (*entity.RecorderChannel, error) {
	var item entity.RecorderChannel
	if err := s.db().First(&item, channelID).Error; err != nil {
		return nil, err
	}
	if accessScope != nil && !accessScope.AllowsChannel(item.FactoryID, item.ZoneID, item.CameraID, item.RecorderID, item.ID) {
		return nil, ErrAccessDenied
	}
	return &item, nil
}

func (s *PlatformService) ensureAlarmAccessible(accessScope *AccessScope, alarmID uint) (*entity.AlarmRecord, error) {
	var item entity.AlarmRecord
	if err := s.db().First(&item, alarmID).Error; err != nil {
		return nil, err
	}
	if accessScope != nil && !accessScope.AllowsAlarm(item.FactoryID, item.ZoneID, item.CameraID, item.RecorderID, item.ChannelID) {
		return nil, ErrAccessDenied
	}
	return &item, nil
}

func validateCameraTargetAccess(accessScope *AccessScope, factoryID, zoneID uint) error {
	if accessScope == nil || accessScope.AllowsCamera(factoryID, zoneID, 0) {
		return nil
	}
	return ErrAccessDenied
}

func validateRecorderTargetAccess(accessScope *AccessScope, factoryID uint) error {
	if accessScope == nil || accessScope.AllowsRecorder(factoryID, 0) {
		return nil
	}
	return ErrAccessDenied
}

func validateChannelTargetAccess(accessScope *AccessScope, factoryID uint, zoneID, cameraID *uint, recorderID, channelID uint) error {
	if accessScope == nil || accessScope.AllowsChannel(factoryID, zoneID, cameraID, recorderID, channelID) {
		return nil
	}
	return ErrAccessDenied
}

func buildPlaybackDownloadFilename(alarmNo, recorderName, channelName string, startTime, endTime time.Time) string {
	prefix := sanitizePlaybackFilenamePart(alarmNo)
	if prefix == "" {
		recorderPart := sanitizePlaybackFilenamePart(recorderName)
		channelPart := sanitizePlaybackFilenamePart(channelName)
		switch {
		case recorderPart != "" && channelPart != "":
			prefix = recorderPart + "_" + channelPart
		case recorderPart != "":
			prefix = recorderPart
		case channelPart != "":
			prefix = channelPart
		default:
			prefix = "playback"
		}
	}
	return fmt.Sprintf(
		"%s_%s-%s.mp4",
		prefix,
		startTime.Local().Format("20060102-150405"),
		endTime.Local().Format("20060102-150405"),
	)
}

func sanitizePlaybackFilenamePart(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	lastUnderscore := false
	for _, char := range trimmed {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
			builder.WriteRune(char)
			lastUnderscore = false
		case char == '-', char == '_':
			if !lastUnderscore && builder.Len() > 0 {
				builder.WriteRune('_')
				lastUnderscore = true
			}
		default:
			if !lastUnderscore && builder.Len() > 0 {
				builder.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(builder.String(), "_")
}

func (s *PlatformService) ExportCSV(kind string, accessScope *AccessScope) ([]byte, string, error) {
	var rows [][]string
	switch kind {
	case "alarms":
		page, err := NewQueryService(s.repo).ListAlarms(1, 5000, dto.AlarmListFilter{}, accessScope)
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, []string{"ID", "告警编号", "级别", "状态", "厂区", "区域", "时间"})
		for _, item := range page.Items {
			rows = append(rows, []string{
				fmt.Sprintf("%d", item.ID),
				item.AlarmNo,
				item.AlarmLevel,
				item.Status,
				valueOrEmpty(item.FactoryName),
				valueOrEmpty(item.ZoneName),
				item.AlarmTime,
			})
		}
	case "device-status":
		data, err := s.ListDeviceStatusLogs(1, 5000, DeviceStatusLogListFilter{}, accessScope)
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, []string{"ID", "设备类型", "设备ID", "设备名称", "原状态", "新状态", "检查时间"})
		for _, raw := range data["items"].([]map[string]any) {
			rows = append(rows, []string{
				fmt.Sprint(raw["id"]),
				fmt.Sprint(raw["deviceType"]),
				fmt.Sprint(raw["deviceId"]),
				fmt.Sprint(raw["deviceName"]),
				fmt.Sprint(raw["oldStatus"]),
				fmt.Sprint(raw["newStatus"]),
				fmt.Sprint(raw["checkedAt"]),
			})
		}
	case "push-logs":
		data, err := s.ListPushLogs(1, 5000, PushLogListFilter{AccessScope: accessScope})
		if err != nil {
			return nil, "", err
		}
		rows = append(rows, []string{"ID", "告警编号", "渠道", "状态", "厂区ID", "区域ID", "推送时间"})
		for _, raw := range data["items"].([]map[string]any) {
			rows = append(rows, []string{
				fmt.Sprint(raw["id"]),
				fmt.Sprint(raw["alarmNo"]),
				fmt.Sprint(raw["channel"]),
				fmt.Sprint(raw["status"]),
				fmt.Sprint(raw["factoryId"]),
				fmt.Sprint(raw["zoneId"]),
				fmt.Sprint(raw["pushedAt"]),
			})
		}
	default:
		rows = append(rows, []string{"message"}, []string{"unsupported export kind"})
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.WriteAll(rows)
	writer.Flush()
	return buffer.Bytes(), fmt.Sprintf("%s_%s.csv", kind, time.Now().Format("20060102150405")), writer.Error()
}

func (s *PlatformService) replaceUserRoles(userID uint, roleIDs []uint) error {
	if err := s.db().Table("sys_user_role").Where("user_id = ?", userID).Delete(nil).Error; err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if err := s.db().Table("sys_user_role").Create(map[string]any{"user_id": userID, "role_id": roleID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *PlatformService) zoneRecord(zoneID uint) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListZones(ZoneListFilter{})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == zoneID {
			return zoneDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) deptRecord(deptID uint) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListDepts(DeptListFilter{})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == deptID {
			return deptDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) dictTypeRecord(dictTypeID uint) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListDictTypes(DictTypeListFilter{})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == dictTypeID {
			return dictTypeDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) dictItemRecord(itemID uint) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListDictTypes(DictTypeListFilter{})
	if err != nil {
		return nil, err
	}
	for _, dictType := range items {
		for _, raw := range dictType.Items {
			if raw.ID == itemID {
				return dictItemDTOToMap(raw), nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) resolveProviderCapabilityIDs(providerCode, capabilityCode string) (uint, uint, error) {
	var provider entity.SmartInterfaceProvider
	if err := s.db().Where("provider_code = ?", providerCode).First(&provider).Error; err != nil {
		return 0, 0, err
	}
	var capability entity.SmartInterfaceCapability
	if err := s.db().Where("capability_code = ?", capabilityCode).First(&capability).Error; err != nil {
		return 0, 0, err
	}
	return provider.ID, capability.ID, nil
}

func (s *PlatformService) smartProviderMap(item entity.SmartInterfaceProvider) map[string]any {
	var bindingRows []entity.SmartDeviceBinding
	_ = s.db().Where("provider_id = ?", item.ID).Find(&bindingRows).Error
	codes := []string{}
	names := []string{}
	for _, binding := range bindingRows {
		var capability entity.SmartInterfaceCapability
		if err := s.db().First(&capability, binding.CapabilityID).Error; err == nil {
			if !containsString(codes, capability.CapabilityCode) {
				codes = append(codes, capability.CapabilityCode)
				names = append(names, capability.CapabilityName)
			}
		}
	}
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
		"capabilityCodes":  codes,
		"capabilityNames":  names,
		"updatedAt":        item.UpdatedAt.Format(time.RFC3339),
		"createdAt":        item.CreatedAt.Format(time.RFC3339),
	}
}

func (s *PlatformService) smartBindingMap(item entity.SmartDeviceBinding, provider entity.SmartInterfaceProvider, capability entity.SmartInterfaceCapability, ruleCount int) map[string]any {
	sourceName, sourcePath := s.resolveSourceName(item.SourceType, item.SourceID)
	var lastEvent entity.SmartEvent
	lastEventTime := any(nil)
	if err := s.db().Where("binding_id = ?", item.ID).Order("event_time DESC").First(&lastEvent).Error; err == nil {
		lastEventTime = lastEvent.EventTime.Format(time.RFC3339)
	}
	return map[string]any{
		"id":                    item.ID,
		"providerId":            item.ProviderID,
		"providerCode":          provider.ProviderCode,
		"providerName":          provider.ProviderName,
		"capabilityId":          item.CapabilityID,
		"capabilityCode":        capability.CapabilityCode,
		"capabilityName":        capability.CapabilityName,
		"sourceType":            item.SourceType,
		"sourceId":              item.SourceID,
		"sourceName":            sourceName,
		"sourcePath":            sourcePath,
		"enabled":               item.Enabled,
		"priority":              item.Priority,
		"connectionConfig":      decodeJSONAny(item.ConnectionConfigJSON),
		"sendToAi":              false,
		"generateAlarmDirectly": true,
		"ruleCount":             ruleCount,
		"lastEventTime":         lastEventTime,
		"updatedAt":             item.UpdatedAt.Format(time.RFC3339),
		"createdAt":             item.CreatedAt.Format(time.RFC3339),
	}
}

func (s *PlatformService) resolveSourceName(sourceType string, sourceID uint) (string, string) {
	switch sourceType {
	case "camera":
		var item entity.CameraDevice
		if err := s.db().First(&item, sourceID).Error; err == nil {
			return item.Name, fmt.Sprintf("camera/%d", item.ID)
		}
	case "recorder":
		var item entity.RecorderDevice
		if err := s.db().First(&item, sourceID).Error; err == nil {
			return item.Name, fmt.Sprintf("recorder/%d", item.ID)
		}
	default:
		var item entity.RecorderChannel
		if err := s.db().First(&item, sourceID).Error; err == nil {
			return item.Name, fmt.Sprintf("channel/%d", item.ID)
		}
	}
	return fmt.Sprintf("%s-%d", sourceType, sourceID), fmt.Sprintf("%s/%d", sourceType, sourceID)
}

func (s *PlatformService) deviceStatusBlock(tableName, deviceType string, accessScope *AccessScope) map[string]any {
	var total, online, offline, disabled int64
	base := s.applyDeviceScopeQuery(s.db().Table(tableName), tableName, accessScope)
	_ = base.Count(&total).Error
	_ = s.applyDeviceScopeQuery(s.db().Table(tableName).Where("status = ?", "online"), tableName, accessScope).Count(&online).Error
	_ = s.applyDeviceScopeQuery(s.db().Table(tableName).Where("status = ?", "offline"), tableName, accessScope).Count(&offline).Error
	_ = s.applyDeviceScopeQuery(s.db().Table(tableName).Where("status = ?", "disabled"), tableName, accessScope).Count(&disabled).Error
	exception := total - online - offline - disabled
	if exception < 0 {
		exception = 0
	}
	return map[string]any{
		"deviceType": deviceType,
		"total":      total,
		"online":     online,
		"offline":    offline,
		"exception":  exception,
		"disabled":   disabled,
		"onlineRate": percent(online, total),
	}
}

func (s *PlatformService) applyAlarmAccessScopeQuery(db *gorm.DB, alias string, accessScope *AccessScope) *gorm.DB {
	return s.applyScopedResourceQuery(
		db,
		alias,
		accessScope,
		"factory_id",
		"zone_id",
		"camera_id",
		"recorder_id",
		"channel_id",
	)
}

func (s *PlatformService) applyPushLogAccessScopeQuery(db *gorm.DB, alias string, accessScope *AccessScope) *gorm.DB {
	return s.applyScopedResourceQuery(
		db,
		alias,
		accessScope,
		"factory_id",
		"zone_id",
		"",
		"",
		"",
	)
}

func (s *PlatformService) applySmartEventAccessScopeQuery(db *gorm.DB, alias string, accessScope *AccessScope) *gorm.DB {
	return s.applyScopedResourceQuery(
		db,
		alias,
		accessScope,
		"factory_id",
		"zone_id",
		"camera_id",
		"recorder_id",
		"channel_id",
	)
}

func (s *PlatformService) applyDeviceScopeQuery(db *gorm.DB, tableName string, accessScope *AccessScope) *gorm.DB {
	switch strings.ToLower(strings.TrimSpace(tableName)) {
	case "camera_device":
		return s.applyScopedResourceQuery(db, tableName, accessScope, "factory_id", "zone_id", "id", "", "")
	case "recorder_device":
		return s.applyScopedResourceQuery(db, tableName, accessScope, "factory_id", "", "", "id", "")
	case "recorder_channel":
		return s.applyScopedResourceQuery(db, tableName, accessScope, "factory_id", "zone_id", "camera_id", "recorder_id", "id")
	default:
		if accessScope == nil || accessScope.All {
			return db
		}
		return db.Where("1 = 0")
	}
}

func (s *PlatformService) applyScopedResourceQuery(db *gorm.DB, alias string, accessScope *AccessScope, factoryColumn, zoneColumn, cameraColumn, recorderColumn, channelColumn string) *gorm.DB {
	if accessScope == nil || accessScope.All {
		return db
	}
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	appendClause := func(column string, ids []uint) {
		if column == "" || len(ids) == 0 {
			return
		}
		clauses = append(clauses, alias+"."+column+" IN ?")
		args = append(args, ids)
	}
	appendClause(factoryColumn, accessScope.FactoryIDs)
	appendClause(zoneColumn, accessScope.ZoneIDs)
	appendClause(cameraColumn, accessScope.CameraIDs)
	appendClause(recorderColumn, accessScope.RecorderIDs)
	appendClause(channelColumn, accessScope.ChannelIDs)
	if len(clauses) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func (s *PlatformService) canAccessPushConfig(item entity.PushConfig, accessScope *AccessScope) bool {
	if accessScope == nil || accessScope.All {
		return true
	}
	factoryIDs := decodeJSONUintSlice(item.FactoryIDsJSON)
	for _, factoryID := range factoryIDs {
		if accessScope.AllowsFactory(factoryID) {
			return true
		}
	}
	zoneIDs := decodeJSONUintSlice(item.ZoneIDsJSON)
	zoneFactoryMap := s.loadZoneFactoryMap(zoneIDs)
	for _, zoneID := range zoneIDs {
		if accessScope.AllowsZone(zoneFactoryMap[zoneID], zoneID) {
			return true
		}
	}
	return false
}

func (s *PlatformService) ensurePushConfigAccessible(id uint, accessScope *AccessScope) (*entity.PushConfig, error) {
	var item entity.PushConfig
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	if !s.canAccessPushConfig(item, accessScope) {
		return nil, ErrAccessDenied
	}
	return &item, nil
}

func (s *PlatformService) validatePushConfigScope(factoryIDs, zoneIDs []uint, accessScope *AccessScope) error {
	if accessScope == nil || accessScope.All {
		return nil
	}
	if len(factoryIDs) == 0 && len(zoneIDs) == 0 {
		return ErrAccessDenied
	}
	for _, factoryID := range factoryIDs {
		if !accessScope.AllowsFactory(factoryID) {
			return ErrAccessDenied
		}
	}
	zoneFactoryMap := s.loadZoneFactoryMap(zoneIDs)
	for _, zoneID := range zoneIDs {
		if !accessScope.AllowsZone(zoneFactoryMap[zoneID], zoneID) {
			return ErrAccessDenied
		}
	}
	return nil
}

func (s *PlatformService) canAccessPushLog(item entity.AlarmPushLog, accessScope *AccessScope) bool {
	if accessScope == nil || accessScope.All {
		return true
	}
	return accessScope.AllowsAlarm(item.FactoryID, item.ZoneID, nil, nil, nil)
}

func (s *PlatformService) ensureSmartEventAccessible(id uint, accessScope *AccessScope) (*entity.SmartEvent, error) {
	var item entity.SmartEvent
	if err := s.db().First(&item, id).Error; err != nil {
		return nil, err
	}
	if accessScope != nil && !accessScope.AllowsAlarm(item.FactoryID, item.ZoneID, item.CameraID, item.RecorderID, item.ChannelID) {
		return nil, ErrAccessDenied
	}
	return &item, nil
}

func (s *PlatformService) loadZoneFactoryMap(zoneIDs []uint) map[uint]uint {
	result := make(map[uint]uint, len(zoneIDs))
	if len(zoneIDs) == 0 {
		return result
	}
	var zones []entity.FactoryZone
	if err := s.db().Where("id IN ?", zoneIDs).Find(&zones).Error; err != nil {
		return result
	}
	for _, zone := range zones {
		result[zone.ID] = zone.FactoryID
	}
	return result
}

func defaultCameraSDKConfig(camera entity.CameraDevice) map[string]any {
	return map[string]any{
		"deviceName":     camera.Name,
		"deviceModel":    "Hikvision Camera",
		"deviceSerialNo": fmt.Sprintf("CAM-%s", camera.DeviceCode),
		"network": map[string]any{
			"supported":    true,
			"ip":           camera.IP,
			"subnetMask":   "255.255.255.0",
			"gateway":      "192.168.1.1",
			"primaryDns":   "8.8.8.8",
			"secondaryDns": "8.8.4.4",
			"dhcpEnabled":  false,
		},
		"image": map[string]any{
			"supported":        true,
			"resolution":       "1920x1080",
			"frameRate":        25,
			"bitrate":          2048,
			"exposureMode":     "auto",
			"exposureTime":     "1/50",
			"whiteBalanceMode": "auto",
		},
		"recording": map[string]any{
			"supported":        true,
			"scheduleMode":     "all_day",
			"storageMode":      "device",
			"overwriteEnabled": true,
			"weeklyPlan":       []map[string]any{},
		},
		"ptz": map[string]any{
			"supported":     true,
			"presetCount":   2,
			"cruiseEnabled": true,
			"trackEnabled":  false,
			"presets":       []map[string]any{{"presetId": 1, "name": "默认点位"}, {"presetId": 2, "name": "门口"}},
		},
		"users": map[string]any{
			"supported": true,
			"items":     []map[string]any{{"userId": 1, "username": camera.Username, "role": "admin", "enabled": true}},
		},
	}
}

func buildPushRecords(items []entity.AlarmPushLog) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"time":         item.PushedAt.Format(time.RFC3339),
			"channel":      item.Channel,
			"status":       item.Status,
			"message":      item.Message,
			"operatorName": nil,
		})
	}
	return result
}

func buildProcessLogs(items []entity.AlarmProcessLog) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"id":           item.ID,
			"action":       item.Action,
			"fromStatus":   nullableString(item.FromStatus),
			"toStatus":     nullableString(item.ToStatus),
			"operatorId":   item.OperatorID,
			"operatorName": nullableString(item.OperatorName),
			"remark":       nullableString(item.Remark),
			"createdAt":    item.CreatedAt.Format(time.RFC3339),
		})
	}
	return result
}

func buildBindingRules(items []entity.SmartBindingRule) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, bindingRuleMap(item))
	}
	return result
}

func (s *PlatformService) buildSmartEvents(items []entity.SmartEvent) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, s.smartEventMapWithRelations(item))
	}
	return result
}

func (s *PlatformService) smartEventMapWithRelations(item entity.SmartEvent) map[string]any {
	base := smartEventMap(item)
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
	if item.ChannelID != nil {
		sourceName, sourcePath := s.resolveSourceName("channel", *item.ChannelID)
		base["sourceName"] = sourceName
		base["sourcePath"] = sourcePath
		return base
	}
	if item.CameraID != nil {
		sourceName, sourcePath := s.resolveSourceName("camera", *item.CameraID)
		base["sourceName"] = sourceName
		base["sourcePath"] = sourcePath
		return base
	}
	if item.RecorderID != nil {
		sourceName, sourcePath := s.resolveSourceName("recorder", *item.RecorderID)
		base["sourceName"] = sourceName
		base["sourcePath"] = sourcePath
	}
	return base
}

func buildAITasks(tasks []entity.AiReviewTask, results []entity.AiReviewResult) []map[string]any {
	latest := make(map[uint]entity.AiReviewResult)
	for _, result := range results {
		if _, exists := latest[result.TaskID]; !exists {
			latest[result.TaskID] = result
		}
	}
	out := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		if result, exists := latest[task.ID]; exists {
			out = append(out, aiTaskMap(task, &result))
		} else {
			out = append(out, aiTaskMap(task, nil))
		}
	}
	return out
}

func buildAIResults(results []entity.AiReviewResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, result := range results {
		out = append(out, aiResultMap(result))
	}
	return out
}

func rowsToItems(rows []nameValueRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"name": row.Name, "value": row.Value})
	}
	return out
}

func buildMenuTreeFromEntities(menus []entity.Menu) []dto.MenuItem {
	byParent := make(map[uint][]entity.Menu)
	roots := make([]entity.Menu, 0)
	for _, menu := range menus {
		if menu.ParentID == nil {
			roots = append(roots, menu)
			continue
		}
		byParent[*menu.ParentID] = append(byParent[*menu.ParentID], menu)
	}

	var walk func(entity.Menu) dto.MenuItem
	walk = func(menu entity.Menu) dto.MenuItem {
		item := dto.MenuItem{
			ID:        menu.ID,
			Key:       menu.Code,
			Label:     menu.Name,
			Icon:      menu.Icon,
			RouteName: menu.RouteName,
			Path:      menu.RoutePath,
		}
		children := byParent[menu.ID]
		if len(children) > 0 {
			item.Children = make([]dto.MenuItem, 0, len(children))
			for _, child := range children {
				item.Children = append(item.Children, walk(child))
			}
		}
		return item
	}

	tree := make([]dto.MenuItem, 0, len(roots))
	for _, root := range roots {
		tree = append(tree, walk(root))
	}
	return tree
}

func keysOfHiddenMenuCodeSet() []string {
	keys := make([]string, 0, len(hiddenMenuCodeSet))
	for code := range hiddenMenuCodeSet {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	return keys
}

func keysOfHiddenPermissionCodeSet() []string {
	keys := make([]string, 0, len(hiddenPermissionCodeSet))
	for code := range hiddenPermissionCodeSet {
		keys = append(keys, code)
	}
	sort.Strings(keys)
	return keys
}

func dedupeUintSlice(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 || containsScopeUint(result, value) {
			continue
		}
		result = append(result, value)
	}
	sortUintAsc(result)
	return result
}

func buildPermissionOptions(permissions []entity.Permission) []dto.PermissionOption {
	options := make([]dto.PermissionOption, 0, len(permissions))
	for _, permission := range permissions {
		moduleKey, resourceKey := parsePermissionCode(permission.Code)
		options = append(options, dto.PermissionOption{
			ID:          permission.ID,
			Name:        permission.Name,
			Code:        permission.Code,
			IsButton:    permission.IsButton,
			ModuleKey:   moduleKey,
			ResourceKey: resourceKey,
		})
	}
	return options
}

func parsePermissionCode(code string) (string, string) {
	parts := strings.Split(strings.TrimSpace(code), ":")
	if len(parts) == 0 {
		return "", ""
	}
	moduleKey := parts[0]
	resourceKey := ""
	if len(parts) > 1 {
		resourceKey = strings.Join(parts[1:len(parts)-1], ":")
		if resourceKey == "" {
			resourceKey = parts[1]
		}
	}
	return moduleKey, resourceKey
}

func filterHiddenMenuCodes(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, hidden := hiddenMenuCodeSet[code]; hidden {
			continue
		}
		out = append(out, code)
	}
	return out
}

func filterHiddenPermissionCodes(codes []string) []string {
	out := make([]string, 0, len(codes))
	for _, code := range codes {
		if _, hidden := hiddenPermissionCodeSet[code]; hidden {
			continue
		}
		out = append(out, code)
	}
	return out
}

func pushConfigToMap(item entity.PushConfig) map[string]any {
	return map[string]any{
		"id":                     item.ID,
		"configName":             item.ConfigName,
		"providerType":           item.ProviderType,
		"webhook":                nullableString(item.Webhook),
		"appId":                  nullableString(item.AppID),
		"templateId":             nullableString(item.TemplateID),
		"receiverOpenIds":        decodeJSONStringSlice(item.ReceiverOpenIDsJSON),
		"factoryIds":             decodeJSONUintSlice(item.FactoryIDsJSON),
		"zoneIds":                decodeJSONUintSlice(item.ZoneIDsJSON),
		"alarmTypes":             decodeJSONStringSlice(item.AlarmTypesJSON),
		"alarmLevels":            decodeJSONStringSlice(item.AlarmLevelsJSON),
		"activeTimeRanges":       decodeJSONAny(item.ActiveTimeRangesJSON),
		"enabled":                item.Enabled,
		"rateLimitWindowSeconds": item.RateLimitWindowSeconds,
		"rateLimitMaxCount":      item.RateLimitMaxCount,
		"retryMaxCount":          item.RetryMaxCount,
		"retryIntervalSeconds":   item.RetryIntervalSeconds,
		"remark":                 nullableString(item.Remark),
		"secretConfigured":       item.SecretEncrypted != "",
		"appSecretConfigured":    item.AppSecretEncrypted != "",
		"createdAt":              item.CreatedAt.Format(time.RFC3339),
		"updatedAt":              item.UpdatedAt.Format(time.RFC3339),
	}
}

func pushLogToMap(item entity.AlarmPushLog) map[string]any {
	return map[string]any{
		"id":           item.ID,
		"alarmId":      item.AlarmID,
		"alarmNo":      nullableString(item.AlarmNo),
		"pushConfigId": item.PushConfigID,
		"configName":   nullableString(item.ConfigName),
		"channel":      item.Channel,
		"providerType": item.ProviderType,
		"status":       item.Status,
		"alarmType":    nullableString(item.AlarmType),
		"alarmLevel":   nullableString(item.AlarmLevel),
		"factoryId":    item.FactoryID,
		"factoryName":  nil,
		"zoneId":       item.ZoneID,
		"zoneName":     nil,
		"triggeredBy":  item.TriggeredBy,
		"retryCount":   item.RetryCount,
		"message":      item.Message,
		"requestBody":  nullableString(item.RequestBody),
		"responseBody": nullableString(item.ResponseBody),
		"errorMessage": nullableString(item.ErrorMessage),
		"pushedAt":     item.PushedAt.Format(time.RFC3339),
	}
}

func bindingRuleMap(item entity.SmartBindingRule) map[string]any {
	return map[string]any{
		"id":                    item.ID,
		"bindingId":             item.BindingID,
		"ruleName":              item.RuleName,
		"enabled":               item.Enabled,
		"alarmEnabled":          item.AlarmEnabled,
		"alarmLevel":            item.AlarmLevel,
		"dedupWindowSeconds":    item.DedupWindowSeconds,
		"cooldownSeconds":       item.CooldownSeconds,
		"minConfidence":         item.MinConfidence,
		"activeTimePlan":        decodeJSONAny(item.ActiveTimePlanJSON),
		"snapshotEnabled":       item.SnapshotEnabled,
		"recordClipEnabled":     item.RecordClipEnabled,
		"recordPreSeconds":      item.RecordPreSeconds,
		"recordPostSeconds":     item.RecordPostSeconds,
		"pushEnabled":           item.PushEnabled,
		"pushChannels":          decodeJSONStringSlice(item.PushChannelsJSON),
		"sendToAi":              item.SendToAI,
		"aiFlowCode":            nullableString(item.AIFlowCode),
		"generateAlarmDirectly": item.GenerateAlarmDirectly,
		"remark":                nullableString(item.Remark),
		"createdAt":             item.CreatedAt.Format(time.RFC3339),
		"updatedAt":             item.UpdatedAt.Format(time.RFC3339),
	}
}

func smartEventMap(item entity.SmartEvent) map[string]any {
	return map[string]any{
		"id":                item.ID,
		"eventCode":         item.EventCode,
		"rawEventId":        item.RawEventID,
		"providerCode":      "",
		"providerName":      "",
		"capabilityCode":    "",
		"capabilityName":    "",
		"eventType":         item.EventType,
		"eventLevel":        item.EventLevel,
		"sourceStage":       item.SourceStage,
		"eventTime":         item.EventTime.Format(time.RFC3339),
		"bindingId":         item.BindingID,
		"cameraId":          item.CameraID,
		"recorderId":        item.RecorderID,
		"channelId":         item.ChannelID,
		"sourceName":        nil,
		"factoryId":         item.FactoryID,
		"zoneId":            item.ZoneID,
		"imageUrl":          nullableString(item.ImageURL),
		"videoUrl":          nullableString(item.VideoURL),
		"confidence":        item.Confidence,
		"status":            item.Status,
		"dedupKey":          item.DedupKey,
		"rawJson":           item.NormalizedPayloadJSON,
		"normalizedPayload": decodeJSONAny(item.NormalizedPayloadJSON),
		"createdAt":         item.CreatedAt.Format(time.RFC3339),
		"linkedAlarm":       nil,
	}
}

func smartBridgeReconnectLogMap(item entity.SmartBridgeReconnectLog) map[string]any {
	return map[string]any{
		"id":            item.ID,
		"taskKey":       item.TaskKey,
		"cycleKey":      item.CycleKey,
		"triggerReason": item.TriggerReason,
		"action":        item.Action,
		"status":        item.Status,
		"deviceType":    item.DeviceType,
		"deviceId":      item.DeviceID,
		"sessionKey":    item.SessionKey,
		"bindingIds":    decodeJSONUintSlice(item.BindingIDsJSON),
		"attempt":       item.Attempt,
		"maxAttempts":   item.MaxAttempts,
		"nextRunAt":     timePtrToRFC3339(item.NextRunAt),
		"detail":        nullableString(item.Detail),
		"lastError":     nullableString(item.LastError),
		"createdAt":     item.CreatedAt.Format(time.RFC3339),
	}
}

func aiTaskMap(task entity.AiReviewTask, latest *entity.AiReviewResult) map[string]any {
	var latestResult any
	if latest != nil {
		latestResult = aiResultMap(*latest)
	}
	return map[string]any{
		"id":             task.ID,
		"taskNo":         task.TaskNo,
		"smartEventId":   task.SmartEventID,
		"aiFlowCode":     task.AIFlowCode,
		"modelCode":      nullableString(task.ModelCode),
		"requestPayload": decodeJSONAny(task.RequestPayloadJSON),
		"status":         task.Status,
		"retryCount":     task.RetryCount,
		"maxRetryCount":  task.MaxRetryCount,
		"submittedAt":    task.SubmittedAt.Format(time.RFC3339),
		"finishedAt":     timePtrToRFC3339(task.FinishedAt),
		"errorMessage":   nullableString(task.ErrorMessage),
		"createdAt":      task.CreatedAt.Format(time.RFC3339),
		"latestResult":   latestResult,
	}
}

func aiResultMap(result entity.AiReviewResult) map[string]any {
	return map[string]any{
		"id":            result.ID,
		"taskId":        result.TaskID,
		"decision":      result.Decision,
		"labels":        decodeJSONStringSlice(result.LabelsJSON),
		"confidence":    result.Confidence,
		"reason":        nullableString(result.Reason),
		"evidence":      decodeJSONAny(result.EvidenceJSON),
		"resultPayload": decodeJSONAny(result.ResultPayloadJSON),
		"createdAt":     result.CreatedAt.Format(time.RFC3339),
	}
}

func alarmRecordToMap(item map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range item {
		out[k] = v
	}
	return out
}

func cameraDTOToMap(item dto.CameraRecord) map[string]any {
	return map[string]any{
		"id":                 item.ID,
		"deviceCode":         item.DeviceCode,
		"name":               item.Name,
		"ip":                 item.IP,
		"sdkPort":            item.SDKPort,
		"httpPort":           item.HTTPPort,
		"rtspPort":           item.RTSPPort,
		"username":           item.Username,
		"factoryId":          item.FactoryID,
		"factoryName":        item.FactoryName,
		"zoneId":             item.ZoneID,
		"zoneName":           item.ZoneName,
		"installLocation":    item.InstallLocation,
		"supportAi":          item.SupportAI,
		"status":             item.Status,
		"lastOnlineAt":       item.LastOnlineAt,
		"remark":             item.Remark,
		"passwordConfigured": item.PasswordConfigured,
	}
}

func zoneDTOToMap(item dto.ZoneRecord) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"factoryId":   item.FactoryID,
		"factoryName": item.FactoryName,
		"zoneCode":    item.ZoneCode,
		"zoneName":    item.ZoneName,
		"status":      item.Status,
		"remark":      item.Remark,
	}
}

func deptDTOToMap(item dto.DeptRecord) map[string]any {
	return map[string]any{
		"id":          item.ID,
		"deptCode":    item.DeptCode,
		"deptName":    item.DeptName,
		"parentId":    item.ParentID,
		"parentName":  item.ParentName,
		"factoryId":   item.FactoryID,
		"factoryName": item.FactoryName,
		"zoneId":      item.ZoneID,
		"zoneName":    item.ZoneName,
		"leader":      item.Leader,
		"phone":       item.Phone,
		"sort":        item.Sort,
		"status":      item.Status,
		"remark":      item.Remark,
	}
}

func dictTypeDTOToMap(item dto.DictTypeRecord) map[string]any {
	dictItems := make([]map[string]any, 0, len(item.Items))
	for _, child := range item.Items {
		dictItems = append(dictItems, dictItemDTOToMap(child))
	}
	return map[string]any{
		"id":       item.ID,
		"dictCode": item.DictCode,
		"dictName": item.DictName,
		"status":   item.Status,
		"remark":   item.Remark,
		"items":    dictItems,
	}
}

func dictItemDTOToMap(item dto.DictItemRecord) map[string]any {
	return map[string]any{
		"id":         item.ID,
		"dictTypeId": item.DictTypeID,
		"itemLabel":  item.ItemLabel,
		"itemValue":  item.ItemValue,
		"itemSort":   item.ItemSort,
		"isDefault":  item.IsDefault,
		"status":     item.Status,
		"remark":     item.Remark,
	}
}

func recorderDTOToMap(item dto.RecorderRecord) map[string]any {
	return map[string]any{
		"id":                 item.ID,
		"deviceCode":         item.DeviceCode,
		"name":               item.Name,
		"ip":                 item.IP,
		"sdkPort":            item.SDKPort,
		"httpPort":           item.HTTPPort,
		"username":           item.Username,
		"channelCount":       item.ChannelCount,
		"factoryId":          item.FactoryID,
		"factoryName":        item.FactoryName,
		"status":             item.Status,
		"lastOnlineAt":       item.LastOnlineAt,
		"passwordConfigured": item.PasswordConfigured,
	}
}

func channelDTOToMap(item dto.RecorderChannelRecord) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"recorderId":      item.RecorderID,
		"recorderName":    item.RecorderName,
		"channelNo":       item.ChannelNo,
		"name":            item.Name,
		"cameraId":        item.CameraID,
		"cameraName":      item.CameraName,
		"factoryId":       item.FactoryID,
		"factoryName":     item.FactoryName,
		"zoneId":          item.ZoneID,
		"zoneName":        item.ZoneName,
		"enabled":         item.Enabled,
		"supportPlayback": item.SupportPlayback,
		"status":          item.Status,
	}
}

func dtoAlarmToMap(item dto.AlarmRecord) map[string]any {
	return map[string]any{
		"id":              item.ID,
		"alarmNo":         item.AlarmNo,
		"aiEventId":       item.AIEventID,
		"alarmType":       item.AlarmType,
		"alarmLevel":      item.AlarmLevel,
		"alarmTime":       item.AlarmTime,
		"status":          item.Status,
		"cameraId":        item.CameraID,
		"cameraName":      item.CameraName,
		"recorderId":      item.RecorderID,
		"recorderName":    item.RecorderName,
		"channelId":       item.ChannelID,
		"channelName":     item.ChannelName,
		"factoryId":       item.FactoryID,
		"factoryName":     item.FactoryName,
		"zoneId":          item.ZoneID,
		"zoneName":        item.ZoneName,
		"message":         item.Message,
		"imageUrl":        item.ImageURL,
		"videoUrl":        item.VideoURL,
		"recordStartTime": item.RecordStartTime,
		"recordEndTime":   item.RecordEndTime,
		"occurrenceCount": item.OccurrenceCount,
		"lastEventTime":   item.LastEventTime,
		"createdAt":       item.CreatedAt,
	}
}

func userRowDeptName(value string) string { return value }

func encodeJSON(value any) string {
	if value == nil {
		return ""
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeJSONStringSlice(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return []string{}
	}
	return values
}

func decodeJSONUintSlice(raw string) []uint {
	if strings.TrimSpace(raw) == "" {
		return []uint{}
	}
	var values []uint
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	var ints []int
	if err := json.Unmarshal([]byte(raw), &ints); err == nil {
		result := make([]uint, 0, len(ints))
		for _, item := range ints {
			result = append(result, uint(item))
		}
		return result
	}
	return []uint{}
}

func decodeJSONAny(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return raw
	}
	return value
}

func normalizePushConfigPayload(payload *PushConfigPayload) error {
	if payload == nil {
		return fmt.Errorf("推送配置参数无效")
	}
	payload.ConfigName = strings.TrimSpace(payload.ConfigName)
	payload.ProviderType = strings.ToLower(strings.TrimSpace(payload.ProviderType))
	if payload.ConfigName == "" {
		return fmt.Errorf("配置名称不能为空")
	}
	switch payload.ProviderType {
	case "dingtalk":
		payload.AppID = nil
		payload.AppSecret = nil
		payload.TemplateID = nil
	case "wechat":
		payload.Webhook = nil
		payload.Secret = nil
	case "email":
		payload.Webhook = nil
		payload.Secret = nil
		payload.AppID = nil
		payload.AppSecret = nil
		payload.TemplateID = nil
	default:
		return fmt.Errorf("不支持的推送渠道")
	}
	return nil
}

func normalizePushConfigSelectors(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		id, ok := parsePushConfigSelector(value)
		if !ok {
			continue
		}
		normalized := fmt.Sprintf("push-config:%d", id)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func normalizedStatus(value, fallback string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func buildSnapshotDataURL(cameraID, channelID *uint) string {
	captureTime := time.Now().Format("2006-01-02 15:04:05")
	cameraText := "-"
	if cameraID != nil {
		cameraText = fmt.Sprintf("%d", *cameraID)
	}
	channelText := "-"
	if channelID != nil {
		channelText = fmt.Sprintf("%d", *channelID)
	}
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="720" viewBox="0 0 1280 720">
<defs>
  <linearGradient id="bg" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
    <stop offset="0%%" stop-color="#0f172a"/>
    <stop offset="100%%" stop-color="#1d4ed8"/>
  </linearGradient>
</defs>
<rect width="1280" height="720" fill="url(#bg)"/>
<rect x="36" y="36" width="1208" height="648" rx="24" fill="rgba(15,23,42,0.42)" stroke="rgba(255,255,255,0.18)"/>
<text x="72" y="118" fill="#e2e8f0" font-size="42" font-family="Microsoft YaHei, Arial, sans-serif">安防监控抓拍</text>
<text x="72" y="182" fill="#bfdbfe" font-size="26" font-family="Consolas, Arial, sans-serif">Capture Time: %s</text>
<text x="72" y="234" fill="#bfdbfe" font-size="26" font-family="Consolas, Arial, sans-serif">Camera ID: %s</text>
<text x="72" y="286" fill="#bfdbfe" font-size="26" font-family="Consolas, Arial, sans-serif">Channel ID: %s</text>
<text x="72" y="562" fill="#f8fafc" font-size="72" font-family="Microsoft YaHei, Arial, sans-serif">MOTION DETECTED</text>
<text x="72" y="620" fill="#cbd5e1" font-size="28" font-family="Microsoft YaHei, Arial, sans-serif">当前环境未接入设备抓图文件落盘，先返回可预览的实时抓拍占位图。</text>
<circle cx="1120" cy="118" r="18" fill="#ef4444"/>
</svg>`,
		captureTime,
		cameraText,
		channelText,
	)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
		return fallback
	}
	return value
}

func (s *PlatformService) ensureGeneratedCode(tableName, columnName, currentValue, prefix string) (string, error) {
	if currentValue != "" {
		return currentValue, nil
	}
	for attempt := 0; attempt < 5; attempt++ {
		candidate := fmt.Sprintf("%s-%d", prefix, time.Now().UnixMilli())
		if attempt > 0 {
			candidate = fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), attempt)
		}
		var count int64
		if err := s.db().Table(tableName).Where(columnName+" = ?", candidate).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return candidate, nil
		}
		time.Sleep(time.Millisecond)
	}
	return "", fmt.Errorf("generate %s code failed", prefix)
}

func percent(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

func wrapDeviceDeleteError(err error) error {
	if isMySQLForeignKeyError(err) {
		return fmt.Errorf("%w: %v", ErrDeviceDeleteForbidden, err)
	}
	return err
}

func isMySQLForeignKeyError(err error) bool {
	var mysqlErr *driverMysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return false
	}
	return mysqlErr.Number == 1451 || mysqlErr.Number == 1452
}

func applyOptionalTimeRange(db *gorm.DB, column string, startAt, endAt *time.Time) *gorm.DB {
	if startAt != nil {
		db = db.Where(column+" >= ?", *startAt)
	}
	if endAt != nil {
		db = db.Where(column+" <= ?", *endAt)
	}
	return db
}

func normalizeDashboardRange(startAt, endAt *time.Time, defaultDays int) (time.Time, time.Time) {
	rangeEnd := time.Now()
	if endAt != nil {
		rangeEnd = *endAt
	}
	rangeStart := rangeEnd.AddDate(0, 0, -(defaultDays - 1))
	if startAt != nil {
		rangeStart = *startAt
	}
	if rangeStart.After(rangeEnd) {
		rangeStart = rangeEnd
	}
	return rangeStart, rangeEnd
}

func truncateToDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func toUint(value any) uint {
	switch typed := value.(type) {
	case uint:
		return typed
	case int:
		return uint(typed)
	case int64:
		return uint(typed)
	case float64:
		return uint(typed)
	default:
		return 0
	}
}

func ageSeconds(value time.Time) int {
	if value.IsZero() {
		return 0
	}
	seconds := int(time.Since(value).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func maxInt(current, fallback int) int {
	if current <= 0 {
		return fallback
	}
	return current
}

func chooseID(enabled bool, id uint) any {
	if enabled {
		return id
	}
	return nil
}

func mapStreamProfileToInt(streamProfile string) int {
	if strings.EqualFold(streamProfile, "sub") {
		return 2
	}
	return 1
}

func resolveDeviceProtocol(httpPort int) string {
	if httpPort == 443 {
		return "https"
	}
	return "http"
}

func (s *PlatformService) deviceSecretKey() string {
	if strings.TrimSpace(s.cfg.DeviceSecretKey) != "" {
		return s.cfg.DeviceSecretKey
	}
	return s.cfg.JWTSecretKey
}

func timePtrToRFC3339(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}
