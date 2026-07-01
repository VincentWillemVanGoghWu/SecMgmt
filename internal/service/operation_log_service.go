package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultOperationLogRetentionDays = 360
	operationLogRetentionSettingKey  = "operation_log_retention_days"
)

type OperationLogService struct {
	repo   *repository.Repository
	logger *zap.Logger
}

type OperationLogActor struct {
	UserID    *uint
	Username  string
	RealName  string
	RoleCodes []string
	RoleNames []string
}

type OperationLogCreateInput struct {
	TraceID          string
	Source           string
	OperatorID       *uint
	OperatorUsername string
	OperatorRealName string
	RoleCodes        []string
	RoleNames        []string
	ClientIP         string
	IPLocation       string
	UserAgent        string
	OSName           string
	MenuCode         string
	MenuName         string
	RoutePath        string
	PageTitle        string
	PageComponent    string
	ActionCode       string
	ActionName       string
	OperationType    string
	ObjectType       string
	ObjectID         string
	ObjectName       string
	ObjectLocation   string
	RequestMethod    string
	RequestPath      string
	RequestQuery     string
	RequestParams    string
	DevicePointInfo  string
	BeforeSnapshot   string
	AfterSnapshot    string
	ErrorStack       string
	ResultStatus     string
	ResponseStatus   int
	DurationMs       int64
	ExtraJSON        string
	OperationTime    time.Time
}

type OperationLogTrackPayload struct {
	Source          string `json:"source"`
	MenuCode        string `json:"menuCode"`
	MenuName        string `json:"menuName"`
	RoutePath       string `json:"routePath"`
	PageTitle       string `json:"pageTitle"`
	PageComponent   string `json:"pageComponent"`
	ActionCode      string `json:"actionCode"`
	ActionName      string `json:"actionName"`
	OperationType   string `json:"operationType"`
	ObjectType      string `json:"objectType"`
	ObjectID        string `json:"objectId"`
	ObjectName      string `json:"objectName"`
	ObjectLocation  string `json:"objectLocation"`
	RequestParams   string `json:"requestParams"`
	DevicePointInfo string `json:"devicePointInfo"`
	BeforeSnapshot  string `json:"beforeSnapshot"`
	AfterSnapshot   string `json:"afterSnapshot"`
	ErrorStack      string `json:"errorStack"`
	ResultStatus    string `json:"resultStatus"`
	DurationMs      int64  `json:"durationMs"`
	ExtraJSON       string `json:"extraJson"`
}

type OperationLogListFilter struct {
	Username      string
	OperationType string
	ResultStatus  string
	MenuCode      string
	Keyword       string
	StartAt       *time.Time
	EndAt         *time.Time
}

type operationStatRow struct {
	Name  string `gorm:"column:name"`
	Value int64  `gorm:"column:value"`
}

func NewOperationLogService(repo *repository.Repository, logger *zap.Logger) *OperationLogService {
	return &OperationLogService{
		repo:   repo,
		logger: logger,
	}
}

func (s *OperationLogService) StartCleanupJob() func() {
	stop := make(chan struct{})
	go func() {
		s.runCleanupSafely()
		ticker := time.NewTicker(12 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.runCleanupSafely()
			case <-stop:
				return
			}
		}
	}()
	return func() {
		close(stop)
	}
}

func (s *OperationLogService) runCleanupSafely() {
	if err := s.CleanupExpiredLogs(); err != nil && s.logger != nil {
		s.logger.Warn("cleanup expired operation logs", zap.Error(err))
	}
}

func (s *OperationLogService) CleanupExpiredLogs() error {
	retentionDays := s.resolveRetentionDays()
	expireBefore := time.Now().AddDate(0, 0, -retentionDays)
	return s.repo.DB().
		Where("operation_time < ?", expireBefore).
		Delete(&entity.OperationLog{}).Error
}

