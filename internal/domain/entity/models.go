package entity

import "time"

type User struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	Username     string    `gorm:"column:username"`
	PasswordHash string    `gorm:"column:password_hash"`
	RealName     string    `gorm:"column:real_name"`
	DeptID       *uint     `gorm:"column:dept_id"`
	Status       string    `gorm:"column:status"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "sys_user" }

type Role struct {
	ID             uint   `gorm:"column:id;primaryKey"`
	RoleCode       string `gorm:"column:role_code"`
	RoleName       string `gorm:"column:role_name"`
	Status         string `gorm:"column:status"`
	Remark         string `gorm:"column:remark"`
	DataScopeType  string `gorm:"column:data_scope_type"`
	DataScopeValue string `gorm:"column:data_scope_value"`
}

func (Role) TableName() string { return "sys_role" }

type Menu struct {
	ID        uint   `gorm:"column:id;primaryKey"`
	Name      string `gorm:"column:name"`
	Code      string `gorm:"column:code"`
	ParentID  *uint  `gorm:"column:parent_id"`
	RouteName string `gorm:"column:route_name"`
	RoutePath string `gorm:"column:route_path"`
	Icon      string `gorm:"column:icon"`
	MenuType  string `gorm:"column:menu_type"`
	Sort      int    `gorm:"column:sort"`
	Status    string `gorm:"column:status"`
}

func (Menu) TableName() string { return "sys_menu" }

type Permission struct {
	ID       uint   `gorm:"column:id;primaryKey"`
	Name     string `gorm:"column:name"`
	Code     string `gorm:"column:code"`
	Status   string `gorm:"column:status"`
	Remark   string `gorm:"column:remark"`
	IsButton bool   `gorm:"column:is_button"`
}

func (Permission) TableName() string { return "sys_permission" }

type FactoryArea struct {
	ID          uint      `gorm:"column:id;primaryKey"`
	FactoryCode string    `gorm:"column:factory_code"`
	FactoryName string    `gorm:"column:factory_name"`
	Status      string    `gorm:"column:status"`
	Remark      string    `gorm:"column:remark"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (FactoryArea) TableName() string { return "factory_area" }

type FactoryZone struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	FactoryID uint      `gorm:"column:factory_id"`
	ZoneCode  string    `gorm:"column:zone_code"`
	ZoneName  string    `gorm:"column:zone_name"`
	Status    string    `gorm:"column:status"`
	Remark    string    `gorm:"column:remark"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (FactoryZone) TableName() string { return "factory_zone" }

type SysDept struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	DeptCode  string    `gorm:"column:dept_code"`
	DeptName  string    `gorm:"column:dept_name"`
	ParentID  *uint     `gorm:"column:parent_id"`
	FactoryID *uint     `gorm:"column:factory_id"`
	ZoneID    *uint     `gorm:"column:zone_id"`
	Leader    string    `gorm:"column:leader"`
	Phone     string    `gorm:"column:phone"`
	Sort      int       `gorm:"column:sort"`
	Status    string    `gorm:"column:status"`
	Remark    string    `gorm:"column:remark"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (SysDept) TableName() string { return "sys_dept" }

