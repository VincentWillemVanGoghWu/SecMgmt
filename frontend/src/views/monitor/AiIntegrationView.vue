<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, View } from '@element-plus/icons-vue'

import PageCard from '../../components/common/PageCard.vue'
import SearchForm from '../../components/common/SearchForm.vue'
import StatusTag from '../../components/common/StatusTag.vue'
import { listCamerasApi } from '../../api/camera'
import { listFactoriesApi, listZonesApi } from '../../api/master-data'
import { listPushConfigsApi } from '../../api/push'
import { listChannelsApi, listRecordersApi } from '../../api/recorder'
import {
  createSmartBindingApi,
  createSmartBindingRuleApi,
  createSmartProviderApi,
  deleteSmartBindingApi,
  deleteSmartBindingRuleApi,
  getSmartBridgeStatusApi,
  getSmartBindingDetailApi,
  getSmartEventDetailApi,
  listSmartBridgeReconnectLogsApi,
  listSmartBindingsApi,
  listSmartCapabilitiesApi,
  listSmartEventsApi,
  listSmartProvidersApi,
  reconnectSmartBindingApi,
  retrySmartAiTaskApi,
  testSmartBindingApi,
  testSmartProviderApi,
  updateSmartBindingApi,
  updateSmartBindingRuleApi,
  updateSmartProviderApi,
} from '../../api/smart-interface'
import type { CameraRecord } from '../../types/camera'
import type { FactoryRecord, ZoneRecord } from '../../types/master-data'
import type { RecorderChannelRecord, RecorderRecord } from '../../types/recorder'
import type { PushConfigRecord } from '../../types/push'
import type {
  SmartAiTaskRecord,
  SmartBindingDetailRecord,
  SmartBindingPageRecord,
  SmartBindingRecord,
  SmartBindingRuleRecord,
  SmartBindingTestResult,
  SmartBridgeStatusRecord,
  SmartBridgeReconnectLogPageRecord,
  SmartBridgeReconnectLogRecord,
  SmartCapabilityRecord,
  SmartEventDetailRecord,
  SmartEventPageRecord,
  SmartEventRecord,
  SmartProviderRecord,
} from '../../types/smart-interface'

type StatusTone = 'danger' | 'warning' | 'info' | 'success' | 'default'

const loading = ref(false)
const providerSubmitting = ref(false)
const bindingSubmitting = ref(false)
const ruleSubmitting = ref(false)
const providerTestingId = ref<number | null>(null)
const bindingTestingId = ref<number | null>(null)
const bindingReconnectingId = ref<number | null>(null)
const retryingTaskId = ref<number | null>(null)

const activeTab = ref<'providers' | 'bindings' | 'events' | 'reconnectLogs'>('providers')

const providers = ref<SmartProviderRecord[]>([])
const bridgeStatus = ref<SmartBridgeStatusRecord | null>(null)
const capabilities = ref<SmartCapabilityRecord[]>([])
const bindings = ref<SmartBindingRecord[]>([])
const events = ref<SmartEventRecord[]>([])
const reconnectLogs = ref<SmartBridgeReconnectLogRecord[]>([])
const bindingPage = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
})
const eventPage = reactive({
  page: 1,
  pageSize: 50,
  total: 0,
})
const reconnectLogPage = reactive({
  page: 1,
  pageSize: 50,
  total: 0,
})
const formOptionsLoaded = ref(false)
const eventsLoaded = ref(false)
const reconnectLogsLoaded = ref(false)
const eventUseRecentDefault = ref(true)
const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const cameras = ref<CameraRecord[]>([])
const recorders = ref<RecorderRecord[]>([])
const channels = ref<RecorderChannelRecord[]>([])
const pushConfigs = ref<PushConfigRecord[]>([])

const providerDialogVisible = ref(false)
const bindingDialogVisible = ref(false)
const bindingDetailVisible = ref(false)
const ruleDialogVisible = ref(false)
const eventDetailVisible = ref(false)

const editingProviderId = ref<number | null>(null)
const editingBindingId = ref<number | null>(null)
const editingRuleId = ref<number | null>(null)
const currentRuleBindingId = ref<number | null>(null)

const bindingDetail = ref<SmartBindingDetailRecord | null>(null)
const eventDetail = ref<SmartEventDetailRecord | null>(null)

const bindingQuery = reactive({
  sourceType: '',
  providerCode: '',
  capabilityCode: '',
  enabled: '',
})

const eventQuery = reactive({
  keyword: '',
  providerCode: '',
  capabilityCode: '',
  status: '',
  sourceStage: '',
})

const reconnectLogQuery = reactive({
  sessionKey: '',
  deviceType: '',
  triggerReason: '',
  action: '',
  status: '',
  range: [] as string[],
})

const providerForm = reactive({
  providerCode: '',
  providerName: '',
  providerType: 'http_callback',
  authType: 'none',
  baseUrl: '',
  callbackPath: '',
  secret: '',
  configSchemaText: '{\n  "headers": {}\n}',
  enabled: true,
  remark: '',
})

const bindingForm = reactive({
  providerCode: '',
  capabilityCode: '',
  sourceType: 'camera',
  sourceId: '',
  enabled: true,
  priority: 100,
  connectionConfigText: '{\n  "channelNo": 1\n}',
})

const ruleForm = reactive({
  ruleName: '',
  enabled: true,
  alarmEnabled: true,
  alarmLevel: 'medium',
  dedupWindowSeconds: 300,
  cooldownSeconds: 60,
  minConfidence: '0.8',
  snapshotEnabled: true,
  recordClipEnabled: false,
  recordPreSeconds: 5,
  recordPostSeconds: 10,
  pushEnabled: false,
  pushChannels: [] as string[],
  sendToAi: false,
  aiFlowCode: '',
  generateAlarmDirectly: true,
  remark: '',
})

const providerTypeOptions = [
  { label: '全部', value: '' },
  { label: 'HTTP 回调', value: 'http_callback' },
  { label: 'HTTP API', value: 'http_api' },
  { label: 'SDK 监听', value: 'sdk_listener' },
]

const authTypeOptions = [
  { label: '无鉴权', value: 'none' },
  { label: 'Token', value: 'token' },
  { label: 'HMAC', value: 'hmac' },
]

const enabledOptions = [
  { label: '全部', value: '' },
  { label: '启用', value: 'true' },
  { label: '停用', value: 'false' },
]

const pushConfigOptionValue = (id: number) => `push-config:${id}`

const sourceTypeOptions = [
  { label: '摄像机', value: 'camera' },
  { label: '录像机', value: 'recorder' },
  { label: '通道', value: 'channel' },
]

const eventStatusOptions = [
  { label: '全部', value: '' },
  { label: '已存储', value: 'stored' },
  { label: '规则过滤', value: 'filtered' },
  { label: 'AI待复核', value: 'ai_pending' },
  { label: 'AI已复核', value: 'ai_reviewed' },
  { label: 'AI已驳回', value: 'ai_rejected' },
  { label: 'AI失败', value: 'ai_failed' },
  { label: '已生成告警', value: 'alarm_generated' },
]

const sourceStageOptions = [
  { label: '全部', value: '' },
  { label: '原始事件', value: 'raw' },
  { label: 'AI复核', value: 'ai_reviewed' },
]

const reconnectReasonOptions = [
  { label: '全部', value: '' },
  { label: '离线后上线', value: 'offline_to_online' },
  { label: '持续离线巡检', value: 'offline_still' },
  { label: '上线转离线', value: 'online_to_offline' },
  { label: '手动重连', value: 'manual_binding_reconnect' },
]

const reconnectActionOptions = [
  { label: '全部', value: '' },
  { label: '已入队', value: 'queued' },
  { label: '已连接', value: 'already_connected' },
  { label: '重试已排程', value: 'retry_scheduled' },
  { label: '重连成功', value: 'success' },
  { label: '重连失败', value: 'failed' },
  { label: '关闭会话', value: 'closed' },
  { label: '无绑定', value: 'no_target' },
  { label: '解析失败', value: 'resolve_failed' },
  { label: '任务执行中', value: 'skip_active' },
  { label: '同轮跳过', value: 'skip_same_cycle' },
]

const reconnectStatusOptions = [
  { label: '全部', value: '' },
  { label: '等待中', value: 'pending' },
  { label: '执行中', value: 'running' },
  { label: '成功', value: 'success' },
  { label: '失败', value: 'failed' },
  { label: '已关闭', value: 'closed' },
  { label: '无绑定', value: 'no_target' },
  { label: '解析失败', value: 'resolve_failed' },
]

const levelToneMap: Record<string, StatusTone> = {
  critical: 'danger',
  high: 'danger',
  medium: 'warning',
  low: 'info',
}

const eventStatusMetaMap: Record<string, { text: string; tone: StatusTone }> = {
  stored: { text: '已存储', tone: 'success' },
  filtered: { text: '规则过滤', tone: 'info' },
  ai_pending: { text: 'AI待复核', tone: 'warning' },
  ai_reviewed: { text: 'AI已复核', tone: 'success' },
  ai_rejected: { text: 'AI已驳回', tone: 'default' },
  ai_failed: { text: 'AI失败', tone: 'danger' },
  alarm_generated: { text: '已生成告警', tone: 'danger' },
}

const sourceStageTextMap: Record<string, string> = {
  raw: '原始事件',
  ai_reviewed: 'AI复核',
}

const reconnectReasonTextMap: Record<string, string> = {
  offline_to_online: '离线后上线',
  offline_still: '持续离线巡检',
  online_to_offline: '上线转离线',
  manual_binding_reconnect: '手动重连',
}

const reconnectActionTextMap: Record<string, string> = {
  queued: '已入队',
  already_connected: '已连接',
  retry_scheduled: '重试已排程',
  success: '重连成功',
  failed: '重连失败',
  closed: '关闭会话',
  no_target: '无绑定',
  resolve_failed: '解析失败',
  skip_active: '任务执行中',
  skip_same_cycle: '同轮跳过',
}

const reconnectStatusMetaMap: Record<string, { text: string; tone: StatusTone }> = {
  pending: { text: '等待中', tone: 'warning' },
  running: { text: '执行中', tone: 'warning' },
  success: { text: '成功', tone: 'success' },
  failed: { text: '失败', tone: 'danger' },
  closed: { text: '已关闭', tone: 'default' },
  no_target: { text: '无绑定', tone: 'info' },
  resolve_failed: { text: '解析失败', tone: 'danger' },
}

