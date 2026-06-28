<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { useRoute } from "vue-router"
import { ElMessage, ElMessageBox } from "element-plus"
import {
  ArrowLeft,
  ArrowRight,
  FullScreen,
  HomeFilled,
  Location,
  LocationFilled,
  Picture,
  RefreshRight,
  Search,
  VideoPause,
  VideoPlay,
  VideoCameraFilled,
} from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import HikWebControlGrid from "../../components/video/HikWebControlGrid.vue"
import VideoPlayer from "../../components/video/VideoPlayer.vue"
import { listCamerasApi } from "../../api/camera"
import { listChannelsApi } from "../../api/recorder"
import { listFactoriesApi, listZonesApi } from "../../api/master-data"
import {
  createSnapshotApi,
  getChannelLiveVideoApi,
  getChannelLiveWebControlConfigApi,
  getLiveVideoApi,
  getLiveWebControlConfigApi,
  stopChannelLiveVideoApi,
  stopLiveVideoApi,
} from "../../api/video"
import type { CameraRecord } from "../../types/camera"
import type { FactoryRecord, ZoneRecord } from "../../types/master-data"
import type { RecorderChannelRecord } from "../../types/recorder"
import type {
  ConnectionMode,
  LiveVideoPayload,
  LiveWebControlConfig,
  PreviewBrowseMode,
  StreamProfile,
  StreamType,
} from "../../types/video"

type PreviewSourceType = "camera" | "channel"
const DEFAULT_STREAM_TYPE: StreamType = "hik-sdk"
const DEFAULT_STREAM_PROFILE: StreamProfile = "main"
const DEFAULT_BROWSE_MODE: PreviewBrowseMode = "webcontrol"
const PREVIEW_UNAVAILABLE_MESSAGE = "摄像机连接不成功"

interface PreviewSlot {
  sourceType: PreviewSourceType
  cameraId: number | null
  channelId: number | null
  recorderName: string
  channelName: string
  cameraName: string
  location: string
  status: string
  streamType: StreamType
  connectionMode: ConnectionMode
  streamProfile: StreamProfile
  playUrl: string | null
  expiresIn: number
  isPlaying: boolean
  message: string
  snapshotUrl: string | null
  diagnosticMessage: string | null
  sourceRtsp: string | null
  isMock: boolean
  playableInBrowser: boolean
  browseMode: PreviewBrowseMode
  webControlConfig: LiveWebControlConfig | null
}

interface ChannelTreeZone {
  id: number | string
  zoneName: string
  groups: ChannelTreeGroup[]
  leaves: PreviewSourceItem[]
}

interface ChannelTreeGroup {
  id: string
  groupName: string
  sources: PreviewSourceItem[]
}

interface ChannelTreeFactory extends FactoryRecord {
  zones: ChannelTreeZone[]
}

interface PreviewSourceItem {
  type: PreviewSourceType
  key: string
  id: number
  factoryId: number
  factoryName: string
  zoneId: number | null
  zoneName: string
  name: string
  meta: string
  status: string
  camera?: CameraRecord
  channel?: RecorderChannelRecord
}

type SlotPreviewPayload =
  | { kind: "browser"; liveData: LiveVideoPayload }
  | { kind: "webcontrol"; config: LiveWebControlConfig }
  | { kind: "unavailable"; message: string }

const loading = ref(false)
const previewLoading = ref(false)
const stopLoading = ref(false)
const snapshotLoading = ref(false)
const layoutMode = ref<1 | 4 | 9>(1)
const browseMode = ref<PreviewBrowseMode>(DEFAULT_BROWSE_MODE)
const activeSlotIndex = ref(0)
const hikGridRef = ref<{
  getOSDTime: (windowIndex?: number) => Promise<string>
  captureCurrentFrame: (windowIndex?: number) => Promise<string | null>
} | null>(null)
const selectedSourceKey = ref("")
const playerGridRef = ref<HTMLElement | null>(null)
const route = useRoute()
const carouselSources = ref<PreviewSourceItem[]>([])
const carouselPageIndex = ref(0)
const carouselSwitching = ref(false)
let carouselTimer: number | null = null

const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const cameras = ref<CameraRecord[]>([])
const channels = ref<RecorderChannelRecord[]>([])

const sourceKeyword = ref("")

const createSlot = (): PreviewSlot => ({
  sourceType: "channel",
  cameraId: null,
  channelId: null,
  recorderName: "",
  channelName: "",
  cameraName: "",
  location: "",
  status: "offline",
  streamType: DEFAULT_STREAM_TYPE,
  connectionMode: "hik-sdk",
  streamProfile: DEFAULT_STREAM_PROFILE,
  playUrl: null,
  expiresIn: 0,
  isPlaying: false,
  message: "",
  snapshotUrl: null,
  diagnosticMessage: null,
  sourceRtsp: null,
  isMock: false,
  playableInBrowser: true,
  browseMode: DEFAULT_BROWSE_MODE,
  webControlConfig: null,
})

const slots = ref<PreviewSlot[]>([
  createSlot(),
  createSlot(),
  createSlot(),
  createSlot(),
  createSlot(),
  createSlot(),
  createSlot(),
  createSlot(),
  createSlot(),
])

const displaySlotCount = computed(() => layoutMode.value)
const visibleSlots = computed(() => slots.value.slice(0, displaySlotCount.value))
const activeSlot = computed(() => slots.value[activeSlotIndex.value])
const hikGridSlots = computed(() =>
  visibleSlots.value.map((slot, index) => ({
    key: `${slot.sourceType}-${slot.channelId ?? slot.cameraId ?? index}`,
    title: slot.cameraName || `窗口 ${index + 1}`,
    config: slot.webControlConfig,
    isPlaying: slot.isPlaying,
    message: slot.message,
  })),
)
const activeSnapshotSource = computed(() => ({
  cameraId: activeSlot.value?.cameraId ?? null,
  channelId: activeSlot.value?.sourceType === "channel" ? activeSlot.value?.channelId ?? null : null,
}))
const snapshotDisabled = computed(() =>
  snapshotLoading.value
  || layoutMode.value !== 1
  || (!activeSnapshotSource.value.cameraId && !activeSnapshotSource.value.channelId),
)

const enabledChannels = computed(() => channels.value.filter((item) => item.enabled))
const enabledCameras = computed(() => cameras.value.filter((item) => item.status !== "disabled"))
const channelSourceItems = computed<PreviewSourceItem[]>(() =>
  enabledChannels.value.map((channel) => ({
    type: "channel",
    key: `channel-${channel.id}`,
    id: channel.id,
    factoryId: channel.factoryId,
    factoryName: channel.factoryName,
    zoneId: channel.zoneId ?? null,
    zoneName: channel.zoneName || "未分区",
    name: channel.name,
    meta: channel.cameraName || channel.recorderName,
    status: channel.status,
    channel,
  })),
)
const cameraSourceItems = computed<PreviewSourceItem[]>(() =>
  enabledCameras.value.map((camera) => ({
    type: "camera",
    key: `camera-${camera.id}`,
    id: camera.id,
    factoryId: camera.factoryId,
    factoryName: camera.factoryName,
    zoneId: camera.zoneId ?? null,
    zoneName: camera.zoneName || "未分区",
    name: camera.name,
    meta: camera.installLocation || camera.zoneName || camera.factoryName,
    status: camera.status,
    camera,
  })),
)
const previewSources = computed(() => [...channelSourceItems.value, ...cameraSourceItems.value])
const normalizedSourceKeyword = computed(() => sourceKeyword.value.trim().toLowerCase())
const filteredPreviewSources = computed(() =>
  previewSources.value.filter((item) => {
    const targetText = `${item.name} ${item.meta} ${item.zoneName} ${item.factoryName}`.toLowerCase()
    const matchesKeyword = !normalizedSourceKeyword.value || targetText.includes(normalizedSourceKeyword.value)
    return matchesKeyword
  }),
)