func (s *OperationLogService) EnsureBootstrapData() error {
	db := s.repo.DB()
	ensureMenu := func(menu *entity.Menu) error {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "parent_id", "route_name", "route_path", "icon", "menu_type", "sort", "status"}),
		}).Create(menu).Error; err != nil {
			return err
		}
		return db.Where("code = ?", menu.Code).First(menu).Error
	}
	ensurePermission := func(permission *entity.Permission) error {
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "status", "is_button"}),
		}).Create(permission).Error; err != nil {
			return err
		}
		return db.Where("code = ?", permission.Code).First(permission).Error
	}
	ensureRoleMenuBinding := func(roleCode, menuCode string) error {
		var role entity.Role
		if err := db.Where("role_code = ?", roleCode).First(&role).Error; err != nil {
			return nil
		}
		var menu entity.Menu
		if err := db.Where("code = ?", menuCode).First(&menu).Error; err != nil {
			return err
		}
		return db.Exec(
			"INSERT IGNORE INTO sys_role_menu (role_id, menu_id) VALUES (?, ?)",
			role.ID,
			menu.ID,
		).Error
	}
	ensureRolePermissionBinding := func(roleCode, permissionCode string) error {
		var role entity.Role
		if err := db.Where("role_code = ?", roleCode).First(&role).Error; err != nil {
			return nil
		}
		var permission entity.Permission
		if err := db.Where("code = ?", permissionCode).First(&permission).Error; err != nil {
			return err
		}
		return db.Exec(
			"INSERT IGNORE INTO sys_role_permission (role_id, permission_id) VALUES (?, ?)",
			role.ID,
			permission.ID,
		).Error
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "setting_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"setting_name", "setting_value", "remark", "updated_at"}),
	}).Create(&entity.SystemSetting{
		SettingKey:   operationLogRetentionSettingKey,
		SettingName:  "操作日志留存天数",
		SettingValue: fmt.Sprint(defaultOperationLogRetentionDays),
		Remark:       "默认360天，可直接在数据库中维护该配置值",
	}).Error; err != nil {
		return err
	}

	safetyMenu := entity.Menu{
		Name:     "安全日志",
		Code:     "safety",
		Icon:     "Warning",
		MenuType: "catalog",
		Sort:     2,
		Status:   "enabled",
	}
	if err := ensureMenu(&safetyMenu); err != nil {
		return err
	}
	pushMenu := entity.Menu{
		Name:     "推送管理",
		Code:     "push",
		Icon:     "Bell",
		MenuType: "catalog",
		Sort:     6,
		Status:   "enabled",
	}
	if err := ensureMenu(&pushMenu); err != nil {
		return err
	}
	deviceMenu := entity.Menu{
		Name:     "设备管理",
		Code:     "device",
		Icon:     "Grid",
		MenuType: "catalog",
		Sort:     5,
		Status:   "enabled",
	}
	if err := ensureMenu(&deviceMenu); err != nil {
		return err
	}

	menu := entity.Menu{
		Name:      "操作日志",
		Code:      "safety-operation-logs",
		ParentID:  &safetyMenu.ID,
		RouteName: "safety-operation-logs",
		RoutePath: "/safety/operation-logs",
		Icon:      "Document",
		MenuType:  "menu",
		Sort:      5,
		Status:    "enabled",
	}
	if err := ensureMenu(&menu); err != nil {
		return err
	}
	pushLogMenu := entity.Menu{
		Name:      "推送日志",
		Code:      "push-logs",
		ParentID:  &pushMenu.ID,
		RouteName: "push-logs",
		RoutePath: "/push/logs",
		Icon:      "Files",
		MenuType:  "menu",
		Sort:      2,
		Status:    "enabled",
	}
	if err := ensureMenu(&pushLogMenu); err != nil {
		return err
	}

	deviceCheckMenu := entity.Menu{
		Name:      "巡检计划",
		Code:      "device-check-schedules",
		ParentID:  &deviceMenu.ID,
		RouteName: "device-check-schedules",
		RoutePath: "/device/check-schedules",
		Icon:      "Timer",
		MenuType:  "menu",
		Sort:      5,
		Status:    "enabled",
	}
	if err := ensureMenu(&deviceCheckMenu); err != nil {
		return err
	}

	permissions := []entity.Permission{
		{Name: "查看操作日志", Code: "log:operation:view", Status: "enabled", IsButton: true},
		{Name: "导出操作日志", Code: "log:operation:export", Status: "enabled", IsButton: true},
		{Name: "查看推送日志", Code: "push:log:view", Status: "enabled", IsButton: true},
		{Name: "重试推送日志", Code: "push:log:retry", Status: "enabled", IsButton: true},
		{Name: "查看巡检计划", Code: "device:check-plan:view", Status: "enabled", IsButton: true},
		{Name: "新增巡检计划", Code: "device:check-plan:create", Status: "enabled", IsButton: true},
		{Name: "编辑巡检计划", Code: "device:check-plan:update", Status: "enabled", IsButton: true},
		{Name: "删除巡检计划", Code: "device:check-plan:delete", Status: "enabled", IsButton: true},
		{Name: "执行巡检计划", Code: "device:check-plan:run", Status: "enabled", IsButton: true},
	}
	for _, permission := range permissions {
		if err := ensurePermission(&permission); err != nil {
			return err
		}
	}

	for _, roleCode := range []string{"admin", "User"} {
		for _, menuCode := range []string{"safety-operation-logs", "push", "push-logs", "device", "device-check-schedules"} {
			if err := ensureRoleMenuBinding(roleCode, menuCode); err != nil {
				return err
			}
		}
		permissionCodes := []string{"log:operation:view", "log:operation:export", "push:log:view", "push:log:retry"}
		if roleCode == "admin" {
			permissionCodes = append(permissionCodes,
				"device:check-plan:view",
				"device:check-plan:create",
				"device:check-plan:update",
				"device:check-plan:delete",
				"device:check-plan:run",
			)
		} else {
			permissionCodes = append(permissionCodes, "device:check-plan:view", "device:check-plan:run")
		}
		for _, permissionCode := range permissionCodes {
			if err := ensureRolePermissionBinding(roleCode, permissionCode); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *OperationLogService) Record(input OperationLogCreateInput) error {
	operationTime := input.OperationTime
	if operationTime.IsZero() {
		operationTime = time.Now()
	}
	resultStatus := strings.TrimSpace(input.ResultStatus)
	if resultStatus == "" {
		if input.ResponseStatus >= 400 {
			resultStatus = "failed"
		} else {
			resultStatus = "success"
		}
	}
	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = "api"
	}
	entry := entity.OperationLog{
		TraceID:          strings.TrimSpace(input.TraceID),
		Source:           source,
		OperatorID:       input.OperatorID,
		OperatorUsername: strings.TrimSpace(input.OperatorUsername),
		OperatorRealName: strings.TrimSpace(input.OperatorRealName),
		RoleCodes:        strings.Join(compactStrings(input.RoleCodes), ","),
		RoleNames:        strings.Join(compactStrings(input.RoleNames), ","),
		ClientIP:         strings.TrimSpace(input.ClientIP),
		IPLocation:       strings.TrimSpace(input.IPLocation),
		UserAgent:        trimText(input.UserAgent),
		OSName:           strings.TrimSpace(input.OSName),
		MenuCode:         strings.TrimSpace(input.MenuCode),
		MenuName:         strings.TrimSpace(input.MenuName),
		RoutePath:        strings.TrimSpace(input.RoutePath),
		PageTitle:        strings.TrimSpace(input.PageTitle),
		PageComponent:    strings.TrimSpace(input.PageComponent),
		ActionCode:       strings.TrimSpace(input.ActionCode),
		ActionName:       strings.TrimSpace(input.ActionName),
		OperationType:    strings.TrimSpace(input.OperationType),
		ObjectType:       strings.TrimSpace(input.ObjectType),
		ObjectID:         strings.TrimSpace(input.ObjectID),
		ObjectName:       strings.TrimSpace(input.ObjectName),
		ObjectLocation:   strings.TrimSpace(input.ObjectLocation),
		RequestMethod:    strings.TrimSpace(input.RequestMethod),
		RequestPath:      strings.TrimSpace(input.RequestPath),
		RequestQuery:     trimText(input.RequestQuery),
		RequestParams:    trimText(input.RequestParams),
		DevicePointInfo:  trimText(input.DevicePointInfo),
		BeforeSnapshot:   trimText(input.BeforeSnapshot),
		AfterSnapshot:    trimText(input.AfterSnapshot),
		ErrorStack:       trimText(input.ErrorStack),
		ResultStatus:     resultStatus,
		ResponseStatus:   input.ResponseStatus,
		DurationMs:       input.DurationMs,
		StoragePartition: operationTime.Format("20060102"),
		RetentionDays:    s.resolveRetentionDays(),
		ExtraJSON:        trimText(input.ExtraJSON),
		OperationTime:    operationTime,
		CreatedAt:        time.Now(),
	}
	return s.repo.DB().Create(&entry).Error
}

