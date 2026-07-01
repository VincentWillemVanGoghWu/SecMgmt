package service

import (
	"fmt"
	"strings"
	"time"

	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/domain/entity"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

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
