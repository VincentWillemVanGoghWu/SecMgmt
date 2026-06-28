<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue"
import { useRoute } from "vue-router"
import { ElMessageBox } from "element-plus"
import {
  ArrowRight,
  FullScreen,
  HomeFilled,
  LocationFilled,
  Picture,
  Search,
  VideoPause,
  VideoPlay,
  VideoCameraFilled,
} from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import HikWebControlPlaybackPlayer from "../../components/video/HikWebControlPlaybackPlayer.vue"
import VideoPlayer from "../../components/video/VideoPlayer.vue"
import { listFactoriesApi, listZonesApi } from "../../api/master-data"
import { listChannelsApi } from "../../api/recorder"
import {
  downloadPlaybackFileApi,
  getChannelLiveWebControlConfigApi,
  getPlaybackUrlApi,
  searchPlaybackSegmentsApi,
  seekPlaybackApi,
  stopPlaybackApi,
} from "../../api/video"
import { playbackDownloadState } from "../../services/playbackDownloadStore"
import type { FactoryRecord, ZoneRecord } from "../../types/master-data"
import type { RecorderChannelRecord } from "../../types/recorder"
import type {
  LiveWebControlConfig,
  PlaybackMode,
  PlaybackSegmentRecord,
  PlaybackTimelineSpan,
  StreamProfile,
  StreamType,
} from "../../types/video"

const route = useRoute()

const loading = ref(false)
const searching = ref(false)
const playbackLoading = ref(false)
const snapshotLoading = ref(false)
const playerErrorSuppressed = ref(false)
const hikDisplayOffsetSeconds = ref(0)
const playerRef = ref<HTMLElement | null>(null)
const videoPlayerRef = ref<{
  captureCurrentFrame: () => string | null
  getPlaybackCurrentDateTime: () => string
} | null>(null)
const hikPlaybackPlayerRef = ref<{
  searchRecords: (params: { startTime: string; endTime: string; streamType?: 1 | 2 }) => Promise<HikSdkPlaybackRecord[]>
  startPlayback: (params: { startTime: string; endTime: string; streamType?: 1 | 2 }) => Promise<void>
  stopPlayback: (options?: { silent?: boolean }) => Promise<void>
  setPlaybackLoading: (message?: string) => void
  clearPlaybackLoading: () => void
  pausePlayback: () => Promise<void>
  resumePlayback: () => Promise<void>
  playFast: () => Promise<void>
  playSlow: () => Promise<void>
  getOSDTime: () => Promise<string>
  captureCurrentFrame: () => Promise<string | null>
  downloadRecord: (params: { playbackUri: string; fileName: string; dateDir?: boolean }) => Promise<void>
  downloadRecordByTime: (params: {
    playbackUri: string
    fileName: string
    startTime: string
    endTime: string
    dateDir?: boolean
  }) => Promise<void>
} | null>(null)

interface HikSdkPlaybackRecord {
  startTime: string
  endTime: string
  playbackUri: string
  fileName: string
  recordType: string
}

interface PlaybackTimelineEntry extends PlaybackSegmentRecord {
  actualStartTime?: string
  actualEndTime?: string
  playbackUri?: string
  fileName?: string
}

interface PlaybackDaySegment extends PlaybackSegmentRecord {
  axisStartTime: string
  axisEndTime: string
  playbackStartTime: string
  playbackEndTime: string
  firstRecordStartTime: string
  lastRecordEndTime: string
  sourceCount: number
  totalRecordedSeconds: number
  spans: PlaybackTimelineSpan[]
}

interface PlaybackTreeSourceItem {
  key: string
  id: number
  factoryId: number
  factoryName: string
  zoneId: number | null
  zoneName: string
  recorderId: number
  recorderName: string
  name: string
  status: string
  channel: RecorderChannelRecord
}

interface PlaybackTreeZone {
  id: number | string
  zoneName: string
  leaves: PlaybackTreeSourceItem[]
}

interface DatePickerCellState {
  dayjs?: { format: (pattern: string) => string }
  text?: number | string
  type?: "normal" | "today" | "week" | "next-month" | "prev-month"
  isCurrent?: boolean
  isSelected?: boolean
  inRange?: boolean
  start?: boolean
  end?: boolean
}

const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const channels = ref<RecorderChannelRecord[]>([])
const segments = ref<PlaybackDaySegment[]>([])
const selectedSegment = ref<PlaybackDaySegment | null>(null)
const hikPlaybackConfig = ref<LiveWebControlConfig | null>(null)
const sourceKeyword = ref("")
const selectedSourceKey = ref("")
const expandedFactoryKey = ref<string | null>(null)
const expandedZoneKey = ref<string | null>(null)

const queryForm = reactive({
  factoryId: "",
  zoneId: "",
  recorderId: "",
  channelId: "",
  range: [] as string[],
  recordType: "all",
})

const playback = reactive({
  streamType: "hik-sdk" as StreamType,
  streamProfile: "main" as StreamProfile,
  playUrl: null as string | null,
  isPlaying: false,
  isPaused: false,
  speed: 1,
  playbackMode: "hik" as PlaybackMode,
  seekOffsetSeconds: 0,
  seekTargetSeconds: null as number | null,
  message: "请选择录像片段后开始回放",
  snapshotUrl: null as string | null,
})
const activePlaybackMode = ref<PlaybackMode | null>(null)
const PLAYBACK_SPEED_STEPS = [0.5, 1, 2, 4, 8] as const
const downloadState = playbackDownloadState
const recordedDayKeys = ref<Set<string>>(new Set())
const recordedDaysLoading = ref(false)
const rangePickerVisible = ref(false)
const rangePickerPanelDates = ref<Date[]>([])
let recordedDaysRequestToken = 0
const recordedDaysCache = new Map<string, string[]>()

const playbackChannelSources = computed<PlaybackTreeSourceItem[]>(() =>
  channels.value
    .filter((item) => item.enabled)
    .map((channel) => ({
      key: `channel-${channel.id}`,
      id: channel.id,
      factoryId: channel.factoryId,
      factoryName: channel.factoryName,
      zoneId: channel.zoneId ?? null,
      zoneName: channel.zoneName || "未分区",
      recorderId: channel.recorderId,
      recorderName: channel.recorderName,
      name: channel.name,
      status: channel.status,
      channel,
    })),
)

const normalizedSourceKeyword = computed(() => sourceKeyword.value.trim().toLowerCase())
const filteredPlaybackSources = computed(() =>
  playbackChannelSources.value.filter((item) => {
    const targetText = `${item.recorderName} ${item.name} ${item.zoneName} ${item.factoryName}`.toLowerCase()
    return !normalizedSourceKeyword.value || targetText.includes(normalizedSourceKeyword.value)
  }),
)

const buildFactoryKey = (factoryId: number) => `factory-${factoryId}`
const buildZoneKey = (factoryId: number, zoneId: number | string) => `zone-${factoryId}-${zoneId}`

