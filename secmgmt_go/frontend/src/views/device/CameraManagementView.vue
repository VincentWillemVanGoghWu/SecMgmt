<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import {
  Connection,
  Delete,
  EditPen,
  Plus,
  RefreshRight,
  Search,
  SwitchButton,
  VideoPlay,
} from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import VideoPlayer from "../../components/video/VideoPlayer.vue"
import { checkAllDevicesStatusApi } from "../../api/device-status"
import {
  checkCameraStatusApi,
  createCameraApi,
  deleteCameraUserApi,
  deleteCameraPtzPresetApi,
  deleteCameraApi,
  fetchCameraDeviceIdentityApi,
  getCameraApi,
  getCameraBrowserLoginApi,
  getCameraSdkConfigApi,
  gotoCameraPtzPresetApi,
  listCamerasApi,
  controlCameraPtzZoomApi,
  setCameraPtzPresetApi,
  testCameraConnectionApi,
  updateCameraApi,
  updateCameraImageConfigApi,
  updateCameraNetworkConfigApi,
  updateCameraPtzConfigApi,
  updateCameraRecordingConfigApi,
  updateCameraStatusApi,
  upsertCameraUserApi,
} from "../../api/camera"
import { listFactoriesApi, listZonesApi } from "../../api/master-data"
import { getLiveWebControlConfigApi } from "../../api/video"
import type {
  CameraConnectionTestData,
  CameraImageConfig,
  CameraPtzConfig,
  CameraPtzPreset,
  CameraRecord,
  CameraRecordingScheduleDay,
  CameraRecordingConfig,
  CameraSdkConfig,
  CameraSubmitPayload,
  CameraNetworkConfig,
  CameraUserAccount,
} from "../../types/camera"
import type { FactoryRecord, ZoneRecord } from "../../types/master-data"

const DEFAULT_DEVICE_PASSWORD = "bhcd2017"
import type { LiveWebControlConfig } from "../../types/video"

interface CameraFormState extends CameraSubmitPayload {
  password: string
}

type CameraConfigTabName = "basic" | "network" | "image" | "recording" | "ptz" | "users"

const loading = ref(false)
const submitting = ref(false)
const configLoading = ref(false)
const networkSubmitting = ref(false)
const imageSubmitting = ref(false)
const recordingSubmitting = ref(false)
const ptzSubmitting = ref(false)
const userSubmitting = ref(false)
const fetchingDeviceIdentity = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const testingId = ref<number | null>(null)
const checkingId = ref<number | null>(null)
const checkingAll = ref(false)
const previewLoading = ref(false)
const previewDialogVisible = ref(false)
const zoomSubmitting = ref<"in" | "out" | null>(null)
const formRef = ref<FormInstance>()
const activeConfigTab = ref<CameraConfigTabName>("basic")

const records = ref<CameraRecord[]>([])
const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const selectedPreviewId = ref<number | null>(null)
const lastConnectionResult = ref<(CameraConnectionTestData & { cameraId: number }) | null>(null)
const previewWebControlConfig = ref<LiveWebControlConfig | null>(null)
const previewStreamCameraId = ref<number | null>(null)
const previewStreamMessage = ref("")
const cameraSdkConfig = ref<CameraSdkConfig | null>(null)
const networkForm = reactive({
  ip: "",
  subnetMask: "",
  gateway: "",
  primaryDns: "",
  secondaryDns: "",
  dhcpEnabled: false,
})
const imageForm = reactive({
  resolution: "",
  frameRate: 25,
  bitrate: 2048,
  exposureMode: "auto",
  exposureTime: "",
  whiteBalanceMode: "auto",
})
const recordingForm = reactive({
  scheduleMode: "",
  storageMode: "sdCard",
  overwriteEnabled: true,
})
const recordingWeeklyPlan = ref<CameraRecordingScheduleDay[]>([])
const ptzPresetForm = reactive({
  presetId: 1,
  name: "",
})
const selectedPtzPresetId = ref<number | null>(null)
const ptzModeForm = reactive({
  cruiseEnabled: false,
  trackEnabled: false,
})
const userForm = reactive({
  userId: null as number | null,
  username: "",
  password: "",
  role: "operator",
  enabled: true,
})
const selectedUserId = ref<number | null>(null)
const configTabs = [
  { label: "基础信息", name: "basic" as const },
  { label: "网络", name: "network" as const },
  { label: "图像", name: "image" as const },
  { label: "录像", name: "recording" as const },
  { label: "云台", name: "ptz" as const },
  { label: "用户", name: "users" as const },
]

const resolutionOptions = [
  "3840x2160",
  "3072x1728",
  "2688x1520",
  "2560x1440",
  "1920x1080",
  "1280x720",
  "704x576",
  "640x480",
]

const exposureModeOptions = [
  { label: "自动", value: "auto" },
  { label: "手动", value: "manual" },
  { label: "光圈优先", value: "irisPriority" },
  { label: "快门优先", value: "shutterPriority" },
]

const whiteBalanceOptions = [
  { label: "自动", value: "auto" },
  { label: "手动", value: "manual" },
  { label: "锁定", value: "locked" },
  { label: "室内", value: "indoor" },
  { label: "室外", value: "outdoor" },
]

const storageModeOptions = [
  { label: "SD卡", value: "sdCard" },
  { label: "NAS", value: "nas" },
  { label: "本地磁盘", value: "localDisk" },
  { label: "自动", value: "auto" },
]

const recordTypeOptions = [
  { label: "全天录像", value: "CMR" },
  { label: "移动侦测", value: "MOTION" },
  { label: "报警触发", value: "ALARM" },
  { label: "智能事件", value: "SMART" },
]

const weekdayLabels: Record<string, string> = {
  monday: "周一",
  tuesday: "周二",
  wednesday: "周三",
  thursday: "周四",
  friday: "周五",
  saturday: "周六",
  sunday: "周日",
}

const userRoleOptions = [
  { label: "管理员", value: "admin" },
  { label: "操作员", value: "operator" },
  { label: "查看员", value: "viewer" },
]

const statusOptions = [
  { label: "在线", value: "online" },
  { label: "离线", value: "offline" },
  { label: "异常", value: "exception" },
  { label: "停用", value: "disabled" },
]

const supportAiOptions = [
  { label: "全部", value: "" },
  { label: "支持 AI", value: "true" },
  { label: "不支持 AI", value: "false" },
]

const queryForm = reactive({
  keyword: "",
  factoryId: "",
  zoneId: "",
  status: "",
  supportAi: "",
})

const formState = reactive<CameraFormState>({
  deviceCode: "",
  name: "",
  ip: "",
  sdkPort: 8000,
  httpPort: 80,
  rtspPort: 554,
  username: "admin",
  password: DEFAULT_DEVICE_PASSWORD,
  factoryId: 0,
  zoneId: 0,
  installLocation: "",
  supportAi: true,
  status: "offline",
  remark: "",
})

