export interface SmartProviderRecord {
  id: number
  providerCode: string
  providerName: string
  providerType: string
  authType: string
  baseUrl?: string | null
  callbackPath?: string | null
  enabled: boolean
  remark?: string | null
  configSchema?: Record<string, unknown> | unknown[] | null
  secretConfigured: boolean
  capabilityCodes: string[]
  capabilityNames: string[]
  updatedAt: string
  createdAt: string
}

export interface SmartProviderSubmitPayload {
  providerCode: string
  providerName: string
  providerType: string
  authType: string
  baseUrl?: string | null
  callbackPath?: string | null
  secret?: string | null
  configSchema?: Record<string, unknown> | unknown[] | null
  enabled: boolean
  remark?: string | null
}

export interface SmartProviderTestResult {
  success: boolean
  message: string
  checkedAt: string
}

export interface SmartBindingTestDeviceResult {
  sourceType: string
  sourceId: number
  sourceName: string
  sourcePath: string
  success: boolean
  message: string
  detail?: Record<string, unknown> | null
}

export interface SmartBindingTestProviderResult {
  id: number
  providerCode: string
  providerName: string
  success: boolean
  message: string
}

export interface SmartBindingRuntimeResult {
  supported: boolean
  success: boolean
  message: string
  bindingIncluded?: boolean
  running?: boolean
  sessionFound?: boolean
  sessionKey?: string | null
  deviceType?: string | null
  deviceId?: number | null
  deviceIp?: string | null
  lastError?: string | null
  session?: Record<string, unknown> | null
  status?: Record<string, unknown> | null
}

export interface SmartBindingRuleSummary {
  success: boolean
  message: string
  ruleCount: number
  enabledRuleCount: number
  alarmEnabledRuleCount: number
  directAlarmRuleCount: number
  sendToAiRuleCount: number
}

export interface SmartBindingLatestEvent {
  found: boolean
  message: string
  id?: number
  code?: string
  time?: string
  eventType?: string
  eventLevel?: string
  sourceStage?: string
  status?: string
  ageSeconds?: number
}

export interface SmartBindingLatestAlarm {
  found: boolean
  message: string
  id?: number
  code?: string
  time?: string
  alarmType?: string
  alarmLevel?: string
  status?: string
  ageSeconds?: number
}

export interface SmartBindingTestResult {
  success: boolean
  message: string
  checkedAt: string
  bindingEnabled: boolean
  providerEnabled: boolean
  capabilityCode: string
  capabilityName: string
  provider: SmartBindingTestProviderResult
  device: SmartBindingTestDeviceResult
  runtime: SmartBindingRuntimeResult
  rules: SmartBindingRuleSummary
  latestEvent: SmartBindingLatestEvent
  latestAlarm: SmartBindingLatestAlarm
}

export interface SmartCapabilityRecord {
  id: number
  capabilityCode: string
  capabilityName: string
  eventCategory: string
  supportsPush: boolean
  supportsPull: boolean
  supportsAiReview: boolean
  payloadSchema?: Record<string, unknown> | unknown[] | null
  defaultRule?: Record<string, unknown> | unknown[] | null
  enabled: boolean
  createdAt: string
}

export interface SmartBindingRuleRecord {
  id: number
  bindingId: number
  ruleName: string
  enabled: boolean
  alarmEnabled: boolean
  alarmLevel: string
  dedupWindowSeconds: number
  cooldownSeconds: number
  minConfidence?: number | null
  activeTimePlan?: Record<string, unknown> | unknown[] | null
  snapshotEnabled: boolean
  recordClipEnabled: boolean
  recordPreSeconds: number
  recordPostSeconds: number
  pushEnabled: boolean
  pushChannels: string[]
  sendToAi: boolean
  aiFlowCode?: string | null
  generateAlarmDirectly: boolean
  remark?: string | null
  createdAt: string
  updatedAt: string
}

export interface SmartBindingRuleSubmitPayload {
  ruleName: string
  enabled: boolean
  alarmEnabled: boolean
  alarmLevel: string
  dedupWindowSeconds: number
  cooldownSeconds: number
  minConfidence?: number | null
  activeTimePlan?: Record<string, unknown> | unknown[] | null
  snapshotEnabled: boolean
  recordClipEnabled: boolean
  recordPreSeconds: number
  recordPostSeconds: number
  pushEnabled: boolean
  pushChannels: string[]
  sendToAi: boolean
  aiFlowCode?: string | null
  generateAlarmDirectly: boolean
  remark?: string | null
}