const playbackChannelTree = computed(() =>
  factories.value
    .map((factory) => {
      const factorySources = filteredPlaybackSources.value.filter((item) => item.factoryId === factory.id)
      const factoryZones: PlaybackTreeZone[] = zones.value
        .filter((zone) => zone.factoryId === factory.id)
        .map((zone) => ({
          id: zone.id,
          zoneName: zone.zoneName,
          leaves: factorySources
            .filter((item) => item.zoneId === zone.id)
            .sort((left, right) => left.name.localeCompare(right.name, "zh-CN")),
        }))
        .filter((zone) => zone.leaves.length)

      const unassignedSources = factorySources.filter((item) => !item.zoneId)
      if (unassignedSources.length) {
        factoryZones.push({
          id: `unassigned-${factory.id}`,
          zoneName: "未分区",
          leaves: [...unassignedSources].sort((left, right) => left.name.localeCompare(right.name, "zh-CN")),
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

const toggleFactoryExpand = (factoryId: number) => {
  const nextKey = buildFactoryKey(factoryId)
  if (expandedFactoryKey.value === nextKey) {
    expandedFactoryKey.value = null
    expandedZoneKey.value = null
    return
  }
  expandedFactoryKey.value = nextKey
  expandedZoneKey.value = null
}

const toggleZoneExpand = (factoryId: number, zoneId: number | string) => {
  const nextKey = buildZoneKey(factoryId, zoneId)
  expandedZoneKey.value = expandedZoneKey.value === nextKey ? null : nextKey
}

const getSourceDisplayName = (source: PlaybackTreeSourceItem) => {
  const cameraName = source.channel.cameraName?.trim() || source.name
  return `${source.recorderName}${source.recorderName && cameraName ? " + " : ""}${cameraName}`.trim()
}

const handleSelectPlaybackSource = (source: PlaybackTreeSourceItem) => {
  const previousChannelId = queryForm.channelId
  queryForm.factoryId = String(source.factoryId)
  queryForm.zoneId = source.zoneId ? String(source.zoneId) : ""
  queryForm.recorderId = String(source.recorderId)
  queryForm.channelId = String(source.id)
  queryForm.recordType = "all"
  selectedSourceKey.value = source.key
  if (previousChannelId && previousChannelId !== queryForm.channelId) {
    clearSegments()
  }
}

const handleBlockedRangePickerClick = () => {
  if (queryForm.channelId) {
    return
  }
  void showPlaybackNotice("请先选择摄像机通道后再选择日期时间。", "warning")
}

const activeCameraName = computed(() => selectedSegment.value?.cameraName || "未选择录像")
const activeLocation = computed(() => {
  if (!selectedSegment.value) return ""
  return `${selectedSegment.value.recorderName} / ${selectedSegment.value.channelName}`
})

const currentDownloadSegment = computed(() =>
  segments.value.find((item) => getSegmentKey(item) === downloadState.segmentKey) ?? null,
)

const selectedTimelineSpans = computed(() => selectedSegment.value?.spans ?? [])
const playerNoticeMessage = computed(() => (playback.isPlaying ? "" : playback.message))

const selectedSource = computed(() =>
  queryForm.channelId
    ? playbackChannelSources.value.find((item) => item.id === Number(queryForm.channelId)) ?? null
    : null,
)

const ensureHikPlaybackConfig = async () => {
  const source = selectedSource.value
  if (!source) {
    throw new Error("请先选择通道。")
  }
  if (hikPlaybackConfig.value?.channelId === source.id) {
    return hikPlaybackConfig.value
  }
  const config = await getChannelLiveWebControlConfigApi(source.id, {
    streamProfile: "main",
  })
  hikPlaybackConfig.value = {
    ...config,
    streamType: 1,
    streamProfile: "main",
  }
  return hikPlaybackConfig.value
}

const formatDateKey = (date: Date) =>
  `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`

const formatMonthKey = (date: Date) => `${date.getFullYear()}-${pad(date.getMonth() + 1)}`

const parseDateValue = (value: string | Date | null | undefined) => {
  if (!value) {
    return null
  }
  const date = value instanceof Date ? new Date(value.getTime()) : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

const buildMonthRange = (monthDate: Date) => {
  const start = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1, 0, 0, 0, 0)
  const end = new Date(monthDate.getFullYear(), monthDate.getMonth() + 1, 0, 23, 59, 59, 0)
  return {
    monthKey: formatMonthKey(start),
    start,
    end,
  }
}

const addMonths = (date: Date, count: number) => {
  const nextDate = new Date(date.getTime())
  nextDate.setMonth(nextDate.getMonth() + count)
  return nextDate
}

const getInitialRangePickerPanelDates = () => {
  const baseDate = parseDateValue(queryForm.range[0]) ?? new Date()
  return [baseDate, addMonths(baseDate, 1)]
}

const normalizeRangePickerPanelDates = (value: unknown) => {
  if (!Array.isArray(value)) {
    return []
  }
  return value
    .map((item) => parseDateValue(item instanceof Date ? item : null))
    .filter((item): item is Date => Boolean(item))
}

const resetRecordedDayHighlights = () => {
  recordedDaysRequestToken += 1
  recordedDayKeys.value = new Set()
  rangePickerPanelDates.value = []
  recordedDaysLoading.value = false
}

const collectRecordedDaysFromRecords = (records: HikSdkPlaybackRecord[], rangeStart: Date, rangeEnd: Date) => {
  const dayKeys = new Set<string>()
  for (const record of records) {
    const recordStart = parseDateValue(record.startTime.replace(" ", "T").replace("Z", ""))
    const recordEnd = parseDateValue(record.endTime.replace(" ", "T").replace("Z", ""))
    if (!recordStart || !recordEnd || recordEnd < rangeStart || recordStart > rangeEnd) {
      continue
    }
    const effectiveStart = new Date(Math.max(recordStart.getTime(), rangeStart.getTime()))
    const effectiveEnd = new Date(Math.min(recordEnd.getTime(), rangeEnd.getTime()))
    const cursor = new Date(effectiveStart.getFullYear(), effectiveStart.getMonth(), effectiveStart.getDate())
    const lastDay = new Date(effectiveEnd.getFullYear(), effectiveEnd.getMonth(), effectiveEnd.getDate())
    while (cursor.getTime() <= lastDay.getTime()) {
      dayKeys.add(formatDateKey(cursor))
      cursor.setDate(cursor.getDate() + 1)
    }
  }
  return [...dayKeys].sort()
}

const buildRecordedDaysCacheKey = (monthKey: string) => {
  const channelId = queryForm.channelId || "none"
  return `${channelId}|${monthKey}`
}

const updateRecordedDayKeysForPanel = (panelDates: Date[]) => {
  const monthKeys = [...new Set(panelDates.map((item) => formatMonthKey(item)))]
  const nextDayKeys = new Set<string>()
  monthKeys.forEach((monthKey) => {
    const cacheKey = buildRecordedDaysCacheKey(monthKey)
    const days = recordedDaysCache.get(cacheKey) ?? []
    days.forEach((day) => nextDayKeys.add(day))
  })
  recordedDayKeys.value = nextDayKeys
  rangePickerPanelDates.value = panelDates
}

const loadRecordedDaysForPanel = async (panelDates?: Date[]) => {
  if (playback.playbackMode !== "hik" || !queryForm.channelId) {
    resetRecordedDayHighlights()
    return
  }
  const normalizedPanelDates = panelDates && panelDates.length ? panelDates : (
    rangePickerPanelDates.value.length ? rangePickerPanelDates.value : getInitialRangePickerPanelDates()
  )
  updateRecordedDayKeysForPanel(normalizedPanelDates)

  const missingMonths = normalizedPanelDates
    .map((item) => buildMonthRange(item))
    .filter((item, index, list) =>
      list.findIndex((candidate) => candidate.monthKey === item.monthKey) === index
      && !recordedDaysCache.has(buildRecordedDaysCacheKey(item.monthKey)))

  if (!missingMonths.length) {
    return
  }

  const requestToken = ++recordedDaysRequestToken
  recordedDaysLoading.value = true
  try {
    await nextTick()
    if (!hikPlaybackPlayerRef.value) {
      return
    }
    const config = await ensureHikPlaybackConfig()
    const recordsByMonth = await Promise.all(
      missingMonths.map(async ({ start, end }) =>
        await hikPlaybackPlayerRef.value?.searchRecords({
          startTime: toSdkSearchDateTime(toLocalDateTimeString(start)),
          endTime: toSdkSearchDateTime(toLocalDateTimeString(end)),
          streamType: config.streamType,
        }) ?? []),
    )
    if (requestToken !== recordedDaysRequestToken) {
      return
    }
    missingMonths.forEach(({ monthKey, start, end }, index) => {
      recordedDaysCache.set(
        buildRecordedDaysCacheKey(monthKey),
        collectRecordedDaysFromRecords(recordsByMonth[index] ?? [], start, end),
      )
    })
    updateRecordedDayKeysForPanel(normalizedPanelDates)
  } catch {
    if (requestToken === recordedDaysRequestToken) {
      recordedDayKeys.value = new Set()
    }
  } finally {
    if (requestToken === recordedDaysRequestToken) {
      recordedDaysLoading.value = false
    }
  }
}

const handleRangePickerVisibleChange = (visible: boolean) => {
  rangePickerVisible.value = visible
  if (!visible) {
    return
  }
  void loadRecordedDaysForPanel(getInitialRangePickerPanelDates())
}

const handleRangePickerPanelChange = (value: unknown) => {
  const panelDates = normalizeRangePickerPanelDates(value)
  if (!panelDates.length) {
    return
  }
  void loadRecordedDaysForPanel(panelDates)
}

const isRecordedDateCell = (cell: DatePickerCellState) => {
  if (!cell.dayjs || cell.type === "prev-month" || cell.type === "next-month") {
    return false
  }
  return recordedDayKeys.value.has(cell.dayjs.format("YYYY-MM-DD"))
}

const isSelectedDateCell = (cell: DatePickerCellState) => Boolean(cell.isSelected || cell.start || cell.end)

watch(
  playbackChannelTree,
  (tree) => {
    if (!tree.length) {
      expandedFactoryKey.value = null
      expandedZoneKey.value = null
      return
    }

    const matchedSource = queryForm.channelId
      ? playbackChannelSources.value.find((item) => item.id === Number(queryForm.channelId))
      : null

    const activeFactory = matchedSource
      ? tree.find((factory) => factory.id === matchedSource.factoryId) ?? tree[0]
      : tree.find((factory) => buildFactoryKey(factory.id) === expandedFactoryKey.value) ?? tree[0]
    expandedFactoryKey.value = buildFactoryKey(activeFactory.id)

    const activeZone = matchedSource
      ? activeFactory.zones.find((zone) => zone.id === (matchedSource.zoneId ?? `unassigned-${activeFactory.id}`)) ?? activeFactory.zones[0]
      : activeFactory.zones.find((zone) => buildZoneKey(activeFactory.id, zone.id) === expandedZoneKey.value) ?? activeFactory.zones[0]
    expandedZoneKey.value = buildZoneKey(activeFactory.id, activeZone.id)

    if (matchedSource) {
      selectedSourceKey.value = matchedSource.key
    }
  },
  { immediate: true },
)

watch(
  () => queryForm.channelId,
  (channelId, previousChannelId) => {
    if (channelId === previousChannelId) {
      return
    }
    hikPlaybackConfig.value = null
    resetRecordedDayHighlights()
    if (playback.playbackMode === "hik" && channelId && rangePickerVisible.value) {
      void loadRecordedDaysForPanel(getInitialRangePickerPanelDates())
    }
  },
)

watch(
  () => playback.playbackMode,
  (mode) => {
    resetRecordedDayHighlights()
    if (mode === "hik" && queryForm.channelId && rangePickerVisible.value) {
      void loadRecordedDaysForPanel(getInitialRangePickerPanelDates())
    }
  },
)

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const normalizePlaybackMessage = (message?: string | null, fallback = "") => {
  const normalized = String(message ?? "").trim()
  if (!normalized) {
    return fallback
  }
  if (normalized.includes("Mock回放流已停止")) {
    return fallback
  }
  return normalized
}

const getPlaybackSpeedStepIndex = (value: number) => {
  const safeValue = Number.isFinite(value) ? value : 1
  const exactIndex = PLAYBACK_SPEED_STEPS.findIndex((item) => item === safeValue)
  if (exactIndex >= 0) {
    return exactIndex
  }
  let nearestIndex = 0
  let nearestDistance = Number.POSITIVE_INFINITY
  PLAYBACK_SPEED_STEPS.forEach((item, index) => {
    const distance = Math.abs(item - safeValue)
    if (distance < nearestDistance) {
      nearestDistance = distance
      nearestIndex = index
    }
  })
  return nearestIndex
}

const getNextPlaybackSpeed = (value: number) =>
  PLAYBACK_SPEED_STEPS[Math.min(PLAYBACK_SPEED_STEPS.length - 1, getPlaybackSpeedStepIndex(value) + 1)]

const getPreviousPlaybackSpeed = (value: number) =>
  PLAYBACK_SPEED_STEPS[Math.max(0, getPlaybackSpeedStepIndex(value) - 1)]

const resetPlaybackSpeed = () => {
  playback.speed = 1
}

type NoticeType = "success" | "warning" | "error" | "info"

const noticeTitleMap: Record<NoticeType, string> = {
  success: "提示",
  warning: "提示",
  error: "错误",
  info: "提示",
}

const showPlaybackNotice = (message: string, type: NoticeType = "info") =>
  ElMessageBox.alert(message, noticeTitleMap[type], {
    appendTo: "body",
    center: true,
    closeOnClickModal: true,
    confirmButtonText: "确定",
    customClass: "playback-page__notice-dialog",
    type,
  }).catch(() => undefined)

const pad = (value: number) => String(value).padStart(2, "0")

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

const toLocalDateTimeString = (value: Date) =>
  `${value.getFullYear()}-${pad(value.getMonth() + 1)}-${pad(value.getDate())}T${pad(value.getHours())}:${pad(value.getMinutes())}:${pad(value.getSeconds())}`

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

const normalizeDateTimeValue = (value: string) => {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return toLocalDateTimeString(date)
}

const splitEntryByDay = (
  item: PlaybackTimelineEntry,
  queryStartMs: number,
  queryEndMs: number,
) => {
  const itemStartMs = new Date(item.startTime).getTime()
  const itemEndMs = new Date(item.endTime).getTime()
  if (Number.isNaN(itemStartMs) || Number.isNaN(itemEndMs) || itemEndMs <= itemStartMs) {
    return []
  }
  const clippedStartMs = Math.max(itemStartMs, queryStartMs)
  const clippedEndMs = Math.min(itemEndMs, queryEndMs)
  if (clippedEndMs <= clippedStartMs) {
    return []
  }

  const chunks: Array<PlaybackTimelineEntry & { clippedStartTime: string; clippedEndTime: string; dayKey: string; dayStartMs: number }> = []
  let cursorMs = clippedStartMs
  while (cursorMs < clippedEndMs) {
    const dayStart = new Date(cursorMs)
    dayStart.setHours(0, 0, 0, 0)
    const nextDayStart = new Date(dayStart)
    nextDayStart.setDate(nextDayStart.getDate() + 1)
    const chunkEndMs = Math.min(clippedEndMs, nextDayStart.getTime())
    chunks.push({
      ...item,
      clippedStartTime: toLocalDateTimeString(new Date(cursorMs)),
      clippedEndTime: toLocalDateTimeString(new Date(chunkEndMs)),
      dayKey: `${item.channelId}-${dayStart.getTime()}`,
      dayStartMs: dayStart.getTime(),
    })
    cursorMs = chunkEndMs
  }
  return chunks
}

const aggregateSegmentsToDaily = (
  items: PlaybackTimelineEntry[],
  queryStartTime: string,
  queryEndTime: string,
): PlaybackDaySegment[] => {
  const buckets = new Map<
    string,
    PlaybackDaySegment & {
      _recordTypes: Set<string>
      _spanMap: Map<string, PlaybackTimelineSpan>
    }
  >()
  const queryStart = new Date(queryStartTime)
  const queryEnd = new Date(queryEndTime)
  if (Number.isNaN(queryStart.getTime()) || Number.isNaN(queryEnd.getTime()) || queryEnd <= queryStart) {
    return []
  }

  for (const item of items) {
    const chunks = splitEntryByDay(item, queryStart.getTime(), queryEnd.getTime())
    for (const chunk of chunks) {
      const dayStart = new Date(chunk.dayStartMs)
      const dayEnd = new Date(chunk.dayStartMs)
      dayEnd.setDate(dayEnd.getDate() + 1)
      const axisStart = new Date(Math.max(dayStart.getTime(), queryStart.getTime()))
      const axisEnd = new Date(Math.min(dayEnd.getTime(), queryEnd.getTime()))
      if (axisEnd <= axisStart) {
        continue
      }

      const existed = buckets.get(chunk.dayKey)
      const nextSpan: PlaybackTimelineSpan = {
        startTime: chunk.clippedStartTime,
        endTime: chunk.clippedEndTime,
        actualStartTime: chunk.actualStartTime || chunk.startTime,
        actualEndTime: chunk.actualEndTime || chunk.endTime,
        recordType: chunk.recordType,
        available: chunk.available,
        playbackUri: chunk.playbackUri,
        fileName: chunk.fileName,
      }

      if (!existed) {
        const spanMap = new Map<string, PlaybackTimelineSpan>()
        spanMap.set(`${nextSpan.startTime}|${nextSpan.endTime}|${nextSpan.playbackUri || ""}`, nextSpan)
        buckets.set(chunk.dayKey, {
          ...chunk,
          startTime: toLocalDateTimeString(axisStart),
          endTime: toLocalDateTimeString(axisEnd),
          axisStartTime: toLocalDateTimeString(axisStart),
          axisEndTime: toLocalDateTimeString(axisEnd),
          playbackStartTime: nextSpan.startTime,
          playbackEndTime: nextSpan.endTime,
          firstRecordStartTime: nextSpan.startTime,
          lastRecordEndTime: nextSpan.endTime,
          sourceCount: 1,
          totalRecordedSeconds: Math.max(1, Math.round((new Date(nextSpan.endTime).getTime() - new Date(nextSpan.startTime).getTime()) / 1000)),
          spans: [nextSpan],
          _recordTypes: new Set([chunk.recordType]),
          _spanMap: spanMap,
        })
        continue
      }

      existed.available = existed.available || chunk.available
      existed.sourceCount += 1
      existed._recordTypes.add(chunk.recordType)
      if (new Date(nextSpan.startTime).getTime() < new Date(existed.firstRecordStartTime).getTime()) {
        existed.firstRecordStartTime = nextSpan.startTime
      }
      if (new Date(nextSpan.endTime).getTime() > new Date(existed.lastRecordEndTime).getTime()) {
        existed.lastRecordEndTime = nextSpan.endTime
      }
      const spanKey = `${nextSpan.startTime}|${nextSpan.endTime}|${nextSpan.playbackUri || ""}`
      if (!existed._spanMap.has(spanKey)) {
        existed._spanMap.set(spanKey, nextSpan)
        existed.spans.push(nextSpan)
        existed.totalRecordedSeconds += Math.max(
          1,
          Math.round((new Date(nextSpan.endTime).getTime() - new Date(nextSpan.startTime).getTime()) / 1000),
        )
      }
    }
  }

  return Array.from(buckets.values())
    .map(({ _recordTypes, _spanMap, ...item }) => ({
      ...item,
      spans: [...item.spans].sort((left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime()),
      recordType: _recordTypes.size === 1 ? Array.from(_recordTypes)[0] : "all",
    }))
    .sort((left, right) => new Date(left.axisStartTime).getTime() - new Date(right.axisStartTime).getTime())
}

const hydrateRouteQuery = () => {
  const query = route.query
  if (typeof query.factoryId === "string") queryForm.factoryId = query.factoryId
  if (typeof query.zoneId === "string") queryForm.zoneId = query.zoneId
  if (typeof query.recorderId === "string") queryForm.recorderId = query.recorderId
  if (typeof query.channelId === "string") queryForm.channelId = query.channelId
  if (typeof query.recordType === "string") queryForm.recordType = query.recordType
  const startTime = typeof query.startTime === "string" ? query.startTime : ""
  const endTime = typeof query.endTime === "string" ? query.endTime : ""
  if (startTime && endTime) {
    queryForm.range = [normalizeDateTimeValue(startTime), normalizeDateTimeValue(endTime)]
  }
}

const loadBaseData = async () => {
  loading.value = true
  try {
    ;[factories.value, zones.value, channels.value] = await Promise.all([
      listFactoriesApi(),
      listZonesApi(),
      listChannelsApi(),
    ])
  } finally {
    loading.value = false
  }
}

const ensureDefaultRange = () => {
  if (queryForm.range.length === 2) return
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  const end = new Date()
  end.setHours(23, 59, 59, 0)
  queryForm.range = [
    toLocalDateTimeString(start),
    toLocalDateTimeString(end),
  ]
}

const getSegmentKey = (segment: PlaybackDaySegment) => `${segment.channelId}-${segment.axisStartTime}`

const formatSegmentRange = (segment: PlaybackDaySegment) => {
  const start = new Date(segment.axisStartTime || segment.startTime)
  const end = new Date(segment.axisEndTime || segment.endTime)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    return `${segment.axisStartTime || segment.startTime} 至 ${segment.axisEndTime || segment.endTime}`
  }
  return `${start.getFullYear()}-${pad(start.getMonth() + 1)}-${pad(start.getDate())} ${pad(start.getHours())}:${pad(start.getMinutes())} 至 ${end.getFullYear()}-${pad(end.getMonth() + 1)}-${pad(end.getDate())} ${pad(end.getHours())}:${pad(end.getMinutes())}`
}

let downloadEstimateTimer: number | null = null
let playerErrorSuppressTimer: number | null = null
let playbackRequestId = 0
let pendingSeekOffset: number | null = null
let flushingPendingSeek = false
let hikClockTimer: number | null = null
let hikClockOsdSyncTimer: number | null = null
let hikClockAnchorOffsetSeconds = 0
let hikClockAnchorStartedAtMs = 0
let hikClockOsdSyncToken = 0
let hikSpeedReplayToken = 0
const playbackSeekPrerollSeconds = 12

const suppressPlayerErrorsBriefly = (durationMs = 15000) => {
  playerErrorSuppressed.value = true
  if (playerErrorSuppressTimer !== null) {
    window.clearTimeout(playerErrorSuppressTimer)
  }
  playerErrorSuppressTimer = window.setTimeout(() => {
    playerErrorSuppressed.value = false
    playerErrorSuppressTimer = null
  }, durationMs)
}

const clearPendingSeek = () => {
  pendingSeekOffset = null
}

const stopHikClock = () => {
  if (hikClockTimer !== null) {
    window.clearInterval(hikClockTimer)
    hikClockTimer = null
  }
  cancelScheduledHikClockOsdSync()
}

const cancelScheduledHikClockOsdSync = () => {
  if (hikClockOsdSyncTimer !== null) {
    window.clearTimeout(hikClockOsdSyncTimer)
    hikClockOsdSyncTimer = null
  }
  hikClockOsdSyncToken += 1
}

const waitForHikPlaybackSettle = (delayMs: number) =>
  new Promise<void>((resolve) => {
    window.setTimeout(resolve, Math.max(0, delayMs))
  })

const getHikSpeedStepDelayMs = (speed: number) => {
  const normalizedSpeed = PLAYBACK_SPEED_STEPS[getPlaybackSpeedStepIndex(speed)]
  if (normalizedSpeed >= 8) {
    return 650
  }
  if (normalizedSpeed >= 4) {
    return 420
  }
  return 260
}

const getHikClockSyncDelayMs = (speed: number) => {
  const normalizedSpeed = PLAYBACK_SPEED_STEPS[getPlaybackSpeedStepIndex(speed)]
  if (normalizedSpeed >= 8) {
    return 1200
  }
  if (normalizedSpeed >= 4) {
    return 850
  }
  if (normalizedSpeed > 1) {
    return 450
  }
  return 0
}

const isHighSpeedHikPlayback = (speed: number) =>
  PLAYBACK_SPEED_STEPS[getPlaybackSpeedStepIndex(speed)] >= 4

const shouldUseHikOsdSync = (speed: number) => !isHighSpeedHikPlayback(speed)

const parseHikOsdOffsetSeconds = (value: string, segment: PlaybackDaySegment) => {
  if (!value) {
    return null
  }
  const normalizedValue = value.trim().replace(" ", "T")
  const osdTimeMs = new Date(normalizedValue).getTime()
  const axisStartMs = resolveAxisStartMs(segment)
  const axisEndMs = resolveAxisEndMs(segment)
  if (Number.isNaN(osdTimeMs) || !axisStartMs || axisEndMs <= axisStartMs) {
    return null
  }
  const maxOffset = Math.max(0, Math.round((axisEndMs - axisStartMs) / 1000))
  return Math.min(maxOffset, Math.max(0, Math.round((osdTimeMs - axisStartMs) / 1000)))
}

const syncHikClock = () => {
  if (!selectedSegment.value) {
    hikDisplayOffsetSeconds.value = 0
    return
  }
  const maxOffset = resolveSegmentDurationSeconds(selectedSegment.value)
  if (playback.isPaused || !playback.isPlaying) {
    hikDisplayOffsetSeconds.value = Math.min(hikClockAnchorOffsetSeconds, maxOffset)
    return
  }
  const elapsedSeconds = Math.max(0, (Date.now() - hikClockAnchorStartedAtMs) / 1000)
  hikDisplayOffsetSeconds.value = Math.min(maxOffset, hikClockAnchorOffsetSeconds + elapsedSeconds * playback.speed)
}

const syncHikClockFromOsd = async (options: {
  delayMs?: number
  retries?: number
  retryDelayMs?: number
  allowBackwardJump?: boolean
} = {}) => {
  if (
    !selectedSegment.value
    || !hikPlaybackPlayerRef.value?.getOSDTime
    || !playback.isPlaying
    || playback.isPaused
    || activePlaybackMode.value !== "hik"
  ) {
    return null
  }
  const attempts = Math.max(1, (options.retries ?? 0) + 1)
  const retryDelayMs = options.retryDelayMs ?? 250
  const allowBackwardJump = options.allowBackwardJump ?? false
  if ((options.delayMs ?? 0) > 0) {
    await waitForHikPlaybackSettle(options.delayMs ?? 0)
  }
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      const osdTime = await hikPlaybackPlayerRef.value.getOSDTime()
      if (!selectedSegment.value) {
        return null
      }
      const resolvedOffsetSeconds = parseHikOsdOffsetSeconds(osdTime || "", selectedSegment.value)
      if (resolvedOffsetSeconds !== null) {
        if (!allowBackwardJump && resolvedOffsetSeconds < Math.floor(hikDisplayOffsetSeconds.value)) {
          return null
        }
        setHikClockAnchor(resolvedOffsetSeconds)
        return resolvedOffsetSeconds
      }
    } catch {
      // Ignore transient OSD time read failures.
    }
    if (attempt < attempts - 1) {
      await waitForHikPlaybackSettle(retryDelayMs)
    }
  }
  return null
}

const setHikClockAnchor = (offsetSeconds: number) => {
  hikClockAnchorOffsetSeconds = Math.max(0, offsetSeconds)
  hikClockAnchorStartedAtMs = Date.now()
  hikDisplayOffsetSeconds.value = hikClockAnchorOffsetSeconds
}

const scheduleHikClockOsdSync = (delayMs = 900) => {
  if (!hikPlaybackPlayerRef.value?.getOSDTime || !shouldUseHikOsdSync(playback.speed)) {
    return
  }
  cancelScheduledHikClockOsdSync()
  const syncToken = ++hikClockOsdSyncToken
  hikClockOsdSyncTimer = window.setTimeout(() => {
    hikClockOsdSyncTimer = null
    void (async () => {
      if (
        syncToken !== hikClockOsdSyncToken
        || !selectedSegment.value
        || !playback.isPlaying
        || playback.isPaused
        || activePlaybackMode.value !== "hik"
        || !shouldUseHikOsdSync(playback.speed)
      ) {
        return
      }
      await syncHikClockFromOsd()
    })()
  }, delayMs)
}

const restartHikClock = () => {
  if (hikClockTimer !== null) {
    window.clearInterval(hikClockTimer)
    hikClockTimer = null
  }
  syncHikClock()
  if (!playback.isPlaying || playback.isPaused || activePlaybackMode.value !== "hik") {
    return
  }
  hikClockTimer = window.setInterval(syncHikClock, 500)
}

const replayHikSpeed = async (targetSpeed: number) => {
  const replayToken = ++hikSpeedReplayToken
  const normalizedTarget = PLAYBACK_SPEED_STEPS[getPlaybackSpeedStepIndex(targetSpeed)]
  if (!hikPlaybackPlayerRef.value || normalizedTarget === 1) {
    return true
  }
  await waitForHikPlaybackSettle(getHikSpeedStepDelayMs(2) + 120)
  if (replayToken !== hikSpeedReplayToken) {
    return false
  }
  let currentSpeed = 1
  while (currentSpeed < normalizedTarget) {
    await hikPlaybackPlayerRef.value.playFast()
    currentSpeed *= 2
    if (currentSpeed < normalizedTarget) {
      await waitForHikPlaybackSettle(getHikSpeedStepDelayMs(currentSpeed * 2))
    }
    if (replayToken !== hikSpeedReplayToken) {
      return false
    }
  }
  while (currentSpeed > normalizedTarget) {
    await hikPlaybackPlayerRef.value.playSlow()
    currentSpeed /= 2
    if (currentSpeed > normalizedTarget) {
      await waitForHikPlaybackSettle(getHikSpeedStepDelayMs(currentSpeed))
    }
    if (replayToken !== hikSpeedReplayToken) {
      return false
    }
  }
  return true
}

const getPlaybackSpeedMessage = (speed: number, mode: PlaybackMode) =>
  `${mode === "hik" ? "HIK 回放速度" : "当前回放速度"} ${speed.toFixed(1)}x`

const cancelHikSpeedReplay = () => {
  hikSpeedReplayToken += 1
}

const flushPendingSeek = () => {
  if (
    flushingPendingSeek
    || pendingSeekOffset === null
    || playbackLoading.value
    || !selectedSegment.value
    || !playback.isPlaying
  ) {
    return
  }
  const nextOffset = pendingSeekOffset
  pendingSeekOffset = null
  flushingPendingSeek = true
  void nextTick(() => {
    flushingPendingSeek = false
    void handleSeekPlayback(nextOffset)
  })
}

const formatDurationText = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return "计算中..."
  }
  const totalSeconds = Math.ceil(seconds)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const remainSeconds = totalSeconds % 60
  if (hours > 0) {
    return `${hours}小时${String(minutes).padStart(2, "0")}分`
  }
  if (minutes > 0) {
    return `${minutes}分${String(remainSeconds).padStart(2, "0")}秒`
  }
  return `${remainSeconds}秒`
}

const resolveSegmentDurationSeconds = (segment: PlaybackDaySegment) => {
  const start = new Date(segment.axisStartTime)
  const end = new Date(segment.axisEndTime)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end <= start) {
    return 0
  }
  return Math.max(1, Math.round((end.getTime() - start.getTime()) / 1000))
}

const resolveAxisStartMs = (segment: PlaybackDaySegment) => {
  const value = new Date(segment.axisStartTime).getTime()
  return Number.isNaN(value) ? 0 : value
}

const resolveAxisEndMs = (segment: PlaybackDaySegment) => {
  const value = new Date(segment.axisEndTime).getTime()
  return Number.isNaN(value) ? resolveAxisStartMs(segment) : value
}

const buildPlayablePoint = (segment: PlaybackDaySegment, requestedOffsetSeconds = 0) => {
  const axisStartMs = resolveAxisStartMs(segment)
  const axisEndMs = resolveAxisEndMs(segment)
  const spans = [...segment.spans].sort((left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime())
  if (!spans.length) {
    return null
  }

  const rawTargetMs = axisStartMs + Math.max(0, Math.floor(requestedOffsetSeconds)) * 1000
  const clampedTargetMs = Math.min(Math.max(axisStartMs, rawTargetMs), Math.max(axisStartMs, axisEndMs - 1000))

  let bestSpan = spans[0]
  let snappedMs = new Date(bestSpan.startTime).getTime()
  let bestDistance = Number.POSITIVE_INFINITY

  for (const span of spans) {
    const spanStartMs = new Date(span.startTime).getTime()
    const spanEndMs = new Date(span.endTime).getTime()
    if (Number.isNaN(spanStartMs) || Number.isNaN(spanEndMs) || spanEndMs <= spanStartMs) {
      continue
    }
    const insideMs = Math.min(Math.max(clampedTargetMs, spanStartMs), Math.max(spanStartMs, spanEndMs - 1000))
    const distance = clampedTargetMs < spanStartMs
      ? spanStartMs - clampedTargetMs
      : clampedTargetMs >= spanEndMs
        ? clampedTargetMs - Math.max(spanStartMs, spanEndMs - 1000)
        : 0
    if (distance < bestDistance) {
      bestDistance = distance
      bestSpan = span
      snappedMs = insideMs
    }
    if (distance === 0) {
      break
    }
  }

  const snappedOffsetSeconds = Math.max(0, Math.round((snappedMs - axisStartMs) / 1000))
  const spanStartMs = new Date(bestSpan.startTime).getTime()
  const spanEndMs = new Date(bestSpan.endTime).getTime()
  return {
    span: bestSpan,
    targetTime: toLocalDateTimeString(new Date(snappedMs)),
    targetOffsetSeconds: snappedOffsetSeconds,
    spanStartOffsetSeconds: Math.max(0, Math.round((spanStartMs - axisStartMs) / 1000)),
    spanEndOffsetSeconds: Math.max(0, Math.round((spanEndMs - axisStartMs) / 1000)),
  }
}

const resolveFirstPlayableOffsetSeconds = (segment: PlaybackDaySegment) => {
  const firstSpan = [...segment.spans].sort((left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime())[0]
  if (!firstSpan) {
    return 0
  }
  return Math.max(0, Math.round((new Date(firstSpan.startTime).getTime() - resolveAxisStartMs(segment)) / 1000))
}

const estimatePrepareSeconds = (segmentDurationSeconds: number) => {
  if (segmentDurationSeconds <= 0) {
    return 60
  }
  return Math.max(30, Math.min(30 * 60, Math.round(segmentDurationSeconds * 0.45 + 20)))
}

const updatePreparingEstimate = () => {
  if (!downloadState.active || !downloadState.preparing || !downloadState.startedAtMs) {
    return
  }
  const elapsedSeconds = Math.max(0, (Date.now() - downloadState.startedAtMs) / 1000)
  const remainingSeconds = downloadState.expectedPrepareSeconds - elapsedSeconds
  if (remainingSeconds > 0) {
    downloadState.estimatedRemainingText = `预计剩余 ${formatDurationText(remainingSeconds)}`
    downloadState.message = `正在准备录像下载，${downloadState.estimatedRemainingText}`
    return
  }
  downloadState.estimatedRemainingText = `已用时 ${formatDurationText(elapsedSeconds)}，仍在准备`
  downloadState.message = `正在准备录像下载，${downloadState.estimatedRemainingText}`
}

const startDownloadEstimateTimer = () => {
  if (downloadEstimateTimer !== null) {
    window.clearInterval(downloadEstimateTimer)
  }
  updatePreparingEstimate()
  downloadEstimateTimer = window.setInterval(updatePreparingEstimate, 1000)
}

const stopDownloadEstimateTimer = () => {
  if (downloadEstimateTimer === null) {
    return
  }
  window.clearInterval(downloadEstimateTimer)
  downloadEstimateTimer = null
}

const updateTransferEstimate = (loaded: number, total?: number) => {
  if (!downloadState.transferStartedAtMs) {
    downloadState.transferStartedAtMs = Date.now()
  }
  const elapsedSeconds = Math.max(1, (Date.now() - downloadState.transferStartedAtMs) / 1000)
  if (total && total > 0 && loaded > 0) {
    const remainingBytes = Math.max(0, total - loaded)
    const bytesPerSecond = loaded / elapsedSeconds
    const remainingSeconds = bytesPerSecond > 0 ? remainingBytes / bytesPerSecond : 0
    downloadState.estimatedRemainingText = remainingSeconds > 0
      ? `预计剩余 ${formatDurationText(remainingSeconds)}`
      : "即将完成"
    return
  }
  const progress = Math.max(downloadState.progress, 1)
  const remainingSeconds = elapsedSeconds * ((100 - progress) / progress)
  downloadState.estimatedRemainingText = remainingSeconds > 0
    ? `预计剩余 ${formatDurationText(remainingSeconds)}`
    : "即将完成"
}

const clearSegments = () => {
  segments.value = []
  selectedSegment.value = null
  hikPlaybackConfig.value = null
  playback.playUrl = null
  playback.isPlaying = false
  playback.isPaused = false
  playback.seekOffsetSeconds = 0
  playback.seekTargetSeconds = null
  playback.snapshotUrl = null
  playback.message = "请选择录像片段后开始回放"
  activePlaybackMode.value = null
  hikDisplayOffsetSeconds.value = 0
  stopHikClock()
  playbackLoading.value = false
  playbackRequestId += 1
  clearPendingSeek()
}

const toSdkSearchDateTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value.replace("T", " ").slice(0, 19)
  }
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const mapHikRecordType = (value: string) => {
  if (value === "motion") return "alarm"
  if (value === "timing") return "timed"
  if (value === "manual") return "manual"
  return value || "all"
}

const buildHikSegments = (
  items: HikSdkPlaybackRecord[],
  source: PlaybackTreeSourceItem,
  queryStartTime: string,
  queryEndTime: string,
): PlaybackDaySegment[] =>
  aggregateSegmentsToDaily(
    items.map((item) => ({
      startTime: normalizeDateTimeValue(item.startTime.replace(" ", "T").replace("Z", "")),
      endTime: normalizeDateTimeValue(item.endTime.replace(" ", "T").replace("Z", "")),
      actualStartTime: normalizeDateTimeValue(item.startTime.replace(" ", "T").replace("Z", "")),
      actualEndTime: normalizeDateTimeValue(item.endTime.replace(" ", "T").replace("Z", "")),
      channelId: source.id,
      channelName: source.name,
      recorderId: source.recorderId,
      recorderName: source.recorderName,
      cameraId: source.channel.cameraId ?? undefined,
      cameraName: source.channel.cameraName ?? undefined,
      recordType: mapHikRecordType(item.recordType),
      available: true,
      playbackUri: item.playbackUri,
      fileName: item.fileName,
    })),
    queryStartTime,
    queryEndTime,
  )

const searchSegments = async (validateChannel = true, autoPlayFirst = false) => {
  ensureDefaultRange()
  if (validateChannel && !queryForm.channelId) {
    await showPlaybackNotice("请选择通道后再查询录像", "warning")
    return
  }
  const [startTime, endTime] = queryForm.range
  searching.value = true
  try {
    if (playback.playbackMode === "hik") {
      const source = selectedSource.value
      const config = await ensureHikPlaybackConfig()
      const records = await hikPlaybackPlayerRef.value?.searchRecords({
        startTime: toSdkSearchDateTime(startTime),
        endTime: toSdkSearchDateTime(endTime),
        streamType: config.streamType,
      })
      segments.value = source && records ? buildHikSegments(records, source, startTime, endTime) : []
    } else {
      const rawSegments = await searchPlaybackSegmentsApi({
        recorder_id: queryForm.recorderId ? Number(queryForm.recorderId) : undefined,
        channel_id: queryForm.channelId ? Number(queryForm.channelId) : undefined,
        start_time: startTime,
        end_time: endTime,
        record_type: queryForm.recordType,
      })
      segments.value = aggregateSegmentsToDaily(rawSegments, startTime, endTime)
    }
    if (segments.value[0]) {
      if (autoPlayFirst) {
        await handleSelectAndPlay(segments.value[0])
      } else {
        selectSegment(segments.value[0])
      }
    } else {
      clearSegments()
    }
    if (!segments.value.length) {
      void showPlaybackNotice(
        playback.playbackMode === "hik"
          ? "未找到录像片段，请扩大时间范围或检查设备侧录像。"
          : "未找到录像片段，请扩大时间范围，并检查后端 runtime/playback-search 搜索日志。",
        "warning",
      )
    }
  } catch (error) {
    void showPlaybackNotice(resolveErrorMessage(error, "查询录像片段失败"), "error")
  } finally {
    searching.value = false
  }
}

const resetQuery = async () => {
  queryForm.factoryId = ""
  queryForm.zoneId = ""
  queryForm.recorderId = ""
  queryForm.channelId = ""
  selectedSourceKey.value = ""
  sourceKeyword.value = ""
  queryForm.recordType = "all"
  queryForm.range = []
  ensureDefaultRange()
  clearSegments()
}

const handleTreeSourceDoubleClick = async (source: PlaybackTreeSourceItem) => {
  handleSelectPlaybackSource(source)
  await searchSegments(false, true)
}

const selectSegment = (segment: PlaybackDaySegment) => {
  cancelHikSpeedReplay()
  const firstPlayableOffsetSeconds = resolveFirstPlayableOffsetSeconds(segment)
  selectedSegment.value = segment
  playback.playUrl = null
  playback.isPlaying = false
  playback.isPaused = false
  resetPlaybackSpeed()
  playback.seekOffsetSeconds = firstPlayableOffsetSeconds
  playback.seekTargetSeconds = null
  playback.snapshotUrl = null
  playback.message = playback.playbackMode === "hik" ? "已选择录像片段，点击播放启动 HIK 回放" : "已选择录像片段，点击播放获取回放地址"
  activePlaybackMode.value = null
  hikDisplayOffsetSeconds.value = firstPlayableOffsetSeconds
  stopHikClock()
  playbackLoading.value = false
  playbackRequestId += 1
  clearPendingSeek()
}

const handleSelectAndPlay = async (segment: PlaybackDaySegment) => {
  selectSegment(segment)
  await handlePlay()
}

const isActiveSegment = (segment: PlaybackDaySegment) =>
  selectedSegment.value?.axisStartTime === segment.axisStartTime && selectedSegment.value?.channelId === segment.channelId

const isSegmentPlaying = (segment: PlaybackDaySegment) =>
  isActiveSegment(segment) && (playbackLoading.value || playback.isPlaying || playback.isPaused || Boolean(playback.playUrl))

const handleSegmentPrimaryAction = async (segment: PlaybackDaySegment) => {
  if (isSegmentPlaying(segment)) {
    return
  }
  await handleSelectAndPlay(segment)
}

const addSecondsToDateTime = (value: string, seconds: number) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return toLocalDateTimeString(new Date(date.getTime() + Math.max(0, seconds) * 1000))
}

const resolvePlaybackSnapshotOsdTime = async () => {
  if (!selectedSegment.value) {
    return ""
  }
  if (activePlaybackMode.value === "hik") {
    try {
      return await hikPlaybackPlayerRef.value?.getOSDTime() || ""
    } catch {
      return addSecondsToDateTime(selectedSegment.value.axisStartTime, Math.floor(hikDisplayOffsetSeconds.value))
    }
  }
  return videoPlayerRef.value?.getPlaybackCurrentDateTime()
    || addSecondsToDateTime(selectedSegment.value.axisStartTime, Math.floor(playback.seekOffsetSeconds))
}

const buildPlaybackSnapshotFileName = async (dataUrl: string) => {
  const recorderName = sanitizeSnapshotFilePart(selectedSegment.value?.recorderName || "录像机")
  const channelName = sanitizeSnapshotFilePart(selectedSegment.value?.channelName || "通道")
  const osdTime = formatSnapshotTime(await resolvePlaybackSnapshotOsdTime()) || formatSnapshotTime(new Date().toISOString())
  const extension = dataUrl.startsWith("data:image/jpeg") ? "jpg" : "png"
  return `${recorderName}+${channelName}+${osdTime}.${extension}`
}

const resolvePlaybackOffsetSeconds = (segment: PlaybackDaySegment, offsetSeconds: number) => {
  const start = new Date(segment.axisStartTime)
  const end = new Date(segment.axisEndTime)
  if (Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end <= start) {
    return 0
  }
  const durationSeconds = Math.max(1, Math.floor((end.getTime() - start.getTime()) / 1000))
  return Math.min(Math.max(0, Math.floor(offsetSeconds)), Math.max(0, durationSeconds - 1))
}

const handlePlay = async (
  offsetSeconds = 0,
  options: { usePreroll?: boolean; resetHikSpeedOnSeek?: boolean } = {},
) => {
  if (!selectedSegment.value) {
    playback.message = "请先选择录像片段"
    return
  }

  const requestId = ++playbackRequestId
  playbackLoading.value = true
  try {
    if (playback.playbackMode === "hik") {
      cancelHikSpeedReplay()
      const config = await ensureHikPlaybackConfig()
      const requestedPlaybackSpeed = playback.speed
      const shouldResetHikSpeedOnSeek = Boolean(options.resetHikSpeedOnSeek) && requestedPlaybackSpeed !== 1
      const targetOffsetSeconds = resolvePlaybackOffsetSeconds(selectedSegment.value, offsetSeconds)
      const targetPoint = buildPlayablePoint(selectedSegment.value, targetOffsetSeconds)
      if (!targetPoint) {
        throw new Error("当前日期没有可播放的录像片段。")
      }
      await hikPlaybackPlayerRef.value?.startPlayback({
        startTime: toSdkSearchDateTime(targetPoint.targetTime),
        endTime: toSdkSearchDateTime(targetPoint.span.endTime),
        streamType: config.streamType,
      })
      if (requestId !== playbackRequestId) {
        return
      }
      playback.playUrl = null
      playback.isPlaying = true
      playback.isPaused = false
      activePlaybackMode.value = "hik"
      playback.seekOffsetSeconds = targetPoint.targetOffsetSeconds
      playback.seekTargetSeconds = null
      if (shouldResetHikSpeedOnSeek) {
        resetPlaybackSpeed()
      }
      setHikClockAnchor(targetPoint.targetOffsetSeconds)
      if (playback.speed !== 1) {
        hikPlaybackPlayerRef.value?.setPlaybackLoading("HIK 鍊嶉€熸仮澶嶄腑...")
        try {
          await replayHikSpeed(playback.speed)
        } catch {
          playback.message = "HIK 倍速恢复失败，请重试调整倍速"
        } finally {
          hikPlaybackPlayerRef.value?.clearPlaybackLoading()
        }
      }
      let syncedOffsetSeconds: number | null = null
      if (shouldUseHikOsdSync(playback.speed)) {
        syncedOffsetSeconds = await syncHikClockFromOsd({
          delayMs: getHikClockSyncDelayMs(playback.speed),
          retries: playback.speed >= 4 ? 1 : 0,
          allowBackwardJump: true,
        })
        if (syncedOffsetSeconds === null) {
          setHikClockAnchor(targetPoint.targetOffsetSeconds)
        }
      }
      restartHikClock()
      if (shouldUseHikOsdSync(playback.speed)) {
        scheduleHikClockOsdSync(getHikClockSyncDelayMs(playback.speed) + 600)
      }
      const playbackMessage = targetPoint.targetOffsetSeconds !== targetOffsetSeconds
        ? "该时段无录像，已跳转到最近录像位置"
        : normalizePlaybackMessage("", getPlaybackSpeedMessage(playback.speed, "hik"))
      playback.message = shouldResetHikSpeedOnSeek
        ? `${playbackMessage}，已为稳定性重置为 1.0x`
        : playbackMessage
      return
    }

    playback.streamType = "hik-sdk"
    const targetOffsetSeconds = resolvePlaybackOffsetSeconds(selectedSegment.value, offsetSeconds)
    const targetPoint = buildPlayablePoint(selectedSegment.value, targetOffsetSeconds)
    if (!targetPoint) {
      throw new Error("当前日期没有可播放的录像片段。")
    }
    const isNativeMode = playback.playbackMode === "native"
    if (isNativeMode) {
      suppressPlayerErrorsBriefly()
    }
    const prerollSeconds = isNativeMode || options.usePreroll === false
      ? 0
      : Math.min(playbackSeekPrerollSeconds, targetPoint.targetOffsetSeconds)
    const streamOffsetSeconds = isNativeMode ? targetPoint.spanStartOffsetSeconds : Math.max(0, targetPoint.targetOffsetSeconds - prerollSeconds)
    const playbackStartTime = addSecondsToDateTime(selectedSegment.value.axisStartTime, streamOffsetSeconds)
    const data = await getPlaybackUrlApi({
      recorder_id: selectedSegment.value.recorderId,
      channel_id: selectedSegment.value.channelId,
      camera_id: selectedSegment.value.cameraId ?? undefined,
      start_time: playbackStartTime,
      end_time: selectedSegment.value.axisEndTime,
      stream_type: "hik-sdk",
      stream_profile: "main",
      prebuffer_seconds: prerollSeconds,
      playback_mode: playback.playbackMode,
    })
    if (requestId !== playbackRequestId) {
      return
    }
    playback.playUrl = `${data.playUrl}${data.playUrl.includes("?") ? "&" : "?"}_ts=${Date.now()}`
    playback.streamType = "hik-sdk"
    playback.streamProfile = "main"
    playback.isPlaying = true
    playback.isPaused = false
    activePlaybackMode.value = playback.playbackMode
    playback.seekOffsetSeconds = streamOffsetSeconds
    playback.seekTargetSeconds = targetPoint.targetOffsetSeconds
    if (isNativeMode) {
      suppressPlayerErrorsBriefly()
    }
    playback.message = normalizePlaybackMessage(
      data.diagnosticMessage,
      targetPoint.targetOffsetSeconds !== targetOffsetSeconds
        ? "该时段无录像，已跳转到最近录像位置"
        : getPlaybackSpeedMessage(playback.speed, playback.playbackMode),
    )
  } catch (error) {
    if (requestId === playbackRequestId) {
      playback.message = resolveErrorMessage(error, "开始回放失败")
    }
  } finally {
    if (requestId === playbackRequestId) {
      playbackLoading.value = false
      flushPendingSeek()
    }
  }
}

const handlePauseToggle = () => {
  if (!playback.isPlaying) {
    playback.message = "当前没有正在回放的录像"
    return
  }
  if (activePlaybackMode.value === "hik") {
    playbackLoading.value = true
    const action = playback.isPaused ? hikPlaybackPlayerRef.value?.resumePlayback() : hikPlaybackPlayerRef.value?.pausePlayback()
    Promise.resolve(action)
      .then(() => {
        syncHikClock()
        playback.isPaused = !playback.isPaused
        if (playback.isPaused) {
          hikClockAnchorOffsetSeconds = hikDisplayOffsetSeconds.value
          stopHikClock()
        } else {
          setHikClockAnchor(hikDisplayOffsetSeconds.value)
          restartHikClock()
          scheduleHikClockOsdSync()
        }
        playback.message = playback.isPaused ? "HIK 回放已暂停" : "HIK 回放继续播放"
      })
      .catch((error) => {
        playback.message = resolveErrorMessage(error, playback.isPaused ? "HIK 回放继续失败" : "HIK 回放暂停失败")
      })
      .finally(() => {
        playbackLoading.value = false
      })
    return
  }
  playback.isPaused = !playback.isPaused
  playback.message = playback.isPaused ? "回放已暂停" : "回放继续播放"
}

const handleStop = async () => {
  if (!selectedSegment.value) {
    playback.message = "请先选择录像片段"
    return
  }
  const channelId = selectedSegment.value.channelId
  playbackRequestId += 1
  playbackLoading.value = false
  cancelHikSpeedReplay()
  playback.isPlaying = false
  playback.isPaused = false
  playback.playUrl = null
  resetPlaybackSpeed()
  playback.seekOffsetSeconds = 0
  playback.seekTargetSeconds = null
  hikDisplayOffsetSeconds.value = 0
  stopHikClock()
  playback.message = "回放已停止"
  clearPendingSeek()
  await nextTick()
  try {
    if (activePlaybackMode.value === "hik") {
      await hikPlaybackPlayerRef.value?.stopPlayback({ silent: true })
      playback.message = "HIK 回放已停止"
      activePlaybackMode.value = null
      return
    }
    const result = await stopPlaybackApi(channelId)
    playback.message = normalizePlaybackMessage(result.message, "回放已停止")
    activePlaybackMode.value = null
  } catch (error) {
    playback.message = resolveErrorMessage(error, "停止回放失败")
  }
}

const handleSpeedUp = () => {
  if (!playback.isPlaying) {
    playback.message = "请先开始回放"
    return
  }
  const targetSpeed = getNextPlaybackSpeed(playback.speed)
  if (targetSpeed === playback.speed) {
    playback.message = getPlaybackSpeedMessage(playback.speed, playback.playbackMode)
    return
  }
  if (activePlaybackMode.value === "hik") {
    cancelHikSpeedReplay()
    cancelScheduledHikClockOsdSync()
    playbackLoading.value = true
    syncHikClock()
    Promise.resolve(hikPlaybackPlayerRef.value?.playFast())
      .then(async () => {
        playback.speed = targetSpeed
        let syncedOffsetSeconds: number | null = null
        if (shouldUseHikOsdSync(targetSpeed)) {
          syncedOffsetSeconds = await syncHikClockFromOsd({
            delayMs: getHikClockSyncDelayMs(targetSpeed),
            retries: targetSpeed >= 4 ? 1 : 0,
          })
        }
        if (syncedOffsetSeconds === null) {
          setHikClockAnchor(hikDisplayOffsetSeconds.value)
        }
        restartHikClock()
        if (shouldUseHikOsdSync(targetSpeed)) {
          scheduleHikClockOsdSync(getHikClockSyncDelayMs(targetSpeed) + 600)
        }
        playback.message = getPlaybackSpeedMessage(playback.speed, "hik")
      })
      .catch((error) => {
        playback.message = resolveErrorMessage(error, "HIK 快进失败")
      })
      .finally(() => {
        playbackLoading.value = false
      })
    return
  }
  playback.speed = targetSpeed
  playback.message = getPlaybackSpeedMessage(playback.speed, playback.playbackMode)
}

const handleSlowDown = () => {
  if (!playback.isPlaying) {
    playback.message = "请先开始回放"
    return
  }
  const targetSpeed = getPreviousPlaybackSpeed(playback.speed)
  if (targetSpeed === playback.speed) {
    playback.message = getPlaybackSpeedMessage(playback.speed, playback.playbackMode)
    return
  }
  if (activePlaybackMode.value === "hik") {
    cancelHikSpeedReplay()
    cancelScheduledHikClockOsdSync()
    playbackLoading.value = true
    syncHikClock()
    Promise.resolve(hikPlaybackPlayerRef.value?.playSlow())
      .then(async () => {
        playback.speed = targetSpeed
        let syncedOffsetSeconds: number | null = null
        if (shouldUseHikOsdSync(targetSpeed)) {
          syncedOffsetSeconds = await syncHikClockFromOsd({
            delayMs: getHikClockSyncDelayMs(targetSpeed),
          })
        }
        if (syncedOffsetSeconds === null) {
          setHikClockAnchor(hikDisplayOffsetSeconds.value)
        }
        restartHikClock()
        if (shouldUseHikOsdSync(targetSpeed)) {
          scheduleHikClockOsdSync(getHikClockSyncDelayMs(targetSpeed) + 600)
        }
        playback.message = getPlaybackSpeedMessage(playback.speed, "hik")
      })
      .catch((error) => {
        playback.message = resolveErrorMessage(error, "HIK 慢放失败")
      })
      .finally(() => {
        playbackLoading.value = false
      })
    return
  }
  playback.speed = targetSpeed
  playback.message = getPlaybackSpeedMessage(playback.speed, playback.playbackMode)
}

const handleSnapshot = async () => {
  if (!playback.isPlaying) {
    void showPlaybackNotice("请先开始回放", "warning")
    return
  }
  snapshotLoading.value = true
  try {
    const dataUrl =
      activePlaybackMode.value === "hik"
        ? (await hikPlaybackPlayerRef.value?.captureCurrentFrame()) ?? null
        : videoPlayerRef.value?.captureCurrentFrame() ?? null
    if (!dataUrl) {
      void showPlaybackNotice("当前画面尚未准备完成，暂时无法截图。", "warning")
      return
    }
    const anchor = document.createElement("a")
    anchor.href = dataUrl
    anchor.download = await buildPlaybackSnapshotFileName(dataUrl)
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    playback.snapshotUrl = dataUrl
    playback.message = "已截取当前回放画面"
  } catch (error) {
    void showPlaybackNotice(resolveErrorMessage(error, "截图失败"), "error")
  } finally {
    snapshotLoading.value = false
  }
}

const handleFullscreen = async (nextFullscreen?: boolean) => {
  if (!playerRef.value) return
  const isFullscreen = document.fullscreenElement === playerRef.value
  const shouldEnterFullscreen = typeof nextFullscreen === "boolean" ? nextFullscreen : !document.fullscreenElement
  if (shouldEnterFullscreen === isFullscreen) {
    return
  }
  if (shouldEnterFullscreen && !document.fullscreenElement) {
    await playerRef.value.requestFullscreen()
    return
  }
  if (!shouldEnterFullscreen && document.fullscreenElement) {
    await document.exitFullscreen()
  }
}

const handleNativeSeekPlayback = async (offsetSeconds: number) => {
  if (!selectedSegment.value) {
    return
  }
  if (!playback.isPlaying) {
    await handlePlay(0, { usePreroll: false })
    if (!playback.isPlaying) {
      return
    }
  }

  const requestId = ++playbackRequestId
  const previousPlayUrl = playback.playUrl
  const previousSeekOffsetSeconds = playback.seekOffsetSeconds
  const previousSeekTargetSeconds = playback.seekTargetSeconds
  suppressPlayerErrorsBriefly()
  playbackLoading.value = true
  playback.isPaused = true
  try {
    const targetOffsetSeconds = resolvePlaybackOffsetSeconds(selectedSegment.value, offsetSeconds)
    const targetPoint = buildPlayablePoint(selectedSegment.value, targetOffsetSeconds)
    if (!targetPoint) {
      throw new Error("当前日期没有可播放的录像片段。")
    }
    const data = await seekPlaybackApi({
      channel_id: selectedSegment.value.channelId,
      target_time: targetPoint.targetTime,
      prebuffer_seconds: 2,
    })
    if (requestId !== playbackRequestId) {
      return
    }
    playback.playUrl = `${data.playUrl}${data.playUrl.includes("?") ? "&" : "?"}_ts=${Date.now()}`
    playback.streamType = "hik-sdk"
    playback.streamProfile = "main"
    playback.isPlaying = true
    playback.isPaused = false
    playback.seekOffsetSeconds = targetPoint.targetOffsetSeconds
    playback.seekTargetSeconds = null
    suppressPlayerErrorsBriefly()
    playback.message = normalizePlaybackMessage(
      data.diagnosticMessage,
      targetPoint.targetOffsetSeconds !== targetOffsetSeconds
        ? "该时段无录像，已跳转到最近录像位置"
        : getPlaybackSpeedMessage(playback.speed, playback.playbackMode),
    )
  } catch (error) {
    if (requestId === playbackRequestId) {
      if (previousPlayUrl) {
        playback.playUrl = previousPlayUrl
        playback.isPlaying = true
        playback.isPaused = false
        playback.seekOffsetSeconds = previousSeekOffsetSeconds
        playback.seekTargetSeconds = previousSeekTargetSeconds
      } else {
        playback.isPlaying = false
        playback.isPaused = false
        playback.playUrl = null
      }
      playback.message = resolveErrorMessage(error, "SDK 定位失败")
    }
  } finally {
    if (requestId === playbackRequestId) {
      playbackLoading.value = false
      flushPendingSeek()
    }
  }
}

const handleSeekPlayback = async (offsetSeconds: number) => {
  if (!selectedSegment.value) {
    return
  }
  if (activePlaybackMode.value === "hik") {
    await handlePlay(offsetSeconds, { usePreroll: false, resetHikSpeedOnSeek: true })
    return
  }
  if (playbackLoading.value) {
    pendingSeekOffset = offsetSeconds
    return
  }
  if (playback.playbackMode === "native") {
    await handleNativeSeekPlayback(offsetSeconds)
    return
  }
  await handlePlay(offsetSeconds, { usePreroll: true })
}

const handleSeekPlaybackEnd = async (_offsetSeconds: number) => {
  if (!selectedSegment.value) {
    return
  }
  const channelId = selectedSegment.value.channelId
  cancelHikSpeedReplay()
  playbackRequestId += 1
  playbackLoading.value = false
  playback.streamType = "hik-sdk"
  playback.isPlaying = false
  playback.isPaused = false
  playback.playUrl = null
  resetPlaybackSpeed()
  playback.seekOffsetSeconds = 0
  playback.seekTargetSeconds = null
  playback.message = "已到录像结束，回放已停止"
  clearPendingSeek()
  await nextTick()
  try {
    if (activePlaybackMode.value === "hik") {
      await hikPlaybackPlayerRef.value?.stopPlayback({ silent: true })
      activePlaybackMode.value = null
      return
    }
    await stopPlaybackApi(channelId)
    activePlaybackMode.value = null
  } catch (error) {
    playback.message = resolveErrorMessage(error, "停止回放失败")
  }
}

const cancelDownload = () => {
  if (!downloadState.active || !downloadState.controller) return
  downloadState.controller.abort()
}

const buildHikDownloadFileName = (segment: PlaybackDaySegment, span?: PlaybackTimelineSpan, index = 0) => {
  const startText = span?.actualStartTime || span?.startTime || segment.firstRecordStartTime || segment.axisStartTime || segment.startTime || Date.now().toString()
  const endText = span?.actualEndTime || span?.endTime || segment.lastRecordEndTime || segment.axisEndTime || segment.endTime || Date.now().toString()
  const suffix = segment.spans.length > 1 ? `-part-${index + 1}` : ""
  const normalizeText = (value: string) => {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) {
      return sanitizeSnapshotFilePart(value.replace("T", " ")).replace(/[-_: ]/g, "")
    }
    return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
  }
  const sanitize = (value: string) =>
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
  const recorderName = sanitize(segment.recorderName || "录像机")
  const channelName = sanitize(segment.channelName || "通道")
  return `${recorderName}+${channelName}(${normalizeText(startText)} 至 ${normalizeText(endText)})${suffix}`
}

