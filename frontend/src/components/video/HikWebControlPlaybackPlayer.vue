<script setup lang="ts">
import { loadWebVideoCtrl } from "hikvideoctrl"
import { computed, nextTick, onBeforeUnmount, ref, useTemplateRef, watch } from "vue"

import type { LiveWebControlConfig, PlaybackTimelineSpan } from "../../types/video"

type HikSdkXmlResponse = { responseXML?: Document | null; responseText?: string }
type HikSdkXml = Document | string | HikSdkXmlResponse | null | undefined

interface HikDevicePort {
  iRtspPort?: number
  iWebSocketPort?: number
  iWebSocketsPort?: number
}

interface HikPlaybackRecord {
  startTime: string
  endTime: string
  playbackUri: string
  fileName: string
  recordType: string
}

interface HikWebVideoCtrl {
  I_SupportNoPlugin?: () => boolean
  I_InitPlugin: (width: string, height: string, options: Record<string, unknown>) => void
  I_InsertOBJECTPlugin: (containerId: string) => number
  I_Login: (
    ip: string,
    protocol: 1 | 2,
    port: number,
    username: string,
    password: string,
    options: Record<string, unknown>,
  ) => number
  I_Logout: (deviceId: string) => number
  I_GetDevicePort: (deviceId: string) => HikDevicePort | null
  I_RecordSearch: (
    deviceId: string,
    channelId: number,
    startTime: string,
    endTime: string,
    options: Record<string, unknown>,
  ) => number | void
  I_StartPlayback: (deviceId: string, options: Record<string, unknown>) => number | void
  I_StartDownloadRecord?: (
    deviceId: string,
    playbackUri: string,
    fileName: string,
    options: Record<string, unknown>,
  ) => Promise<unknown> | number | void
  I_StartDownloadRecordByTime?: (
    deviceId: string,
    playbackUri: string,
    fileName: string,
    startTime: string,
    endTime: string,
    options: Record<string, unknown>,
  ) => Promise<unknown> | number | void
  I_Stop: (options: Record<string, unknown>) => number | void
  I_StopAll?: () => Promise<unknown> | void
  I_Pause?: (options: Record<string, unknown>) => number | void
  I_Resume?: (options: Record<string, unknown>) => number | void
  I_PlayFast?: (options: Record<string, unknown>) => number | void
  I_PlaySlow?: (options: Record<string, unknown>) => number | void
  I_GetOSDTime?: (options: Record<string, unknown>) => number | void
  I2_CapturePic?: (
    picName: string,
    options?: {
      iWndIndex?: number
      cbCallback?: (data: Uint8Array) => void
    },
  ) => Promise<unknown> | number | void
  I_DestroyWorker?: () => void
  w_options?: { proxyAddress?: { ip: string; port: string | number } | null }
}

const emit = defineEmits<{
  seek: [offsetSeconds: number]
  seekEnd: [offsetSeconds: number]
  toggleFullscreen: [fullscreen?: boolean]
  playbackEnd: [offsetSeconds: number]
  fallback: [message: string]
}>()

const props = withDefaults(
  defineProps<{
    config?: LiveWebControlConfig | null
    message?: string
    playbackStartTime?: string
    playbackEndTime?: string
    showSeekBar?: boolean
    recordedSpans?: PlaybackTimelineSpan[]
    currentOffsetSeconds?: number
    playbackEndedErrorCodes?: number[]
  }>(),
  {
    config: null,
    message: "",
    playbackStartTime: "",
    playbackEndTime: "",
    showSeekBar: true,
    recordedSpans: () => [],
    currentOffsetSeconds: 0,
    playbackEndedErrorCodes: () => [],
  },
)

const containerRef = useTemplateRef<HTMLDivElement>("containerRef")
const playerState = ref<"idle" | "loading" | "ready">("idle")
const playerMessage = ref("")
const containerId = `hik-webcontrol-playback-${Math.random().toString(36).slice(2, 10)}`

const sdkScriptUrl = import.meta.env.VITE_HIK_WEBCTRL_SCRIPT_URL ?? "/codebase/webVideoCtrl.js?v=20260530-loginfix7"
const sdkDependencyUrls = [
  "/codebase/jsPlugin/jquery.min.js",
  "/codebase/encryption/AES.js",
  "/codebase/encryption/cryptico.min.js",
  "/codebase/encryption/crypto-3.1.2.min.js",
]
const pluginErrorMessages: Record<number, string> = {
  1001: "码流传输异常。",
  1003: "取流失败，连接被动断开。",
  1006: "视频编码格式不支持，目前仅支持 H.264 / H.265。",
  1007: "网络异常导致 WebSocket 断开。",
  1008: "首帧回调超时，通常是 WebSocket 端口协商或代理转发失败。",
  1011: "数据接收异常，请检查设备视频格式。",
  1012: "播放资源不足。",
  1015: "获取播放 URL 失败。",
  1017: "设备用户名或密码错误。",
  1020: "当前通道需要重新播放。",
  1021: "播放缓存溢出。",
}
const LOAD_TIMEOUT_MS = 15000
const LOGIN_TIMEOUT_MS = 15000
const PLAYBACK_TIMEOUT_MS = 15000
const SEARCH_TIMEOUT_MS = 15000
const PROXY_COOKIE_MAX_AGE_SECONDS = 300
const TRANSIENT_PLUGIN_ERROR_DELAY_MS = 1800
const PLAYBACK_RETRY_DELAY_MS = 160
let sdk: HikWebVideoCtrl | null = null
let sdkInitialized = false
let activeDeviceId: string | null = null
let startToken = 0
let pendingPluginErrorTimer: number | null = null

const visibleMessage = computed(() => {
  if (playerMessage.value) return playerMessage.value
  if (props.message) return props.message
  if (playerState.value === "loading") return "HIK 连接中..."
  return ""
})

const selectedPlaybackStart = computed(() => {
  const value = new Date(props.playbackStartTime || "")
  return Number.isNaN(value.getTime()) ? null : value
})

const selectedPlaybackEnd = computed(() => {
  const value = new Date(props.playbackEndTime || "")
  return Number.isNaN(value.getTime()) ? null : value
})

const seekBarMax = computed(() => {
  if (!selectedPlaybackStart.value || !selectedPlaybackEnd.value) {
    return 0
  }
  return Math.max(0, Math.round((selectedPlaybackEnd.value.getTime() - selectedPlaybackStart.value.getTime()) / 1000))
})

const formatSeekLabel = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value ? value.replace("T", " ").slice(11, 19) : "--:--:--"
  }
  const pad = (item: number) => String(item).padStart(2, "0")
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const addSecondsToDateTime = (value: string, seconds: number) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Date(date.getTime() + Math.max(0, seconds) * 1000).toISOString()
}

