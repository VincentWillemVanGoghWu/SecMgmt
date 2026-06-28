<script setup lang="ts">
import type Hls from "hls.js"
import { computed, defineAsyncComponent, nextTick, onBeforeUnmount, ref, useTemplateRef, watch } from "vue"

import type {
  ConnectionMode,
  LiveWebControlConfig,
  PlaybackTimelineSpan,
  PreviewBrowseMode,
  StreamProfile,
  StreamType,
} from "../../types/video"

type HlsConstructor = typeof import("hls.js")["default"]

const HikWebControlPlayer = defineAsyncComponent(() => import("./HikWebControlPlayer.vue"))

const emit = defineEmits<{
  seek: [offsetSeconds: number]
  seekEnd: [offsetSeconds: number]
}>()

const props = withDefaults(
  defineProps<{
    title?: string
    playUrl?: string | null
    streamType?: StreamType
    streamProfile?: StreamProfile
    isPlaying?: boolean
    cameraName?: string
    cameraLocation?: string
    snapshotUrl?: string | null
    message?: string
    diagnosticMessage?: string | null
    sourceRtsp?: string | null
    isMock?: boolean
    playableInBrowser?: boolean
    connectionMode?: ConnectionMode
    active?: boolean
    visibleSlotCount?: number
    showSeekBar?: boolean
    playbackStartTime?: string
    playbackEndTime?: string
    recordedSpans?: PlaybackTimelineSpan[]
    seekBaseSeconds?: number
    seekTargetSeconds?: number | null
    isPaused?: boolean
    playbackRate?: number
    suppressErrors?: boolean
    protocolName?: string
    browseMode?: PreviewBrowseMode
    webControlConfig?: LiveWebControlConfig | null
  }>(),
  {
    title: "实时预览",
    playUrl: null,
    streamType: "http-flv",
    streamProfile: "main",
    isPlaying: false,
    cameraName: "",
    cameraLocation: "",
    snapshotUrl: null,
    message: "",
    diagnosticMessage: null,
    sourceRtsp: null,
    isMock: false,
    playableInBrowser: true,
    connectionMode: "standard",
    active: false,
    visibleSlotCount: 1,
    showSeekBar: false,
    playbackStartTime: "",
    playbackEndTime: "",
    recordedSpans: () => [],
    seekBaseSeconds: 0,
    seekTargetSeconds: null,
    isPaused: false,
    playbackRate: 1,
    suppressErrors: false,
    protocolName: "",
    browseMode: "browser",
    webControlConfig: null,
  },
)

const renderMode = computed(() => ((props.isMock || props.playUrl?.includes("mock=1")) ? "mock" : props.streamType))
const isWebControlMode = computed(() => props.browseMode === "webcontrol")
const playerError = ref("")
const playerState = ref<"idle" | "loading" | "ready" | "playing">("idle")
const playerRootRef = useTemplateRef<HTMLElement>("playerRootRef")
const videoRef = useTemplateRef<HTMLVideoElement>("videoRef")
const playbackCurrentTime = ref(0)
const playbackDuration = ref(0)
const isSeeking = ref(false)
const lastFrameUrl = ref<string | null>(null)
const frameHoldActive = ref(false)
const timelineViewStart = ref(0)
const timelineViewDuration = ref(0)
const timelinePreviewVisible = ref(false)
const timelinePreviewSeconds = ref(0)
const timelinePreviewLeft = ref("0%")
const isTimelinePanning = ref(false)
let hls: Hls | null = null
let hlsConstructorPromise: Promise<HlsConstructor> | null = null
let errorDialogVisible = false
let lastDialogMessage = ""
let suppressMediaErrors = false
let sourceChangeTimestamp = 0
let attachPlayerSourceToken = 0
let appliedInitialSeekSignature = ""
let hlsNetworkRecoveryUntil = 0
let playAttemptId = 0
let playRetryTimer: number | null = null
let playRetryCount = 0
let timelinePanOriginX = 0
let timelinePanOriginStart = 0
let timelinePanWidth = 0
let timelinePanLeft = 0
const MIN_TIMELINE_WINDOW_SECONDS = 300

const clampNumber = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

const resolveSpanOffset = (offsetSeconds: number, spans: PlaybackTimelineSpan[], axisStart: Date | null, maxOffset: number) => {
  const normalizedOffset = clampNumber(offsetSeconds, 0, maxOffset)
  if (!axisStart || !spans.length) {
    return normalizedOffset
  }
  const axisStartMs = axisStart.getTime()
  const targetMs = axisStartMs + normalizedOffset * 1000
  let snappedOffset = normalizedOffset
  let bestDistance = Number.POSITIVE_INFINITY

  for (const span of spans) {
    const spanStartMs = new Date(span.startTime).getTime()
    const spanEndMs = new Date(span.endTime).getTime()
    if (Number.isNaN(spanStartMs) || Number.isNaN(spanEndMs) || spanEndMs <= spanStartMs) {
      continue
    }
    const insideMs = Math.min(Math.max(targetMs, spanStartMs), Math.max(spanStartMs, spanEndMs - 1000))
    const distance = targetMs < spanStartMs
      ? spanStartMs - targetMs
      : targetMs >= spanEndMs
        ? targetMs - Math.max(spanStartMs, spanEndMs - 1000)
        : 0
    if (distance < bestDistance) {
      bestDistance = distance
      snappedOffset = Math.max(0, Math.round((insideMs - axisStartMs) / 1000))
    }
    if (distance === 0) {
      break
    }
  }

  return clampNumber(snappedOffset, 0, maxOffset)
}

const destroyHls = () => {
  if (hls) {
    hls.destroy()
    hls = null
  }
}