const rules: FormRules<CameraFormState> = {
  deviceCode: [{ required: true, message: "请输入设备编码", trigger: "blur" }],
  name: [{ required: true, message: "请输入摄像机名称", trigger: "blur" }],
  ip: [{ required: true, message: "请输入设备 IP", trigger: "blur" }],
  username: [{ required: true, message: "请输入登录账号", trigger: "blur" }],
  factoryId: [{ required: true, message: "请选择所属厂区", trigger: "change" }],
  zoneId: [{ required: true, message: "请选择所属区域", trigger: "change" }],
  password: [
    {
      validator: (_, value, callback) => {
        if (!editingId.value && !String(value || "").trim()) {
          callback(new Error("请输入设备密码"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
}

const queryZoneOptions = computed(() => {
  if (!queryForm.factoryId) {
    return zones.value
  }
  return zones.value.filter((item) => item.factoryId === Number(queryForm.factoryId))
})

const formZoneOptions = computed(() => zones.value.filter((item) => item.factoryId === formState.factoryId))
const currentTabSupported = computed(() => {
  if (!cameraSdkConfig.value) {
    return true
  }
  if (activeConfigTab.value === "network") {
    return cameraSdkConfig.value.network.supported
  }
  if (activeConfigTab.value === "image") {
    return cameraSdkConfig.value.image.supported
  }
  if (activeConfigTab.value === "recording") {
    return cameraSdkConfig.value.recording.supported
  }
  if (activeConfigTab.value === "ptz") {
    return cameraSdkConfig.value.ptz.supported
  }
  if (activeConfigTab.value === "users") {
    return cameraSdkConfig.value.users.supported
  }
  return true
})

const currentTabMessage = computed(() => {
  if (!cameraSdkConfig.value) {
    return ""
  }
  if (activeConfigTab.value === "network") {
    return cameraSdkConfig.value.network.message || ""
  }
  if (activeConfigTab.value === "image") {
    return cameraSdkConfig.value.image.message || ""
  }
  if (activeConfigTab.value === "recording") {
    return cameraSdkConfig.value.recording.message || ""
  }
  if (activeConfigTab.value === "ptz") {
    return cameraSdkConfig.value.ptz.message || ""
  }
  if (activeConfigTab.value === "users") {
    return cameraSdkConfig.value.users.message || ""
  }
  return ""
})
const ptzPresets = computed<CameraPtzPreset[]>(() => cameraSdkConfig.value?.ptz.presets ?? [])
const cameraUsers = computed<CameraUserAccount[]>(() => cameraSdkConfig.value?.users.items ?? [])

const selectedPreviewCamera = computed(
  () => records.value.find((item) => item.id === selectedPreviewId.value) ?? null,
)

const selectedPreviewConfig = computed(() =>
  selectedPreviewCamera.value && previewStreamCameraId.value === selectedPreviewCamera.value.id ? previewWebControlConfig.value : null,
)

const selectedPreviewUrl = computed(() => null)

const selectedPreviewStreamType = computed(() => "hik-sdk" as const)

const selectedPreviewStreamProfile = computed(() => selectedPreviewConfig.value?.streamProfile ?? "main")

const selectedPreviewConnectionMode = computed(() => "hik-sdk" as const)

const selectedPreviewDiagnosticMessage = computed(() =>
  selectedPreviewCamera.value && previewStreamCameraId.value === selectedPreviewCamera.value.id
    ? selectedPreviewConfig.value?.message ?? null
    : null,
)

const selectedPreviewSourceRtsp = computed(() =>
  selectedPreviewCamera.value && previewStreamCameraId.value === selectedPreviewCamera.value.id
    ? (lastConnectionResult.value?.cameraId === selectedPreviewCamera.value.id ? lastConnectionResult.value.rtspUrl : null)
    : null,
)

const selectedPreviewIsPlaying = computed(
  () =>
    Boolean(
      selectedPreviewCamera.value &&
        selectedPreviewConfig.value &&
        previewStreamCameraId.value === selectedPreviewCamera.value.id,
    ),
)

const selectedPreviewMessage = computed(() => {
  return selectedPreviewCamera.value && previewStreamCameraId.value === selectedPreviewCamera.value.id
    ? previewStreamMessage.value
    : ""
})

const cameraMetrics = computed(() => {
  const total = records.value.length
  const online = records.value.filter((item) => item.status === "online").length
  const exception = records.value.filter((item) => item.status === "exception").length
  const supportAi = records.value.filter((item) => item.supportAi).length
  return { total, online, exception, supportAi }
})

const metricCards = computed(() => [
  { label: "摄像机总数", value: cameraMetrics.value.total, accent: "primary" },
  { label: "在线设备", value: cameraMetrics.value.online, accent: "success" },
  { label: "异常设备", value: cameraMetrics.value.exception, accent: "danger" },
  { label: "支持 AI", value: cameraMetrics.value.supportAi, accent: "info" },
])

const getStatusText = (status: string) => {
  if (status === "online") {
    return "在线"
  }
  if (status === "offline") {
    return "离线"
  }
  if (status === "exception") {
    return "异常"
  }
  if (status === "disabled") {
    return "停用"
  }
  return status
}

const getStatusTone = (status: string) => {
  if (status === "online") {
    return "success"
  }
  if (status === "exception") {
    return "danger"
  }
  if (status === "disabled") {
    return "warning"
  }
  return "default"
}

const getAiTone = (supportAi: boolean) => (supportAi ? "info" : "default")
const getAiText = (supportAi: boolean) => (supportAi ? "支持 AI" : "基础视频")
const formatDeviceCode = (deviceCode: string) => {
  const trimmed = deviceCode.trim()
  if (trimmed.length <= 4) {
    return trimmed
  }
  return `...${trimmed.slice(-4)}`
}

const formatDateTime = (value?: string | null) => {
  if (!value) {
    return "-"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return date.toLocaleString("zh-CN", { hour12: false })
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) {
    return detail
  }
  if (error instanceof Error && error.message) {
    return error.message
  }
  return fallback
}

const ensurePreviewSelection = () => {
  if (!records.value.length) {
    selectedPreviewId.value = null
    return
  }

  if (selectedPreviewId.value && records.value.some((item) => item.id === selectedPreviewId.value)) {
    return
  }

  selectedPreviewId.value = records.value[0].id
}

const syncFormZone = () => {
  if (formZoneOptions.value.some((item) => item.id === formState.zoneId)) {
    return
  }
  formState.zoneId = formZoneOptions.value[0]?.id ?? 0
}

const loadLookups = async () => {
  ;[factories.value, zones.value] = await Promise.all([listFactoriesApi(), listZonesApi()])
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listCamerasApi({
      keyword: queryForm.keyword || undefined,
      factory_id: queryForm.factoryId ? Number(queryForm.factoryId) : undefined,
      zone_id: queryForm.zoneId ? Number(queryForm.zoneId) : undefined,
      status: queryForm.status || undefined,
      support_ai: queryForm.supportAi === "" ? undefined : queryForm.supportAi === "true",
    })
    ensurePreviewSelection()
  } finally {
    loading.value = false
  }
}

const resetFormState = () => {
  editingId.value = null
  activeConfigTab.value = "basic"
  cameraSdkConfig.value = null
  formState.deviceCode = ""
  formState.name = ""
  formState.ip = ""
  formState.sdkPort = 8000
  formState.httpPort = 80
  formState.rtspPort = 554
  formState.username = "admin"
  formState.password = DEFAULT_DEVICE_PASSWORD
  formState.factoryId = factories.value[0]?.id ?? 0
  formState.installLocation = ""
  formState.supportAi = true
  formState.status = "offline"
  formState.remark = ""
  networkForm.ip = ""
  networkForm.subnetMask = ""
  networkForm.gateway = ""
  networkForm.primaryDns = ""
  networkForm.secondaryDns = ""
  networkForm.dhcpEnabled = false
  imageForm.resolution = "1920x1080"
  imageForm.frameRate = 25
  imageForm.bitrate = 2048
  imageForm.exposureMode = "auto"
  imageForm.exposureTime = ""
  imageForm.whiteBalanceMode = "auto"
  recordingForm.scheduleMode = ""
  recordingForm.storageMode = "sdCard"
  recordingForm.overwriteEnabled = true
  recordingWeeklyPlan.value = []
  ptzPresetForm.presetId = 1
  ptzPresetForm.name = ""
  selectedPtzPresetId.value = null
  ptzModeForm.cruiseEnabled = false
  ptzModeForm.trackEnabled = false
  userForm.userId = null
  userForm.username = ""
  userForm.password = ""
  userForm.role = "operator"
  userForm.enabled = true
  selectedUserId.value = null
  syncFormZone()
}

const openCreateDialog = () => {
  resetFormState()
  dialogVisible.value = true
}

const openEditDialog = async (record: CameraRecord) => {
  try {
    const detail = await getCameraApi(record.id)
    cameraSdkConfig.value = null
    editingId.value = detail.id
    formState.deviceCode = detail.deviceCode
    formState.name = detail.name
    formState.ip = detail.ip
    formState.sdkPort = detail.sdkPort
    formState.httpPort = detail.httpPort
    formState.rtspPort = detail.rtspPort
    formState.username = detail.username
    formState.password = ""
    formState.factoryId = detail.factoryId
    formState.zoneId = detail.zoneId
    formState.installLocation = detail.installLocation ?? ""
    formState.supportAi = detail.supportAi
    formState.status = detail.status
    formState.remark = detail.remark ?? ""
    activeConfigTab.value = "basic"
    dialogVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载摄像机详情失败"))
  }
}

const applyNetworkConfig = (config: CameraNetworkConfig) => {
  networkForm.ip = config.ip || formState.ip
  networkForm.subnetMask = config.subnetMask || ""
  networkForm.gateway = config.gateway || ""
  networkForm.primaryDns = config.primaryDns || ""
  networkForm.secondaryDns = config.secondaryDns || ""
  networkForm.dhcpEnabled = config.dhcpEnabled
}

const applyImageConfig = (config: CameraImageConfig) => {
  imageForm.resolution = config.resolution || "1920x1080"
  imageForm.frameRate = config.frameRate || 25
  imageForm.bitrate = config.bitrate || 2048
  imageForm.exposureMode = config.exposureMode || "auto"
  imageForm.exposureTime = config.exposureTime || ""
  imageForm.whiteBalanceMode = config.whiteBalanceMode || "auto"
}

const applyRecordingConfig = (config: CameraRecordingConfig) => {
  recordingForm.scheduleMode = config.scheduleMode || ""
  recordingForm.storageMode = config.storageMode || "sdCard"
  recordingForm.overwriteEnabled = config.overwriteEnabled ?? true
  recordingWeeklyPlan.value = config.weeklyPlan?.length
    ? config.weeklyPlan.map((day) => ({
        dayOfWeek: day.dayOfWeek,
        enabled: day.enabled,
        slots: day.slots.map((slot) => ({ ...slot })),
      }))
    : []
}

const applyPtzConfig = (config: CameraPtzConfig) => {
  const firstPreset = config.presets[0]
  selectedPtzPresetId.value = firstPreset?.presetId ?? null
  ptzPresetForm.presetId = firstPreset?.presetId ?? 1
  ptzPresetForm.name = firstPreset?.name ?? ""
  ptzModeForm.cruiseEnabled = Boolean(config.cruiseEnabled)
  ptzModeForm.trackEnabled = Boolean(config.trackEnabled)
}

const applyUserConfig = (items: CameraUserAccount[]) => {
  const firstUser = items[0]
  selectedUserId.value = firstUser?.userId ?? null
  userForm.userId = firstUser?.userId ?? null
  userForm.username = firstUser?.username ?? ""
  userForm.password = ""
  userForm.role = firstUser?.role || "operator"
  userForm.enabled = firstUser?.enabled ?? true
}

const loadCameraSdkConfig = async (cameraId: number) => {
  configLoading.value = true
  try {
    const config = await getCameraSdkConfigApi(cameraId)
    cameraSdkConfig.value = config
    applyNetworkConfig(config.network)
    applyImageConfig(config.image)
    applyRecordingConfig(config.recording)
    applyPtzConfig(config.ptz)
    applyUserConfig(config.users.items)
  } catch (error) {
    cameraSdkConfig.value = null
    ElMessage.warning(resolveErrorMessage(error, "读取海康摄像机配置失败"))
  } finally {
    configLoading.value = false
  }
}

const handleTabChange = async (tabName: string | number) => {
  activeConfigTab.value = tabName as CameraConfigTabName
  if (!editingId.value || cameraSdkConfig.value) {
    return
  }
  await loadCameraSdkConfig(editingId.value)
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  const payload: CameraSubmitPayload = {
    deviceCode: formState.deviceCode.trim(),
    name: formState.name.trim(),
    ip: formState.ip.trim(),
    sdkPort: Number(formState.sdkPort),
    httpPort: Number(formState.httpPort),
    rtspPort: Number(formState.rtspPort),
    username: formState.username.trim(),
    factoryId: Number(formState.factoryId),
    zoneId: Number(formState.zoneId),
    installLocation: formState.installLocation?.trim() || null,
    supportAi: formState.supportAi,
    status: formState.status,
    remark: formState.remark?.trim() || null,
  }

  submitting.value = true
  try {
    if (editingId.value) {
      await updateCameraApi(editingId.value, {
        ...payload,
        password: formState.password.trim() || undefined,
      })
      ElMessage.success("摄像机更新成功")
    } else {
      await createCameraApi({
        ...payload,
        password: formState.password.trim(),
      })
      ElMessage.success("摄像机创建成功")
    }

    dialogVisible.value = false
    resetFormState()
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存摄像机失败"))
  } finally {
    submitting.value = false
  }
}

const handleFetchDeviceIdentity = async () => {
  if (!formState.ip.trim()) {
    ElMessage.warning("请先输入设备 IP")
    return
  }
  if (!formState.username.trim()) {
    ElMessage.warning("请先输入登录账号")
    return
  }
  if (!formState.factoryId || !formState.zoneId) {
    ElMessage.warning("请先选择所属厂区和区域")
    return
  }

  fetchingDeviceIdentity.value = true
  try {
    const identity = await fetchCameraDeviceIdentityApi({
      cameraId: editingId.value,
      ip: formState.ip.trim(),
      sdkPort: Number(formState.sdkPort),
      httpPort: Number(formState.httpPort),
      rtspPort: Number(formState.rtspPort),
      username: formState.username.trim(),
      password: formState.password.trim() || null,
      factoryId: Number(formState.factoryId),
      zoneId: Number(formState.zoneId),
    })
    if (identity.deviceName) {
      formState.name = identity.deviceName
    }
    if (identity.deviceSerialNo) {
      formState.deviceCode = identity.deviceSerialNo
    }
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.deviceName = identity.deviceName || cameraSdkConfig.value.deviceName
      cameraSdkConfig.value.deviceSerialNo = identity.deviceSerialNo || cameraSdkConfig.value.deviceSerialNo
      cameraSdkConfig.value.deviceModel = identity.deviceModel || cameraSdkConfig.value.deviceModel
    }
    ElMessage.success("已获取设备名称和序列号")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "获取设备名称失败"))
  } finally {
    fetchingDeviceIdentity.value = false
  }
}

const handleSaveNetworkConfig = async () => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再配置网络参数")
    return
  }
  networkSubmitting.value = true
  try {
    const response = await updateCameraNetworkConfigApi(editingId.value, {
      ip: networkForm.ip.trim(),
      subnetMask: networkForm.subnetMask.trim() || null,
      gateway: networkForm.gateway.trim() || null,
      primaryDns: networkForm.primaryDns.trim() || null,
      secondaryDns: networkForm.secondaryDns.trim() || null,
      dhcpEnabled: networkForm.dhcpEnabled,
    })
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.network = response
    }
    formState.ip = response.ip
    ElMessage.success("网络配置已下发到摄像机")
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存网络配置失败"))
  } finally {
    networkSubmitting.value = false
  }
}

