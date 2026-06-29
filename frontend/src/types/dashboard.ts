export interface DashboardSummary {
  todayAlarmCount: number
  pendingAlarmCount: number
  criticalAlarmCount: number
  cameraOnlineRate: number
  recorderOnlineRate: number
  pushSuccessRate: number
  cameraOnlineCount: number
  cameraTotalCount: number
  recorderOnlineCount: number
  recorderTotalCount: number
}

export interface ChartSeries {
  name: string
  data: number[]
}

export interface CategoryChart {
  categories: string[]
  series: ChartSeries[]
}

export interface NameValueItem {
  name: string
  value: number
}

export interface NameValueChart {
  items: NameValueItem[]
}

export interface OperationDashboardItem {
  name: string
  value: number
}

export interface OperationDashboardStats {
  todayCount: number
  successCount: number
  failedCount: number
  topUsers: OperationDashboardItem[]
  topDevices: OperationDashboardItem[]
  topActions: OperationDashboardItem[]
}

export interface ZoneRankingItem {
  factoryId?: number | null
  factoryName?: string | null
  zoneId?: number | null
  zoneName?: string | null
  alarmCount: number
  pendingCount: number
  criticalCount: number
}

export interface ZoneRankingChart {
  items: ZoneRankingItem[]
}

export interface ZoneRankingPage {
  items: ZoneRankingItem[]
  total: number
  page: number
  pageSize: number
}

export interface DeviceStatusBlock {
  deviceType: string
  total: number
  online: number
  offline: number
  exception: number
  disabled: number
  onlineRate: number
}

export interface DashboardDeviceStatus {
  camera: DeviceStatusBlock
  recorder: DeviceStatusBlock
  channel: DeviceStatusBlock
}

export interface AlarmStatusItem {
  name: string
  value: number
}

export interface AlarmReportData {
  summary: DashboardSummary
  trend: CategoryChart
  alarmTypes: NameValueChart
  statusSummary: AlarmStatusItem[]
  zoneRanking: ZoneRankingPage
}

export interface DeviceFactoryStat {
  factoryId: number
  factoryName: string
  cameraTotal: number
  cameraOnline: number
  recorderTotal: number
  recorderOnline: number
}

export interface DeviceFactoryStatPage {
  items: DeviceFactoryStat[]
  total: number
  page: number
  pageSize: number
}

export interface DeviceReportData {
  cameraStatus: DeviceStatusBlock
  recorderStatus: DeviceStatusBlock
  channelStatus: DeviceStatusBlock
  statusTrend: CategoryChart
  factoryStats: DeviceFactoryStatPage
}

export interface PushOverview {
  total: number
  success: number
  failed: number
  rateLimited: number
  successRate: number
}

export interface PushReportData {
  overview: PushOverview
  channelDistribution: NameValueChart
  statusDistribution: NameValueChart
  trend: CategoryChart
}