const loadHlsConstructor = async () => {
  hlsConstructorPromise ??= import("hls.js/light").then((module) => module.default as HlsConstructor)
  return await hlsConstructorPromise
}

const suppressTeardownErrorsBriefly = () => {
  suppressMediaErrors = true
  window.setTimeout(() => {
    suppressMediaErrors = false
  }, 2000)
}

const markSourceChange = () => {
  sourceChangeTimestamp = Date.now()
  playRetryCount = 0
}

const clearPlaybackRetry = () => {
  if (playRetryTimer === null) {
    return
  }
  window.clearTimeout(playRetryTimer)
  playRetryTimer = null
}

const schedulePlaybackRetry = (requestedUrl: string | null | undefined) => {
  if (!props.showSeekBar || !requestedUrl) {
    return
  }
  if (playRetryCount >= 5) {
    return
  }
  playRetryCount += 1
  clearPlaybackRetry()
  playRetryTimer = window.setTimeout(() => {
    playRetryTimer = null
    if (requestedUrl !== props.playUrl || !props.isPlaying || props.isPaused) {
      return
    }
    applyPlaybackState()
  }, 600)
}

const shouldIgnoreAutoplayError = (error: unknown, requestedUrl: string | null | undefined) => {
  if (!requestedUrl || requestedUrl !== props.playUrl || !props.isPlaying) {
    return true
  }
  if (props.suppressErrors || (props.showSeekBar && props.streamType === "hik-sdk")) {
    return true
  }
  if (Date.now() - sourceChangeTimestamp < 1200) {
    const name = error instanceof DOMException ? error.name : ""
    const message = error instanceof Error ? error.message.toLowerCase() : String(error ?? "").toLowerCase()
    if (
      name === "AbortError"
      || name === "NotAllowedError"
      || message.includes("interrupted")
      || message.includes("abort")
      || message.includes("new load request")
      || message.includes("play() request was interrupted")
    ) {
      return true
    }
  }
  return false
}

const detachPlayerSource = () => {
  attachPlayerSourceToken += 1
  suppressTeardownErrorsBriefly()
  markSourceChange()
  destroyHls()
  playerError.value = ""
  playerState.value = "idle"
  resetPlaybackProgress()
  const element = videoRef.value
  if (!element) {
    return
  }
  element.pause()
  element.removeAttribute("src")
  element.load()
}

const showPlayerErrorDialog = (message: string) => {
  if (!message || (errorDialogVisible && lastDialogMessage === message)) {
    return
  }
  errorDialogVisible = true
  lastDialogMessage = message
  void import("element-plus")
    .then(({ ElMessageBox }) =>
      ElMessageBox.alert(message, "预览失败", {
        type: "error",
        confirmButtonText: "确定",
      }),
    )
    .finally(() => {
      errorDialogVisible = false
    })
}

const applyPlaybackState = () => {
  const element = videoRef.value
  if (!element) {
    return
  }
  clearPlaybackRetry()
  element.playbackRate = props.playbackRate
  if (!props.isPlaying) {
    playAttemptId += 1
    element.pause()
    return
  }
  if (props.isPaused) {
    playAttemptId += 1
    element.pause()
    return
  }
  const requestedUrl = props.playUrl
  const attemptId = ++playAttemptId
  void element.play().catch((error) => {
    if (attemptId !== playAttemptId) {
      return
    }
    if (shouldIgnoreAutoplayError(error, requestedUrl)) {
      schedulePlaybackRetry(requestedUrl)
      return
    }
    playerState.value = "idle"
    showPlayerErrorDialog("\u81ea\u52a8\u64ad\u653e\u5931\u8d25\uff0c\u8bf7\u91cd\u8bd5\u9884\u89c8\u3002")
  })
}

const resetPlaybackProgress = () => {
  playbackCurrentTime.value = Math.max(0, props.seekBaseSeconds || 0)
  playbackDuration.value = 0
  isSeeking.value = false
  timelineViewStart.value = 0
  timelineViewDuration.value = 0
  timelinePreviewVisible.value = false
  isTimelinePanning.value = false
}

const storeCurrentFrame = () => {
  const element = videoRef.value
  if (!props.showSeekBar || !element || !element.videoWidth || !element.videoHeight) {
    return
  }
  try {
    const canvas = document.createElement("canvas")
    canvas.width = element.videoWidth
    canvas.height = element.videoHeight
    const context = canvas.getContext("2d")
    if (!context) {
      return
    }
    context.drawImage(element, 0, 0, canvas.width, canvas.height)
    lastFrameUrl.value = canvas.toDataURL("image/png")
    frameHoldActive.value = true
  } catch {
    frameHoldActive.value = false
  }
}

const releaseFrameHold = () => {
  frameHoldActive.value = false
  window.setTimeout(() => {
    if (!frameHoldActive.value) {
      lastFrameUrl.value = null
    }
  }, 300)
}

const selectedPlaybackStart = computed(() => {
  const value = new Date(props.playbackStartTime || "")
  return Number.isNaN(value.getTime()) ? null : value
})

const selectedPlaybackEnd = computed(() => {
  const value = new Date(props.playbackEndTime || "")
  return Number.isNaN(value.getTime()) ? null : value
})

const selectedPlaybackDurationSeconds = computed(() => {
  if (!selectedPlaybackStart.value || !selectedPlaybackEnd.value) {
    return 0
  }
  return Math.max(0, Math.round((selectedPlaybackEnd.value.getTime() - selectedPlaybackStart.value.getTime()) / 1000))
})