const resolveSourceGroupLabel = (source: PreviewSourceItem) => {
  const rawLabel = source.type === "camera" ? source.camera?.installLocation?.trim() || source.meta.trim() : ""
  if (!rawLabel || rawLabel === source.name || rawLabel === source.zoneName || rawLabel === source.factoryName) {
    return "未分类"
  }
  return rawLabel
}

const buildFactoryKey = (factoryId: number) => `factory-${factoryId}`
const buildZoneKey = (factoryId: number, zoneId: number | string) => `zone-${factoryId}-${zoneId}`
const buildGroupKey = (factoryId: number, zoneId: number | string, groupId: string) => `group-${factoryId}-${zoneId}-${groupId}`
const expandedFactoryKey = ref<string | null>(null)
const expandedZoneKey = ref<string | null>(null)
const expandedGroupKey = ref<string | null>(null)

const channelTree = computed<ChannelTreeFactory[]>(() =>
  factories.value
    .map((factory) => {
      const factorySources = filteredPreviewSources.value.filter((item) => item.factoryId === factory.id)
      const factoryZones: ChannelTreeZone[] = zones.value
        .filter((zone) => zone.factoryId === factory.id)
        .map((zone) => {
          const zoneSources = factorySources.filter((item) => item.zoneId === zone.id)
          const leaves = zoneSources
            .filter((source) => source.type === "channel")
            .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"))
          const groupMap = new Map<string, PreviewSourceItem[]>()
          zoneSources
            .filter((source) => source.type === "camera")
            .forEach((source) => {
            const groupLabel = resolveSourceGroupLabel(source)
              if (groupLabel === "未分类") {
                leaves.push(source)
                return
              }
              const groupSources = groupMap.get(groupLabel) ?? []
              groupSources.push(source)
              groupMap.set(groupLabel, groupSources)
            })
          const groups = Array.from(groupMap.entries())
            .sort(([left], [right]) => left.localeCompare(right, "zh-CN"))
            .map(([groupName, sources]) => ({
              id: `${zone.id}-${groupName}`,
              groupName,
              sources: [...sources].sort((left, right) => left.name.localeCompare(right.name, "zh-CN")),
            }))
          return {
            id: zone.id,
            zoneName: zone.zoneName,
            groups,
            leaves,
          }
        })
        .filter((zone) => zone.groups.length || zone.leaves.length)

      const unassignedSources = factorySources.filter((item) => !item.zoneId)
      if (unassignedSources.length) {
        const leaves = unassignedSources
          .filter((source) => source.type === "channel")
          .sort((left, right) => left.name.localeCompare(right.name, "zh-CN"))
        const groupMap = new Map<string, PreviewSourceItem[]>()
        unassignedSources
          .filter((source) => source.type === "camera")
          .forEach((source) => {
          const groupLabel = resolveSourceGroupLabel(source)
            if (groupLabel === "未分类") {
              leaves.push(source)
              return
            }
            const groupSources = groupMap.get(groupLabel) ?? []
            groupSources.push(source)
            groupMap.set(groupLabel, groupSources)
          })
        factoryZones.push({
          id: `unassigned-${factory.id}`,
          zoneName: "未分区",
          leaves,
          groups: Array.from(groupMap.entries()).map(([groupName, sources]) => ({
            id: `unassigned-${factory.id}-${groupName}`,
            groupName,
            sources: [...sources].sort((left, right) => left.name.localeCompare(right.name, "zh-CN")),
          })),
        })
      }
      return {
        ...factory,
        zones: factoryZones,
      }
    })
    .filter((factory) => factory.zones.length),
)

const isFactoryExpanded = (factoryId: number) => expandedFactoryKey.value === buildFactoryKey(factoryId)
const isZoneExpanded = (factoryId: number, zoneId: number | string) => expandedZoneKey.value === buildZoneKey(factoryId, zoneId)
const isGroupExpanded = (factoryId: number, zoneId: number | string, groupId: string) =>
  expandedGroupKey.value === buildGroupKey(factoryId, zoneId, groupId)

const toggleFactoryExpand = (factoryId: number) => {
  const nextKey = buildFactoryKey(factoryId)
  if (expandedFactoryKey.value === nextKey) {
    expandedFactoryKey.value = null
    expandedZoneKey.value = null
    expandedGroupKey.value = null
    return
  }
  expandedFactoryKey.value = nextKey
  expandedZoneKey.value = null
  expandedGroupKey.value = null
}

const toggleZoneExpand = (factoryId: number, zoneId: number | string) => {
  const nextKey = buildZoneKey(factoryId, zoneId)
  if (expandedZoneKey.value === nextKey) {
    expandedZoneKey.value = null
    expandedGroupKey.value = null
    return
  }
  expandedZoneKey.value = nextKey
  expandedGroupKey.value = null
}

const toggleGroupExpand = (factoryId: number, zoneId: number | string, groupId: string) => {
  const nextKey = buildGroupKey(factoryId, zoneId, groupId)
  expandedGroupKey.value = expandedGroupKey.value === nextKey ? null : nextKey
}

const getFactorySourceCount = (zones: ChannelTreeZone[]) => zones.reduce((count, zone) => count + getZoneSourceCount(zone), 0)
const getZoneSourceCount = (zone: ChannelTreeZone) => zone.leaves.length + zone.groups.reduce((count, group) => count + group.sources.length, 0)
const dedupePreviewSources = (sources: PreviewSourceItem[]) => Array.from(new Map(sources.map((item) => [item.key, item])).values())
const collectZoneSources = (zone: ChannelTreeZone) => dedupePreviewSources([...zone.leaves, ...zone.groups.flatMap((group) => group.sources)])
const collectFactorySources = (factory: ChannelTreeFactory) => dedupePreviewSources(factory.zones.flatMap((zone) => collectZoneSources(zone)))
const resolveBatchLayout = (sourceCount: number): 1 | 4 | 9 => {
  if (sourceCount <= 1) {
    return 1
  }
  if (sourceCount <= 4) {
    return 4
  }
  return 9
}

const getSourceDisplayName = (source: PreviewSourceItem) => {
  if (source.type === "channel") {
    const recorderName = source.channel?.recorderName?.trim() || ""
    const cameraName = source.channel?.cameraName?.trim() || source.name
    return `${recorderName}${recorderName && cameraName ? " + " : ""}${cameraName}`.trim()
  }
  return source.name
}

