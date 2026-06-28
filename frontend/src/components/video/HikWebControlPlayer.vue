<script setup lang="ts">
import { loadWebVideoCtrl } from "hikvideoctrl"
import { computed, onBeforeUnmount, ref, useTemplateRef, watch } from "vue"

import type { LiveWebControlConfig } from "../../types/video"

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
  I_Stop: (options: Record<string, unknown>) => void
  I_StopAll?: () => Promise<unknown> | void
  I_DestroyWorker?: () => void
  w_options?: { proxyAddress?: { ip: string; port: string | number } | null }
}

const props = withDefaults(
  defineProps<{
    config?: LiveWebControlConfig | null
    isPlaying?: boolean
    visibleSlotCount?: number
    message?: string
  }>(),
  {
    config: null,
    isPlaying: false,
    visibleSlotCount: 1,
    message: "",
  },
)

const containerRef = useTemplateRef<HTMLDivElement>("containerRef")
const playerMessage = ref("")
const playerState = ref<"idle" | "loading" | "ready">("idle")
const sdkScriptUrl = import.meta.env.VITE_HIK_WEBCTRL_SCRIPT_URL ?? "/codebase/webVideoCtrl.js?v=20260530-loginfix7"
const sdkDependencyUrls = [
  "/codebase/jsPlugin/jquery.min.js",
  "/codebase/encryption/AES.js",
  "/codebase/encryption/cryptico.min.js",
  "/codebase/encryption/crypto-3.1.2.min.js",
]
const containerId = `hik-webcontrol-${Math.random().toString(36).slice(2, 10)}`
const LOAD_TIMEOUT_MS = 15000
const LOGIN_TIMEOUT_MS = 15000
const CHANNEL_TIMEOUT_MS = 10000
const PREVIEW_TIMEOUT_MS = 15000
const MULTIVIEW_PREVIEW_TIMEOUT_MS = 3000
const MULTIVIEW_DIRECT_EXPLICIT_TIMEOUT_MS = 5000
const MULTIVIEW_RETRY_DELAY_MS = 200

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

let sdk: HikWebVideoCtrl | null = null
let activeDeviceId: string | null = null
let sdkInitialized = false
let startToken = 0
let pendingPreviewReject: ((error: Error) => void) | null = null
let previewStarting = false

const PROXY_COOKIE_MAX_AGE_SECONDS = 300

const visibleMessage = computed(() => {
  if (!props.isPlaying && props.message) {
    return props.message
  }
  if (playerMessage.value) {
    return playerMessage.value
  }
  if (props.isPlaying && playerState.value === "loading") {
    return "海康客户端加载中..."
  }
  return ""
})
const isMultiViewMode = computed(() => props.visibleSlotCount > 1)

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