const aiTaskStatusMetaMap: Record<string, { text: string; tone: StatusTone }> = {
  pending: { text: '待提交', tone: 'info' },
  queued: { text: '处理中', tone: 'warning' },
  success: { text: '已完成', tone: 'success' },
  failed: { text: '失败', tone: 'danger' },
}

const aiDecisionMetaMap: Record<string, { text: string; tone: StatusTone }> = {
  positive: { text: '阳性', tone: 'danger' },
  negative: { text: '阴性', tone: 'success' },
  uncertain: { text: '不确定', tone: 'warning' },
}

const parseStatusMetaMap: Record<string, { text: string; tone: StatusTone }> = {
  success: { text: '解析成功', tone: 'success' },
  failed: { text: '解析失败', tone: 'danger' },
}

const metrics = computed(() => [
  { label: '接口提供方', value: providers.value.length },
  { label: '绑定项', value: bindings.value.length },
  { label: 'Bridge会话', value: bridgeStatus.value?.bridge.sessionCount ?? 0 },
  { label: '重连任务', value: bridgeStatus.value?.reconnect.taskCount ?? 0 },
])

const sourceOptions = computed(() => {
  if (bindingForm.sourceType === 'camera') {
    return cameras.value.map((item) => ({ value: String(item.id), label: `${item.name} / ${item.deviceCode}` }))
  }
  if (bindingForm.sourceType === 'recorder') {
    return recorders.value.map((item) => ({ value: String(item.id), label: `${item.name} / ${item.deviceCode}` }))
  }
  return channels.value.map((item) => ({
    value: String(item.id),
    label: `${item.recorderName} / CH${String(item.channelNo).padStart(2, '0')} / ${item.name}`,
  }))
})

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === 'string' && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const formatDateTime = (value?: string | null) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false })
}

const formatJsonText = (value: unknown) => JSON.stringify(value ?? {}, null, 2)

const formatJsonBlock = (value: unknown) => {
  if (value == null || value === '') return '{}'
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value, null, 2)
}

const parseJsonText = (value: string, fallbackMessage: string) => {
  const text = value.trim()
  if (!text) return null
  try {
    return JSON.parse(text) as Record<string, unknown> | unknown[]
  } catch {
    throw new Error(fallbackMessage)
  }
}

const getEnabledText = (enabled: boolean) => (enabled ? '启用' : '停用')
const getEnabledTone = (enabled: boolean) => (enabled ? 'success' : 'default')
const getProviderTypeText = (value: string) => providerTypeOptions.find((item) => item.value === value)?.label ?? value
const formatPushProviderType = (value: string) => {
  switch (value) {
    case 'email':
      return '邮件'
    case 'wechat':
      return '微信'
    case 'dingtalk':
      return '钉钉'
    default:
      return value
  }
}
const pushConfigOptions = computed(() =>
  pushConfigs.value
    .map((item) => ({
      label: `${item.configName}（${formatPushProviderType(item.providerType)}${item.enabled ? '' : '，停用'}）`,
      value: pushConfigOptionValue(item.id),
      disabled: !item.enabled,
    })),
)
const getSourceTypeText = (value: string) => sourceTypeOptions.find((item) => item.value === value)?.label ?? value
const getEventLevelTone = (value: string) => levelToneMap[value] ?? 'info'
const getEventStatusTone = (value: string) => eventStatusMetaMap[value]?.tone ?? 'info'
const getEventStatusText = (value: string) => eventStatusMetaMap[value]?.text ?? value
const getSourceStageText = (value: string) => sourceStageTextMap[value] ?? value
const getReconnectReasonText = (value: string) => reconnectReasonTextMap[value] ?? value
const getReconnectActionText = (value: string) => reconnectActionTextMap[value] ?? value
const getReconnectStatusTone = (value: string) => reconnectStatusMetaMap[value]?.tone ?? 'info'
const getReconnectStatusText = (value: string) => reconnectStatusMetaMap[value]?.text ?? value
const getAiTaskStatusTone = (value: string) => aiTaskStatusMetaMap[value]?.tone ?? 'info'
const getAiTaskStatusText = (value: string) => aiTaskStatusMetaMap[value]?.text ?? value
const getAiDecisionTone = (value: string) => aiDecisionMetaMap[value]?.tone ?? 'info'
const getAiDecisionText = (value: string) => aiDecisionMetaMap[value]?.text ?? value
const getParseStatusTone = (value: string) => parseStatusMetaMap[value]?.tone ?? 'info'
const getParseStatusText = (value: string) => parseStatusMetaMap[value]?.text ?? value
const formatAttemptText = (item: SmartBridgeReconnectLogRecord) => {
  if (!item.maxAttempts) return String(item.attempt || 0)
  return `${item.attempt || 0}/${item.maxAttempts}`
}

const getDefaultBindingSourceId = (sourceType: string) => {
  if (sourceType === 'camera') return cameras.value[0] ? String(cameras.value[0].id) : ''
  if (sourceType === 'recorder') return recorders.value[0] ? String(recorders.value[0].id) : ''
  return channels.value[0] ? String(channels.value[0].id) : ''
}

const getDefaultConnectionConfigText = (sourceType: string) => {
  if (sourceType === 'recorder') {
    return '{\n  "matchMode": "recorder"\n}'
  }
  return '{\n  "channelNo": 1\n}'
}

const loadFormOptions = async () => {
  const [factoryList, zoneList, cameraList, recorderList, channelList, pushConfigList] = await Promise.all([
    listFactoriesApi(),
    listZonesApi(),
    listCamerasApi(),
    listRecordersApi(),
    listChannelsApi(),
    listPushConfigsApi(),
  ])
  factories.value = factoryList
  zones.value = zoneList
  cameras.value = cameraList
  recorders.value = recorderList
  channels.value = channelList
  pushConfigs.value = pushConfigList
  formOptionsLoaded.value = true
}

const ensureFormOptionsLoaded = async () => {
  if (formOptionsLoaded.value) return
  await loadFormOptions()
}

const loadCapabilities = async () => {
  capabilities.value = await listSmartCapabilitiesApi()
}

const loadProviders = async () => {
  providers.value = await listSmartProvidersApi()
}

const loadBridgeStatus = async () => {
  bridgeStatus.value = await getSmartBridgeStatusApi()
}

const loadBindings = async () => {
  const result: SmartBindingPageRecord = await listSmartBindingsApi({
    source_type: bindingQuery.sourceType || undefined,
    provider_code: bindingQuery.providerCode || undefined,
    capability_code: bindingQuery.capabilityCode || undefined,
    enabled: bindingQuery.enabled ? bindingQuery.enabled === 'true' : undefined,
    page: bindingPage.page,
    page_size: bindingPage.pageSize,
  })
  bindings.value = result.items
  bindingPage.total = result.total
  bindingPage.page = result.page
  bindingPage.pageSize = result.pageSize
}

const loadEvents = async () => {
  const result: SmartEventPageRecord = await listSmartEventsApi({
    keyword: eventQuery.keyword || undefined,
    provider_code: eventQuery.providerCode || undefined,
    capability_code: eventQuery.capabilityCode || undefined,
    status: eventQuery.status || undefined,
    source_stage: eventQuery.sourceStage || undefined,
    recent_days: eventUseRecentDefault.value ? 3 : undefined,
    page: eventPage.page,
    page_size: eventPage.pageSize,
  })
  events.value = result.items
  eventPage.total = result.total
  eventPage.page = result.page
  eventPage.pageSize = result.pageSize
}

const loadReconnectLogs = async () => {
  const [startAt, endAt] = reconnectLogQuery.range
  const result: SmartBridgeReconnectLogPageRecord = await listSmartBridgeReconnectLogsApi({
    session_key: reconnectLogQuery.sessionKey || undefined,
    device_type: reconnectLogQuery.deviceType || undefined,
    trigger_reason: reconnectLogQuery.triggerReason || undefined,
    action: reconnectLogQuery.action || undefined,
    status: reconnectLogQuery.status || undefined,
    start_at: startAt || undefined,
    end_at: endAt || undefined,
    page: reconnectLogPage.page,
    page_size: reconnectLogPage.pageSize,
  })
  reconnectLogs.value = result.items
  reconnectLogPage.total = result.total
  reconnectLogPage.page = result.page
  reconnectLogPage.pageSize = result.pageSize
}

const loadEventsWithFeedback = async () => {
  loading.value = true
  try {
    await loadEvents()
    eventsLoaded.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载事件流水失败'))
  } finally {
    loading.value = false
  }
}

const loadReconnectLogsWithFeedback = async () => {
  loading.value = true
  try {
    await loadReconnectLogs()
    reconnectLogsLoaded.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载重连日志失败'))
  } finally {
    loading.value = false
  }
}

const refreshEventsIfLoaded = async () => {
  if (!eventsLoaded.value) return
  await loadEvents()
}

const refreshReconnectLogsIfLoaded = async () => {
  if (!reconnectLogsLoaded.value) return
  await loadReconnectLogs()
}