func (s *OperationLogService) RecordTrack(actor OperationLogActor, meta OperationLogCreateInput, payload OperationLogTrackPayload) error {
	return s.Record(OperationLogCreateInput{
		TraceID:          meta.TraceID,
		Source:           opLogFirstNonEmpty(payload.Source, meta.Source, "ui"),
		OperatorID:       actor.UserID,
		OperatorUsername: actor.Username,
		OperatorRealName: actor.RealName,
		RoleCodes:        actor.RoleCodes,
		RoleNames:        actor.RoleNames,
		ClientIP:         meta.ClientIP,
		IPLocation:       meta.IPLocation,
		UserAgent:        meta.UserAgent,
		OSName:           meta.OSName,
		MenuCode:         opLogFirstNonEmpty(payload.MenuCode, meta.MenuCode),
		MenuName:         opLogFirstNonEmpty(payload.MenuName, meta.MenuName),
		RoutePath:        opLogFirstNonEmpty(payload.RoutePath, meta.RoutePath),
		PageTitle:        opLogFirstNonEmpty(payload.PageTitle, meta.PageTitle),
		PageComponent:    opLogFirstNonEmpty(payload.PageComponent, meta.PageComponent),
		ActionCode:       opLogFirstNonEmpty(payload.ActionCode, meta.ActionCode),
		ActionName:       opLogFirstNonEmpty(payload.ActionName, meta.ActionName),
		OperationType:    opLogFirstNonEmpty(payload.OperationType, meta.OperationType, "按钮点击"),
		ObjectType:       opLogFirstNonEmpty(payload.ObjectType, meta.ObjectType),
		ObjectID:         opLogFirstNonEmpty(payload.ObjectID, meta.ObjectID),
		ObjectName:       opLogFirstNonEmpty(payload.ObjectName, meta.ObjectName),
		ObjectLocation:   opLogFirstNonEmpty(payload.ObjectLocation, meta.ObjectLocation),
		RequestMethod:    meta.RequestMethod,
		RequestPath:      meta.RequestPath,
		RequestQuery:     meta.RequestQuery,
		RequestParams:    opLogFirstNonEmpty(payload.RequestParams, meta.RequestParams),
		DevicePointInfo:  payload.DevicePointInfo,
		BeforeSnapshot:   payload.BeforeSnapshot,
		AfterSnapshot:    payload.AfterSnapshot,
		ErrorStack:       opLogFirstNonEmpty(payload.ErrorStack, meta.ErrorStack),
		ResultStatus:     opLogFirstNonEmpty(payload.ResultStatus, meta.ResultStatus),
		ResponseStatus:   meta.ResponseStatus,
		DurationMs:       firstNonZero(payload.DurationMs, meta.DurationMs),
		ExtraJSON:        opLogFirstNonEmpty(payload.ExtraJSON, meta.ExtraJSON),
		OperationTime:    time.Now(),
	})
}

