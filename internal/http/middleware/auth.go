package middleware

import (
	"strings"

	"secmgmt_go/internal/domain/entity"
	"secmgmt_go/internal/http/response"
	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/service"
	"secmgmt_go/internal/util"

	"github.com/gin-gonic/gin"
)

const ContextUserIDKey = "currentUserID"
const ContextUsernameKey = "currentUsername"
const ContextUserRealNameKey = "currentUserRealName"
const ContextRoleCodesKey = "currentRoleCodes"
const ContextRoleNamesKey = "currentRoleNames"
const ContextPermissionCodesKey = "currentPermissionCodes"
const ContextAccessScopeKey = "currentAccessScope"

func Auth(secret string, repo *repository.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			response.Error(c, 401, "缺少认证信息")
			c.Abort()
			return
		}

		tokenValue := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		if tokenValue == header || tokenValue == "" {
			response.Error(c, 401, "认证头格式错误")
			c.Abort()
			return
		}

		claims, err := util.ParseToken(secret, tokenValue)
		if err != nil {
			response.Error(c, 401, "登录状态无效")
			c.Abort()
			return
		}

		roles, err := repo.ListRolesByUserID(claims.UserID)
		if err != nil {
			response.Error(c, 500, "加载角色信息失败")
			c.Abort()
			return
		}
		roleCodes := make([]string, 0, len(roles))
		roleNames := make([]string, 0, len(roles))
		for _, role := range roles {
			if code := strings.TrimSpace(role.RoleCode); code != "" {
				roleCodes = append(roleCodes, code)
			}
			if roleName := strings.TrimSpace(role.RoleName); roleName != "" {
				roleNames = append(roleNames, roleName)
			}
		}
		permissionCodes, err := repo.ListPermissionCodesByUserID(claims.UserID)
		if err != nil {
			response.Error(c, 500, "加载权限信息失败")
			c.Abort()
			return
		}
		user, err := repo.GetUserByID(claims.UserID)
		if err != nil {
			response.Error(c, 500, "加载用户信息失败")
			c.Abort()
			return
		}
		var deptScope *service.AccessScope
		if user != nil {
			var dept *entity.SysDept
			if user.DeptID != nil && *user.DeptID > 0 {
				dept, _ = repo.GetDeptByID(*user.DeptID)
			}
			deptScope = service.BuildAccessScope(roles, user, dept)
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Set(ContextUserRealNameKey, user.RealName)
		c.Set(ContextRoleCodesKey, roleCodes)
		c.Set(ContextRoleNamesKey, roleNames)
		c.Set(ContextPermissionCodesKey, permissionCodes)
		c.Set(ContextAccessScopeKey, deptScope)
		c.Next()
	}
}

func RequirePermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if permissionCode == "" {
			c.Next()
			return
		}
		if IsAdmin(c) {
			c.Next()
			return
		}
		if HasPermission(c, permissionCode) {
			c.Next()
			return
		}
		response.Error(c, 403, "当前账号无权执行此操作")
		c.Abort()
	}
}

func RequireAnyPermission(permissionCodes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if IsAdmin(c) {
			c.Next()
			return
		}
		for _, permissionCode := range permissionCodes {
			if permissionCode != "" && HasPermission(c, permissionCode) {
				c.Next()
				return
			}
		}
		response.Error(c, 403, "当前账号无权执行此操作")
		c.Abort()
	}
}

func CurrentUserID(c *gin.Context) uint {
	value, ok := c.Get(ContextUserIDKey)
	if !ok {
		return 0
	}
	userID, ok := value.(uint)
	if !ok {
		return 0
	}
	return userID
}

func CurrentUsername(c *gin.Context) string {
	value, ok := c.Get(ContextUsernameKey)
	if !ok {
		return ""
	}
	username, ok := value.(string)
	if !ok {
		return ""
	}
	return username
}

func CurrentUserRealName(c *gin.Context) string {
	value, ok := c.Get(ContextUserRealNameKey)
	if !ok {
		return ""
	}
	realName, ok := value.(string)
	if !ok {
		return ""
	}
	return realName
}

func CurrentRoleCodes(c *gin.Context) []string {
	value, ok := c.Get(ContextRoleCodesKey)
	if !ok {
		return nil
	}
	roleCodes, ok := value.([]string)
	if !ok {
		return nil
	}
	return roleCodes
}

func CurrentRoleNames(c *gin.Context) []string {
	value, ok := c.Get(ContextRoleNamesKey)
	if !ok {
		return nil
	}
	roleNames, ok := value.([]string)
	if !ok {
		return nil
	}
	return roleNames
}

func CurrentPermissionCodes(c *gin.Context) []string {
	value, ok := c.Get(ContextPermissionCodesKey)
	if !ok {
		return nil
	}
	permissionCodes, ok := value.([]string)
	if !ok {
		return nil
	}
	return permissionCodes
}

func CurrentAccessScope(c *gin.Context) *service.AccessScope {
	value, ok := c.Get(ContextAccessScopeKey)
	if !ok || value == nil {
		return nil
	}
	accessScope, ok := value.(*service.AccessScope)
	if !ok {
		return nil
	}
	return accessScope
}

func HasPermission(c *gin.Context, permissionCode string) bool {
	for _, code := range CurrentPermissionCodes(c) {
		if code == permissionCode {
			return true
		}
	}
	return false
}

func IsAdmin(c *gin.Context) bool {
	for _, roleCode := range CurrentRoleCodes(c) {
		if strings.EqualFold(strings.TrimSpace(roleCode), "admin") {
			return true
		}
	}
	return false
}
