package dto

import "time"

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginTokenData struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type AuthUser struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	RealName string `json:"realName"`
	Status   string `json:"status"`
}

type RoleInfo struct {
	ID       uint   `json:"id"`
	RoleCode string `json:"roleCode"`
	RoleName string `json:"roleName"`
	Status   string `json:"status"`
}

type DataScopeInfo struct {
	RoleCode   string  `json:"roleCode"`
	ScopeType  string  `json:"scopeType"`
	ScopeValue *string `json:"scopeValue,omitempty"`
}

type MenuItem struct {
	ID        uint       `json:"id,omitempty"`
	Key       string     `json:"key"`
	Label     string     `json:"label"`
	Icon      string     `json:"icon,omitempty"`
	RouteName string     `json:"routeName,omitempty"`
	Path      string     `json:"path,omitempty"`
	Children  []MenuItem `json:"children,omitempty"`
}

type MeData struct {
	User              AuthUser        `json:"user"`
	Roles             []RoleInfo      `json:"roles"`
	Menus             []MenuItem      `json:"menus"`
	ButtonPermissions []string        `json:"buttonPermissions"`
	DataScopes        []DataScopeInfo `json:"dataScopes"`
}

type FactoryRecord struct {
	ID          uint    `json:"id"`
	FactoryCode string  `json:"factoryCode"`
	FactoryName string  `json:"factoryName"`
	Status      string  `json:"status"`
	Remark      *string `json:"remark,omitempty"`
}

type ZoneRecord struct {
	ID          uint    `json:"id"`
	FactoryID   uint    `json:"factoryId"`
	FactoryName string  `json:"factoryName"`
	ZoneCode    string  `json:"zoneCode"`
	ZoneName    string  `json:"zoneName"`
	Status      string  `json:"status"`
	Remark      *string `json:"remark,omitempty"`
}