const recordedSpanStyles = computed(() => {
  if (!selectedPlaybackStart.value || !selectedPlaybackEnd.value) {
    return []
  }
  const axisStartMs = selectedPlaybackStart.value.getTime()
  const windowStartSeconds = visibleTimelineStart.value
  const windowDurationSeconds = visibleTimelineDuration.value
  const windowStartMs = axisStartMs + windowStartSeconds * 1000
  const windowEndMs = windowStartMs + windowDurationSeconds * 1000
  if (windowDurationSeconds <= 0 || windowEndMs <= windowStartMs) {
    return []
  }
  return props.recordedSpans
    .map((span) => {
      const startMs = new Date(span.startTime).getTime()
      const endMs = new Date(span.endTime).getTime()
      if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) {
        return null
      }
      const clippedStartMs = Math.max(windowStartMs, startMs)
      const clippedEndMs = Math.min(windowEndMs, endMs)
      if (clippedEndMs <= clippedStartMs) {
        return null
      }
      return {
        left: `${((clippedStartMs - windowStartMs) / (windowDurationSeconds * 1000)) * 100}%`,
        width: `${Math.max(0.35, ((clippedEndMs - clippedStartMs) / (windowDurationSeconds * 1000)) * 100)}%`,
        active:
          playbackCurrentTime.value >= Math.round((clippedStartMs - axisStartMs) / 1000)
          && playbackCurrentTime.value <= Math.round((clippedEndMs - axisStartMs) / 1000),
      }
    })
    .filter((item): item is { left: string; width: string; active: boolean } => Boolean(item))
})

const seekBarMax = computed(() => {
  if (selectedPlaybackDurationSeconds.value > 0) {
    return selectedPlaybackDurationSeconds.value
  }
  return Math.max(playbackDuration.value, 0)
})

const visibleTimelineDuration = computed(() => {
  const max = seekBarMax.value
  if (max <= 0) {
    return 0
  }
  const fallback = timelineViewDuration.value > 0 ? timelineViewDuration.value : max
  return clampNumber(fallback, Math.min(MIN_TIMELINE_WINDOW_SECONDS, max), max)
})

const visibleTimelineStart = computed(() => {
  const max = seekBarMax.value
  const duration = visibleTimelineDuration.value
  if (max <= 0 || duration >= max) {
    return 0
  }
  return clampNumber(timelineViewStart.value, 0, max - duration)
})

const visibleTimelineEnd = computed(() => {
  const max = seekBarMax.value
  if (max <= 0) {
    return 0
  }
  return Math.min(max, visibleTimelineStart.value + visibleTimelineDuration.value)
})

const timelineIsZoomed = computed(() =>
  seekBarMax.value > 0 && visibleTimelineDuration.value < seekBarMax.value - 1,
)

const isPlaybackEndOffset = (offsetSeconds: number) =>
  props.showSeekBar && seekBarMax.value > 0 && offsetSeconds >= seekBarMax.value

const shouldSuppressPlaybackStreamError = (errorType?: string, errorDetail?: string) => {
  if (!props.showSeekBar) {
    return false
  }
  if (props.suppressErrors) {
    return true
  }
  if (Date.now() - sourceChangeTimestamp < 5000) {
    return true
  }
  if (!isPlaybackEndOffset(playbackCurrentTime.value)) {
    return false
  }
  return (
    errorType === "networkError"
    || errorDetail === "levelLoadError"
    || errorDetail === "fragLoadError"
  )
}

const syncPlaybackProgress = () => {
  const element = videoRef.value
  if (!element) {
    return
  }
  const duration = Number.isFinite(element.duration) && element.duration > 0 ? element.duration : 0
  playbackDuration.value = duration
  if (!isSeeking.value) {
    const nextTime = Math.max(0, props.seekBaseSeconds + (element.currentTime || 0))
    playbackCurrentTime.value = seekBarMax.value > 0 ? Math.min(nextTime, seekBarMax.value) : nextTime
  }
}

const applyInitialMediaSeek = () => {
  const element = videoRef.value
  if (!element || !props.showSeekBar || props.seekTargetSeconds == null) {
    return
  }
  const baseOffset = Math.max(0, props.seekBaseSeconds || 0)
  const targetOffset = Math.max(0, Math.min(props.seekTargetSeconds, seekBarMax.value || props.seekTargetSeconds))
  const mediaOffset = targetOffset - baseOffset
  if (mediaOffset <= 0.2) {
    return
  }
  const signature = `${props.playUrl || ""}|${baseOffset}|${targetOffset}`
  if (appliedInitialSeekSignature === signature) {
    return
  }
  const duration = Number.isFinite(element.duration) && element.duration > 0 ? element.duration : 0
  const safeMediaOffset = duration > 0 ? Math.min(mediaOffset, Math.max(0, duration - 0.25)) : mediaOffset
  try {
    element.currentTime = safeMediaOffset
    playbackCurrentTime.value = targetOffset
    appliedInitialSeekSignature = signature
  } catch {
    // HLS may reject early seeks before enough metadata is available; later media events retry this once.
  }
}