watch(
  channelTree,
  (tree) => {
    if (!tree.length) {
      expandedFactoryKey.value = null
      expandedZoneKey.value = null
      expandedGroupKey.value = null
      return
    }

    const activeFactory = tree.find((factory) => buildFactoryKey(factory.id) === expandedFactoryKey.value) ?? tree[0]
    expandedFactoryKey.value = buildFactoryKey(activeFactory.id)

    if (!activeFactory.zones.length) {
      expandedZoneKey.value = null
      expandedGroupKey.value = null
      return
    }

    const activeZone =
      activeFactory.zones.find((zone) => buildZoneKey(activeFactory.id, zone.id) === expandedZoneKey.value) ?? activeFactory.zones[0]
    expandedZoneKey.value = buildZoneKey(activeFactory.id, activeZone.id)

    if (!activeZone.groups.length) {
      expandedGroupKey.value = null
      return
    }

    const activeGroup =
      activeZone.groups.find((group) => buildGroupKey(activeFactory.id, activeZone.id, group.id) === expandedGroupKey.value) ??
      activeZone.groups[0]
    expandedGroupKey.value = buildGroupKey(activeFactory.id, activeZone.id, activeGroup.id)
  },
  { immediate: true },
)

const carouselTotalPages = computed(() => Math.max(1, Math.ceil(carouselSources.value.length / displaySlotCount.value)))
const carouselPageText = computed(() =>
  carouselSources.value.length ? `${carouselPageIndex.value + 1} / ${carouselTotalPages.value}` : "-",
)
const carouselEnabled = computed(() => carouselSources.value.length > displaySlotCount.value)

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const responseData = (error as { response?: { data?: { detail?: string; message?: string } } })?.response?.data
  if (typeof responseData?.message === "string" && responseData.message) return responseData.message
  if (typeof responseData?.detail === "string" && responseData.detail) return responseData.detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const isPreviewSourceOnline = (status: string) => status === "online"
const pad = (value: number) => String(value).padStart(2, "0")

const showPreviewErrorDialog = (message: string, title = "预览提示") =>
  ElMessageBox.alert(message, title, {
    type: "error",
    confirmButtonText: "确定",
  })