const handleSaveImageConfig = async () => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再配置图像参数")
    return
  }
  imageSubmitting.value = true
  try {
    const response = await updateCameraImageConfigApi(editingId.value, {
      resolution: imageForm.resolution || null,
      frameRate: Number(imageForm.frameRate) || null,
      bitrate: Number(imageForm.bitrate) || null,
      exposureMode: imageForm.exposureMode || null,
      exposureTime: imageForm.exposureTime.trim() || null,
      whiteBalanceMode: imageForm.whiteBalanceMode || null,
    })
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.image = response
    }
    ElMessage.success("图像配置已下发到摄像机")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存图像配置失败"))
  } finally {
    imageSubmitting.value = false
  }
}

const handleSaveRecordingConfig = async () => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再配置录像参数")
    return
  }
  recordingSubmitting.value = true
  try {
    const response = await updateCameraRecordingConfigApi(editingId.value, {
      storageMode: recordingForm.storageMode || null,
      overwriteEnabled: recordingForm.overwriteEnabled,
      weeklyPlan: recordingWeeklyPlan.value,
    })
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.recording = response
    }
    applyRecordingConfig(response)
    ElMessage.success("录像配置已下发到摄像机")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存录像配置失败"))
  } finally {
    recordingSubmitting.value = false
  }
}