const formatPlaybackSeconds = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds < 0) {
    return "00:00"
  }
  const totalSeconds = Math.floor(seconds)
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const remainSeconds = totalSeconds % 60
  if (hours > 0) {
    return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(remainSeconds).padStart(2, "0")}`
  }
  return `${String(minutes).padStart(2, "0")}:${String(remainSeconds).padStart(2, "0")}`
}

const formatTickTime = (offsetSeconds: number) => {
  if (!selectedPlaybackStart.value) {
    return formatPlaybackSeconds(offsetSeconds)
  }
  const date = new Date(selectedPlaybackStart.value.getTime() + Math.max(0, offsetSeconds) * 1000)
  return date.toLocaleTimeString("zh-CN", {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  })
}

const formatPreviewTime = (offsetSeconds: number) => {
  if (!selectedPlaybackStart.value) {
    return formatPlaybackSeconds(offsetSeconds)
  }
  const date = new Date(selectedPlaybackStart.value.getTime() + Math.max(0, offsetSeconds) * 1000)
  return date.toLocaleString("zh-CN", { hour12: false })
}

const timelinePreviewLabel = computed(() => formatPreviewTime(timelinePreviewSeconds.value))

const revealTimelineOffset = (offsetSeconds: number) => {
  if (!timelineIsZoomed.value) {
    return
  }
  const duration = visibleTimelineDuration.value
  const max = seekBarMax.value
  if (duration <= 0 || max <= 0) {
    return
  }
  if (offsetSeconds < visibleTimelineStart.value) {
    timelineViewStart.value = clampNumber(offsetSeconds, 0, max - duration)
    return
  }
  if (offsetSeconds > visibleTimelineEnd.value) {
    timelineViewStart.value = clampNumber(offsetSeconds - duration * 0.85, 0, max - duration)
  }
}

const syncTimelineWindow = () => {
  const max = seekBarMax.value
  if (max <= 0) {
    timelineViewStart.value = 0
    timelineViewDuration.value = 0
    return
  }
  if (timelineViewDuration.value <= 0 || timelineViewDuration.value > max) {
    timelineViewStart.value = 0
    timelineViewDuration.value = max
    return
  }
  const minDuration = Math.min(MIN_TIMELINE_WINDOW_SECONDS, max)
  timelineViewDuration.value = clampNumber(timelineViewDuration.value, minDuration, max)
  timelineViewStart.value = clampNumber(timelineViewStart.value, 0, Math.max(0, max - timelineViewDuration.value))
}

const seekTicks = computed(() => {
  const duration = visibleTimelineDuration.value
  const start = visibleTimelineStart.value
  if (!Number.isFinite(duration) || duration <= 0) {
    return []
  }
  const ratios = [0, 0.25, 0.5, 0.75, 1]
  return ratios.map((ratio) => {
    const value = Math.round(start + duration * ratio)
    return {
      value,
      label: formatTickTime(value),
      position: `${ratio * 100}%`,
    }
  })
})

const shouldShowPlaybackSeekBar = computed(() =>
  props.showSeekBar && seekBarMax.value > 0 && renderMode.value !== "mock",
)

const attachPlayerSource = async () => {
  const token = ++attachPlayerSourceToken
  const element = videoRef.value
  if (isWebControlMode.value || !element || !props.playUrl || !props.isPlaying || renderMode.value === "mock") {
    detachPlayerSource()
    return
  }

  markSourceChange()
  storeCurrentFrame()
  suppressTeardownErrorsBriefly()
  const hadExistingSource = Boolean(hls || element.currentSrc || element.src)
  destroyHls()
  playerState.value = "loading"
  appliedInitialSeekSignature = ""
  hlsNetworkRecoveryUntil = Date.now() + (props.streamType === "hik-sdk" ? 15000 : 0)
  resetPlaybackProgress()
  element.pause()
  if (!hadExistingSource) {
    element.removeAttribute("src")
    element.load()
  }
  element.muted = true
  element.autoplay = true
  element.crossOrigin = "anonymous"
  element.controls = !props.showSeekBar
  element.playbackRate = props.playbackRate

  const isHlsSource = props.playUrl.includes(".m3u8") || props.streamType === "hls"

  if (isHlsSource) {
    if (element.canPlayType("application/vnd.apple.mpegurl")) {
      element.src = props.playUrl
      applyPlaybackState()
      return
    }

    let HlsConstructor: HlsConstructor
    try {
      HlsConstructor = await loadHlsConstructor()
    } catch {
      playerError.value = "HLS 播放器加载失败，请刷新页面后重试。"
      playerState.value = "idle"
      showPlayerErrorDialog(playerError.value)
      return
    }
    if (token !== attachPlayerSourceToken || element !== videoRef.value || !props.playUrl || !props.isPlaying) {
      return
    }

    if (HlsConstructor.isSupported()) {
      const hlsInstance = new HlsConstructor({
        manifestLoadingMaxRetry: 8,
        manifestLoadingRetryDelay: 500,
        levelLoadingMaxRetry: 8,
        levelLoadingRetryDelay: 500,
        fragLoadingMaxRetry: 8,
        fragLoadingRetryDelay: 500,
      })
      hls = hlsInstance
      hlsInstance.loadSource(props.playUrl)
      hlsInstance.attachMedia(element)
      hlsInstance.on(HlsConstructor.Events.MANIFEST_PARSED, () => {
        if (hlsInstance !== hls) {
          return
        }
        playerState.value = "ready"
        applyPlaybackState()
      })
      hlsInstance.on(HlsConstructor.Events.ERROR, (_, data) => {
        if (hlsInstance !== hls) {
          return
        }
        if (suppressMediaErrors || !props.playUrl || !props.isPlaying) {
          return
        }
        const errorType = typeof data.type === "string" ? data.type : "unknown"
        const errorDetail = typeof data.details === "string" ? data.details : "unknown"
        if (
          data.fatal
          && props.streamType === "hik-sdk"
          && errorType === "networkError"
          && Date.now() < hlsNetworkRecoveryUntil
        ) {
          hlsInstance.startLoad()
          playerState.value = "loading"
          return
        }
        if (shouldSuppressPlaybackStreamError(errorType, errorDetail)) {
          if (props.streamType === "hik-sdk" && errorType === "networkError") {
            hlsInstance.startLoad()
          }
          if (props.streamType === "hik-sdk" && errorType === "mediaError") {
            hlsInstance.recoverMediaError()
          }
          if (isPlaybackEndOffset(playbackCurrentTime.value)) {
            playbackCurrentTime.value = 0
          }
          playerState.value = "ready"
          return
        }
        if (data.fatal) {
          const protocolName = props.protocolName || (props.streamType === "hik-sdk" ? "SDK" : "HLS")
          playerError.value = `${protocolName} 播放失败：${errorType} / ${errorDetail}。请检查 m3u8、ts 分片和视频编码兼容性。`
          playerState.value = "idle"
          destroyHls()
          showPlayerErrorDialog(playerError.value)
        }
      })
      return
    }

    playerError.value = "当前浏览器既不支持原生 HLS，也无法启用 hls.js。"
    playerState.value = "idle"
    showPlayerErrorDialog(playerError.value)
    return
  }

  element.src = props.playUrl
  applyPlaybackState()
}

const scheduleAttachPlayerSource = () => {
  void nextTick(() => {
    playerError.value = ""
    playerState.value = "idle"
    resetPlaybackProgress()
    void attachPlayerSource().catch(() => {
      playerError.value = "播放器初始化失败，请刷新页面后重试。"
      playerState.value = "idle"
      showPlayerErrorDialog(playerError.value)
    })
  })
}

watch(
  () => [props.playUrl, props.streamType, props.isPlaying, renderMode.value, props.browseMode] as const,
  (current, previous) => {
    if (previous?.[0] && current[0] && previous[0] !== current[0]) {
      storeCurrentFrame()
    }
    scheduleAttachPlayerSource()
  },
  { immediate: true },
)

watch(
  () => videoRef.value,
  (element) => {
    if (element) {
      scheduleAttachPlayerSource()
    }
  },
)

watch(
  () => [props.isPlaying, props.isPaused, props.playbackRate] as const,
  () => {
    void nextTick(() => {
      applyPlaybackState()
    })
  },
)

watch(seekBarMax, () => {
  syncTimelineWindow()
})

watch(playbackCurrentTime, (value) => {
  if (!isSeeking.value && !isTimelinePanning.value) {
    revealTimelineOffset(value)
  }
})

const handlePlayerError = () => {
  if (suppressMediaErrors || !props.playUrl || !props.isPlaying) {
    return
  }
  if (shouldSuppressPlaybackStreamError()) {
    if (isPlaybackEndOffset(playbackCurrentTime.value)) {
      playbackCurrentTime.value = 0
    }
    return
  }
  playerError.value = "浏览器无法直接播放当前地址，请确认媒体服务已输出浏览器可播放的 HLS/MP4 流。"
  playerState.value = "idle"
  showPlayerErrorDialog(playerError.value)
}

const handleLoadedMetadata = () => {
  syncPlaybackProgress()
  applyInitialMediaSeek()
}

const handleFrameReady = () => {
  syncPlaybackProgress()
  applyInitialMediaSeek()
  releaseFrameHold()
}

const handleDurationChange = () => {
  syncPlaybackProgress()
}

const handleTimeUpdate = () => {
  syncPlaybackProgress()
  applyInitialMediaSeek()
  if (frameHoldActive.value && videoRef.value && videoRef.value.readyState >= 2) {
    releaseFrameHold()
  }
}

const handlePlaying = () => {
  playerState.value = "playing"
  playRetryCount = 0
  clearPlaybackRetry()
  releaseFrameHold()
}

const updateTimelinePreview = (offsetSeconds: number, ratio: number) => {
  timelinePreviewSeconds.value = clampNumber(offsetSeconds, 0, seekBarMax.value || offsetSeconds)
  timelinePreviewLeft.value = `${clampNumber(ratio, 0, 1) * 100}%`
  timelinePreviewVisible.value = true
}

const handleSeekInput = (event: Event) => {
  const target = event.target as HTMLInputElement
  const nextTime = Number(target.value || 0)
  const rawOffset = seekBarMax.value > 0 ? Math.min(Math.max(0, nextTime), seekBarMax.value) : Math.max(0, nextTime)
  const snappedOffset = resolveSpanOffset(rawOffset, props.recordedSpans, selectedPlaybackStart.value, seekBarMax.value || rawOffset)
  playbackCurrentTime.value = snappedOffset
  isSeeking.value = true
  const min = Number(target.min || 0)
  const max = Number(target.max || seekBarMax.value || 0)
  const ratio = max > min ? (snappedOffset - min) / (max - min) : 0
  updateTimelinePreview(snappedOffset, ratio)
}

const handleSeekCommit = () => {
  const maxOffset = seekBarMax.value || playbackCurrentTime.value
  const targetOffset = Math.max(0, Math.min(playbackCurrentTime.value, maxOffset))
  if (isPlaybackEndOffset(targetOffset)) {
    storeCurrentFrame()
    playbackCurrentTime.value = 0
    emit("seekEnd", maxOffset)
    isSeeking.value = false
    timelinePreviewVisible.value = false
    return
  }
  emit("seek", targetOffset)
  isSeeking.value = false
  timelinePreviewVisible.value = false
}

const getTimelinePointerState = (event: MouseEvent, wrap: HTMLElement) => {
  const rect = wrap.getBoundingClientRect()
  const ratio = rect.width > 0 ? clampNumber((event.clientX - rect.left) / rect.width, 0, 1) : 0
  const rawSeconds = visibleTimelineStart.value + visibleTimelineDuration.value * ratio
  const snappedSeconds = resolveSpanOffset(rawSeconds, props.recordedSpans, selectedPlaybackStart.value, seekBarMax.value || rawSeconds)
  const snappedRatio = visibleTimelineDuration.value > 0
    ? clampNumber((snappedSeconds - visibleTimelineStart.value) / visibleTimelineDuration.value, 0, 1)
    : ratio
  return { ratio: snappedRatio, seconds: snappedSeconds, rect }
}

const handleTimelineHover = (event: MouseEvent) => {
  const wrap = event.currentTarget as HTMLElement | null
  if (!wrap || isTimelinePanning.value) {
    return
  }
  const { ratio, seconds } = getTimelinePointerState(event, wrap)
  updateTimelinePreview(seconds, ratio)
}

const handleTimelineLeave = () => {
  if (isSeeking.value || isTimelinePanning.value) {
    return
  }
  timelinePreviewVisible.value = false
}

const handleTimelineWheel = (event: WheelEvent) => {
  if (seekBarMax.value <= 0) {
    return
  }
  const wrap = event.currentTarget as HTMLElement | null
  if (!wrap) {
    return
  }
  event.preventDefault()
  const { ratio, seconds } = getTimelinePointerState(event as unknown as MouseEvent, wrap)
  const currentDuration = visibleTimelineDuration.value || seekBarMax.value
  const minDuration = Math.min(MIN_TIMELINE_WINDOW_SECONDS, seekBarMax.value)
  const zoomFactor = event.deltaY < 0 ? 0.75 : 1.25
  const nextDuration = clampNumber(currentDuration * zoomFactor, minDuration, seekBarMax.value)
  const nextStart = clampNumber(seconds - ratio * nextDuration, 0, Math.max(0, seekBarMax.value - nextDuration))
  timelineViewDuration.value = nextDuration
  timelineViewStart.value = nextStart
  updateTimelinePreview(seconds, ratio)
}

const resolveTimelineWrap = (target: EventTarget | null) => {
  if (!(target instanceof HTMLElement)) {
    return null
  }
  if (target.classList.contains("video-player__range-wrap")) {
    return target
  }
  const wrap = target.closest(".video-player__range-wrap")
  return wrap instanceof HTMLElement ? wrap : null
}

const stopTimelinePan = () => {
  if (!isTimelinePanning.value) {
    return
  }
  isTimelinePanning.value = false
  window.removeEventListener("mousemove", handleTimelinePanMove)
  window.removeEventListener("mouseup", stopTimelinePan)
  if (!isSeeking.value) {
    timelinePreviewVisible.value = false
  }
}

function handleTimelinePanMove(event: MouseEvent) {
  if (!isTimelinePanning.value || timelinePanWidth <= 0 || !timelineIsZoomed.value) {
    return
  }
  const maxStart = Math.max(0, seekBarMax.value - visibleTimelineDuration.value)
  const deltaSeconds = ((event.clientX - timelinePanOriginX) / timelinePanWidth) * visibleTimelineDuration.value
  timelineViewStart.value = clampNumber(timelinePanOriginStart - deltaSeconds, 0, maxStart)
  const ratio = clampNumber((event.clientX - timelinePanLeft) / Math.max(timelinePanWidth, 1), 0, 1)
  const seconds = visibleTimelineStart.value + visibleTimelineDuration.value * ratio
  updateTimelinePreview(seconds, ratio)
}

const handleTimelinePanStart = (event: MouseEvent) => {
  if (event.button !== 1 || !timelineIsZoomed.value) {
    return
  }
  const wrap = resolveTimelineWrap(event.currentTarget)
  if (!wrap) {
    return
  }
  timelinePanOriginX = event.clientX
  timelinePanOriginStart = visibleTimelineStart.value
  const rect = wrap.getBoundingClientRect()
  timelinePanWidth = rect.width
  timelinePanLeft = rect.left
  if (timelinePanWidth <= 0) {
    return
  }
  event.preventDefault()
  isTimelinePanning.value = true
  const { ratio, seconds } = getTimelinePointerState(event, wrap)
  updateTimelinePreview(seconds, ratio)
  window.addEventListener("mousemove", handleTimelinePanMove)
  window.addEventListener("mouseup", stopTimelinePan)
}

const handleToggleFullscreen = async () => {
  const root = playerRootRef.value
  if (!root) {
    return
  }
  if (document.fullscreenElement) {
    await document.exitFullscreen()
    return
  }
  await root.requestFullscreen()
}

const captureCurrentFrame = (): string | null => {
  const element = videoRef.value
  if (!element || !element.videoWidth || !element.videoHeight) {
    return null
  }
  const canvas = document.createElement("canvas")
  canvas.width = element.videoWidth
  canvas.height = element.videoHeight
  const context = canvas.getContext("2d")
  if (!context) {
    return null
  }
  context.drawImage(element, 0, 0, canvas.width, canvas.height)
  return canvas.toDataURL("image/png")
}

const getPlaybackCurrentDateTime = (): string => {
  const startValue = props.playbackStartTime || props.playbackEndTime
  if (!startValue) {
    return ""
  }
  const start = new Date(startValue)
  if (Number.isNaN(start.getTime())) {
    return ""
  }
  return new Date(start.getTime() + Math.max(0, playbackCurrentTime.value) * 1000).toISOString()
}

defineExpose({
  captureCurrentFrame,
  getPlaybackCurrentDateTime,
})

onBeforeUnmount(() => {
  clearPlaybackRetry()
  destroyHls()
  stopTimelinePan()
})
</script>

<template>
  <section ref="playerRootRef" class="video-player" :class="{ 'video-player--active': props.active }">
    <div class="video-player__screen" @dblclick="handleToggleFullscreen">
      <HikWebControlPlayer
        v-if="props.browseMode === 'webcontrol'"
        :config="props.webControlConfig"
        :is-playing="props.isPlaying"
        :visible-slot-count="props.visibleSlotCount"
        :message="props.message"
      />
      <video
        v-else-if="props.isPlaying && props.playUrl && renderMode !== 'mock'"
        ref="videoRef"
        class="video-player__native"
        autoplay
        crossorigin="anonymous"
        :controls="!props.showSeekBar"
        muted
        playsinline
        @error="handlePlayerError"
        @loadedmetadata="handleLoadedMetadata"
        @loadeddata="handleFrameReady"
        @canplay="handleFrameReady"
        @durationchange="handleDurationChange"
        @timeupdate="handleTimeUpdate"
        @playing="handlePlaying"
      />
      <div v-else class="video-player__placeholder" />
      <div v-if="props.browseMode !== 'webcontrol' && !props.isPlaying && props.message" class="video-player__notice">
        {{ props.message }}
      </div>
      <img
        v-if="frameHoldActive && lastFrameUrl"
        class="video-player__frame-hold"
        :src="lastFrameUrl"
        alt=""
      />
      <div
        v-if="shouldShowPlaybackSeekBar"
        class="video-player__seekbar video-player__seekbar--overlay"
      >
        <div
          class="video-player__range-wrap"
          :class="{ 'video-player__range-wrap--pannable': timelineIsZoomed }"
          @mousemove="handleTimelineHover"
          @mouseleave="handleTimelineLeave"
          @wheel="handleTimelineWheel"
          @mousedown.middle.prevent="handleTimelinePanStart"
        >
          <div
            v-if="timelinePreviewVisible"
            class="video-player__preview"
            :style="{ left: timelinePreviewLeft }"
          >
            {{ timelinePreviewLabel }}
          </div>
          <div class="video-player__spans" aria-hidden="true">
            <span
              v-for="(span, index) in recordedSpanStyles"
              :key="`${span.left}-${span.width}-${index}`"
              class="video-player__span"
              :class="{ 'video-player__span--active': span.active }"
              :style="{ left: span.left, width: span.width }"
            />
          </div>
          <input
            class="video-player__range"
            type="range"
            :min="visibleTimelineStart"
            :max="visibleTimelineEnd"
            step="1"
            :value="playbackCurrentTime"
            :disabled="seekBarMax <= 0"
            @input="handleSeekInput"
            @change="handleSeekCommit"
            @mousedown.middle.prevent="handleTimelinePanStart"
          />
          <div class="video-player__ticks" aria-hidden="true">
            <span
              v-for="(tick, index) in seekTicks"
              :key="`${tick.value}-${index}`"
              class="video-player__tick"
              :class="{
                'video-player__tick--first': index === 0,
                'video-player__tick--last': index === seekTicks.length - 1,
              }"
              :style="{ left: tick.position }"
            >
              <i />
              <small>{{ tick.label }}</small>
            </span>
          </div>
        </div>
      </div>
    </div>
    <div
      v-if="shouldShowPlaybackSeekBar"
      class="video-player__seekbar video-player__seekbar--dock"
    >
      <div
        class="video-player__range-wrap"
        :class="{ 'video-player__range-wrap--pannable': timelineIsZoomed }"
        @mousemove="handleTimelineHover"
        @mouseleave="handleTimelineLeave"
        @wheel="handleTimelineWheel"
        @mousedown.middle.prevent="handleTimelinePanStart"
      >
        <div
          v-if="timelinePreviewVisible"
          class="video-player__preview"
          :style="{ left: timelinePreviewLeft }"
        >
          {{ timelinePreviewLabel }}
        </div>
        <div class="video-player__spans" aria-hidden="true">
          <span
            v-for="(span, index) in recordedSpanStyles"
            :key="`${span.left}-${span.width}-${index}`"
            class="video-player__span"
            :class="{ 'video-player__span--active': span.active }"
            :style="{ left: span.left, width: span.width }"
          />
        </div>
        <input
          class="video-player__range"
          type="range"
          :min="visibleTimelineStart"
          :max="visibleTimelineEnd"
          step="1"
          :value="playbackCurrentTime"
          :disabled="seekBarMax <= 0"
          @input="handleSeekInput"
          @change="handleSeekCommit"
          @mousedown.middle.prevent="handleTimelinePanStart"
        />
        <div class="video-player__ticks" aria-hidden="true">
          <span
            v-for="(tick, index) in seekTicks"
            :key="`${tick.value}-${index}`"
            class="video-player__tick"
            :class="{
              'video-player__tick--first': index === 0,
              'video-player__tick--last': index === seekTicks.length - 1,
            }"
            :style="{ left: tick.position }"
          >
            <i />
            <small>{{ tick.label }}</small>
          </span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.video-player {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  border-radius: 14px;
  overflow: hidden;
  border: 1px solid rgba(84, 129, 176, 0.22);
  background: linear-gradient(180deg, #102640 0%, #091727 100%);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.02);
}

.video-player--active {
  border-color: rgba(92, 174, 255, 0.64);
  box-shadow:
    0 0 0 1px rgba(92, 174, 255, 0.18),
    0 16px 36px rgba(7, 20, 39, 0.32);
}

.video-player__screen {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 28px;
  cursor: pointer;
  background:
    radial-gradient(circle at top, rgba(51, 123, 197, 0.22), transparent 42%),
    linear-gradient(180deg, #102640 0%, #091727 100%);
}

.video-player__native {
  width: 100%;
  height: 100%;
  min-height: 0;
  border-radius: 10px;
  background: #000000;
  object-fit: contain;
}

.video-player__placeholder {
  width: 100%;
  min-height: 250px;
  border-radius: 10px;
  background:
    radial-gradient(circle at top, rgba(51, 123, 197, 0.22), transparent 42%),
    linear-gradient(180deg, #102640 0%, #091727 100%);
}

.video-player__notice {
  position: absolute;
  z-index: 3;
  left: 50%;
  top: 50%;
  max-width: min(80%, 480px);
  transform: translate(-50%, -50%);
  padding: 12px 20px;
  border-radius: 999px;
  border: 1px solid rgba(129, 185, 244, 0.24);
  background: rgba(7, 20, 39, 0.7);
  color: rgba(237, 245, 255, 0.96);
  font-size: 18px;
  font-weight: 600;
  line-height: 1.4;
  text-align: center;
  letter-spacing: 0.02em;
  backdrop-filter: blur(6px);
  pointer-events: none;
}

.video-player__frame-hold {
  position: absolute;
  inset: 28px;
  z-index: 2;
  width: calc(100% - 56px);
  height: calc(100% - 56px);
  border-radius: 10px;
  object-fit: contain;
  background: #000000;
  pointer-events: none;
}

.video-player__seekbar {
  display: flex;
  align-items: center;
  gap: 0;
  padding: 12px 10px 26px;
  background: rgba(7, 19, 33, 0.92);
  border-top: 1px solid rgba(84, 129, 176, 0.18);
}

.video-player__seekbar--overlay {
  display: none;
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 5;
  border-top: 1px solid rgba(92, 174, 255, 0.28);
  background: rgba(5, 17, 30, 0.84);
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.36);
}

.video-player:fullscreen .video-player__screen {
  padding: 0;
}

.video-player:fullscreen .video-player__native {
  min-height: 0;
  border-radius: 0;
}

.video-player:fullscreen .video-player__frame-hold {
  inset: 0;
  width: 100%;
  height: 100%;
  border-radius: 0;
}

.video-player:fullscreen .video-player__seekbar--overlay {
  display: flex;
}

.video-player:fullscreen .video-player__seekbar--dock {
  display: none;
}

.video-player__range {
  appearance: none;
  -webkit-appearance: none;
  width: 100%;
  flex: 1;
  height: 18px;
  margin: 0;
  background: transparent;
  cursor: pointer;
}

.video-player__range::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 999px;
  background: rgba(214, 230, 246, 0.55);
}

.video-player__range::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  margin-top: -6px;
  border: 2px solid #cfe7ff;
  border-radius: 999px;
  background: #3b93e8;
  box-shadow: 0 0 0 3px rgba(59, 147, 232, 0.2);
}

.video-player__range::-moz-range-track {
  height: 4px;
  border-radius: 999px;
  background: rgba(214, 230, 246, 0.55);
}

.video-player__range::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border: 2px solid #cfe7ff;
  border-radius: 999px;
  background: #3b93e8;
  box-shadow: 0 0 0 3px rgba(59, 147, 232, 0.2);
}

.video-player__range-wrap {
  position: relative;
  flex: 1;
  width: 100%;
  min-width: 0;
  padding-bottom: 24px;
}

.video-player__spans {
  position: absolute;
  left: 0;
  right: 0;
  top: 7px;
  height: 10px;
  pointer-events: none;
}

.video-player__span {
  position: absolute;
  height: 10px;
  border-radius: 999px;
  background: rgba(68, 160, 255, 0.38);
  box-shadow:
    0 0 0 1px rgba(126, 197, 255, 0.12) inset,
    0 4px 10px rgba(7, 20, 39, 0.18);
}

.video-player__span--active {
  background: linear-gradient(180deg, rgba(73, 178, 255, 0.94), rgba(32, 122, 255, 0.94));
  box-shadow:
    0 0 0 1px rgba(170, 223, 255, 0.28) inset,
    0 8px 16px rgba(6, 18, 31, 0.26);
}

.video-player__range-wrap--pannable {
  cursor: default;
}

.video-player__range-wrap--pannable:active {
  cursor: default;
}

.video-player__ticks {
  position: absolute;
  left: 0;
  right: 0;
  top: 20px;
  height: 24px;
  pointer-events: none;
}

.video-player__preview {
  position: absolute;
  bottom: calc(100% + 8px);
  transform: translateX(-50%);
  padding: 6px 10px;
  border: 1px solid rgba(88, 181, 255, 0.48);
  border-radius: 8px;
  background: linear-gradient(180deg, rgba(32, 122, 255, 0.96), rgba(17, 92, 218, 0.96));
  color: #f4fbff;
  font-size: 11px;
  line-height: 1;
  white-space: nowrap;
  pointer-events: none;
  box-shadow:
    0 10px 24px rgba(6, 18, 31, 0.34),
    0 0 0 1px rgba(255, 255, 255, 0.08) inset;
}

.video-player__preview::after {
  content: "";
  position: absolute;
  top: 100%;
  left: 50%;
  width: 8px;
  height: 8px;
  border-right: 1px solid rgba(88, 181, 255, 0.48);
  border-bottom: 1px solid rgba(88, 181, 255, 0.48);
  background: rgba(17, 92, 218, 0.96);
  transform: translate(-50%, -4px) rotate(45deg);
}

.video-player__tick {
  position: absolute;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  transform: translateX(-50%);
  color: #9eb9d4;
  font-size: 10px;
  white-space: nowrap;
}

.video-player__tick--first {
  align-items: flex-start;
  transform: translateX(0);
}

.video-player__tick--last {
  align-items: flex-end;
  transform: translateX(-100%);
}

.video-player__tick i {
  display: block;
  width: 1px;
  height: 6px;
  background: rgba(214, 230, 246, 0.68);
}

.video-player__tick small {
  font-size: 10px;
  line-height: 1;
}
</style>