func (s *OperationLogService) List(page, pageSize int, filter OperationLogListFilter) (map[string]any, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	base := s.applyListFilter(s.repo.DB().Model(&entity.OperationLog{}), filter)
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []entity.OperationLog
	if err := base.Order("operation_time DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, s.listItemMap(item))
	}
	return map[string]any{
		"items":    result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	}, nil
}

func (s *OperationLogService) GetDetail(id uint) (map[string]any, error) {
	var item entity.OperationLog
	if err := s.repo.DB().First(&item, id).Error; err != nil {
		return nil, err
	}
	return s.detailMap(item), nil
}

func (s *OperationLogService) Export(filter OperationLogListFilter) ([]byte, string, error) {
	base := s.applyListFilter(s.repo.DB().Model(&entity.OperationLog{}), filter)
	var items []entity.OperationLog
	if err := base.Order("operation_time DESC, id DESC").Limit(5000).Find(&items).Error; err != nil {
		return nil, "", err
	}

	rows := [][]string{{
		"日志ID", "操作时间", "操作账号", "操作人", "所属角色", "菜单名称", "页面标题", "按钮操作",
		"操作类型", "操作对象类型", "操作对象名称", "IP", "结果", "耗时(ms)", "请求路径",
	}}
	for _, item := range items {
		rows = append(rows, []string{
			fmt.Sprint(item.ID),
			item.OperationTime.Format("2006-01-02 15:04:05.000"),
			item.OperatorUsername,
			item.OperatorRealName,
			item.RoleNames,
			item.MenuName,
			item.PageTitle,
			item.ActionName,
			item.OperationType,
			item.ObjectType,
			item.ObjectName,
			item.ClientIP,
			item.ResultStatus,
			fmt.Sprint(item.DurationMs),
			item.RequestPath,
		})
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.WriteAll(rows)
	writer.Flush()
	filename := fmt.Sprintf("operation_logs_%s.csv", time.Now().Format("20060102150405"))
	return buffer.Bytes(), filename, writer.Error()
}

func (s *OperationLogService) GetDashboardStats(startAt, endAt *time.Time) map[string]any {
	base := s.repo.DB().Model(&entity.OperationLog{})
	base = opLogApplyTimeRange(base, "operation_time", startAt, endAt)
	if startAt == nil && endAt == nil {
		startOfDay := operationLocalStartOfDay(time.Now())
		base = base.Where("operation_time >= ?", startOfDay)
	}

	var overview struct {
		TodayCount   int64 `gorm:"column:today_count"`
		SuccessCount int64 `gorm:"column:success_count"`
		FailedCount  int64 `gorm:"column:failed_count"`
	}
	_ = base.Select(`count(*) AS today_count,
		coalesce(sum(case when result_status = 'success' then 1 else 0 end), 0) AS success_count,
		coalesce(sum(case when result_status = 'failed' then 1 else 0 end), 0) AS failed_count`).
		Scan(&overview).Error

	return map[string]any{
		"todayCount":   overview.TodayCount,
		"successCount": overview.SuccessCount,
		"failedCount":  overview.FailedCount,
		"topUsers":     s.topUsers(startAt, endAt),
		"topDevices":   s.topDevices(startAt, endAt),
		"topActions":   s.topActions(startAt, endAt),
	}
}

func operationLocalStartOfDay(now time.Time) time.Time {
	localNow := now.In(time.Local)
	year, month, day := localNow.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, localNow.Location())
}