const previewLabel = ref("")
const previewLeft = ref("0%")
const previewVisible = ref(false)
const isSeeking = ref(false)
const isTimelinePanning = ref(false)
const seekInputValue = ref(0)
const pendingSeekValue = ref<number | null>(null)
const timelineViewStart = ref(0)
const timelineViewDuration = ref(0)
const MIN_TIMELINE_WINDOW_SECONDS = 300
const SEEK_COMMIT_RELEASE_MS = 1200
let seekReleaseTimer: number | null = null

const clampNumber = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

const clearSeekReleaseTimer = () => {
  if (seekReleaseTimer !== null) {
    window.clearTimeout(seekReleaseTimer)
    seekReleaseTimer = null
  }
}

const resolveSpanOffset = (offsetSeconds: number) => {
  const normalizedOffset = clampNumber(offsetSeconds, 0, seekBarMax.value)
  if (!selectedPlaybackStart.value || !props.recordedSpans.length) {
    return normalizedOffset
  }
  const axisStartMs = selectedPlaybackStart.value.getTime()
  const targetMs = axisStartMs + normalizedOffset * 1000
  let snappedOffset = normalizedOffset
  let bestDistance = Number.POSITIVE_INFINITY

  for (const span of props.recordedSpans) {
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
  return clampNumber(snappedOffset, 0, seekBarMax.value)
}

const visibleTimelineDuration = computed(() => {
  if (seekBarMax.value <= 0) {
    return 0
  }
  const fallback = timelineViewDuration.value > 0 ? timelineViewDuration.value : seekBarMax.value
  return clampNumber(fallback, Math.min(MIN_TIMELINE_WINDOW_SECONDS, seekBarMax.value), seekBarMax.value)
})

const visibleTimelineStart = computed(() => {
  if (seekBarMax.value <= 0 || visibleTimelineDuration.value >= seekBarMax.value) {
    return 0
  }
  return clampNumber(timelineViewStart.value, 0, seekBarMax.value - visibleTimelineDuration.value)
})

const visibleTimelineEnd = computed(() => Math.min(seekBarMax.value, visibleTimelineStart.value + visibleTimelineDuration.value))
const timelineIsZoomed = computed(() =>
  seekBarMax.value > 0 && visibleTimelineDuration.value < seekBarMax.value - 1,
)

const seekSliderValue = computed(() => {
  const baseValue = isSeeking.value ? seekInputValue.value : resolveSpanOffset(props.currentOffsetSeconds)
  return clampNumber(baseValue, visibleTimelineStart.value, visibleTimelineEnd.value)
})

const recordedSpanStyles = computed(() => {
  if (!selectedPlaybackStart.value || !selectedPlaybackEnd.value) {
    return []
  }
  const axisStartMs = selectedPlaybackStart.value.getTime()
  const windowStartMs = axisStartMs + visibleTimelineStart.value * 1000
  const windowDurationMs = visibleTimelineDuration.value * 1000
  const windowEndMs = windowStartMs + windowDurationMs
  if (windowDurationMs <= 0 || windowEndMs <= windowStartMs) {
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
        left: `${((clippedStartMs - windowStartMs) / windowDurationMs) * 100}%`,
        width: `${Math.max(0.35, ((clippedEndMs - clippedStartMs) / windowDurationMs) * 100)}%`,
        active:
          props.currentOffsetSeconds >= Math.round((clippedStartMs - axisStartMs) / 1000)
          && props.currentOffsetSeconds <= Math.round((clippedEndMs - axisStartMs) / 1000),
      }
    })
    .filter((item): item is { left: string; width: string; active: boolean } => Boolean(item))
})

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
      label: formatSeekLabel(addSecondsToDateTime(props.playbackStartTime, value)),
      position: `${ratio * 100}%`,
    }
  })
})

const revealTimelineOffset = (offsetSeconds: number) => {
  const duration = visibleTimelineDuration.value
  if (duration <= 0 || duration >= seekBarMax.value) {
    return
  }
  if (offsetSeconds < visibleTimelineStart.value) {
    timelineViewStart.value = clampNumber(offsetSeconds, 0, seekBarMax.value - duration)
    return
  }
  if (offsetSeconds > visibleTimelineEnd.value) {
    timelineViewStart.value = clampNumber(offsetSeconds - duration * 0.85, 0, seekBarMax.value - duration)
  }
}

const getTimelinePointerState = (event: MouseEvent, wrap: HTMLElement) => {
  const rect = wrap.getBoundingClientRect()
  const ratio = rect.width > 0 ? clampNumber((event.clientX - rect.left) / rect.width, 0, 1) : 0
  const rawSeconds = visibleTimelineStart.value + visibleTimelineDuration.value * ratio
  const snappedSeconds = resolveSpanOffset(rawSeconds)
  const snappedRatio = visibleTimelineDuration.value > 0
    ? clampNumber((snappedSeconds - visibleTimelineStart.value) / visibleTimelineDuration.value, 0, 1)
    : ratio
  return { ratio: snappedRatio, seconds: snappedSeconds }
}

const updatePreviewByOffset = (offsetSeconds: number, ratio: number) => {
  previewLabel.value = formatSeekLabel(addSecondsToDateTime(props.playbackStartTime, offsetSeconds))
  previewLeft.value = `${Math.min(100, Math.max(0, ratio * 100))}%`
  previewVisible.value = true
}

const handlePreviewUpdate = (event: Event) => {
  const target = event.target as HTMLInputElement
  const value = resolveSpanOffset(Number(target.value || 0))
  clearSeekReleaseTimer()
  isSeeking.value = true
  pendingSeekValue.value = null
  seekInputValue.value = value
  const min = Number(target.min || 0)
  const max = Number(target.max || seekBarMax.value || 0)
  const ratio = max > min ? (value - min) / (max - min) : 0
  updatePreviewByOffset(value, ratio)
}

const handleSeekCommit = (event: Event) => {
  const target = event.target as HTMLInputElement
  const value = resolveSpanOffset(Number(target.value || 0))
  clearSeekReleaseTimer()
  isSeeking.value = true
  seekInputValue.value = value
  pendingSeekValue.value = value
  if (seekBarMax.value > 0 && value >= seekBarMax.value) {
    emit("seekEnd", seekBarMax.value)
  } else {
    emit("seek", value)
  }
  previewVisible.value = false
  seekReleaseTimer = window.setTimeout(() => {
    if (pendingSeekValue.value === value) {
      isSeeking.value = false
      pendingSeekValue.value = null
      seekInputValue.value = resolveSpanOffset(props.currentOffsetSeconds)
    }
  }, SEEK_COMMIT_RELEASE_MS)
}