export interface SmartBindingRecord {
  id: number
  providerId: number
  providerCode: string
  providerName: string
  capabilityId: number
  capabilityCode: string
  capabilityName: string
  sourceType: string
  sourceId: number
  sourceName: string
  sourcePath: string
  enabled: boolean
  priority: number
  connectionConfig?: Record<string, unknown> | unknown[] | null
  sendToAi: boolean
  generateAlarmDirectly: boolean
  ruleCount: number
  lastEventTime?: string | null
  updatedAt: string
  createdAt: string
}

export interface SmartBindingSubmitPayload {
  providerCode: string
  capabilityCode: string
  sourceType: string
  sourceId: number
  enabled: boolean
  priority: number
  connectionConfig?: Record<string, unknown> | unknown[] | null
}

export interface SmartEventLinkRecord {
  id: number
  code: string
  level: string
  status: string
  time: string
  message?: string | null
  imageUrl?: string | null
  videoUrl?: string | null
  recordStartTime?: string | null
  recordEndTime?: string | null
}

export interface SmartRawEventRecord {
  id: number
  providerCode: string
  providerName: string
  capabilityCode?: string | null
  capabilityName?: string | null
  bindingId?: number | null
  sourceType?: string | null
  sourceId?: number | null
  sourceEventId?: string | null
  eventNo: string
  eventTime: string
  signatureValid?: boolean | null
  parseStatus: string
  parseError?: string | null
  headersJson?: string | null
  rawPayloadJson: string
  createdAt: string
}

export interface SmartAiReviewResultRecord {
  id: number
  taskId: number
  decision: string
  labels: string[]
  confidence?: number | null
  reason?: string | null
  evidence?: Record<string, unknown> | unknown[] | null
  resultPayload?: Record<string, unknown> | unknown[] | string | null
  createdAt: string
}

export interface SmartAiTaskRecord {
  id: number
  taskNo: string
  smartEventId: number
  aiFlowCode: string
  modelCode?: string | null
  requestPayload?: Record<string, unknown> | unknown[] | string | null
  status: string
  retryCount: number
  maxRetryCount: number
  submittedAt: string
  finishedAt?: string | null
  errorMessage?: string | null
  createdAt: string
  latestResult?: SmartAiReviewResultRecord | null
}

export interface SmartEventRecord {
  id: number
  eventCode: string
  rawEventId?: number | null
  providerCode: string
  providerName: string
  capabilityCode: string
  capabilityName: string
  eventType: string
  eventLevel: string
  sourceStage: string
  eventTime: string
  bindingId?: number | null
  cameraId?: number | null
  cameraName?: string | null
  recorderId?: number | null
  recorderName?: string | null
  channelId?: number | null
  channelName?: string | null
  sourceName?: string | null
  factoryId?: number | null
  factoryName?: string | null
  zoneId?: number | null
  zoneName?: string | null
  imageUrl?: string | null
  videoUrl?: string | null
  confidence?: number | null
  status: string
  dedupKey: string
  rawJson: string
  normalizedPayload?: Record<string, unknown> | unknown[] | string | null
  createdAt: string
  linkedAlarm?: SmartEventLinkRecord | null
}

export interface SmartEventPageRecord {
  items: SmartEventRecord[]
  total: number
  page: number
  pageSize: number
}

export interface SmartEventDetailRecord extends SmartEventRecord {
  rawEvent?: SmartRawEventRecord | null
  aiTasks: SmartAiTaskRecord[]
  aiResults: SmartAiReviewResultRecord[]
}

export interface SmartBindingDetailRecord extends SmartBindingRecord {
  rules: SmartBindingRuleRecord[]
  recentEvents: SmartEventRecord[]
  recentAlarms: SmartEventLinkRecord[]
}

export interface SmartEventIngestResponse {
  accepted: boolean
  rawEventId: number
  smartEventId?: number | null
  aiTaskId?: number | null
  reason: string
}

export interface SmartAiReviewSubmitPayload {
  aiFlowCode: string
  modelCode?: string | null
  force: boolean
}

export interface SmartAiCallbackPayload {
  taskNo: string
  decision: string
  labels: string[]
  confidence?: number | null
  reason?: string | null
  evidence?: Record<string, unknown> | unknown[] | null
  raw?: Record<string, unknown> | unknown[] | string | null
}