const handleDownload = async (segment: PlaybackDaySegment) => {
  if (!segment.available) {
    void showPlaybackNotice("当前录像片段不可下载", "warning")
    return
  }
  if (downloadState.active) {
    void showPlaybackNotice("当前已有下载任务，请先等待完成或取消", "warning")
    return
  }

  const start = new Date(segment.axisStartTime)
  const end = new Date(segment.axisEndTime)
  if (!Number.isNaN(start.getTime()) && !Number.isNaN(end.getTime()) && end.getTime() - start.getTime() > 60 * 60 * 1000) {
    void showPlaybackNotice("录像过大，下载可能需要较长时间", "warning")
  }

  const controller = new AbortController()
  const segmentDurationSeconds = resolveSegmentDurationSeconds(segment)
  downloadState.active = true
  downloadState.preparing = true
  downloadState.progress = 0
  downloadState.startedAtMs = Date.now()
  downloadState.transferStartedAtMs = 0
  downloadState.expectedPrepareSeconds = estimatePrepareSeconds(segmentDurationSeconds)
  downloadState.estimatedRemainingText = `预计剩余 ${formatDurationText(downloadState.expectedPrepareSeconds)}`
  downloadState.message = `正在准备录像下载，${downloadState.estimatedRemainingText}`
  downloadState.segmentKey = getSegmentKey(segment)
  downloadState.controller = controller
  startDownloadEstimateTimer()

  try {
    if (playback.playbackMode === "hik") {
      const downloadableSpans = segment.spans.filter((item) => item.available && item.playbackUri)
      if (!downloadableSpans.length) {
        throw new Error("当前 HIK 录像片段缺少 playbackURI，无法下载。")
      }
      await ensureHikPlaybackConfig()
      stopDownloadEstimateTimer()
      downloadState.controller = null
      downloadState.preparing = true
      downloadState.progress = 35
      downloadState.estimatedRemainingText = "下载任务已提交到浏览器"
      downloadState.message = "正在启动 HIK 录像下载，请稍候..."

      for (const [index, span] of downloadableSpans.entries()) {
        const fileName = buildHikDownloadFileName(segment, span, index)
        await hikPlaybackPlayerRef.value?.downloadRecord({
          playbackUri: span.playbackUri || "",
          fileName,
          dateDir: true,
        })
      }

      downloadState.preparing = false
      downloadState.progress = 100
      downloadState.estimatedRemainingText = "请到浏览器下载列表查看进度"
      downloadState.message = `HIK 录像下载任务已启动，共 ${downloadableSpans.length} 段`
      void showPlaybackNotice(`HIK 录像下载任务已启动，共 ${downloadableSpans.length} 段，请到浏览器下载列表查看。`, "success")
      return
    }

    await downloadPlaybackFileApi(
      {
        recorder_id: segment.recorderId,
        channel_id: segment.channelId,
        camera_id: segment.cameraId ?? undefined,
        start_time: segment.firstRecordStartTime,
        end_time: segment.lastRecordEndTime,
        stream_profile: "main",
      },
      {
        signal: controller.signal,
        onProgress: (event) => {
          downloadState.preparing = false
          stopDownloadEstimateTimer()
          if (event.total && event.total > 0) {
            downloadState.progress = Math.min(100, Math.round((event.loaded / event.total) * 100))
            updateTransferEstimate(event.loaded, event.total)
            downloadState.message = `正在下载录像 ${downloadState.progress}%，${downloadState.estimatedRemainingText}`
            return
          }
          downloadState.progress = downloadState.progress >= 95 ? 95 : downloadState.progress + 5
          updateTransferEstimate(event.loaded)
          downloadState.message = `正在下载录像...，${downloadState.estimatedRemainingText}`
        },
      },
    )
    downloadState.preparing = false
    stopDownloadEstimateTimer()
    downloadState.progress = 100
    downloadState.estimatedRemainingText = "已完成"
    downloadState.message = "录像下载完成"
    void showPlaybackNotice("录像下载完成", "success")
  } catch (error) {
    const canceled = (error as { code?: string; message?: string })?.code === "ERR_CANCELED"
      || (error as { message?: string })?.message?.includes("canceled")
    if (canceled) {
      void showPlaybackNotice("已取消录像下载", "info")
    } else {
      void showPlaybackNotice(resolveErrorMessage(error, "录像下载失败"), "error")
    }
  } finally {
    stopDownloadEstimateTimer()
    downloadState.active = false
    downloadState.preparing = false
    downloadState.controller = null
    if (downloadState.progress < 100) {
      downloadState.progress = 0
      downloadState.message = ""
      downloadState.estimatedRemainingText = ""
      downloadState.startedAtMs = 0
      downloadState.transferStartedAtMs = 0
      downloadState.expectedPrepareSeconds = 0
      downloadState.segmentKey = ""
    }
  }
}

