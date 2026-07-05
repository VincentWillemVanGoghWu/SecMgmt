package router

import (
	"net/http"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/http/handler"
	"secmgmt_go/internal/http/middleware"
	"secmgmt_go/internal/repository"
	"secmgmt_go/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth      *handler.AuthHandler
	Query     *handler.QueryHandler
	Platform  *handler.PlatformHandler
	Operation *handler.OperationLogHandler
}

func New(cfg *config.Config, repo *repository.Repository, operationLogService *service.OperationLogService, handlers Handlers) *gin.Engine {
	engine := gin.Default()
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Track-Source", "X-Menu-Code", "X-Menu-Name", "X-Page-Route", "X-Page-Title", "X-Page-Component", "X-Action-Code", "X-Action-Name", "X-Operation-Type", "X-Object-Type", "X-Object-Id", "X-Object-Name", "X-Object-Location", "X-Trace-Id", "X-Client-OS"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))
	engine.Use(middleware.OperationLogger(operationLogService, repo))
	engine.Static(cfg.MediaMountPath, cfg.MediaRootDir)

	engine.GET("/healthz", handlers.Query.Health)
	engine.POST("/smart/events/ingest/:providerCode", handlers.Platform.IngestSmartProviderEvent)

	api := engine.Group("/api")
	api.POST("/auth/login", handlers.Auth.Login)
	api.GET("/sse/alarms", handlers.Platform.AlarmSSE)
	api.POST("/smart/events/ingest/:providerCode", handlers.Platform.IngestSmartProviderEvent)

	protected := api.Group("/")
	protected.Use(middleware.Auth(cfg.JWTSecretKey, repo))

	withPermission := func(permission string, handler gin.HandlerFunc) []gin.HandlerFunc {
		return []gin.HandlerFunc{middleware.RequirePermission(permission), handler}
	}
	withAnyPermission := func(handler gin.HandlerFunc, permissions ...string) []gin.HandlerFunc {
		return []gin.HandlerFunc{middleware.RequireAnyPermission(permissions...), handler}
	}

	protected.POST("/auth/logout", handlers.Auth.Logout)
	protected.GET("/auth/me", handlers.Auth.Me)
	protected.GET("/menus", handlers.Auth.Menus)
	protected.GET("/factories", withPermission("basic:factory:view", handlers.Query.ListFactories)...)
	protected.GET("/zones", withPermission("basic:zone:view", handlers.Query.ListZones)...)
	protected.GET("/depts", withPermission("basic:dept:view", handlers.Query.ListDepts)...)
	protected.GET("/dicts", withPermission("basic:dict:view", handlers.Query.ListDictTypes)...)
	protected.GET("/cameras", withPermission("device:camera:view", handlers.Query.ListCameras)...)
	protected.GET("/recorders", withPermission("device:recorder:view", handlers.Query.ListRecorders)...)
	protected.GET("/channels", withPermission("device:channel:view", handlers.Query.ListChannels)...)
	protected.GET("/alarms/realtime", withPermission("alarm:realtime:view", handlers.Query.ListRealtimeAlarms)...)
	protected.GET("/alarms", withPermission("alarm:view", handlers.Query.ListAlarms)...)
	protected.GET("/dashboard/summary", withPermission("dashboard:stats:view", handlers.Query.DashboardSummary)...)
	protected.GET("/dashboard/operation-stats", withPermission("dashboard:stats:view", handlers.Operation.DashboardStats)...)
	protected.GET("/users", withPermission("system:user:view", handlers.Platform.ListUsers)...)
	protected.POST("/users", withPermission("system:user:create", handlers.Platform.CreateUser)...)
	protected.PUT("/users/:id", withPermission("system:user:update", handlers.Platform.UpdateUser)...)
	protected.DELETE("/users/:id", withPermission("system:user:delete", handlers.Platform.DeleteUser)...)

	protected.GET("/roles", withPermission("system:role:view", handlers.Platform.ListRoles)...)
	protected.GET("/roles/menu-tree", withPermission("system:role:view", handlers.Platform.ListRoleMenuTree)...)
	protected.GET("/roles/permission-options", withPermission("system:role:view", handlers.Platform.ListRolePermissionOptions)...)
	protected.POST("/roles", withPermission("system:role:create", handlers.Platform.CreateRole)...)
	protected.PUT("/roles/:id", withPermission("system:role:update", handlers.Platform.UpdateRole)...)
	protected.PATCH("/roles/:id/status", withAnyPermission(handlers.Platform.UpdateRoleStatus, "system:role:update", "system:role:enable", "system:role:disable")...)
	protected.PUT("/roles/:id/data-scope", withPermission("system:role:update", handlers.Platform.UpdateRoleDataScope)...)
	protected.PUT("/roles/:id/menus", withPermission("system:role:update", handlers.Platform.UpdateRoleMenus)...)
	protected.PUT("/roles/:id/permissions", withPermission("system:role:update", handlers.Platform.UpdateRolePermissions)...)
	protected.DELETE("/roles/:id", withPermission("system:role:delete", handlers.Platform.DeleteRole)...)

	protected.POST("/factories", withPermission("basic:factory:create", handlers.Platform.CreateFactory)...)
	protected.PUT("/factories/:id", withPermission("basic:factory:update", handlers.Platform.UpdateFactory)...)
	protected.PATCH("/factories/:id/status", withPermission("basic:factory:update", handlers.Platform.UpdateFactoryStatus)...)
	protected.DELETE("/factories/:id", withPermission("basic:factory:delete", handlers.Platform.DeleteFactory)...)

	protected.POST("/zones", withPermission("basic:zone:create", handlers.Platform.CreateZone)...)
	protected.PUT("/zones/:id", withPermission("basic:zone:update", handlers.Platform.UpdateZone)...)
	protected.PATCH("/zones/:id/status", withPermission("basic:zone:update", handlers.Platform.UpdateZoneStatus)...)
	protected.DELETE("/zones/:id", withPermission("basic:zone:delete", handlers.Platform.DeleteZone)...)

	protected.POST("/depts", withPermission("basic:dept:create", handlers.Platform.CreateDept)...)
	protected.PUT("/depts/:id", withPermission("basic:dept:update", handlers.Platform.UpdateDept)...)
	protected.PATCH("/depts/:id/status", withPermission("basic:dept:update", handlers.Platform.UpdateDeptStatus)...)
	protected.DELETE("/depts/:id", withPermission("basic:dept:delete", handlers.Platform.DeleteDept)...)

	protected.POST("/dicts/types", withPermission("basic:dict:create", handlers.Platform.CreateDictType)...)
	protected.PUT("/dicts/types/:id", withPermission("basic:dict:update", handlers.Platform.UpdateDictType)...)
	protected.PATCH("/dicts/types/:id/status", withPermission("basic:dict:update", handlers.Platform.UpdateDictTypeStatus)...)
	protected.DELETE("/dicts/types/:id", withPermission("basic:dict:delete", handlers.Platform.DeleteDictType)...)
	protected.POST("/dicts/items", withPermission("basic:dict:create", handlers.Platform.CreateDictItem)...)
	protected.PUT("/dicts/items/:id", withPermission("basic:dict:update", handlers.Platform.UpdateDictItem)...)
	protected.PATCH("/dicts/items/:id/status", withPermission("basic:dict:update", handlers.Platform.UpdateDictItemStatus)...)
	protected.DELETE("/dicts/items/:id", withPermission("basic:dict:delete", handlers.Platform.DeleteDictItem)...)

	protected.GET("/cameras/:id", withPermission("device:camera:view", handlers.Platform.GetCamera)...)
	protected.GET("/cameras/:id/browser-login", withPermission("device:camera:view", handlers.Platform.GetCameraBrowserLogin)...)
	protected.POST("/cameras", withPermission("device:camera:create", handlers.Platform.CreateCamera)...)
	protected.POST("/cameras/sdk-device-identity", withPermission("device:camera:create", handlers.Platform.FetchCameraDeviceIdentity)...)
	protected.PUT("/cameras/:id", withPermission("device:camera:update", handlers.Platform.UpdateCamera)...)
	protected.PATCH("/cameras/:id/status", withPermission("device:camera:update", handlers.Platform.UpdateCameraStatus)...)
	protected.DELETE("/cameras/:id", withPermission("device:camera:delete", handlers.Platform.DeleteCamera)...)
	protected.POST("/cameras/:id/test", withPermission("device:camera:test", handlers.Platform.TestCameraConnection)...)
	protected.POST("/cameras/:id/status/check", withPermission("device:camera:check", handlers.Platform.CheckCameraStatus)...)
	protected.GET("/cameras/:id/sdk-config", withPermission("device:camera:update", handlers.Platform.GetCameraSDKConfig)...)
	protected.PUT("/cameras/:id/sdk-config/network", withPermission("device:camera:update", handlers.Platform.UpdateCameraNetworkConfig)...)
	protected.PUT("/cameras/:id/sdk-config/image", withPermission("device:camera:update", handlers.Platform.UpdateCameraImageConfig)...)
	protected.PUT("/cameras/:id/sdk-config/recording", withPermission("device:camera:update", handlers.Platform.UpdateCameraRecordingConfig)...)
	protected.PUT("/cameras/:id/sdk-config/ptz/presets", withPermission("device:camera:update", handlers.Platform.SetCameraPTZPreset)...)
	protected.DELETE("/cameras/:id/sdk-config/ptz/presets/:presetId", withPermission("device:camera:update", handlers.Platform.DeleteCameraPTZPreset)...)
	protected.PUT("/cameras/:id/sdk-config/ptz/presets/:presetId/goto", withPermission("device:camera:update", handlers.Platform.GotoCameraPTZPreset)...)
	protected.PUT("/cameras/:id/sdk-config/ptz", withPermission("device:camera:update", handlers.Platform.UpdateCameraPTZConfig)...)
	protected.PUT("/cameras/:id/sdk-config/ptz/zoom/:action", withPermission("device:camera:update", handlers.Platform.ControlCameraPTZZoom)...)
	protected.PUT("/cameras/:id/sdk-config/users", withPermission("device:camera:update", handlers.Platform.UpsertCameraUser)...)
	protected.DELETE("/cameras/:id/sdk-config/users/:userId", withPermission("device:camera:update", handlers.Platform.DeleteCameraUser)...)

	protected.GET("/recorders/:id", withPermission("device:recorder:view", handlers.Platform.GetRecorder)...)
	protected.POST("/recorders", withPermission("device:recorder:create", handlers.Platform.CreateRecorder)...)
	protected.PUT("/recorders/:id", withPermission("device:recorder:update", handlers.Platform.UpdateRecorder)...)
	protected.DELETE("/recorders/:id", withPermission("device:recorder:delete", handlers.Platform.DeleteRecorder)...)
	protected.POST("/recorders/:id/test", withPermission("device:recorder:test", handlers.Platform.TestRecorderConnection)...)
	protected.POST("/recorders/:id/status/check", withPermission("device:recorder:test", handlers.Platform.CheckRecorderStatus)...)
	protected.POST("/recorders/:id/sync-channels", withPermission("device:recorder:sync", handlers.Platform.SyncRecorderChannels)...)
	protected.GET("/recorders/:id/channels", withPermission("device:recorder:view", handlers.Platform.ListRecorderChannels)...)

	protected.PUT("/channels/:id", withPermission("device:channel:update", handlers.Platform.UpdateChannel)...)

	protected.GET("/device-status/logs", withPermission("device:status:log:view", handlers.Platform.ListDeviceStatusLogs)...)
	protected.POST("/devices/status/check-all", withPermission("device:status:check", handlers.Platform.CheckAllDevicesStatus)...)
	protected.GET("/device-check/schedules", withPermission("device:check-plan:view", handlers.Platform.ListDeviceCheckSchedules)...)
	protected.POST("/device-check/schedules", withPermission("device:check-plan:create", handlers.Platform.CreateDeviceCheckSchedule)...)
	protected.PUT("/device-check/schedules/:id", withPermission("device:check-plan:update", handlers.Platform.UpdateDeviceCheckSchedule)...)
	protected.PATCH("/device-check/schedules/:id/status", withPermission("device:check-plan:update", handlers.Platform.UpdateDeviceCheckScheduleStatus)...)
	protected.DELETE("/device-check/schedules/:id", withPermission("device:check-plan:delete", handlers.Platform.DeleteDeviceCheckSchedule)...)
	protected.POST("/device-check/schedules/:id/run", withPermission("device:check-plan:run", handlers.Platform.RunDeviceCheckScheduleNow)...)
	protected.GET("/device-check/runs", withPermission("device:check-plan:view", handlers.Platform.ListDeviceCheckRuns)...)

	protected.GET("/alarms/:id", withPermission("alarm:view", handlers.Platform.GetAlarmDetail)...)
	protected.POST("/alarms/:id/process", withPermission("alarm:process", handlers.Platform.ProcessAlarm)...)
	protected.POST("/alarms/:id/false-alarm", withPermission("alarm:process", handlers.Platform.FalseAlarm)...)
	protected.POST("/alarms/:id/repush", withPermission("alarm:repush", handlers.Platform.RePushAlarm)...)

	protected.GET("/dashboard/alarm-trend", withPermission("dashboard:stats:view", handlers.Platform.DashboardAlarmTrend)...)
	protected.GET("/dashboard/alarm-types", withPermission("dashboard:stats:view", handlers.Platform.DashboardAlarmTypes)...)
	protected.GET("/dashboard/zone-ranking", withPermission("dashboard:stats:view", handlers.Platform.DashboardZoneRanking)...)
	protected.GET("/dashboard/device-status", withPermission("dashboard:stats:view", handlers.Platform.DashboardDeviceStatus)...)

	protected.POST("/operation-logs/track", withPermission("log:operation:view", handlers.Operation.Track)...)
	protected.GET("/operation-logs", withPermission("log:operation:view", handlers.Operation.List)...)
	protected.GET("/operation-logs/:id", withPermission("log:operation:view", handlers.Operation.Detail)...)

	protected.GET("/reports/alarms", withPermission("report:alarm:view", handlers.Platform.AlarmReport)...)
	protected.GET("/reports/devices", withPermission("report:device:view", handlers.Platform.DeviceReport)...)
	protected.GET("/reports/push", withPermission("report:push:view", handlers.Platform.PushReport)...)

	protected.GET("/push/configs", withPermission("push:config:view", handlers.Platform.ListPushConfigs)...)
	protected.POST("/push/configs", withPermission("push:config:create", handlers.Platform.CreatePushConfig)...)
	protected.PUT("/push/configs/:id", withPermission("push:config:update", handlers.Platform.UpdatePushConfig)...)
	protected.PATCH("/push/configs/:id/status", withPermission("push:config:update", handlers.Platform.UpdatePushConfigStatus)...)
	protected.DELETE("/push/configs/:id", withPermission("push:config:delete", handlers.Platform.DeletePushConfig)...)
	protected.POST("/push/configs/:id/test", withPermission("push:config:test", handlers.Platform.TestPushConfig)...)
	protected.GET("/push/logs", withPermission("push:log:view", handlers.Platform.ListPushLogs)...)
	protected.POST("/push/logs/:id/retry", withPermission("push:log:retry", handlers.Platform.RetryPushLog)...)

	protected.GET("/smart/providers", withPermission("smart:provider:view", handlers.Platform.ListSmartProviders)...)
	protected.POST("/smart/providers", withPermission("smart:provider:create", handlers.Platform.CreateSmartProvider)...)
	protected.PUT("/smart/providers/:id", withPermission("smart:provider:update", handlers.Platform.UpdateSmartProvider)...)
	protected.POST("/smart/providers/:id/test", withPermission("smart:provider:test", handlers.Platform.TestSmartProvider)...)
	protected.GET("/smart/bridge/status", withPermission("smart:binding:view", handlers.Platform.GetSmartBridgeStatus)...)
	protected.GET("/smart/capabilities", withPermission("smart:capability:view", handlers.Platform.ListSmartCapabilities)...)
	protected.GET("/smart/bindings", withPermission("smart:binding:view", handlers.Platform.ListSmartBindings)...)
	protected.POST("/smart/bindings", withPermission("smart:binding:create", handlers.Platform.CreateSmartBinding)...)
	protected.GET("/smart/bindings/:id", withPermission("smart:binding:view", handlers.Platform.GetSmartBindingDetail)...)
	protected.POST("/smart/bindings/:id/test", withPermission("smart:binding:test", handlers.Platform.TestSmartBinding)...)
	protected.POST("/smart/bindings/:id/reconnect", withPermission("smart:binding:test", handlers.Platform.ReconnectSmartBinding)...)
	protected.POST("/smart/bindings/:id/reload", withPermission("smart:binding:test", handlers.Platform.ReloadSmartBinding)...)
	protected.PUT("/smart/bindings/:id", withPermission("smart:binding:update", handlers.Platform.UpdateSmartBinding)...)
	protected.DELETE("/smart/bindings/:id", withPermission("smart:binding:delete", handlers.Platform.DeleteSmartBinding)...)
	protected.POST("/smart/bindings/:id/rules", withPermission("smart:rule:create", handlers.Platform.CreateSmartBindingRule)...)
	protected.PUT("/smart/bindings/:id/rules/:ruleId", withPermission("smart:rule:update", handlers.Platform.UpdateSmartBindingRule)...)
	protected.DELETE("/smart/bindings/:id/rules/:ruleId", withPermission("smart:rule:delete", handlers.Platform.DeleteSmartBindingRule)...)
	protected.GET("/smart/raw-events", withPermission("smart:event:view", handlers.Platform.ListSmartRawEvents)...)
	protected.GET("/smart/events", withPermission("smart:event:view", handlers.Platform.ListSmartEvents)...)
	protected.GET("/smart/bridge/reconnect-logs", withPermission("smart:event:view", handlers.Platform.ListSmartBridgeReconnectLogs)...)
	protected.GET("/smart/events/:id", withPermission("smart:event:detail:view", handlers.Platform.GetSmartEventDetail)...)
	protected.POST("/smart/events/:id/submit-ai-review", withPermission("smart:event:submit-ai-review", handlers.Platform.SubmitSmartAIReview)...)
	protected.GET("/smart/ai-tasks", withPermission("smart:ai-task:view", handlers.Platform.ListSmartAITasks)...)
	protected.GET("/smart/ai-tasks/:taskId", withPermission("smart:ai-task:view", handlers.Platform.GetSmartAITask)...)
	protected.POST("/smart/ai-tasks/:taskId/retry", withPermission("smart:ai-task:retry", handlers.Platform.RetrySmartAITask)...)
	protected.POST("/smart/ai/callback", handlers.Platform.HandleSmartAICallback)

	protected.GET("/video/live/channel/:id", withPermission("video:live:view", handlers.Platform.GetChannelLiveVideo)...)
	protected.POST("/video/live/channel/:id/stop", withPermission("video:live:control", handlers.Platform.StopChannelLiveVideo)...)
	protected.GET("/video/live/channel/:id/webcontrol-config", withPermission("video:webcontrol:view", handlers.Platform.GetChannelLiveWebControlConfig)...)
	protected.GET("/video/live/:id", withPermission("video:live:view", handlers.Platform.GetLiveVideo)...)
	protected.POST("/video/live/:id/stop", withPermission("video:live:control", handlers.Platform.StopLiveVideo)...)
	protected.GET("/video/live/:id/webcontrol-config", withPermission("video:webcontrol:view", handlers.Platform.GetLiveWebControlConfig)...)
	protected.POST("/video/snapshot", withPermission("video:snapshot:create", handlers.Platform.CreateSnapshot)...)
	protected.GET("/video/playback/search", withPermission("video:playback:search", handlers.Platform.SearchPlaybackSegments)...)
	protected.GET("/video/playback/url", withPermission("video:playback:view", handlers.Platform.GetPlaybackURL)...)
	protected.POST("/video/playback/seek", withPermission("video:playback:view", handlers.Platform.SeekPlayback)...)
	protected.GET("/video/playback/download", withPermission("video:playback:download", handlers.Platform.DownloadPlaybackFile)...)
	protected.POST("/video/playback/stop", withPermission("video:playback:view", handlers.Platform.StopPlayback)...)

	protected.GET("/export/alarms", withPermission("report:alarm:export", handlers.Platform.ExportAlarms)...)
	protected.GET("/export/device-status", withPermission("report:device:export", handlers.Platform.ExportDeviceStatus)...)
	protected.GET("/export/push-logs", withPermission("report:push:export", handlers.Platform.ExportPushLogs)...)
	protected.GET("/export/operation-logs", withPermission("log:operation:export", handlers.Operation.Export)...)

	protected.GET("/ai/config", withPermission("ai:event:view", handlers.Platform.GetAIConfig)...)
	protected.POST("/ai/events/callback", handlers.Platform.CreateAIEventCallback)
	protected.GET("/ai/events", withPermission("ai:event:view", handlers.Platform.ListAIEvents)...)
	protected.GET("/ai/events/:id", withPermission("ai:event:view", handlers.Platform.GetAIEventDetail)...)

	return engine
}