const resolvePlayerErrorMessage = (error: unknown) => {
  const text = error instanceof Error ? error.message : String(error ?? "")
  const normalized = text.toLowerCase()
  if (normalized.includes("failed to fetch") || normalized.includes("404")) {
    return `未找到海康 WebSDK 静态资源，请将官方 WebSDK 的 codebase 目录部署到 ${sdkScriptUrl.replace(/\/webVideoCtrl\.js$/, "")}。`
  }
  if (normalized.includes("loadwebvideoctrl")) {
    return `海康 WebSDK 加载失败，请确认 ${sdkScriptUrl} 可以被浏览器访问。`
  }
  return text || "海康客户端播放器初始化失败。"
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

const failPendingPreview = (message: string) => {
  const reject = pendingPreviewReject
  pendingPreviewReject = null
  if (reject) {
    reject(new Error(message))
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

const handlePreviewError = (message: string) => {
  if (pendingPreviewReject || previewStarting) {
    failPendingPreview(message)
    return
  }
  playerMessage.value = message
  playerState.value = "idle"
  failPendingPreview(message)
}

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

const sleep = async (delayMs: number) =>
  await new Promise<void>((resolve) => {
    window.setTimeout(resolve, delayMs)
  })

const getSdk = async () => {
  for (const dependencyUrl of sdkDependencyUrls) {
    await loadScriptOnce(dependencyUrl)
  }
  const loadedSdk = await loadWebVideoCtrl(sdkScriptUrl)
  sdk = loadedSdk as unknown as HikWebVideoCtrl
  return sdk
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
        iWndowType: 1,
        bNoPlugin: true,
        bWndFull: true,
        iPlayMode: 2,
        iPackageType: 2,
        cbEvent: (eventType: number, _param1: number, param2: number) => {
          const message = pluginErrorMessages[param2] ?? (eventType === 0 ? "海康码流播放异常，请检查代理和设备状态。" : playerMessage.value)
          if (eventType === 0 || pluginErrorMessages[param2]) {
            handlePreviewError(message)
          }
        },
        cbPluginErrorHandler: (_windowIndex: number, errorCode: number) => {
          const message = pluginErrorMessages[errorCode] ?? `海康播放器错误，代码 ${errorCode}。`
          handlePreviewError(message)
        },
        cbPerformanceLack: () => {
          playerMessage.value = "浏览器性能不足，海康客户端预览无法稳定播放。"
        },
        cbSecretKeyError: () => {
          const message = "码流加密密钥错误，当前通道无法播放。"
          handlePreviewError(message)
        },
        cbInitPluginComplete: () => {
          try {
            const result = sdk?.I_InsertOBJECTPlugin(containerId) ?? -1
            if (result !== 0) {
              reject(new Error(`插入海康播放器失败，返回值 ${result}`))
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
    "初始化海康播放器",
  )
  return sdkInstance
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
      Array.from(xml.querySelectorAll("InputProxyChannelStatus")).map((node, index) => {
        const id = Number(node.querySelector("id")?.textContent?.trim() ?? String(index + 1))
        const name = node.querySelector("name")?.textContent?.trim() || `IPCamera ${index + 1}`
        const online = (node.querySelector("online")?.textContent?.trim() ?? "false") === "true"
        return {
          id,
          name,
          online,
          enabled: true,
          zeroChannel: false,
        }
      }),
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

const stopActivePreview = () => {
  try {
    sdk?.I_Stop?.({ iWndIndex: 0 })
  } catch {
    // Ignore stop errors during re-init.
  }
}

const tryStartRealPlay = async (
  sdkRef: HikWebVideoCtrl,
  deviceId: string,
  options: {
    stepLabel: string
    rtspPort?: number
    webSocketPort?: number
    streamType: number
    channelId: number
    zeroChannel: boolean
    useProxy: boolean
  },
) => {
  let previewRejector: ((error: Error) => void) | null = null
  const previewTimeoutMs =
    isMultiViewMode.value && !options.useProxy && Boolean(options.webSocketPort)
      ? MULTIVIEW_DIRECT_EXPLICIT_TIMEOUT_MS
      : isMultiViewMode.value
        ? MULTIVIEW_PREVIEW_TIMEOUT_MS
        : PREVIEW_TIMEOUT_MS
  try {
    return await withTimeout(
      new Promise<void>((resolve, reject) => {
        previewRejector = (error: Error) => {
          if (pendingPreviewReject === previewRejector) {
            pendingPreviewReject = null
          }
          reject(error)
        }
        pendingPreviewReject = previewRejector
        const ret = sdkRef.I_StartRealPlay(deviceId, {
          iWndIndex: 0,
          iRtspPort: options.rtspPort,
          iStreamType: options.streamType,
          iChannelID: options.channelId,
          bZeroChannel: options.zeroChannel,
          ...(options.webSocketPort ? { iWSPort: options.webSocketPort } : {}),
          bProxy: options.useProxy,
          success: () => {
            if (pendingPreviewReject === previewRejector) {
              pendingPreviewReject = null
            }
            resolve()
          },
          error: (status?: number, payload?: HikSdkXml) => {
            if (pendingPreviewReject === previewRejector) {
              pendingPreviewReject = null
            }
            reject(new Error(`${options.stepLabel}失败：${resolveHttpStatusMessage(payload, status)}`))
          },
        })
        if (ret === -1) {
          if (pendingPreviewReject === previewRejector) {
            pendingPreviewReject = null
          }
          reject(new Error(`${options.stepLabel}同步返回 -1`))
        }
      }),
      previewTimeoutMs,
      options.stepLabel,
    )
  } catch (error) {
    throw error
  } finally {
    if (pendingPreviewReject === previewRejector) {
      pendingPreviewReject = null
    }
  }
}

const destroyPlayer = async () => {
  pendingPreviewReject = null
  previewStarting = false
  clearProxyCookies()
  stopActivePreview()
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
}

const startPlayer = async () => {
  const config = props.config
  const container = containerRef.value
  const token = ++startToken

  if (!props.isPlaying || !config || !container) {
    await destroyPlayer()
    return
  }

  playerState.value = "loading"
  playerMessage.value = ""

  try {
    const sdkRef = await ensureSdkInitialized()
    if (token !== startToken) {
      return
    }

    if (activeDeviceId) {
      try {
        sdkRef.I_Logout(activeDeviceId)
      } catch {
        // Ignore relogin cleanup errors.
      }
      activeDeviceId = null
    }

    const protocolValue = config.protocol === "https" ? 2 : 1
    syncHttpProxyCookie(config)
    await wrapSdkCallback(
      "登录海康设备",
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
    if (token !== startToken) {
      return
    }

    const deviceId = `${config.host}_${config.port}`
    activeDeviceId = deviceId
    const channels = await listSdkChannels(deviceId)
    const previewTarget = resolvePreviewChannel(config, channels)
    const ports = sdkRef.I_GetDevicePort(deviceId)
    const rtspPort = normalizePositivePort(config.rtspPort) ?? normalizePositivePort(ports?.iRtspPort) ?? undefined
    const webSocketPort =
      normalizePositivePort(config.webSocketPort)
      ?? normalizePositivePort(config.protocol === "https" ? ports?.iWebSocketsPort : ports?.iWebSocketPort)

    syncWebSocketProxyCookie(config, webSocketPort)
    const explicitOnlyInMultiView = isMultiViewMode.value && Boolean(webSocketPort)

    const proxyAttempts = [
      ...(webSocketPort ? [{
        stepLabel: "启动海康实时预览(代理+显式WS端口)",
        rtspPort,
        webSocketPort,
        streamType: config.streamType,
        channelId: previewTarget.channelId,
        zeroChannel: previewTarget.zeroChannel || config.zeroChannel,
        useProxy: true,
      }] : []),
      ...(!explicitOnlyInMultiView ? [{
        stepLabel: "启动海康实时预览(代理+自动协商端口)",
        rtspPort,
        streamType: config.streamType,
        channelId: previewTarget.channelId,
        zeroChannel: previewTarget.zeroChannel || config.zeroChannel,
        useProxy: true,
      }] : []),
    ] as const
    const directAttempts = [
      ...(webSocketPort ? [{
        stepLabel: "启动海康实时预览(直连+显式WS端口)",
        rtspPort,
        webSocketPort,
        streamType: config.streamType,
        channelId: previewTarget.channelId,
        zeroChannel: previewTarget.zeroChannel || config.zeroChannel,
        useProxy: false,
      }] : []),
      ...(explicitOnlyInMultiView && webSocketPort ? [{
        stepLabel: "启动海康实时预览(直连+显式WS端口重试)",
        rtspPort,
        webSocketPort,
        streamType: config.streamType,
        channelId: previewTarget.channelId,
        zeroChannel: previewTarget.zeroChannel || config.zeroChannel,
        useProxy: false,
      }] : []),
      ...(!explicitOnlyInMultiView ? [{
        stepLabel: "启动海康实时预览(直连+自动协商端口)",
        rtspPort,
        streamType: config.streamType,
        channelId: previewTarget.channelId,
        zeroChannel: previewTarget.zeroChannel || config.zeroChannel,
        useProxy: false,
      }] : []),
    ] as const
    const previewAttempts =
      explicitOnlyInMultiView
        ? [...directAttempts]
        : isMultiViewMode.value
          ? [...directAttempts, ...proxyAttempts]
          : (config.useProxy ? [...proxyAttempts, ...directAttempts] : [...directAttempts, ...proxyAttempts])

    previewStarting = true
    try {
      let lastPreviewError: unknown = null
      for (const attempt of previewAttempts) {
        try {
          playerMessage.value = ""
          await tryStartRealPlay(sdkRef, deviceId, attempt)
          lastPreviewError = null
          playerMessage.value = ""
          break
        } catch (error) {
          lastPreviewError = error
          stopActivePreview()
          playerMessage.value = ""
          if (isMultiViewMode.value && webSocketPort && attempt.stepLabel.includes("直连+显式WS端口")) {
            await sleep(MULTIVIEW_RETRY_DELAY_MS)
          }
        }
      }
      if (lastPreviewError) {
        const message = normalizePreviewStartError(lastPreviewError)
        if (!webSocketPort && !config.webSocketPort) {
          throw new Error(`设备未返回有效的 WebSocket 取流端口，当前设备固件可能不支持海康无插件预览；最后错误：${message}`)
        }
        throw new Error(message)
      }
      playerState.value = "ready"
    } finally {
      previewStarting = false
    }
  } catch (error) {
    playerMessage.value = resolvePlayerErrorMessage(error)
    await destroyPlayer()
  }
}

watch(
  () => [props.isPlaying, props.config] as const,
  () => {
    void startPlayer()
  },
  { immediate: true, deep: true },
)

watch(
  () => containerRef.value,
  (element) => {
    if (element) {
      element.id = containerId
      void startPlayer()
    }
  },
)

onBeforeUnmount(() => {
  startToken += 1
  void destroyPlayer()
  clearProxyCookies()
})
</script>

<template>
  <section class="hik-webcontrol-player">
    <div
      ref="containerRef"
      class="hik-webcontrol-player__surface"
      :class="{ 'hik-webcontrol-player__surface--ready': playerState === 'ready' }"
    />
    <div v-if="visibleMessage" class="hik-webcontrol-player__notice">
      {{ visibleMessage }}
    </div>
  </section>
</template>

<style scoped>
.hik-webcontrol-player {
  position: absolute;
  inset: 0;
}

.hik-webcontrol-player__surface {
  position: absolute;
  inset: 0;
  background: #0b1220;
}

.hik-webcontrol-player__surface--ready {
  background: #000;
}

.hik-webcontrol-player__notice {
  position: absolute;
  left: 50%;
  top: 50%;
  max-width: min(82%, 540px);
  padding: 12px 16px;
  border-radius: 12px;
  background: rgba(11, 18, 32, 0.78);
  color: #e5edf7;
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
  transform: translate(-50%, -50%);
  box-shadow: 0 16px 32px rgba(15, 23, 42, 0.24);
}
</style>
