package repository

import (
	"fmt"
	"strings"
	"time"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var hiddenMenuCodes = []string{"safety-data-items", "monitor-config"}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) FindUserByUsername(username string) (*entity.User, error) {
	var user entity.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetDeptByID(deptID uint) (*entity.SysDept, error) {
	var item entity.SysDept
	if err := r.db.First(&item, deptID).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Repository) GetUserByID(userID uint) (*entity.User, error) {
	var user entity.User
	if err := r.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) ListRolesByUserID(userID uint) ([]entity.Role, error) {
	var roles []entity.Role
	err := r.db.Table("sys_role AS r").
		Select("DISTINCT r.*").
		Joins("JOIN sys_user_role ur ON ur.role_id = r.id").
		Where("ur.user_id = ?", userID).
		Order("r.id ASC").
		Scan(&roles).Error
	return roles, err
}

func (r *Repository) ListRoleCodesByUserID(userID uint) ([]string, error) {
	var roleCodes []string
	err := r.db.Table("sys_role AS r").
		Select("r.role_code").
		Joins("JOIN sys_user_role ur ON ur.role_id = r.id").
		Where("ur.user_id = ?", userID).
		Group("r.role_code").
		Order("MIN(r.id) ASC").
		Scan(&roleCodes).Error
	return roleCodes, err
}

func (r *Repository) ListMenusByUserID(userID uint) ([]entity.Menu, error) {
	var menus []entity.Menu
	err := r.db.Table("sys_menu AS m").
		Select("DISTINCT m.*").
		Joins("JOIN sys_role_menu rm ON rm.menu_id = m.id").
		Joins("JOIN sys_user_role ur ON ur.role_id = rm.role_id").
		Where("ur.user_id = ? AND m.status = ?", userID, "enabled").
		Where("m.code NOT IN ?", hiddenMenuCodes).
		Order("m.sort ASC, m.id ASC").
		Scan(&menus).Error
	return menus, err
}

func (r *Repository) ListPermissionsByUserID(userID uint) ([]entity.Permission, error) {
	var permissions []entity.Permission
	err := r.db.Table("sys_permission AS p").
		Select("DISTINCT p.*").
		Joins("JOIN sys_role_permission rp ON rp.permission_id = p.id").
		Joins("JOIN sys_user_role ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ? AND p.status = ?", userID, "enabled").
		Order("p.id ASC").
		Scan(&permissions).Error
	return permissions, err
}

func (r *Repository) ListPermissionCodesByUserID(userID uint) ([]string, error) {
	var permissionCodes []string
	err := r.db.Table("sys_permission AS p").
		Select("p.code").
		Joins("JOIN sys_role_permission rp ON rp.permission_id = p.id").
		Joins("JOIN sys_user_role ur ON ur.role_id = rp.role_id").
		Where("ur.user_id = ? AND p.status = ?", userID, "enabled").
		Group("p.code").
		Order("MIN(p.id) ASC").
		Scan(&permissionCodes).Error
	return permissionCodes, err
}

func (r *Repository) ListFactories() ([]entity.FactoryArea, error) {
	var items []entity.FactoryArea
	err := r.db.Order("id ASC").Find(&items).Error
	return items, err
}

type ZoneRow struct {
	entity.FactoryZone
	FactoryName string `gorm:"column:factory_name"`
}

func (r *Repository) ListZones() ([]ZoneRow, error) {
	var items []ZoneRow
	err := r.db.Table("factory_zone AS z").
		Select("z.*, f.factory_name").
		Joins("LEFT JOIN factory_area f ON f.id = z.factory_id").
		Order("z.id ASC").
		Scan(&items).Error
	return items, err
}

type DeptRow struct {
	entity.SysDept
	ParentName  *string `gorm:"column:parent_name"`
	FactoryName *string `gorm:"column:factory_name"`
	ZoneName    *string `gorm:"column:zone_name"`
}

func (r *Repository) ListDepts() ([]DeptRow, error) {
	var items []DeptRow
	err := r.db.Table("sys_dept AS d").
		Select("d.*, p.dept_name AS parent_name, f.factory_name, z.zone_name").
		Joins("LEFT JOIN sys_dept p ON p.id = d.parent_id").
		Joins("LEFT JOIN factory_area f ON f.id = d.factory_id").
		Joins("LEFT JOIN factory_zone z ON z.id = d.zone_id").
		Order("d.sort ASC, d.id ASC").
		Scan(&items).Error
	return items, err
}

func (r *Repository) ListDictTypes() ([]entity.SysDictType, error) {
	var items []entity.SysDictType
	err := r.db.Order("id ASC").Find(&items).Error
	return items, err
}

func (r *Repository) ListDictItems() ([]entity.SysDictItem, error) {
	var items []entity.SysDictItem
	err := r.db.Order("dict_type_id ASC, item_sort ASC, id ASC").Find(&items).Error
	return items, err
}

type CameraRow struct {
	entity.CameraDevice
	FactoryName string `gorm:"column:factory_name"`
	ZoneName    string `gorm:"column:zone_name"`
}

func (r *Repository) ListCameras() ([]CameraRow, error) {
	var items []CameraRow
	err := r.db.Table("camera_device AS c").
		Select("c.*, f.factory_name, z.zone_name").
		Joins("LEFT JOIN factory_area f ON f.id = c.factory_id").
		Joins("LEFT JOIN factory_zone z ON z.id = c.zone_id").
		Order("c.id DESC").
		Scan(&items).Error
	return items, err
}

type RecorderRow struct {
	entity.RecorderDevice
	FactoryName string `gorm:"column:factory_name"`
}

func (r *Repository) ListRecorders() ([]RecorderRow, error) {
	var items []RecorderRow
	err := r.db.Table("recorder_device AS r").
		Select("r.*, f.factory_name").
		Joins("LEFT JOIN factory_area f ON f.id = r.factory_id").
		Order("r.id DESC").
		Scan(&items).Error
	return items, err
}

type ChannelRow struct {
	entity.RecorderChannel
	RecorderName string  `gorm:"column:recorder_name"`
	CameraName   *string `gorm:"column:camera_name"`
	FactoryName  string  `gorm:"column:factory_name"`
	ZoneName     *string `gorm:"column:zone_name"`
}

func (r *Repository) ListChannels() ([]ChannelRow, error) {
	var items []ChannelRow
	err := r.db.Table("recorder_channel AS ch").
		Select("ch.*, r.name AS recorder_name, c.name AS camera_name, f.factory_name, z.zone_name").
		Joins("LEFT JOIN recorder_device r ON r.id = ch.recorder_id").
		Joins("LEFT JOIN camera_device c ON c.id = ch.camera_id").
		Joins("LEFT JOIN factory_area f ON f.id = ch.factory_id").
		Joins("LEFT JOIN factory_zone z ON z.id = ch.zone_id").
		Order("ch.id DESC").
		Scan(&items).Error
	return items, err
}

type AlarmRow struct {
	entity.AlarmRecord
	CameraName   *string `gorm:"column:camera_name"`
	RecorderName *string `gorm:"column:recorder_name"`
	ChannelName  *string `gorm:"column:channel_name"`
	FactoryName  *string `gorm:"column:factory_name"`
	ZoneName     *string `gorm:"column:zone_name"`
}

func (r *Repository) ListAlarms(page, pageSize int, filter dto.AlarmListFilter) ([]AlarmRow, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	base := r.db.Table("alarm_record AS a").
		Joins("LEFT JOIN camera_device c ON c.id = a.camera_id").
		Joins("LEFT JOIN recorder_device r ON r.id = a.recorder_id").
		Joins("LEFT JOIN recorder_channel ch ON ch.id = a.channel_id").
		Joins("LEFT JOIN factory_area f ON f.id = a.factory_id").
		Joins("LEFT JOIN factory_zone z ON z.id = a.zone_id")
	base = applyAlarmListFilter(base, filter)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []AlarmRow
	err := base.Select("a.*, c.name AS camera_name, r.name AS recorder_name, ch.name AS channel_name, f.factory_name, z.zone_name").
		Order("a.alarm_time DESC, a.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	return rows, total, nil
}

func applyAlarmListFilter(db *gorm.DB, filter dto.AlarmListFilter) *gorm.DB {
	if filter.DenyAll {
		return db.Where("1 = 0")
	}
	db = applyTimeRange(db, "a.alarm_time", filter.StartAt, filter.EndAt)
	keywordText := strings.TrimSpace(filter.Keyword)
	if keywordText != "" {
		keyword := "%" + keywordText + "%"
		db = db.Where(
			"(a.alarm_no LIKE ? OR a.alarm_type LIKE ? OR a.message LIKE ? OR c.name LIKE ? OR r.name LIKE ? OR ch.name LIKE ? OR f.factory_name LIKE ? OR z.zone_name LIKE ?)",
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
			keyword,
		)
	}
	if filter.ExcludeDone {
		db = db.Where("a.status <> ?", "done")
	}
	if filter.Status != "" {
		db = db.Where("a.status = ?", filter.Status)
	}
	if filter.Level != "" {
		db = db.Where("a.alarm_level = ?", filter.Level)
	}
	if filter.AlarmType != "" {
		db = db.Where("a.alarm_type = ?", filter.AlarmType)
	}
	scopeClauses := make([]string, 0, 5)
	scopeArgs := make([]any, 0, 5)
	if len(filter.FactoryIDs) > 0 {
		scopeClauses = append(scopeClauses, "a.factory_id IN ?")
		scopeArgs = append(scopeArgs, filter.FactoryIDs)
	}
	if len(filter.ZoneIDs) > 0 {
		scopeClauses = append(scopeClauses, "a.zone_id IN ?")
		scopeArgs = append(scopeArgs, filter.ZoneIDs)
	}
	if len(filter.CameraIDs) > 0 {
		scopeClauses = append(scopeClauses, "a.camera_id IN ?")
		scopeArgs = append(scopeArgs, filter.CameraIDs)
	}
	if len(filter.RecorderIDs) > 0 {
		scopeClauses = append(scopeClauses, "a.recorder_id IN ?")
		scopeArgs = append(scopeArgs, filter.RecorderIDs)
	}
	if len(filter.ChannelIDs) > 0 {
		scopeClauses = append(scopeClauses, "a.channel_id IN ?")
		scopeArgs = append(scopeArgs, filter.ChannelIDs)
	}
	if len(scopeClauses) > 0 {
		db = db.Where("("+strings.Join(scopeClauses, " OR ")+")", scopeArgs...)
	}
	return db
}

type DashboardSummaryRow struct {
	TodayAlarmCount     int64
	PendingAlarmCount   int64
	CriticalAlarmCount  int64
	PushSuccessCount    int64
	PushFailedCount     int64
	CameraOnlineCount   int64
	CameraTotalCount    int64
	RecorderOnlineCount int64
	RecorderTotalCount  int64
}

func (r *Repository) GetDashboardSummary(startAt, endAt *time.Time, accessScope dto.AccessScopeFilter) (*DashboardSummaryRow, error) {
	startOfDay := time.Now().Truncate(24 * time.Hour)
	row := &DashboardSummaryRow{}

	todayQuery := r.db.Table("alarm_record")
	todayQuery = applyAlarmScopeToTable(todayQuery, "alarm_record", accessScope)
	if startAt != nil || endAt != nil {
		todayQuery = applyTimeRange(todayQuery, "alarm_time", startAt, endAt)
	} else {
		todayQuery = todayQuery.Where("alarm_time >= ?", startOfDay)
	}
	if err := todayQuery.Count(&row.TodayAlarmCount).Error; err != nil {
		return nil, fmt.Errorf("count today's alarms: %w", err)
	}
	if err := applyTimeRange(applyAlarmScopeToTable(r.db.Table("alarm_record").Where("status = ?", "pending"), "alarm_record", accessScope), "alarm_time", startAt, endAt).Count(&row.PendingAlarmCount).Error; err != nil {
		return nil, err
	}
	if err := applyTimeRange(applyAlarmScopeToTable(r.db.Table("alarm_record").Where("alarm_level = ?", "critical"), "alarm_record", accessScope), "alarm_time", startAt, endAt).Count(&row.CriticalAlarmCount).Error; err != nil {
		return nil, err
	}
	if err := applyTimeRange(applyPushScopeToTable(r.db.Table("alarm_push_log").Where("status = ?", "success"), "alarm_push_log", accessScope), "pushed_at", startAt, endAt).Count(&row.PushSuccessCount).Error; err != nil {
		return nil, err
	}
	if err := applyTimeRange(applyPushScopeToTable(r.db.Table("alarm_push_log").Where("status = ?", "failed"), "alarm_push_log", accessScope), "pushed_at", startAt, endAt).Count(&row.PushFailedCount).Error; err != nil {
		return nil, err
	}
	if err := applyCameraScopeToTable(r.db.Table("camera_device").Where("status = ?", "online"), "camera_device", accessScope).Count(&row.CameraOnlineCount).Error; err != nil {
		return nil, err
	}
	if err := applyCameraScopeToTable(r.db.Table("camera_device"), "camera_device", accessScope).Count(&row.CameraTotalCount).Error; err != nil {
		return nil, err
	}
	if err := applyRecorderScopeToTable(r.db.Table("recorder_device").Where("status = ?", "online"), "recorder_device", accessScope).Count(&row.RecorderOnlineCount).Error; err != nil {
		return nil, err
	}
	if err := applyRecorderScopeToTable(r.db.Table("recorder_device"), "recorder_device", accessScope).Count(&row.RecorderTotalCount).Error; err != nil {
		return nil, err
	}

	return row, nil
}

func applyAlarmScopeToTable(db *gorm.DB, alias string, accessScope dto.AccessScopeFilter) *gorm.DB {
	if accessScope.All {
		return db
	}
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if len(accessScope.FactoryIDs) > 0 {
		clauses = append(clauses, alias+".factory_id IN ?")
		args = append(args, accessScope.FactoryIDs)
	}
	if len(accessScope.ZoneIDs) > 0 {
		clauses = append(clauses, alias+".zone_id IN ?")
		args = append(args, accessScope.ZoneIDs)
	}
	if len(accessScope.CameraIDs) > 0 {
		clauses = append(clauses, alias+".camera_id IN ?")
		args = append(args, accessScope.CameraIDs)
	}
	if len(accessScope.RecorderIDs) > 0 {
		clauses = append(clauses, alias+".recorder_id IN ?")
		args = append(args, accessScope.RecorderIDs)
	}
	if len(accessScope.ChannelIDs) > 0 {
		clauses = append(clauses, alias+".channel_id IN ?")
		args = append(args, accessScope.ChannelIDs)
	}
	if len(clauses) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func applyPushScopeToTable(db *gorm.DB, alias string, accessScope dto.AccessScopeFilter) *gorm.DB {
	if accessScope.All {
		return db
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if len(accessScope.FactoryIDs) > 0 {
		clauses = append(clauses, alias+".factory_id IN ?")
		args = append(args, accessScope.FactoryIDs)
	}
	if len(accessScope.ZoneIDs) > 0 {
		clauses = append(clauses, alias+".zone_id IN ?")
		args = append(args, accessScope.ZoneIDs)
	}
	if len(clauses) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func applyCameraScopeToTable(db *gorm.DB, alias string, accessScope dto.AccessScopeFilter) *gorm.DB {
	if accessScope.All {
		return db
	}
	clauses := make([]string, 0, 3)
	args := make([]any, 0, 3)
	if len(accessScope.FactoryIDs) > 0 {
		clauses = append(clauses, alias+".factory_id IN ?")
		args = append(args, accessScope.FactoryIDs)
	}
	if len(accessScope.ZoneIDs) > 0 {
		clauses = append(clauses, alias+".zone_id IN ?")
		args = append(args, accessScope.ZoneIDs)
	}
	if len(accessScope.CameraIDs) > 0 {
		clauses = append(clauses, alias+".id IN ?")
		args = append(args, accessScope.CameraIDs)
	}
	if len(clauses) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func applyRecorderScopeToTable(db *gorm.DB, alias string, accessScope dto.AccessScopeFilter) *gorm.DB {
	if accessScope.All {
		return db
	}
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if len(accessScope.FactoryIDs) > 0 {
		clauses = append(clauses, alias+".factory_id IN ?")
		args = append(args, accessScope.FactoryIDs)
	}
	if len(accessScope.RecorderIDs) > 0 {
		clauses = append(clauses, alias+".id IN ?")
		args = append(args, accessScope.RecorderIDs)
	}
	if len(clauses) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func applyChannelScopeToTable(db *gorm.DB, alias string, accessScope dto.AccessScopeFilter) *gorm.DB {
	if accessScope.All {
		return db
	}
	clauses := make([]string, 0, 5)
	args := make([]any, 0, 5)
	if len(accessScope.FactoryIDs) > 0 {
		clauses = append(clauses, alias+".factory_id IN ?")
		args = append(args, accessScope.FactoryIDs)
	}
	if len(accessScope.ZoneIDs) > 0 {
		clauses = append(clauses, alias+".zone_id IN ?")
		args = append(args, accessScope.ZoneIDs)
	}
	if len(accessScope.CameraIDs) > 0 {
		clauses = append(clauses, alias+".camera_id IN ?")
		args = append(args, accessScope.CameraIDs)
	}
	if len(accessScope.RecorderIDs) > 0 {
		clauses = append(clauses, alias+".recorder_id IN ?")
		args = append(args, accessScope.RecorderIDs)
	}
	if len(accessScope.ChannelIDs) > 0 {
		clauses = append(clauses, alias+".id IN ?")
		args = append(args, accessScope.ChannelIDs)
	}
	if len(clauses) == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("("+strings.Join(clauses, " OR ")+")", args...)
}

func applyTimeRange(db *gorm.DB, column string, startAt, endAt *time.Time) *gorm.DB {
	if startAt != nil {
		db = db.Where(column+" >= ?", *startAt)
	}
	if endAt != nil {
		db = db.Where(column+" <= ?", *endAt)
	}
	return db
}
