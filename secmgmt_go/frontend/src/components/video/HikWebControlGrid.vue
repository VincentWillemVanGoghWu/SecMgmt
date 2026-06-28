<script setup lang="ts">
import { loadWebVideoCtrl } from "hikvideoctrl"
import { computed, onBeforeUnmount, ref, useTemplateRef, watch } from "vue"

import type { LiveWebControlConfig } from "../../types/video"
import {
  clearHikProxyDeviceTargets,
  ensureHikProxyRoutingInstalled,
  syncHikProxyDeviceTargets,
} from "./hikProxyRouting"

type HikSdkXmlResponse = { responseXML?: Document | null; responseText?: string }
type HikSdkXml = Document | string | HikSdkXmlResponse | null | undefined

interface HikDevicePort {
  iRtspPort?: number
  iWebSocketPort?: number
  iWebSocketsPort?: number
}

interface HikChannelInfo {
  id: number
  name: string
  online: boolean
  enabled: boolean
  zeroChannel: boolean
}

interface XmlLikeNode {
  querySelector: (selector: string) => Element | null
}

interface HikWebVideoCtrl {
  I_SupportNoPlugin?: () => boolean
  I_InitPlugin: (width: string, height: string, options: Record<string, unknown>) => void
  I_InsertOBJECTPlugin: (containerId: string) => number
  I_ChangeWndNum?: (windowType: number) => Promise<unknown> | void
  I_Login: (
    ip: string,
    protocol: 1 | 2,
    port: number,
    username: string,
    password: string,
    options: Record<string, unknown>,
  ) => number
  I_Logout: (deviceId: string) => number
  I_GetAnalogChannelInfo: (deviceId: string, options: Record<string, unknown>) => void
  I_GetDigitalChannelInfo: (deviceId: string, options: Record<string, unknown>) => void
  I_GetZeroChannelInfo: (deviceId: string, options: Record<string, unknown>) => void
  I_GetDevicePort: (deviceId: string) => HikDevicePort | null
  I_StartRealPlay: (deviceId: string, options: Record<string, unknown>) => number | void
  I_GetOSDTime?: (options: Record<string, unknown>) => string | number | void
  I2_CapturePic?: (
    fileName: string,
    options?: {
      iWndIndex?: number
      cbCallback?: (data: Uint8Array) => void
    },
  ) => Promise<unknown> | number | void
  I_Resize?: (width: number | string, height: number | string) => Promise<unknown> | void
  I_Stop: (options: Record<string, unknown>) => void
  I_StopAll?: () => Promise<unknown> | void
  I_DestroyWorker?: () => void
  w_options?: { proxyAddress?: { ip: string; port: string | number } | null }
}

interface HikGridSlot {
  key: string
  title: string
  config?: LiveWebControlConfig | null
  isPlaying: boolean
  message?: string
}

interface WindowState {
  status: "idle" | "loading" | "ready"
  message: string
}

interface PreviewAttempt {
  stepLabel: string
  rtspPort?: number
  webSocketPort?: number
  streamType: number
  channelId: number
  zeroChannel: boolean
  useProxy: boolean
}

const props = withDefaults(
  defineProps<{
    layoutMode: 1 | 4 | 9
    activeSlotIndex?: number
    slots?: HikGridSlot[]
  }>(),
  {
    activeSlotIndex: 0,
    slots: () => [],
  },
)

const emit = defineEmits<{
  select: [index: number]
}>()

const containerRef = useTemplateRef<HTMLDivElement>("containerRef")
const containerId = `hik-webcontrol-grid-${Math.random().toString(36).slice(2, 10)}`
const sdkScriptUrl = import.meta.env.VITE_HIK_WEBCTRL_SCRIPT_URL ?? "/codebase/webVideoCtrl.js?v=20260530-loginfix7"
const sdkDependencyUrls = [
  "/codebase/jsPlugin/jquery.min.js",
  "/codebase/encryption/AES.js",
  "/codebase/encryption/cryptico.min.js",
  "/codebase/encryption/crypto-3.1.2.min.js",
]
const LOAD_TIMEOUT_MS = 15000
const LOGIN_TIMEOUT_MS = 15000
const CHANNEL_TIMEOUT_MS = 10000
const PREVIEW_TIMEOUT_MS = 15000
const MULTI_WINDOW_PREVIEW_TIMEOUT_MS = 5000
const RETRY_DELAY_MS = 200
const PROXY_COOKIE_MAX_AGE_SECONDS = 300

