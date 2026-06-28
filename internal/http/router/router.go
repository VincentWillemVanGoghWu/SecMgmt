package router

import (
	"net/http"

	"secmgmt_go/internal/config"
	"secmgmt_go/internal/http/handler"
	"secmgmt_go/internal/http/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth     *handler.AuthHandler
	Query    *handler.QueryHandler
	Platform *handler.PlatformHandler
}

func New(cfg *config.Config, handlers Handlers) *gin.Engine {
	engine := gin.Default()
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))
	engine.Static(cfg.MediaMountPath, cfg.MediaRootDir)

	engine.GET("/healthz", handlers.Query.Health)

	api := engine.Group("/api")
	api.POST("/auth/login", handlers.Auth.Login)
	api.GET("/sse/alarms", handlers.Platform.AlarmSSE)

	protected := api.Group("/")
	protected.Use(middleware.Auth(cfg.JWTSecretKey))

	protected.POST("/auth/logout", handlers.Auth.Logout)
	protected.GET("/auth/me", handlers.Auth.Me)
	protected.GET("/menus", handlers.Auth.Menus)
	protected.GET("/factories", handlers.Query.ListFactories)
	protected.GET("/zones", handlers.Query.ListZones)
	protected.GET("/depts", handlers.Query.ListDepts)
	protected.GET("/dicts", handlers.Query.ListDictTypes)
	protected.GET("/cameras", handlers.Query.ListCameras)
	protected.GET("/recorders", handlers.Query.ListRecorders)
	protected.GET("/channels", handlers.Query.ListChannels)
	protected.GET("/alarms/realtime", handlers.Query.ListRealtimeAlarms)
	protected.GET("/alarms", handlers.Query.ListAlarms)
	protected.GET("/dashboard/summary", handlers.Query.DashboardSummary)
	protected.GET("/users", handlers.Platform.ListUsers)
	protected.POST("/users", handlers.Platform.CreateUser)
	protected.PUT("/users/:id", handlers.Platform.UpdateUser)
	protected.DELETE("/users/:id", handlers.Platform.DeleteUser)

	protected.GET("/roles", handlers.Platform.ListRoles)
	protected.POST("/roles", handlers.Platform.CreateRole)
	protected.PUT("/roles/:id", handlers.Platform.UpdateRole)
	protected.PATCH("/roles/:id/status", handlers.Platform.UpdateRoleStatus)
	protected.PUT("/roles/:id/data-scope", handlers.Platform.UpdateRoleDataScope)
	protected.DELETE("/roles/:id", handlers.Platform.DeleteRole)

	protected.POST("/factories", handlers.Platform.CreateFactory)
	protected.PUT("/factories/:id", handlers.Platform.UpdateFactory)
	protected.PATCH("/factories/:id/status", handlers.Platform.UpdateFactoryStatus)
	protected.DELETE("/factories/:id", handlers.Platform.DeleteFactory)

	protected.POST("/zones", handlers.Platform.CreateZone)
	protected.PUT("/zones/:id", handlers.Platform.UpdateZone)
	protected.PATCH("/zones/:id/status", handlers.Platform.UpdateZoneStatus)
	protected.DELETE("/zones/:id", handlers.Platform.DeleteZone)

	protected.POST("/depts", handlers.Platform.CreateDept)
	protected.PUT("/depts/:id", handlers.Platform.UpdateDept)
	protected.PATCH("/depts/:id/status", handlers.Platform.UpdateDeptStatus)
	protected.DELETE("/depts/:id", handlers.Platform.DeleteDept)

	protected.POST("/dicts/types", handlers.Platform.CreateDictType)
	protected.PUT("/dicts/types/:id", handlers.Platform.UpdateDictType)
	protected.PATCH("/dicts/types/:id/status", handlers.Platform.UpdateDictTypeStatus)
	protected.DELETE("/dicts/types/:id", handlers.Platform.DeleteDictType)
	protected.POST("/dicts/items", handlers.Platform.CreateDictItem)
	protected.PUT("/dicts/items/:id", handlers.Platform.UpdateDictItem)
	protected.PATCH("/dicts/items/:id/status", handlers.Platform.UpdateDictItemStatus)
	protected.DELETE("/dicts/items/:id", handlers.Platform.DeleteDictItem)

	protected.GET("/cameras/:id", handlers.Platform.GetCamera)
	protected.GET("/cameras/:id/browser-login", handlers.Platform.GetCameraBrowserLogin)
	protected.POST("/cameras", handlers.Platform.CreateCamera)
	protected.POST("/cameras/sdk-device-identity", handlers.Platform.FetchCameraDeviceIdentity)
	protected.PUT("/cameras/:id", handlers.Platform.UpdateCamera)
	protected.PATCH("/cameras/:id/status", handlers.Platform.UpdateCameraStatus)
	protected.DELETE("/cameras/:id", handlers.Platform.DeleteCamera)
	protected.POST("/cameras/:id/test", handlers.Platform.TestCameraConnection)
	protected.POST("/cameras/:id/status/check", handlers.Platform.CheckCameraStatus)
	protected.GET("/cameras/:id/sdk-config", handlers.Platform.GetCameraSDKConfig)
	protected.PUT("/cameras/:id/sdk-config/network", handlers.Platform.UpdateCameraNetworkConfig)
	protected.PUT("/cameras/:id/sdk-config/image", handlers.Platform.UpdateCameraImageConfig)
	protected.PUT("/cameras/:id/sdk-config/recording", handlers.Platform.UpdateCameraRecordingConfig)
	protected.PUT("/cameras/:id/sdk-config/ptz/presets", handlers.Platform.SetCameraPTZPreset)
	protected.DELETE("/cameras/:id/sdk-config/ptz/presets/:presetId", handlers.Platform.DeleteCameraPTZPreset)
	protected.PUT("/cameras/:id/sdk-config/ptz/presets/:presetId/goto", handlers.Platform.GotoCameraPTZPreset)
	protected.PUT("/cameras/:id/sdk-config/ptz", handlers.Platform.UpdateCameraPTZConfig)
	protected.PUT("/cameras/:id/sdk-config/ptz/zoom/:action", handlers.Platform.ControlCameraPTZZoom)
	protected.PUT("/cameras/:id/sdk-config/users", handlers.Platform.UpsertCameraUser)
	protected.DELETE("/cameras/:id/sdk-config/users/:userId", handlers.Platform.DeleteCameraUser)

	protected.GET("/recorders/:id", handlers.Platform.GetRecorder)
	protected.POST("/recorders", handlers.Platform.CreateRecorder)
	protected.PUT("/recorders/:id", handlers.Platform.UpdateRecorder)
	protected.DELETE("/recorders/:id", handlers.Platform.DeleteRecorder)
	protected.POST("/recorders/:id/test", handlers.Platform.TestRecorderConnection)
	protected.POST("/recorders/:id/status/check", handlers.Platform.CheckRecorderStatus)
	protected.POST("/recorders/:id/sync-channels", handlers.Platform.SyncRecorderChannels)
	protected.GET("/recorders/:id/channels", handlers.Platform.ListRecorderChannels)

	protected.PUT("/channels/:id", handlers.Platform.UpdateChannel)

	protected.GET("/device-status/logs", handlers.Platform.ListDeviceStatusLogs)
	protected.POST("/devices/status/check-all", handlers.Platform.CheckAllDevicesStatus)

	protected.GET("/alarms/:id", handlers.Platform.GetAlarmDetail)
	protected.POST("/alarms/:id/process", handlers.Platform.ProcessAlarm)
	protected.POST("/alarms/:id/false-alarm", handlers.Platform.FalseAlarm)
	protected.POST("/alarms/:id/repush", handlers.Platform.RePushAlarm)

	protected.GET("/dashboard/alarm-trend", handlers.Platform.DashboardAlarmTrend)
	protected.GET("/dashboard/alarm-types", handlers.Platform.DashboardAlarmTypes)
	protected.GET("/dashboard/zone-ranking", handlers.Platform.DashboardZoneRanking)
	protected.GET("/dashboard/device-status", handlers.Platform.DashboardDeviceStatus)

	protected.GET("/reports/alarms", handlers.Platform.AlarmReport)
	protected.GET("/reports/devices", handlers.Platform.DeviceReport)
	protected.GET("/reports/push", handlers.Platform.PushReport)

	protected.GET("/push/configs", handlers.Platform.ListPushConfigs)
	protected.POST("/push/configs", handlers.Platform.CreatePushConfig)
	protected.PUT("/push/configs/:id", handlers.Platform.UpdatePushConfig)
	protected.PATCH("/push/configs/:id/status", handlers.Platform.UpdatePushConfigStatus)
	protected.DELETE("/push/configs/:id", handlers.Platform.DeletePushConfig)
	protected.POST("/push/configs/:id/test", handlers.Platform.TestPushConfig)
	protected.GET("/push/logs", handlers.Platform.ListPushLogs)
	protected.POST("/push/logs/:id/retry", handlers.Platform.RetryPushLog)

	protected.GET("/smart/providers", handlers.Platform.ListSmartProviders)
	protected.POST("/smart/providers", handlers.Platform.CreateSmartProvider)
	protected.PUT("/smart/providers/:id", handlers.Platform.UpdateSmartProvider)
	protected.POST("/smart/providers/:id/test", handlers.Platform.TestSmartProvider)
	protected.GET("/smart/capabilities", handlers.Platform.ListSmartCapabilities)
	protected.GET("/smart/bindings", handlers.Platform.ListSmartBindings)
	protected.POST("/smart/bindings", handlers.Platform.CreateSmartBinding)
	protected.GET("/smart/bindings/:id", handlers.Platform.GetSmartBindingDetail)
	protected.POST("/smart/bindings/:id/test", handlers.Platform.TestSmartBinding)
	protected.PUT("/smart/bindings/:id", handlers.Platform.UpdateSmartBinding)
	protected.DELETE("/smart/bindings/:id", handlers.Platform.DeleteSmartBinding)
	protected.POST("/smart/bindings/:id/rules", handlers.Platform.CreateSmartBindingRule)
	protected.PUT("/smart/bindings/:id/rules/:ruleId", handlers.Platform.UpdateSmartBindingRule)
	protected.DELETE("/smart/bindings/:id/rules/:ruleId", handlers.Platform.DeleteSmartBindingRule)
	protected.POST("/smart/events/ingest/:providerCode", handlers.Platform.IngestSmartProviderEvent)
	protected.GET("/smart/raw-events", handlers.Platform.ListSmartRawEvents)
	protected.GET("/smart/events", handlers.Platform.ListSmartEvents)
	protected.GET("/smart/events/:id", handlers.Platform.GetSmartEventDetail)
	protected.POST("/smart/events/:id/submit-ai-review", handlers.Platform.SubmitSmartAIReview)
	protected.GET("/smart/ai-tasks", handlers.Platform.ListSmartAITasks)
	protected.GET("/smart/ai-tasks/:taskId", handlers.Platform.GetSmartAITask)
	protected.POST("/smart/ai-tasks/:taskId/retry", handlers.Platform.RetrySmartAITask)
	protected.POST("/smart/ai/callback", handlers.Platform.HandleSmartAICallback)

	protected.GET("/video/live/channel/:id", handlers.Platform.GetChannelLiveVideo)
	protected.POST("/video/live/channel/:id/stop", handlers.Platform.StopChannelLiveVideo)
	protected.GET("/video/live/channel/:id/webcontrol-config", handlers.Platform.GetChannelLiveWebControlConfig)
	protected.GET("/video/live/:id", handlers.Platform.GetLiveVideo)
	protected.POST("/video/live/:id/stop", handlers.Platform.StopLiveVideo)
	protected.GET("/video/live/:id/webcontrol-config", handlers.Platform.GetLiveWebControlConfig)
	protected.POST("/video/snapshot", handlers.Platform.CreateSnapshot)
	protected.GET("/video/playback/search", handlers.Platform.SearchPlaybackSegments)
	protected.GET("/video/playback/url", handlers.Platform.GetPlaybackURL)
	protected.POST("/video/playback/seek", handlers.Platform.SeekPlayback)
	protected.GET("/video/playback/download", handlers.Platform.DownloadPlaybackFile)
	protected.POST("/video/playback/stop", handlers.Platform.StopPlayback)

	protected.GET("/export/alarms", handlers.Platform.ExportAlarms)
	protected.GET("/export/device-status", handlers.Platform.ExportDeviceStatus)
	protected.GET("/export/push-logs", handlers.Platform.ExportPushLogs)

	protected.GET("/ai/config", handlers.Platform.GetAIConfig)
	protected.POST("/ai/events/callback", handlers.Platform.CreateAIEventCallback)
	protected.GET("/ai/events", handlers.Platform.ListAIEvents)
	protected.GET("/ai/events/:id", handlers.Platform.GetAIEventDetail)

	return engine
}
