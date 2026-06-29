export interface OperationLogRecord {
  id: number
  operationTime: string
  operatorName: string
  operatorUsername: string
  roleNames: string
  menuPage: string
  menuName: string
  pageTitle: string
  actionName: string
  objectName: string
  operationType: string
  clientIp: string
  resultStatus: string
  durationMs: number
}

export interface OperationLogPageData {
  items: OperationLogRecord[]
  total: number
  page: number
  pageSize: number
}

export interface OperationLogDetail {
  id: number
  traceId: string
  source: string
  operatorId?: number | null
  operatorUsername: string
  operatorRealName: string
  roleCodes: string[]
  roleNames: string[]
  clientIp: string
  ipLocation: string
  userAgent: string
  osName: string
  menuCode: string
  menuName: string
  routePath: string
  pageTitle: string
  pageComponent: string
  actionCode: string
  actionName: string
  operationType: string
  objectType: string
  objectId: string
  objectName: string
  objectLocation: string
  requestMethod: string
  requestPath: string
  requestQuery: string
  requestParams: string
  devicePointInfo: string
  beforeSnapshot: string
  afterSnapshot: string
  errorStack: string
  resultStatus: string
  responseStatus: number
  durationMs: number
  storagePartition: string
  retentionDays: number
  extraJson: string
  operationTime: string
}

export interface OperationLogTrackPayload {
  source?: string
  menuCode?: string
  menuName?: string
  routePath?: string
  pageTitle?: string
  pageComponent?: string
  actionCode?: string
  actionName?: string
  operationType?: string
  objectType?: string
  objectId?: string
  objectName?: string
  objectLocation?: string
  requestParams?: string
  devicePointInfo?: string
  beforeSnapshot?: string
  afterSnapshot?: string
  errorStack?: string
  resultStatus?: string
  durationMs?: number
  extraJson?: string
}

export interface OperationStatsItem {
  name: string
  value: number
}

export interface OperationDashboardStats {
  todayCount: number
  successCount: number
  failedCount: number
  topUsers: OperationStatsItem[]
  topDevices: OperationStatsItem[]
  topActions: OperationStatsItem[]
}
