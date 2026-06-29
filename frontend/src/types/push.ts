export interface PushTimeRange {
  start: string
  end: string
}

export interface PushConfigRecord {
  id: number
  configName: string
  providerType: "dingtalk" | "wechat" | "email"
  webhook?: string | null
  appId?: string | null
  templateId?: string | null
  receiverOpenIds: string[]
  factoryIds: number[]
  zoneIds: number[]
  alarmTypes: string[]
  alarmLevels: string[]
  activeTimeRanges: PushTimeRange[]
  enabled: boolean
  rateLimitWindowSeconds: number
  rateLimitMaxCount: number
  retryMaxCount: number
  retryIntervalSeconds: number
  remark?: string | null
  secretConfigured: boolean
  appSecretConfigured: boolean
  createdAt: string
  updatedAt: string
}

export interface PushConfigSubmitPayload {
  configName: string
  providerType: "dingtalk" | "wechat" | "email"
  webhook?: string | null
  appId?: string | null
  templateId?: string | null
  receiverOpenIds: string[]
  factoryIds: number[]
  zoneIds: number[]
  alarmTypes: string[]
  alarmLevels: string[]
  activeTimeRanges: PushTimeRange[]
  enabled: boolean
  rateLimitWindowSeconds: number
  rateLimitMaxCount: number
  retryMaxCount: number
  retryIntervalSeconds: number
  remark?: string | null
  secret?: string
  appSecret?: string
}

export interface PushConfigTestResult {
  success: boolean
  status: string
  message: string
  pushedAt: string
}

export interface PushLogRecord {
  id: number
  alarmId?: number | null
  alarmNo?: string | null
  pushConfigId?: number | null
  configName?: string | null
  channel: string
  providerType: string
  status: string
  alarmType?: string | null
  alarmLevel?: string | null
  factoryId?: number | null
  factoryName?: string | null
  zoneId?: number | null
  zoneName?: string | null
  triggeredBy: string
  retryCount: number
  message: string
  requestBody?: string | null
  responseBody?: string | null
  errorMessage?: string | null
  pushedAt: string
}

export interface PushLogPageRecord {
  items: PushLogRecord[]
  total: number
  page: number
  pageSize: number
}