const waitForNextPaint = () =>
  new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => window.requestAnimationFrame(() => resolve()))
  })

const syncPluginViewport = async (sdkRef: HikWebVideoCtrl) => {
  const container = containerRef.value
  if (!container) {
    return
  }
  const width = Math.max(0, Math.floor(container.clientWidth))
  const height = Math.max(0, Math.floor(container.clientHeight))
  if (!width || !height) {
    return
  }
  try {
    await sdkRef.I_Resize?.(width, height)
  } catch {
    // Ignore resize sync failures. Playback can still recover on the next cycle.
  }
}

const syncPluginWindowLayout = async (sdkRef: HikWebVideoCtrl) => {
  try {
    await sdkRef.I_ChangeWndNum?.(sdkWindowType.value)
  } catch {
    // Ignore layout sync failures. The SDK may still apply the window count lazily.
  }
}

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

const windowStates = ref<WindowState[]>([])
const fullscreenWindowIndex = ref<number | null>(null)

let sdk: HikWebVideoCtrl | null = null
let sdkInitialized = false
let startToken = 0
let pendingRejectors = new Map<number, (error: Error) => void>()
let startingWindowIndexes = new Set<number>()
let loggedDeviceIds = new Set<string>()
let startGridQueued = false
let startGridRunning = false
let startGridTimer: number | null = null

const displayWindowCount = computed(() => props.layoutMode)
const sdkWindowType = computed<1 | 2 | 3>(() => {
  if (props.layoutMode === 1) {
    return 1
  }
  if (props.layoutMode === 4) {
    return 2
  }
  return 3
})
const layoutClass = computed(() => `hik-webcontrol-grid--${props.layoutMode}`)
const displayCells = computed(() =>
  Array.from({ length: displayWindowCount.value }, (_, index) => {
    const slot = props.slots[index]
    const state = windowStates.value[index]
    const message =
      slot?.message?.trim()
      || state?.message?.trim()
      || (!slot ? "空闲窗口" : slot.isPlaying ? (state?.status === "loading" ? "海康客户端加载中..." : "") : "未开始预览")
    return {
      index,
      slot,
      state,
      message,
      active: index === props.activeSlotIndex,
      fullscreen: fullscreenWindowIndex.value === index,
      hidden: fullscreenWindowIndex.value !== null && fullscreenWindowIndex.value !== index,
    }
  }),
)

const ensureWindowStates = () => {
  windowStates.value = Array.from({ length: displayWindowCount.value }, (_, index) => {
    const existing = windowStates.value[index]
    return existing ?? { status: "idle", message: "" }
  })
}

const updateWindowState = (index: number, patch: Partial<WindowState>) => {
  ensureWindowStates()
  windowStates.value[index] = {
    ...windowStates.value[index],
    ...patch,
  }
}

const handleSelectWindow = (windowIndex: number) => {
  if (windowIndex < 0 || windowIndex >= displayWindowCount.value) {
    return
  }
  emit("select", windowIndex)
}

const handleToggleWindowFullscreen = (windowIndex: number, fullscreen: boolean) => {
  fullscreenWindowIndex.value = fullscreen ? windowIndex : null
  emit("select", windowIndex)
}

const isSdkXmlResponse = (payload: HikSdkXml): payload is HikSdkXmlResponse =>
  typeof payload === "object" && payload !== null && !(typeof Document !== "undefined" && payload instanceof Document)

const parseXmlDocument = (payload: HikSdkXml): Document | null => {
  if (!payload) {
    return null
  }
  if (typeof Document !== "undefined" && payload instanceof Document) {
    return payload
  }
  if (typeof payload === "string") {
    return new DOMParser().parseFromString(payload, "application/xml")
  }
  if (isSdkXmlResponse(payload) && payload.responseXML) {
    return payload.responseXML
  }
  if (isSdkXmlResponse(payload) && payload.responseText) {
    return new DOMParser().parseFromString(payload.responseText, "application/xml")
  }
  return null
}