onBeforeUnmount(() => {
  stopHikClock()
  stopDownloadEstimateTimer()
  if (playerErrorSuppressTimer !== null) {
    window.clearTimeout(playerErrorSuppressTimer)
  }
})

onMounted(async () => {
  hydrateRouteQuery()
  ensureDefaultRange()
  await loadBaseData()
  if (queryForm.channelId) {
    await searchSegments(false)
  }
})
</script>

<template>
  <div class="playback-page">
    <div class="playback-page__layout">
      <div class="playback-page__main">
        <PageCard class="playback-page__controls">
          <div class="playback-toolbar">
            <div class="playback-toolbar__group playback-toolbar__group--meta">
              <span class="playback-toolbar__meta">
                {{ selectedSegment ? `${selectedSegment.recorderName} / ${selectedSegment.channelName}` : "请选择右侧录像片段" }}
              </span>
            </div>
            <div class="playback-toolbar__group playback-toolbar__group--speed">
              <span class="playback-toolbar__speed">倍速 {{ playback.speed.toFixed(1) }}x</span>
            </div>
            <div class="playback-toolbar__group playback-toolbar__group--mode">
              <select
                v-model="playback.playbackMode"
                class="playback-toolbar__mode-select"
                disabled
              >
                <option value="hik">HIK</option>
              </select>
            </div>
            <div class="playback-toolbar__actions">
              <button class="app-button app-button--success playback-page__button" :disabled="playbackLoading" @click="() => void handlePlay()">
                <el-icon><VideoPlay /></el-icon>
                <span>{{ playbackLoading ? "播放中..." : "播放" }}</span>
              </button>
              <button class="app-button app-button--warning playback-page__button" @click="handlePauseToggle">
                <el-icon><VideoPause /></el-icon>
                <span>{{ playback.isPaused ? "继续" : "暂停" }}</span>
              </button>
              <button class="app-button app-button--secondary playback-page__button" @click="handleStop">停止</button>
              <button class="app-button app-button--secondary playback-page__button" :disabled="playbackLoading" @click="handleSpeedUp">快进</button>
              <button class="app-button app-button--secondary playback-page__button" :disabled="playbackLoading" @click="handleSlowDown">慢放</button>
              <button class="app-button app-button--secondary playback-page__button" :disabled="snapshotLoading" @click="handleSnapshot">
                <el-icon><Picture /></el-icon>
                <span>{{ snapshotLoading ? "截图中..." : "截图" }}</span>
              </button>
              <button class="app-button app-button--secondary playback-page__button" @click="() => void handleFullscreen()">
                <el-icon><FullScreen /></el-icon>
                <span>全屏</span>
              </button>
            </div>
          </div>
        </PageCard>

        <div ref="playerRef" class="playback-page__player-wrapper">
          <HikWebControlPlaybackPlayer
            v-if="playback.playbackMode === 'hik'"
            ref="hikPlaybackPlayerRef"
            :config="hikPlaybackConfig"
            :message="playerNoticeMessage"
            :playback-start-time="selectedSegment?.axisStartTime || ''"
            :playback-end-time="selectedSegment?.axisEndTime || ''"
            :recorded-spans="selectedTimelineSpans"
            :current-offset-seconds="hikDisplayOffsetSeconds"
            @seek="handleSeekPlayback"
            @seek-end="handleSeekPlaybackEnd"
            @toggle-fullscreen="handleFullscreen"
          />
          <VideoPlayer
            v-else
            ref="videoPlayerRef"
            title="录像回放"
            :play-url="playback.playUrl"
            :stream-type="playback.streamType"
            :stream-profile="playback.streamProfile"
            :is-playing="playback.isPlaying"
            :is-paused="playback.isPaused"
            :playback-rate="playback.speed"
            :show-seek-bar="true"
            :playback-start-time="selectedSegment?.axisStartTime || selectedSegment?.startTime || ''"
            :playback-end-time="selectedSegment?.axisEndTime || selectedSegment?.endTime || ''"
            :recorded-spans="selectedTimelineSpans"
            :seek-base-seconds="playback.seekOffsetSeconds"
            :seek-target-seconds="playback.seekTargetSeconds"
            :camera-name="activeCameraName"
            :camera-location="activeLocation"
            :snapshot-url="playback.snapshotUrl"
            :message="playerNoticeMessage"
            :protocol-name="playback.playbackMode === 'native' ? 'SDK' : 'HLS'"
            :active="true"
            :suppress-errors="playback.playbackMode === 'native' || playbackLoading || playerErrorSuppressed"
            @seek="handleSeekPlayback"
            @seek-end="handleSeekPlaybackEnd"
          />
        </div>
      </div>

      <PageCard class="playback-page__sources">
        <div class="playback-source-panel">
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
              <div v-else-if="!playbackChannelTree.length" class="camera-tree__empty">暂无可回放通道</div>
              <div v-for="factory in playbackChannelTree" :key="factory.id" class="camera-tree__branch">
                <button class="camera-tree__node camera-tree__node--factory" @click="toggleFactoryExpand(factory.id)">
                  <span class="camera-tree__node-main">
                    <el-icon class="camera-tree__caret" :class="{ 'camera-tree__caret--expanded': isFactoryExpanded(factory.id) }"><ArrowRight /></el-icon>
                    <el-icon class="camera-tree__node-icon camera-tree__node-icon--factory"><HomeFilled /></el-icon>
                    <span class="camera-tree__node-label">{{ factory.factoryName }}</span>
                  </span>
                </button>
                <div v-if="isFactoryExpanded(factory.id)" class="camera-tree__children">
                  <div v-for="zone in factory.zones" :key="zone.id" class="camera-tree__branch">
                    <button class="camera-tree__node camera-tree__node--zone" @click="toggleZoneExpand(factory.id, zone.id)">
                      <span class="camera-tree__node-main">
                        <el-icon class="camera-tree__caret" :class="{ 'camera-tree__caret--expanded': isZoneExpanded(factory.id, zone.id) }"><ArrowRight /></el-icon>
                        <el-icon class="camera-tree__node-icon camera-tree__node-icon--zone"><LocationFilled /></el-icon>
                        <span class="camera-tree__node-label">{{ zone.zoneName }}</span>
                      </span>
                    </button>
                    <div v-if="isZoneExpanded(factory.id, zone.id)" class="camera-tree__children camera-tree__children--zone">
                      <button
                        v-for="source in zone.leaves"
                        :key="source.key"
                        class="camera-tree__leaf"
                        :class="{ 'camera-tree__leaf--active': selectedSourceKey === source.key }"
                        @click="handleSelectPlaybackSource(source)"
                        @dblclick="() => void handleTreeSourceDoubleClick(source)"
                      >
                        <span class="camera-tree__leaf-main">
                          <el-icon class="camera-tree__leaf-icon"><VideoCameraFilled /></el-icon>
                          <span class="camera-tree__leaf-label">{{ getSourceDisplayName(source) }}</span>
                        </span>
                        <span class="camera-tree__leaf-status" :class="`camera-tree__leaf-status--${source.status === 'online' ? 'success' : 'default'}`" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <SearchForm class="playback-page__search-form">
            <div class="app-field playback-page__range">
              <el-date-picker
                v-model="queryForm.range"
                type="datetimerange"
                start-placeholder="开始时间"
                end-placeholder="结束时间"
                range-separator="至"
                format="MM-DD HH:mm"
                value-format="YYYY-MM-DDTHH:mm:ss"
                style="width: 100%"
                @visible-change="handleRangePickerVisibleChange"
                @panel-change="handleRangePickerPanelChange"
              >
                <template v-if="playback.playbackMode === 'hik'" #default="cell">
                  <div
                    class="playback-page__date-cell"
                    :class="{
                      'playback-page__date-cell--recorded': isRecordedDateCell(cell),
                      'playback-page__date-cell--in-range': cell.inRange,
                      'playback-page__date-cell--selected': isSelectedDateCell(cell),
                    }"
                  >
                    <span class="playback-page__date-cell-text">{{ cell.renderText ?? cell.text }}</span>
                    <span v-if="isRecordedDateCell(cell)" class="playback-page__date-cell-dot" />
                  </div>
                </template>
              </el-date-picker>
              <button
                v-if="!queryForm.channelId"
                type="button"
                class="playback-page__range-blocker"
                aria-label="请先选择摄像机通道"
                @click.prevent="handleBlockedRangePickerClick"
              />
            </div>
            <div v-if="playback.playbackMode === 'hik' && queryForm.channelId && recordedDaysLoading" class="playback-page__range-hint">
              正在加载当月录像日期...
            </div>
            <template #actions>
              <button class="app-button app-button--primary playback-page__button" @click="() => void searchSegments()">
                <el-icon><Search /></el-icon>
                <span>{{ searching ? "查询中..." : "查询录像" }}</span>
              </button>
              <button class="app-button app-button--secondary playback-page__button" @click="resetQuery">重置</button>
            </template>
          </SearchForm>

          <div v-if="downloadState.active" class="playback-page__download-panel">
            <div class="playback-page__download-header">
              <strong>录像下载</strong>
              <span>{{ currentDownloadSegment?.recorderName || "-" }} / {{ currentDownloadSegment?.channelName || "-" }}</span>
            </div>
            <div class="playback-page__download-progress">
              <div
                class="playback-page__download-progress-bar"
                :class="{ 'playback-page__download-progress-bar--preparing': downloadState.preparing }"
                :style="{ width: `${downloadState.preparing ? 35 : downloadState.progress}%` }"
              />
            </div>
            <div class="playback-page__download-footer">
              <span>{{ downloadState.message || "下载已完成" }}</span>
              <small v-if="downloadState.estimatedRemainingText">{{ downloadState.estimatedRemainingText }}</small>
              <button
                v-if="downloadState.active && downloadState.controller"
                class="app-button app-button--secondary playback-page__button"
                @click="cancelDownload"
              >
                取消下载
              </button>
            </div>
          </div>

          <div class="playback-source-panel__segments">
            <div class="playback-source-panel__header">
              <span>录像片段</span>
              <span class="playback-source-panel__count">{{ segments.length }} 条</span>
            </div>
            <div class="playback-page__segments-table-wrapper">
              <table class="app-table playback-page__segments-table">
                <thead>
                  <tr>
                    <th>日期 (点击日期直接播放录像)</th>
                    <th>下载</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="!segments.length">
                    <td colspan="2" class="app-table__empty">{{ searching || loading ? "加载中..." : "暂无录像片段" }}</td>
                  </tr>
                  <tr
                    v-for="segment in segments"
                    :key="getSegmentKey(segment)"
                    :class="{ 'app-table__row--active': isActiveSegment(segment) }"
                    @click="selectSegment(segment)"
                    @dblclick.stop.prevent
                  >
                    <td
                      class="playback-page__segment-range"
                      @click.stop="() => void handleSegmentPrimaryAction(segment)"
                      @dblclick.stop.prevent
                    >
                      {{ formatSegmentRange(segment) }}
                    </td>
                    <td>
                      <button
                        class="app-button app-button--secondary playback-page__button playback-page__table-button"
                        :disabled="!segment.available || (downloadState.active && downloadState.segmentKey !== getSegmentKey(segment))"
                        @click.stop="() => void handleDownload(segment)"
                      >
                        <span>
                          {{
                            downloadState.active && downloadState.segmentKey === getSegmentKey(segment)
                              ? "下载中..."
                              : "下载"
                          }}
                        </span>
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </PageCard>
    </div>
  </div>