const handleAddScheduleDay = () => {
  const next = Object.keys(weekdayLabels).find((day) => !recordingWeeklyPlan.value.some((item) => item.dayOfWeek === day))
  recordingWeeklyPlan.value.push({
    dayOfWeek: next || "monday",
    enabled: true,
    slots: [{ startTime: "00:00", endTime: "24:00", recordType: "CMR" }],
  })
}

const handleRemoveScheduleDay = (index: number) => {
  recordingWeeklyPlan.value.splice(index, 1)
}

const handleAddScheduleSlot = (dayIndex: number) => {
  recordingWeeklyPlan.value[dayIndex]?.slots.push({ startTime: "08:00", endTime: "18:00", recordType: "CMR" })
}

const handleRemoveScheduleSlot = (dayIndex: number, slotIndex: number) => {
  recordingWeeklyPlan.value[dayIndex]?.slots.splice(slotIndex, 1)
}

const handleSelectPtzPreset = (preset: CameraPtzPreset) => {
  selectedPtzPresetId.value = preset.presetId
  ptzPresetForm.presetId = preset.presetId
  ptzPresetForm.name = preset.name
}

const handleSavePtzPreset = async () => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再配置预置点")
    return
  }
  ptzSubmitting.value = true
  try {
    const response = await setCameraPtzPresetApi(editingId.value, {
      presetId: Number(ptzPresetForm.presetId),
      name: ptzPresetForm.name.trim(),
    })
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.ptz = response
    }
    applyPtzConfig(response)
    selectedPtzPresetId.value = Number(ptzPresetForm.presetId)
    ElMessage.success("预置点已保存")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存预置点失败"))
  } finally {
    ptzSubmitting.value = false
  }
}

const handleGotoPtzPreset = async (presetId?: number) => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再调用预置点")
    return
  }
  const targetPresetId = presetId ?? selectedPtzPresetId.value ?? Number(ptzPresetForm.presetId)
  if (!targetPresetId) {
    ElMessage.warning("请选择预置点")
    return
  }
  ptzSubmitting.value = true
  try {
    const result = await gotoCameraPtzPresetApi(editingId.value, targetPresetId)
    ElMessage.success(result.message)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "调用预置点失败"))
  } finally {
    ptzSubmitting.value = false
  }
}

const handleDeletePtzPreset = async (presetId?: number) => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再删除预置点")
    return
  }
  const targetPresetId = presetId ?? selectedPtzPresetId.value ?? Number(ptzPresetForm.presetId)
  if (!targetPresetId) {
    ElMessage.warning("请选择预置点")
    return
  }
  try {
    await ElMessageBox.confirm(`确认删除预置点 ${targetPresetId} 吗？`, "删除确认", { type: "warning" })
    ptzSubmitting.value = true
    const response = await deleteCameraPtzPresetApi(editingId.value, targetPresetId)
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.ptz = response
    }
    applyPtzConfig(response)
    ElMessage.success("预置点已删除")
  } catch (error) {
    if ((error as { message?: string })?.message === "cancel") {
      return
    }
    ElMessage.error(resolveErrorMessage(error, "删除预置点失败"))
  } finally {
    ptzSubmitting.value = false
  }
}

const handleSavePtzModeConfig = async () => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再配置巡航和轨迹")
    return
  }
  ptzSubmitting.value = true
  try {
    const response = await updateCameraPtzConfigApi(editingId.value, {
      cruiseEnabled: ptzModeForm.cruiseEnabled,
      trackEnabled: ptzModeForm.trackEnabled,
    })
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.ptz = response
    }
    applyPtzConfig(response)
    ElMessage.success("云台配置已保存")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存云台配置失败"))
  } finally {
    ptzSubmitting.value = false
  }
}

const handleSelectUser = (user: CameraUserAccount) => {
  selectedUserId.value = user.userId
  userForm.userId = user.userId
  userForm.username = user.username
  userForm.password = ""
  userForm.role = user.role || "operator"
  userForm.enabled = user.enabled
}

const handleSaveCameraUser = async () => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再配置账号")
    return
  }
  if (!userForm.username.trim()) {
    ElMessage.warning("请输入账号名")
    return
  }
  if (!userForm.userId && !userForm.password.trim()) {
    ElMessage.warning("新增账号时请输入密码")
    return
  }
  userSubmitting.value = true
  try {
    const response = await upsertCameraUserApi(editingId.value, {
      userId: userForm.userId,
      username: userForm.username.trim(),
      password: userForm.password.trim() || null,
      role: userForm.role,
      enabled: userForm.enabled,
    })
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.users = response
    }
    applyUserConfig(response.items)
    ElMessage.success("账号配置已保存")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存账号配置失败"))
  } finally {
    userSubmitting.value = false
  }
}

const handleDeleteCameraUser = async (userId?: number) => {
  if (!editingId.value) {
    ElMessage.warning("请先保存摄像机基础信息后再删除账号")
    return
  }
  const targetUserId = userId ?? selectedUserId.value
  if (!targetUserId) {
    ElMessage.warning("请选择账号")
    return
  }
  try {
    await ElMessageBox.confirm(`确认删除账号 ${targetUserId} 吗？`, "删除确认", { type: "warning" })
    userSubmitting.value = true
    const response = await deleteCameraUserApi(editingId.value, targetUserId)
    if (cameraSdkConfig.value) {
      cameraSdkConfig.value.users = response
    }
    applyUserConfig(response.items)
    ElMessage.success("账号已删除")
  } catch (error) {
    if ((error as { message?: string })?.message === "cancel") {
      return
    }
    ElMessage.error(resolveErrorMessage(error, "删除账号失败"))
  } finally {
    userSubmitting.value = false
  }
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.factoryId = ""
  queryForm.zoneId = ""
  queryForm.status = ""
  queryForm.supportAi = ""
  await loadRecords()
}

const handleToggleStatus = async (record: CameraRecord) => {
  const nextStatus = record.status === "disabled" ? "offline" : "disabled"
  try {
    await updateCameraStatusApi(record.id, nextStatus)
    ElMessage.success(nextStatus === "disabled" ? "摄像机已停用" : "摄像机已启用")
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新摄像机状态失败"))
  }
}