const loadAll = async () => {
  loading.value = true
  try {
    await Promise.all([loadCapabilities(), loadProviders(), loadBindings(), loadBridgeStatus()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载智能接口数据失败'))
  } finally {
    loading.value = false
  }
}

const handleEventSearch = async () => {
  eventPage.page = 1
  eventUseRecentDefault.value = false
  await loadEventsWithFeedback()
}

const handleReconnectLogSearch = async () => {
  reconnectLogPage.page = 1
  await loadReconnectLogsWithFeedback()
}

const handleBindingSearch = async () => {
  bindingPage.page = 1
  await loadBindings()
}

const handleBindingPageChange = async (page: number) => {
  bindingPage.page = page
  await loadBindings()
}

const handleEventPageChange = async (page: number) => {
  eventPage.page = page
  await loadEventsWithFeedback()
}

const handleReconnectLogPageChange = async (page: number) => {
  reconnectLogPage.page = page
  await loadReconnectLogsWithFeedback()
}

const resetProviderForm = () => {
  editingProviderId.value = null
  providerForm.providerCode = ''
  providerForm.providerName = ''
  providerForm.providerType = 'http_callback'
  providerForm.authType = 'none'
  providerForm.baseUrl = ''
  providerForm.callbackPath = ''
  providerForm.secret = ''
  providerForm.configSchemaText = '{\n  "headers": {}\n}'
  providerForm.enabled = true
  providerForm.remark = ''
}

const resetBindingForm = () => {
  editingBindingId.value = null
  bindingForm.providerCode = providers.value[0]?.providerCode ?? ''
  bindingForm.capabilityCode = capabilities.value[0]?.capabilityCode ?? ''
  bindingForm.sourceType = 'camera'
  bindingForm.sourceId = getDefaultBindingSourceId('camera')
  bindingForm.enabled = true
  bindingForm.priority = 100
  bindingForm.connectionConfigText = getDefaultConnectionConfigText('camera')
}

watch(
  () => bindingForm.sourceType,
  (value) => {
    if (editingBindingId.value) return
    bindingForm.sourceId = getDefaultBindingSourceId(value)
    bindingForm.connectionConfigText = getDefaultConnectionConfigText(value)
  },
)

const resetRuleForm = () => {
  editingRuleId.value = null
  currentRuleBindingId.value = null
  ruleForm.ruleName = ''
  ruleForm.enabled = true
  ruleForm.alarmEnabled = true
  ruleForm.alarmLevel = 'medium'
  ruleForm.dedupWindowSeconds = 300
  ruleForm.cooldownSeconds = 60
  ruleForm.minConfidence = '0.8'
  ruleForm.snapshotEnabled = true
  ruleForm.recordClipEnabled = false
  ruleForm.recordPreSeconds = 5
  ruleForm.recordPostSeconds = 10
  ruleForm.pushEnabled = false
  ruleForm.pushChannels = []
  ruleForm.sendToAi = false
  ruleForm.aiFlowCode = ''
  ruleForm.generateAlarmDirectly = true
  ruleForm.remark = ''
}

const normalizeRulePushConfigSelections = (values: string[]) =>
  values.filter((value) => value.trim().toLowerCase().startsWith('push-config:'))

const openCreateProviderDialog = () => {
  resetProviderForm()
  providerDialogVisible.value = true
}

const openEditProviderDialog = (record: SmartProviderRecord) => {
  editingProviderId.value = record.id
  providerForm.providerCode = record.providerCode
  providerForm.providerName = record.providerName
  providerForm.providerType = record.providerType
  providerForm.authType = record.authType
  providerForm.baseUrl = record.baseUrl ?? ''
  providerForm.callbackPath = record.callbackPath ?? ''
  providerForm.secret = ''
  providerForm.configSchemaText = formatJsonText(record.configSchema ?? {})
  providerForm.enabled = record.enabled
  providerForm.remark = record.remark ?? ''
  providerDialogVisible.value = true
}

const submitProvider = async () => {
  providerSubmitting.value = true
  try {
    const payload = {
      providerCode: providerForm.providerCode.trim(),
      providerName: providerForm.providerName.trim(),
      providerType: providerForm.providerType,
      authType: providerForm.authType,
      baseUrl: providerForm.baseUrl.trim() || null,
      callbackPath: providerForm.callbackPath.trim() || null,
      secret: providerForm.secret.trim() || null,
      configSchema: parseJsonText(providerForm.configSchemaText, '提供方配置结构 JSON 格式不正确'),
      enabled: providerForm.enabled,
      remark: providerForm.remark.trim() || null,
    }
    if (!payload.providerCode || !payload.providerName) {
      throw new Error('请填写接口名称和提供方编码')
    }
    if (editingProviderId.value) {
      await updateSmartProviderApi(editingProviderId.value, payload)
      ElMessage.success('智能接口已更新')
    } else {
      await createSmartProviderApi(payload)
      ElMessage.success('智能接口已创建')
    }
    providerDialogVisible.value = false
    await loadProviders()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '保存智能接口失败'))
  } finally {
    providerSubmitting.value = false
  }
}

const handleTestProvider = async (record: SmartProviderRecord) => {
  providerTestingId.value = record.id
  try {
    const result = await testSmartProviderApi(record.id)
    ElMessage[result.success ? 'success' : 'warning'](`${result.message} (${formatDateTime(result.checkedAt)})`)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '测试连接失败'))
  } finally {
    providerTestingId.value = null
  }
}

const buildBindingTestMessage = (result: SmartBindingTestResult) => {
  const segments = [result.message]
  if (result.device?.message) {
    segments.push(`设备：${result.device.message}`)
  }
  if (result.provider?.message) {
    segments.push(`接口：${result.provider.message}`)
  }
  segments.push(formatDateTime(result.checkedAt))
  return segments.filter(Boolean).join(' | ')
}

const formatElapsed = (seconds?: number) => {
  if (seconds == null || Number.isNaN(seconds)) {
    return '-'
  }
  if (seconds < 60) {
    return `${seconds} 秒前`
  }
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) {
    return `${minutes} 分钟前`
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours} 小时前`
  }
  const days = Math.floor(hours / 24)
  return `${days} 天前`
}

const buildBindingSelfCheckMessage = (result: SmartBindingTestResult) => {
  const lines = [
    `结论：${result.message}`,
    `设备：${result.device.success ? '正常' : '异常'}，${result.device.message}`,
    `接口：${result.provider.success ? '正常' : '异常'}，${result.provider.message}`,
    `运行态：${result.runtime.success ? '正常' : '异常'}，${result.runtime.message}`,
    `规则：${result.rules.message}；告警规则 ${result.rules.alarmEnabledRuleCount} 条，直接告警 ${result.rules.directAlarmRuleCount} 条`,
  ]
  if (result.latestEvent.found) {
    lines.push(
      `最近事件：${result.latestEvent.code}，${result.latestEvent.eventType}，${formatDateTime(result.latestEvent.time)}，${formatElapsed(result.latestEvent.ageSeconds)}`,
    )
  } else {
    lines.push(`最近事件：${result.latestEvent.message}`)
  }
  if (result.latestAlarm.found) {
    lines.push(
      `最近告警：${result.latestAlarm.code}，${result.latestAlarm.alarmType}，${formatDateTime(result.latestAlarm.time)}，${formatElapsed(result.latestAlarm.ageSeconds)}`,
    )
  } else {
    lines.push(`最近告警：${result.latestAlarm.message}`)
  }
  if (result.runtime.supported) {
    lines.push(
      `Bridge：${result.runtime.running ? '在线' : '离线'}，会话${result.runtime.sessionFound ? '已命中' : '未命中'}${result.runtime.sessionKey ? ` (${result.runtime.sessionKey})` : ''}`,
    )
  }
  lines.push(`检查时间：${formatDateTime(result.checkedAt)}`)
  return lines.join('\n')
}

const handleTestBinding = async (record: SmartBindingRecord) => {
  bindingTestingId.value = record.id
  try {
    const result = await testSmartBindingApi(record.id)
    ElMessage[result.success ? 'success' : 'warning'](buildBindingTestMessage(result))
    await ElMessageBox.alert(
      h(
        'div',
        {
          style: {
            whiteSpace: 'pre-wrap',
            lineHeight: '1.7',
            wordBreak: 'break-word',
          },
        },
        buildBindingSelfCheckMessage(result),
      ),
      '绑定链路自检',
      { confirmButtonText: '知道了' },
    )
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '绑定自检失败'))
  } finally {
    bindingTestingId.value = null
  }
}

const handleReconnectBinding = async (record: SmartBindingRecord) => {
  bindingReconnectingId.value = record.id
  try {
    const result = await reconnectSmartBindingApi(record.id)
    ElMessage.success(result.message || '智能接口重连任务已提交')
    await Promise.all([loadBridgeStatus(), refreshReconnectLogsIfLoaded()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '提交智能接口重连失败'))
  } finally {
    bindingReconnectingId.value = null
  }
}

const openCreateBindingDialog = async () => {
  await ensureFormOptionsLoaded()
  resetBindingForm()
  bindingDialogVisible.value = true
}

const openEditBindingDialog = async (record: SmartBindingRecord) => {
  await ensureFormOptionsLoaded()
  editingBindingId.value = record.id
  bindingForm.providerCode = record.providerCode
  bindingForm.capabilityCode = record.capabilityCode
  bindingForm.sourceType = record.sourceType
  bindingForm.sourceId = String(record.sourceId)
  bindingForm.enabled = record.enabled
  bindingForm.priority = record.priority
  bindingForm.connectionConfigText = formatJsonText(record.connectionConfig ?? {})
  bindingDialogVisible.value = true
}

const submitBinding = async () => {
  bindingSubmitting.value = true
  try {
    if (!bindingForm.sourceId || !bindingForm.providerCode || !bindingForm.capabilityCode) {
      throw new Error('请完整选择提供方、能力和绑定对象')
    }
    const payload = {
      providerCode: bindingForm.providerCode,
      capabilityCode: bindingForm.capabilityCode,
      sourceType: bindingForm.sourceType,
      sourceId: Number(bindingForm.sourceId),
      enabled: bindingForm.enabled,
      priority: Number(bindingForm.priority),
      connectionConfig: parseJsonText(bindingForm.connectionConfigText, '连接配置 JSON 格式不正确'),
    }
    if (editingBindingId.value) {
      await updateSmartBindingApi(editingBindingId.value, payload)
      ElMessage.success('绑定项已更新')
    } else {
      await createSmartBindingApi(payload)
      ElMessage.success('绑定项已创建，并自动生成默认规则')
    }
    bindingDialogVisible.value = false
    await Promise.all([loadBindings(), loadProviders()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '保存绑定项失败'))
  } finally {
    bindingSubmitting.value = false
  }
}

const handleDeleteBinding = async (record: SmartBindingRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除绑定“${record.sourceName} / ${record.capabilityName}”吗？`, '删除绑定', {
      type: 'warning',
    })
    await deleteSmartBindingApi(record.id)
    ElMessage.success('绑定项已删除')
    await Promise.all([loadBindings(), loadProviders(), refreshEventsIfLoaded()])
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(resolveErrorMessage(error, '删除绑定项失败'))
    }
  }
}