</template>

<style scoped>
.playback-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  height: calc(100vh - var(--layout-header-height) - (var(--layout-page-padding) * 2));
  min-height: 0;
  overflow: hidden;
}

.playback-page__layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 20px;
  flex: 1;
  min-height: 0;
}

.playback-page__main {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: 0;
  order: 1;
}

.playback-page__sources {
  order: 2;
  position: sticky;
  top: 0;
  align-self: start;
  min-height: 0;
  max-height: 100%;
}

.playback-page__sources :deep(.page-card) {
  height: 100%;
}

.playback-page__sources :deep(.page-card__body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  padding: 10px 8px 10px 10px;
}

.playback-page__main :deep(.page-card) {
  border-color: rgba(88, 139, 200, 0.18);
  background:
    radial-gradient(circle at top right, rgba(58, 134, 255, 0.14), transparent 28%),
    linear-gradient(180deg, rgba(11, 34, 63, 0.98) 0%, rgba(9, 24, 45, 0.98) 100%);
  box-shadow: 0 22px 46px rgba(8, 24, 46, 0.22);
}

.playback-page__main :deep(.page-card__header) {
  border-bottom-color: rgba(147, 185, 229, 0.14);
}

.playback-page__main :deep(.page-card__title) {
  color: #edf5ff;
}