type DeptRecord struct {
	ID          uint    `json:"id"`
	DeptCode    string  `json:"deptCode"`
	DeptName    string  `json:"deptName"`
	ParentID    *uint   `json:"parentId,omitempty"`
	ParentName  *string `json:"parentName,omitempty"`
	FactoryID   *uint   `json:"factoryId,omitempty"`
	FactoryName *string `json:"factoryName,omitempty"`
	ZoneID      *uint   `json:"zoneId,omitempty"`
	ZoneName    *string `json:"zoneName,omitempty"`
	Leader      *string `json:"leader,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	Sort        int     `json:"sort"`
	Status      string  `json:"status"`
	Remark      *string `json:"remark,omitempty"`
}

type DictItemRecord struct {
	ID         uint    `json:"id"`
	DictTypeID uint    `json:"dictTypeId"`
	ItemLabel  string  `json:"itemLabel"`
	ItemValue  string  `json:"itemValue"`
	ItemSort   int     `json:"itemSort"`
	IsDefault  bool    `json:"isDefault"`
	Status     string  `json:"status"`
	Remark     *string `json:"remark,omitempty"`
}

type DictTypeRecord struct {
	ID       uint             `json:"id"`
	DictCode string           `json:"dictCode"`
	DictName string           `json:"dictName"`
	Status   string           `json:"status"`
	Remark   *string          `json:"remark,omitempty"`
	Items    []DictItemRecord `json:"items"`
}

type CameraRecord struct {
	ID                 uint    `json:"id"`
	DeviceCode         string  `json:"deviceCode"`
	Name               string  `json:"name"`
	IP                 string  `json:"ip"`
	SDKPort            int     `json:"sdkPort"`
	HTTPPort           int     `json:"httpPort"`
	RTSPPort           int     `json:"rtspPort"`
	Username           string  `json:"username"`
	FactoryID          uint    `json:"factoryId"`
	FactoryName        string  `json:"factoryName"`
	ZoneID             uint    `json:"zoneId"`
	ZoneName           string  `json:"zoneName"`
	InstallLocation    *string `json:"installLocation,omitempty"`
	SupportAI          bool    `json:"supportAi"`
	Status             string  `json:"status"`
	LastOnlineAt       *string `json:"lastOnlineAt,omitempty"`
	Remark             *string `json:"remark,omitempty"`
	PasswordConfigured bool    `json:"passwordConfigured"`
}

type RecorderRecord struct {
	ID                 uint    `json:"id"`
	DeviceCode         string  `json:"deviceCode"`
	Name               string  `json:"name"`
	IP                 string  `json:"ip"`
	SDKPort            int     `json:"sdkPort"`
	HTTPPort           int     `json:"httpPort"`
	Username           string  `json:"username"`
	ChannelCount       int     `json:"channelCount"`
	FactoryID          uint    `json:"factoryId"`
	FactoryName        string  `json:"factoryName"`
	Status             string  `json:"status"`
	LastOnlineAt       *string `json:"lastOnlineAt,omitempty"`
	PasswordConfigured bool    `json:"passwordConfigured"`
}

type RecorderChannelRecord struct {
	ID              uint    `json:"id"`
	RecorderID      uint    `json:"recorderId"`
	RecorderName    string  `json:"recorderName"`
	ChannelNo       int     `json:"channelNo"`
	Name            string  `json:"name"`
	CameraID        *uint   `json:"cameraId,omitempty"`
	CameraName      *string `json:"cameraName,omitempty"`
	FactoryID       uint    `json:"factoryId"`
	FactoryName     string  `json:"factoryName"`
	ZoneID          *uint   `json:"zoneId,omitempty"`
	ZoneName        *string `json:"zoneName,omitempty"`
	Enabled         bool    `json:"enabled"`
	SupportPlayback bool    `json:"supportPlayback"`
	Status          string  `json:"status"`
}

type AlarmRecord struct {
	ID              uint    `json:"id"`
	AlarmNo         string  `json:"alarmNo"`
	AIEventID       *uint   `json:"aiEventId,omitempty"`
	AlarmType       string  `json:"alarmType"`
	AlarmLevel      string  `json:"alarmLevel"`
	AlarmTime       string  `json:"alarmTime"`
	Status          string  `json:"status"`
	CameraID        *uint   `json:"cameraId,omitempty"`
	CameraName      *string `json:"cameraName,omitempty"`
	RecorderID      *uint   `json:"recorderId,omitempty"`
	RecorderName    *string `json:"recorderName,omitempty"`
	ChannelID       *uint   `json:"channelId,omitempty"`
	ChannelName     *string `json:"channelName,omitempty"`
	FactoryID       *uint   `json:"factoryId,omitempty"`
	FactoryName     *string `json:"factoryName,omitempty"`
	ZoneID          *uint   `json:"zoneId,omitempty"`
	ZoneName        *string `json:"zoneName,omitempty"`
	Message         *string `json:"message,omitempty"`
	ImageURL        *string `json:"imageUrl,omitempty"`
	VideoURL        *string `json:"videoUrl,omitempty"`
	RecordStartTime *string `json:"recordStartTime,omitempty"`
	RecordEndTime   *string `json:"recordEndTime,omitempty"`
	OccurrenceCount int     `json:"occurrenceCount"`
	LastEventTime   *string `json:"lastEventTime,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}

type AlarmRealtimePageRecord struct {
	Items    []AlarmRecord `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

type AlarmListFilter struct {
	Keyword     string
	Status      string
	Level       string
	AlarmType   string
	ExcludeDone bool
	StartAt     *time.Time
	EndAt       *time.Time
}

type AlarmPageRecord struct {
	Items    []AlarmRecord `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

type DashboardSummary struct {
	TodayAlarmCount     int64   `json:"todayAlarmCount"`
	PendingAlarmCount   int64   `json:"pendingAlarmCount"`
	CriticalAlarmCount  int64   `json:"criticalAlarmCount"`
	CameraOnlineRate    float64 `json:"cameraOnlineRate"`
	RecorderOnlineRate  float64 `json:"recorderOnlineRate"`
	PushSuccessRate     float64 `json:"pushSuccessRate"`
	CameraOnlineCount   int64   `json:"cameraOnlineCount"`
	CameraTotalCount    int64   `json:"cameraTotalCount"`
	RecorderOnlineCount int64   `json:"recorderOnlineCount"`
	RecorderTotalCount  int64   `json:"recorderTotalCount"`
}