const getXmlText = (xml: Document | null, selector: string, fallback = "") =>
  xml?.querySelector(selector)?.textContent?.trim() ?? fallback

const getNodeText = (node: XmlLikeNode, selector: string, fallback = "") =>
  node.querySelector(selector)?.textContent?.trim() ?? fallback

const resolveHttpStatusMessage = (payload: HikSdkXml, status?: number) => {
  const xml = parseXmlDocument(payload)
  const statusString = getXmlText(xml, "statusString")
  const subStatusCode = getXmlText(xml, "subStatusCode")
  if (status === 401 || subStatusCode.toLowerCase().includes("unauthorized")) {
    return "海康设备登录失败，请检查设备账号密码。"
  }
  if (status === 403) {
    return "设备不支持当前海康无插件预览方式，或代理转发被拒绝。"
  }
  if (statusString) {
    return `海康设备返回 ${statusString}${subStatusCode ? ` / ${subStatusCode}` : ""}。`
  }
  return status ? `海康设备请求失败，状态码 ${status}。` : "海康设备请求失败。"
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

const normalizePositivePort = (port?: number | null) => (typeof port === "number" && Number.isFinite(port) && port > 0 ? port : undefined)
const resolveDeviceId = (config: LiveWebControlConfig) => `${config.host}_${config.port}`

const withTimeout = async <T,>(promise: Promise<T>, timeoutMs: number, stepLabel: string): Promise<T> =>
  await Promise.race([
    promise,
    new Promise<T>((_, reject) => {
      window.setTimeout(() => reject(new Error(`${stepLabel}超时`)), timeoutMs)
    }),
  ])

const sleep = async (delayMs: number) =>
  await new Promise<void>((resolve) => {
    window.setTimeout(resolve, delayMs)
  })

const loadScriptOnce = async (scriptUrl: string) => {
  const absoluteUrl = new URL(scriptUrl, window.location.origin).toString()
  const existingScript = document.querySelector<HTMLScriptElement>(`script[data-hik-sdk-url="${absoluteUrl}"]`)
  if (existingScript?.dataset.loaded === "true") {
    return
  }
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
  ensureHikProxyRoutingInstalled()
  for (const dependencyUrl of sdkDependencyUrls) {
    await loadScriptOnce(dependencyUrl)
  }
  const loadedSdk = await loadWebVideoCtrl(sdkScriptUrl)
  sdk = loadedSdk as unknown as HikWebVideoCtrl
  return sdk
}

const failWindowRejector = (windowIndex: number, message: string) => {
  const rejector = pendingRejectors.get(windowIndex)
  pendingRejectors.delete(windowIndex)
  if (rejector) {
    rejector(new Error(message))
  }
}

const normalizePreviewStartError = (error: unknown) => {
  const raw = error instanceof Error ? error.message : String(error ?? "")
  const message = raw
    .replace(/^.*?\([^)]*\)\s*失败[：:，]\s*/u, "")
    .replace(/^.*?\([^)]*\)\s*同步返回 -1\s*$/u, "海康实时预览启动失败。")
    .replace(/^.*?\([^)]*\)\s*超时\s*$/u, "海康实时预览启动超时。")
    .trim()
  return message || "海康实时预览启动失败。"
}