.playback-page__main :deep(.page-card__description) {
  color: rgba(219, 232, 248, 0.72);
}

.playback-page__controls :deep(.page-card__body) {
  padding: 14px 18px;
}

.playback-page__player-wrapper {
  position: relative;
  flex: 1;
  min-height: 420px;
  border-radius: 18px;
  overflow: hidden;
  background:
    radial-gradient(circle at top, rgba(68, 147, 255, 0.14), transparent 30%),
    linear-gradient(180deg, #07172b 0%, #0b213f 100%);
  border: 1px solid rgba(87, 125, 163, 0.22);
  box-shadow: 0 20px 42px rgba(8, 24, 46, 0.2);
}

.playback-page__player-wrapper:fullscreen {
  display: flex;
  align-items: stretch;
  justify-content: stretch;
  width: 100%;
  height: 100%;
  padding: 16px;
  background: #061320;
  box-sizing: border-box;
}

.playback-page__player-wrapper:fullscreen :deep(.video-player) {
  width: 100%;
  height: 100%;
}

.playback-page__player-wrapper:fullscreen :deep(.hik-webcontrol-playback-player) {
  width: 100%;
  height: 100%;
}

.playback-page__player-wrapper:fullscreen :deep(.hik-webcontrol-playback-player__screen) {
  min-height: 0;
  padding: 0;
}

.playback-page__player-wrapper:fullscreen :deep(.hik-webcontrol-playback-player__surface) {
  inset: 0;
  border-radius: 0;
}

.playback-page__player-wrapper:fullscreen :deep(.video-player__screen) {
  min-height: 0;
  padding: 0;
}

.playback-page__player-wrapper:fullscreen :deep(.video-player__native) {
  min-height: 0;
  border-radius: 0;
}

.playback-page__player-wrapper:fullscreen :deep(.video-player__frame-hold) {
  inset: 0;
  width: 100%;
  height: 100%;
  border-radius: 0;
}

.playback-page__player-wrapper:fullscreen :deep(.video-player__seekbar--overlay) {
  display: flex;
  left: 32px;
  right: 32px;
  bottom: 28px;
}

.playback-page__player-wrapper:fullscreen :deep(.video-player__seekbar--dock) {
  display: none;
}

.playback-page__player-wrapper:fullscreen :deep(.hik-webcontrol-playback-player__seekbar--overlay) {
  display: flex;
  left: 32px;
  right: 32px;
  bottom: 28px;
}

.playback-page__player-wrapper:fullscreen :deep(.hik-webcontrol-playback-player__seekbar--dock) {
  display: none;
}

.playback-source-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.camera-tree {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-height: 260px;
  height: 260px;
  flex: 0 0 260px;
  overflow: hidden;
  border-bottom: 1px solid rgba(143, 166, 191, 0.16);
  padding-bottom: 10px;
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
  width: 100%;
  height: 36px;
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

.camera-tree__content {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 0;
  overflow-y: auto;
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

.camera-tree__empty {
  padding: 24px;
  text-align: center;
  color: #74889d;
  background: linear-gradient(180deg, #f8fbfe 0%, #f2f7fb 100%);
}

.camera-tree__branch {
  display: flex;
  flex-direction: column;
  gap: 0;
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

.camera-tree__node {
  display: flex;
  align-items: center;
  justify-content: flex-start;
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

.camera-tree__node-label {
  overflow: hidden;
  color: inherit;
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  text-overflow: ellipsis;
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

.camera-tree__leaf-status--default {
  background: #a7b5c4;
}

.playback-page__search-form :deep(.search-form) {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.playback-page__search-form :deep(.search-form__fields) {
  grid-template-columns: 1fr;
  gap: 10px;
}

.playback-page__search-form :deep(.search-form__actions) {
  display: flex;
  gap: 10px;
  align-items: center;
  justify-content: flex-start;
  flex-wrap: nowrap;
}

.playback-page__search-form :deep(.app-field select),
.playback-page__search-form :deep(.el-date-editor) {
  width: 100%;
  height: 36px;
  font-size: 13px;
}

.playback-page__search-form :deep(.el-date-editor--datetimerange) {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  min-height: 36px;
}

.playback-page__search-form :deep(.el-date-editor--datetimerange .el-range-input) {
  font-size: 12px;
  white-space: nowrap;
}

.playback-page__search-form :deep(.el-date-editor--datetimerange .el-range-separator) {
  min-width: 22px;
  padding: 0 4px;
  white-space: nowrap;
}

.playback-page__range {
  position: relative;
  min-width: 0;
}

.playback-page__range-hint {
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
}

.playback-page__range-blocker {
  position: absolute;
  inset: 0;
  z-index: 2;
  border: 0;
  padding: 0;
  background: transparent;
  cursor: pointer;
}

.playback-page__date-cell {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  border-radius: 8px;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.playback-page__date-cell-text {
  position: relative;
  z-index: 1;
}

.playback-page__date-cell--in-range {
  background: rgba(59, 130, 246, 0.16);
}

.playback-page__date-cell--selected {
  background: rgba(59, 130, 246, 0.3);
}

.playback-page__date-cell--selected .playback-page__date-cell-text {
  color: #1d4ed8;
  font-weight: 600;
}

.playback-page__date-cell--recorded .playback-page__date-cell-text {
  color: #e11d48;
  font-weight: 600;
}

.playback-page__date-cell--recorded.playback-page__date-cell--in-range {
  background: rgba(59, 130, 246, 0.2);
}

.playback-page__date-cell--recorded.playback-page__date-cell--selected {
  background: rgba(59, 130, 246, 0.34);
}

.playback-page__date-cell-dot {
  position: absolute;
  bottom: 4px;
  left: 50%;
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: #1d4ed8;
  transform: translateX(-50%);
}

.playback-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 12px;
  font-size: 13px;
  white-space: nowrap;
}

.playback-page__table-button {
  min-width: 58px;
  justify-content: center;
  padding: 0 8px;
  font-size: 12px;
  white-space: nowrap;
}

.playback-source-panel__segments {
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.playback-page__segments-table-wrapper {
  flex: 1;
  min-height: 0;
  max-height: min(46vh, 420px);
  overflow-y: auto;
  overflow-x: hidden;
  border-radius: 14px;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
}

.playback-source-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 13px;
  font-weight: 700;
  color: #1e3554;
}

.playback-source-panel__count {
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(52, 123, 255, 0.08);
  color: #2877ff;
  font-size: 12px;
  font-weight: 700;
}

.playback-page__segments-table {
  table-layout: fixed;
  min-width: 0;
}

.playback-page__segments-table th,
.playback-page__segments-table td {
  font-size: 12px;
  line-height: 1.2;
  white-space: nowrap;
}

.playback-page__segments-table th {
  padding: 8px 10px;
}

.playback-page__segments-table td {
  padding: 8px 10px;
}

.playback-page__segments-table tbody tr.app-table__row--active td {
  background: linear-gradient(135deg, rgba(31, 124, 255, 0.14), rgba(64, 205, 255, 0.08));
  box-shadow: inset 0 0 0 1px rgba(31, 124, 255, 0.12);
}

.playback-page__segments-table tbody tr.app-table__row--active {
  box-shadow: 0 10px 20px rgba(14, 79, 191, 0.16);
}

.playback-page__segments-table th:last-child,
.playback-page__segments-table td:last-child {
  width: 72px;
  text-align: center;
}

.playback-page__segment-range {
  color: #183b66;
  font-weight: 600;
  cursor: pointer;
}

.playback-page__download-panel {
  padding: 14px;
  border: 1px solid rgba(36, 125, 255, 0.16);
  border-radius: 16px;
  background: linear-gradient(180deg, #fbfdff 0%, #f3f8fd 100%);
  box-shadow: 0 12px 26px rgba(17, 43, 74, 0.05);
}

.playback-page__download-header,
.playback-page__download-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.playback-page__download-header strong {
  color: #0e2b4b;
  font-size: 14px;
}

.playback-page__download-header span,
.playback-page__download-footer span {
  color: #60778f;
  font-size: 13px;
}

.playback-page__download-footer small {
  margin-left: auto;
  color: #0e7490;
  font-size: 12px;
  white-space: nowrap;
}

.playback-page__download-progress {
  position: relative;
  overflow: hidden;
  height: 12px;
  margin: 12px 0;
  border-radius: 999px;
  background: #e8f0f8;
}

.playback-page__download-progress-bar {
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #409eff, #67c23a);
  transition: width 0.25s ease;
}

.playback-page__download-progress-bar--preparing {
  position: relative;
  animation: playback-download-loading 1.2s ease-in-out infinite alternate;
}

@keyframes playback-download-loading {
  from {
    transform: translateX(-8%);
  }
  to {
    transform: translateX(8%);
  }
}

.playback-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: nowrap;
  overflow-x: auto;
}

.playback-toolbar__group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}

.playback-toolbar__group--meta {
  min-width: 0;
}

.playback-toolbar__meta {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  max-width: 100%;
  padding: 0 12px;
  border-radius: 12px;
  border: 1px solid rgba(147, 185, 229, 0.14);
  background: rgba(255, 255, 255, 0.08);
  color: rgba(236, 245, 255, 0.84);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.playback-toolbar__speed {
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  padding: 0 12px;
  border-radius: 12px;
  border: 1px solid rgba(147, 185, 229, 0.14);
  background: rgba(255, 255, 255, 0.08);
  color: #edf5ff;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.playback-toolbar__group--mode {
  margin-left: 0;
}

.playback-toolbar__mode-select {
  width: 128px;
  height: 36px;
  min-height: 36px;
  border-radius: 12px;
  border: 1px solid rgba(132, 154, 180, 0.2);
  background-color: rgba(255, 255, 255, 0.92);
  background-image:
    linear-gradient(45deg, transparent 50%, #5d718d 50%),
    linear-gradient(135deg, #5d718d 50%, transparent 50%);
  background-position:
    calc(100% - 18px) calc(50% - 1px),
    calc(100% - 12px) calc(50% - 1px);
  background-repeat: no-repeat;
  background-size: 6px 6px, 6px 6px;
  box-shadow: 0 6px 18px rgba(18, 45, 77, 0.06);
  color: #4f6480;
  font-size: 13px;
  font-weight: 600;
  line-height: 36px;
  padding: 0 34px 0 12px;
  cursor: pointer;
  appearance: none;
  box-sizing: border-box;
  transition: transform 0.18s ease, box-shadow 0.18s ease, filter 0.18s ease, background-color 0.18s ease;
}

.playback-toolbar__mode-select:hover {
  filter: brightness(0.99);
  transform: translateY(-1px);
}

.playback-toolbar__mode-select:focus {
  outline: none;
  border-color: rgba(98, 160, 232, 0.35);
  box-shadow: 0 0 0 3px rgba(73, 146, 255, 0.12), 0 6px 18px rgba(18, 45, 77, 0.06);
}

.playback-toolbar__mode-select:disabled {
  cursor: not-allowed;
  opacity: 0.72;
}

.playback-toolbar__actions {
  display: flex;
  flex-wrap: nowrap;
  gap: 8px;
  align-items: center;
  margin-left: auto;
}

@media (max-width: 1280px) {
  .playback-page__layout {
    grid-template-columns: 1fr;
  }

  .playback-page__sources {
    position: static;
    max-height: none;
  }

  .playback-toolbar__group--mode {
    margin-left: 0;
  }

  .playback-toolbar {
    flex-wrap: wrap;
    overflow-x: visible;
  }

  .playback-toolbar__actions {
    flex-wrap: wrap;
    margin-left: 0;
  }

  .playback-page__segments-table-wrapper {
    max-height: 360px;
  }
}

@media (max-width: 768px) {
  .playback-page {
    height: auto;
    overflow: visible;
  }

  .playback-page__layout {
    flex: none;
  }

  .playback-page__search-form :deep(.search-form__actions) {
    flex-wrap: wrap;
  }

  .playback-page__segments-table-wrapper {
    max-height: 320px;
  }
}
</style>