type SysDictType struct {
	ID        uint      `gorm:"column:id;primaryKey"`
	DictCode  string    `gorm:"column:dict_code"`
	DictName  string    `gorm:"column:dict_name"`
	Status    string    `gorm:"column:status"`
	Remark    string    `gorm:"column:remark"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (SysDictType) TableName() string { return "sys_dict_type" }

type SysDictItem struct {
	ID         uint      `gorm:"column:id;primaryKey"`
	DictTypeID uint      `gorm:"column:dict_type_id"`
	ItemLabel  string    `gorm:"column:item_label"`
	ItemValue  string    `gorm:"column:item_value"`
	ItemSort   int       `gorm:"column:item_sort"`
	IsDefault  bool      `gorm:"column:is_default"`
	Status     string    `gorm:"column:status"`
	Remark     string    `gorm:"column:remark"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (SysDictItem) TableName() string { return "sys_dict_item" }

type CameraDevice struct {
	ID                uint       `gorm:"column:id;primaryKey"`
	DeviceCode        string     `gorm:"column:device_code"`
	Name              string     `gorm:"column:name"`
	IP                string     `gorm:"column:ip"`
	SDKPort           int        `gorm:"column:sdk_port"`
	HTTPPort          int        `gorm:"column:http_port"`
	RTSPPort          int        `gorm:"column:rtsp_port"`
	Username          string     `gorm:"column:username"`
	PasswordEncrypted string     `gorm:"column:password_encrypted"`
	FactoryID         uint       `gorm:"column:factory_id"`
	ZoneID            uint       `gorm:"column:zone_id"`
	InstallLocation   string     `gorm:"column:install_location"`
	SupportAI         bool       `gorm:"column:support_ai"`
	Status            string     `gorm:"column:status"`
	LastOnlineAt      *time.Time `gorm:"column:last_online_at"`
	Remark            string     `gorm:"column:remark"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (CameraDevice) TableName() string { return "camera_device" }

type RecorderDevice struct {
	ID                uint       `gorm:"column:id;primaryKey"`
	DeviceCode        string     `gorm:"column:device_code"`
	Name              string     `gorm:"column:name"`
	IP                string     `gorm:"column:ip"`
	SDKPort           int        `gorm:"column:sdk_port"`
	HTTPPort          int        `gorm:"column:http_port"`
	Username          string     `gorm:"column:username"`
	PasswordEncrypted string     `gorm:"column:password_encrypted"`
	ChannelCount      int        `gorm:"column:channel_count"`
	FactoryID         uint       `gorm:"column:factory_id"`
	Status            string     `gorm:"column:status"`
	LastOnlineAt      *time.Time `gorm:"column:last_online_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (RecorderDevice) TableName() string { return "recorder_device" }

type RecorderChannel struct {
	ID              uint      `gorm:"column:id;primaryKey"`
	RecorderID      uint      `gorm:"column:recorder_id"`
	ChannelNo       int       `gorm:"column:channel_no"`
	Name            string    `gorm:"column:name"`
	CameraID        *uint     `gorm:"column:camera_id"`
	FactoryID       uint      `gorm:"column:factory_id"`
	ZoneID          *uint     `gorm:"column:zone_id"`
	Enabled         bool      `gorm:"column:enabled"`
	SupportPlayback bool      `gorm:"column:support_playback"`
	Status          string    `gorm:"column:status"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (RecorderChannel) TableName() string { return "recorder_channel" }

type AlarmRecord struct {
	ID              uint       `gorm:"column:id;primaryKey"`
	AlarmNo         string     `gorm:"column:alarm_no"`
	AIEventID       *uint      `gorm:"column:ai_event_id"`
	SmartEventID    *uint      `gorm:"column:smart_event_id"`
	RawEventID      *uint      `gorm:"column:raw_event_id"`
	AIReviewTaskID  *uint      `gorm:"column:ai_review_task_id"`
	AlarmType       string     `gorm:"column:alarm_type"`
	AlarmLevel      string     `gorm:"column:alarm_level"`
	AlarmTime       time.Time  `gorm:"column:alarm_time"`
	Status          string     `gorm:"column:status"`
	SourceStage     string     `gorm:"column:source_stage"`
	ParentAlarmID   *uint      `gorm:"column:parent_alarm_id"`
	AlarmOrigin     string     `gorm:"column:alarm_origin"`
	CameraID        *uint      `gorm:"column:camera_id"`
	RecorderID      *uint      `gorm:"column:recorder_id"`
	ChannelID       *uint      `gorm:"column:channel_id"`
	FactoryID       *uint      `gorm:"column:factory_id"`
	ZoneID          *uint      `gorm:"column:zone_id"`
	Message         string     `gorm:"column:message"`
	ImageURL        string     `gorm:"column:image_url"`
	VideoURL        string     `gorm:"column:video_url"`
	RecordStartTime *time.Time `gorm:"column:record_start_time"`
	RecordEndTime   *time.Time `gorm:"column:record_end_time"`
	PushRecordsJSON string     `gorm:"column:push_records_json"`
	DedupKey        string     `gorm:"column:dedup_key"`
	OccurrenceCount int        `gorm:"column:occurrence_count"`
	LastEventTime   *time.Time `gorm:"column:last_event_time"`
	LastPushAt      *time.Time `gorm:"column:last_push_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (AlarmRecord) TableName() string { return "alarm_record" }

type AlarmProcessLog struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	AlarmID      uint      `gorm:"column:alarm_id"`
	Action       string    `gorm:"column:action"`
	FromStatus   string    `gorm:"column:from_status"`
	ToStatus     string    `gorm:"column:to_status"`
	OperatorID   *uint     `gorm:"column:operator_id"`
	OperatorName string    `gorm:"column:operator_name"`
	Remark       string    `gorm:"column:remark"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (AlarmProcessLog) TableName() string { return "alarm_process_log" }

type DeviceStatusLog struct {
	ID         uint      `gorm:"column:id;primaryKey"`
	DeviceType string    `gorm:"column:device_type"`
	DeviceID   uint      `gorm:"column:device_id"`
	OldStatus  string    `gorm:"column:old_status"`
	NewStatus  string    `gorm:"column:new_status"`
	Message    string    `gorm:"column:message"`
	CheckedAt  time.Time `gorm:"column:checked_at"`
}

func (DeviceStatusLog) TableName() string { return "device_status_log" }

type DeviceCheckSchedule struct {
	ID                uint       `gorm:"column:id;primaryKey"`
	Name              string     `gorm:"column:name;type:varchar(100);not null"`
	Enabled           bool       `gorm:"column:enabled;not null"`
	FrequencyPerDay   int        `gorm:"column:frequency_per_day;not null"`
	NotifyEnabled     bool       `gorm:"column:notify_enabled;not null"`
	PushConfigIDsJSON string     `gorm:"column:push_config_ids_json;type:text"`
	NotifyMode        string     `gorm:"column:notify_mode;type:varchar(30);not null"`
	LastRunAt         *time.Time `gorm:"column:last_run_at"`
	NextRunAt         *time.Time `gorm:"column:next_run_at"`
	LastSuccessAt     *time.Time `gorm:"column:last_success_at"`
	LastError         string     `gorm:"column:last_error;type:text"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (DeviceCheckSchedule) TableName() string { return "device_check_schedule" }

type DeviceCheckRun struct {
	ID            uint       `gorm:"column:id;primaryKey"`
	ScheduleID    *uint      `gorm:"column:schedule_id"`
	StartedAt     time.Time  `gorm:"column:started_at;not null"`
	FinishedAt    *time.Time `gorm:"column:finished_at"`
	Status        string     `gorm:"column:status;type:varchar(20);not null"`
	CheckedTotal  int        `gorm:"column:checked_total;not null"`
	OnlineTotal   int        `gorm:"column:online_total;not null"`
	OfflineTotal  int        `gorm:"column:offline_total;not null"`
	DisabledTotal int        `gorm:"column:disabled_total;not null"`
	ChangedTotal  int        `gorm:"column:changed_total;not null"`
	Notified      bool       `gorm:"column:notified;not null"`
	ErrorMessage  string     `gorm:"column:error_message;type:text"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
}

func (DeviceCheckRun) TableName() string { return "device_check_run" }

type DeviceCheckPushLog struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	ScheduleID   *uint     `gorm:"column:schedule_id"`
	RunID        *uint     `gorm:"column:run_id"`
	PushConfigID *uint     `gorm:"column:push_config_id"`
	Status       string    `gorm:"column:status;type:varchar(30);not null"`
	ConfigName   string    `gorm:"column:config_name;type:varchar(100)"`
	OfflineCount int       `gorm:"column:offline_count;not null"`
	Message      string    `gorm:"column:message;type:varchar(255)"`
	RequestBody  string    `gorm:"column:request_body;type:text"`
	ResponseBody string    `gorm:"column:response_body;type:text"`
	ErrorMessage string    `gorm:"column:error_message;type:text"`
	PushedAt     time.Time `gorm:"column:pushed_at;not null"`
}

func (DeviceCheckPushLog) TableName() string { return "device_check_push_log" }

type OperationLog struct {
	ID               uint      `gorm:"column:id;primaryKey"`
	TraceID          string    `gorm:"column:trace_id;type:varchar(64)"`
	Source           string    `gorm:"column:source;type:varchar(20)"`
	OperatorID       *uint     `gorm:"column:operator_id;type:int"`
	OperatorUsername string    `gorm:"column:operator_username;type:varchar(50)"`
	OperatorRealName string    `gorm:"column:operator_real_name;type:varchar(50)"`
	RoleCodes        string    `gorm:"column:role_codes;type:varchar(255)"`
	RoleNames        string    `gorm:"column:role_names;type:varchar(255)"`
	ClientIP         string    `gorm:"column:client_ip;type:varchar(64)"`
	IPLocation       string    `gorm:"column:ip_location;type:varchar(120)"`
	UserAgent        string    `gorm:"column:user_agent;type:text"`
	OSName           string    `gorm:"column:os_name;type:varchar(50)"`
	MenuCode         string    `gorm:"column:menu_code;type:varchar(100)"`
	MenuName         string    `gorm:"column:menu_name;type:varchar(150)"`
	RoutePath        string    `gorm:"column:route_path;type:varchar(255)"`
	PageTitle        string    `gorm:"column:page_title;type:varchar(150)"`
	PageComponent    string    `gorm:"column:page_component;type:varchar(100)"`
	ActionCode       string    `gorm:"column:action_code;type:varchar(150)"`
	ActionName       string    `gorm:"column:action_name;type:varchar(150)"`
	OperationType    string    `gorm:"column:operation_type;type:varchar(80)"`
	ObjectType       string    `gorm:"column:object_type;type:varchar(80)"`
	ObjectID         string    `gorm:"column:object_id;type:varchar(100)"`
	ObjectName       string    `gorm:"column:object_name;type:varchar(255)"`
	ObjectLocation   string    `gorm:"column:object_location;type:varchar(255)"`
	RequestMethod    string    `gorm:"column:request_method;type:varchar(10)"`
	RequestPath      string    `gorm:"column:request_path;type:varchar(255)"`
	RequestQuery     string    `gorm:"column:request_query;type:text"`
	RequestParams    string    `gorm:"column:request_params;type:longtext"`
	DevicePointInfo  string    `gorm:"column:device_point_info;type:longtext"`
	BeforeSnapshot   string    `gorm:"column:before_snapshot;type:longtext"`
	AfterSnapshot    string    `gorm:"column:after_snapshot;type:longtext"`
	ErrorStack       string    `gorm:"column:error_stack;type:longtext"`
	ResultStatus     string    `gorm:"column:result_status;type:varchar(20)"`
	ResponseStatus   int       `gorm:"column:response_status;type:int"`
	DurationMs       int64     `gorm:"column:duration_ms;type:bigint"`
	StoragePartition string    `gorm:"column:storage_partition;type:varchar(32)"`
	RetentionDays    int       `gorm:"column:retention_days;type:int"`
	ExtraJSON        string    `gorm:"column:extra_json;type:longtext"`
	OperationTime    time.Time `gorm:"column:operation_time;type:datetime(3);not null"`
	CreatedAt        time.Time `gorm:"column:created_at;type:datetime;not null"`
}

func (OperationLog) TableName() string { return "operation_log" }

type SystemSetting struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	SettingKey   string    `gorm:"column:setting_key;type:varchar(120);not null"`
	SettingName  string    `gorm:"column:setting_name;type:varchar(150);not null"`
	SettingValue string    `gorm:"column:setting_value;type:varchar(255);not null"`
	Remark       string    `gorm:"column:remark;type:varchar(255)"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (SystemSetting) TableName() string { return "system_setting" }

type PushConfig struct {
	ID                     uint      `gorm:"column:id;primaryKey"`
	ConfigName             string    `gorm:"column:config_name"`
	ProviderType           string    `gorm:"column:provider_type"`
	Webhook                string    `gorm:"column:webhook"`
	SecretEncrypted        string    `gorm:"column:secret_encrypted"`
	AppID                  string    `gorm:"column:app_id"`
	AppSecretEncrypted     string    `gorm:"column:app_secret_encrypted"`
	TemplateID             string    `gorm:"column:template_id"`
	ReceiverOpenIDsJSON    string    `gorm:"column:receiver_open_ids_json"`
	FactoryIDsJSON         string    `gorm:"column:factory_ids_json"`
	ZoneIDsJSON            string    `gorm:"column:zone_ids_json"`
	AlarmTypesJSON         string    `gorm:"column:alarm_types_json"`
	AlarmLevelsJSON        string    `gorm:"column:alarm_levels_json"`
	ActiveTimeRangesJSON   string    `gorm:"column:active_time_ranges_json"`
	Enabled                bool      `gorm:"column:enabled"`
	RateLimitWindowSeconds int       `gorm:"column:rate_limit_window_seconds"`
	RateLimitMaxCount      int       `gorm:"column:rate_limit_max_count"`
	RetryMaxCount          int       `gorm:"column:retry_max_count"`
	RetryIntervalSeconds   int       `gorm:"column:retry_interval_seconds"`
	Remark                 string    `gorm:"column:remark"`
	CreatedAt              time.Time `gorm:"column:created_at"`
	UpdatedAt              time.Time `gorm:"column:updated_at"`
}

func (PushConfig) TableName() string { return "push_config" }

type AlarmPushLog struct {
	ID           uint      `gorm:"column:id;primaryKey"`
	AlarmID      *uint     `gorm:"column:alarm_id"`
	PushConfigID *uint     `gorm:"column:push_config_id"`
	RetryOfLogID *uint     `gorm:"column:retry_of_log_id"`
	Channel      string    `gorm:"column:channel"`
	ProviderType string    `gorm:"column:provider_type"`
	Status       string    `gorm:"column:status"`
	ConfigName   string    `gorm:"column:config_name"`
	AlarmNo      string    `gorm:"column:alarm_no"`
	AlarmType    string    `gorm:"column:alarm_type"`
	AlarmLevel   string    `gorm:"column:alarm_level"`
	FactoryID    *uint     `gorm:"column:factory_id"`
	ZoneID       *uint     `gorm:"column:zone_id"`
	TriggeredBy  string    `gorm:"column:triggered_by"`
	RetryCount   int       `gorm:"column:retry_count"`
	Message      string    `gorm:"column:message"`
	RequestBody  string    `gorm:"column:request_body"`
	ResponseBody string    `gorm:"column:response_body"`
	ErrorMessage string    `gorm:"column:error_message"`
	PushedAt     time.Time `gorm:"column:pushed_at"`
}

func (AlarmPushLog) TableName() string { return "alarm_push_log" }

type SmartInterfaceProvider struct {
	ID               uint      `gorm:"column:id;primaryKey"`
	ProviderCode     string    `gorm:"column:provider_code"`
	ProviderName     string    `gorm:"column:provider_name"`
	ProviderType     string    `gorm:"column:provider_type"`
	AuthType         string    `gorm:"column:auth_type"`
	BaseURL          string    `gorm:"column:base_url"`
	CallbackPath     string    `gorm:"column:callback_path"`
	SecretEncrypted  string    `gorm:"column:secret_encrypted"`
	ConfigSchemaJSON string    `gorm:"column:config_schema_json"`
	Enabled          bool      `gorm:"column:enabled"`
	Remark           string    `gorm:"column:remark"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (SmartInterfaceProvider) TableName() string { return "smart_interface_provider" }

type SmartInterfaceCapability struct {
	ID                uint      `gorm:"column:id;primaryKey"`
	CapabilityCode    string    `gorm:"column:capability_code"`
	CapabilityName    string    `gorm:"column:capability_name"`
	EventCategory     string    `gorm:"column:event_category"`
	SupportsPush      bool      `gorm:"column:supports_push"`
	SupportsPull      bool      `gorm:"column:supports_pull"`
	SupportsAIReview  bool      `gorm:"column:supports_ai_review"`
	PayloadSchemaJSON string    `gorm:"column:payload_schema_json"`
	DefaultRuleJSON   string    `gorm:"column:default_rule_json"`
	Enabled           bool      `gorm:"column:enabled"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (SmartInterfaceCapability) TableName() string { return "smart_interface_capability" }

type SmartDeviceBinding struct {
	ID                   uint      `gorm:"column:id;primaryKey"`
	ProviderID           uint      `gorm:"column:provider_id"`
	CapabilityID         uint      `gorm:"column:capability_id"`
	SourceType           string    `gorm:"column:source_type"`
	SourceID             uint      `gorm:"column:source_id"`
	Enabled              bool      `gorm:"column:enabled"`
	Priority             int       `gorm:"column:priority"`
	ConnectionConfigJSON string    `gorm:"column:connection_config_json"`
	CreatedAt            time.Time `gorm:"column:created_at"`
	UpdatedAt            time.Time `gorm:"column:updated_at"`
}

func (SmartDeviceBinding) TableName() string { return "smart_device_binding" }

type SmartBindingRule struct {
	ID                    uint      `gorm:"column:id;primaryKey"`
	BindingID             uint      `gorm:"column:binding_id"`
	RuleName              string    `gorm:"column:rule_name"`
	Enabled               bool      `gorm:"column:enabled"`
	AlarmEnabled          bool      `gorm:"column:alarm_enabled"`
	AlarmLevel            string    `gorm:"column:alarm_level"`
	DedupWindowSeconds    int       `gorm:"column:dedup_window_seconds"`
	CooldownSeconds       int       `gorm:"column:cooldown_seconds"`
	MinConfidence         *float64  `gorm:"column:min_confidence"`
	ActiveTimePlanJSON    string    `gorm:"column:active_time_plan_json"`
	SnapshotEnabled       bool      `gorm:"column:snapshot_enabled"`
	RecordClipEnabled     bool      `gorm:"column:record_clip_enabled"`
	RecordPreSeconds      int       `gorm:"column:record_pre_seconds"`
	RecordPostSeconds     int       `gorm:"column:record_post_seconds"`
	PushEnabled           bool      `gorm:"column:push_enabled"`
	PushChannelsJSON      string    `gorm:"column:push_channels_json"`
	SendToAI              bool      `gorm:"column:send_to_ai"`
	AIFlowCode            string    `gorm:"column:ai_flow_code"`
	GenerateAlarmDirectly bool      `gorm:"column:generate_alarm_directly"`
	Remark                string    `gorm:"column:remark"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (SmartBindingRule) TableName() string { return "smart_binding_rule" }

type SmartBridgeReconnectLog struct {
	ID             uint       `gorm:"column:id;primaryKey"`
	TaskKey        string     `gorm:"column:task_key"`
	CycleKey       string     `gorm:"column:cycle_key"`
	TriggerReason  string     `gorm:"column:trigger_reason"`
	Action         string     `gorm:"column:action"`
	Status         string     `gorm:"column:status"`
	DeviceType     string     `gorm:"column:device_type"`
	DeviceID       uint       `gorm:"column:device_id"`
	SessionKey     string     `gorm:"column:session_key"`
	BindingIDsJSON string     `gorm:"column:binding_ids_json"`
	Attempt        int        `gorm:"column:attempt"`
	MaxAttempts    int        `gorm:"column:max_attempts"`
	NextRunAt      *time.Time `gorm:"column:next_run_at"`
	Detail         string     `gorm:"column:detail"`
	LastError      string     `gorm:"column:last_error"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

func (SmartBridgeReconnectLog) TableName() string { return "smart_bridge_reconnect_log" }

type SmartRawEvent struct {
	ID             uint      `gorm:"column:id;primaryKey"`
	ProviderID     uint      `gorm:"column:provider_id"`
	CapabilityID   *uint     `gorm:"column:capability_id"`
	BindingID      *uint     `gorm:"column:binding_id"`
	SourceType     string    `gorm:"column:source_type"`
	SourceID       *uint     `gorm:"column:source_id"`
	SourceEventID  string    `gorm:"column:source_event_id"`
	EventNo        string    `gorm:"column:event_no"`
	EventTime      time.Time `gorm:"column:event_time"`
	SignatureValid *bool     `gorm:"column:signature_valid"`
	HeadersJSON    string    `gorm:"column:headers_json"`
	RawPayloadJSON string    `gorm:"column:raw_payload_json"`
	ParseStatus    string    `gorm:"column:parse_status"`
	ParseError     string    `gorm:"column:parse_error"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (SmartRawEvent) TableName() string { return "smart_raw_event" }

type SmartEvent struct {
	ID                    uint      `gorm:"column:id;primaryKey"`
	RawEventID            *uint     `gorm:"column:raw_event_id"`
	BindingID             *uint     `gorm:"column:binding_id"`
	ProviderID            uint      `gorm:"column:provider_id"`
	CapabilityID          *uint     `gorm:"column:capability_id"`
	EventCode             string    `gorm:"column:event_code"`
	EventType             string    `gorm:"column:event_type"`
	EventLevel            string    `gorm:"column:event_level"`
	SourceStage           string    `gorm:"column:source_stage"`
	EventTime             time.Time `gorm:"column:event_time"`
	CameraID              *uint     `gorm:"column:camera_id"`
	RecorderID            *uint     `gorm:"column:recorder_id"`
	ChannelID             *uint     `gorm:"column:channel_id"`
	FactoryID             *uint     `gorm:"column:factory_id"`
	ZoneID                *uint     `gorm:"column:zone_id"`
	ImageURL              string    `gorm:"column:image_url"`
	VideoURL              string    `gorm:"column:video_url"`
	Confidence            *float64  `gorm:"column:confidence"`
	DedupKey              string    `gorm:"column:dedup_key"`
	NormalizedPayloadJSON string    `gorm:"column:normalized_payload_json"`
	Status                string    `gorm:"column:status"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

func (SmartEvent) TableName() string { return "smart_event" }

type AiReviewTask struct {
	ID                 uint       `gorm:"column:id;primaryKey"`
	SmartEventID       uint       `gorm:"column:smart_event_id"`
	TaskNo             string     `gorm:"column:task_no"`
	AIFlowCode         string     `gorm:"column:ai_flow_code"`
	ModelCode          string     `gorm:"column:model_code"`
	RequestPayloadJSON string     `gorm:"column:request_payload_json"`
	Status             string     `gorm:"column:status"`
	RetryCount         int        `gorm:"column:retry_count"`
	MaxRetryCount      int        `gorm:"column:max_retry_count"`
	SubmittedAt        time.Time  `gorm:"column:submitted_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at"`
	ErrorMessage       string     `gorm:"column:error_message"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
}

func (AiReviewTask) TableName() string { return "ai_review_task" }

type AiReviewResult struct {
	ID                uint      `gorm:"column:id;primaryKey"`
	TaskID            uint      `gorm:"column:task_id"`
	Decision          string    `gorm:"column:decision"`
	LabelsJSON        string    `gorm:"column:labels_json"`
	Confidence        *float64  `gorm:"column:confidence"`
	Reason            string    `gorm:"column:reason"`
	EvidenceJSON      string    `gorm:"column:evidence_json"`
	ResultPayloadJSON string    `gorm:"column:result_payload_json"`
	CreatedAt         time.Time `gorm:"column:created_at"`
}

func (AiReviewResult) TableName() string { return "ai_review_result" }