func (s *OperationLogService) topUsers(startAt, endAt *time.Time) []map[string]any {
	rows := make([]operationStatRow, 0, 5)
	db := s.repo.DB().Model(&entity.OperationLog{}).
		Select("CASE WHEN operator_real_name <> '' THEN CONCAT(operator_real_name, '(', operator_username, ')') ELSE operator_username END AS name, COUNT(*) AS value").
		Where("operator_username <> ''")
	db = opLogApplyTimeRange(db, "operation_time", startAt, endAt)
	if startAt == nil && endAt == nil {
		db = db.Where("operation_time >= ?", time.Now().Truncate(24*time.Hour))
	}
	_ = db.Group("operator_username, operator_real_name").Order("value DESC").Limit(5).Scan(&rows).Error
	return statRowsToMaps(rows)
}

func (s *OperationLogService) topDevices(startAt, endAt *time.Time) []map[string]any {
	rows := make([]operationStatRow, 0, 5)
	db := s.repo.DB().Model(&entity.OperationLog{}).
		Select("object_name AS name, COUNT(*) AS value").
		Where("object_name <> ''").
		Where("object_type IN ?", []string{"摄像头设备", "监控通道", "录像文件", "设备", "camera", "channel", "recorder"})
	db = opLogApplyTimeRange(db, "operation_time", startAt, endAt)
	if startAt == nil && endAt == nil {
		db = db.Where("operation_time >= ?", time.Now().Truncate(24*time.Hour))
	}
	_ = db.Group("object_name").Order("value DESC").Limit(5).Scan(&rows).Error
	return statRowsToMaps(rows)
}

