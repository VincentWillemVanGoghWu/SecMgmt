import type { LiveWebControlConfig } from "../../types/video"

const HTTP_PROXY_HEADER = "X-Hik-Proxy-Target"
const WS_PROXY_TARGET_PARAM = "__hikProxyTarget"
const WS_PROXY_CHANNEL_PARAM = "__hikProxyChannel"
const GLOBAL_STATE_KEY = "__hikProxyRoutingState__"

interface PendingProxyState {
  httpTarget?: string
  wsTarget?: string
  wsChannel?: string
}

interface HikProxyRoutingState {
  installed: boolean
  deviceTargets: Map<string, string>
  pending: PendingProxyState
}

declare global {
  interface Window {
    __hikProxyRoutingState__?: HikProxyRoutingState
  }
}

const resolveGlobalState = () => {
  if (!window[GLOBAL_STATE_KEY]) {
    window[GLOBAL_STATE_KEY] = {
      installed: false,
      deviceTargets: new Map<string, string>(),
      pending: {},
    }
  }
  return window[GLOBAL_STATE_KEY] as HikProxyRoutingState
}

const resolveCookieDescriptor = () =>
  Object.getOwnPropertyDescriptor(Document.prototype, "cookie")
  ?? Object.getOwnPropertyDescriptor(HTMLDocument.prototype, "cookie")

const parseCookieAssignment = (value: string) => {
  const firstSegment = value.split(";")[0]?.trim() ?? ""
  const separatorIndex = firstSegment.indexOf("=")
  if (separatorIndex < 0) {
    return { name: "", cookieValue: "" }
  }
  const name = firstSegment.slice(0, separatorIndex).trim()
  const cookieValue = decodeURIComponent(firstSegment.slice(separatorIndex + 1).trim())
  return { name, cookieValue }
}

const buildHttpTarget = (state: HikProxyRoutingState, cookieValue: string) =>
  state.deviceTargets.get(cookieValue) ?? (cookieValue.includes("://") ? cookieValue : `http://${cookieValue}`)

const buildWebSocketTarget = (protocol: "ws" | "wss", cookieValue: string) =>
  cookieValue.startsWith("ws://") || cookieValue.startsWith("wss://") ? cookieValue : `${protocol}://${cookieValue}`

const isSameOriginProxyUrl = (url: URL) => {
  if (url.origin !== window.location.origin) {
    return false
  }
  return url.pathname.startsWith("/ISAPI") || url.pathname.startsWith("/SDK") || url.pathname.startsWith("/PSIA")
}

const isWebSocketProxyUrl = (url: URL) => url.pathname.endsWith("/webSocketVideoCtrlProxy")

const patchCookieAccess = (state: HikProxyRoutingState) => {
  const descriptor = resolveCookieDescriptor()
  if (!descriptor?.get || !descriptor.set) {
    return
  }

  Object.defineProperty(document, "cookie", {
    configurable: true,
    enumerable: descriptor.enumerable ?? true,
    get() {
      return descriptor.get!.call(document)
    },
    set(value: string) {
      const { name, cookieValue } = parseCookieAssignment(value)
      if (name === "webVideoCtrlProxy" && cookieValue) {
        state.pending.httpTarget = buildHttpTarget(state, cookieValue)
      } else if (name === "webVideoCtrlProxyWs" && cookieValue) {
        state.pending.wsTarget = buildWebSocketTarget("ws", cookieValue)
      } else if (name === "webVideoCtrlProxyWss" && cookieValue) {
        state.pending.wsTarget = buildWebSocketTarget("wss", cookieValue)
      } else if (name === "webVideoCtrlProxyWsChannel" && cookieValue) {
        state.pending.wsChannel = cookieValue
      }
      descriptor.set!.call(document, value)
    },
  })
}

const patchXmlHttpRequest = (state: HikProxyRoutingState) => {
  const originalOpen = XMLHttpRequest.prototype.open
  const originalSend = XMLHttpRequest.prototype.send
  const originalSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader
  const requestMetadata = new WeakMap<XMLHttpRequest, { target?: string }>()

  XMLHttpRequest.prototype.open = function patchedOpen(
    method: string,
    url: string | URL,
    async?: boolean,
    username?: string | null,
    password?: string | null,
  ) {
    try {
      const resolvedUrl = new URL(String(url), window.location.origin)
      if (isSameOriginProxyUrl(resolvedUrl) && state.pending.httpTarget) {
        requestMetadata.set(this, { target: state.pending.httpTarget })
        state.pending.httpTarget = undefined
      }
    } catch {
      // Ignore URL parsing errors and fall back to the original request.
    }

    return originalOpen.call(this, method, url, async ?? true, username ?? null, password ?? null)
  }

  XMLHttpRequest.prototype.send = function patchedSend(body?: Document | XMLHttpRequestBodyInit | null) {
    const metadata = requestMetadata.get(this)
    if (metadata?.target) {
      originalSetRequestHeader.call(this, HTTP_PROXY_HEADER, metadata.target)
    }
    return originalSend.call(this, body)
  }
}

const patchWebSocket = (state: HikProxyRoutingState) => {
  const OriginalWebSocket = window.WebSocket

  const PatchedWebSocket = function patchedWebSocket(
    url: string | URL,
    protocols?: string | string[],
  ) {
    let resolvedUrl = String(url)
    try {
      const candidate = new URL(resolvedUrl, window.location.origin.replace(/^http/, "ws"))
      if (isWebSocketProxyUrl(candidate)) {
        if (state.pending.wsTarget) {
          candidate.searchParams.set(WS_PROXY_TARGET_PARAM, state.pending.wsTarget)
        }
        if (state.pending.wsChannel) {
          candidate.searchParams.set(WS_PROXY_CHANNEL_PARAM, state.pending.wsChannel)
        }
        resolvedUrl = candidate.toString()
      }
    } catch {
      // Keep the original URL when parsing fails.
    }

    state.pending.wsTarget = undefined
    state.pending.wsChannel = undefined
    return protocols === undefined ? new OriginalWebSocket(resolvedUrl) : new OriginalWebSocket(resolvedUrl, protocols)
  } as unknown as typeof WebSocket

  Object.setPrototypeOf(PatchedWebSocket, OriginalWebSocket)
  PatchedWebSocket.prototype = OriginalWebSocket.prototype
  window.WebSocket = PatchedWebSocket
}

export const ensureHikProxyRoutingInstalled = () => {
  const state = resolveGlobalState()
  if (state.installed) {
    return state
  }
  patchCookieAccess(state)
  patchXmlHttpRequest(state)
  patchWebSocket(state)
  state.installed = true
  return state
}

export const syncHikProxyDeviceTargets = (configs: Array<LiveWebControlConfig | null | undefined>) => {
  const state = ensureHikProxyRoutingInstalled()
  state.deviceTargets.clear()
  configs.forEach((config) => {
    if (!config) {
      return
    }
    const key = `${config.host}:${config.port}`
    const target = `${config.protocol}://${config.host}:${config.port}`
    state.deviceTargets.set(key, target)
  })
}

export const clearHikProxyDeviceTargets = () => {
  const state = ensureHikProxyRoutingInstalled()
  state.deviceTargets.clear()
  state.pending.httpTarget = undefined
  state.pending.wsTarget = undefined
  state.pending.wsChannel = undefined
}

export const HIK_PROXY_HTTP_HEADER = HTTP_PROXY_HEADER
export const HIK_PROXY_WS_TARGET_PARAM = WS_PROXY_TARGET_PARAM
export const HIK_PROXY_WS_CHANNEL_PARAM = WS_PROXY_CHANNEL_PARAM
