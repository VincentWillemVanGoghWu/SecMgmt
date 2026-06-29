package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/service"

	"github.com/gin-gonic/gin"
)

type operationLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *operationLogWriter) Write(data []byte) (int, error) {
	if w.body != nil {
		_, _ = w.body.Write(data)
	}
	return w.ResponseWriter.Write(data)
}

func OperationLogger(logService *service.OperationLogService, repo *repository.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shouldSkipOperationLogPath(c.Request.URL.Path) || c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		requestBody := readRequestBody(c)
		writer := &operationLogWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = writer

		startedAt := time.Now()
		c.Next()

		actor := resolveOperationActor(c, repo, c.Request.URL.Path, requestBody, c.Writer.Status())
		meta := buildOperationLogMeta(c, requestBody, writer.body.String(), startedAt, actor)
		if err := logService.Record(meta); err != nil {
			return
		}
	}
}

func shouldSkipOperationLogPath(path string) bool {
	return path == "/healthz" ||
		strings.HasPrefix(path, "/api/operation-logs") ||
		strings.HasPrefix(path, "/api/sse/")
}

func readRequestBody(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	contentType := strings.ToLower(strings.TrimSpace(c.GetHeader("Content-Type")))
	if contentType != "" &&
		!strings.Contains(contentType, "application/json") &&
		!strings.Contains(contentType, "application/x-www-form-urlencoded") &&
		!strings.Contains(contentType, "multipart/form-data") {
		return ""
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return string(body)
}

func resolveOperationActor(c *gin.Context, repo *repository.Repository, path string, requestBody string, status int) service.OperationLogCreateInput {
	input := service.OperationLogCreateInput{
		OperatorID:       nil,
		OperatorUsername: CurrentUsername(c),
		OperatorRealName: CurrentUserRealName(c),
		RoleCodes:        CurrentRoleCodes(c),
		RoleNames:        CurrentRoleNames(c),
	}
	if strings.EqualFold(path, "/api/auth/login") {
		username := extractLoginUsername(requestBody)
		input.OperatorUsername = username
		if status < http.StatusBadRequest && strings.TrimSpace(username) != "" {
			if user, err := repo.FindUserByUsername(username); err == nil {
				input.OperatorID = &user.ID
				input.OperatorRealName = user.RealName
				if roles, err := repo.ListRolesByUserID(user.ID); err == nil {
					roleCodes := make([]string, 0, len(roles))
					roleNames := make([]string, 0, len(roles))
					for _, role := range roles {
						if strings.TrimSpace(role.RoleCode) != "" {
							roleCodes = append(roleCodes, role.RoleCode)
						}
						if strings.TrimSpace(role.RoleName) != "" {
							roleNames = append(roleNames, role.RoleName)
						}
					}
					input.RoleCodes = roleCodes
					input.RoleNames = roleNames
				}
			}
		}
	}
	return input
}

func buildOperationLogMeta(
	c *gin.Context,
	requestBody string,
	responseBody string,
	startedAt time.Time,
	actor service.OperationLogCreateInput,
) service.OperationLogCreateInput {
	durationMs := time.Since(startedAt).Milliseconds()
	routeMeta := resolveRouteMeta(c.Request.URL.Path)
	actionCode := firstHeader(c, "X-Action-Code")
	actionName := firstHeader(c, "X-Action-Name")
	operationType := firstHeader(c, "X-Operation-Type")
	objectType := firstHeader(c, "X-Object-Type")
	objectName := firstHeader(c, "X-Object-Name")

	resultStatus := "success"
	errorStack := ""
	if c.Writer.Status() >= http.StatusBadRequest {
		resultStatus = "failed"
		errorStack = extractResponseMessage(responseBody)
	}
	if operationType == "" {
		operationType = inferOperationType(c.Request.Method, c.Request.URL.Path, actionName)
	}
	if actionName == "" {
		actionName = operationType
	}
	if actionCode == "" {
		actionCode = inferActionCode(c.Request.Method, c.FullPath(), actionName)
	}
	if objectType == "" {
		objectType = routeMeta.ObjectType
	}
	if objectName == "" {
		objectName = firstHeader(c, "X-Object-Name")
	}

	return service.OperationLogCreateInput{
		TraceID:          buildTraceID(),
		Source:           firstNonEmpty(firstHeader(c, "X-Track-Source"), "api"),
		OperatorID:       actor.OperatorID,
		OperatorUsername: actor.OperatorUsername,
		OperatorRealName: actor.OperatorRealName,
		RoleCodes:        actor.RoleCodes,
		RoleNames:        actor.RoleNames,
		ClientIP:         c.ClientIP(),
		IPLocation:       resolveIPLocation(c.ClientIP()),
		UserAgent:        c.Request.UserAgent(),
		OSName:           resolveOSName(c.Request.UserAgent()),
		MenuCode:         firstNonEmpty(firstHeader(c, "X-Menu-Code"), routeMeta.MenuCode),
		MenuName:         firstNonEmpty(firstHeader(c, "X-Menu-Name"), routeMeta.MenuName),
		RoutePath:        firstNonEmpty(firstHeader(c, "X-Page-Route"), routeMeta.RoutePath, c.Request.URL.Path),
		PageTitle:        firstNonEmpty(firstHeader(c, "X-Page-Title"), routeMeta.PageTitle),
		PageComponent:    firstNonEmpty(firstHeader(c, "X-Page-Component"), routeMeta.PageComponent),
		ActionCode:       actionCode,
		ActionName:       actionName,
		OperationType:    operationType,
		ObjectType:       objectType,
		ObjectID:         firstHeader(c, "X-Object-Id"),
		ObjectName:       objectName,
		ObjectLocation:   firstHeader(c, "X-Object-Location"),
		RequestMethod:    c.Request.Method,
		RequestPath:      c.Request.URL.Path,
		RequestQuery:     c.Request.URL.RawQuery,
		RequestParams:    requestBody,
		ErrorStack:       errorStack,
		ResultStatus:     resultStatus,
		ResponseStatus:   c.Writer.Status(),
		DurationMs:       durationMs,
		OperationTime:    startedAt,
	}
}

type routeMeta struct {
	MenuCode      string
	MenuName      string
	RoutePath     string
	PageTitle     string
	PageComponent string
	ObjectType    string
}

func resolveRouteMeta(path string) routeMeta {
	switch {
	case strings.Contains(path, "/auth/login"):
		return routeMeta{MenuCode: "login", MenuName: "登录", RoutePath: "/login", PageTitle: "登录", PageComponent: "LoginView", ObjectType: "用户"}
	case strings.Contains(path, "/auth/logout"):
		return routeMeta{MenuCode: "login", MenuName: "退出登录", RoutePath: "/login", PageTitle: "退出登录", PageComponent: "AppHeader", ObjectType: "用户"}
	case strings.Contains(path, "/users"):
		return routeMeta{MenuCode: "system-users", MenuName: "系统管理 / 用户管理", RoutePath: "/system/users", PageTitle: "系统管理 / 用户管理", PageComponent: "UserManagementView", ObjectType: "用户"}
	case strings.Contains(path, "/roles"):
		return routeMeta{MenuCode: "system-roles", MenuName: "系统管理 / 角色权限", RoutePath: "/system/roles", PageTitle: "系统管理 / 角色权限", PageComponent: "RoleManagementView", ObjectType: "角色"}
	case strings.Contains(path, "/cameras"):
		return routeMeta{MenuCode: "device-cameras", MenuName: "设备管理 / 摄像机管理", RoutePath: "/device/cameras", PageTitle: "设备管理 / 摄像机管理", PageComponent: "CameraManagementView", ObjectType: "摄像头设备"}
	case strings.Contains(path, "/recorders"):
		return routeMeta{MenuCode: "device-recorders", MenuName: "设备管理 / 录像机管理", RoutePath: "/device/recorders", PageTitle: "设备管理 / 录像机管理", PageComponent: "RecorderManagementView", ObjectType: "录像机设备"}
	case strings.Contains(path, "/channels"):
		return routeMeta{MenuCode: "device-channels", MenuName: "设备管理 / 通道管理", RoutePath: "/device/channels", PageTitle: "设备管理 / 通道管理", PageComponent: "ChannelManagementView", ObjectType: "监控通道"}
	case strings.Contains(path, "/alarms"):
		return routeMeta{MenuCode: "safety-alarm-list", MenuName: "安全日志 / 告警查询", RoutePath: "/safety/alarm-list", PageTitle: "安全日志 / 告警查询", PageComponent: "AlarmQueryView", ObjectType: "告警记录"}
	case strings.Contains(path, "/video/live"):
		return routeMeta{MenuCode: "monitor-preview", MenuName: "监控管理 / 监控预览", RoutePath: "/monitor/preview", PageTitle: "监控管理 / 监控预览", PageComponent: "MonitorPreviewView", ObjectType: "监控通道"}
	case strings.Contains(path, "/video/playback"):
		return routeMeta{MenuCode: "monitor-playback", MenuName: "监控管理 / 录像查看", RoutePath: "/monitor/playback", PageTitle: "监控管理 / 录像查看", PageComponent: "PlaybackView", ObjectType: "录像文件"}
	case strings.Contains(path, "/push/configs"):
		return routeMeta{MenuCode: "push-config", MenuName: "推送管理 / 推送配置", RoutePath: "/push/config", PageTitle: "推送管理 / 推送配置", PageComponent: "PushConfigView", ObjectType: "系统配置"}
	case strings.Contains(path, "/push/logs"):
		return routeMeta{MenuCode: "push-logs", MenuName: "推送管理 / 推送日志", RoutePath: "/push/logs", PageTitle: "推送管理 / 推送日志", PageComponent: "PushLogView", ObjectType: "推送记录"}
	case strings.Contains(path, "/dashboard"):
		return routeMeta{MenuCode: "dashboard", MenuName: "首页驾驶舱", RoutePath: "/dashboard", PageTitle: "首页驾驶舱", PageComponent: "DashboardView", ObjectType: "系统配置"}
	default:
		return routeMeta{}
	}
}

func inferOperationType(method string, path string, actionName string) string {
	actionLower := strings.ToLower(strings.TrimSpace(actionName))
	pathLower := strings.ToLower(path)
	switch {
	case strings.Contains(pathLower, "/auth/login"):
		return "登录"
	case strings.Contains(pathLower, "/auth/logout"):
		return "退出"
	case strings.Contains(actionLower, "导出") || strings.Contains(pathLower, "/export/"):
		return "导出"
	case strings.Contains(actionLower, "预览") || strings.Contains(pathLower, "/video/live"):
		return "预览"
	case strings.Contains(actionLower, "回放") || strings.Contains(pathLower, "/video/playback"):
		return "回放"
	case strings.Contains(actionLower, "配置") || strings.Contains(pathLower, "/sdk-config"):
		return "设备配置"
	case strings.Contains(actionLower, "删除") || method == http.MethodDelete:
		return "删除"
	case strings.Contains(actionLower, "新增") || method == http.MethodPost:
		return "新增"
	case strings.Contains(actionLower, "编辑") || method == http.MethodPut || method == http.MethodPatch:
		return "编辑"
	case method == http.MethodGet:
		if strings.Contains(pathLower, "/dashboard") {
			return "浏览页面"
		}
		return "查询"
	default:
		return "按钮点击"
	}
}

func inferActionCode(method string, fullPath string, actionName string) string {
	if strings.TrimSpace(actionName) != "" {
		return strings.ToLower(strings.ReplaceAll(actionName, " ", "-"))
	}
	normalizedPath := strings.TrimSpace(fullPath)
	if normalizedPath == "" {
		return strings.ToLower(method)
	}
	replacer := strings.NewReplacer("/", "-", ":", "", "{", "", "}", "")
	return strings.ToLower(strings.Trim(replacer.Replace(method+"-"+normalizedPath), "-"))
}

func extractLoginUsername(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	var payload struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Username)
}

func extractResponseMessage(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		return payload.Message
	}
	return strings.TrimSpace(body)
}

func resolveIPLocation(ip string) string {
	parsedIP := net.ParseIP(strings.TrimSpace(ip))
	if parsedIP == nil {
		return "未知"
	}
	if parsedIP.IsLoopback() {
		return "本机"
	}
	if parsedIP.IsPrivate() {
		return "局域网"
	}
	return "外网"
}

func resolveOSName(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "mac os"):
		return "macOS"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "iphone"), strings.Contains(ua, "ipad"), strings.Contains(ua, "ios"):
		return "iOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "未知"
	}
}

func buildTraceID() string {
	return time.Now().Format("20060102150405.000000000")
}

func firstHeader(c *gin.Context, key string) string {
	return strings.TrimSpace(c.GetHeader(key))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