const handleWindowPreviewError = (windowIndex: number, message: string) => {
  if (pendingRejectors.has(windowIndex) || startingWindowIndexes.has(windowIndex)) {
    failWindowRejector(windowIndex, message)
    return
  }
  updateWindowState(windowIndex, { status: "idle", message })
  failWindowRejector(windowIndex, message)
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
        success: (payload?: T) => resolve(payload),
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

const listSdkChannels = async (deviceId: string) => {
  const channels: HikChannelInfo[] = []
  const mergeUniqueChannels = (items: HikChannelInfo[]) => {
    items.forEach((item) => {
      if (!channels.some((existing) => existing.id === item.id && existing.zeroChannel === item.zeroChannel)) {
        channels.push(item)
      }
    })
  }

  const loadAnalog = async () => {
    const payload = await wrapSdkCallback<HikSdkXml>("读取模拟通道", CHANNEL_TIMEOUT_MS, (callbacks) =>
      sdk?.I_GetAnalogChannelInfo(deviceId, {
        async: false,
        success: callbacks.success,
        error: callbacks.error,
      }),
    )
    const xml = parseXmlDocument(payload)
    if (!xml) {
      return
    }
    mergeUniqueChannels(
      Array.from(xml.querySelectorAll("VideoInputChannel")).map((node, index) => ({
        id: Number(getNodeText(node, "id", String(index + 1))),
        name: getNodeText(node, "name", `Camera ${index + 1}`),
        online: true,
        enabled: getNodeText(node, "videoInputEnabled", "true") !== "false",
        zeroChannel: false,
      })),
    )
  }

  const loadDigital = async () => {
    const payload = await wrapSdkCallback<HikSdkXml>("读取数字通道", CHANNEL_TIMEOUT_MS, (callbacks) =>
      sdk?.I_GetDigitalChannelInfo(deviceId, {
        async: false,
        success: callbacks.success,
        error: callbacks.error,
      }),
    )
    const xml = parseXmlDocument(payload)
    if (!xml) {
      return
    }
    mergeUniqueChannels(
      Array.from(xml.querySelectorAll("InputProxyChannelStatus")).map((node, index) => ({
        id: Number(node.querySelector("id")?.textContent?.trim() ?? String(index + 1)),
        name: node.querySelector("name")?.textContent?.trim() || `IPCamera ${index + 1}`,
        online: (node.querySelector("online")?.textContent?.trim() ?? "false") === "true",
        enabled: true,
        zeroChannel: false,
      })),
    )
  }

  const loadZero = async () => {
    const payload = await wrapSdkCallback<HikSdkXml>("读取零通道", CHANNEL_TIMEOUT_MS, (callbacks) =>
      sdk?.I_GetZeroChannelInfo(deviceId, {
        async: false,
        success: callbacks.success,
        error: callbacks.error,
      }),
    )
    const xml = parseXmlDocument(payload)
    if (!xml) {
      return
    }
    mergeUniqueChannels(
      Array.from(xml.querySelectorAll("ZeroVideoChannel"))
        .filter((node) => (node.querySelector("enabled")?.textContent?.trim() ?? "false") === "true")
        .map((node, index) => ({
          id: Number(node.querySelector("id")?.textContent?.trim() ?? String(index + 1)),
          name: node.querySelector("name")?.textContent?.trim() || `Zero Channel ${index + 1}`,
          online: true,
          enabled: true,
          zeroChannel: true,
        })),
    )
  }

  await Promise.allSettled([loadAnalog(), loadDigital(), loadZero()])
  return channels
}

const resolvePreviewChannel = (config: LiveWebControlConfig, channels: HikChannelInfo[]) => {
  if (!channels.length) {
    return { channelId: config.channelNo, zeroChannel: false }
  }
  const exactMatch = channels.find((item) => item.id === config.channelNo && item.enabled && item.online)
  if (exactMatch) {
    return { channelId: exactMatch.id, zeroChannel: exactMatch.zeroChannel }
  }
  if (config.sourceType === "camera") {
    const firstOnline = channels.find((item) => item.enabled && item.online)
    if (firstOnline) {
      return { channelId: firstOnline.id, zeroChannel: firstOnline.zeroChannel }
    }
  }
  throw new Error(`海康 SDK 通道列表中未找到可播放通道 ${config.channelNo}。`)
}

const stopWindowPreview = (windowIndex: number) => {
  try {
    sdk?.I_Stop?.({ iWndIndex: windowIndex })
  } catch {
    // Ignore stop errors during restart.
  }
}

const destroyGrid = async () => {
  pendingRejectors.clear()
  startingWindowIndexes.clear()
  clearProxyCookies()
  try {
    await sdk?.I_StopAll?.()
  } catch {
    // Ignore stop-all errors during teardown.
  }
  loggedDeviceIds.forEach((deviceId) => {
    try {
      sdk?.I_Logout(deviceId)
    } catch {
      // Ignore logout errors during teardown.
    }
  })
  loggedDeviceIds.clear()
  try {
    sdk?.I_DestroyWorker?.()
  } catch {
    // Ignore worker teardown errors.
  }
  sdk = null
  sdkInitialized = false
  windowStates.value = []
}

const ensureSdkInitialized = async () => {
  const sdkInstance = await getSdk()
  if (!sdkInstance.I_SupportNoPlugin?.()) {
    throw new Error("当前浏览器不支持海康无插件播放器，请使用 Chromium 内核浏览器。")
  }
  if (sdkInitialized) {
    sdkInstance.w_options = sdkInstance.w_options ?? {}
    sdkInstance.w_options.proxyAddress = null
    return sdkInstance
  }

  await withTimeout(
    new Promise<void>((resolve, reject) => {
      const options = {
        iWndowType: sdkWindowType.value,
        bNoPlugin: true,
        bWndFull: true,
        iPlayMode: 2,
        iPackageType: 2,
        cbSelWnd: (xmlDoc: Document | null) => {
          const selectedIndex = Number(xmlDoc?.querySelector("SelectWnd")?.textContent?.trim() ?? "")
          if (Number.isInteger(selectedIndex)) {
            handleSelectWindow(selectedIndex)
          }
        },
        cbDoubleClickWnd: (windowIndex: number, fullscreen: boolean) => {
          handleToggleWindowFullscreen(windowIndex, fullscreen)
        },
        cbEvent: (eventType: number, param1: number, param2: number) => {
          const windowIndex = Number.isFinite(param1) ? Number(param1) : 0
          const message = pluginErrorMessages[param2] ?? (eventType === 0 ? "海康码流播放异常，请检查设备状态。" : "")
          if (!message) {
            return
          }
          handleWindowPreviewError(windowIndex, message)
        },
        cbPluginErrorHandler: (windowIndex: number, errorCode: number) => {
          const message = pluginErrorMessages[errorCode] ?? ("海康播放器错误，代码 " + errorCode + "。")
          handleWindowPreviewError(windowIndex, message)
        },
        cbPerformanceLack: () => {
          windowStates.value = windowStates.value.map((item) => ({
            ...item,
            message: item.message || "浏览器性能不足，海康客户端预览无法稳定播放。",
          }))
        },
        cbSecretKeyError: (windowIndex: number) => {
          const message = "码流加密密钥错误，当前通道无法播放。"
          handleWindowPreviewError(windowIndex, message)
        },
        cbInitPluginComplete: () => {
          try {
            const result = sdk?.I_InsertOBJECTPlugin(containerId) ?? -1
            if (result !== 0) {
              reject(new Error("插入海康播放器失败，返回值 " + result))
              return
            }
            sdkInitialized = true
            void (async () => {
              await waitForNextPaint()
              if (sdk) {
                await syncPluginWindowLayout(sdk)
                await syncPluginViewport(sdk)
              }
              resolve()
            })()
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
    "初始化海康多窗口播放器",
  )
  return sdkInstance
}

const ensureDeviceLoggedIn = async (sdkRef: HikWebVideoCtrl, config: LiveWebControlConfig) => {
  const deviceId = resolveDeviceId(config)
  syncHttpProxyCookie(config)
  if (loggedDeviceIds.has(deviceId)) {
    return deviceId
  }
  const protocolValue = config.protocol === "https" ? 2 : 1
  await wrapSdkCallback(
    "登录海康设备 " + config.host,
    LOGIN_TIMEOUT_MS,
    (callbacks) =>
      sdkRef.I_Login(
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
  loggedDeviceIds.add(deviceId)
  return deviceId
}

const tryStartRealPlay = async (
  sdkRef: HikWebVideoCtrl,
  deviceId: string,
  windowIndex: number,
  options: PreviewAttempt,
) => {
  const previewTimeoutMs =
    !options.useProxy && options.webSocketPort ? MULTI_WINDOW_PREVIEW_TIMEOUT_MS : PREVIEW_TIMEOUT_MS
  try {
    await withTimeout(
      new Promise<void>((resolve, reject) => {
        pendingRejectors.set(windowIndex, reject)
        const ret = sdkRef.I_StartRealPlay(deviceId, {
          iWndIndex: windowIndex,
          iRtspPort: options.rtspPort,
          iStreamType: options.streamType,
          iChannelID: options.channelId,
          bZeroChannel: options.zeroChannel,
          ...(options.webSocketPort ? { iWSPort: options.webSocketPort } : {}),
          bProxy: options.useProxy,
          success: () => {
            pendingRejectors.delete(windowIndex)
            resolve()
          },
          error: (status?: number, payload?: HikSdkXml) => {
            pendingRejectors.delete(windowIndex)
            reject(new Error(options.stepLabel + "失败：" + resolveHttpStatusMessage(payload, status)))
          },
        })
        if (ret === -1) {
          pendingRejectors.delete(windowIndex)
          reject(new Error(options.stepLabel + "同步返回 -1"))
        }
      }),
      previewTimeoutMs,
      options.stepLabel,
    )
  } catch (error) {
    throw error
  } finally {
    pendingRejectors.delete(windowIndex)
  }
}

const startWindowPreview = async (sdkRef: HikWebVideoCtrl, slot: HikGridSlot, windowIndex: number) => {
  const config = slot.config
  if (!config || !slot.isPlaying) {
    updateWindowState(windowIndex, { status: "idle", message: slot.message?.trim() || "" })
    stopWindowPreview(windowIndex)
    return
  }

  updateWindowState(windowIndex, { status: "loading", message: "" })
  startingWindowIndexes.add(windowIndex)
  try {
    const deviceId = await ensureDeviceLoggedIn(sdkRef, config)
    const channels = await listSdkChannels(deviceId)
    const previewTarget = resolvePreviewChannel(config, channels)
    const ports = sdkRef.I_GetDevicePort(deviceId)
    const rtspPort = normalizePositivePort(config.rtspPort) ?? normalizePositivePort(ports?.iRtspPort) ?? undefined
    const webSocketPort =
      normalizePositivePort(config.webSocketPort)
      ?? normalizePositivePort(config.protocol === "https" ? ports?.iWebSocketsPort : ports?.iWebSocketPort)

    const proxyPreferred = config.useProxy || config.protocol === "https"
    const createAttempt = (
      stepLabel: string,
      useProxy: boolean,
      explicitWebSocketPort?: number,
    ): PreviewAttempt => ({
      stepLabel,
      rtspPort,
      ...(explicitWebSocketPort ? { webSocketPort: explicitWebSocketPort } : {}),
      streamType: config.streamType,
      channelId: previewTarget.channelId,
      zeroChannel: previewTarget.zeroChannel || config.zeroChannel,
      useProxy,
    })
    const directAttempts: PreviewAttempt[] = [
      ...(webSocketPort ? [createAttempt("启动海康实时预览(直连+显式WS端口)", false, webSocketPort)] : []),
      createAttempt("启动海康实时预览(直连+自动协商端口)", false),
    ]
    const proxyAttempts: PreviewAttempt[] = [
      ...(webSocketPort ? [createAttempt("启动海康实时预览(代理+显式WS端口)", true, webSocketPort)] : []),
      createAttempt("启动海康实时预览(代理+自动协商端口)", true),
    ]
    const attempts: PreviewAttempt[] = proxyPreferred
      ? [...proxyAttempts, ...directAttempts]
      : [...directAttempts, ...proxyAttempts]

    let lastError: unknown = null
    for (const attempt of attempts) {
      try {
        syncWebSocketProxyCookie(config, attempt.webSocketPort)
        await tryStartRealPlay(sdkRef, deviceId, windowIndex, attempt)
        updateWindowState(windowIndex, { status: "ready", message: "" })
        return
      } catch (error) {
        lastError = error
        stopWindowPreview(windowIndex)
        if (attempt.webSocketPort) {
          await sleep(RETRY_DELAY_MS)
        }
      }
    }

    const message = normalizePreviewStartError(lastError)
    updateWindowState(windowIndex, { status: "idle", message })
    throw new Error(message)
  } finally {
    startingWindowIndexes.delete(windowIndex)
  }
}

const executeStartGrid = async () => {
  const token = ++startToken
  ensureWindowStates()

  await destroyGrid()
  ensureWindowStates()

  const playableSlots = props.slots.slice(0, props.layoutMode).filter((slot) => slot.isPlaying && slot.config)
  if (!playableSlots.length || !containerRef.value) {
    clearHikProxyDeviceTargets()
    return
  }
  syncHikProxyDeviceTargets(
    playableSlots.map((slot) => slot.config).filter((config): config is LiveWebControlConfig => Boolean(config)),
  )

  try {
    const sdkRef = await ensureSdkInitialized()
    if (token !== startToken) {
      return
    }
    await waitForNextPaint()
    await syncPluginWindowLayout(sdkRef)
    await syncPluginViewport(sdkRef)
    await sleep(120)

    for (let windowIndex = 0; windowIndex < props.layoutMode; windowIndex += 1) {
      const slot = props.slots[windowIndex]
      if (!slot) {
        updateWindowState(windowIndex, { status: "idle", message: "空闲窗口" })
        stopWindowPreview(windowIndex)
        continue
      }
      try {
        await startWindowPreview(sdkRef, slot, windowIndex)
        await syncPluginViewport(sdkRef)
        if (props.layoutMode > 1) {
          await sleep(RETRY_DELAY_MS)
        }
      } catch {
        // Window message has already been updated.
      }
      if (token !== startToken) {
        return
      }
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : "海康多窗口播放器初始化失败。"
    windowStates.value = Array.from({ length: displayWindowCount.value }, (_, index) => ({
      status: "idle",
      message: index < props.layoutMode ? message : "空闲窗口",
    }))
  }
}

const flushStartGridQueue = async () => {
  if (startGridRunning) {
    return
  }
  startGridRunning = true
  try {
    while (startGridQueued) {
      startGridQueued = false
      await executeStartGrid()
    }
  } finally {
    startGridRunning = false
  }
}

const scheduleStartGrid = () => {
  startGridQueued = true
  if (startGridTimer !== null) {
    window.clearTimeout(startGridTimer)
  }
  startGridTimer = window.setTimeout(() => {
    startGridTimer = null
    void flushStartGridQueue()
  }, 80)
}

const getOSDTime = async (windowIndex = props.activeSlotIndex) => {
  if (!sdk?.I_GetOSDTime) {
    throw new Error("当前 HIK 预览播放器不支持获取 OSD 时间。")
  }
  const targetWindowIndex = Number.isInteger(windowIndex) && windowIndex >= 0 ? Number(windowIndex) : 0
  return await withTimeout(
    new Promise<string>((resolve, reject) => {
      const ret = sdk?.I_GetOSDTime?.({
        iWndIndex: targetWindowIndex,
        success: (value?: string) => {
          resolve(typeof value === "string" ? value.trim() : "")
        },
        error: (status?: number, payload?: HikSdkXml) => {
          reject(new Error(`获取 HIK OSD 时间失败：${resolveHttpStatusMessage(payload, status)}`))
        },
      })
      if (typeof ret === "string" && ret.trim()) {
        resolve(ret.trim())
        return
      }
      if (ret === -1) {
        reject(new Error("获取 HIK OSD 时间同步返回 -1"))
      }
    }),
    3000,
    "获取 HIK OSD 时间",
  )
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

const captureCurrentFrame = async (windowIndex = props.activeSlotIndex) => {
  if (!sdk?.I2_CapturePic) {
    throw new Error("当前 HIK 预览播放器不支持截图。")
  }
  const targetWindowIndex = Number.isInteger(windowIndex) && windowIndex >= 0 ? Number(windowIndex) : 0
  let capturedDataUrl: string | null = null
  const fileName = `hik-preview-${Date.now()}.jpg`
  const captureResult = sdk.I2_CapturePic(fileName, {
    iWndIndex: targetWindowIndex,
    cbCallback: (data) => {
      void uint8ArrayToDataUrl(data).then((result) => {
        capturedDataUrl = result
      })
    },
  })
  await resolvePromiseLike(captureResult, "HIK 预览截图")
  for (let attempt = 0; attempt < 20; attempt += 1) {
    if (capturedDataUrl) {
      return capturedDataUrl
    }
    await new Promise<void>((resolve) => {
      window.setTimeout(resolve, 50)
    })
  }
  throw new Error("HIK 预览截图数据未返回。")
}

const slotSignature = computed(() =>
  JSON.stringify({
    layoutMode: props.layoutMode,
    slots: props.slots.map((slot) => ({
      key: slot.key,
      isPlaying: slot.isPlaying,
      message: slot.message ?? "",
      config: slot.config
        ? {
            host: slot.config.host,
            port: slot.config.port,
            channelNo: slot.config.channelNo,
            streamType: slot.config.streamType,
            webSocketPort: slot.config.webSocketPort ?? null,
            rtspPort: slot.config.rtspPort ?? null,
            useProxy: slot.config.useProxy,
          }
        : null,
    })),
  }),
)

watch(
  slotSignature,
  () => {
    scheduleStartGrid()
  },
  { immediate: true },
)

watch(
  () => containerRef.value,
  (element) => {
    if (element) {
      element.id = containerId
      scheduleStartGrid()
    }
  },
)

onBeforeUnmount(() => {
  startToken += 1
  startGridQueued = false
  if (startGridTimer !== null) {
    window.clearTimeout(startGridTimer)
    startGridTimer = null
  }
  fullscreenWindowIndex.value = null
  void destroyGrid()
})

defineExpose({
  captureCurrentFrame,
  getOSDTime,
})
</script>

<template>
  <section class="hik-webcontrol-grid" :class="layoutClass">
    <div ref="containerRef" class="hik-webcontrol-grid__surface" />
    <div
      v-for="cell in displayCells"
      :key="cell.index"
      class="hik-webcontrol-grid__cell"
      :class="{
        'hik-webcontrol-grid__cell--active': cell.active,
        'hik-webcontrol-grid__cell--blank': !cell.slot,
        'hik-webcontrol-grid__cell--fullscreen': cell.fullscreen,
        'hik-webcontrol-grid__cell--hidden': cell.hidden,
      }"
    >
      <span v-if="cell.slot" class="hik-webcontrol-grid__title">{{ cell.slot.title }}</span>
      <span v-if="cell.message" class="hik-webcontrol-grid__notice">{{ cell.message }}</span>
    </div>
  </section>
</template>

<style scoped>
.hik-webcontrol-grid {
  position: relative;
  display: grid;
  gap: 18px;
  width: 100%;
  height: 100%;
  min-height: 0;
}

.hik-webcontrol-grid--1 {
  grid-template-columns: 1fr;
  grid-auto-rows: minmax(0, 1fr);
}

.hik-webcontrol-grid--4 {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-auto-rows: minmax(0, 1fr);
}

.hik-webcontrol-grid--9 {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  grid-auto-rows: minmax(0, 1fr);
}

.hik-webcontrol-grid__surface {
  position: absolute;
  inset: 0;
  background: #000000;
}

.hik-webcontrol-grid__cell {
  position: relative;
  z-index: 1;
  min-height: 0;
  padding: 12px;
  border: 1px solid rgba(87, 125, 163, 0.28);
  border-radius: 18px;
  background: transparent;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.03);
  pointer-events: none;
}

.hik-webcontrol-grid__cell--blank {
  cursor: default;
}

.hik-webcontrol-grid__cell--hidden {
  opacity: 0;
}

.hik-webcontrol-grid__cell--fullscreen {
  grid-column: 1 / -1;
  grid-row: 1 / -1;
}

.hik-webcontrol-grid__cell--active {
  border-color: rgba(92, 174, 255, 0.72);
  box-shadow: inset 0 1px 0 rgba(128, 192, 255, 0.12), 0 0 0 1px rgba(92, 174, 255, 0.2);
}

.hik-webcontrol-grid__title {
  position: absolute;
  right: 16px;
  top: 12px;
  max-width: calc(100% - 32px);
  overflow: hidden;
  color: rgba(235, 243, 255, 0.9);
  font-size: 12px;
  line-height: 1.4;
  white-space: nowrap;
  text-overflow: ellipsis;
  pointer-events: none;
  text-align: right;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.45);
}

.hik-webcontrol-grid__notice {
  position: absolute;
  left: 50%;
  top: 50%;
  max-width: min(82%, 260px);
  padding: 10px 14px;
  border-radius: 12px;
  background: rgba(11, 18, 32, 0.78);
  color: #e5edf7;
  font-size: 12px;
  line-height: 1.6;
  text-align: center;
  transform: translate(-50%, -50%);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.24);
  pointer-events: none;
}
</style>