const sanitizeSnapshotFilePart = (value: string) =>
  value
    .replace(/[\\/:*?"<>|]/g, (char) =>
      ({
        "\\": "＼",
        "/": "／",
        ":": ":",
        "*": "＊",
        "?": "？",
        "\"": "＂",
        "<": "＜",
        ">": "＞",
        "|": "｜",
      })[char] || "_")
    .trim()

const formatSnapshotTime = (value?: string | null) => {
  if (!value) {
    return ""
  }
  const normalizedValue = value.trim().replace(" ", "T")
  const date = new Date(normalizedValue)
  if (Number.isNaN(date.getTime())) {
    return sanitizeSnapshotFilePart(value.replace("T", " ")).replace(/[-_: ]/g, "")
  }
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
}

const buildPreviewSnapshotFileName = (slot: PreviewSlot, osdTime?: string | null) => {
  const resolvedTime = formatSnapshotTime(osdTime) || formatSnapshotTime(new Date().toISOString())
  const baseName =
    slot.sourceType === "camera"
      ? sanitizeSnapshotFilePart(slot.cameraName || "摄像机")
      : `${sanitizeSnapshotFilePart(slot.recorderName || "录像机")}+${sanitizeSnapshotFilePart(slot.channelName || slot.cameraName || "通道")}`
  return `${baseName}+${resolvedTime}.jpg`
}

const triggerSnapshotDownload = async (snapshotUrl: string, fileName: string) => {
  const response = await fetch(snapshotUrl, {
    credentials: "include",
  })
  if (!response.ok) {
    throw new Error(`下载截图失败，状态码 ${response.status}`)
  }
  const blob = await response.blob()
  const objectUrl = window.URL.createObjectURL(blob)
  const anchor = document.createElement("a")
  anchor.href = objectUrl
  anchor.download = fileName
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  window.URL.revokeObjectURL(objectUrl)
}

const ensureActiveSlotInRange = () => {
  if (activeSlotIndex.value >= displaySlotCount.value) {
    activeSlotIndex.value = 0
  }
}

const resetSlotPlayback = (slot: PreviewSlot) => {
  slot.playUrl = null
  slot.expiresIn = 0
  slot.isPlaying = false
  slot.message = ""
  slot.snapshotUrl = null
  slot.diagnosticMessage = null
  slot.sourceRtsp = null
  slot.isMock = false
  slot.playableInBrowser = true
  slot.browseMode = browseMode.value
  slot.webControlConfig = null
}

const clearSlot = (slot: PreviewSlot) => {
  slot.sourceType = "channel"
  slot.cameraId = null
  slot.channelId = null
  slot.recorderName = ""
  slot.channelName = ""
  slot.cameraName = ""
  slot.location = ""
  slot.status = "offline"
  slot.streamType = DEFAULT_STREAM_TYPE
  slot.connectionMode = "hik-sdk"
  slot.streamProfile = DEFAULT_STREAM_PROFILE
  resetSlotPlayback(slot)
}

const assignChannelToSlot = (channel: RecorderChannelRecord, slot: PreviewSlot) => {
  slot.sourceType = "channel"
  slot.channelId = channel.id
  slot.cameraId = channel.cameraId ?? null
  slot.recorderName = channel.recorderName
  slot.channelName = channel.name
  slot.cameraName = channel.cameraName || channel.name
  slot.location = `${channel.factoryName} / ${channel.zoneName || "未分区"} / ${channel.recorderName}`
  slot.status = channel.status
  slot.streamType = DEFAULT_STREAM_TYPE
  slot.streamProfile = DEFAULT_STREAM_PROFILE
  if (!slot.isPlaying) {
    slot.playUrl = null
    slot.message = ""
  }
}

const assignCameraToSlot = (camera: CameraRecord, slot: PreviewSlot) => {
  slot.sourceType = "camera"
  slot.cameraId = camera.id
  slot.channelId = null
  slot.recorderName = ""
  slot.channelName = ""
  slot.cameraName = camera.name
  slot.location = `${camera.factoryName} / ${camera.zoneName}${camera.installLocation ? ` / ${camera.installLocation}` : ""}`
  slot.status = camera.status
  slot.streamType = DEFAULT_STREAM_TYPE
  slot.streamProfile = DEFAULT_STREAM_PROFILE
  if (!slot.isPlaying) {
    slot.playUrl = null
    slot.message = ""
  }
}

const assignSourceToSlot = (source: PreviewSourceItem, slot: PreviewSlot) => {
  if (source.type === "channel" && source.channel) {
    assignChannelToSlot(source.channel, slot)
    return
  }
  if (source.type === "camera" && source.camera) {
    assignCameraToSlot(source.camera, slot)
  }
}

const assignSourceToPrimarySlot = (source: PreviewSourceItem) => {
  const slot = slots.value[0]
  if (!slot) return
  carouselSources.value = []
  stopCarouselTimer()
  activeSlotIndex.value = 0
  assignSourceToSlot(source, slot)
  selectedSourceKey.value = source.key
}

const handleSelectSource = async (source: PreviewSourceItem) => {
  previewLoading.value = true
  try {
    const primarySlot = slots.value[0]
    if (primarySlot?.isPlaying && (primarySlot.cameraId || primarySlot.channelId)) {
      await stopSlotPreview(primarySlot)
    } else if (primarySlot) {
      resetSlotPlayback(primarySlot)
    }

    assignSourceToPrimarySlot(source)
    await startSlotPreview(slots.value[0])
    await scrollPreviewToBottom()
  } catch (error) {
    await showPreviewErrorDialog(resolveErrorMessage(error, "开始预览失败"))
  } finally {
    previewLoading.value = false
  }
}

const handleBatchPreview = async (
  sources: PreviewSourceItem[],
  label: string,
  preferredLayout?: 1 | 4 | 9,
) => {
  const nextSources = dedupePreviewSources(sources)
  if (!nextSources.length) {
    await showPreviewErrorDialog(`${label}暂无可预览的通道或摄像机`, "上屏失败")
    return
  }

  previewLoading.value = true
  try {
    await stopAllPreviewSlots()

    if (preferredLayout && layoutMode.value !== preferredLayout) {
      layoutMode.value = preferredLayout
      ensureActiveSlotInRange()
    }

    carouselSources.value = nextSources
    carouselPageIndex.value = 0
    activeSlotIndex.value = 0
    selectedSourceKey.value = nextSources[0]?.key ?? ""
    await previewCarouselPage()
    await scrollPreviewToBottom()
  } catch (error) {
    await showPreviewErrorDialog(resolveErrorMessage(error, `${label}上屏失败`), "上屏失败")
  } finally {
    previewLoading.value = false
  }
}

const handleFactoryBatchPreview = async (factory: ChannelTreeFactory) => {
  const factorySources = collectFactorySources(factory)
  await handleBatchPreview(factorySources, `${factory.factoryName}`, resolveBatchLayout(factorySources.length))
}

const handleZoneBatchPreview = async (zone: ChannelTreeZone) => {
  const zoneSources = collectZoneSources(zone)
  await handleBatchPreview(zoneSources, `${zone.zoneName}`, resolveBatchLayout(zoneSources.length))
}

const assignChannelToActiveSlot = (channel: RecorderChannelRecord) => {
  const slot = slots.value[0]
  if (!slot) return
  carouselSources.value = []
  stopCarouselTimer()
  activeSlotIndex.value = 0
  assignChannelToSlot(channel, slot)
  selectedSourceKey.value = `channel-${channel.id}`
}

const assignCameraToActiveSlot = (camera: CameraRecord) => {
  const slot = slots.value[0]
  if (!slot) return
  carouselSources.value = []
  stopCarouselTimer()
  activeSlotIndex.value = 0
  assignCameraToSlot(camera, slot)
  selectedSourceKey.value = `camera-${camera.id}`
}

const findCameraByRoute = () => {
  const rawCameraId = route.query.cameraId
  const cameraId = typeof rawCameraId === "string" ? Number(rawCameraId) : NaN
  if (!Number.isInteger(cameraId)) {
    return null
  }
  return cameras.value.find((item) => item.id === cameraId) ?? null
}

const findChannelByRoute = () => {
  const rawChannelId = route.query.channelId
  const channelId = typeof rawChannelId === "string" ? Number(rawChannelId) : NaN
  if (!Number.isInteger(channelId)) {
    return null
  }
  return enabledChannels.value.find((item) => item.id === channelId) ?? null
}

const handleActivateSlot = (index: number) => {
  activeSlotIndex.value = index
  const slot = slots.value[index]
  selectedSourceKey.value =
    slot.sourceType === "channel" && slot.channelId
      ? `channel-${slot.channelId}`
      : slot.cameraId
        ? `camera-${slot.cameraId}`
        : ""
}

const loadPreviewBaseData = async () => {
  loading.value = true
  try {
    ;[factories.value, zones.value, cameras.value, channels.value] = await Promise.all([
      listFactoriesApi(),
      listZonesApi(),
      listCamerasApi(),
      listChannelsApi({ enabled: true }),
    ])
  } finally {
    loading.value = false
  }
}

const applyLiveResponseToSlot = (slot: PreviewSlot, liveData: LiveVideoPayload) => {
  slot.browseMode = "browser"
  slot.streamType = liveData.streamType
  slot.connectionMode = liveData.connectionMode
  slot.playUrl = liveData.playUrl
  slot.expiresIn = liveData.expiresIn
  slot.isPlaying = !liveData.isMock && liveData.playableInBrowser
  slot.streamProfile = DEFAULT_STREAM_PROFILE
  slot.isMock = liveData.isMock
  slot.playableInBrowser = liveData.playableInBrowser
  slot.diagnosticMessage = liveData.diagnosticMessage ?? null
  slot.sourceRtsp = liveData.sourceRtsp ?? null
  slot.message = ""
}

const applyWebControlConfigToSlot = (slot: PreviewSlot, config: LiveWebControlConfig) => {
  slot.browseMode = "webcontrol"
  slot.streamType = "hik-sdk"
  slot.connectionMode = "hik-sdk"
  slot.playUrl = null
  slot.expiresIn = 0
  slot.isPlaying = true
  slot.streamProfile = config.streamProfile
  slot.isMock = false
  slot.playableInBrowser = false
  slot.diagnosticMessage = config.message ?? null
  slot.sourceRtsp = null
  slot.webControlConfig = config
  slot.message = ""
}

const resolveSlotPreviewPayload = async (slot: PreviewSlot): Promise<SlotPreviewPayload> => {
  if (!slot.cameraId && !slot.channelId) {
    throw new Error("请先选择通道或摄像机")
  }
  if (!isPreviewSourceOnline(slot.status)) {
    return {
      kind: "unavailable",
      message: PREVIEW_UNAVAILABLE_MESSAGE,
    }
  }

  if (browseMode.value === "webcontrol") {
    const config =
      slot.sourceType === "channel"
        ? await getChannelLiveWebControlConfigApi(Number(slot.channelId), {
            streamProfile: DEFAULT_STREAM_PROFILE,
          })
        : await getLiveWebControlConfigApi(Number(slot.cameraId), {
            streamProfile: DEFAULT_STREAM_PROFILE,
          })
    return {
      kind: "webcontrol",
      config,
    }
  }

  const liveData =
    slot.sourceType === "channel" && slot.channelId
      ? await getChannelLiveVideoApi(slot.channelId, {
          streamType: DEFAULT_STREAM_TYPE,
          streamProfile: DEFAULT_STREAM_PROFILE,
        })
      : await getLiveVideoApi(Number(slot.cameraId), {
          streamType: DEFAULT_STREAM_TYPE,
          streamProfile: DEFAULT_STREAM_PROFILE,
        })
  return {
    kind: "browser",
    liveData,
  }
}

const applyPreviewPayloadToSlot = (slot: PreviewSlot, payload: SlotPreviewPayload) => {
  if (payload.kind === "webcontrol") {
    applyWebControlConfigToSlot(slot, payload.config)
    return
  }
  if (payload.kind === "browser") {
    applyLiveResponseToSlot(slot, payload.liveData)
    return
  }
  resetSlotPlayback(slot)
  slot.message = payload.message
}

const startSlotPreview = async (slot: PreviewSlot) => {
  const payload = await resolveSlotPreviewPayload(slot)
  applyPreviewPayloadToSlot(slot, payload)
}

const stopSlotPreview = async (slot: PreviewSlot) => {
  if (!slot.cameraId && !slot.channelId) {
    return
  }

  if (slot.browseMode === "webcontrol") {
    resetSlotPlayback(slot)
    return
  }

  if (slot.sourceType === "channel" && slot.channelId) {
    await stopChannelLiveVideoApi(slot.channelId)
  } else if (slot.cameraId) {
    await stopLiveVideoApi(slot.cameraId)
  }
  resetSlotPlayback(slot)
}

const stopAllPreviewSlots = async () => {
  await Promise.allSettled(
    slots.value
      .filter((slot) => slot.isPlaying && (slot.cameraId || slot.channelId))
      .map((slot) => stopSlotPreview(slot)),
  )
}

const closeAllPreviewSlots = async () => {
  stopCarouselTimer()
  await stopAllPreviewSlots()
  slots.value.forEach((slot) => clearSlot(slot))
  carouselSources.value = []
  carouselPageIndex.value = 0
  activeSlotIndex.value = 0
  selectedSourceKey.value = ""
}

const handleChangeBrowseMode = async (nextMode: PreviewBrowseMode) => {
  if (nextMode === "browser") {
    return
  }
  if (browseMode.value === nextMode) {
    return
  }

  previewLoading.value = true
  try {
    await stopAllPreviewSlots()
    visibleSlots.value.forEach((slot) => {
      resetSlotPlayback(slot)
      slot.browseMode = nextMode
    })
    browseMode.value = nextMode
  } finally {
    previewLoading.value = false
  }
}

const handleStartPreview = async () => {
  const slot = activeSlot.value
  if (!slot || (!slot.cameraId && !slot.channelId)) {
    await showPreviewErrorDialog("请先选择通道或摄像机", "操作失败")
    return
  }

  previewLoading.value = true
  try {
    await startSlotPreview(slot)
    await scrollPreviewToBottom()
  } catch (error) {
    await showPreviewErrorDialog(resolveErrorMessage(error, "开始预览失败"))
  } finally {
    previewLoading.value = false
  }
}

const handleStopPreview = async () => {
  if (layoutMode.value !== 1) {
    const hasAssignedSlot = slots.value.some((slot) => slot.cameraId || slot.channelId)
    if (!hasAssignedSlot) {
      await showPreviewErrorDialog("当前画面未分配通道或摄像机", "操作失败")
      return
    }

    stopLoading.value = true
    try {
      await closeAllPreviewSlots()
    } catch (error) {
      await showPreviewErrorDialog(resolveErrorMessage(error, "停止预览失败"))
    } finally {
      stopLoading.value = false
    }
    return
  }

  const slot = activeSlot.value
  if (!slot || (!slot.cameraId && !slot.channelId)) {
    await showPreviewErrorDialog("当前窗口未分配通道或摄像机", "操作失败")
    return
  }

  stopLoading.value = true
  try {
    await stopSlotPreview(slot)
  } catch (error) {
    await showPreviewErrorDialog(resolveErrorMessage(error, "停止预览失败"))
  } finally {
    stopLoading.value = false
  }
}

const handleSnapshot = async () => {
  const slot = activeSlot.value
  if (layoutMode.value !== 1) {
    await showPreviewErrorDialog("仅单画面模式支持截图，请先切换到单画面。", "操作失败")
    return
  }
  if (!slot?.cameraId && !slot?.channelId) {
    await showPreviewErrorDialog("当前窗口未分配通道或摄像机，暂不支持截图。", "操作失败")
    return
  }

  snapshotLoading.value = true
  try {
    let snapshotOsdTime = ""
    if (slot.browseMode === "webcontrol" && hikGridRef.value?.getOSDTime) {
      try {
        snapshotOsdTime = await hikGridRef.value.getOSDTime(activeSlotIndex.value)
      } catch {
        snapshotOsdTime = ""
      }
    }
    if (slot.browseMode === "webcontrol" && hikGridRef.value?.captureCurrentFrame) {
      const dataUrl = await hikGridRef.value.captureCurrentFrame(activeSlotIndex.value)
      if (!dataUrl) {
        throw new Error("当前画面尚未准备完成，暂时无法截图。")
      }
      slot.snapshotUrl = dataUrl
    } else {
      const result = await createSnapshotApi({
        cameraId: slot.sourceType === "camera" ? (slot.cameraId ?? undefined) : undefined,
        channelId: slot.sourceType === "channel" ? (slot.channelId ?? undefined) : undefined,
        streamProfile: DEFAULT_STREAM_PROFILE,
        preferDeviceSnapshot: slot.connectionMode === "hik-sdk",
      })
      slot.snapshotUrl = result.snapshotUrl
    }
    slot.message = ""
    await triggerSnapshotDownload(slot.snapshotUrl, buildPreviewSnapshotFileName(slot, snapshotOsdTime))
    ElMessage.success("截图成功，已开始下载")
  } catch (error) {
    await showPreviewErrorDialog(resolveErrorMessage(error, "截图失败"))
  } finally {
    snapshotLoading.value = false
  }
}

const getCarouselPageSources = () => {
  const start = carouselPageIndex.value * displaySlotCount.value
  return carouselSources.value.slice(start, start + displaySlotCount.value)
}

const stopCarouselTimer = () => {
  if (carouselTimer !== null) {
    window.clearInterval(carouselTimer)
    carouselTimer = null
  }
}

const previewCarouselPage = async (showErrorDialog = true) => {
  if (!carouselSources.value.length || carouselSwitching.value) return
  carouselSwitching.value = true
  previewLoading.value = true
  try {
    await Promise.allSettled(visibleSlots.value.map((slot) => stopSlotPreview(slot)))

    const pageSources = getCarouselPageSources()
    slots.value.slice(0, displaySlotCount.value).forEach((slot, index) => {
      const source = pageSources[index]
      if (source) {
        assignSourceToSlot(source, slot)
      } else {
        clearSlot(slot)
      }
    })

    activeSlotIndex.value = 0
    selectedSourceKey.value = pageSources[0]?.key ?? ""
    const pageSlots = visibleSlots.value.filter((slot) => slot.cameraId || slot.channelId)
    const startResults = await Promise.allSettled(pageSlots.map((slot) => resolveSlotPreviewPayload(slot)))
    startResults.forEach((result, index) => {
      const slot = pageSlots[index]
      if (!slot) {
        return
      }
      if (result.status === "fulfilled") {
        applyPreviewPayloadToSlot(slot, result.value)
        return
      }
      resetSlotPlayback(slot)
      slot.message = resolveErrorMessage(result.reason, "开始预览失败")
    })
    const failedResults = startResults.filter((result) => result.status === "rejected")
    const firstFailure = failedResults[0]
    if (firstFailure && failedResults.length === startResults.length && showErrorDialog) {
      await showPreviewErrorDialog(resolveErrorMessage(firstFailure.reason, "部分画面开始预览失败"))
    }
  } finally {
    previewLoading.value = false
    carouselSwitching.value = false
  }
}

const handleCarouselPrevious = async () => {
  if (!carouselEnabled.value) return
  carouselPageIndex.value = (carouselPageIndex.value - 1 + carouselTotalPages.value) % carouselTotalPages.value
  await previewCarouselPage()
}

const handleCarouselNext = async () => {
  if (!carouselEnabled.value) return
  carouselPageIndex.value = (carouselPageIndex.value + 1) % carouselTotalPages.value
  await previewCarouselPage(false)
}

const handleToggleLayout = async (nextMode: 1 | 4 | 9) => {
  layoutMode.value = nextMode
  ensureActiveSlotInRange()
  carouselPageIndex.value = Math.min(carouselPageIndex.value, carouselTotalPages.value - 1)
  if (carouselSources.value.length) {
    await previewCarouselPage(false)
  }
}

const handleFullscreen = async () => {
  if (!playerGridRef.value) return
  if (document.fullscreenElement) {
    await document.exitFullscreen()
    return
  }
  await playerGridRef.value.requestFullscreen()
}

const scrollPreviewToBottom = async () => {
  await nextTick()
  const scrollToPageBottom = (behavior: ScrollBehavior) => {
    const scrollTarget = Math.max(
      document.documentElement.scrollHeight,
      document.body.scrollHeight,
      document.documentElement.offsetHeight,
      document.body.offsetHeight,
    )
    const scrollingElement = document.scrollingElement ?? document.documentElement
    scrollingElement.scrollTo({
      top: scrollTarget,
      behavior,
    })
  }
  window.requestAnimationFrame(() => {
    scrollToPageBottom("smooth")
    window.setTimeout(() => scrollToPageBottom("smooth"), 180)
    window.setTimeout(() => scrollToPageBottom("smooth"), 420)
  })
}

const getStatusTone = (status: string) => {
  if (status === "online") return "success"
  if (status === "exception") return "danger"
  if (status === "disabled") return "warning"
  return "default"
}

onMounted(async () => {
  await loadPreviewBaseData()
  const routeChannel = findChannelByRoute()
  if (routeChannel) {
    assignChannelToActiveSlot(routeChannel)
    if (route.query.autoplay === "1") {
      await handleStartPreview()
    }
    return
  }
  const routeCamera = findCameraByRoute()
  if (routeCamera) {
    assignCameraToActiveSlot(routeCamera)
    if (route.query.autoplay === "1") {
      await handleStartPreview()
    }
  }
})

onBeforeUnmount(() => {
  stopCarouselTimer()
  void stopAllPreviewSlots()
})
</script>

<template>
  <div class="monitor-preview-page">
    <div class="monitor-preview-page__layout">
      <PageCard class="monitor-preview-page__sources">
        <div class="camera-tree">
          <div class="camera-tree__filters">
            <div class="camera-tree__search-box">
              <input
                v-model.trim="sourceKeyword"
                class="camera-tree__search"
                type="text"
                placeholder="搜索"
              />
              <el-icon class="camera-tree__search-icon"><Search /></el-icon>
            </div>
          </div>
          <div class="camera-tree__content">
            <div v-if="loading" class="camera-tree__empty">加载中...</div>
            <div v-else-if="!channelTree.length" class="camera-tree__empty">暂无可预览的通道或摄像机</div>
            <div v-for="factory in channelTree" :key="factory.id" class="camera-tree__branch">
              <div class="camera-tree__node-row">
                <button class="camera-tree__node camera-tree__node--factory" @click="toggleFactoryExpand(factory.id)">
                  <span class="camera-tree__node-main">
                    <el-icon class="camera-tree__caret" :class="{ 'camera-tree__caret--expanded': isFactoryExpanded(factory.id) }"><ArrowRight /></el-icon>
                    <el-icon class="camera-tree__node-icon camera-tree__node-icon--factory"><HomeFilled /></el-icon>
                    <span class="camera-tree__node-label">{{ factory.factoryName }}</span>
                  </span>
                  <span class="camera-tree__node-meta">{{ getFactorySourceCount(factory.zones) }}</span>
                </button>
                <button class="camera-tree__action" @click="() => void handleFactoryBatchPreview(factory)">
                  上屏
                </button>
              </div>
              <div v-if="isFactoryExpanded(factory.id)" class="camera-tree__children">
                <div v-for="zone in factory.zones" :key="zone.id" class="camera-tree__branch">
                  <div class="camera-tree__node-row">
                    <button class="camera-tree__node camera-tree__node--zone" @click="toggleZoneExpand(factory.id, zone.id)">
                      <span class="camera-tree__node-main">
                        <el-icon class="camera-tree__caret" :class="{ 'camera-tree__caret--expanded': isZoneExpanded(factory.id, zone.id) }"><ArrowRight /></el-icon>
                        <el-icon class="camera-tree__node-icon camera-tree__node-icon--zone"><LocationFilled /></el-icon>
                        <span class="camera-tree__node-label">{{ zone.zoneName }}</span>
                      </span>
                      <span class="camera-tree__node-meta">{{ getZoneSourceCount(zone) }}</span>
                    </button>
                    <button class="camera-tree__action" @click="() => void handleZoneBatchPreview(zone)">
                      上屏
                    </button>
                  </div>
                  <div v-if="isZoneExpanded(factory.id, zone.id)" class="camera-tree__children camera-tree__children--zone">
                    <button
                      v-for="source in zone.leaves"
                      :key="source.key"
                      class="camera-tree__leaf"
                      :class="{ 'camera-tree__leaf--active': selectedSourceKey === source.key }"
                      @click="() => void handleSelectSource(source)"
                    >
                      <span class="camera-tree__leaf-main">
                        <el-icon class="camera-tree__leaf-icon"><VideoCameraFilled /></el-icon>
                        <span class="camera-tree__leaf-label">{{ getSourceDisplayName(source) }}</span>
                      </span>
                      <span class="camera-tree__leaf-status" :class="`camera-tree__leaf-status--${getStatusTone(source.status)}`" />
                    </button>
                    <div v-for="group in zone.groups" :key="group.id" class="camera-tree__branch">
                      <button class="camera-tree__node camera-tree__node--group" @click="toggleGroupExpand(factory.id, zone.id, group.id)">
                        <span class="camera-tree__node-main">
                          <el-icon class="camera-tree__caret" :class="{ 'camera-tree__caret--expanded': isGroupExpanded(factory.id, zone.id, group.id) }"><ArrowRight /></el-icon>
                          <el-icon class="camera-tree__node-icon camera-tree__node-icon--group"><Location /></el-icon>
                          <span class="camera-tree__node-label">{{ group.groupName }}</span>
                        </span>
                      </button>
                      <div v-if="isGroupExpanded(factory.id, zone.id, group.id)" class="camera-tree__children camera-tree__children--leaf">
                        <button
                          v-for="source in group.sources"
                          :key="source.key"
                          class="camera-tree__leaf"
                          :class="{ 'camera-tree__leaf--active': selectedSourceKey === source.key }"
                          @click="() => void handleSelectSource(source)"
                        >
                          <span class="camera-tree__leaf-main">
                            <el-icon class="camera-tree__leaf-icon"><VideoCameraFilled /></el-icon>
                            <span class="camera-tree__leaf-label">{{ getSourceDisplayName(source) }}</span>
                          </span>
                          <span class="camera-tree__leaf-status" :class="`camera-tree__leaf-status--${getStatusTone(source.status)}`" />
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </PageCard>

      <div class="monitor-preview-page__main">
        <PageCard class="monitor-preview-page__controls">
          <div class="monitor-preview-page__toolbar">
            <div class="preview-toolbar">
              <div class="preview-toolbar__group">
                <div class="preview-toolbar__inline">
                  <button
                    class="app-button"
                    :class="browseMode === 'browser' ? 'app-button--primary' : 'app-button--secondary'"
                    disabled
                  >
                    HLS
                  </button>
                  <button
                    class="app-button"
                    :class="browseMode === 'webcontrol' ? 'app-button--primary' : 'app-button--secondary'"
                    @click="() => void handleChangeBrowseMode('webcontrol')"
                  >
                    HIK
                  </button>
                </div>
              </div>
              <div class="preview-toolbar__group">
                <div class="preview-toolbar__inline">
                  <button
                    class="app-button"
                    :class="layoutMode === 1 ? 'app-button--primary' : 'app-button--secondary'"
                    @click="handleToggleLayout(1)"
                  >
                    单画面
                  </button>
                  <button
                    class="app-button"
                    :class="layoutMode === 4 ? 'app-button--primary' : 'app-button--secondary'"
                    @click="handleToggleLayout(4)"
                  >
                    四画面
                  </button>
                  <button
                    class="app-button"
                    :class="layoutMode === 9 ? 'app-button--primary' : 'app-button--secondary'"
                    @click="handleToggleLayout(9)"
                  >
                    九画面
                  </button>
                </div>
              </div>
              <div class="preview-toolbar__group">
                <div class="preview-toolbar__inline">
                  <button
                    class="app-button app-button--secondary"
                    :disabled="!carouselEnabled || carouselSwitching"
                    @click="handleCarouselPrevious"
                  >
                    <el-icon><ArrowLeft /></el-icon>
                    <span>上一页</span>
                  </button>
                  <button
                    class="app-button app-button--secondary"
                    :disabled="!carouselEnabled || carouselSwitching"
                    @click="handleCarouselNext"
                  >
                    <span>下一页</span>
                    <el-icon><ArrowRight /></el-icon>
                  </button>
                  <span class="preview-toolbar__page">{{ carouselPageText }}</span>
                </div>
              </div>
              <div class="preview-toolbar__group preview-toolbar__group--actions">
                <div class="preview-toolbar__inline">
                  <button class="app-button app-button--success" :disabled="previewLoading" @click="handleStartPreview">
                    <el-icon><VideoPlay /></el-icon>
                    <span>{{ previewLoading ? "启动中..." : "开始预览" }}</span>
                  </button>
                  <button class="app-button app-button--warning" :disabled="stopLoading" @click="handleStopPreview">
                    <el-icon><VideoPause /></el-icon>
                    <span>{{ stopLoading ? (layoutMode === 1 ? "停止中..." : "关闭中...") : (layoutMode === 1 ? "停止" : "全部关闭") }}</span>
                  </button>
                  <button class="app-button app-button--secondary" :disabled="snapshotDisabled" @click="handleSnapshot">
                    <el-icon><Picture /></el-icon>
                    <span>{{ snapshotLoading ? "截图中..." : "截图" }}</span>
                  </button>
                  <button class="app-button app-button--secondary" @click="handleFullscreen">
                    <el-icon><FullScreen /></el-icon>
                    <span>全屏</span>
                  </button>
                  <button class="app-button app-button--secondary" @click="loadPreviewBaseData">
                    <el-icon><RefreshRight /></el-icon>
                    <span>刷新设备</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </PageCard>

        <section
          ref="playerGridRef"
          class="player-grid"
          :class="[browseMode === 'webcontrol' ? `player-grid--hik-${layoutMode}` : `player-grid--${layoutMode}`]"
        >
          <HikWebControlGrid
            v-if="browseMode === 'webcontrol'"
            ref="hikGridRef"
            :layout-mode="layoutMode"
            :active-slot-index="activeSlotIndex"
            :slots="hikGridSlots"
            @select="handleActivateSlot"
          />
          <article
            v-else
            v-for="(slot, index) in visibleSlots"
            :key="index"
            class="player-grid__slot"
            :class="{ 'player-grid__slot--active': activeSlotIndex === index }"
            @click="handleActivateSlot(index)"
          >
            <VideoPlayer
              :play-url="slot.playUrl"
              :stream-type="slot.streamType"
              :stream-profile="slot.streamProfile"
              :is-playing="slot.isPlaying"
              :visible-slot-count="layoutMode"
              :browse-mode="slot.browseMode"
              :web-control-config="slot.webControlConfig"
              :camera-name="slot.cameraName"
              :camera-location="slot.location"
              :snapshot-url="slot.snapshotUrl"
              :message="slot.message"
              :diagnostic-message="slot.diagnosticMessage"
              :source-rtsp="slot.sourceRtsp"
              :is-mock="slot.isMock"
              :playable-in-browser="slot.playableInBrowser"
              :connection-mode="slot.connectionMode"
              :active="activeSlotIndex === index"
            />
          </article>
        </section>
      </div>
    </div>
  </div>
</template>

<style scoped>
.monitor-preview-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: calc(100vh - var(--layout-header-height) - (var(--layout-page-padding) * 2));
  min-height: 0;
  overflow: hidden;
}