const handleDelete = async (record: CameraRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除摄像机“${record.name}”吗？`, "删除确认", {
      type: "warning",
    })
    await deleteCameraApi(record.id)
    ElMessage.success("摄像机删除成功")
    await loadRecords()
  } catch (error) {
    if ((error as { message?: string })?.message === "cancel") {
      return
    }
    ElMessage.error(resolveErrorMessage(error, "删除摄像机失败"))
  }
}

const handleTestConnection = async (record: CameraRecord) => {
  testingId.value = record.id
  selectedPreviewId.value = record.id
  previewDialogVisible.value = true
  previewWebControlConfig.value = null
  previewStreamCameraId.value = record.id
  previewStreamMessage.value = ""
  try {
    const result = await testCameraConnectionApi(record.id)
    lastConnectionResult.value = { ...result, cameraId: record.id }
    if (result.success) {
      await startInlinePreview(record.id)
    } else {
      previewWebControlConfig.value = null
      previewStreamMessage.value = result.message
    }
    await loadRecords()
  } catch (error) {
    previewWebControlConfig.value = null
    previewStreamCameraId.value = record.id
    previewStreamMessage.value = resolveErrorMessage(error, "连接测试失败")
  } finally {
    testingId.value = null
  }
}

const handlePreviewZoom = async (action: "in" | "out") => {
  const cameraId = selectedPreviewCamera.value?.id
  if (!cameraId) {
    ElMessage.warning("请先选择要控制的摄像机")
    return
  }
  zoomSubmitting.value = action
  try {
    const result = await controlCameraPtzZoomApi(cameraId, action)
    ElMessage.success(result.message)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, action === "in" ? "镜头放大失败" : "镜头缩小失败"))
  } finally {
    zoomSubmitting.value = null
  }
}

const handleCheckStatus = async (record: CameraRecord) => {
  checkingId.value = record.id
  try {
    const result = await checkCameraStatusApi(record.id)
    ElMessage.success(result.message)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "状态检测失败"))
  } finally {
    checkingId.value = null
  }
}

const handleOpenPreview = async (record: CameraRecord) => {
  selectedPreviewId.value = record.id
  try {
    const loginInfo = await getCameraBrowserLoginApi(record.id)
    const targetUrl = new URL(loginInfo.loginUrl)
    targetUrl.username = loginInfo.username
    targetUrl.password = loginInfo.password
    const openedWindow = window.open(targetUrl.toString(), "_blank", "noopener")
    if (!openedWindow) {
      throw new Error("浏览器拦截了新窗口，请允许弹窗后重试。")
    }
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "打开摄像机网页登录失败"))
  } finally {
    selectedPreviewId.value = null
  }
}

const clearInlinePreview = async (cameraId?: number) => {
  if (!cameraId || previewStreamCameraId.value === cameraId) {
    previewWebControlConfig.value = null
    previewStreamCameraId.value = null
    previewStreamMessage.value = ""
  }
}

const handlePreviewDialogClosed = async () => {
  await clearInlinePreview()
}

const startInlinePreview = async (cameraId: number) => {
  if (previewStreamCameraId.value && previewStreamCameraId.value !== cameraId) {
    await clearInlinePreview(previewStreamCameraId.value)
  }
  previewLoading.value = true
  previewStreamCameraId.value = cameraId
  previewStreamMessage.value = ""
  try {
    const config = await getLiveWebControlConfigApi(cameraId, { streamProfile: "main" })
    previewWebControlConfig.value = config
    previewStreamMessage.value = ""
  } catch (error) {
    previewWebControlConfig.value = null
    previewStreamMessage.value = resolveErrorMessage(error, "连接已通过，但 WebSDK 预览配置获取失败")
  } finally {
    previewLoading.value = false
  }
}

const handleCheckAll = async () => {
  checkingAll.value = true
  try {
    const result = await checkAllDevicesStatusApi()
    ElMessage.success(result.message)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "执行全部检测失败"))
  } finally {
    checkingAll.value = false
  }
}

const handleFactoryQueryChange = () => {
  if (!queryZoneOptions.value.some((item) => item.id === Number(queryForm.zoneId))) {
    queryForm.zoneId = ""
  }
}

const handleFormFactoryChange = () => {
  syncFormZone()
}

onMounted(async () => {
  await loadLookups()
  resetFormState()
  await loadRecords()
})

onBeforeUnmount(async () => {
  await clearInlinePreview()
})
</script>

<template>
  <div class="camera-page">
    <PageCard class="camera-page__filters-card">
      <SearchForm class="camera-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.factoryId" @change="handleFactoryQueryChange">
            <option value="">所属厂区</option>
            <option v-for="item in factories" :key="item.id" :value="String(item.id)">{{ item.factoryName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.zoneId">
            <option value="">所属区域</option>
            <option v-for="item in queryZoneOptions" :key="item.id" :value="String(item.id)">{{ item.zoneName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.status">
            <option value="">状态</option>
            <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.supportAi">
            <option value="">AI 分析</option>
            <option v-for="item in supportAiOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field camera-page__keyword">
          <input v-model="queryForm.keyword" type="text" placeholder="输入摄像机名称、编码或 IP" />
        </div>
        <template #actions>
          <button class="app-button app-button--primary camera-page__button camera-page__search-button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary camera-page__button camera-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'device:status:check'"
            class="app-button app-button--warning camera-page__button camera-page__search-button"
            :disabled="checkingAll"
            @click="handleCheckAll"
          >
            <el-icon><RefreshRight /></el-icon>
            <span>{{ checkingAll ? "检测中..." : "全部检测" }}</span>
          </button>
          <button
            v-permission="'device:camera:create'"
            class="app-button app-button--success camera-page__button camera-page__search-button"
            @click="openCreateDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增摄像机</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="camera-page__summary">
      <article
        v-for="card in metricCards"
        :key="card.label"
        class="camera-page__metric"
        :class="`camera-page__metric--${card.accent}`"
      >
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <PageCard >
      <table class="app-table camera-page__table">
        <colgroup>
          <col class="camera-page__col-code" />
          <col class="camera-page__col-name" />
          <col class="camera-page__col-area" />
          <col class="camera-page__col-ip" />
          <col class="camera-page__col-account" />
          <col class="camera-page__col-ai" />
          <col class="camera-page__col-status" />
          <col class="camera-page__col-time" />
          <col class="camera-page__col-location" />
          <col class="camera-page__col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>设备编码</th>
            <th>摄像机名称</th>
            <th>厂区 / 区域</th>
            <th>IP</th>
            <th>账号</th>
            <th>AI 分析</th>
            <th>状态</th>
            <th>最后在线时间</th>
            <th>安装位置</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="10" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr
            v-for="record in records"
            :key="record.id"
            :class="{ 'app-table__row--active': selectedPreviewId === record.id }"
            @click="selectedPreviewId = record.id"
          >
            <td :title="record.deviceCode">{{ formatDeviceCode(record.deviceCode) }}</td>
            <td>
              <div class="camera-page__name-cell">
                <strong>{{ record.name }}</strong>
              </div>
            </td>
            <td>{{ record.factoryName }} / {{ record.zoneName }}</td>
            <td>{{ record.ip }}</td>
            <td>{{ record.username }}</td>
            <td><StatusTag :text="getAiText(record.supportAi)" :tone="getAiTone(record.supportAi)" /></td>
            <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td>{{ formatDateTime(record.lastOnlineAt) }}</td>
            <td>{{ record.installLocation || "-" }}</td>
            <td>
              <div class="table-actions" @click.stop>
                <button
                  v-permission="'device:camera:update'"
                  class="app-button app-button--secondary camera-page__button camera-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
                <button
                  v-permission="'device:camera:update'"
                  class="app-button app-button--warning camera-page__button camera-page__table-button"
                  @click="handleToggleStatus(record)"
                >
                  <el-icon><SwitchButton /></el-icon>
                  <span>{{ record.status === "disabled" ? "启用" : "停用" }}</span>
                </button>
                <button
                  v-permission="'device:camera:test'"
                  class="app-button app-button--primary camera-page__button camera-page__table-button"
                  :disabled="testingId === record.id"
                  @click="handleTestConnection(record)"
                >
                  <el-icon><Connection /></el-icon>
                  <span>{{ testingId === record.id ? "测试中" : "测试" }}</span>
                </button>
                <button
                  v-permission="'device:camera:check'"
                  class="app-button app-button--secondary camera-page__button camera-page__table-button"
                  :disabled="checkingId === record.id"
                  @click="handleCheckStatus(record)"
                >
                  <el-icon><RefreshRight /></el-icon>
                  <span>{{ checkingId === record.id ? "检测中" : "检测" }}</span>
                </button>
                <button class="app-button app-button--secondary camera-page__button camera-page__table-button" @click="handleOpenPreview(record)">
                  <el-icon><VideoPlay /></el-icon>
                  <span>预览</span>
                </button>
                <button
                  v-permission="'device:camera:delete'"
                  class="app-button app-button--danger camera-page__button camera-page__table-button"
                  @click="handleDelete(record)"
                >
                  <el-icon><Delete /></el-icon>
                  <span>删除</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </PageCard>

    <el-dialog
      v-model="previewDialogVisible"
      title="连接测试结果"
      width="960px"
      top="4vh"
      destroy-on-close
      @closed="handlePreviewDialogClosed"
    >
      <div class="camera-preview-dialog__content">
        <div class="camera-page__preview-toolbar">
          <div class="camera-page__preview-field">
            <div class="camera-page__preview-zoom-actions">
              <button
                class="app-button app-button--secondary camera-page__button"
                :disabled="zoomSubmitting !== null || !selectedPreviewCamera"
                @click="handlePreviewZoom('in')"
              >
                {{ zoomSubmitting === "in" ? "处理中..." : "镜头放大" }}
              </button>
              <button
                class="app-button app-button--secondary camera-page__button"
                :disabled="zoomSubmitting !== null || !selectedPreviewCamera"
                @click="handlePreviewZoom('out')"
              >
                {{ zoomSubmitting === "out" ? "处理中..." : "镜头缩小" }}
              </button>
            </div>
          </div>
        </div>
        <VideoPlayer
          class="camera-preview-dialog__player"
          :title="selectedPreviewCamera?.name || '连接结果'"
          :play-url="selectedPreviewUrl"
          :stream-type="selectedPreviewStreamType"
          :stream-profile="selectedPreviewStreamProfile"
          :is-playing="selectedPreviewIsPlaying"
          :camera-name="selectedPreviewCamera?.name"
          :camera-location="selectedPreviewCamera ? `${selectedPreviewCamera.factoryName} / ${selectedPreviewCamera.zoneName}${selectedPreviewCamera.installLocation ? ` / ${selectedPreviewCamera.installLocation}` : ''}` : ''"
          :message="selectedPreviewMessage"
          :diagnostic-message="selectedPreviewDiagnosticMessage"
          :source-rtsp="selectedPreviewSourceRtsp"
          :is-mock="false"
          :playable-in-browser="false"
          :connection-mode="selectedPreviewConnectionMode"
          :snapshot-url="null"
          :active="Boolean(selectedPreviewCamera)"
          browse-mode="webcontrol"
          :web-control-config="selectedPreviewConfig"
        />
      </div>
    </el-dialog>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑摄像机' : '新增摄像机'"
      width="920px"
      top="4vh"
      destroy-on-close
    >
      <el-tabs v-model="activeConfigTab" class="camera-config-tabs" @tab-change="handleTabChange">
        <el-tab-pane v-for="item in configTabs" :key="item.name" :label="item.label" :name="item.name" />
      </el-tabs>

      <div v-if="configLoading" class="camera-config__loading">正在读取海康设备配置...</div>
      <div v-else class="camera-config">
        <el-alert
          v-if="activeConfigTab !== 'basic' && currentTabMessage"
          :title="currentTabMessage"
          :type="currentTabSupported ? 'info' : 'warning'"
          show-icon
          :closable="false"
          class="camera-config__alert"
        />

        <el-form
          v-if="activeConfigTab === 'basic'"
          ref="formRef"
          :model="formState"
          :rules="rules"
          label-width="110px"
          class="camera-form"
        >
          <el-form-item label="设备编码" prop="deviceCode">
            <el-input v-model="formState.deviceCode" placeholder="例如 cam-steel-001" />
          </el-form-item>
          <el-form-item label="摄像机名称" prop="name">
            <el-input v-model="formState.name" placeholder="例如 转炉平台枪机" />
          </el-form-item>
          <el-form-item label="设备 IP" prop="ip">
            <el-input v-model="formState.ip" placeholder="例如 192.168.1.22" />
          </el-form-item>
          <el-form-item label="SDK 端口">
            <el-input-number v-model="formState.sdkPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item label="HTTP 端口">
            <el-input-number v-model="formState.httpPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item label="RTSP 端口">
            <el-input-number v-model="formState.rtspPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
          </el-form-item>
          <el-form-item label="登录账号" prop="username">
            <el-input v-model="formState.username" />
          </el-form-item>
          <el-form-item label="设备密码" prop="password">
            <el-input
              v-model="formState.password"
              type="password"
              show-password
              :placeholder="editingId ? '留空表示保持原密码' : '请输入设备密码'"
            />
          </el-form-item>
          <el-form-item label="所属厂区" prop="factoryId">
            <el-select v-model="formState.factoryId" style="width: 100%" @change="handleFormFactoryChange">
              <el-option v-for="item in factories" :key="item.id" :label="item.factoryName" :value="item.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="所属区域" prop="zoneId">
            <el-select v-model="formState.zoneId" style="width: 100%">
              <el-option v-for="item in formZoneOptions" :key="item.id" :label="item.zoneName" :value="item.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="安装位置">
            <el-input v-model="formState.installLocation" placeholder="例如 转炉平台东侧立柱" />
          </el-form-item>
          <el-form-item label="运行状态">
            <el-select v-model="formState.status" style="width: 100%">
              <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="支持 AI 分析">
            <el-switch v-model="formState.supportAi" />
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="formState.remark" placeholder="记录安装环境、镜头朝向或维护备注" />
          </el-form-item>
        </el-form>

        <div v-else-if="activeConfigTab === 'network'" class="camera-sdk-panel">
          <div class="camera-sdk-panel__grid">
            <div class="camera-sdk-panel__field">
              <label>IP</label>
              <el-input v-model="networkForm.ip" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>子网掩码</label>
              <el-input v-model="networkForm.subnetMask" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>网关</label>
              <el-input v-model="networkForm.gateway" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>主 DNS</label>
              <el-input v-model="networkForm.primaryDns" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>备 DNS</label>
              <el-input v-model="networkForm.secondaryDns" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field camera-sdk-panel__field--switch">
              <label>DHCP</label>
              <el-switch v-model="networkForm.dhcpEnabled" :disabled="!currentTabSupported" />
            </div>
          </div>
          <div class="camera-sdk-panel__actions">
            <button class="app-button app-button--primary" :disabled="networkSubmitting || !currentTabSupported" @click="handleSaveNetworkConfig">
              {{ networkSubmitting ? "保存中..." : "保存网络配置" }}
            </button>
          </div>
        </div>

        <div v-else-if="activeConfigTab === 'image'" class="camera-sdk-panel">
          <div class="camera-sdk-panel__grid">
            <div class="camera-sdk-panel__field">
              <label>分辨率</label>
              <el-select v-model="imageForm.resolution" style="width: 100%" :disabled="!currentTabSupported" allow-create filterable default-first-option>
                <el-option v-for="item in resolutionOptions" :key="item" :label="item" :value="item" />
              </el-select>
            </div>
            <div class="camera-sdk-panel__field">
              <label>帧率</label>
              <el-input-number v-model="imageForm.frameRate" :min="1" :max="60" style="width: 100%" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>码率(Kbps)</label>
              <el-input-number v-model="imageForm.bitrate" :min="32" :max="16384" :step="128" style="width: 100%" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>曝光模式</label>
              <el-select v-model="imageForm.exposureMode" style="width: 100%" :disabled="!currentTabSupported">
                <el-option v-for="item in exposureModeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </div>
            <div class="camera-sdk-panel__field">
              <label>曝光时间</label>
              <el-input v-model="imageForm.exposureTime" placeholder="例如 1/50" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>白平衡</label>
              <el-select v-model="imageForm.whiteBalanceMode" style="width: 100%" :disabled="!currentTabSupported">
                <el-option v-for="item in whiteBalanceOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </div>
          </div>
          <div class="camera-sdk-panel__actions">
            <button class="app-button app-button--primary" :disabled="imageSubmitting || !currentTabSupported" @click="handleSaveImageConfig">
              {{ imageSubmitting ? "保存中..." : "保存图像配置" }}
            </button>
          </div>
        </div>

        <div v-else-if="activeConfigTab === 'recording'" class="camera-sdk-panel">
          <div class="camera-sdk-panel__grid">
            <div class="camera-sdk-panel__field">
              <label>计划模式</label>
              <el-input v-model="recordingForm.scheduleMode" disabled placeholder="当前阶段先只读展示" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>存储模式</label>
              <el-select v-model="recordingForm.storageMode" style="width: 100%" :disabled="!currentTabSupported">
                <el-option v-for="item in storageModeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </div>
            <div class="camera-sdk-panel__field camera-sdk-panel__field--switch">
              <label>覆盖录像</label>
              <el-switch v-model="recordingForm.overwriteEnabled" :disabled="!currentTabSupported" />
            </div>
          </div>
          <div class="camera-sdk-panel__section">
            <div class="camera-sdk-panel__section-header">
              <strong>周计划</strong>
              <button class="app-button app-button--secondary camera-page__table-button" :disabled="!currentTabSupported" @click="handleAddScheduleDay">
                新增日期
              </button>
            </div>
            <div v-if="recordingWeeklyPlan.length" class="camera-schedule-list">
              <div v-for="(day, dayIndex) in recordingWeeklyPlan" :key="`${day.dayOfWeek}-${dayIndex}`" class="camera-schedule-list__day">
                <div class="camera-schedule-list__day-header">
                  <el-select v-model="day.dayOfWeek" style="width: 140px" :disabled="!currentTabSupported">
                    <el-option v-for="(label, value) in weekdayLabels" :key="value" :label="label" :value="value" />
                  </el-select>
                  <el-switch v-model="day.enabled" active-text="启用" inactive-text="停用" :disabled="!currentTabSupported" />
                  <button class="app-button app-button--danger camera-page__table-button" :disabled="!currentTabSupported" @click="handleRemoveScheduleDay(dayIndex)">
                    删除日期
                  </button>
                </div>
                <div class="camera-schedule-list__slots">
                  <div v-for="(slot, slotIndex) in day.slots" :key="`${day.dayOfWeek}-${slotIndex}`" class="camera-schedule-list__slot">
                    <el-input v-model="slot.startTime" placeholder="08:00" :disabled="!currentTabSupported || !day.enabled" />
                    <span class="camera-schedule-list__to">至</span>
                    <el-input v-model="slot.endTime" placeholder="18:00" :disabled="!currentTabSupported || !day.enabled" />
                    <el-select v-model="slot.recordType" style="width: 140px" :disabled="!currentTabSupported || !day.enabled">
                      <el-option v-for="item in recordTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
                    </el-select>
                    <button class="app-button app-button--danger camera-page__table-button" :disabled="!currentTabSupported || !day.enabled" @click="handleRemoveScheduleSlot(dayIndex, slotIndex)">
                      删除时段
                    </button>
                  </div>
                </div>
                <button class="app-button app-button--secondary camera-page__table-button" :disabled="!currentTabSupported || !day.enabled" @click="handleAddScheduleSlot(dayIndex)">
                  新增时段
                </button>
              </div>
            </div>
            <div v-else class="camera-ptz-list__empty">暂无周计划，可先新增日期后配置录像时段。</div>
          </div>
          <div class="camera-sdk-panel__actions">
            <button class="app-button app-button--primary" :disabled="recordingSubmitting || !currentTabSupported" @click="handleSaveRecordingConfig">
              {{ recordingSubmitting ? "保存中..." : "保存录像配置" }}
            </button>
          </div>
        </div>

        <div v-else-if="activeConfigTab === 'ptz'" class="camera-sdk-panel">
          <div class="camera-sdk-panel__meta">
            <span>预置点数量：{{ cameraSdkConfig?.ptz.presetCount ?? 0 }}</span>
            <span>巡航：{{ cameraSdkConfig?.ptz.cruiseEnabled ? "支持" : "未接入" }}</span>
            <span>轨迹：{{ cameraSdkConfig?.ptz.trackEnabled ? "支持" : "未接入" }}</span>
          </div>
          <div class="camera-sdk-panel__grid">
            <div class="camera-sdk-panel__field camera-sdk-panel__field--switch">
              <label>巡航开关</label>
              <el-switch v-model="ptzModeForm.cruiseEnabled" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field camera-sdk-panel__field--switch">
              <label>轨迹开关</label>
              <el-switch v-model="ptzModeForm.trackEnabled" :disabled="!currentTabSupported" />
            </div>
          </div>
          <div class="camera-sdk-panel__actions">
            <button class="app-button app-button--primary" :disabled="ptzSubmitting || !currentTabSupported" @click="handleSavePtzModeConfig">
              {{ ptzSubmitting ? "处理中..." : "保存云台配置" }}
            </button>
          </div>
          <div class="camera-sdk-panel__grid">
            <div class="camera-sdk-panel__field">
              <label>预置点编号</label>
              <el-input-number v-model="ptzPresetForm.presetId" :min="1" :max="256" style="width: 100%" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>预置点名称</label>
              <el-input v-model="ptzPresetForm.name" :disabled="!currentTabSupported" placeholder="例如 炉前平台" />
            </div>
          </div>
          <div class="camera-sdk-panel__actions camera-sdk-panel__actions--start">
            <button class="app-button app-button--primary" :disabled="ptzSubmitting || !currentTabSupported || !ptzPresetForm.name.trim()" @click="handleSavePtzPreset">
              {{ ptzSubmitting ? "处理中..." : "保存预置点" }}
            </button>
            <button class="app-button app-button--secondary" :disabled="ptzSubmitting || !currentTabSupported" @click="handleGotoPtzPreset()">
              调用预置点
            </button>
            <button class="app-button app-button--danger" :disabled="ptzSubmitting || !currentTabSupported" @click="handleDeletePtzPreset()">
              删除预置点
            </button>
          </div>
          <div class="camera-ptz-list">
            <div
              v-for="preset in ptzPresets"
              :key="preset.presetId"
              class="camera-ptz-list__item"
              :class="{ 'camera-ptz-list__item--active': selectedPtzPresetId === preset.presetId }"
              @click="handleSelectPtzPreset(preset)"
            >
              <div class="camera-ptz-list__text">
                <strong>预置点 {{ preset.presetId }}</strong>
                <span>{{ preset.name }}</span>
              </div>
              <div class="camera-ptz-list__actions">
                <button class="app-button app-button--secondary camera-page__table-button" @click.stop="handleGotoPtzPreset(preset.presetId)">调用</button>
                <button class="app-button app-button--danger camera-page__table-button" @click.stop="handleDeletePtzPreset(preset.presetId)">删除</button>
              </div>
            </div>
            <div v-if="!ptzPresets.length" class="camera-ptz-list__empty">暂无预置点，可先输入编号和名称后保存。</div>
          </div>
        </div>

        <div v-else-if="activeConfigTab === 'users'" class="camera-sdk-panel">
          <div class="camera-sdk-panel__grid">
            <div class="camera-sdk-panel__field">
              <label>账号名</label>
              <el-input v-model="userForm.username" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>密码</label>
              <el-input v-model="userForm.password" type="password" show-password :placeholder="userForm.userId ? '留空表示不修改密码' : '新增账号必须输入密码'" :disabled="!currentTabSupported" />
            </div>
            <div class="camera-sdk-panel__field">
              <label>权限角色</label>
              <el-select v-model="userForm.role" style="width: 100%" :disabled="!currentTabSupported">
                <el-option v-for="item in userRoleOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </div>
            <div class="camera-sdk-panel__field camera-sdk-panel__field--switch">
              <label>启用</label>
              <el-switch v-model="userForm.enabled" :disabled="!currentTabSupported" />
            </div>
          </div>
          <div class="camera-sdk-panel__actions camera-sdk-panel__actions--start">
            <button class="app-button app-button--primary" :disabled="userSubmitting || !currentTabSupported || !userForm.username.trim()" @click="handleSaveCameraUser">
              {{ userSubmitting ? "处理中..." : "保存账号" }}
            </button>
            <button class="app-button app-button--danger" :disabled="userSubmitting || !currentTabSupported || !selectedUserId" @click="handleDeleteCameraUser()">
              删除账号
            </button>
          </div>
          <div class="camera-ptz-list">
            <div
              v-for="user in cameraUsers"
              :key="user.userId"
              class="camera-ptz-list__item"
              :class="{ 'camera-ptz-list__item--active': selectedUserId === user.userId }"
              @click="handleSelectUser(user)"
            >
              <div class="camera-ptz-list__text">
                <strong>{{ user.username }}</strong>
                <span>角色：{{ user.role || "operator" }} / 状态：{{ user.enabled ? "启用" : "停用" }}</span>
              </div>
              <div class="camera-ptz-list__actions">
                <button class="app-button app-button--danger camera-page__table-button" @click.stop="handleDeleteCameraUser(user.userId)">删除</button>
              </div>
            </div>
            <div v-if="!cameraUsers.length" class="camera-ptz-list__empty">暂无摄像机账号，可在上方填写后新增。</div>
          </div>
        </div>
      </div>
      <template #footer>
        <button
          v-if="activeConfigTab === 'basic'"
          class="app-button app-button--secondary"
          :disabled="fetchingDeviceIdentity"
          @click="handleFetchDeviceIdentity"
        >
          {{ fetchingDeviceIdentity ? "获取中..." : "获取名称" }}
        </button>
        <button class="app-button app-button--secondary" @click="dialogVisible = false">取消</button>
        <button v-if="activeConfigTab === 'basic'" class="app-button app-button--primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? "保存中..." : "保存基础信息" }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.camera-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.camera-page__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.camera-page__metric {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.camera-page__metric span {
  color: #667b91;
  font-size: 13px;
}

.camera-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
}

.camera-page__metric--success strong {
  color: #1d9b52;
}

.camera-page__metric--danger strong {
  color: #d64f5a;
}

.camera-page__metric--info strong {
  color: #1d7ad9;
}

.camera-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.camera-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.camera-page__search-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.camera-page__search-form :deep(.search-form__fields) {
  grid-template-columns: 140px 140px 140px 140px minmax(220px, 300px);
  gap: 10px;
  align-items: end;
}

.camera-page__search-form :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.camera-page__search-form :deep(.app-field select),
.camera-page__search-form :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.camera-page__search-button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.camera-page__search-button :deep(.el-icon) {
  font-size: 13px;
}

.camera-preview-dialog__content {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.camera-preview-dialog__player {
  width: 100%;
  max-width: 100%;
  min-height: 360px;
  aspect-ratio: 16 / 9;
  height: auto !important;
}

.camera-page__preview-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 14px;
}

.camera-page__preview-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 180px;
}

.camera-page__preview-zoom-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.camera-page__preview-field label {
  color: #60778f;
  font-size: 12px;
  font-weight: 700;
}

.camera-page__preview-field select {
  height: 38px;
  padding: 0 12px;
  border: 1px solid #d5e2ed;
  border-radius: 8px;
  background: #ffffff;
}

.camera-page__keyword {
  min-width: 0;
  max-width: 300px;
}

.camera-page__name-cell,
.camera-page__port-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.camera-page__name-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.camera-page__port-cell small {
  color: #708398;
  font-size: 11px;
  line-height: 1.4;
}

.camera-page__table {
  table-layout: fixed;
}

.camera-page__table th,
.camera-page__table td {
  padding: 9px 10px;
  font-size: 12px;
  vertical-align: middle;
}

.camera-page__table th {
  font-size: 12px;
  white-space: nowrap;
}

.camera-page__table td:nth-child(1),
.camera-page__table td:nth-child(5),
.camera-page__table td:nth-child(6),
.camera-page__table td:nth-child(7) {
  white-space: nowrap;
}

.camera-page__table td:nth-child(8) {
  font-size: 12px;
  line-height: 1.4;
}

.camera-page__col-code {
  width: 74px;
}

.camera-page__col-name {
  width: 118px;
}

.camera-page__col-area {
  width: 126px;
}

.camera-page__col-ip {
  width: 112px;
}

.camera-page__col-account {
  width: 62px;
}

.camera-page__col-ai {
  width: 92px;
}

.camera-page__col-status {
  width: 72px;
}

.camera-page__col-time {
  width: 140px;
}

.camera-page__col-location {
  width: 72px;
}

.camera-page__col-actions {
  width: 296px;
}

.camera-page__table .table-actions {
  flex-wrap: nowrap;
  gap: 4px;
}

.camera-page__table-button {
  min-height: 30px;
  padding: 0 7px;
  font-size: 11px;
  gap: 3px;
  white-space: nowrap;
}

.camera-page__table-button :deep(.el-icon) {
  font-size: 11px;
}

.camera-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 16px;
}

.camera-config-tabs {
  margin-bottom: 14px;
}

.camera-config__loading {
  padding: 32px 0;
  color: #5d7288;
  text-align: center;
}

.camera-config__alert {
  margin-bottom: 16px;
}

.camera-form :deep(.el-form-item) {
  margin-bottom: 18px;
}

.camera-sdk-panel {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.camera-sdk-panel__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: #627890;
  font-size: 13px;
}

.camera-sdk-panel__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.camera-sdk-panel__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.camera-sdk-panel__field label {
  color: #5f7489;
  font-size: 13px;
  font-weight: 700;
}

.camera-sdk-panel__field--switch {
  justify-content: center;
}

.camera-sdk-panel__actions {
  display: flex;
  justify-content: flex-end;
}

.camera-sdk-panel__actions--start {
  justify-content: flex-start;
  gap: 10px;
  flex-wrap: wrap;
}

.camera-sdk-panel__section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.camera-sdk-panel__section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.camera-sdk-panel__section-header strong {
  color: #17385a;
  font-size: 14px;
}

.camera-schedule-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.camera-schedule-list__day {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border: 1px solid #dbe6f0;
  border-radius: 10px;
  background: #fbfdff;
}

.camera-schedule-list__day-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}

.camera-schedule-list__slots {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.camera-schedule-list__slot {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) 140px auto;
  gap: 10px;
  align-items: center;
}

.camera-schedule-list__to {
  color: #688096;
  font-size: 12px;
  text-align: center;
}

.camera-ptz-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.camera-ptz-list__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid #dbe6f0;
  border-radius: 10px;
  background: #ffffff;
  cursor: pointer;
}

.camera-ptz-list__item--active {
  border-color: #2f7df6;
  background: #f3f8ff;
}

.camera-ptz-list__text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.camera-ptz-list__text strong {
  color: #17385a;
  font-size: 13px;
}

.camera-ptz-list__text span {
  color: #6e8297;
  font-size: 12px;
}

.camera-ptz-list__actions {
  display: flex;
  gap: 8px;
}

.camera-ptz-list__empty {
  padding: 20px 12px;
  border: 1px dashed #d7e3ee;
  border-radius: 10px;
  color: #6d8196;
  text-align: center;
}

.camera-sdk-placeholder {
  min-height: 220px;
  padding: 24px 20px;
  border: 1px dashed #d7e3ee;
  border-radius: 12px;
  background: #f8fbfe;
  color: #5e7489;
  line-height: 1.7;
}

@media (max-width: 1280px) {
  .camera-page__summary {
    grid-template-columns: 1fr 1fr;
  }

  .camera-page__search-form {
    grid-template-columns: 1fr;
  }

  .camera-page__search-form :deep(.search-form__fields) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .camera-page__search-form :deep(.search-form__actions) {
    flex-wrap: wrap;
  }
}

@media (max-width: 960px) {
  .camera-page__summary,
  .camera-form,
  .camera-sdk-panel__grid {
    grid-template-columns: 1fr;
  }

  .camera-schedule-list__slot {
    grid-template-columns: 1fr;
  }

  .camera-page__search-form :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }

  .camera-page__table .table-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 768px) {
  .camera-page__search-form :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