const handleTimelineHover = (event: MouseEvent) => {
  const wrap = event.currentTarget as HTMLElement | null
  if (!wrap) {
    return
  }
  const { ratio, seconds } = getTimelinePointerState(event, wrap)
  updatePreviewByOffset(seconds, ratio)
}

const handleLeavePreview = () => {
  previewVisible.value = false
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
  updatePreviewByOffset(seconds, ratio)
}

let timelinePanOriginX = 0
let timelinePanOriginStart = 0
let timelinePanWidth = 0
let timelinePanLeft = 0

const resolveTimelineWrap = (target: EventTarget | null) => {
  if (!(target instanceof HTMLElement)) {
    return null
  }
  if (target.classList.contains("hik-webcontrol-playback-player__range-wrap")) {
    return target
  }
  const wrap = target.closest(".hik-webcontrol-playback-player__range-wrap")
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
    previewVisible.value = false
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
  updatePreviewByOffset(seconds, ratio)
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
  updatePreviewByOffset(seconds, ratio)
  window.addEventListener("mousemove", handleTimelinePanMove)
  window.addEventListener("mouseup", stopTimelinePan)
}

watch(seekBarMax, (value) => {
  if (value <= 0) {
    timelineViewStart.value = 0
    timelineViewDuration.value = 0
    isSeeking.value = false
    seekInputValue.value = 0
    pendingSeekValue.value = null
    clearSeekReleaseTimer()
    return
  }
  if (timelineViewDuration.value <= 0 || timelineViewDuration.value > value) {
    timelineViewDuration.value = value
    timelineViewStart.value = 0
  } else {
    timelineViewStart.value = clampNumber(timelineViewStart.value, 0, Math.max(0, value - timelineViewDuration.value))
  }
  if (!isSeeking.value) {
    seekInputValue.value = resolveSpanOffset(props.currentOffsetSeconds)
  }
})

watch(
  () => props.currentOffsetSeconds,
  (value) => {
    revealTimelineOffset(value)
    if (pendingSeekValue.value !== null && Math.abs(value - pendingSeekValue.value) <= 1) {
      clearSeekReleaseTimer()
      isSeeking.value = false
      pendingSeekValue.value = null
    }
    if (!isSeeking.value) {
      seekInputValue.value = resolveSpanOffset(value)
    }
  },
)

watch(
  () => [visibleTimelineStart.value, visibleTimelineEnd.value] as const,
  () => {
    seekInputValue.value = clampNumber(seekInputValue.value, visibleTimelineStart.value, visibleTimelineEnd.value)
  },
)


const isSdkXmlResponse = (payload: HikSdkXml): payload is HikSdkXmlResponse =>
  typeof payload === "object" && payload !== null && !(typeof Document !== "undefined" && payload instanceof Document)

const parseXmlDocument = (payload: HikSdkXml): Document | null => {
  if (!payload) return null
  if (typeof Document !== "undefined" && payload instanceof Document) return payload
  if (typeof payload === "string") return new DOMParser().parseFromString(payload, "application/xml")
  if (isSdkXmlResponse(payload) && payload.responseXML) return payload.responseXML
  if (isSdkXmlResponse(payload) && payload.responseText) {
    return new DOMParser().parseFromString(payload.responseText, "application/xml")
  }
  return null
}

const getXmlText = (node: ParentNode, selector: string) => node.querySelector(selector)?.textContent?.trim() ?? ""

const resolveHttpStatusMessage = (payload: HikSdkXml, status?: number) => {
  const xml = parseXmlDocument(payload)
  const statusString = xml ? getXmlText(xml, "statusString") : ""
  const subStatusCode = xml ? getXmlText(xml, "subStatusCode") : ""
  if (status === 401 || subStatusCode.toLowerCase().includes("unauthorized")) {
    return "设备认证请求被拒绝，请检查 HTTP/HTTPS 协议、端口或代理转发。"
  }
  if (status === 403) {
    return "设备不支持当前 HIK 无插件方式，或代理转发被拒绝。"
  }
  if (statusString) {
    return `设备返回 ${statusString}${subStatusCode ? ` / ${subStatusCode}` : ""}。`
  }
  return status ? `设备请求失败，状态码 ${status}。` : "设备请求失败。"
}

const setCookieValue = (name: string, value: string) => {
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; max-age=${PROXY_COOKIE_MAX_AGE_SECONDS}; samesite=lax`
}

const clearCookieValue = (name: string) => {
  document.cookie = `${name}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; samesite=lax`
}

const syncHttpProxyCookie = (config: LiveWebControlConfig) => {
  setCookieValue("webVideoCtrlProxy", `${config.protocol}://${config.host}:${config.port}`)
}

const syncWebSocketProxyCookie = (config: LiveWebControlConfig, webSocketPort?: number) => {
  if (!webSocketPort) {
    clearCookieValue("webVideoCtrlProxyWs")
    clearCookieValue("webVideoCtrlProxyWss")
    return
  }
  if (config.protocol === "https") {
    clearCookieValue("webVideoCtrlProxyWs")
    setCookieValue("webVideoCtrlProxyWss", `https://${config.host}:${webSocketPort}`)
    return
  }
  clearCookieValue("webVideoCtrlProxyWss")
  setCookieValue("webVideoCtrlProxyWs", `http://${config.host}:${webSocketPort}`)
}

const clearProxyCookies = () => {
  clearCookieValue("webVideoCtrlProxy")
  clearCookieValue("webVideoCtrlProxyWs")
  clearCookieValue("webVideoCtrlProxyWss")
}

const getErrorMessage = (error: unknown) =>
  error instanceof Error ? error.message : String(error ?? "")

const isRetryableLoginError = (error: unknown) => {
  const message = getErrorMessage(error).toLowerCase()
  return [
    "设备登录失败",
    "用户名或密码错误",
    "401",
    "unauthorized",
    "1017",
    "设备请求失败",
    "同步返回 -1",
  ].some((keyword) => message.includes(keyword.toLowerCase()))
}

const isRetryablePlaybackStartError = (error: unknown) => {
  const message = getErrorMessage(error).toLowerCase()
  return [
    "设备请求失败",
    "同步返回 -1",
    "1003",
    "1007",
    "1008",
    "1020",
    "直连+自动协商端口",
    "直连+显式ws端口",
    "启动 hik 回放",
  ].some((keyword) => message.includes(keyword.toLowerCase()))
}