func (s *OperationLogService) topActions(startAt, endAt *time.Time) []map[string]any {
	rows := make([]operationStatRow, 0, 5)
	db := s.repo.DB().Model(&entity.OperationLog{}).
		Select("CASE WHEN action_name <> '' THEN action_name ELSE operation_type END AS name, COUNT(*) AS value").
		Where("(action_name <> '' OR operation_type <> '')")
	db = opLogApplyTimeRange(db, "operation_time", startAt, endAt)
	if startAt == nil && endAt == nil {
		db = db.Where("operation_time >= ?", time.Now().Truncate(24*time.Hour))
	}
	_ = db.Group("CASE WHEN action_name <> '' THEN action_name ELSE operation_type END").Order("value DESC").Limit(5).Scan(&rows).Error
	return statRowsToMaps(rows)
}

func (s *OperationLogService) resolveRetentionDays() int {
	var setting entity.SystemSetting
	if err := s.repo.DB().Where("setting_key = ?", operationLogRetentionSettingKey).First(&setting).Error; err != nil {
		return defaultOperationLogRetentionDays
	}
	value := strings.TrimSpace(setting.SettingValue)
	if value == "" {
		return defaultOperationLogRetentionDays
	}
	var retentionDays int
	if _, err := fmt.Sscanf(value, "%d", &retentionDays); err != nil || retentionDays <= 0 {
		return defaultOperationLogRetentionDays
	}
	return retentionDays
}

func (s *OperationLogService) applyListFilter(db *gorm.DB, filter OperationLogListFilter) *gorm.DB {
	db = opLogApplyTimeRange(db, "operation_time", filter.StartAt, filter.EndAt)
	if filter.StartAt == nil && filter.EndAt == nil {
		db = db.Where("operation_time >= ?", time.Now().Truncate(24*time.Hour))
	}
	if username := strings.TrimSpace(filter.Username); username != "" {
		db = db.Where("(operator_username LIKE ? OR operator_real_name LIKE ?)", "%"+username+"%", "%"+username+"%")
	}
	if operationType := strings.TrimSpace(filter.OperationType); operationType != "" {
		db = db.Where("operation_type = ?", operationType)
	}
	if resultStatus := strings.TrimSpace(filter.ResultStatus); resultStatus != "" {
		db = db.Where("result_status = ?", resultStatus)
	}
	if menuCode := strings.TrimSpace(filter.MenuCode); menuCode != "" {
		db = db.Where("menu_code = ?", menuCode)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(`(
			menu_name LIKE ? OR page_title LIKE ? OR action_name LIKE ? OR
			object_name LIKE ? OR object_location LIKE ? OR request_path LIKE ? OR client_ip LIKE ?
		)`, like, like, like, like, like, like, like)
	}
	return db
}

