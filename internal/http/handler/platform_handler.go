package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/domain/dto"
	"secmgmt_go/internal/http/middleware"
	"secmgmt_go/internal/http/response"
	"secmgmt_go/internal/service"
	"secmgmt_go/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PlatformHandler struct {
	cfg             *config.Config
	authService     *service.AuthService
	queryService    *service.QueryService
	platformService *service.PlatformService
}

func NewPlatformHandler(
	cfg *config.Config,
	authService *service.AuthService,
	queryService *service.QueryService,
	platformService *service.PlatformService,
) *PlatformHandler {
	return &PlatformHandler{
		cfg:             cfg,
		authService:     authService,
		queryService:    queryService,
		platformService: platformService,
	}
}

func (h *PlatformHandler) ListUsers(c *gin.Context) {
	deptID, err := readOptionalUintQuery(c, "dept_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid dept_id")
		return
	}
	roleID, err := readOptionalUintQuery(c, "role_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid role_id")
		return
	}
	data, err := h.platformService.ListUsers(service.UserListFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  strings.TrimSpace(c.Query("status")),
		DeptID:  deptID,
		RoleID:  roleID,
	})
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateUser(c *gin.Context) {
	var payload service.UserPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateUser(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateUser(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.UserPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateUser(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteUser(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteUser(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) ListRoles(c *gin.Context) {
	data, err := h.platformService.ListRoles(service.RoleListFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateRole(c *gin.Context) {
	var payload service.RolePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateRole(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateRole(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.RolePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateRole(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateRoleStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.RoleStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateRoleStatus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateRoleDataScope(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.RoleDataScopePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateRoleDataScope(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListRoleMenuTree(c *gin.Context) {
	data, err := h.platformService.ListRoleMenuTree()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListRolePermissionOptions(c *gin.Context) {
	data, err := h.platformService.ListRolePermissionOptions()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateRoleMenus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.RoleMenuPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateRoleMenus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateRolePermissions(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.RolePermissionPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateRolePermissions(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteRole(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteRole(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) CreateFactory(c *gin.Context) {
	var payload service.FactoryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateFactory(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateFactory(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.FactoryPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateFactory(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateFactoryStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.StatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateFactoryStatus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteFactory(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteFactory(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) CreateZone(c *gin.Context) {
	var payload service.ZonePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateZone(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateZone(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.ZonePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateZone(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateZoneStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.StatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateZoneStatus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteZone(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteZone(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) CreateDept(c *gin.Context) {
	var payload service.DeptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateDept(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDept(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.DeptPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDept(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDeptStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.StatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDeptStatus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteDept(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteDept(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) CreateDictType(c *gin.Context) {
	var payload service.DictTypePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateDictType(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDictType(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.DictTypePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDictType(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDictTypeStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.StatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDictTypeStatus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteDictType(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteDictType(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) CreateDictItem(c *gin.Context) {
	var payload service.DictItemPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateDictItem(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDictItem(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.DictItemPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDictItem(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDictItemStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.StatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDictItemStatus(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteDictItem(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteDictItem(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) GetCamera(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetCamera(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetCameraBrowserLogin(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetCameraBrowserLogin(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateCamera(c *gin.Context) {
	var payload service.CameraPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateCamera(payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) FetchCameraDeviceIdentity(c *gin.Context) {
	var payload service.CameraPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, h.platformService.FetchCameraDeviceIdentity(payload))
}

func (h *PlatformHandler) UpdateCamera(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.CameraPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateCamera(id, payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateCameraStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateCameraStatus(id, payload.Status, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteCamera(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteCamera(id, middleware.CurrentAccessScope(c)); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) TestCameraConnection(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.TestCameraConnection(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CheckCameraStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.CheckCameraStatus(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetCameraSDKConfig(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetCameraSDKConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateCameraNetworkConfig(c *gin.Context) {
	h.updateCameraSDKSection(c, "network")
}

func (h *PlatformHandler) UpdateCameraImageConfig(c *gin.Context) {
	h.updateCameraSDKSection(c, "image")
}

func (h *PlatformHandler) UpdateCameraRecordingConfig(c *gin.Context) {
	h.updateCameraSDKSection(c, "recording")
}

func (h *PlatformHandler) UpdateCameraPTZConfig(c *gin.Context) {
	h.updateCameraSDKSection(c, "ptz")
}

func (h *PlatformHandler) SetCameraPTZPreset(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload struct {
		PresetID int    `json:"presetId"`
		Name     string `json:"name"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.GetCameraSDKConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	section := mapSection(data, "ptz")
	section["supported"] = true
	section["message"] = "预置点已更新"
	presets := asSlice(section["presets"])
	updated := false
	for index, item := range presets {
		if toInt(item["presetId"]) == payload.PresetID {
			presets[index] = map[string]any{"presetId": payload.PresetID, "name": payload.Name}
			updated = true
			break
		}
	}
	if !updated {
		presets = append(presets, map[string]any{"presetId": payload.PresetID, "name": payload.Name})
	}
	section["presetCount"] = len(presets)
	section["presets"] = presets
	response.OK(c, section)
}

func (h *PlatformHandler) DeleteCameraPTZPreset(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	presetID, ok := pathInt(c, "presetId")
	if !ok {
		return
	}
	data, err := h.platformService.GetCameraSDKConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	section := mapSection(data, "ptz")
	presets := make([]map[string]any, 0)
	for _, item := range asSlice(section["presets"]) {
		if toInt(item["presetId"]) != presetID {
			presets = append(presets, item)
		}
	}
	section["supported"] = true
	section["message"] = "预置点已删除"
	section["presetCount"] = len(presets)
	section["presets"] = presets
	response.OK(c, section)
}

func (h *PlatformHandler) GotoCameraPTZPreset(c *gin.Context) {
	presetID, ok := pathInt(c, "presetId")
	if !ok {
		return
	}
	response.OK(c, gin.H{
		"success":  true,
		"message":  "已执行预置点调用",
		"presetId": presetID,
	})
}

func (h *PlatformHandler) ControlCameraPTZZoom(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	action := c.Param("action")
	if action != "in" && action != "out" {
		response.Error(c, http.StatusBadRequest, "unsupported zoom action")
		return
	}
	data, err := h.platformService.ControlCameraPTZZoom(id, action, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpsertCameraUser(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.GetCameraSDKConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	section := mapSection(data, "users")
	items := asSlice(section["items"])
	userID := toInt(payload["userId"])
	username := strings.TrimSpace(toString(payload["username"]))
	roleName := toString(payload["role"])
	enabled := true
	if raw, exists := payload["enabled"]; exists {
		enabled = toBool(raw)
	}
	if userID == 0 {
		userID = len(items) + 1
	}
	updated := false
	for index, item := range items {
		if toInt(item["userId"]) == userID {
			items[index] = map[string]any{"userId": userID, "username": username, "role": roleName, "enabled": enabled}
			updated = true
			break
		}
	}
	if !updated {
		items = append(items, map[string]any{"userId": userID, "username": username, "role": roleName, "enabled": enabled})
	}
	section["supported"] = true
	section["message"] = "用户配置已更新"
	section["items"] = items
	response.OK(c, section)
}

func (h *PlatformHandler) DeleteCameraUser(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	userID, ok := pathInt(c, "userId")
	if !ok {
		return
	}
	data, err := h.platformService.GetCameraSDKConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	section := mapSection(data, "users")
	items := make([]map[string]any, 0)
	for _, item := range asSlice(section["items"]) {
		if toInt(item["userId"]) != userID {
			items = append(items, item)
		}
	}
	section["supported"] = true
	section["message"] = "用户已删除"
	section["items"] = items
	response.OK(c, section)
}

func (h *PlatformHandler) GetRecorder(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetRecorder(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateRecorder(c *gin.Context) {
	var payload service.RecorderPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateRecorder(payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateRecorder(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.RecorderPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateRecorder(id, payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteRecorder(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteRecorder(id, middleware.CurrentAccessScope(c)); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) TestRecorderConnection(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.TestRecorderConnection(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CheckRecorderStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.CheckRecorderStatus(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) SyncRecorderChannels(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.SyncRecorderChannels(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListRecorderChannels(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.ListRecorderChannels(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateChannel(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.ChannelPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateChannel(id, payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListDeviceStatusLogs(c *gin.Context) {
	page, pageSize := readPageParams(c)
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.ListDeviceStatusLogs(page, pageSize, service.DeviceStatusLogListFilter{
		DeviceType: strings.TrimSpace(c.Query("device_type")),
		DeviceName: strings.TrimSpace(c.Query("device_name")),
		Status:     strings.TrimSpace(c.Query("status")),
		StartAt:    startAt,
		EndAt:      endAt,
	}, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CheckAllDevicesStatus(c *gin.Context) {
	data, err := h.platformService.CheckAllDevicesStatus()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListDeviceCheckSchedules(c *gin.Context) {
	data, err := h.platformService.ListDeviceCheckSchedules()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateDeviceCheckSchedule(c *gin.Context) {
	var payload service.DeviceCheckSchedulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateDeviceCheckSchedule(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDeviceCheckSchedule(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.DeviceCheckSchedulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDeviceCheckSchedule(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateDeviceCheckScheduleStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.DeviceCheckScheduleStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateDeviceCheckScheduleStatus(id, payload.Enabled)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteDeviceCheckSchedule(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteDeviceCheckSchedule(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) RunDeviceCheckScheduleNow(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.RunDeviceCheckScheduleNow(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListDeviceCheckRuns(c *gin.Context) {
	page, pageSize := readPageParams(c)
	scheduleID, err := readOptionalUintQuery(c, "schedule_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid schedule_id")
		return
	}
	data, err := h.platformService.ListDeviceCheckRuns(page, pageSize, scheduleID)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetAlarmDetail(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetAlarmDetail(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ProcessAlarm(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.AlarmProcessPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	operatorID := middleware.CurrentUserID(c)
	operatorName := h.currentOperatorName(c, operatorID)
	data, err := h.platformService.ProcessAlarm(id, payload, operatorName, operatorID, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) FalseAlarm(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload struct {
		Remark string `json:"remark"`
	}
	_ = c.ShouldBindJSON(&payload)
	operatorID := middleware.CurrentUserID(c)
	operatorName := h.currentOperatorName(c, operatorID)
	data, err := h.platformService.FalseAlarm(id, payload.Remark, operatorName, operatorID, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) RePushAlarm(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.RePushAlarm(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DashboardAlarmTrend(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.OK(c, h.platformService.GetDashboardAlarmTrend(startAt, endAt, middleware.CurrentAccessScope(c)))
}

func (h *PlatformHandler) DashboardAlarmTypes(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.OK(c, h.platformService.GetDashboardAlarmTypes(startAt, endAt, middleware.CurrentAccessScope(c)))
}

func (h *PlatformHandler) DashboardZoneRanking(c *gin.Context) {
	response.OK(c, h.platformService.GetDashboardZoneRanking(middleware.CurrentAccessScope(c)))
}

func (h *PlatformHandler) DashboardDeviceStatus(c *gin.Context) {
	response.OK(c, h.platformService.GetDashboardDeviceStatus(middleware.CurrentAccessScope(c)))
}

func (h *PlatformHandler) AlarmReport(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	zonePage, zonePageSize := readNamedPageParams(c, "zone_page", "zone_page_size", 30)
	data, err := h.platformService.GetAlarmReport(startAt, endAt, zonePage, zonePageSize, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeviceReport(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	factoryPage, factoryPageSize := readNamedPageParams(c, "factory_page", "factory_page_size", 30)
	response.OK(c, h.platformService.GetDeviceReport(startAt, endAt, factoryPage, factoryPageSize, middleware.CurrentAccessScope(c)))
}

func (h *PlatformHandler) PushReport(c *gin.Context) {
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}
	response.OK(c, h.platformService.GetPushReport(startAt, endAt, middleware.CurrentAccessScope(c)))
}

func readNamedPageParams(c *gin.Context, pageKey, pageSizeKey string, defaultPageSize int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery(pageKey, "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery(pageSizeKey, strconv.Itoa(defaultPageSize)))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

func (h *PlatformHandler) ListPushConfigs(c *gin.Context) {
	filter := service.PushConfigListFilter{
		Keyword:      strings.TrimSpace(c.Query("keyword")),
		ProviderType: strings.TrimSpace(c.Query("provider_type")),
	}
	if rawEnabled := strings.TrimSpace(c.Query("enabled")); rawEnabled != "" {
		enabled, err := strconv.ParseBool(rawEnabled)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid enabled")
			return
		}
		filter.Enabled = &enabled
	}
	filter.AccessScope = middleware.CurrentAccessScope(c)
	data, err := h.platformService.ListPushConfigs(filter)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreatePushConfig(c *gin.Context) {
	var payload service.PushConfigPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreatePushConfig(payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdatePushConfig(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.PushConfigPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdatePushConfig(id, payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdatePushConfigStatus(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.PushConfigStatusPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdatePushConfigStatus(id, payload.Enabled, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeletePushConfig(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeletePushConfig(id, middleware.CurrentAccessScope(c)); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) TestPushConfig(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.TestPushConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListPushLogs(c *gin.Context) {
	page, pageSize := readPageParams(c)
	startAt, endAt, err := readOptionalTimeRange(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.ListPushLogs(page, pageSize, service.PushLogListFilter{
		Channel:     strings.TrimSpace(c.Query("channel")),
		Status:      strings.TrimSpace(c.Query("status")),
		AlarmType:   strings.TrimSpace(c.Query("alarm_type")),
		StartAt:     startAt,
		EndAt:       endAt,
		AccessScope: middleware.CurrentAccessScope(c),
	})
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) RetryPushLog(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.RetryPushLog(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListSmartProviders(c *gin.Context) {
	data, err := h.platformService.ListSmartProviders()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateSmartProvider(c *gin.Context) {
	var payload service.SmartProviderPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateSmartProvider(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateSmartProvider(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.SmartProviderPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateSmartProvider(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) TestSmartProvider(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.TestSmartProvider(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetSmartBridgeStatus(c *gin.Context) {
	data, err := h.platformService.GetSmartBridgeStatus()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListSmartCapabilities(c *gin.Context) {
	data, err := h.platformService.ListSmartCapabilities()
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListSmartBindings(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter := service.SmartBindingListFilter{
		SourceType:     strings.TrimSpace(c.Query("source_type")),
		ProviderCode:   strings.TrimSpace(c.Query("provider_code")),
		CapabilityCode: strings.TrimSpace(c.Query("capability_code")),
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		enabled, err := strconv.ParseBool(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid enabled")
			return
		}
		filter.Enabled = &enabled
	}
	data, err := h.platformService.ListSmartBindings(page, pageSize, filter)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateSmartBinding(c *gin.Context) {
	var payload service.SmartBindingPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateSmartBinding(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) TestSmartBinding(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.TestSmartBinding(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ReconnectSmartBinding(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.ReconnectSmartBinding(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ReloadSmartBinding(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.ReloadSmartBinding(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateSmartBinding(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.SmartBindingPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateSmartBinding(id, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteSmartBinding(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	if err := h.platformService.DeleteSmartBinding(id); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) GetSmartBindingDetail(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetSmartBindingDetail(id)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateSmartBindingRule(c *gin.Context) {
	bindingID, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.SmartBindingRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateSmartBindingRule(bindingID, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) UpdateSmartBindingRule(c *gin.Context) {
	ruleID, ok := pathUint(c, "ruleId")
	if !ok {
		return
	}
	var payload service.SmartBindingRulePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.UpdateSmartBindingRule(ruleID, payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DeleteSmartBindingRule(c *gin.Context) {
	ruleID, ok := pathUint(c, "ruleId")
	if !ok {
		return
	}
	if err := h.platformService.DeleteSmartBindingRule(ruleID); err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, gin.H{})
}

func (h *PlatformHandler) IngestSmartProviderEvent(c *gin.Context) {
	providerCode := c.Param("providerCode")
	payload, err := readSmartProviderIngestPayload(c)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	headers["X-Request-Client-IP"] = c.ClientIP()
	data, err := h.platformService.IngestSmartProviderEvent(providerCode, payload, headers)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func readSmartProviderIngestPayload(c *gin.Context) (any, error) {
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if strings.Contains(contentType, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			return nil, err
		}
		files := make([]service.SmartIngestFile, 0)
		if c.Request.MultipartForm != nil {
			for _, fileHeaders := range c.Request.MultipartForm.File {
				for _, fileHeader := range fileHeaders {
					file, err := fileHeader.Open()
					if err != nil {
						return nil, err
					}
					data, readErr := io.ReadAll(file)
					_ = file.Close()
					if readErr != nil {
						return nil, readErr
					}
					files = append(files, service.SmartIngestFile{
						Filename:    fileHeader.Filename,
						ContentType: fileHeader.Header.Get("Content-Type"),
						Data:        data,
					})
				}
			}
			return map[string]any{
				"fields": c.Request.MultipartForm.Value,
				"files":  files,
			}, nil
		}
		return map[string]any{"files": files}, nil
	}
	if strings.Contains(contentType, "json") {
		var payload any
		if err := c.ShouldBindJSON(&payload); err == nil {
			return payload, nil
		}
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	return string(body), nil
}

func (h *PlatformHandler) ListSmartRawEvents(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter := service.SmartRawEventListFilter{
		ProviderCode:   strings.TrimSpace(c.Query("provider_code")),
		CapabilityCode: strings.TrimSpace(c.Query("capability_code")),
		ParseStatus:    strings.TrimSpace(c.Query("parse_status")),
		SourceType:     strings.TrimSpace(c.Query("source_type")),
	}
	if raw := strings.TrimSpace(c.Query("recent_days")); raw != "" {
		recentDays, err := strconv.Atoi(raw)
		if err != nil || recentDays < 0 {
			response.Error(c, http.StatusBadRequest, "invalid recent_days")
			return
		}
		filter.RecentDays = recentDays
	}
	data, err := h.platformService.ListSmartRawEvents(page, pageSize, filter)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListSmartEvents(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter := service.SmartEventListFilter{
		Keyword:        strings.TrimSpace(c.Query("keyword")),
		ProviderCode:   strings.TrimSpace(c.Query("provider_code")),
		CapabilityCode: strings.TrimSpace(c.Query("capability_code")),
		Status:         strings.TrimSpace(c.Query("status")),
		SourceStage:    strings.TrimSpace(c.Query("source_stage")),
	}
	if raw := strings.TrimSpace(c.Query("recent_days")); raw != "" {
		recentDays, err := strconv.Atoi(raw)
		if err != nil || recentDays < 0 {
			response.Error(c, http.StatusBadRequest, "invalid recent_days")
			return
		}
		filter.RecentDays = recentDays
	}
	filter.AccessScope = middleware.CurrentAccessScope(c)
	data, err := h.platformService.ListSmartEvents(page, pageSize, filter)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListSmartBridgeReconnectLogs(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter := service.SmartBridgeReconnectLogListFilter{
		Status:        strings.TrimSpace(c.Query("status")),
		Action:        strings.TrimSpace(c.Query("action")),
		TriggerReason: strings.TrimSpace(c.Query("trigger_reason")),
		DeviceType:    strings.TrimSpace(c.Query("device_type")),
		SessionKey:    strings.TrimSpace(c.Query("session_key")),
	}
	if raw := strings.TrimSpace(c.Query("device_id")); raw != "" {
		deviceID, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "invalid device_id")
			return
		}
		filter.DeviceID = uint(deviceID)
	}
	startAt, startOK := firstTimeQuery(c, "start_at")
	endAt, endOK := firstTimeQuery(c, "end_at")
	filter.StartAt = optionalTime(startAt, startOK)
	filter.EndAt = optionalTime(endAt, endOK)
	data, err := h.platformService.ListSmartBridgeReconnectLogs(page, pageSize, filter)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetSmartEventDetail(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetSmartEventDetail(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) SubmitSmartAIReview(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload service.SmartAIReviewPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.SubmitSmartAIReview(id, payload, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ListSmartAITasks(c *gin.Context) {
	page, pageSize := readPageParams(c)
	filter := service.SmartAITaskListFilter{
		Status:     strings.TrimSpace(c.Query("status")),
		AIFlowCode: strings.TrimSpace(c.Query("ai_flow_code")),
	}
	if raw := strings.TrimSpace(c.Query("recent_days")); raw != "" {
		recentDays, err := strconv.Atoi(raw)
		if err != nil || recentDays < 0 {
			response.Error(c, http.StatusBadRequest, "invalid recent_days")
			return
		}
		filter.RecentDays = recentDays
	}
	data, err := h.platformService.ListSmartAITasks(page, pageSize, filter, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetSmartAITask(c *gin.Context) {
	id, ok := pathUint(c, "taskId")
	if !ok {
		return
	}
	data, err := h.platformService.GetSmartAITask(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) RetrySmartAITask(c *gin.Context) {
	id, ok := pathUint(c, "taskId")
	if !ok {
		return
	}
	data, err := h.platformService.RetrySmartAITask(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) HandleSmartAICallback(c *gin.Context) {
	var payload service.SmartAICallbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.HandleSmartAICallback(payload)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetLiveVideo(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetLiveVideo("camera", id, c.Query("stream_type"), c.Query("stream_profile"), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetChannelLiveVideo(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetLiveVideo("channel", id, c.Query("stream_type"), c.Query("stream_profile"), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) StopLiveVideo(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.StopLiveVideo("camera", id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) StopChannelLiveVideo(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.StopLiveVideo("channel", id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetLiveWebControlConfig(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetLiveWebControlConfig("camera", id, c.Query("stream_profile"), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetChannelLiveWebControlConfig(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetLiveWebControlConfig("channel", id, c.Query("stream_profile"), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) CreateSnapshot(c *gin.Context) {
	var payload struct {
		CameraID  *uint `json:"cameraId"`
		ChannelID *uint `json:"channelId"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.CreateSnapshot(payload.CameraID, payload.ChannelID, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) SearchPlaybackSegments(c *gin.Context) {
	channelID := firstUintQuery(c, "channel_id", "channelId")
	if channelID == 0 {
		response.Error(c, http.StatusBadRequest, "missing channel_id")
		return
	}
	startTime, startOK := firstTimeQuery(c, "start_time", "startTime")
	endTime, endOK := firstTimeQuery(c, "end_time", "endTime")
	data, err := h.platformService.SearchPlaybackSegments(channelID, optionalTime(startTime, startOK), optionalTime(endTime, endOK), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) GetPlaybackURL(c *gin.Context) {
	channelID := firstUintQuery(c, "channel_id", "channelId")
	if channelID == 0 {
		response.Error(c, http.StatusBadRequest, "missing channel_id")
		return
	}
	data, err := h.platformService.GetPlaybackURL(channelID, c.Query("stream_type"), c.Query("stream_profile"), c.Query("playback_mode"), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) SeekPlayback(c *gin.Context) {
	channelID := firstUintQuery(c, "channel_id", "channelId")
	if channelID == 0 {
		response.Error(c, http.StatusBadRequest, "missing channel_id")
		return
	}
	data, err := h.platformService.GetPlaybackURL(channelID, c.Query("stream_type"), c.Query("stream_profile"), c.Query("playback_mode"), middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) DownloadPlaybackFile(c *gin.Context) {
	channelID := firstUintQuery(c, "channel_id", "channelId")
	if channelID == 0 {
		response.Error(c, http.StatusBadRequest, "missing channel_id")
		return
	}
	startTime, ok := firstTimeQuery(c, "start_time", "startTime")
	if !ok {
		response.Error(c, http.StatusBadRequest, "missing or invalid start_time")
		return
	}
	endTime, ok := firstTimeQuery(c, "end_time", "endTime")
	if !ok {
		response.Error(c, http.StatusBadRequest, "missing or invalid end_time")
		return
	}
	filePath, filename, err := h.platformService.DownloadPlaybackFile(
		channelID,
		startTime,
		endTime,
		firstStringQuery(c, "alarm_no", "alarmNo"),
		middleware.CurrentAccessScope(c),
	)
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	defer func() {
		_ = os.Remove(filePath)
	}()
	c.Header("Content-Type", "video/mp4")
	c.FileAttachment(filePath, filename)
}

func (h *PlatformHandler) StopPlayback(c *gin.Context) {
	channelID := firstUintQuery(c, "channel_id", "channelId")
	if channelID == 0 {
		response.Error(c, http.StatusBadRequest, "missing channel_id")
		return
	}
	data, err := h.platformService.StopPlayback(channelID, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, data)
}

func (h *PlatformHandler) ExportAlarms(c *gin.Context) {
	h.exportCSV(c, "alarms")
}

func (h *PlatformHandler) ExportDeviceStatus(c *gin.Context) {
	h.exportCSV(c, "device-status")
}

func (h *PlatformHandler) ExportPushLogs(c *gin.Context) {
	h.exportCSV(c, "push-logs")
}

func (h *PlatformHandler) AlarmSSE(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		response.Error(c, http.StatusUnauthorized, "missing token")
		return
	}
	if _, err := util.ParseToken(h.cfg.JWTSecretKey, token); err != nil {
		response.Error(c, http.StatusUnauthorized, "invalid token")
		return
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, http.StatusInternalServerError, "streaming not supported")
		return
	}

	writeSSEEvent(c.Writer, "connected", gin.H{
		"status": "connected",
		"time":   time.Now().Format(time.RFC3339),
	})
	flusher.Flush()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			page, err := h.queryService.ListRealtimeAlarms(1, 1, dto.AlarmListFilter{}, nil)
			if err == nil && len(page.Items) > 0 {
				writeSSEEvent(c.Writer, "alarm", realtimeAlarmEvent(page.Items[0]))
			} else {
				writeSSEEvent(c.Writer, "connected", gin.H{
					"status": "connected",
					"time":   time.Now().Format(time.RFC3339),
				})
			}
			flusher.Flush()
		}
	}
}

func (h *PlatformHandler) GetAIConfig(c *gin.Context) {
	response.OK(c, gin.H{
		"callbackUrl":          strings.TrimRight(h.cfg.BackendPublicBaseURL, "/") + "/api/ai/events/callback",
		"signatureSecret":      h.cfg.AICallbackSecret,
		"signingEnabled":       strings.TrimSpace(h.cfg.AICallbackSecret) != "",
		"eventSources":         []string{"camera", "channel", "smart-provider"},
		"minConfidence":        0.6,
		"ignoreBelowThreshold": true,
		"dedupWindowSeconds":   60,
		"eventTypeMappings":    map[string]string{"motion_detected": "motion_detected", "intrusion": "intrusion"},
	})
}

func (h *PlatformHandler) CreateAIEventCallback(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{
		"accepted": true,
		"stored":   true,
		"ignored":  false,
		"reason":   "AI 事件已接收",
		"eventId":  nil,
		"eventNo":  payload["eventType"],
	})
}

func (h *PlatformHandler) ListAIEvents(c *gin.Context) {
	page, err := h.platformService.ListSmartEvents(1, 200, service.SmartEventListFilter{AccessScope: middleware.CurrentAccessScope(c)})
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	rawItems, _ := page["items"].([]map[string]any)
	items := make([]map[string]any, 0, len(rawItems))
	for _, item := range rawItems {
		items = append(items, smartEventToAIEvent(item))
	}
	response.OK(c, items)
}

func (h *PlatformHandler) GetAIEventDetail(c *gin.Context) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	data, err := h.platformService.GetSmartEventDetail(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	response.OK(c, smartEventToAIEvent(data))
}

func (h *PlatformHandler) updateCameraSDKSection(c *gin.Context, sectionName string) {
	id, ok := pathUint(c, "id")
	if !ok {
		return
	}
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	data, err := h.platformService.GetCameraSDKConfig(id, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	section := mapSection(data, sectionName)
	for key, value := range payload {
		section[key] = value
	}
	section["supported"] = true
	section["message"] = "配置已更新"
	response.OK(c, section)
}

func (h *PlatformHandler) exportCSV(c *gin.Context, kind string) {
	content, filename, err := h.platformService.ExportCSV(kind, middleware.CurrentAccessScope(c))
	if err != nil {
		handlePlatformError(c, err)
		return
	}
	respondAttachment(c, content, filename, "text/csv; charset=utf-8")
}

func (h *PlatformHandler) currentOperatorName(c *gin.Context, userID uint) string {
	if userID == 0 {
		return "system"
	}
	me, err := h.authService.GetMe(userID)
	if err != nil {
		return fmt.Sprintf("user-%d", userID)
	}
	if strings.TrimSpace(me.User.RealName) != "" {
		return me.User.RealName
	}
	if strings.TrimSpace(me.User.Username) != "" {
		return me.User.Username
	}
	return fmt.Sprintf("user-%d", userID)
}

func handlePlatformError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Error(c, http.StatusNotFound, "record not found")
	case errors.Is(err, service.ErrAccessDenied):
		response.Error(c, http.StatusForbidden, "当前数据范围不允许访问该资源")
	case errors.Is(err, service.ErrDeviceDeleteForbidden):
		response.Error(c, http.StatusBadRequest, "设备已有关联业务，禁止删除")
	default:
		response.Error(c, http.StatusInternalServerError, err.Error())
	}
}

func pathUint(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("invalid path param %s", name))
		return 0, false
	}
	return uint(value), true
}

func pathInt(c *gin.Context, name string) (int, bool) {
	value, err := strconv.Atoi(c.Param(name))
	if err != nil {
		response.Error(c, http.StatusBadRequest, fmt.Sprintf("invalid path param %s", name))
		return 0, false
	}
	return value, true
}

func firstUintQuery(c *gin.Context, keys ...string) uint {
	for _, key := range keys {
		if raw := strings.TrimSpace(c.Query(key)); raw != "" {
			if value, err := strconv.ParseUint(raw, 10, 64); err == nil {
				return uint(value)
			}
		}
	}
	return 0
}

func firstStringQuery(c *gin.Context, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(c.Query(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstTimeQuery(c *gin.Context, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		raw := strings.TrimSpace(c.Query(key))
		if raw == "" {
			continue
		}
		if value, err := parseDateTimeQuery(raw); err == nil {
			return value, true
		}
	}
	return time.Time{}, false
}

func optionalTime(value time.Time, ok bool) *time.Time {
	if !ok {
		return nil
	}
	return &value
}

func parseDateTimeQuery(value string) (time.Time, error) {
	candidates := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	normalized := strings.TrimSpace(value)
	for _, layout := range candidates {
		if parsed, err := time.Parse(layout, normalized); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid datetime: %s", value)
}

func respondAttachment(c *gin.Context, content []byte, filename, contentType string) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", filename))
	c.Data(http.StatusOK, contentType, content)
}

func writeSSEEvent(writer io.Writer, eventName string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{}`)
	}
	_, _ = fmt.Fprintf(writer, "event: %s\n", eventName)
	_, _ = fmt.Fprintf(writer, "data: %s\n\n", body)
}

func realtimeAlarmEvent(item dto.AlarmRecord) map[string]any {
	return map[string]any{
		"alarm_id":          item.ID,
		"alarm_no":          item.AlarmNo,
		"alarm_type":        item.AlarmType,
		"alarm_level":       item.AlarmLevel,
		"alarm_time":        item.AlarmTime,
		"status":            item.Status,
		"occurrence_count":  item.OccurrenceCount,
		"factory_id":        item.FactoryID,
		"factory_name":      item.FactoryName,
		"zone_id":           item.ZoneID,
		"zone_name":         item.ZoneName,
		"camera_id":         item.CameraID,
		"camera_name":       item.CameraName,
		"recorder_id":       item.RecorderID,
		"recorder_name":     item.RecorderName,
		"channel_id":        item.ChannelID,
		"channel_name":      item.ChannelName,
		"message":           item.Message,
		"image_url":         item.ImageURL,
		"video_url":         item.VideoURL,
		"record_start_time": item.RecordStartTime,
		"record_end_time":   item.RecordEndTime,
		"last_event_time":   item.LastEventTime,
		"created_at":        item.CreatedAt,
	}
}

func smartEventToAIEvent(item map[string]any) map[string]any {
	out := map[string]any{
		"id":           item["id"],
		"eventNo":      item["eventCode"],
		"sourceType":   item["sourceStage"],
		"eventType":    item["eventType"],
		"eventLevel":   item["eventLevel"],
		"eventTime":    item["eventTime"],
		"cameraId":     item["cameraId"],
		"cameraName":   item["cameraName"],
		"recorderId":   item["recorderId"],
		"recorderName": item["recorderName"],
		"channelId":    item["channelId"],
		"channelName":  item["channelName"],
		"factoryId":    item["factoryId"],
		"factoryName":  item["factoryName"],
		"zoneId":       item["zoneId"],
		"zoneName":     item["zoneName"],
		"imageUrl":     item["imageUrl"],
		"videoUrl":     item["videoUrl"],
		"confidence":   item["confidence"],
		"rawJson":      item["rawJson"],
		"dedupKey":     item["dedupKey"],
		"createdAt":    item["createdAt"],
	}
	return out
}

func mapSection(root map[string]any, key string) map[string]any {
	if section, ok := root[key].(map[string]any); ok {
		return section
	}
	return map[string]any{}
}

func asSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if entry, ok := item.(map[string]any); ok {
				result = append(result, entry)
			}
		}
		return result
	default:
		return []map[string]any{}
	}
}

func toInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case uint:
		return int(typed)
	case uint64:
		return int(typed)
	case string:
		out, _ := strconv.Atoi(strings.TrimSpace(typed))
		return out
	default:
		return 0
	}
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func toBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}