const normalizePositivePort = (port?: number | null) => (typeof port === "number" && Number.isFinite(port) && port > 0 ? port : undefined)

const sleep = (ms: number) =>
  new Promise<void>((resolve) => {
    window.setTimeout(resolve, Math.max(0, ms))
  })

const withTimeout = async <T,>(promise: Promise<T>, timeoutMs: number, stepLabel: string): Promise<T> =>
  await Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${stepLabel}超时`)), timeoutMs)
    }),
  ])

const loadScriptOnce = async (scriptUrl: string) => {
  const absoluteUrl = new URL(scriptUrl, window.location.origin).toString()
  const existingScript = document.querySelector<HTMLScriptElement>(`script[data-hik-sdk-url="${absoluteUrl}"]`)
  if (existingScript?.dataset.loaded === "true") return

  await new Promise<void>((resolve, reject) => {
    if (existingScript) {
      existingScript.addEventListener("load", () => resolve(), { once: true })
      existingScript.addEventListener("error", () => reject(new Error(`加载脚本失败: ${absoluteUrl}`)), { once: true })
      return
    }

    const script = document.createElement("script")
    script.src = absoluteUrl
    script.async = false
    script.dataset.hikSdkUrl = absoluteUrl
    script.addEventListener(
      "load",
      () => {
        script.dataset.loaded = "true"
        resolve()
      },
      { once: true },
    )
    script.addEventListener("error", () => reject(new Error(`加载脚本失败: ${absoluteUrl}`)), { once: true })
    document.head.appendChild(script)
  })
}

const getSdk = async () => {
  for (const dependencyUrl of sdkDependencyUrls) {
    await loadScriptOnce(dependencyUrl)
  }
  const loadedSdk = await loadWebVideoCtrl(sdkScriptUrl)
  sdk = loadedSdk as unknown as HikWebVideoCtrl
  return sdk
}

const wrapSdkCallback = async <T,>(
  stepLabel: string,
  timeoutMs: number,
  invoker: (callbacks: {
    success: (payload?: T) => void
    error: (status?: number, payload?: HikSdkXml) => void
  }) => number | void,
  options?: {
    allowRetMinusOne?: boolean
  },
): Promise<T | undefined> =>
  await withTimeout(
    new Promise<T | undefined>((resolve, reject) => {
      const ret = invoker({
        success: (payload?: T) => {
          resolve(payload)
        },
        error: (status?: number, payload?: HikSdkXml) => {
          reject(new Error(`${stepLabel}失败：${resolveHttpStatusMessage(payload, status)}`))
        },
      })
      if (ret === -1) {
        if (options?.allowRetMinusOne) {
          resolve(undefined)
          return
        }
        reject(new Error(`${stepLabel}同步返回 -1`))
      }
    }),
    timeoutMs,
    stepLabel,
  )

const ensureSdkInitialized = async () => {
  const sdkInstance = await getSdk()
  if (!sdkInstance.I_SupportNoPlugin?.()) {
    throw new Error("当前浏览器不支持 HIK 无插件播放器，请使用 Chromium 内核浏览器。")
  }
  if (sdkInitialized) {
    sdkInstance.w_options = sdkInstance.w_options ?? {}
    sdkInstance.w_options.proxyAddress = null
    return sdkInstance
  }

  const container = containerRef.value
  if (!container) {
    throw new Error("播放器容器尚未准备完成。")
  }

  await withTimeout(
    new Promise<void>((resolve, reject) => {
      const options = {
        iWndowType: 1,
        bNoPlugin: true,
        bWndFull: true,
        iPlayMode: 2,
        iPackageType: 2,
        cbDoubleClickWnd: (_windowIndex: number, fullscreen: boolean) => {
          emit("toggleFullscreen", fullscreen)
        },
        cbEvent: (_eventType: number, _param1: number, param2: number) => {
          handleSdkPluginEvent(param2)
        },
        cbPluginErrorHandler: (_windowIndex: number, errorCode: number) => {
          handleSdkPluginEvent(errorCode)
        },
        cbInitPluginComplete: () => {
          try {
            const result = sdkInstance.I_InsertOBJECTPlugin(containerId)
            if (result !== 0) {
              reject(new Error(`插入 HIK 播放器失败，返回值 ${result}`))
              return
            }
            sdkInitialized = true
            resolve()
          } catch (error) {
            reject(error)
          }
        },
      }
      try {
        sdkInstance.I_InitPlugin("100%", "100%", options)
      } catch (error) {
        reject(error)
      }
    }),
    LOAD_TIMEOUT_MS,
    "初始化 HIK 播放器",
  )

  return sdkInstance
}

const ensureLoggedIn = async (config: LiveWebControlConfig) => {
  const sdkInstance = await ensureSdkInitialized()
  const deviceId = `${config.host}_${config.port}`
  syncHttpProxyCookie(config)
  if (activeDeviceId === deviceId) {
    return { sdkInstance, deviceId }
  }

  if (activeDeviceId) {
    try {
      sdkInstance.I_Logout(activeDeviceId)
    } catch {
      // Ignore logout errors when switching devices.
    }
    activeDeviceId = null
  }

  const protocolValue = config.protocol === "https" ? 2 : 1
  syncHttpProxyCookie(config)
  await wrapSdkCallback(
    "登录 HIK 设备",
    LOGIN_TIMEOUT_MS,
    (callbacks) =>
      sdkInstance.I_Login(
        config.host,
        protocolValue,
        config.port,
        config.username,
        config.password,
        {
          success: callbacks.success,
          error: callbacks.error,
        },
      ),
    { allowRetMinusOne: true },
  )
  activeDeviceId = deviceId
  return { sdkInstance, deviceId }
}

const reloginDevice = async (config: LiveWebControlConfig) => {
  const sdkInstance = await ensureSdkInitialized()
  const deviceId = `${config.host}_${config.port}`
  if (activeDeviceId === deviceId) {
    try {
      sdkInstance.I_Logout(deviceId)
    } catch {
      // Ignore logout errors when forcing a fresh login.
    }
    activeDeviceId = null
  }
  return await ensureLoggedIn(config)
}

const withReloginRetry = async <T,>(
  config: LiveWebControlConfig,
  action: () => Promise<T>,
): Promise<T> => {
  try {
    return await action()
  } catch (error) {
    if (!isRetryableLoginError(error)) {
      throw error
    }
  }
  await reloginDevice(config)
  return await action()
}

const clearPendingPluginError = () => {
  if (pendingPluginErrorTimer !== null) {
    window.clearTimeout(pendingPluginErrorTimer)
    pendingPluginErrorTimer = null
  }
}

const stopActivePlayback = () => {
  try {
    sdk?.I_Stop?.({ iWndIndex: 0 })
  } catch {
    // Ignore stop errors during re-init.
  }
}

const stopPlayback = async (options?: { silent?: boolean }) => {
  if (!sdkInitialized || !sdk) {
    playerState.value = "idle"
    return
  }
  clearPendingPluginError()
  try {
    await wrapSdkCallback(
      "停止 HIK 回放",
      6000,
      (callbacks) =>
        sdk?.I_Stop({
          iWndIndex: 0,
          success: callbacks.success,
          error: callbacks.error,
        }),
      { allowRetMinusOne: true },
    )
  } catch (error) {
    if (!options?.silent) {
      throw error
    }
  } finally {
    playerState.value = "idle"
  }
}

const setPlaybackLoading = (message = "HIK 回放恢复中...") => {
  clearPendingPluginError()
  playerState.value = "loading"
  playerMessage.value = message
}

const clearPlaybackLoading = () => {
  clearPendingPluginError()
  playerState.value = "ready"
  playerMessage.value = ""
}

const handleSdkPluginEvent = (errorCode: number) => {
  if (!errorCode) {
    return
  }
  clearPendingPluginError()
  if (props.playbackEndedErrorCodes.includes(errorCode)) {
    playerMessage.value = ""
    playerState.value = "idle"
    emit("playbackEnd", seekBarMax.value)
    return
  }
  const message = pluginErrorMessages[errorCode] ?? `HIK 播放器错误，代码 ${errorCode}。`
  if (errorCode === 1003) {
    pendingPluginErrorTimer = window.setTimeout(() => {
      pendingPluginErrorTimer = null
      if (playerState.value === "ready") {
        return
      }
      playerMessage.value = message
      playerState.value = "idle"
      emit("fallback", message)
    }, TRANSIENT_PLUGIN_ERROR_DELAY_MS)
    return
  }
  playerMessage.value = message
  playerState.value = "idle"
  emit("fallback", message)
}

const parseRecordSearchXml = (xmlDoc: Document | null): { status: string; items: HikPlaybackRecord[] } => {
  if (!xmlDoc) {
    return { status: "", items: [] }
  }
  const status = getXmlText(xmlDoc, "responseStatusStrg")
  const items = Array.from(xmlDoc.querySelectorAll("searchMatchItem"))
    .map((node) => {
      const playbackUri = getXmlText(node, "playbackURI")
      const startTime = getXmlText(node, "startTime")
      const endTime = getXmlText(node, "endTime")
      const recordType = getXmlText(node, "metadataDescriptor")
      const fileNameMatch = playbackUri.match(/[?&]name=([^&]+)/)
      const fileName = fileNameMatch?.[1] ? decodeURIComponent(fileNameMatch[1]) : ""
      return {
        startTime,
        endTime,
        playbackUri,
        fileName,
        recordType,
      }
    })
    .filter((item) => item.playbackUri && item.startTime && item.endTime)
  return { status, items }
}

const searchRecords = async (params: {
  startTime: string
  endTime: string
  streamType?: 1 | 2
}): Promise<HikPlaybackRecord[]> => {
  const config = props.config
  if (!config) {
    throw new Error("请先选择通道。")
  }
  if (!params.startTime || !params.endTime) {
    throw new Error("请先选择查询时间范围。")
  }

  playerMessage.value = ""
  return await withReloginRetry(config, async () => {
    const { sdkInstance, deviceId } = await ensureLoggedIn(config)
    const result: HikPlaybackRecord[] = []
    let searchPos = 0

    while (true) {
      const payload = await wrapSdkCallback<HikSdkXml>(
        "搜索 HIK 录像",
        SEARCH_TIMEOUT_MS,
        (callbacks) =>
          sdkInstance.I_RecordSearch(deviceId, config.channelNo, params.startTime, params.endTime, {
            async: false,
            iStreamType: params.streamType ?? config.streamType,
            iSearchPos: searchPos,
            success: callbacks.success,
            error: callbacks.error,
          }),
      )
      const xml = parseXmlDocument(payload)
      const parsed = parseRecordSearchXml(xml)
      result.push(...parsed.items)
      if (parsed.status === "MORE") {
        searchPos += 40
        continue
      }
      if (parsed.status === "NO MATCHES" || !parsed.status || parsed.status === "OK") {
        break
      }
      break
    }

    return result
  })
}

const startPlayback = async (params: {
  startTime: string
  endTime: string
  streamType?: 1 | 2
}) => {
  const config = props.config
  if (!config) {
    throw new Error("当前通道缺少 HIK 连接配置。")
  }

  const token = ++startToken
  clearPendingPluginError()
  playerState.value = "loading"
  playerMessage.value = ""

  const runStartPlayback = async () => {
    await withReloginRetry(config, async () => {
      const { sdkInstance, deviceId } = await ensureLoggedIn(config)
      if (token !== startToken) return

      const ports = sdkInstance.I_GetDevicePort(deviceId)
      const rtspPort = normalizePositivePort(config.rtspPort) ?? normalizePositivePort(ports?.iRtspPort)
      const webSocketPort =
        normalizePositivePort(config.webSocketPort)
        ?? normalizePositivePort(config.protocol === "https" ? ports?.iWebSocketsPort : ports?.iWebSocketPort)

      await stopPlayback({ silent: true })
      stopActivePlayback()
      await nextTick()
      await sleep(PLAYBACK_RETRY_DELAY_MS)

      const createAttempt = (label: string, useProxy: boolean, explicitWebSocketPort?: number) => ({
        label,
        useProxy,
        ...(explicitWebSocketPort ? { webSocketPort: explicitWebSocketPort } : {}),
      })
      const directAttempts = [
        ...(webSocketPort ? [createAttempt("直连+显式WS端口", false, webSocketPort)] : []),
        createAttempt("直连+自动协商端口", false),
      ]
      const proxyAttempts = [
        ...(webSocketPort ? [createAttempt("代理+显式WS端口", true, webSocketPort)] : []),
        createAttempt("代理+自动协商端口", true),
      ]
      const attempts = config.useProxy ? [...proxyAttempts, ...directAttempts] : [...directAttempts, ...proxyAttempts]

      let lastError: unknown = null
      for (const attempt of attempts) {
        try {
          syncWebSocketProxyCookie(config, attempt.webSocketPort)
          await wrapSdkCallback(
            `启动 HIK 回放(${attempt.label})`,
            PLAYBACK_TIMEOUT_MS,
            (callbacks) =>
              sdkInstance.I_StartPlayback(deviceId, {
                iWndIndex: 0,
                iRtspPort: rtspPort,
                ...(attempt.webSocketPort ? { iWSPort: attempt.webSocketPort } : {}),
                iStreamType: params.streamType ?? config.streamType,
                iChannelID: config.channelNo,
                szStartTime: params.startTime,
                szEndTime: params.endTime,
                bProxy: attempt.useProxy,
                success: callbacks.success,
                error: callbacks.error,
              }),
          )
          lastError = null
          break
        } catch (error) {
          lastError = error
          stopActivePlayback()
          if (attempt.webSocketPort) {
            await sleep(PLAYBACK_RETRY_DELAY_MS)
          }
        }
      }

      if (lastError) {
        throw lastError
      }
    })
  }

  try {
    await runStartPlayback()

    if (token !== startToken) return
    playerState.value = "ready"
    playerMessage.value = ""
  } catch (error) {
    if (token !== startToken) return
    if (isRetryablePlaybackStartError(error)) {
      await teardownPlayer({ bumpStartToken: false })
      if (token !== startToken) return
      await sleep(PLAYBACK_RETRY_DELAY_MS)
      clearPendingPluginError()
      playerState.value = "loading"
      playerMessage.value = "HIK 回放重试中..."
      try {
        await runStartPlayback()
        if (token !== startToken) return
        playerState.value = "ready"
        playerMessage.value = ""
        return
      } catch (retryError) {
        if (token !== startToken) return
        const retryMessage = getErrorMessage(retryError)
        await teardownPlayer({ bumpStartToken: false })
        playerState.value = "idle"
        playerMessage.value = retryMessage
        throw retryError
      }
    }
    const message = getErrorMessage(error)
    await teardownPlayer({ bumpStartToken: false })
    playerState.value = "idle"
    playerMessage.value = message
    throw error
  }
}

const pausePlayback = async () => {
  if (!sdk?.I_Pause) throw new Error("当前 HIK 播放器不支持暂停。")
  await wrapSdkCallback(
    "暂停 HIK 回放",
    6000,
    (callbacks) =>
      sdk?.I_Pause?.({
        success: callbacks.success,
        error: callbacks.error,
      }),
  )
}

const resumePlayback = async () => {
  if (!sdk?.I_Resume) throw new Error("当前 HIK 播放器不支持继续播放。")
  await wrapSdkCallback(
    "继续 HIK 回放",
    6000,
    (callbacks) =>
      sdk?.I_Resume?.({
        success: callbacks.success,
        error: callbacks.error,
      }),
  )
}

const playFast = async () => {
  if (!sdk?.I_PlayFast) throw new Error("当前 HIK 播放器不支持快放。")
  await wrapSdkCallback(
    "HIK 快放",
    6000,
    (callbacks) =>
      sdk?.I_PlayFast?.({
        success: callbacks.success,
        error: callbacks.error,
      }),
  )
}

const playSlow = async () => {
  if (!sdk?.I_PlaySlow) throw new Error("当前 HIK 播放器不支持慢放。")
  await wrapSdkCallback(
    "HIK 慢放",
    6000,
    (callbacks) =>
      sdk?.I_PlaySlow?.({
        success: callbacks.success,
        error: callbacks.error,
      }),
  )
}

const getOSDTime = async () => {
  if (!sdk?.I_GetOSDTime) {
    throw new Error("当前 HIK 播放器不支持获取 OSD 时间。")
  }
  const getOsdTime = sdk.I_GetOSDTime
  try {
    const osdTime = await wrapSdkCallback<string>(
      "获取 HIK OSD 时间",
      3000,
      (callbacks) =>
        getOsdTime({
          success: callbacks.success,
          error: callbacks.error,
        }),
    )
    return osdTime ?? ""
  } catch (error) {
    throw error
  }
}

const resolvePromiseLike = async (value: Promise<unknown> | number | void, stepLabel: string) => {
  if (typeof value === "number" && value < 0) {
    throw new Error(`${stepLabel}失败`)
  }
  if (value && typeof (value as Promise<unknown>).then === "function") {
    await value
  }
}

const uint8ArrayToDataUrl = async (data: Uint8Array) => {
  let binary = ""
  for (const byte of data) {
    binary += String.fromCharCode(byte)
  }
  return `data:image/jpeg;base64,${window.btoa(binary)}`
}

const captureCurrentFrame = async () => {
  if (!sdk?.I2_CapturePic) {
    throw new Error("当前 HIK 播放器不支持截图。")
  }
  let capturedDataUrl: string | null = null
  const fileName = `hik-playback-${Date.now()}.jpg`
  const captureResult = sdk.I2_CapturePic(fileName, {
    iWndIndex: 0,
    cbCallback: (data) => {
      void uint8ArrayToDataUrl(data).then((result) => {
        capturedDataUrl = result
      })
    },
  })
  await resolvePromiseLike(captureResult, "HIK 回放截图")
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (capturedDataUrl) {
      return capturedDataUrl
    }
    await new Promise<void>((resolve) => {
      window.setTimeout(resolve, 50)
    })
  }
  throw new Error("HIK 回放截图数据未返回。")
}

const downloadRecord = async (params: {
  playbackUri: string
  fileName: string
  dateDir?: boolean
}) => {
  const config = props.config
  if (!config) {
    throw new Error("当前通道缺少 HIK 连接配置。")
  }
  if (!params.playbackUri) {
    throw new Error("当前录像片段缺少 playbackURI，无法下载。")
  }
  const { sdkInstance, deviceId } = await ensureLoggedIn(config)
  if (!sdkInstance.I_StartDownloadRecord) {
    throw new Error("当前 HIK 播放器不支持录像下载。")
  }
  const targetFileName = params.fileName?.trim() || "playback"
  playerMessage.value = "HIK 录像下载已开始，请留意浏览器下载列表。"
  try {
    await resolvePromiseLike(
      sdkInstance.I_StartDownloadRecord(deviceId, params.playbackUri, targetFileName, {
        bDateDir: params.dateDir ?? true,
      }),
      "启动 HIK 录像下载",
    )
  } catch (error) {
    playerMessage.value = error instanceof Error ? error.message : String(error || "HIK SDK 未返回具体错误。")
    throw error
  }
}

const downloadRecordByTime = async (params: {
  playbackUri: string
  fileName: string
  startTime: string
  endTime: string
  dateDir?: boolean
}) => {
  const config = props.config
  if (!config) {
    throw new Error("当前通道缺少 HIK 连接配置。")
  }
  if (!params.playbackUri) {
    throw new Error("当前录像片段缺少 playbackURI，无法按时间下载。")
  }
  const { sdkInstance, deviceId } = await ensureLoggedIn(config)
  if (!sdkInstance.I_StartDownloadRecordByTime) {
    throw new Error("当前 HIK 播放器不支持按时间下载录像。")
  }
  const targetFileName = params.fileName?.trim() || "playback"
  playerMessage.value = "HIK 按时间下载录像已开始，请留意浏览器下载列表。"
  try {
    await resolvePromiseLike(
      sdkInstance.I_StartDownloadRecordByTime(
        deviceId,
        params.playbackUri,
        targetFileName,
        params.startTime,
        params.endTime,
        {
          bDateDir: params.dateDir ?? true,
        },
      ),
      "启动 HIK 按时间下载",
    )
  } catch (error) {
    playerMessage.value = error instanceof Error ? error.message : String(error)
    throw error
  }
}

const teardownPlayer = async (options?: { bumpStartToken?: boolean }) => {
  if (options?.bumpStartToken !== false) {
    startToken += 1
  }
  clearPendingPluginError()
  stopActivePlayback()
  try {
    await stopPlayback({ silent: true })
  } catch {
    // Ignore stop errors during teardown.
  }
  if (activeDeviceId) {
    try {
      sdk?.I_Logout(activeDeviceId)
    } catch {
      // Ignore logout errors during teardown.
    }
    activeDeviceId = null
  }
  try {
    await sdk?.I_StopAll?.()
  } catch {
    // Ignore stop-all errors during teardown.
  }
  try {
    sdk?.I_DestroyWorker?.()
  } catch {
    // Ignore worker teardown errors.
  }
  sdk = null
  sdkInitialized = false
  playerState.value = "idle"
  clearProxyCookies()
}

const destroyPlayer = async () => {
  await teardownPlayer()
}

defineExpose({
  searchRecords,
  startPlayback,
  stopPlayback,
  destroyPlayer,
  setPlaybackLoading,
  clearPlaybackLoading,
  pausePlayback,
  resumePlayback,
  playFast,
  playSlow,
  getOSDTime,
  captureCurrentFrame,
  downloadRecord,
  downloadRecordByTime,
})

watch(
  () => props.config?.channelId,
  () => {
    playerMessage.value = ""
  },
)

watch(
  () => containerRef.value,
  (element) => {
    if (element) {
      element.id = containerId
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  clearSeekReleaseTimer()
  stopTimelinePan()
  void destroyPlayer()
})
</script>

<template>
  <section class="hik-webcontrol-playback-player">
    <div class="hik-webcontrol-playback-player__screen">
      <div
        ref="containerRef"
        class="hik-webcontrol-playback-player__surface"
        :class="{ 'hik-webcontrol-playback-player__surface--ready': playerState === 'ready' }"
      />
      <button
        type="button"
        class="hik-webcontrol-playback-player__fullscreen-hitbox"
        aria-label="双击全屏"
        @dblclick.stop.prevent="emit('toggleFullscreen')"
      />
      <div v-if="visibleMessage" class="hik-webcontrol-playback-player__notice">
        {{ visibleMessage }}
      </div>
      <div v-if="showSeekBar" class="hik-webcontrol-playback-player__seekbar hik-webcontrol-playback-player__seekbar--overlay">
        <div class="hik-webcontrol-playback-player__range-wrap" :class="{ 'hik-webcontrol-playback-player__range-wrap--pannable': timelineIsZoomed }" @mousemove="handleTimelineHover" @mouseleave="handleLeavePreview" @wheel="handleTimelineWheel" @mousedown.middle.prevent="handleTimelinePanStart">
          <div
            v-if="previewVisible"
            class="hik-webcontrol-playback-player__preview"
            :style="{ left: previewLeft }"
          >
            {{ previewLabel }}
          </div>
          <div class="hik-webcontrol-playback-player__spans" aria-hidden="true">
            <span
              v-for="(span, index) in recordedSpanStyles"
              :key="`${span.left}-${span.width}-${index}`"
              class="hik-webcontrol-playback-player__span"
              :class="{ 'hik-webcontrol-playback-player__span--active': span.active }"
              :style="{ left: span.left, width: span.width }"
            />
          </div>
          <input
            class="hik-webcontrol-playback-player__range"
            type="range"
            :min="visibleTimelineStart"
            :max="visibleTimelineEnd"
            step="1"
            :value="seekSliderValue"
            :disabled="seekBarMax <= 0"
            @input="handlePreviewUpdate"
            @change="handleSeekCommit"
          />
          <div class="hik-webcontrol-playback-player__track" />
          <div class="hik-webcontrol-playback-player__ticks" aria-hidden="true">
            <span
              v-for="(tick, index) in seekTicks"
              :key="`${tick.value}-${index}`"
              class="hik-webcontrol-playback-player__tick"
              :class="{
                'hik-webcontrol-playback-player__tick--first': index === 0,
                'hik-webcontrol-playback-player__tick--middle': index === Math.floor(seekTicks.length / 2),
                'hik-webcontrol-playback-player__tick--last': index === seekTicks.length - 1,
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
    <div v-if="showSeekBar" class="hik-webcontrol-playback-player__seekbar hik-webcontrol-playback-player__seekbar--dock">
      <div class="hik-webcontrol-playback-player__range-wrap" :class="{ 'hik-webcontrol-playback-player__range-wrap--pannable': timelineIsZoomed }" @mousemove="handleTimelineHover" @mouseleave="handleLeavePreview" @wheel="handleTimelineWheel" @mousedown.middle.prevent="handleTimelinePanStart">
        <div
          v-if="previewVisible"
          class="hik-webcontrol-playback-player__preview"
          :style="{ left: previewLeft }"
        >
          {{ previewLabel }}
        </div>
        <div class="hik-webcontrol-playback-player__spans" aria-hidden="true">
          <span
            v-for="(span, index) in recordedSpanStyles"
            :key="`${span.left}-${span.width}-${index}`"
            class="hik-webcontrol-playback-player__span"
            :class="{ 'hik-webcontrol-playback-player__span--active': span.active }"
            :style="{ left: span.left, width: span.width }"
          />
        </div>
        <input
          class="hik-webcontrol-playback-player__range"
          type="range"
          :min="visibleTimelineStart"
          :max="visibleTimelineEnd"
          step="1"
          :value="seekSliderValue"
          :disabled="seekBarMax <= 0"
          @input="handlePreviewUpdate"
          @change="handleSeekCommit"
        />
        <div class="hik-webcontrol-playback-player__track" />
        <div class="hik-webcontrol-playback-player__ticks" aria-hidden="true">
          <span
            v-for="(tick, index) in seekTicks"
            :key="`${tick.value}-${index}`"
            class="hik-webcontrol-playback-player__tick"
            :class="{
              'hik-webcontrol-playback-player__tick--first': index === 0,
              'hik-webcontrol-playback-player__tick--middle': index === Math.floor(seekTicks.length / 2),
              'hik-webcontrol-playback-player__tick--last': index === seekTicks.length - 1,
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
.hik-webcontrol-playback-player {
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

.hik-webcontrol-playback-player__screen {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 28px;
  background:
    radial-gradient(circle at top, rgba(51, 123, 197, 0.22), transparent 42%),
    linear-gradient(180deg, #102640 0%, #091727 100%);
}

.hik-webcontrol-playback-player__surface {
  position: absolute;
  inset: 28px;
  border-radius: 10px;
  background: #000000;
}

.hik-webcontrol-playback-player__surface--ready {
  background: #000;
}

.hik-webcontrol-playback-player__fullscreen-hitbox {
  position: absolute;
  z-index: 2;
  inset: 28px;
  padding: 0;
  border: 0;
  border-radius: 10px;
  background: transparent;
  cursor: pointer;
}

.hik-webcontrol-playback-player__notice {
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
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.24);
  pointer-events: none;
}

.hik-webcontrol-playback-player__seekbar {
  display: flex;
  align-items: center;
  gap: 0;
  padding: 12px 10px 26px;
  background: rgba(7, 19, 33, 0.92);
  border-top: 1px solid rgba(84, 129, 176, 0.18);
}

.hik-webcontrol-playback-player__seekbar--overlay {
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

.hik-webcontrol-playback-player__range-wrap {
  position: relative;
  flex: 1;
  width: 100%;
  min-width: 0;
  padding-bottom: 24px;
}

.hik-webcontrol-playback-player__range-wrap--pannable {
  cursor: grab;
}

.hik-webcontrol-playback-player__range-wrap--pannable:active {
  cursor: grabbing;
}

.hik-webcontrol-playback-player__range {
  appearance: none;
  -webkit-appearance: none;
  position: absolute;
  inset: 0 0 auto 0;
  width: 100%;
  height: 18px;
  margin: 0;
  background: transparent;
  cursor: pointer;
  z-index: 2;
}

.hik-webcontrol-playback-player__range::-webkit-slider-runnable-track {
  height: 4px;
  border-radius: 999px;
  background: transparent;
}

.hik-webcontrol-playback-player__range::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 16px;
  height: 16px;
  margin-top: -6px;
  border: 2px solid #cfe7ff;
  border-radius: 999px;
  background: #3b93e8;
  box-shadow: 0 0 0 3px rgba(59, 147, 232, 0.2);
}

.hik-webcontrol-playback-player__range::-moz-range-track {
  height: 4px;
  border-radius: 999px;
  background: transparent;
}

.hik-webcontrol-playback-player__range::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border: 2px solid #cfe7ff;
  border-radius: 999px;
  background: #3b93e8;
  box-shadow: 0 0 0 3px rgba(59, 147, 232, 0.2);
}

.hik-webcontrol-playback-player__spans {
  position: absolute;
  left: 0;
  right: 0;
  top: 7px;
  height: 10px;
  pointer-events: none;
}

.hik-webcontrol-playback-player__span {
  position: absolute;
  height: 10px;
  border-radius: 999px;
  background: rgba(68, 160, 255, 0.38);
  box-shadow:
    0 0 0 1px rgba(126, 197, 255, 0.12) inset,
    0 4px 10px rgba(7, 20, 39, 0.18);
}

.hik-webcontrol-playback-player__span--active {
  background: linear-gradient(180deg, rgba(73, 178, 255, 0.94), rgba(32, 122, 255, 0.94));
  box-shadow:
    0 0 0 1px rgba(170, 223, 255, 0.28) inset,
    0 8px 16px rgba(6, 18, 31, 0.26);
}

.hik-webcontrol-playback-player__preview {
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

.hik-webcontrol-playback-player__track {
  height: 4px;
  border-radius: 999px;
  background: rgba(214, 230, 246, 0.55);
}

.hik-webcontrol-playback-player__ticks {
  position: absolute;
  left: 0;
  right: 0;
  top: 10px;
  height: 24px;
  pointer-events: none;
}

.hik-webcontrol-playback-player__tick {
  position: absolute;
  display: flex;
  flex-direction: column;
  gap: 5px;
  align-items: center;
  transform: translateX(-50%);
  color: #cfe4f8;
  font-size: 10px;
  line-height: 1;
  white-space: nowrap;
}

.hik-webcontrol-playback-player__tick i {
  width: 1px;
  height: 8px;
  background: rgba(199, 220, 244, 0.72);
}

.hik-webcontrol-playback-player__tick small {
  color: rgba(223, 236, 249, 0.72);
}

.hik-webcontrol-playback-player__tick--first {
  left: 0;
  transform: none;
  align-items: flex-start;
}

.hik-webcontrol-playback-player__tick--first small {
  transform: translateX(0);
}

.hik-webcontrol-playback-player__tick--middle {
  left: 50%;
}

.hik-webcontrol-playback-player__tick--last {
  left: 100%;
  transform: translateX(-100%);
  align-items: flex-end;
}

.hik-webcontrol-playback-player__tick--last small {
  transform: translateX(0);
}

.hik-webcontrol-playback-player:fullscreen .hik-webcontrol-playback-player__screen,
.hik-webcontrol-playback-player--fullscreen .hik-webcontrol-playback-player__screen {
  padding: 0;
}

.hik-webcontrol-playback-player:fullscreen .hik-webcontrol-playback-player__surface,
.hik-webcontrol-playback-player--fullscreen .hik-webcontrol-playback-player__surface {
  inset: 0;
  border-radius: 0;
}

.hik-webcontrol-playback-player:fullscreen .hik-webcontrol-playback-player__fullscreen-hitbox,
.hik-webcontrol-playback-player--fullscreen .hik-webcontrol-playback-player__fullscreen-hitbox {
  inset: 0;
  border-radius: 0;
}

.hik-webcontrol-playback-player:fullscreen .hik-webcontrol-playback-player__seekbar--overlay,
.hik-webcontrol-playback-player--fullscreen .hik-webcontrol-playback-player__seekbar--overlay {
  display: flex;
  left: 32px;
  right: 32px;
  bottom: 28px;
}

.hik-webcontrol-playback-player:fullscreen .hik-webcontrol-playback-player__seekbar--dock,
.hik-webcontrol-playback-player--fullscreen .hik-webcontrol-playback-player__seekbar--dock {
  display: none;
}
</style>