.monitor-preview-page__layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 20px;
  flex: 1;
  min-height: 0;
}

.monitor-preview-page__main {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: 0;
  order: 1;
}

.monitor-preview-page__sources {
  order: 2;
  position: sticky;
  top: 0;
  align-self: stretch;
  height: 100%;
  min-height: 0;
  max-height: calc(100vh - var(--layout-header-height) - (var(--layout-page-padding) * 2));
  overflow: hidden;
}

.monitor-preview-page__sources :deep(.page-card) {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.monitor-preview-page__sources :deep(.page-card__body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  padding: 10px 8px 10px 10px;
}

.monitor-preview-page__main :deep(.page-card) {
  border-color: rgba(88, 139, 200, 0.18);
  background:
    radial-gradient(circle at top right, rgba(58, 134, 255, 0.14), transparent 28%),
    linear-gradient(180deg, rgba(11, 34, 63, 0.98) 0%, rgba(9, 24, 45, 0.98) 100%);
  box-shadow: 0 22px 46px rgba(8, 24, 46, 0.22);
}

.monitor-preview-page__main :deep(.page-card__header) {
  border-bottom-color: rgba(147, 185, 229, 0.14);
}

.monitor-preview-page__main :deep(.page-card__title) {
  color: #edf5ff;
}

.monitor-preview-page__main :deep(.page-card__description) {
  color: rgba(219, 232, 248, 0.72);
}

.monitor-preview-page__controls :deep(.page-card__body) {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px 18px 18px;
}

.monitor-preview-page__toolbar {
  display: flex;
  justify-content: flex-start;
}

.camera-tree {
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.camera-tree__filters {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0;
}

.camera-tree__search-box {
  position: relative;
}

.camera-tree__search {
  height: 36px;
  width: 100%;
  padding: 0 38px 0 10px;
  border: 1px solid #c7ccd3;
  border-radius: 2px;
  background: #ffffff;
  color: #4d4d4d;
  font-size: 13px;
}

.camera-tree__search-icon {
  position: absolute;
  top: 50%;
  right: 10px;
  color: #656d78;
  font-size: 16px;
  transform: translateY(-50%);
  pointer-events: none;
}

.camera-tree__empty {
  padding: 24px;
  border-radius: 14px;
  text-align: center;
  color: #74889d;
  background: linear-gradient(180deg, #f8fbfe 0%, #f2f7fb 100%);
}

.camera-tree__content {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 0;
  overflow-y: scroll;
  overflow-x: hidden;
  padding-top: 2px;
  padding-right: 2px;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.camera-tree__content::-webkit-scrollbar {
  width: 8px;
}

.camera-tree__content::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(118, 146, 177, 0.45);
}

.camera-tree__content::-webkit-scrollbar-track {
  background: transparent;
}

.camera-tree__branch {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.camera-tree__node-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.camera-tree__children {
  display: flex;
  flex-direction: column;
  gap: 0;
  margin-left: 18px;
}

.camera-tree__children--zone {
  margin-left: 18px;
}

.camera-tree__children--leaf {
  margin-left: 18px;
}

.camera-tree__node {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  min-height: 32px;
  padding: 0 4px;
  border: none;
  border-radius: 0;
  color: #4d4d4d;
  background: transparent;
  cursor: pointer;
  transition: color 0.18s ease;
}

.camera-tree__node:hover {
  color: #222222;
}

.camera-tree__node-main {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.camera-tree__node--factory {
  color: #4a4a4a;
}

.camera-tree__node--zone,
.camera-tree__node--group {
  color: #4a4a4a;
}

.camera-tree__caret {
  color: #666666;
  font-size: 12px;
  transition: transform 0.18s ease;
}

.camera-tree__caret--expanded {
  transform: rotate(90deg);
}

.camera-tree__node-icon {
  flex: 0 0 auto;
  color: #6a7a8d;
  font-size: 14px;
}

.camera-tree__node-icon--factory {
  color: #6e7f90;
}

.camera-tree__node-label {
  overflow: hidden;
  color: inherit;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.camera-tree__node-meta {
  flex: 0 0 auto;
  color: #8a8a8a;
  font-size: 12px;
}

.camera-tree__action {
  flex: 0 0 auto;
  min-width: 42px;
  height: 24px;
  padding: 0 8px;
  border: 1px solid rgba(0, 82, 217, 0.18);
  border-radius: 6px;
  color: #0052d9;
  background: rgba(0, 82, 217, 0.06);
  cursor: pointer;
  font-size: 12px;
  line-height: 22px;
  transition: background-color 0.18s ease, border-color 0.18s ease, color 0.18s ease;
}

.camera-tree__action:hover {
  border-color: rgba(0, 82, 217, 0.3);
  background: rgba(0, 82, 217, 0.12);
}

.camera-tree__leaf {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
  min-height: 32px;
  padding: 0 8px 0 6px;
  border: none;
  border-radius: 0;
  color: #4d4d4d;
  background: transparent;
  cursor: pointer;
  transition: background-color 0.18s ease, color 0.18s ease;
}

.camera-tree__leaf:hover {
  background: rgba(243, 39, 39, 0.08);
}

.camera-tree__leaf--active {
  color: #ffffff;
  background: #ef2020;
}

.camera-tree__leaf-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.camera-tree__leaf-icon {
  flex: 0 0 auto;
  color: currentColor;
  font-size: 12px;
}

.camera-tree__leaf-label {
  min-width: 0;
  overflow: hidden;
  font-size: 13px;
  font-weight: 500;
  text-align: left;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.camera-tree__leaf-status {
  flex: 0 0 auto;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: #a7b5c4;
}

.camera-tree__leaf-status--success {
  background: #2ab06f;
}

.camera-tree__leaf-status--danger {
  background: #eb5555;
}

.camera-tree__leaf-status--warning {
  background: #ffb648;
}

.camera-tree__leaf-status--default {
  background: #a7b5c4;
}

.preview-toolbar {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  width: 100%;
  overflow-x: auto;
}

.preview-toolbar__group {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(147, 185, 229, 0.12);
}

.preview-toolbar__inline {
  align-items: center;
  display: flex;
  gap: 8px;
  flex-wrap: nowrap;
}

.preview-toolbar__page {
  color: rgba(219, 232, 248, 0.78);
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.preview-toolbar__group--actions {
  margin-left: auto;
}

.preview-toolbar :deep(.app-button) {
  min-height: 36px;
  padding: 0 12px;
  font-size: 13px;
}

.player-grid {
  display: grid;
  gap: 18px;
  flex: 1;
  min-height: 0;
  height: 100%;
  align-content: stretch;
}

.player-grid--1 {
  grid-template-columns: 1fr;
  grid-auto-rows: minmax(0, 1fr);
}

.player-grid--4 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-auto-rows: minmax(0, 1fr);
}

.player-grid--9 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-auto-rows: minmax(0, 1fr);
}

.player-grid--hik-1,
.player-grid--hik-4,
.player-grid--hik-9 {
  display: block;
}

.player-grid--hik-1 {
  min-height: 0;
}

.player-grid--hik-4,
.player-grid--hik-9 {
  min-height: 0;
}

.player-grid--9 .player-grid__slot {
  justify-content: center;
}

.player-grid__slot {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 0;
  height: 100%;
  padding: 12px;
  border-radius: 18px;
  background:
    radial-gradient(circle at top, rgba(63, 141, 255, 0.1), transparent 36%),
    linear-gradient(180deg, #08172a 0%, #0c213c 100%);
  border: 1px solid rgba(87, 125, 163, 0.24);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03), 0 16px 36px rgba(4, 14, 28, 0.24);
  cursor: pointer;
}

.player-grid__slot--active {
  border-color: rgba(92, 174, 255, 0.68);
  box-shadow: inset 0 1px 0 rgba(128, 192, 255, 0.12), 0 20px 42px rgba(10, 28, 52, 0.34);
}

.player-grid__slot :deep(.video-player),
.player-grid__slot :deep(.video-player__screen),
.player-grid__slot :deep(.video-player__native),
.player-grid__slot :deep(.video-player__placeholder) {
  min-height: 0;
}

.player-grid--9 .player-grid__slot :deep(.video-player) {
  height: auto;
  max-height: 100%;
}

.player-grid__slot :deep(.video-player__screen) {
  padding: 16px;
}

.player-grid__slot :deep(.video-player__frame-hold) {
  inset: 16px;
  width: calc(100% - 32px);
  height: calc(100% - 32px);
}

@media (max-width: 1024px) {
  .monitor-preview-page {
    height: auto;
    overflow: visible;
  }

  .monitor-preview-page__layout,
  .player-grid--4,
  .player-grid--9 {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .preview-toolbar {
    flex-wrap: wrap;
  }
}
</style>

