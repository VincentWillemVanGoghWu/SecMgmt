package service

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/integration/hikvision"
	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/util"

	driverMysql "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type PlatformService struct {
	cfg             *config.Config
	repo            *repository.Repository
	logger          *zap.Logger
	hikvisionBridge *HikvisionAlarmBridgeService
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

type SmartEventListFilter struct {
	Keyword        string
	ProviderCode   string
	CapabilityCode string
	Status         string
	SourceStage    string
	RecentDays     int
	AccessScope    *AccessScope
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

func (s *PlatformService) ListUsers(filter UserListFilter) ([]map[string]any, error) {
	type userRow struct {
		entity.User
		DeptName string `gorm:"column:dept_name"`
	}
	var users []userRow
	query := s.db().Table("sys_user AS u").
		Select("u.*, d.dept_name").
		Joins("LEFT JOIN sys_dept d ON d.id = u.dept_id").
		Order("u.id DESC")
	if filter.RoleID > 0 {
		query = query.Joins("JOIN sys_user_role ur ON ur.user_id = u.id").Where("ur.role_id = ?", filter.RoleID).Distinct("u.id, u.username, u.password_hash, u.real_name, u.dept_id, u.status, u.created_at, u.updated_at, d.dept_name")
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("(u.username LIKE ? OR u.real_name LIKE ?)", likeKeyword, likeKeyword)
	}
	if filter.Status != "" {
		query = query.Where("u.status = ?", filter.Status)
	}
	if filter.DeptID > 0 {
		query = query.Where("u.dept_id = ?", filter.DeptID)
	}
	if err := query.Scan(&users).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(users))
	for _, user := range users {
		roles, _ := s.repo.ListRolesByUserID(user.ID)
		roleList := make([]map[string]any, 0, len(roles))
		for _, role := range roles {
			roleList = append(roleList, map[string]any{
				"id":       role.ID,
				"roleCode": role.RoleCode,
				"roleName": role.RoleName,
				"status":   role.Status,
			})
		}
		result = append(result, map[string]any{
			"id":        user.ID,
			"username":  user.Username,
			"realName":  user.RealName,
			"deptId":    user.DeptID,
			"deptName":  nullableString(userRowDeptName(user.DeptName)),
			"status":    user.Status,
			"roles":     roleList,
			"createdAt": user.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, nil
}

func (s *PlatformService) CreateUser(payload UserPayload) (map[string]any, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := entity.User{
		Username:     strings.TrimSpace(payload.Username),
		PasswordHash: string(passwordHash),
		RealName:     strings.TrimSpace(payload.RealName),
		DeptID:       payload.DeptID,
		Status:       normalizedStatus(payload.Status, "enabled"),
	}
	if err := s.db().Create(&user).Error; err != nil {
		return nil, err
	}
	if err := s.replaceUserRoles(user.ID, payload.RoleIDs); err != nil {
		return nil, err
	}
	return s.GetUserRecord(user.ID)
}

func (s *PlatformService) UpdateUser(userID uint, payload UserPayload) (map[string]any, error) {
	var user entity.User
	if err := s.db().First(&user, userID).Error; err != nil {
		return nil, err
	}
	user.RealName = strings.TrimSpace(payload.RealName)
	user.DeptID = payload.DeptID
	user.Status = normalizedStatus(payload.Status, user.Status)
	if payload.Password != "" {
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = string(passwordHash)
	}
	if err := s.db().Save(&user).Error; err != nil {
		return nil, err
	}
	if err := s.replaceUserRoles(user.ID, payload.RoleIDs); err != nil {
		return nil, err
	}
	return s.GetUserRecord(user.ID)
}

func (s *PlatformService) DeleteUser(userID uint) error {
	_ = s.db().Table("sys_user_role").Where("user_id = ?", userID).Delete(nil).Error
	return s.db().Delete(&entity.User{}, userID).Error
}

func (s *PlatformService) GetUserRecord(userID uint) (map[string]any, error) {
	all, err := s.ListUsers(UserListFilter{})
	if err != nil {
		return nil, err
	}
	for _, item := range all {
		if toUint(item["id"]) == userID {
			return item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) ListRoles(filter RoleListFilter) ([]map[string]any, error) {
	var roles []entity.Role
	query := s.db().Order("id ASC")
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		likeKeyword := "%" + keyword + "%"
		query = query.Where("(role_code LIKE ? OR role_name LIKE ? OR remark LIKE ?)", likeKeyword, likeKeyword, likeKeyword)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if err := query.Find(&roles).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		menuCodes := []string{}
		permissionCodes := []string{}
		_ = s.db().Table("sys_menu AS m").Select("m.code").Joins("JOIN sys_role_menu rm ON rm.menu_id = m.id").Where("rm.role_id = ?", role.ID).Order("m.id ASC").Scan(&menuCodes).Error
		menuCodes = filterHiddenMenuCodes(menuCodes)
		_ = s.db().Table("sys_permission AS p").Select("p.code").Joins("JOIN sys_role_permission rp ON rp.permission_id = p.id").Where("rp.role_id = ?", role.ID).Order("p.id ASC").Scan(&permissionCodes).Error
		permissionCodes = filterHiddenPermissionCodes(permissionCodes)
		result = append(result, map[string]any{
			"id":              role.ID,
			"roleCode":        role.RoleCode,
			"roleName":        role.RoleName,
			"status":          role.Status,
			"remark":          nullableString(role.Remark),
			"dataScopeType":   role.DataScopeType,
			"dataScopeValue":  nullableString(role.DataScopeValue),
			"menuCodes":       menuCodes,
			"permissionCodes": permissionCodes,
		})
	}
	return result, nil
}

func (s *PlatformService) CreateRole(payload RolePayload) (map[string]any, error) {
	item := entity.Role{
		RoleCode:      strings.TrimSpace(payload.RoleCode),
		RoleName:      strings.TrimSpace(payload.RoleName),
		Status:        normalizedStatus(payload.Status, "enabled"),
		Remark:        valueOrEmpty(payload.Remark),
		DataScopeType: "all",
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.GetRoleRecord(item.ID)
}

func (s *PlatformService) UpdateRole(roleID uint, payload RolePayload) (map[string]any, error) {
	var item entity.Role
	if err := s.db().First(&item, roleID).Error; err != nil {
		return nil, err
	}
	item.RoleCode = strings.TrimSpace(payload.RoleCode)
	item.RoleName = strings.TrimSpace(payload.RoleName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return s.GetRoleRecord(roleID)
}

func (s *PlatformService) UpdateRoleStatus(roleID uint, payload RoleStatusPayload) (map[string]any, error) {
	var item entity.Role
	if err := s.db().First(&item, roleID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return s.GetRoleRecord(roleID)
}

func (s *PlatformService) UpdateRoleDataScope(roleID uint, payload RoleDataScopePayload) (map[string]any, error) {
	var item entity.Role
	if err := s.db().First(&item, roleID).Error; err != nil {
		return nil, err
	}
	item.DataScopeType = strings.TrimSpace(payload.DataScopeType)
	item.DataScopeValue = encodeJSON(payload.DataScopeValue)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return s.GetRoleRecord(roleID)
}

func (s *PlatformService) ListRoleMenuTree() ([]dto.MenuItem, error) {
	var menus []entity.Menu
	if err := s.db().
		Where("status = ?", "enabled").
		Where("code NOT IN ?", keysOfHiddenMenuCodeSet()).
		Order("sort ASC, id ASC").
		Find(&menus).Error; err != nil {
		return nil, err
	}
	return buildMenuTreeFromEntities(menus), nil
}

func (s *PlatformService) UpdateRoleMenus(roleID uint, payload RoleMenuPayload) (map[string]any, error) {
	var role entity.Role
	if err := s.db().First(&role, roleID).Error; err != nil {
		return nil, err
	}

	menuIDs := dedupeUintSlice(payload.MenuIDs)
	if len(menuIDs) > 0 {
		var count int64
		if err := s.db().
			Model(&entity.Menu{}).
			Where("id IN ?", menuIDs).
			Where("status = ?", "enabled").
			Where("code NOT IN ?", keysOfHiddenMenuCodeSet()).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count != int64(len(menuIDs)) {
			return nil, fmt.Errorf("menu ids contains invalid records")
		}
	}

	if err := s.db().Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("sys_role_menu").Where("role_id = ?", roleID).Delete(nil).Error; err != nil {
			return err
		}
		for _, menuID := range menuIDs {
			if err := tx.Table("sys_role_menu").Create(map[string]any{
				"role_id": roleID,
				"menu_id": menuID,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.GetRoleRecord(roleID)
}

func (s *PlatformService) ListRolePermissionOptions() ([]dto.PermissionOption, error) {
	var permissions []entity.Permission
	if err := s.db().
		Where("status = ?", "enabled").
		Where("code NOT IN ?", keysOfHiddenPermissionCodeSet()).
		Order("id ASC").
		Find(&permissions).Error; err != nil {
		return nil, err
	}
	return buildPermissionOptions(permissions), nil
}

func (s *PlatformService) UpdateRolePermissions(roleID uint, payload RolePermissionPayload) (map[string]any, error) {
	var role entity.Role
	if err := s.db().First(&role, roleID).Error; err != nil {
		return nil, err
	}

	permissionIDs := dedupeUintSlice(payload.PermissionIDs)
	if len(permissionIDs) > 0 {
		var count int64
		if err := s.db().
			Model(&entity.Permission{}).
			Where("id IN ?", permissionIDs).
			Where("status = ?", "enabled").
			Where("code NOT IN ?", keysOfHiddenPermissionCodeSet()).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count != int64(len(permissionIDs)) {
			return nil, fmt.Errorf("permission ids contains invalid records")
		}
	}

	if err := s.db().Transaction(func(tx *gorm.DB) error {
		hiddenPermissionIDs := make([]uint, 0)
		if err := tx.Table("sys_permission AS p").
			Select("p.id").
			Joins("JOIN sys_role_permission rp ON rp.permission_id = p.id").
			Where("rp.role_id = ?", roleID).
			Where("p.code IN ?", keysOfHiddenPermissionCodeSet()).
			Scan(&hiddenPermissionIDs).Error; err != nil {
			return err
		}

		if err := tx.Table("sys_role_permission").Where("role_id = ?", roleID).Delete(nil).Error; err != nil {
			return err
		}
		for _, permissionID := range append(hiddenPermissionIDs, permissionIDs...) {
			if err := tx.Table("sys_role_permission").Create(map[string]any{
				"role_id":       roleID,
				"permission_id": permissionID,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return s.GetRoleRecord(roleID)
}

func (s *PlatformService) DeleteRole(roleID uint) error {
	_ = s.db().Table("sys_user_role").Where("role_id = ?", roleID).Delete(nil).Error
	_ = s.db().Table("sys_role_menu").Where("role_id = ?", roleID).Delete(nil).Error
	_ = s.db().Table("sys_role_permission").Where("role_id = ?", roleID).Delete(nil).Error
	return s.db().Delete(&entity.Role{}, roleID).Error
}

func (s *PlatformService) GetRoleRecord(roleID uint) (map[string]any, error) {
	all, err := s.ListRoles(RoleListFilter{})
	if err != nil {
		return nil, err
	}
	for _, item := range all {
		if toUint(item["id"]) == roleID {
			return item, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) CreateFactory(payload FactoryPayload) (map[string]any, error) {
	factoryCode, err := s.ensureGeneratedCode("factory_area", "factory_code", strings.TrimSpace(payload.FactoryCode), "factory")
	if err != nil {
		return nil, err
	}
	item := entity.FactoryArea{
		FactoryCode: factoryCode,
		FactoryName: strings.TrimSpace(payload.FactoryName),
		Status:      normalizedStatus(payload.Status, "enabled"),
		Remark:      valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "factoryCode": item.FactoryCode, "factoryName": item.FactoryName, "status": item.Status, "remark": nullableString(item.Remark)}, nil
}

func (s *PlatformService) UpdateFactory(factoryID uint, payload FactoryPayload) (map[string]any, error) {
	var item entity.FactoryArea
	if err := s.db().First(&item, factoryID).Error; err != nil {
		return nil, err
	}
	item.FactoryName = strings.TrimSpace(payload.FactoryName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "factoryCode": item.FactoryCode, "factoryName": item.FactoryName, "status": item.Status, "remark": nullableString(item.Remark)}, nil
}

func (s *PlatformService) UpdateFactoryStatus(factoryID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.FactoryArea
	if err := s.db().First(&item, factoryID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"id": item.ID, "factoryCode": item.FactoryCode, "factoryName": item.FactoryName, "status": item.Status, "remark": nullableString(item.Remark)}, nil
}

func (s *PlatformService) DeleteFactory(factoryID uint) error {
	return s.db().Delete(&entity.FactoryArea{}, factoryID).Error
}

func (s *PlatformService) CreateZone(payload ZonePayload) (map[string]any, error) {
	zoneCode, err := s.ensureGeneratedCode("factory_zone", "zone_code", strings.TrimSpace(payload.ZoneCode), "zone")
	if err != nil {
		return nil, err
	}
	item := entity.FactoryZone{
		FactoryID: payload.FactoryID,
		ZoneCode:  zoneCode,
		ZoneName:  strings.TrimSpace(payload.ZoneName),
		Status:    normalizedStatus(payload.Status, "enabled"),
		Remark:    valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.zoneRecord(item.ID)
}

func (s *PlatformService) UpdateZone(zoneID uint, payload ZonePayload) (map[string]any, error) {
	var item entity.FactoryZone
	if err := s.db().First(&item, zoneID).Error; err != nil {
		return nil, err
	}
	item.FactoryID = payload.FactoryID
	item.ZoneName = strings.TrimSpace(payload.ZoneName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.zoneRecord(zoneID)
}

func (s *PlatformService) UpdateZoneStatus(zoneID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.FactoryZone
	if err := s.db().First(&item, zoneID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.zoneRecord(zoneID)
}

func (s *PlatformService) DeleteZone(zoneID uint) error {
	return s.db().Delete(&entity.FactoryZone{}, zoneID).Error
}

func (s *PlatformService) CreateDept(payload DeptPayload) (map[string]any, error) {
	deptCode, err := s.ensureGeneratedCode("sys_dept", "dept_code", strings.TrimSpace(payload.DeptCode), "dept")
	if err != nil {
		return nil, err
	}
	item := entity.SysDept{
		DeptCode:  deptCode,
		DeptName:  strings.TrimSpace(payload.DeptName),
		ParentID:  payload.ParentID,
		FactoryID: payload.FactoryID,
		ZoneID:    payload.ZoneID,
		Leader:    valueOrEmpty(payload.Leader),
		Phone:     valueOrEmpty(payload.Phone),
		Sort:      payload.Sort,
		Status:    normalizedStatus(payload.Status, "enabled"),
		Remark:    valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.deptRecord(item.ID)
}

func (s *PlatformService) UpdateDept(deptID uint, payload DeptPayload) (map[string]any, error) {
	var item entity.SysDept
	if err := s.db().First(&item, deptID).Error; err != nil {
		return nil, err
	}
	item.DeptName = strings.TrimSpace(payload.DeptName)
	item.ParentID = payload.ParentID
	item.FactoryID = payload.FactoryID
	item.ZoneID = payload.ZoneID
	item.Leader = valueOrEmpty(payload.Leader)
	item.Phone = valueOrEmpty(payload.Phone)
	item.Sort = payload.Sort
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.deptRecord(deptID)
}

func (s *PlatformService) UpdateDeptStatus(deptID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.SysDept
	if err := s.db().First(&item, deptID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.deptRecord(deptID)
}

func (s *PlatformService) DeleteDept(deptID uint) error {
	return s.db().Delete(&entity.SysDept{}, deptID).Error
}

func (s *PlatformService) CreateDictType(payload DictTypePayload) (map[string]any, error) {
	item := entity.SysDictType{
		DictCode: strings.TrimSpace(payload.DictCode),
		DictName: strings.TrimSpace(payload.DictName),
		Status:   normalizedStatus(payload.Status, "enabled"),
		Remark:   valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.dictTypeRecord(item.ID)
}

func (s *PlatformService) UpdateDictType(dictTypeID uint, payload DictTypePayload) (map[string]any, error) {
	var item entity.SysDictType
	if err := s.db().First(&item, dictTypeID).Error; err != nil {
		return nil, err
	}
	item.DictCode = strings.TrimSpace(payload.DictCode)
	item.DictName = strings.TrimSpace(payload.DictName)
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictTypeRecord(dictTypeID)
}

func (s *PlatformService) UpdateDictTypeStatus(dictTypeID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.SysDictType
	if err := s.db().First(&item, dictTypeID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictTypeRecord(dictTypeID)
}

func (s *PlatformService) DeleteDictType(dictTypeID uint) error {
	_ = s.db().Where("dict_type_id = ?", dictTypeID).Delete(&entity.SysDictItem{}).Error
	return s.db().Delete(&entity.SysDictType{}, dictTypeID).Error
}

func (s *PlatformService) CreateDictItem(payload DictItemPayload) (map[string]any, error) {
	item := entity.SysDictItem{
		DictTypeID: payload.DictTypeID,
		ItemLabel:  strings.TrimSpace(payload.ItemLabel),
		ItemValue:  strings.TrimSpace(payload.ItemValue),
		ItemSort:   payload.ItemSort,
		IsDefault:  payload.IsDefault,
		Status:     normalizedStatus(payload.Status, "enabled"),
		Remark:     valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.dictItemRecord(item.ID)
}

func (s *PlatformService) UpdateDictItem(itemID uint, payload DictItemPayload) (map[string]any, error) {
	var item entity.SysDictItem
	if err := s.db().First(&item, itemID).Error; err != nil {
		return nil, err
	}
	item.DictTypeID = payload.DictTypeID
	item.ItemLabel = strings.TrimSpace(payload.ItemLabel)
	item.ItemValue = strings.TrimSpace(payload.ItemValue)
	item.ItemSort = payload.ItemSort
	item.IsDefault = payload.IsDefault
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictItemRecord(itemID)
}

func (s *PlatformService) UpdateDictItemStatus(itemID uint, payload StatusPayload) (map[string]any, error) {
	var item entity.SysDictItem
	if err := s.db().First(&item, itemID).Error; err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.dictItemRecord(itemID)
}

func (s *PlatformService) DeleteDictItem(itemID uint) error {
	return s.db().Delete(&entity.SysDictItem{}, itemID).Error
}

func (s *PlatformService) GetCamera(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListCameras(CameraListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == cameraID {
			return cameraDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) CreateCamera(payload CameraPayload, accessScope *AccessScope) (map[string]any, error) {
	if err := validateCameraTargetAccess(accessScope, payload.FactoryID, payload.ZoneID); err != nil {
		return nil, err
	}
	encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt camera password: %w", err)
	}
	item := entity.CameraDevice{
		DeviceCode:        payload.DeviceCode,
		Name:              payload.Name,
		IP:                payload.IP,
		SDKPort:           defaultInt(payload.SDKPort, 8000),
		HTTPPort:          defaultInt(payload.HTTPPort, 80),
		RTSPPort:          defaultInt(payload.RTSPPort, 554),
		Username:          payload.Username,
		PasswordEncrypted: encryptedPassword,
		FactoryID:         payload.FactoryID,
		ZoneID:            payload.ZoneID,
		InstallLocation:   valueOrEmpty(payload.InstallLocation),
		SupportAI:         payload.SupportAI,
		Status:            normalizedStatus(payload.Status, "offline"),
		Remark:            valueOrEmpty(payload.Remark),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.GetCamera(item.ID, accessScope)
}

func (s *PlatformService) UpdateCamera(cameraID uint, payload CameraPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	if err := validateCameraTargetAccess(accessScope, payload.FactoryID, payload.ZoneID); err != nil {
		return nil, err
	}
	item.DeviceCode = payload.DeviceCode
	item.Name = payload.Name
	item.IP = payload.IP
	item.SDKPort = defaultInt(payload.SDKPort, item.SDKPort)
	item.HTTPPort = defaultInt(payload.HTTPPort, item.HTTPPort)
	item.RTSPPort = defaultInt(payload.RTSPPort, item.RTSPPort)
	item.Username = payload.Username
	if payload.Password != "" {
		encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt camera password: %w", err)
		}
		item.PasswordEncrypted = encryptedPassword
	}
	item.FactoryID = payload.FactoryID
	item.ZoneID = payload.ZoneID
	item.InstallLocation = valueOrEmpty(payload.InstallLocation)
	item.SupportAI = payload.SupportAI
	item.Status = normalizedStatus(payload.Status, item.Status)
	item.Remark = valueOrEmpty(payload.Remark)
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.GetCamera(cameraID, accessScope)
}

func (s *PlatformService) UpdateCameraStatus(cameraID uint, status string, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	item.Status = normalizedStatus(status, item.Status)
	now := time.Now()
	item.LastOnlineAt = &now
	if err := s.db().Save(&item).Error; err != nil {
		return nil, err
	}
	return s.GetCamera(cameraID, accessScope)
}

func (s *PlatformService) DeleteCamera(cameraID uint, accessScope *AccessScope) error {
	if _, err := s.ensureCameraAccessible(accessScope, cameraID); err != nil {
		return err
	}
	if err := s.db().Delete(&entity.CameraDevice{}, cameraID).Error; err != nil {
		return wrapDeviceDeleteError(err)
	}
	return nil
}

func (s *PlatformService) TestCameraConnection(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	record, err := s.GetCamera(cameraID, accessScope)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"success": true,
		"status":  "online",
		"message": "设备连接测试成功",
		"rtspUrl": fmt.Sprintf("rtsp://%s:%d/Streaming/Channels/101", record["ip"], record["rtspPort"]),
	}, nil
}

func (s *PlatformService) CheckCameraStatus(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	_, err := s.UpdateCameraStatus(cameraID, "online", accessScope)
	if err != nil {
		return nil, err
	}
	now := time.Now().Format(time.RFC3339)
	return map[string]any{
		"status":       "online",
		"lastOnlineAt": now,
		"message":      "状态检查完成",
	}, nil
}

func (s *PlatformService) ControlCameraPTZZoom(cameraID uint, action string, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), item.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve camera password: %w", err)
	}

	zoom := 0
	switch action {
	case "in":
		zoom = 60
	case "out":
		zoom = -60
	default:
		return nil, fmt.Errorf("unsupported PTZ zoom action %q", action)
	}

	client := &http.Client{Timeout: 8 * time.Second}
	baseURL := fmt.Sprintf("%s://%s", resolveDeviceProtocol(item.HTTPPort), item.IP)
	if item.HTTPPort > 0 {
		baseURL = fmt.Sprintf("%s:%d", baseURL, item.HTTPPort)
	}
	targetURL := baseURL + "/ISAPI/PTZCtrl/channels/1/continuous"
	startPayload := buildHikPTZContinuousPayload(0, 0, zoom)
	stopPayload := buildHikPTZContinuousPayload(0, 0, 0)

	if err := hikHTTPPutXML(client, targetURL, item.Username, password, startPayload); err != nil {
		return nil, fmt.Errorf("start PTZ zoom: %w", err)
	}
	time.Sleep(350 * time.Millisecond)
	if err := hikHTTPPutXML(client, targetURL, item.Username, password, stopPayload); err != nil {
		return nil, fmt.Errorf("stop PTZ zoom: %w", err)
	}
	return map[string]any{
		"success": true,
		"message": "镜头动作已执行",
		"action":  action,
	}, nil
}

func (s *PlatformService) GetCameraBrowserLogin(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), item.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve camera password: %w", err)
	}
	return map[string]any{
		"ip":       item.IP,
		"port":     item.HTTPPort,
		"protocol": "http",
		"username": item.Username,
		"password": password,
		"loginUrl": fmt.Sprintf("http://%s:%d", item.IP, item.HTTPPort),
	}, nil
}

func (s *PlatformService) FetchCameraDeviceIdentity(payload CameraPayload) map[string]any {
	return map[string]any{
		"deviceName":     fmt.Sprintf("%s-%s", payload.Name, payload.DeviceCode),
		"deviceModel":    "Hikvision Camera",
		"deviceSerialNo": fmt.Sprintf("CAM-%s", strings.ToUpper(payload.DeviceCode)),
	}
}

func (s *PlatformService) GetCameraSDKConfig(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureCameraAccessible(accessScope, cameraID)
	if err != nil {
		return nil, err
	}
	return defaultCameraSDKConfig(*item), nil
}

func (s *PlatformService) UpdateCameraSDKSubConfig(cameraID uint, accessScope *AccessScope) (map[string]any, error) {
	return s.GetCameraSDKConfig(cameraID, accessScope)
}

func (s *PlatformService) GetRecorder(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	items, err := NewQueryService(s.repo).ListRecorders(RecorderListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.ID == recorderID {
			return recorderDTOToMap(item), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) CreateRecorder(payload RecorderPayload, accessScope *AccessScope) (map[string]any, error) {
	if err := validateRecorderTargetAccess(accessScope, payload.FactoryID); err != nil {
		return nil, err
	}
	encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt recorder password: %w", err)
	}
	item := entity.RecorderDevice{
		DeviceCode:        payload.DeviceCode,
		Name:              payload.Name,
		IP:                payload.IP,
		SDKPort:           defaultInt(payload.SDKPort, 8000),
		HTTPPort:          defaultInt(payload.HTTPPort, 80),
		Username:          payload.Username,
		PasswordEncrypted: encryptedPassword,
		ChannelCount:      payload.ChannelCount,
		FactoryID:         payload.FactoryID,
		Status:            normalizedStatus(payload.Status, "offline"),
	}
	if err := s.db().Create(&item).Error; err != nil {
		return nil, err
	}
	return s.GetRecorder(item.ID, accessScope)
}

func (s *PlatformService) UpdateRecorder(recorderID uint, payload RecorderPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureRecorderAccessible(accessScope, recorderID)
	if err != nil {
		return nil, err
	}
	if err := validateRecorderTargetAccess(accessScope, payload.FactoryID); err != nil {
		return nil, err
	}
	item.DeviceCode = payload.DeviceCode
	item.Name = payload.Name
	item.IP = payload.IP
	item.SDKPort = defaultInt(payload.SDKPort, item.SDKPort)
	item.HTTPPort = defaultInt(payload.HTTPPort, item.HTTPPort)
	item.Username = payload.Username
	if payload.Password != "" {
		encryptedPassword, err := util.EncryptDeviceSecret(s.deviceSecretKey(), payload.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt recorder password: %w", err)
		}
		item.PasswordEncrypted = encryptedPassword
	}
	item.ChannelCount = payload.ChannelCount
	item.FactoryID = payload.FactoryID
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return s.GetRecorder(recorderID, accessScope)
}

func (s *PlatformService) DeleteRecorder(recorderID uint, accessScope *AccessScope) error {
	if _, err := s.ensureRecorderAccessible(accessScope, recorderID); err != nil {
		return err
	}
	if err := s.db().Where("recorder_id = ?", recorderID).Delete(&entity.RecorderChannel{}).Error; err != nil {
		return wrapDeviceDeleteError(err)
	}
	if err := s.db().Delete(&entity.RecorderDevice{}, recorderID).Error; err != nil {
		return wrapDeviceDeleteError(err)
	}
	return nil
}

func (s *PlatformService) TestRecorderConnection(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	if _, err := s.ensureRecorderAccessible(accessScope, recorderID); err != nil {
		return nil, err
	}
	return map[string]any{"success": true, "status": "online", "message": "录像机连接测试成功"}, nil
}

func (s *PlatformService) CheckRecorderStatus(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureRecorderAccessible(accessScope, recorderID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	item.Status = "online"
	item.LastOnlineAt = &now
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	return map[string]any{"status": "online", "lastOnlineAt": now.Format(time.RFC3339), "message": "状态检查完成"}, nil
}

func (s *PlatformService) SyncRecorderChannels(recorderID uint, accessScope *AccessScope) (map[string]any, error) {
	recorder, err := s.ensureRecorderAccessible(accessScope, recorderID)
	if err != nil {
		return nil, err
	}
	deviceChannels, err := s.fetchRecorderChannels(*recorder)
	if err != nil {
		if recorder.ChannelCount <= 0 {
			return nil, err
		}
		if s.logger != nil {
			s.logger.Warn("fetch recorder channels failed, fallback to configured channel count",
				zap.Uint("recorderId", recorder.ID),
				zap.String("recorderIP", recorder.IP),
				zap.Error(err),
			)
		}
		deviceChannels = buildDefaultRecorderSyncChannels(recorder.ChannelCount)
	}
	if len(deviceChannels) == 0 {
		if recorder.ChannelCount <= 0 {
			return nil, fmt.Errorf("recorder returned empty channel list")
		}
		deviceChannels = buildDefaultRecorderSyncChannels(recorder.ChannelCount)
	}
	deviceChannels = completeRecorderSyncChannels(deviceChannels, recorder.ChannelCount)

	now := time.Now()
	if err := s.db().Transaction(func(tx *gorm.DB) error {
		var existing []entity.RecorderChannel
		if err := tx.Where("recorder_id = ?", recorderID).Find(&existing).Error; err != nil {
			return err
		}
		existingByNo := make(map[int]entity.RecorderChannel, len(existing))
		for _, channel := range existing {
			existingByNo[channel.ChannelNo] = channel
		}

		for _, deviceChannel := range deviceChannels {
			channel, exists := existingByNo[deviceChannel.ChannelNo]
			if exists {
				channel.Name = defaultString(deviceChannel.Name, channel.Name)
				channel.FactoryID = recorder.FactoryID
				channel.Enabled = deviceChannel.Enabled
				channel.Status = normalizedStatus(deviceChannel.Status, channel.Status)
				if err := tx.Save(&channel).Error; err != nil {
					return err
				}
				continue
			}
			channel = entity.RecorderChannel{
				RecorderID:      recorderID,
				ChannelNo:       deviceChannel.ChannelNo,
				Name:            defaultString(deviceChannel.Name, fmt.Sprintf("通道 %02d", deviceChannel.ChannelNo)),
				FactoryID:       recorder.FactoryID,
				Enabled:         deviceChannel.Enabled,
				SupportPlayback: true,
				Status:          normalizedStatus(deviceChannel.Status, "online"),
			}
			if err := tx.Create(&channel).Error; err != nil {
				return err
			}
		}

		recorder.ChannelCount = len(deviceChannels)
		recorder.Status = "online"
		recorder.LastOnlineAt = &now
		return tx.Save(recorder).Error
	}); err != nil {
		return nil, err
	}
	channels, err := NewQueryService(s.repo).ListChannels(ChannelListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	filtered := make([]map[string]any, 0)
	for _, item := range channels {
		if item.RecorderID == recorderID {
			filtered = append(filtered, channelDTOToMap(item))
		}
	}
	return map[string]any{
		"recorderId":   recorder.ID,
		"recorderName": recorder.Name,
		"channelCount": len(filtered),
		"channels":     filtered,
	}, nil
}

type recorderSyncChannel struct {
	ChannelNo    int
	Name         string
	Enabled      bool
	Status       string
	NamePriority int
}

type hikVideoInputChannelList struct {
	Channels []hikVideoInputChannel `xml:"VideoInputChannel"`
}

type hikVideoInputChannel struct {
	ID      int    `xml:"id"`
	Name    string `xml:"name"`
	Enabled string `xml:"enabled"`
}

type hikInputProxyChannelStatusList struct {
	Channels []hikInputProxyChannelStatus `xml:"InputProxyChannelStatus"`
}

type hikInputProxyChannelStatus struct {
	ID                        int                          `xml:"id"`
	Name                      string                       `xml:"name"`
	DeviceName                string                       `xml:"deviceName"`
	SourceInputPortDescriptor hikSourceInputPortDescriptor `xml:"sourceInputPortDescriptor"`
	IPAddress                 string                       `xml:"ipAddress"`
	Online                    string                       `xml:"online"`
	Enabled                   string                       `xml:"enabled"`
}

type hikInputProxyChannelList struct {
	Channels []hikInputProxyChannel `xml:"InputProxyChannel"`
}

type hikInputProxyChannel struct {
	ID                        int                          `xml:"id"`
	Name                      string                       `xml:"name"`
	DeviceName                string                       `xml:"deviceName"`
	SourceInputPortDescriptor hikSourceInputPortDescriptor `xml:"sourceInputPortDescriptor"`
	IPAddress                 string                       `xml:"ipAddress"`
	Online                    string                       `xml:"online"`
	Enabled                   string                       `xml:"enabled"`
}

type hikSourceInputPortDescriptor struct {
	Text         string `xml:",chardata"`
	IPAddress    string `xml:"ipAddress"`
	Online       string `xml:"online"`
	SrcInputPort string `xml:"srcInputPort"`
}

func (s *PlatformService) fetchRecorderChannels(recorder entity.RecorderDevice) ([]recorderSyncChannel, error) {
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve recorder password: %w", err)
	}
	if strings.TrimSpace(recorder.IP) == "" {
		return nil, fmt.Errorf("recorder ip is empty")
	}

	client := &http.Client{Timeout: 8 * time.Second}
	baseURL := fmt.Sprintf("%s://%s", resolveDeviceProtocol(recorder.HTTPPort), recorder.IP)
	if recorder.HTTPPort > 0 {
		baseURL = fmt.Sprintf("%s:%d", baseURL, recorder.HTTPPort)
	}

	merged := make(map[int]recorderSyncChannel)
	for _, endpoint := range []struct {
		path string
		kind string
	}{
		{path: "/ISAPI/System/Video/inputs/channels", kind: "analog"},
		{path: "/ISAPI/ContentMgmt/InputProxy/channels/status", kind: "digital"},
		{path: "/ISAPI/ContentMgmt/InputProxy/channels", kind: "digital-config"},
	} {
		body, requestErr := hikHTTPGet(client, baseURL+endpoint.path, recorder.Username, password)
		if requestErr != nil {
			if s.logger != nil {
				s.logger.Debug("fetch recorder channel endpoint failed",
					zap.Uint("recorderId", recorder.ID),
					zap.String("endpoint", endpoint.path),
					zap.Error(requestErr),
				)
			}
			continue
		}
		channels, parseErr := parseHikRecorderChannels(body, endpoint.kind)
		if parseErr != nil {
			if s.logger != nil {
				s.logger.Debug("parse recorder channel endpoint failed",
					zap.Uint("recorderId", recorder.ID),
					zap.String("endpoint", endpoint.path),
					zap.Error(parseErr),
				)
			}
			continue
		}
		for _, channel := range channels {
			if channel.ChannelNo <= 0 {
				continue
			}
			if existing, ok := merged[channel.ChannelNo]; ok {
				merged[channel.ChannelNo] = mergeRecorderSyncChannel(existing, channel)
				continue
			}
			merged[channel.ChannelNo] = channel
		}
	}
	if len(merged) == 0 {
		return nil, fmt.Errorf("recorder channel list is empty")
	}

	channels := make([]recorderSyncChannel, 0, len(merged))
	for _, channel := range merged {
		channels = append(channels, channel)
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ChannelNo < channels[j].ChannelNo
	})
	return channels, nil
}

func parseHikRecorderChannels(body []byte, kind string) ([]recorderSyncChannel, error) {
	switch kind {
	case "analog":
		var parsed hikVideoInputChannelList
		if err := xml.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		channels := make([]recorderSyncChannel, 0, len(parsed.Channels))
		for index, item := range parsed.Channels {
			channelNo := item.ID
			if channelNo <= 0 {
				channelNo = index + 1
			}
			enabled := parseOptionalBool(item.Enabled, true)
			channels = append(channels, recorderSyncChannel{
				ChannelNo:    channelNo,
				Name:         strings.TrimSpace(item.Name),
				Enabled:      enabled,
				Status:       chooseString(enabled, "online", "offline"),
				NamePriority: 10,
			})
		}
		return channels, nil
	case "digital":
		var parsed hikInputProxyChannelStatusList
		if err := xml.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		channels := make([]recorderSyncChannel, 0, len(parsed.Channels))
		for index, item := range parsed.Channels {
			channelNo := item.ID
			if channelNo <= 0 {
				channelNo = index + 1
			}
			enabled := parseOptionalBool(item.Enabled, true)
			online := parseOptionalBool(defaultString(item.Online, item.SourceInputPortDescriptor.Online), enabled)
			name, namePriority := resolveHikChannelDeviceName(
				item.DeviceName,
				item.Name,
				item.IPAddress,
				item.SourceInputPortDescriptor.IPAddress,
				item.SourceInputPortDescriptor.Text,
			)
			channels = append(channels, recorderSyncChannel{
				ChannelNo:    channelNo,
				Name:         name,
				Enabled:      enabled,
				Status:       chooseString(online, "online", "offline"),
				NamePriority: namePriority,
			})
		}
		return channels, nil
	case "digital-config":
		var parsed hikInputProxyChannelList
		if err := xml.Unmarshal(body, &parsed); err != nil {
			return nil, err
		}
		channels := make([]recorderSyncChannel, 0, len(parsed.Channels))
		for index, item := range parsed.Channels {
			channelNo := item.ID
			if channelNo <= 0 {
				channelNo = index + 1
			}
			enabled := parseOptionalBool(item.Enabled, true)
			online := parseOptionalBool(defaultString(item.Online, item.SourceInputPortDescriptor.Online), enabled)
			name, namePriority := resolveHikChannelDeviceName(
				item.DeviceName,
				item.Name,
				item.IPAddress,
				item.SourceInputPortDescriptor.IPAddress,
				item.SourceInputPortDescriptor.Text,
			)
			channels = append(channels, recorderSyncChannel{
				ChannelNo:    channelNo,
				Name:         name,
				Enabled:      enabled,
				Status:       chooseString(online, "online", "offline"),
				NamePriority: namePriority,
			})
		}
		return channels, nil
	default:
		return nil, fmt.Errorf("unsupported channel kind %q", kind)
	}
}

func buildDefaultRecorderSyncChannels(count int) []recorderSyncChannel {
	channels := make([]recorderSyncChannel, 0, maxInt(count, 0))
	for i := 1; i <= count; i++ {
		channels = append(channels, recorderSyncChannel{
			ChannelNo: i,
			Name:      fmt.Sprintf("通道 %02d", i),
			Enabled:   true,
			Status:    "online",
		})
	}
	return channels
}

func completeRecorderSyncChannels(channels []recorderSyncChannel, expectedCount int) []recorderSyncChannel {
	if expectedCount <= 0 || len(channels) >= expectedCount {
		return channels
	}
	merged := make(map[int]recorderSyncChannel, expectedCount)
	for _, channel := range channels {
		if channel.ChannelNo <= 0 {
			continue
		}
		if existing, ok := merged[channel.ChannelNo]; ok {
			merged[channel.ChannelNo] = mergeRecorderSyncChannel(existing, channel)
			continue
		}
		merged[channel.ChannelNo] = channel
	}
	for _, channel := range buildDefaultRecorderSyncChannels(expectedCount) {
		if _, exists := merged[channel.ChannelNo]; !exists {
			merged[channel.ChannelNo] = channel
		}
	}
	completed := make([]recorderSyncChannel, 0, len(merged))
	for _, channel := range merged {
		completed = append(completed, channel)
	}
	sort.Slice(completed, func(i, j int) bool {
		return completed[i].ChannelNo < completed[j].ChannelNo
	})
	return completed
}

func mergeRecorderSyncChannel(current, next recorderSyncChannel) recorderSyncChannel {
	merged := current
	if next.Enabled != current.Enabled {
		merged.Enabled = next.Enabled
	}
	if strings.TrimSpace(next.Status) != "" {
		merged.Status = next.Status
	}
	if strings.TrimSpace(next.Name) != "" && next.NamePriority >= current.NamePriority {
		merged.Name = next.Name
		merged.NamePriority = next.NamePriority
	}
	return merged
}

func resolveHikChannelDeviceName(values ...string) (string, int) {
	for index, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		return normalized, 40 - index*10
	}
	return "", 0
}

func parseOptionalBool(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on", "enabled":
		return true
	case "false", "0", "no", "off", "disabled":
		return false
	default:
		return fallback
	}
}

func chooseString(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func buildHikPTZContinuousPayload(pan, tilt, zoom int) []byte {
	return []byte(fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?><PTZData><pan>%d</pan><tilt>%d</tilt><zoom>%d</zoom></PTZData>`,
		pan,
		tilt,
		zoom,
	))
}

func hikHTTPGet(client *http.Client, targetURL, username, password string) ([]byte, error) {
	body, status, wwwAuthenticate, err := hikHTTPGetOnce(client, targetURL, username, password, "")
	if err == nil && status >= 200 && status < 300 {
		return body, nil
	}
	if status != http.StatusUnauthorized || !strings.Contains(strings.ToLower(wwwAuthenticate), "digest") {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("GET %s returned status %d", targetURL, status)
	}

	authorization, digestErr := buildDigestAuthorization(wwwAuthenticate, "GET", targetURL, username, password)
	if digestErr != nil {
		return nil, digestErr
	}
	body, status, _, err = hikHTTPGetOnce(client, targetURL, username, password, authorization)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("GET %s returned status %d", targetURL, status)
	}
	return body, nil
}

func hikHTTPPutXML(client *http.Client, targetURL, username, password string, payload []byte) error {
	_, status, wwwAuthenticate, err := hikHTTPPutXMLOnce(client, targetURL, username, password, payload, "")
	if err == nil && status >= 200 && status < 300 {
		return nil
	}
	if status != http.StatusUnauthorized || !strings.Contains(strings.ToLower(wwwAuthenticate), "digest") {
		if err != nil {
			return err
		}
		return fmt.Errorf("PUT %s returned status %d", targetURL, status)
	}

	authorization, digestErr := buildDigestAuthorization(wwwAuthenticate, "PUT", targetURL, username, password)
	if digestErr != nil {
		return digestErr
	}
	_, status, _, err = hikHTTPPutXMLOnce(client, targetURL, username, password, payload, authorization)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("PUT %s returned status %d", targetURL, status)
	}
	return nil
}

func hikHTTPGetOnce(client *http.Client, targetURL, username, password, authorization string) ([]byte, int, string, error) {
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, 0, "", err
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	} else if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), err
	}
	return body, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), nil
}

func hikHTTPPutXMLOnce(client *http.Client, targetURL, username, password string, payload []byte, authorization string) ([]byte, int, string, error) {
	req, err := http.NewRequest(http.MethodPut, targetURL, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, "", err
	}
	req.Header.Set("Content-Type", "application/xml")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	} else if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), err
	}
	return body, resp.StatusCode, resp.Header.Get("WWW-Authenticate"), nil
}

func buildDigestAuthorization(challenge, method, targetURL, username, password string) (string, error) {
	values := parseDigestChallenge(challenge)
	realm := values["realm"]
	nonce := values["nonce"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("invalid digest challenge")
	}
	parsedURL, err := http.NewRequest(method, targetURL, nil)
	if err != nil {
		return "", err
	}
	uri := parsedURL.URL.RequestURI()
	qop := values["qop"]
	if strings.Contains(qop, ",") {
		for _, item := range strings.Split(qop, ",") {
			if strings.TrimSpace(item) == "auth" {
				qop = "auth"
				break
			}
		}
	}
	if qop != "" && qop != "auth" {
		qop = "auth"
	}
	nc := "00000001"
	cnonce := strings.ReplaceAll(uuid.NewString(), "-", "")
	ha1 := md5Hex(fmt.Sprintf("%s:%s:%s", username, realm, password))
	ha2 := md5Hex(fmt.Sprintf("%s:%s", method, uri))
	response := ""
	if qop == "" {
		response = md5Hex(fmt.Sprintf("%s:%s:%s", ha1, nonce, ha2))
	} else {
		response = md5Hex(fmt.Sprintf("%s:%s:%s:%s:%s:%s", ha1, nonce, nc, cnonce, qop, ha2))
	}

	parts := []string{
		fmt.Sprintf(`username="%s"`, digestQuote(username)),
		fmt.Sprintf(`realm="%s"`, digestQuote(realm)),
		fmt.Sprintf(`nonce="%s"`, digestQuote(nonce)),
		fmt.Sprintf(`uri="%s"`, digestQuote(uri)),
		fmt.Sprintf(`response="%s"`, response),
	}
	if values["opaque"] != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, digestQuote(values["opaque"])))
	}
	if qop != "" {
		parts = append(parts,
			fmt.Sprintf(`qop=%s`, qop),
			fmt.Sprintf(`nc=%s`, nc),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
		)
	}
	if values["algorithm"] != "" {
		parts = append(parts, fmt.Sprintf(`algorithm=%s`, values["algorithm"]))
	}
	return "Digest " + strings.Join(parts, ", "), nil
}

func parseDigestChallenge(challenge string) map[string]string {
	challenge = strings.TrimSpace(challenge)
	challenge = strings.TrimPrefix(challenge, "Digest")
	challenge = strings.TrimSpace(challenge)
	values := make(map[string]string)
	for _, part := range strings.Split(challenge, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.Trim(strings.TrimSpace(value), `"`)
	}
	return values
}

func md5Hex(value string) string {
	sum := md5.Sum([]byte(value))
	return fmt.Sprintf("%x", sum)
}

func digestQuote(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func (s *PlatformService) ListRecorderChannels(recorderID uint, accessScope *AccessScope) ([]map[string]any, error) {
	if _, err := s.ensureRecorderAccessible(accessScope, recorderID); err != nil {
		return nil, err
	}
	channels, err := NewQueryService(s.repo).ListChannels(ChannelListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	filtered := make([]map[string]any, 0)
	for _, item := range channels {
		if item.RecorderID == recorderID {
			filtered = append(filtered, channelDTOToMap(item))
		}
	}
	return filtered, nil
}

func (s *PlatformService) UpdateChannel(channelID uint, payload ChannelPayload, accessScope *AccessScope) (map[string]any, error) {
	item, err := s.ensureChannelAccessible(accessScope, channelID)
	if err != nil {
		return nil, err
	}
	if err := validateChannelTargetAccess(accessScope, payload.FactoryID, payload.ZoneID, payload.CameraID, item.RecorderID, channelID); err != nil {
		return nil, err
	}
	item.Name = payload.Name
	item.CameraID = payload.CameraID
	item.FactoryID = payload.FactoryID
	item.ZoneID = payload.ZoneID
	item.Enabled = payload.Enabled
	item.SupportPlayback = payload.SupportPlayback
	item.Status = normalizedStatus(payload.Status, item.Status)
	if err := s.db().Save(item).Error; err != nil {
		return nil, err
	}
	channels, err := NewQueryService(s.repo).ListChannels(ChannelListFilter{AccessScope: accessScope})
	if err != nil {
		return nil, err
	}
	for _, record := range channels {
		if record.ID == channelID {
			return channelDTOToMap(record), nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *PlatformService) ListDeviceStatusLogs(page, pageSize int, filter DeviceStatusLogListFilter, accessScope *AccessScope) (map[string]any, error) {
	query := s.db().Model(&entity.DeviceStatusLog{})
	if filter.DeviceType != "" {
		query = query.Where("device_type = ?", filter.DeviceType)
	}
	if filter.Status != "" {
		query = query.Where("new_status = ?", filter.Status)
	}
	if filter.StartAt != nil {
		query = query.Where("checked_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("checked_at <= ?", *filter.EndAt)
	}

	var items []entity.DeviceStatusLog
	if err := query.Order("checked_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	filtered := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if accessScope != nil && !s.canAccessStatusLog(item, accessScope) {
			continue
		}
		deviceName := s.resolveDeviceName(item.DeviceType, item.DeviceID)
		if keyword := strings.TrimSpace(filter.DeviceName); keyword != "" && !strings.Contains(strings.ToLower(deviceName), strings.ToLower(keyword)) {
			continue
		}
		filtered = append(filtered, map[string]any{
			"id":         item.ID,
			"deviceType": item.DeviceType,
			"deviceId":   item.DeviceID,
			"deviceName": deviceName,
			"oldStatus":  nullableString(item.OldStatus),
			"newStatus":  item.NewStatus,
			"message":    nullableString(item.Message),
			"checkedAt":  item.CheckedAt.Format(time.RFC3339),
		})
	}

	total := len(filtered)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return map[string]any{"items": filtered[start:end], "total": total, "page": page, "pageSize": pageSize}, nil
}

func (s *PlatformService) CheckAllDevicesStatus() (map[string]any, error) {
	var cameras []entity.CameraDevice
	var recorders []entity.RecorderDevice
	var channels []entity.RecorderChannel
	_ = s.db().Find(&cameras).Error
	_ = s.db().Find(&recorders).Error
	_ = s.db().Find(&channels).Error
	changed := 0
	for _, camera := range cameras {
		if err := s.insertStatusLog("camera", camera.ID, camera.Status, "online", "批量状态检查"); err == nil {
			changed++
		}
	}
	for _, recorder := range recorders {
		if err := s.insertStatusLog("recorder", recorder.ID, recorder.Status, "online", "批量状态检查"); err == nil {
			changed++
		}
	}
	for _, channel := range channels {
		if err := s.insertStatusLog("channel", channel.ID, channel.Status, "online", "批量状态检查"); err == nil {
			changed++
		}
	}
	return map[string]any{
		"checkedDevices":   len(cameras) + len(recorders) + len(channels),
		"changedDevices":   changed,
		"checkedCameras":   len(cameras),
		"checkedRecorders": len(recorders),
		"checkedChannels":  len(channels),
		"message":          "全部设备状态检查完成",
	}, nil
}

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
	rangeStart, rangeEnd := normalizeDashboardRange(startAt, endAt, 7)
	categories := []string{}
	seriesData := []int{}
	for day := truncateToDay(rangeStart); !day.After(truncateToDay(rangeEnd)); day = day.AddDate(0, 0, 1) {
		categories = append(categories, day.Format("01-02"))
		var count int64
		start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		end := start.Add(24 * time.Hour)
		query := s.db().Model(&entity.AlarmRecord{}).Where("alarm_time >= ? AND alarm_time < ?", start, end)
		query = s.applyAlarmAccessScopeQuery(query, "alarm_record", accessScope)
		query = applyOptionalTimeRange(query, "alarm_time", startAt, endAt)
		_ = query.Count(&count).Error
		seriesData = append(seriesData, int(count))
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
	var statusRows []nameValueRow
	var channelRows []nameValueRow
	var total, success, failed, rateLimited int64
	pushLogQuery := func() *gorm.DB {
		return applyOptionalTimeRange(s.applyPushLogAccessScopeQuery(s.db().Model(&entity.AlarmPushLog{}), "alarm_push_log", accessScope), "pushed_at", startAt, endAt)
	}
	pushLogTable := func() *gorm.DB {
		return applyOptionalTimeRange(s.applyPushLogAccessScopeQuery(s.db().Table("alarm_push_log"), "alarm_push_log", accessScope), "pushed_at", startAt, endAt)
	}
	_ = pushLogQuery().Count(&total).Error
	_ = pushLogQuery().Where("status = ?", "success").Count(&success).Error
	_ = pushLogQuery().Where("status = ?", "failed").Count(&failed).Error
	_ = pushLogQuery().Where("status = ?", "rate_limited").Count(&rateLimited).Error
	_ = pushLogTable().Select("status AS name, count(*) AS value").Group("status").Scan(&statusRows).Error
	_ = pushLogTable().Select("channel AS name, count(*) AS value").Group("channel").Scan(&channelRows).Error
	return map[string]any{
		"overview": map[string]any{
			"total": total, "success": success, "failed": failed, "rateLimited": rateLimited,
			"successRate": percent(success, total),
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

func (s *PlatformService) ListSmartProviders() ([]map[string]any, error) {
	var items []entity.SmartInterfaceProvider
	if err := s.db().Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, s.smartProviderMap(item))
	}
	return result, nil
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
	if provider.ProviderCode == "hikvision-sdk" {
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
		message := "Hikvision SDK bridge is not running"
		if running {
			message = "Hikvision SDK bridge is running"
		}
		if s.logger != nil {
			s.logger.Info("smart provider test",
				zap.String("providerCode", provider.ProviderCode),
				zap.Bool("success", running),
				zap.Any("hikvisionBridgeStatus", status),
			)
		}
		return map[string]any{
			"success":   running,
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
	if provider.ProviderCode != "hikvision-sdk" || capability.CapabilityCode != "motion_detect" {
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
	target, ok, err := s.hikvisionBridge.bindingToTarget(binding)
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
	switch {
	case !bindingIncluded:
		result["message"] = "当前绑定未处于可监听状态"
	case !running:
		result["message"] = "Hikvision SDK bridge 未运行"
	case !found:
		result["message"] = fmt.Sprintf("bridge 已运行，但未命中当前绑定会话 %s", target.SessionKey)
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

func (s *PlatformService) ListSmartBindings(filter SmartBindingListFilter) ([]map[string]any, error) {
	var bindings []entity.SmartDeviceBinding
	var providers []entity.SmartInterfaceProvider
	var capabilities []entity.SmartInterfaceCapability
	var rules []entity.SmartBindingRule
	_ = s.db().Find(&bindings).Error
	_ = s.db().Find(&providers).Error
	_ = s.db().Find(&capabilities).Error
	_ = s.db().Find(&rules).Error
	providerMap := make(map[uint]entity.SmartInterfaceProvider)
	capabilityMap := make(map[uint]entity.SmartInterfaceCapability)
	ruleCount := make(map[uint]int)
	for _, item := range providers {
		providerMap[item.ID] = item
	}
	for _, item := range capabilities {
		capabilityMap[item.ID] = item
	}
	for _, item := range rules {
		ruleCount[item.BindingID]++
	}
	result := make([]map[string]any, 0, len(bindings))
	for _, item := range bindings {
		provider := providerMap[item.ProviderID]
		capability := capabilityMap[item.CapabilityID]
		if filter.SourceType != "" && item.SourceType != filter.SourceType {
			continue
		}
		if filter.ProviderCode != "" && provider.ProviderCode != filter.ProviderCode {
			continue
		}
		if filter.CapabilityCode != "" && capability.CapabilityCode != filter.CapabilityCode {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		result = append(result, s.smartBindingMap(item, provider, capability, ruleCount[item.ID]))
	}
	sort.Slice(result, func(i, j int) bool { return toUint(result[i]["id"]) > toUint(result[j]["id"]) })
	return result, nil
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

func (s *PlatformService) ListSmartRawEvents() ([]map[string]any, error) {
	var items []entity.SmartRawEvent
	_ = s.db().Order("id DESC").Find(&items).Error
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		var provider entity.SmartInterfaceProvider
		var capability entity.SmartInterfaceCapability
		if item.ProviderID != 0 {
			_ = s.db().First(&provider, item.ProviderID).Error
		}
		if item.CapabilityID != nil {
			_ = s.db().First(&capability, *item.CapabilityID).Error
		}
		result = append(result, map[string]any{
			"id":             item.ID,
			"providerCode":   provider.ProviderCode,
			"providerName":   provider.ProviderName,
			"capabilityCode": nullableString(capability.CapabilityCode),
			"capabilityName": nullableString(capability.CapabilityName),
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
	return result, nil
}

func (s *PlatformService) ListSmartEvents(page, pageSize int, filter SmartEventListFilter) (map[string]any, error) {
	query := s.db().Table("smart_event AS e").
		Joins("LEFT JOIN smart_interface_provider p ON p.id = e.provider_id").
		Joins("LEFT JOIN smart_interface_capability c ON c.id = e.capability_id")
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
	var items []entity.SmartEvent
	if err := query.Select("e.*").Order("e.event_time DESC, e.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
		return nil, err
	}
	eventItems := s.buildSmartEvents(items)
	return map[string]any{"items": eventItems, "total": total, "page": page, "pageSize": pageSize}, nil
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

func (s *PlatformService) ListSmartAITasks(accessScope *AccessScope) ([]map[string]any, error) {
	var tasks []entity.AiReviewTask
	var results []entity.AiReviewResult
	query := s.db().Table("ai_review_task AS t").Joins("JOIN smart_event e ON e.id = t.smart_event_id")
	query = s.applySmartEventAccessScopeQuery(query, "e", accessScope)
	_ = query.Select("t.*").Order("t.submitted_at DESC, t.id DESC").Scan(&tasks).Error
	taskIDs := make([]uint, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	if len(taskIDs) > 0 {
		_ = s.db().Where("task_id IN ?", taskIDs).Order("id DESC").Find(&results).Error
	}
	return buildAITasks(tasks, results), nil
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

func (s *PlatformService) GetLiveVideo(sourceType string, id uint, streamType, streamProfile string, accessScope *AccessScope) (map[string]any, error) {
	switch sourceType {
	case "camera":
		if _, err := s.ensureCameraAccessible(accessScope, id); err != nil {
			return nil, err
		}
	case "channel":
		if _, err := s.ensureChannelAccessible(accessScope, id); err != nil {
			return nil, err
		}
	}
	playURL := fmt.Sprintf("%s/mock/live/%s/%d.m3u8?profile=%s", strings.TrimRight(s.cfg.BackendPublicBaseURL, "/"), sourceType, id, streamProfile)
	return map[string]any{
		"cameraId":          chooseID(sourceType == "camera", id),
		"channelId":         chooseID(sourceType == "channel", id),
		"streamType":        defaultString(streamType, "hik-sdk"),
		"connectionMode":    "hik-sdk",
		"playUrl":           playURL,
		"expiresIn":         300,
		"isMock":            true,
		"playableInBrowser": false,
		"diagnosticMessage": "当前为后端开发阶段的模拟播放地址",
		"sourceRtsp":        fmt.Sprintf("rtsp://mock/%s/%d", sourceType, id),
	}, nil
}

func (s *PlatformService) StopLiveVideo(sourceType string, id uint, accessScope *AccessScope) (map[string]any, error) {
	switch sourceType {
	case "camera":
		if _, err := s.ensureCameraAccessible(accessScope, id); err != nil {
			return nil, err
		}
	case "channel":
		if _, err := s.ensureChannelAccessible(accessScope, id); err != nil {
			return nil, err
		}
	}
	return map[string]any{
		"cameraId":  chooseID(sourceType == "camera", id),
		"channelId": chooseID(sourceType == "channel", id),
		"stopped":   true,
		"message":   "已停止实时预览",
	}, nil
}

func (s *PlatformService) GetLiveWebControlConfig(sourceType string, id uint, streamProfile string, accessScope *AccessScope) (map[string]any, error) {
	if sourceType == "camera" {
		camera, err := s.ensureCameraAccessible(accessScope, id)
		if err != nil {
			return nil, err
		}
		password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), camera.PasswordEncrypted)
		if err != nil {
			return nil, fmt.Errorf("resolve camera password: %w", err)
		}
		return map[string]any{
			"sourceType":    "camera",
			"cameraId":      camera.ID,
			"deviceName":    camera.Name,
			"host":          camera.IP,
			"port":          camera.HTTPPort,
			"protocol":      resolveDeviceProtocol(camera.HTTPPort),
			"username":      camera.Username,
			"password":      password,
			"channelNo":     1,
			"streamType":    mapStreamProfileToInt(streamProfile),
			"streamProfile": defaultString(streamProfile, "main"),
			"zeroChannel":   false,
			"useProxy":      true,
			"webSocketPort": nil,
			"rtspPort":      camera.RTSPPort,
			"supported":     true,
			"message":       "当前为海康客户端预览，默认通过前端同源代理转发，请确保已部署 WebSDK_noPlugin codebase 静态资源。",
		}, nil
	}
	channel, err := s.ensureChannelAccessible(accessScope, id)
	if err != nil {
		return nil, err
	}
	var recorder entity.RecorderDevice
	if err := s.db().First(&recorder, channel.RecorderID).Error; err != nil {
		return nil, err
	}
	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("resolve recorder password: %w", err)
	}
	return map[string]any{
		"sourceType":    "channel",
		"cameraId":      channel.CameraID,
		"channelId":     channel.ID,
		"deviceName":    recorder.Name,
		"host":          recorder.IP,
		"port":          recorder.HTTPPort,
		"protocol":      resolveDeviceProtocol(recorder.HTTPPort),
		"username":      recorder.Username,
		"password":      password,
		"channelNo":     channel.ChannelNo,
		"streamType":    mapStreamProfileToInt(streamProfile),
		"streamProfile": defaultString(streamProfile, "main"),
		"zeroChannel":   false,
		"useProxy":      true,
		"webSocketPort": nil,
		"rtspPort":      nil,
		"supported":     true,
		"message":       "当前为海康客户端预览，默认通过前端同源代理转发，请确保已部署 WebSDK_noPlugin codebase 静态资源。",
	}, nil
}

func (s *PlatformService) CreateSnapshot(cameraID, channelID *uint, accessScope *AccessScope) (map[string]any, error) {
	if cameraID != nil {
		if _, err := s.ensureCameraAccessible(accessScope, *cameraID); err != nil {
			return nil, err
		}
	}
	if channelID != nil {
		if _, err := s.ensureChannelAccessible(accessScope, *channelID); err != nil {
			return nil, err
		}
	}
	snapshotURL := buildSnapshotDataURL(cameraID, channelID)
	return map[string]any{"cameraId": cameraID, "channelId": channelID, "snapshotUrl": snapshotURL, "expiresIn": 300}, nil
}

func (s *PlatformService) SearchPlaybackSegments(channelID uint, startAt, endAt *time.Time, accessScope *AccessScope) ([]map[string]any, error) {
	channel, err := s.ensureChannelAccessible(accessScope, channelID)
	var recorder entity.RecorderDevice
	var camera entity.CameraDevice
	if err != nil {
		return nil, err
	}
	_ = s.db().First(&recorder, channel.RecorderID).Error
	if channel.CameraID != nil {
		_ = s.db().First(&camera, *channel.CameraID).Error
	}
	start := time.Now().Add(-30 * time.Minute)
	end := time.Now()
	if startAt != nil {
		start = *startAt
	}
	if endAt != nil {
		end = *endAt
	}
	if !end.After(start) {
		end = start.Add(2 * time.Minute)
	}
	return []map[string]any{{
		"startTime":    start.Format(time.RFC3339),
		"endTime":      end.Format(time.RFC3339),
		"channelId":    channel.ID,
		"channelName":  channel.Name,
		"recorderId":   recorder.ID,
		"recorderName": recorder.Name,
		"cameraId":     channel.CameraID,
		"cameraName":   nullableString(camera.Name),
		"recordType":   "alarm",
		"available":    channel.SupportPlayback,
	}}, nil
}

func (s *PlatformService) GetPlaybackURL(channelID uint, streamType, streamProfile, playbackMode string, accessScope *AccessScope) (map[string]any, error) {
	if _, err := s.ensureChannelAccessible(accessScope, channelID); err != nil {
		return nil, err
	}
	start := time.Now().Add(-30 * time.Minute)
	end := time.Now()
	return map[string]any{
		"streamType":        defaultString(streamType, "hik-sdk"),
		"streamProfile":     defaultString(streamProfile, "main"),
		"playbackMode":      defaultString(playbackMode, "hik"),
		"playUrl":           fmt.Sprintf("%s/mock/playback/channel/%d.m3u8", strings.TrimRight(s.cfg.BackendPublicBaseURL, "/"), channelID),
		"startTime":         start.Format(time.RFC3339),
		"endTime":           end.Format(time.RFC3339),
		"expiresIn":         300,
		"isMock":            true,
		"playableInBrowser": false,
		"diagnosticMessage": "当前返回模拟回放地址",
		"sourceRtsp":        fmt.Sprintf("rtsp://mock/playback/%d", channelID),
	}, nil
}

func (s *PlatformService) DownloadPlaybackFile(channelID uint, startTime, endTime time.Time, alarmNo string, accessScope *AccessScope) (string, string, error) {
	if !endTime.After(startTime) {
		return "", "", fmt.Errorf("invalid playback time range")
	}

	channel, err := s.ensureChannelAccessible(accessScope, channelID)
	if err != nil {
		return "", "", err
	}

	var recorder entity.RecorderDevice
	if err := s.db().First(&recorder, channel.RecorderID).Error; err != nil {
		return "", "", err
	}

	password, err := util.ResolveDeviceSecret(s.deviceSecretKey(), recorder.PasswordEncrypted)
	if err != nil {
		return "", "", fmt.Errorf("resolve recorder password: %w", err)
	}

	sdk, err := hikvision.NewSDK(s.cfg.HikvisionSDKPath)
	if err != nil {
		return "", "", fmt.Errorf("create hikvision sdk: %w", err)
	}
	defer func() {
		_ = sdk.Cleanup()
	}()

	userID, deviceInfo, err := sdk.LoginRecorder(recorder.IP, recorder.SDKPort, recorder.Username, password)
	if err != nil {
		return "", "", fmt.Errorf("login recorder: %w", err)
	}
	defer func() {
		_ = sdk.Logout(userID)
	}()

	fileName := buildPlaybackDownloadFilename(alarmNo, recorder.Name, channel.Name, startTime, endTime)
	outputDir := filepath.Join(s.cfg.MediaRootDir, "playback-downloads", time.Now().Format("20060102"))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create playback output dir: %w", err)
	}
	outputPath := filepath.Join(outputDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileName))
	if err := sdk.DownloadRecordByTime(userID, channel.ChannelNo, startTime, endTime, outputPath, deviceInfo); err != nil {
		return "", "", fmt.Errorf("download playback by time: %w", err)
	}
	return outputPath, fileName, nil
}

func (s *PlatformService) StopPlayback(channelID uint, accessScope *AccessScope) (map[string]any, error) {
	if _, err := s.ensureChannelAccessible(accessScope, channelID); err != nil {
		return nil, err
	}
	return map[string]any{"channelId": channelID, "stopped": true, "message": "已停止回放"}, nil
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

func (s *PlatformService) insertStatusLog(deviceType string, deviceID uint, oldStatus, newStatus, message string) error {
	return s.db().Create(&entity.DeviceStatusLog{
		DeviceType: deviceType,
		DeviceID:   deviceID,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Message:    message,
		CheckedAt:  time.Now(),
	}).Error
}

func (s *PlatformService) resolveDeviceName(deviceType string, deviceID uint) string {
	switch deviceType {
	case "camera":
		var item entity.CameraDevice
		if err := s.db().First(&item, deviceID).Error; err == nil {
			return item.Name
		}
	case "recorder":
		var item entity.RecorderDevice
		if err := s.db().First(&item, deviceID).Error; err == nil {
			return item.Name
		}
	default:
		var item entity.RecorderChannel
		if err := s.db().First(&item, deviceID).Error; err == nil {
			return item.Name
		}
	}
	return fmt.Sprintf("%s-%d", deviceType, deviceID)
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

func (s *PlatformService) canAccessStatusLog(item entity.DeviceStatusLog, accessScope *AccessScope) bool {
	if accessScope == nil || accessScope.All {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(item.DeviceType)) {
	case "camera":
		var device entity.CameraDevice
		if err := s.db().First(&device, item.DeviceID).Error; err != nil {
			return false
		}
		return accessScope.AllowsCamera(device.FactoryID, device.ZoneID, device.ID)
	case "recorder":
		var device entity.RecorderDevice
		if err := s.db().First(&device, item.DeviceID).Error; err != nil {
			return false
		}
		return accessScope.AllowsRecorder(device.FactoryID, device.ID)
	case "channel":
		var device entity.RecorderChannel
		if err := s.db().First(&device, item.DeviceID).Error; err != nil {
			return false
		}
		return accessScope.AllowsChannel(device.FactoryID, device.ZoneID, device.CameraID, device.RecorderID, device.ID)
	default:
		return false
	}
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