func (s *OperationLogService) listItemMap(item entity.OperationLog) map[string]any {
	menuPage := strings.TrimSpace(strings.Join(compactStrings([]string{item.MenuName, item.PageTitle}), " / "))
	if menuPage == "" {
		menuPage = item.RoutePath
	}
	return map[string]any{
		"id":               item.ID,
		"operationTime":    item.OperationTime.Format(time.RFC3339Nano),
		"operatorName":     opLogFirstNonEmpty(item.OperatorRealName, item.OperatorUsername),
		"operatorUsername": item.OperatorUsername,
		"roleNames":        item.RoleNames,
		"menuPage":         menuPage,
		"menuName":         item.MenuName,
		"pageTitle":        item.PageTitle,
		"actionName":       opLogFirstNonEmpty(item.ActionName, item.OperationType),
		"objectName":       opLogFirstNonEmpty(item.ObjectName, item.ObjectLocation, "-"),
		"operationType":    item.OperationType,
		"clientIp":         item.ClientIP,
		"resultStatus":     item.ResultStatus,
		"durationMs":       item.DurationMs,
	}
}

func (s *OperationLogService) detailMap(item entity.OperationLog) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"traceId":          item.TraceID,
		"source":           item.Source,
		"operatorId":       item.OperatorID,
		"operatorUsername": item.OperatorUsername,
		"operatorRealName": item.OperatorRealName,
		"roleCodes":        splitCSV(item.RoleCodes),
		"roleNames":        splitCSV(item.RoleNames),
		"clientIp":         item.ClientIP,
		"ipLocation":       item.IPLocation,
		"userAgent":        item.UserAgent,
		"osName":           item.OSName,
		"menuCode":         item.MenuCode,
		"menuName":         item.MenuName,
		"routePath":        item.RoutePath,
		"pageTitle":        item.PageTitle,
		"pageComponent":    item.PageComponent,
		"actionCode":       item.ActionCode,
		"actionName":       item.ActionName,
		"operationType":    item.OperationType,
		"objectType":       item.ObjectType,
		"objectId":         item.ObjectID,
		"objectName":       item.ObjectName,
		"objectLocation":   item.ObjectLocation,
		"requestMethod":    item.RequestMethod,
		"requestPath":      item.RequestPath,
		"requestQuery":     item.RequestQuery,
		"requestParams":    item.RequestParams,
		"devicePointInfo":  item.DevicePointInfo,
		"beforeSnapshot":   item.BeforeSnapshot,
		"afterSnapshot":    item.AfterSnapshot,
		"errorStack":       item.ErrorStack,
		"resultStatus":     item.ResultStatus,
		"responseStatus":   item.ResponseStatus,
		"durationMs":       item.DurationMs,
		"storagePartition": item.StoragePartition,
		"retentionDays":    item.RetentionDays,
		"extraJson":        item.ExtraJSON,
		"operationTime":    item.OperationTime.Format(time.RFC3339Nano),
	}
}

func statRowsToMaps(rows []operationStatRow) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		result = append(result, map[string]any{
			"name":  item.Name,
			"value": item.Value,
		})
	}
	return result
}

func opLogApplyTimeRange(db *gorm.DB, column string, startAt, endAt *time.Time) *gorm.DB {
	if startAt != nil {
		db = db.Where(column+" >= ?", *startAt)
	}
	if endAt != nil {
		db = db.Where(column+" <= ?", *endAt)
	}
	return db
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	return compactStrings(strings.Split(value, ","))
}

func opLogFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonZero(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func trimText(value string) string {
	const limit = 64000
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit]
}