const openBindingDetail = async (record: SmartBindingRecord) => {
  try {
    bindingDetail.value = await getSmartBindingDetailApi(record.id)
    bindingDetailVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载绑定详情失败'))
  }
}

const openCreateRuleDialog = async () => {
  if (!bindingDetail.value) return
  try {
    await ensureFormOptionsLoaded()
    resetRuleForm()
    currentRuleBindingId.value = bindingDetail.value.id
    ruleForm.ruleName = `${bindingDetail.value.capabilityName}规则`
    ruleForm.sendToAi = bindingDetail.value.sendToAi
    ruleForm.generateAlarmDirectly = bindingDetail.value.generateAlarmDirectly
    ruleDialogVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载推送配置失败'))
  }
}

const openEditRuleDialog = async (rule: SmartBindingRuleRecord) => {
  try {
    await ensureFormOptionsLoaded()
    editingRuleId.value = rule.id
    currentRuleBindingId.value = rule.bindingId
    ruleForm.ruleName = rule.ruleName
    ruleForm.enabled = rule.enabled
    ruleForm.alarmEnabled = rule.alarmEnabled
    ruleForm.alarmLevel = rule.alarmLevel
    ruleForm.dedupWindowSeconds = rule.dedupWindowSeconds
    ruleForm.cooldownSeconds = rule.cooldownSeconds
    ruleForm.minConfidence = rule.minConfidence == null ? '' : String(rule.minConfidence)
    ruleForm.snapshotEnabled = rule.snapshotEnabled
    ruleForm.recordClipEnabled = rule.recordClipEnabled
    ruleForm.recordPreSeconds = rule.recordPreSeconds
    ruleForm.recordPostSeconds = rule.recordPostSeconds
    ruleForm.pushEnabled = rule.pushEnabled
    ruleForm.pushChannels = normalizeRulePushConfigSelections(rule.pushChannels)
    ruleForm.sendToAi = rule.sendToAi
    ruleForm.aiFlowCode = rule.aiFlowCode ?? ''
    ruleForm.generateAlarmDirectly = rule.generateAlarmDirectly
    ruleForm.remark = rule.remark ?? ''
    ruleDialogVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载推送配置失败'))
  }
}

const submitRule = async () => {
  if (!currentRuleBindingId.value) return
  ruleSubmitting.value = true
  try {
    if (!ruleForm.ruleName.trim()) throw new Error('请输入规则名称')
    if (ruleForm.pushEnabled && ruleForm.pushChannels.length === 0) throw new Error('请选择推送配置')
    const payload = {
      ruleName: ruleForm.ruleName.trim(),
      enabled: ruleForm.enabled,
      alarmEnabled: ruleForm.alarmEnabled,
      alarmLevel: ruleForm.alarmLevel,
      dedupWindowSeconds: Number(ruleForm.dedupWindowSeconds),
      cooldownSeconds: Number(ruleForm.cooldownSeconds),
      minConfidence: ruleForm.minConfidence === '' ? null : Number(ruleForm.minConfidence),
      activeTimePlan: null,
      snapshotEnabled: ruleForm.snapshotEnabled,
      recordClipEnabled: ruleForm.recordClipEnabled,
      recordPreSeconds: Number(ruleForm.recordPreSeconds),
      recordPostSeconds: Number(ruleForm.recordPostSeconds),
      pushEnabled: ruleForm.pushEnabled,
      pushChannels: [...ruleForm.pushChannels],
      sendToAi: ruleForm.sendToAi,
      aiFlowCode: ruleForm.aiFlowCode.trim() || null,
      generateAlarmDirectly: ruleForm.generateAlarmDirectly,
      remark: ruleForm.remark.trim() || null,
    }
    if (editingRuleId.value) {
      await updateSmartBindingRuleApi(currentRuleBindingId.value, editingRuleId.value, payload)
      ElMessage.success('规则已更新')
    } else {
      await createSmartBindingRuleApi(currentRuleBindingId.value, payload)
      ElMessage.success('规则已新增')
    }
    ruleDialogVisible.value = false
    bindingDetail.value = await getSmartBindingDetailApi(currentRuleBindingId.value)
    await loadBindings()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '保存规则失败'))
  } finally {
    ruleSubmitting.value = false
  }
}

const handleDeleteRule = async (rule: SmartBindingRuleRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除规则“${rule.ruleName}”吗？`, '删除规则', { type: 'warning' })
    await deleteSmartBindingRuleApi(rule.bindingId, rule.id)
    ElMessage.success('规则已删除')
    bindingDetail.value = await getSmartBindingDetailApi(rule.bindingId)
    await loadBindings()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(resolveErrorMessage(error, '删除规则失败'))
    }
  }
}

const openEventDetail = async (record: SmartEventRecord) => {
  try {
    eventDetail.value = await getSmartEventDetailApi(record.id)
    eventDetailVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '加载事件详情失败'))
  }
}

const refreshEventDetail = async () => {
  if (!eventDetail.value) return
  eventDetail.value = await getSmartEventDetailApi(eventDetail.value.id)
}

const handleRetryAiTask = async (task: SmartAiTaskRecord) => {
  retryingTaskId.value = task.id
  try {
    await retrySmartAiTaskApi(task.id)
    ElMessage.success('AI 任务已重新入队')
    await Promise.all([refreshEventDetail(), refreshEventsIfLoaded()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, '重试 AI 任务失败'))
  } finally {
    retryingTaskId.value = null
  }
}

watch(
  () => activeTab.value,
  async (value) => {
    if (value === 'events' && !eventsLoaded.value) {
      await loadEventsWithFeedback()
    }
    if (value === 'reconnectLogs' && !reconnectLogsLoaded.value) {
      await loadReconnectLogsWithFeedback()
    }
  },
)

onMounted(async () => {
  await loadAll()
  resetBindingForm()
})
</script>

<template>
  <div class="smart-interface-page unified-list-page">
    <section class="smart-interface-page__metrics">
      <article v-for="item in metrics" :key="item.label" class="smart-interface-page__metric-card">
        <span>{{ item.label }}</span>
        <strong>{{ item.value }}</strong>
      </article>
    </section>

    <PageCard>
      <el-tabs v-model="activeTab" class="smart-interface-page__tabs">
        <el-tab-pane label="智能接口管理" name="providers">
          <div class="smart-interface-page__toolbar">
            <button class="app-button app-button--secondary" @click="openCreateProviderDialog">新增接口</button>
          </div>

          <table class="app-table smart-interface-page__table smart-interface-page__provider-table unified-list-page__table">
            <colgroup>
              <col class="smart-interface-page__provider-col-name" />
              <col class="smart-interface-page__provider-col-code" />
              <col class="smart-interface-page__provider-col-type" />
              <col class="smart-interface-page__provider-col-auth" />
              <col class="smart-interface-page__provider-col-callback" />
              <col class="smart-interface-page__provider-col-capability" />
              <col class="smart-interface-page__provider-col-status" />
              <col class="smart-interface-page__provider-col-time" />
              <col class="smart-interface-page__provider-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>接口名称</th>
                <th>提供方编码</th>
                <th>接入方式</th>
                <th>鉴权</th>
                <th>回调地址</th>
                <th>支持能力</th>
                <th>状态</th>
                <th>更新时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!providers.length">
                <td colspan="9" class="app-table__empty">{{ loading ? '加载中...' : '暂无接口配置' }}</td>
              </tr>
              <tr v-for="item in providers" :key="item.id">
                <td>{{ item.providerName }}</td>
                <td>{{ item.providerCode }}</td>
                <td>{{ getProviderTypeText(item.providerType) }}</td>
                <td>{{ item.authType }}</td>
                <td>{{ item.callbackPath || item.baseUrl || '-' }}</td>
                <td>{{ item.capabilityNames.length ? item.capabilityNames.join(' / ') : '-' }}</td>
                <td><StatusTag :text="getEnabledText(item.enabled)" :tone="getEnabledTone(item.enabled)" /></td>
                <td>{{ formatDateTime(item.updatedAt) }}</td>
                <td>
                  <div class="table-actions">
                    <button class="app-button app-button--secondary smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button" @click="openEditProviderDialog(item)">编辑</button>
                    <button
                      class="app-button app-button--primary smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button"
                      :disabled="providerTestingId === item.id"
                      @click="handleTestProvider(item)"
                    >
                      {{ providerTestingId === item.id ? '测试中...' : '测试连接' }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </el-tab-pane>

        <el-tab-pane label="接口绑定" name="bindings">
          <SearchForm class="smart-interface-page__search-form smart-interface-page__search-form--bindings unified-list-page__search-form">
            <div class="app-field">
              <select v-model="bindingQuery.sourceType" v-refresh-on-empty="handleBindingSearch">
                <option value="" hidden>绑定对象</option>
                <option value="">全部</option>
                <option v-for="item in sourceTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="bindingQuery.providerCode" v-refresh-on-empty="handleBindingSearch">
                <option value="" hidden>接口提供方</option>
                <option value="">全部</option>
                <option v-for="item in providers" :key="item.id" :value="item.providerCode">{{ item.providerName }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="bindingQuery.capabilityCode" v-refresh-on-empty="handleBindingSearch">
                <option value="" hidden>能力类型</option>
                <option value="">全部</option>
                <option v-for="item in capabilities" :key="item.id" :value="item.capabilityCode">{{ item.capabilityName }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="bindingQuery.enabled" v-refresh-on-empty="handleBindingSearch">
                <option value="" hidden>启用状态</option>
                <option v-for="item in enabledOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <template #actions>
              <button class="app-button app-button--primary smart-interface-page__button unified-list-page__button unified-list-page__search-button" @click="handleBindingSearch">
                <el-icon><Search /></el-icon>
                <span>查询</span>
              </button>
              <button class="app-button app-button--secondary smart-interface-page__button unified-list-page__button unified-list-page__search-button" @click="openCreateBindingDialog">新增绑定</button>
            </template>
          </SearchForm>

          <table class="app-table smart-interface-page__table smart-interface-page__binding-table unified-list-page__table">
            <colgroup>
              <col class="smart-interface-page__binding-col-source" />
              <col class="smart-interface-page__binding-col-provider" />
              <col class="smart-interface-page__binding-col-capability" />
              <col class="smart-interface-page__binding-col-priority" />
              <col class="smart-interface-page__binding-col-rule" />
              <col class="smart-interface-page__binding-col-ai" />
              <col class="smart-interface-page__binding-col-alarm" />
              <col class="smart-interface-page__binding-col-event" />
              <col class="smart-interface-page__binding-col-status" />
              <col class="smart-interface-page__binding-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>绑定对象</th>
                <th>接口提供方</th>
                <th>能力类型</th>
                <th>优先级</th>
                <th>规则数</th>
                <th>AI</th>
                <th>直接告警</th>
                <th>最近事件</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!bindings.length">
                <td colspan="10" class="app-table__empty">{{ loading ? '加载中...' : '暂无绑定项' }}</td>
              </tr>
              <tr v-for="item in bindings" :key="item.id">
                <td>
                  <div class="smart-interface-page__name-cell">
                    <strong>{{ item.sourceName }}</strong>
                    <span>{{ getSourceTypeText(item.sourceType) }}</span>
                  </div>
                </td>
                <td>{{ item.providerName }}</td>
                <td>{{ item.capabilityName }}</td>
                <td>{{ item.priority }}</td>
                <td>{{ item.ruleCount }}</td>
                <td><StatusTag :text="item.sendToAi ? '送AI' : '不送AI'" :tone="item.sendToAi ? 'success' : 'default'" /></td>
                <td>
                  <StatusTag
                    :text="item.generateAlarmDirectly ? '直接告警' : '规则判断'"
                    :tone="item.generateAlarmDirectly ? 'danger' : 'info'"
                  />
                </td>
                <td>{{ formatDateTime(item.lastEventTime) }}</td>
                <td><StatusTag :text="getEnabledText(item.enabled)" :tone="getEnabledTone(item.enabled)" /></td>
                <td>
                  <div class="table-actions">
                    <button class="app-button app-button--primary smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button" @click="openBindingDetail(item)">详情</button>
                    <button
                      class="app-button app-button--secondary smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button"
                      :disabled="bindingTestingId === item.id"
                      @click="handleTestBinding(item)"
                    >
                      {{ bindingTestingId === item.id ? '自检中...' : '自检' }}
                    </button>
                    <button
                      class="app-button app-button--warning smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button"
                      :disabled="bindingReconnectingId === item.id"
                      @click="handleReconnectBinding(item)"
                    >
                      {{ bindingReconnectingId === item.id ? '提交中...' : '重连' }}
                    </button>
                    <button class="app-button app-button--secondary smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button" @click="openEditBindingDialog(item)">编辑</button>
                    <button class="app-button app-button--danger smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button" @click="handleDeleteBinding(item)">
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <div class="smart-interface-page__pagination">
            <el-pagination
              background
              layout="total, prev, pager, next"
              :current-page="bindingPage.page"
              :page-size="bindingPage.pageSize"
              :total="bindingPage.total"
              @current-change="handleBindingPageChange"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="事件流水" name="events">
          <SearchForm class="smart-interface-page__search-form smart-interface-page__search-form--events unified-list-page__search-form">
            <div class="app-field">
              <ClearableSearchInput v-model="eventQuery.keyword" placeholder="事件编号 / 类型 / 去重键" @clear="handleEventSearch" />
            </div>
            <div class="app-field">
              <select v-model="eventQuery.providerCode" v-refresh-on-empty="handleEventSearch">
                <option value="" hidden>来源接口</option>
                <option value="">全部</option>
                <option v-for="item in providers" :key="item.id" :value="item.providerCode">{{ item.providerName }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="eventQuery.capabilityCode" v-refresh-on-empty="handleEventSearch">
                <option value="" hidden>能力类型</option>
                <option value="">全部</option>
                <option v-for="item in capabilities" :key="item.id" :value="item.capabilityCode">{{ item.capabilityName }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="eventQuery.sourceStage" v-refresh-on-empty="handleEventSearch">
                <option value="" hidden>事件阶段</option>
                <option v-for="item in sourceStageOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="eventQuery.status" v-refresh-on-empty="handleEventSearch">
                <option value="" hidden>状态</option>
                <option v-for="item in eventStatusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <template #actions>
              <button class="app-button app-button--primary smart-interface-page__button unified-list-page__button unified-list-page__search-button" @click="handleEventSearch">
                <el-icon><Search /></el-icon>
                <span>查询</span>
              </button>
            </template>
          </SearchForm>

          <table class="app-table smart-interface-page__table smart-interface-page__event-table unified-list-page__table">
            <colgroup>
              <col class="smart-interface-page__event-col-code" />
              <col class="smart-interface-page__event-col-provider" />
              <col class="smart-interface-page__event-col-capability" />
              <col class="smart-interface-page__event-col-source" />
              <col class="smart-interface-page__event-col-level" />
              <col class="smart-interface-page__event-col-confidence" />
              <col class="smart-interface-page__event-col-stage" />
              <col class="smart-interface-page__event-col-status" />
              <col class="smart-interface-page__event-col-time" />
              <col class="smart-interface-page__event-col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>事件编号</th>
                <th>来源接口</th>
                <th>能力类型</th>
                <th>来源对象</th>
                <th>事件等级</th>
                <th>置信度</th>
                <th>事件阶段</th>
                <th>状态</th>
                <th>时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!events.length">
                <td colspan="10" class="app-table__empty">{{ loading ? '加载中...' : '暂无事件流水' }}</td>
              </tr>
              <tr v-for="item in events" :key="item.id">
                <td>{{ item.eventCode }}</td>
                <td>{{ item.providerName }}</td>
                <td>{{ item.capabilityName }}</td>
                <td>
                  <div class="smart-interface-page__name-cell unified-list-page__name-cell">
                    <strong>{{ item.sourceName || '-' }}</strong>
                  </div>
                </td>
                <td><StatusTag :text="item.eventLevel" :tone="getEventLevelTone(item.eventLevel)" /></td>
                <td>{{ item.confidence ?? '-' }}</td>
                <td>{{ getSourceStageText(item.sourceStage) }}</td>
                <td><StatusTag :text="getEventStatusText(item.status)" :tone="getEventStatusTone(item.status)" /></td>
                <td>{{ formatDateTime(item.eventTime) }}</td>
                <td>
                  <div class="table-actions">
                    <button class="app-button app-button--primary smart-interface-page__button smart-interface-page__table-button unified-list-page__button unified-list-page__table-button" @click="openEventDetail(item)">
                      <el-icon><View /></el-icon>
                      <span>详情</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
          <div class="smart-interface-page__pagination">
            <el-pagination
              background
              layout="total, prev, pager, next"
              :current-page="eventPage.page"
              :page-size="eventPage.pageSize"
              :total="eventPage.total"
              @current-change="handleEventPageChange"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="重连日志" name="reconnectLogs">
          <SearchForm class="smart-interface-page__search-form smart-interface-page__search-form--reconnect unified-list-page__search-form">
            <div class="app-field">
              <ClearableSearchInput v-model="reconnectLogQuery.sessionKey" placeholder="Session Key" @clear="handleReconnectLogSearch" />
            </div>
            <div class="app-field">
              <select v-model="reconnectLogQuery.deviceType" v-refresh-on-empty="handleReconnectLogSearch">
                <option value="" hidden>设备类型</option>
                <option value="">全部</option>
                <option v-for="item in sourceTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="reconnectLogQuery.triggerReason" v-refresh-on-empty="handleReconnectLogSearch">
                <option value="" hidden>触发原因</option>
                <option v-for="item in reconnectReasonOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="reconnectLogQuery.action" v-refresh-on-empty="handleReconnectLogSearch">
                <option value="" hidden>动作</option>
                <option v-for="item in reconnectActionOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="app-field">
              <select v-model="reconnectLogQuery.status" v-refresh-on-empty="handleReconnectLogSearch">
                <option value="" hidden>状态</option>
                <option v-for="item in reconnectStatusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
              </select>
            </div>
            <div class="app-field">
              <el-date-picker
                v-model="reconnectLogQuery.range"
                type="datetimerange"
                value-format="YYYY-MM-DDTHH:mm:ss"
                start-placeholder="开始时间"
                end-placeholder="结束时间"
                range-separator="至"
                clearable
                @change="handleReconnectLogSearch"
              />
            </div>
            <template #actions>
              <button class="app-button app-button--primary smart-interface-page__button unified-list-page__button unified-list-page__search-button" @click="handleReconnectLogSearch">
                <el-icon><Search /></el-icon>
                <span>查询</span>
              </button>
            </template>
          </SearchForm>

          <table class="app-table smart-interface-page__table smart-interface-page__event-table smart-interface-page__reconnect-table unified-list-page__table">
            <colgroup>
              <col class="smart-interface-page__reconnect-col-time" />
              <col class="smart-interface-page__reconnect-col-device" />
              <col class="smart-interface-page__reconnect-col-session" />
              <col class="smart-interface-page__reconnect-col-reason" />
              <col class="smart-interface-page__reconnect-col-action" />
              <col class="smart-interface-page__reconnect-col-status" />
              <col class="smart-interface-page__reconnect-col-attempt" />
              <col class="smart-interface-page__reconnect-col-next" />
              <col class="smart-interface-page__reconnect-col-detail" />
              <col class="smart-interface-page__reconnect-col-error" />
            </colgroup>
            <thead>
              <tr>
                <th>时间</th>
                <th>设备</th>
                <th>Session</th>
                <th>触发原因</th>
                <th>动作</th>
                <th>状态</th>
                <th>尝试</th>
                <th>下次执行</th>
                <th>说明</th>
                <th>错误</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!reconnectLogs.length">
                <td colspan="10" class="app-table__empty">{{ loading ? '加载中...' : '暂无重连日志' }}</td>
              </tr>
              <tr v-for="item in reconnectLogs" :key="item.id">
                <td>{{ formatDateTime(item.createdAt) }}</td>
                <td>{{ getSourceTypeText(item.deviceType) }} #{{ item.deviceId }}</td>
                <td>{{ item.sessionKey || '-' }}</td>
                <td>{{ getReconnectReasonText(item.triggerReason) }}</td>
                <td>{{ getReconnectActionText(item.action) }}</td>
                <td><StatusTag :text="getReconnectStatusText(item.status)" :tone="getReconnectStatusTone(item.status)" /></td>
                <td>{{ formatAttemptText(item) }}</td>
                <td>{{ formatDateTime(item.nextRunAt) }}</td>
                <td>{{ item.detail || '-' }}</td>
                <td>{{ item.lastError || '-' }}</td>
              </tr>
            </tbody>
          </table>
          <div class="smart-interface-page__pagination">
            <el-pagination
              background
              layout="total, prev, pager, next"
              :current-page="reconnectLogPage.page"
              :page-size="reconnectLogPage.pageSize"
              :total="reconnectLogPage.total"
              @current-change="handleReconnectLogPageChange"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </PageCard>

    <el-dialog v-model="providerDialogVisible" :title="editingProviderId ? '编辑智能接口' : '新增智能接口'" width="760px">
      <div class="smart-interface-page__dialog-grid">
        <div class="app-field">
          <label>接口名称</label>
          <input v-model="providerForm.providerName" type="text" placeholder="如 海康 SDK 报警监听" />
        </div>
        <div class="app-field">
          <label>提供方编码</label>
          <input v-model="providerForm.providerCode" type="text" placeholder="如 hikvision-sdk" />
        </div>
        <div class="app-field">
          <label>接入方式</label>
          <select v-model="providerForm.providerType">
            <option v-for="item in providerTypeOptions.filter((item) => item.value)" :key="item.value" :value="item.value">
              {{ item.label }}
            </option>
          </select>
        </div>
        <div class="app-field">
          <label>鉴权方式</label>
          <select v-model="providerForm.authType">
            <option v-for="item in authTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>基础地址</label>
          <input v-model="providerForm.baseUrl" type="text" placeholder="可选，如 http://example.com/api" />
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>回调路径</label>
          <input v-model="providerForm.callbackPath" type="text" placeholder="如 /smart/events/ingest/hikvision-sdk" />
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>签名密钥</label>
          <input v-model="providerForm.secret" type="text" placeholder="留空表示不更新密钥" />
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>配置结构</label>
          <textarea v-model="providerForm.configSchemaText" rows="8" />
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>备注</label>
          <textarea v-model="providerForm.remark" rows="3" />
        </div>
        <label class="smart-interface-page__checkbox">
          <input v-model="providerForm.enabled" type="checkbox" />
          <span>启用该接口</span>
        </label>
      </div>
      <template #footer>
        <button class="app-button app-button--secondary" @click="providerDialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="providerSubmitting" @click="submitProvider">
          {{ providerSubmitting ? '保存中...' : '保存' }}
        </button>
      </template>
    </el-dialog>

    <el-dialog v-model="bindingDialogVisible" :title="editingBindingId ? '编辑绑定项' : '新增绑定项'" width="760px">
      <div class="smart-interface-page__dialog-grid">
        <div class="app-field">
          <label>绑定对象类型</label>
          <select v-model="bindingForm.sourceType">
            <option v-for="item in sourceTypeOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <label>绑定对象</label>
          <select v-model="bindingForm.sourceId">
            <option value="">请选择</option>
            <option v-for="item in sourceOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <label>接口提供方</label>
          <select v-model="bindingForm.providerCode">
            <option value="">请选择</option>
            <option v-for="item in providers" :key="item.id" :value="item.providerCode">{{ item.providerName }}</option>
          </select>
        </div>
        <div class="app-field">
          <label>能力类型</label>
          <select v-model="bindingForm.capabilityCode">
            <option value="">请选择</option>
            <option v-for="item in capabilities" :key="item.id" :value="item.capabilityCode">{{ item.capabilityName }}</option>
          </select>
        </div>
        <div class="app-field">
          <label>优先级</label>
          <input v-model.number="bindingForm.priority" type="number" min="0" step="1" />
        </div>
        <label class="smart-interface-page__checkbox smart-interface-page__checkbox--inline">
          <input v-model="bindingForm.enabled" type="checkbox" />
          <span>启用绑定</span>
        </label>
        <div class="app-field smart-interface-page__full-row">
          <label>连接配置</label>
          <textarea v-model="bindingForm.connectionConfigText" rows="8" />
        </div>
      </div>
      <template #footer>
        <button class="app-button app-button--secondary" @click="bindingDialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="bindingSubmitting" @click="submitBinding">
          {{ bindingSubmitting ? '保存中...' : '保存' }}
        </button>
      </template>
    </el-dialog>

    <el-dialog v-model="bindingDetailVisible" title="绑定详情" width="1080px">
      <div v-if="bindingDetail" class="smart-interface-page__detail-layout">
        <section class="smart-interface-page__detail-summary">
          <article>
            <span>绑定对象</span>
            <strong>{{ bindingDetail.sourceName }}</strong>
          </article>
          <article>
            <span>提供方</span>
            <strong>{{ bindingDetail.providerName }}</strong>
          </article>
          <article>
            <span>能力类型</span>
            <strong>{{ bindingDetail.capabilityName }}</strong>
          </article>
          <article>
            <span>最近事件</span>
            <strong>{{ formatDateTime(bindingDetail.lastEventTime) }}</strong>
          </article>
        </section>

        <section class="smart-interface-page__sub-card">
          <div class="smart-interface-page__sub-card-header">
            <h4>规则配置</h4>
            <button class="app-button app-button--secondary" @click="openCreateRuleDialog">新增规则</button>
          </div>
          <table class="app-table smart-interface-page__table">
            <thead>
              <tr>
                <th>规则名称</th>
                <th>启用</th>
                <th>告警等级</th>
                <th>送AI</th>
                <th>直接告警</th>
                <th>去重窗口</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!bindingDetail.rules.length">
                <td colspan="7" class="app-table__empty">暂无规则</td>
              </tr>
              <tr v-for="item in bindingDetail.rules" :key="item.id">
                <td>{{ item.ruleName }}</td>
                <td><StatusTag :text="getEnabledText(item.enabled)" :tone="getEnabledTone(item.enabled)" /></td>
                <td><StatusTag :text="item.alarmLevel" :tone="getEventLevelTone(item.alarmLevel)" /></td>
                <td>{{ item.sendToAi ? '是' : '否' }}</td>
                <td>{{ item.generateAlarmDirectly ? '是' : '否' }}</td>
                <td>{{ item.dedupWindowSeconds }} 秒</td>
                <td>
                  <div class="smart-interface-page__actions-cell">
                    <button class="app-button app-button--ghost" @click="openEditRuleDialog(item)">编辑</button>
                    <button class="app-button app-button--ghost smart-interface-page__danger" @click="handleDeleteRule(item)">
                      删除
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </section>

        <section class="smart-interface-page__sub-grid">
          <section class="smart-interface-page__sub-card">
            <h4>最近事件</h4>
            <table class="app-table smart-interface-page__table">
              <thead>
                <tr>
                  <th>事件编号</th>
                  <th>等级</th>
                  <th>状态</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!bindingDetail.recentEvents.length">
                  <td colspan="4" class="app-table__empty">暂无事件</td>
                </tr>
                <tr v-for="item in bindingDetail.recentEvents" :key="item.id">
                  <td>{{ item.eventCode }}</td>
                  <td>{{ item.eventLevel }}</td>
                  <td>{{ getEventStatusText(item.status) }}</td>
                  <td>{{ formatDateTime(item.eventTime) }}</td>
                </tr>
              </tbody>
            </table>
          </section>

          <section class="smart-interface-page__sub-card">
            <h4>最近告警</h4>
            <table class="app-table smart-interface-page__table">
              <thead>
                <tr>
                  <th>告警编号</th>
                  <th>等级</th>
                  <th>状态</th>
                  <th>时间</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="!bindingDetail.recentAlarms.length">
                  <td colspan="4" class="app-table__empty">暂无告警</td>
                </tr>
                <tr v-for="item in bindingDetail.recentAlarms" :key="item.id">
                  <td>{{ item.code }}</td>
                  <td>{{ item.level }}</td>
                  <td>{{ item.status }}</td>
                  <td>{{ formatDateTime(item.time) }}</td>
                </tr>
              </tbody>
            </table>
          </section>
        </section>
      </div>
    </el-dialog>

    <el-dialog v-model="ruleDialogVisible" :title="editingRuleId ? '编辑规则' : '新增规则'" width="760px">
      <div class="smart-interface-page__dialog-grid">
        <div class="app-field">
          <label>规则名称</label>
          <input v-model="ruleForm.ruleName" type="text" />
        </div>
        <div class="app-field">
          <label>告警等级</label>
          <select v-model="ruleForm.alarmLevel">
            <option value="critical">critical</option>
            <option value="high">high</option>
            <option value="medium">medium</option>
            <option value="low">low</option>
          </select>
        </div>
        <div class="app-field">
          <label>去重窗口秒数</label>
          <input v-model.number="ruleForm.dedupWindowSeconds" type="number" min="0" step="1" />
        </div>
        <div class="app-field">
          <label>冷却时间秒数</label>
          <input v-model.number="ruleForm.cooldownSeconds" type="number" min="0" step="1" />
        </div>
        <div class="app-field">
          <label>最低置信度</label>
          <input v-model="ruleForm.minConfidence" type="number" min="0" max="1" step="0.01" />
        </div>
        <div class="app-field">
          <label>AI 流程编码</label>
          <input v-model="ruleForm.aiFlowCode" type="text" placeholder="可选，如 motion-review-v1" />
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>推送配置</label>
          <el-select v-model="ruleForm.pushChannels" multiple clearable placeholder="请选择推送配置" style="width: 100%">
            <el-option
              v-for="item in pushConfigOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
              :disabled="item.disabled"
            />
          </el-select>
        </div>
        <div class="app-field smart-interface-page__full-row">
          <label>备注</label>
          <textarea v-model="ruleForm.remark" rows="3" />
        </div>
        <section class="smart-interface-page__check-grid smart-interface-page__full-row">
          <label class="smart-interface-page__checkbox"><input v-model="ruleForm.enabled" type="checkbox" /><span>启用规则</span></label>
          <label class="smart-interface-page__checkbox"><input v-model="ruleForm.alarmEnabled" type="checkbox" /><span>生成告警</span></label>
          <label class="smart-interface-page__checkbox"><input v-model="ruleForm.snapshotEnabled" type="checkbox" /><span>抓拍</span></label>
          <label class="smart-interface-page__checkbox"><input v-model="ruleForm.recordClipEnabled" type="checkbox" /><span>截取录像</span></label>
          <label class="smart-interface-page__checkbox"><input v-model="ruleForm.pushEnabled" type="checkbox" /><span>推送</span></label>
          <label class="smart-interface-page__checkbox"><input v-model="ruleForm.sendToAi" type="checkbox" /><span>送 AI</span></label>
          <label class="smart-interface-page__checkbox">
            <input v-model="ruleForm.generateAlarmDirectly" type="checkbox" />
            <span>直接生成告警</span>
          </label>
        </section>
      </div>
      <template #footer>
        <button class="app-button app-button--secondary" @click="ruleDialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="ruleSubmitting" @click="submitRule">
          {{ ruleSubmitting ? '保存中...' : '保存' }}
        </button>
      </template>
    </el-dialog>

    <el-dialog v-model="eventDetailVisible" title="事件详情" width="980px">
      <div v-if="eventDetail" class="smart-interface-page__detail-layout">
        <section class="smart-interface-page__detail-summary">
          <article>
            <span>事件编号</span>
            <strong>{{ eventDetail.eventCode }}</strong>
          </article>
          <article>
            <span>来源接口</span>
            <strong>{{ eventDetail.providerName }}</strong>
          </article>
          <article>
            <span>能力类型</span>
            <strong>{{ eventDetail.capabilityName }}</strong>
          </article>
          <article>
            <span>状态</span>
            <strong>{{ getEventStatusText(eventDetail.status) }}</strong>
          </article>
        </section>

        <section class="smart-interface-page__sub-grid">
          <section class="smart-interface-page__sub-card">
            <h4>标准事件</h4>
            <dl class="smart-interface-page__detail-list">
              <div><dt>来源对象</dt><dd>{{ eventDetail.sourceName || '-' }}</dd></div>
              <div><dt>事件类型</dt><dd>{{ eventDetail.eventType }}</dd></div>
              <div><dt>事件等级</dt><dd>{{ eventDetail.eventLevel }}</dd></div>
              <div><dt>事件阶段</dt><dd>{{ getSourceStageText(eventDetail.sourceStage) }}</dd></div>
              <div><dt>事件时间</dt><dd>{{ formatDateTime(eventDetail.eventTime) }}</dd></div>
              <div><dt>去重键</dt><dd>{{ eventDetail.dedupKey }}</dd></div>
              <div><dt>置信度</dt><dd>{{ eventDetail.confidence ?? '-' }}</dd></div>
            </dl>
          </section>

          <section class="smart-interface-page__sub-card">
            <h4>原始事件</h4>
            <dl v-if="eventDetail.rawEvent" class="smart-interface-page__detail-list">
              <div><dt>原始编号</dt><dd>{{ eventDetail.rawEvent.eventNo }}</dd></div>
              <div><dt>来源事件ID</dt><dd>{{ eventDetail.rawEvent.sourceEventId || '-' }}</dd></div>
              <div><dt>解析状态</dt><dd><StatusTag :text="getParseStatusText(eventDetail.rawEvent.parseStatus)" :tone="getParseStatusTone(eventDetail.rawEvent.parseStatus)" /></dd></div>
              <div><dt>签名校验</dt><dd>{{ eventDetail.rawEvent.signatureValid == null ? '-' : eventDetail.rawEvent.signatureValid ? '通过' : '失败' }}</dd></div>
              <div><dt>原始时间</dt><dd>{{ formatDateTime(eventDetail.rawEvent.eventTime) }}</dd></div>
              <div><dt>接收时间</dt><dd>{{ formatDateTime(eventDetail.rawEvent.createdAt) }}</dd></div>
            </dl>
            <div v-else class="smart-interface-page__empty-card">当前标准事件没有关联原始事件</div>
          </section>
        </section>

        <section class="smart-interface-page__sub-card">
          <h4>AI 复核任务</h4>
          <table class="app-table smart-interface-page__table">
            <thead>
              <tr>
                <th>任务编号</th>
                <th>流程编码</th>
                <th>状态</th>
                <th>重试</th>
                <th>最新结论</th>
                <th>提交时间</th>
                <th>完成时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!eventDetail.aiTasks.length">
                <td colspan="8" class="app-table__empty">暂无 AI 复核任务</td>
              </tr>
              <tr v-for="task in eventDetail.aiTasks" :key="task.id">
                <td>{{ task.taskNo }}</td>
                <td>{{ task.aiFlowCode }}</td>
                <td><StatusTag :text="getAiTaskStatusText(task.status)" :tone="getAiTaskStatusTone(task.status)" /></td>
                <td>{{ task.retryCount }} / {{ task.maxRetryCount }}</td>
                <td>
                  <StatusTag
                    v-if="task.latestResult"
                    :text="getAiDecisionText(task.latestResult.decision)"
                    :tone="getAiDecisionTone(task.latestResult.decision)"
                  />
                  <span v-else>-</span>
                </td>
                <td>{{ formatDateTime(task.submittedAt) }}</td>
                <td>{{ formatDateTime(task.finishedAt) }}</td>
                <td>
                  <button
                    class="app-button app-button--ghost"
                    :disabled="retryingTaskId === task.id || task.retryCount >= task.maxRetryCount"
                    @click="handleRetryAiTask(task)"
                  >
                    {{ retryingTaskId === task.id ? '重试中...' : '重试' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </section>

        <section class="smart-interface-page__sub-card">
          <h4>AI 复核结果</h4>
          <table class="app-table smart-interface-page__table">
            <thead>
              <tr>
                <th>任务ID</th>
                <th>结论</th>
                <th>标签</th>
                <th>置信度</th>
                <th>说明</th>
                <th>时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!eventDetail.aiResults.length">
                <td colspan="6" class="app-table__empty">暂无 AI 复核结果</td>
              </tr>
              <tr v-for="result in eventDetail.aiResults" :key="result.id">
                <td>{{ result.taskId }}</td>
                <td><StatusTag :text="getAiDecisionText(result.decision)" :tone="getAiDecisionTone(result.decision)" /></td>
                <td>{{ result.labels.length ? result.labels.join(', ') : '-' }}</td>
                <td>{{ result.confidence ?? '-' }}</td>
                <td>{{ result.reason || '-' }}</td>
                <td>{{ formatDateTime(result.createdAt) }}</td>
              </tr>
            </tbody>
          </table>
        </section>

        <section class="smart-interface-page__sub-grid">
          <section class="smart-interface-page__sub-card">
            <h4>告警结果</h4>
            <dl v-if="eventDetail.linkedAlarm" class="smart-interface-page__detail-list">
              <div><dt>告警编号</dt><dd>{{ eventDetail.linkedAlarm.code }}</dd></div>
              <div><dt>告警等级</dt><dd>{{ eventDetail.linkedAlarm.level }}</dd></div>
              <div><dt>告警状态</dt><dd>{{ eventDetail.linkedAlarm.status }}</dd></div>
              <div><dt>告警时间</dt><dd>{{ formatDateTime(eventDetail.linkedAlarm.time) }}</dd></div>
              <div><dt>说明</dt><dd>{{ eventDetail.linkedAlarm.message || '-' }}</dd></div>
            </dl>
            <div v-else class="smart-interface-page__empty-card">当前事件尚未关联告警</div>
          </section>

          <section class="smart-interface-page__sub-card">
            <h4>扩展信息</h4>
            <dl class="smart-interface-page__detail-list">
              <div>
                <dt>抓拍地址</dt>
                <dd>
                  <a
                    v-if="eventDetail.imageUrl"
                    :href="eventDetail.imageUrl"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="smart-interface-page__link"
                  >
                    {{ eventDetail.imageUrl }}
                  </a>
                  <template v-else>-</template>
                </dd>
              </div>
              <div>
                <dt>视频地址</dt>
                <dd v-if="eventDetail.videoUrl" class="smart-interface-page__detail-value">
                  <a
                    :href="eventDetail.videoUrl"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="smart-interface-page__link"
                  >
                    {{ eventDetail.videoUrl }}
                  </a>
                </dd>
                <dd v-else class="smart-interface-page__detail-value">
                  <span>-</span>
                </dd>
              </div>
              <div><dt>摄像机</dt><dd>{{ eventDetail.cameraName || '-' }}</dd></div>
              <div><dt>通道</dt><dd>{{ eventDetail.channelName || '-' }}</dd></div>
              <div><dt>厂区</dt><dd>{{ eventDetail.factoryName || '-' }}</dd></div>
              <div><dt>区域</dt><dd>{{ eventDetail.zoneName || '-' }}</dd></div>
            </dl>
          </section>
        </section>

        <section class="smart-interface-page__json-grid">
          <section class="smart-interface-page__sub-card">
            <h4>标准事件载荷</h4>
            <pre class="smart-interface-page__json-block">{{ formatJsonBlock(eventDetail.normalizedPayload) }}</pre>
          </section>
          <section class="smart-interface-page__sub-card">
            <h4>原始事件 JSON</h4>
            <pre class="smart-interface-page__json-block">{{ formatJsonBlock(eventDetail.rawEvent?.rawPayloadJson ?? eventDetail.rawJson) }}</pre>
          </section>
        </section>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.smart-interface-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.smart-interface-page__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.smart-interface-page__metric-card {
  padding: 16px 18px;
  border: 1px solid #d8e2ee;
  border-radius: 10px;
  background: linear-gradient(180deg, #ffffff, #f7fbff);
}

.smart-interface-page__metric-card span {
  display: block;
  color: #66788a;
  font-size: 13px;
}

.smart-interface-page__metric-card strong {
  display: block;
  margin-top: 8px;
  color: #17324d;
  font-size: 28px;
}

.smart-interface-page__tabs :deep(.el-tabs__header) {
  margin-bottom: 18px;
}

.smart-interface-page__toolbar {
  display: flex;
  justify-content: flex-end;
}

.smart-interface-page__search-form {
  margin-bottom: 16px;
}

.smart-interface-page__search-form :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.smart-interface-page__search-form :deep(.search-form__fields) {
  gap: 10px;
  align-items: end;
}

.smart-interface-page__search-form--bindings :deep(.search-form__fields) {
  grid-template-columns: repeat(4, minmax(160px, 1fr));
}

.smart-interface-page__search-form--events :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
}

.smart-interface-page__search-form--events :deep(.search-form__fields) {
  grid-template-columns: minmax(220px, 1.35fr) repeat(4, minmax(120px, 0.82fr));
  min-width: 0;
}

.smart-interface-page__search-form--events :deep(.search-form__fields > *) {
  min-width: 0;
}

.smart-interface-page__search-form--events :deep(.search-form__actions) {
  flex-wrap: nowrap;
  white-space: nowrap;
}

.smart-interface-page__search-form--reconnect :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) 92px;
  align-items: end;
}

.smart-interface-page__search-form--reconnect :deep(.search-form__fields) {
  grid-template-columns: minmax(190px, 1.45fr) repeat(4, minmax(96px, 0.58fr)) minmax(280px, 1.55fr);
  min-width: 0;
}

.smart-interface-page__search-form--reconnect :deep(.search-form__fields > *) {
  min-width: 0;
}

.smart-interface-page__search-form--reconnect :deep(.search-form__actions) {
  min-width: 92px;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.smart-interface-page__search-form--reconnect :deep(.unified-list-page__search-button) {
  width: 92px;
  justify-content: center;
  padding: 0 10px;
}

@media (max-width: 1440px) {
  .smart-interface-page__search-form--reconnect :deep(.search-form__fields) {
    grid-template-columns: repeat(3, minmax(160px, 1fr));
  }
}

.smart-interface-page__search-form :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.smart-interface-page__search-form :deep(.app-field input),
.smart-interface-page__search-form :deep(.app-field select),
.smart-interface-page__search-form :deep(.el-date-editor) {
  height: 36px;
  font-size: 13px;
}

.smart-interface-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.smart-interface-page__table-button {
  min-height: 30px;
  padding: 0 9px;
  font-size: 11px;
  line-height: 1;
  white-space: nowrap;
}

.smart-interface-page__table-button :deep(.el-icon) {
  font-size: 11px;
}

.smart-interface-page__table {
  margin-top: 16px;
}

.smart-interface-page__provider-table,
.smart-interface-page__binding-table,
.smart-interface-page__event-table,
.smart-interface-page__reconnect-table {
  table-layout: fixed;
}

.smart-interface-page__provider-table th,
.smart-interface-page__provider-table td,
.smart-interface-page__binding-table th,
.smart-interface-page__binding-table td,
.smart-interface-page__event-table th,
.smart-interface-page__event-table td,
.smart-interface-page__reconnect-table th,
.smart-interface-page__reconnect-table td {
  padding: 9px 10px;
  font-size: 12px;
  vertical-align: middle;
}

.smart-interface-page__provider-table th,
.smart-interface-page__binding-table th,
.smart-interface-page__event-table th,
.smart-interface-page__reconnect-table th {
  white-space: nowrap;
}

.smart-interface-page__provider-table td:nth-child(2),
.smart-interface-page__provider-table td:nth-child(3),
.smart-interface-page__provider-table td:nth-child(4),
.smart-interface-page__provider-table td:nth-child(7),
.smart-interface-page__provider-table td:nth-child(8),
.smart-interface-page__binding-table td:nth-child(4),
.smart-interface-page__binding-table td:nth-child(5),
.smart-interface-page__binding-table td:nth-child(6),
.smart-interface-page__binding-table td:nth-child(7),
.smart-interface-page__binding-table td:nth-child(8),
.smart-interface-page__binding-table td:nth-child(9),
.smart-interface-page__event-table td:nth-child(1),
.smart-interface-page__event-table td:nth-child(2),
.smart-interface-page__event-table td:nth-child(5),
.smart-interface-page__event-table td:nth-child(6),
.smart-interface-page__event-table td:nth-child(7),
.smart-interface-page__event-table td:nth-child(8),
.smart-interface-page__event-table td:nth-child(9),
.smart-interface-page__reconnect-table td:nth-child(1),
.smart-interface-page__reconnect-table td:nth-child(2),
.smart-interface-page__reconnect-table td:nth-child(4),
.smart-interface-page__reconnect-table td:nth-child(5),
.smart-interface-page__reconnect-table td:nth-child(6),
.smart-interface-page__reconnect-table td:nth-child(7),
.smart-interface-page__reconnect-table td:nth-child(8) {
  white-space: nowrap;
}

.smart-interface-page__provider-col-name {
  width: 120px;
}

.smart-interface-page__provider-col-code {
  width: 120px;
}

.smart-interface-page__provider-col-type {
  width: 82px;
}

.smart-interface-page__provider-col-auth {
  width: 68px;
}

.smart-interface-page__provider-col-callback {
  width: 180px;
}

.smart-interface-page__provider-col-capability {
  width: 128px;
}

.smart-interface-page__provider-col-status {
  width: 72px;
}

.smart-interface-page__provider-col-time {
  width: 132px;
}

.smart-interface-page__provider-col-actions {
  width: 118px;
}

.smart-interface-page__binding-col-source {
  width: 120px;
}

.smart-interface-page__binding-col-provider {
  width: 100px;
}

.smart-interface-page__binding-col-capability {
  width: 88px;
}

.smart-interface-page__binding-col-priority {
  width: 58px;
}

.smart-interface-page__binding-col-rule {
  width: 54px;
}

.smart-interface-page__binding-col-ai {
  width: 68px;
}

.smart-interface-page__binding-col-alarm {
  width: 82px;
}

.smart-interface-page__binding-col-event {
  width: 132px;
}

.smart-interface-page__binding-col-status {
  width: 72px;
}

.smart-interface-page__binding-col-actions {
  width: 270px;
}

.smart-interface-page__event-col-code {
  width: 248px;
}

.smart-interface-page__event-col-provider {
  width: 108px;
}

.smart-interface-page__event-col-capability {
  width: 82px;
}

.smart-interface-page__event-col-source {
  width: 92px;
}

.smart-interface-page__event-col-level {
  width: 72px;
}

.smart-interface-page__event-col-confidence {
  width: 68px;
}

.smart-interface-page__event-col-stage {
  width: 78px;
}

.smart-interface-page__event-col-status {
  width: 82px;
}

.smart-interface-page__event-col-time {
  width: 132px;
}

.smart-interface-page__event-col-actions {
  width: 72px;
}

.smart-interface-page__reconnect-col-time {
  width: 132px;
}

.smart-interface-page__reconnect-col-device {
  width: 96px;
}

.smart-interface-page__reconnect-col-session {
  width: 188px;
}

.smart-interface-page__reconnect-col-reason {
  width: 98px;
}

.smart-interface-page__reconnect-col-action {
  width: 94px;
}

.smart-interface-page__reconnect-col-status {
  width: 82px;
}

.smart-interface-page__reconnect-col-attempt {
  width: 58px;
}

.smart-interface-page__reconnect-col-next {
  width: 132px;
}

.smart-interface-page__reconnect-col-detail {
  width: 178px;
}

.smart-interface-page__reconnect-col-error {
  width: 178px;
}

.smart-interface-page__provider-table .table-actions,
.smart-interface-page__binding-table .table-actions,
.smart-interface-page__event-table .table-actions,
.smart-interface-page__reconnect-table .table-actions {
  flex-wrap: nowrap;
  gap: 4px;
}

.smart-interface-page__actions-cell {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.smart-interface-page__name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.smart-interface-page__name-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.smart-interface-page__name-cell span {
  color: #708195;
  font-size: 12px;
}

.smart-interface-page__danger {
  color: #c6404c;
}

.smart-interface-page__dialog-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.smart-interface-page__full-row {
  grid-column: 1 / -1;
}

.smart-interface-page__checkbox {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #30475f;
  font-size: 14px;
}

.smart-interface-page__checkbox--inline {
  margin-top: 30px;
}

.smart-interface-page__detail-layout {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.smart-interface-page__detail-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.smart-interface-page__detail-summary article,
.smart-interface-page__sub-card {
  padding: 14px 16px;
  border: 1px solid #d8e2ee;
  border-radius: 10px;
  background: #fbfdff;
}

.smart-interface-page__detail-summary span {
  display: block;
  color: #738396;
  font-size: 13px;
}

.smart-interface-page__detail-summary strong {
  display: block;
  margin-top: 8px;
  color: #17324d;
}

.smart-interface-page__sub-card h4 {
  margin: 0 0 12px;
  color: #17324d;
  font-size: 15px;
}

.smart-interface-page__sub-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.smart-interface-page__sub-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.smart-interface-page__json-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.smart-interface-page__detail-list {
  display: grid;
  gap: 10px;
}

.smart-interface-page__detail-list div {
  display: grid;
  grid-template-columns: 88px 1fr;
  gap: 12px;
}

.smart-interface-page__detail-list dt {
  color: #738396;
}

.smart-interface-page__detail-list dd {
  margin: 0;
  color: #17324d;
  word-break: break-all;
}

.smart-interface-page__detail-value {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.smart-interface-page__link {
  color: #2f7cf6;
  text-decoration: underline;
}

.smart-interface-page__link:hover {
  color: #1f66d1;
}

.smart-interface-page__json-block {
  margin: 0;
  max-height: 360px;
  overflow: auto;
  padding: 12px;
  border-radius: 8px;
  background: #0f2236;
  color: #d8e7f5;
  font-size: 12px;
  line-height: 1.6;
}

.smart-interface-page__check-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.smart-interface-page__empty-card {
  padding: 24px 12px;
  color: #738396;
  text-align: center;
}


textarea {
  width: 100%;
  resize: vertical;
}

@media (max-width: 1100px) {
  .smart-interface-page__metrics,
  .smart-interface-page__detail-summary,
  .smart-interface-page__sub-grid,
  .smart-interface-page__json-grid,
  .smart-interface-page__dialog-grid,
  .smart-interface-page__check-grid {
    grid-template-columns: 1fr;
  }

  .smart-interface-page__search-form--bindings :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }

  .smart-interface-page__checkbox--inline {
    margin-top: 0;
  }
}

@media (max-width: 768px) {
  .smart-interface-page__search-form--bindings :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
