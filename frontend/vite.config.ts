import type { ClientRequest, IncomingMessage, ServerResponse } from 'node:http'
import type { Socket } from 'node:net'

import httpProxy from 'http-proxy'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

const { createProxyServer } = httpProxy

type ProxyOptionsLike = { target?: string | URL }
type ProxyResponseLike = { statusCode?: number }
const HIK_PROXY_HTTP_HEADER = 'x-hik-proxy-target'
const HIK_PROXY_WS_TARGET_PARAM = '__hikProxyTarget'

const webControlHeaders = {
  'Cross-Origin-Embedder-Policy': 'require-corp',
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Resource-Policy': 'cross-origin',
} as const

const parseCookieMap = (cookieHeader?: string) =>
  Object.fromEntries(
    (cookieHeader ?? '')
      .split(';')
      .map((item) => item.trim())
      .filter(Boolean)
      .map((item) => {
        const separatorIndex = item.indexOf('=')
        if (separatorIndex < 0) {
          return [item, '']
        }
        return [item.slice(0, separatorIndex), decodeURIComponent(item.slice(separatorIndex + 1))]
      }),
  )

const resolveRequestUrl = (value: string | undefined) => {
  if (!value) {
    return null
  }
  try {
    return new URL(value, 'http://127.0.0.1')
  } catch {
    return null
  }
}

const normalizeTarget = (value: string | undefined, protocol: 'http' | 'https') => {
  if (!value) {
    return null
  }
  return value.startsWith('http://') || value.startsWith('https://') ? value : `${protocol}://${value}`
}

const resolveHttpProxyTarget = (request: IncomingMessage) => {
  const explicitTarget = request.headers[HIK_PROXY_HTTP_HEADER]
  const headerTarget = Array.isArray(explicitTarget) ? explicitTarget[0] : explicitTarget
  const normalizedHeaderTarget = normalizeTarget(headerTarget, 'http')
  if (normalizedHeaderTarget) {
    return normalizedHeaderTarget
  }

  const requestUrl = resolveRequestUrl(request.url)
  const queryTarget = normalizeTarget(requestUrl?.searchParams.get(HIK_PROXY_WS_TARGET_PARAM) ?? undefined, 'http')
  if (queryTarget) {
    return queryTarget
  }

  const cookies = parseCookieMap(request.headers.cookie)
  return normalizeTarget(cookies.webVideoCtrlProxy, 'http')
}

const resolveWebSocketProxyTarget = (request: IncomingMessage) => {
  const requestUrl = resolveRequestUrl(request.url)
  const explicitTarget = requestUrl?.searchParams.get(HIK_PROXY_WS_TARGET_PARAM) ?? undefined
  if (explicitTarget) {
    return explicitTarget.startsWith('ws://') || explicitTarget.startsWith('wss://')
      ? explicitTarget
      : normalizeTarget(explicitTarget, 'http')?.replace(/^http:/, 'ws:')?.replace(/^https:/, 'wss:') ?? null
  }

  const cookies = parseCookieMap(request.headers.cookie)
  const secureTarget = normalizeTarget(cookies.webVideoCtrlProxyWss, 'https')
  if (secureTarget) {
    return secureTarget.replace(/^https:/, 'wss:')
  }

  const plainWsTarget = normalizeTarget(cookies.webVideoCtrlProxyWs, 'http')
  if (plainWsTarget) {
    return plainWsTarget.replace(/^http:/, 'ws:')
  }

  const fallbackHttpTarget = normalizeTarget(cookies.webVideoCtrlProxy, 'http')
  if (fallbackHttpTarget) {
    return fallbackHttpTarget.replace(/^http:/, 'ws:')
  }

  return null
}

const applyProxyRequestHeaders = (
  proxyRequest: ClientRequest,
  target: string | undefined,
) => {
  if (target) {
    proxyRequest.setHeader('host', new URL(target).host)
  }
  proxyRequest.removeHeader('origin')
  proxyRequest.removeHeader('referer')
  proxyRequest.removeHeader('sec-fetch-site')
  proxyRequest.removeHeader('sec-fetch-mode')
  proxyRequest.removeHeader('sec-fetch-dest')
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    Components({
      dts: false,
      resolvers: [ElementPlusResolver({ importStyle: 'css' })],
    }),
    {
      name: 'hik-webcontrol-dev-proxy',
      configureServer(server) {
        const httpProxy = createProxyServer({
          changeOrigin: true,
          secure: false,
          target: 'http://127.0.0.1',
        })
        const wsProxy = createProxyServer({
          changeOrigin: true,
          secure: false,
          ws: true,
          target: 'http://127.0.0.1',
        })

        httpProxy.on('proxyReq', (proxyReq: ClientRequest, _req: IncomingMessage, _res: ServerResponse, options: ProxyOptionsLike) => {
          applyProxyRequestHeaders(proxyReq, typeof options.target === 'string' ? options.target : undefined)
        })
        httpProxy.on('proxyRes', (proxyRes: ProxyResponseLike, req: IncomingMessage, _res: ServerResponse) => {
          const target = resolveHttpProxyTarget(req) ?? 'unknown-target'
          console.log(`[hik-proxy] ${req.method} ${req.url} -> ${target} ${proxyRes.statusCode ?? ''}`)
        })
        httpProxy.on('error', (error: Error, _req: IncomingMessage, response: ServerResponse) => {
          const res = response as ServerResponse | undefined
          if (!res || res.headersSent) {
            return
          }
          res.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8', ...webControlHeaders })
          res.end(JSON.stringify({ message: `海康代理转发失败: ${error.message}` }))
        })

        wsProxy.on('proxyReqWs', (proxyReq: ClientRequest, _req: IncomingMessage, _socket: Socket, options: ProxyOptionsLike) => {
          applyProxyRequestHeaders(proxyReq, typeof options.target === 'string' ? options.target : undefined)
        })
        wsProxy.on('error', (error: Error, _req: IncomingMessage, socket: Socket) => {
          const wsSocket = socket as Socket | undefined
          if (wsSocket && !wsSocket.destroyed) {
            wsSocket.destroy(new Error(`海康 WebSocket 代理失败: ${error.message}`))
          }
        })

        server.middlewares.use((req, res, next) => {
          const url = req.url ?? ''
          if (!url.startsWith('/ISAPI') && !url.startsWith('/SDK')) {
            next()
            return
          }

          const target = resolveHttpProxyTarget(req)
          if (!target) {
            res.writeHead(502, { 'Content-Type': 'application/json; charset=utf-8', ...webControlHeaders })
            res.end(JSON.stringify({ message: '缺少 HIK 代理目标，无法转发海康 ISAPI/SDK 请求。' }))
            return
          }

          console.log(`[hik-proxy] ${req.method} ${url} -> ${target}`)
          httpProxy.web(req, res, { target })
        })

        server.httpServer?.on('upgrade', (req, socket, head) => {
          const url = req.url ?? ''
          if (!url.startsWith('/webSocketVideoCtrlProxy')) {
            return
          }

          const target = resolveWebSocketProxyTarget(req)
          if (!target) {
            socket.destroy()
            return
          }

          console.log(`[hik-proxy] WS ${url} -> ${target}`)
          wsProxy.ws(req, socket, head, { target })
        })
      },
    },
  ],
  server: {
    headers: webControlHeaders,
  },
  preview: {
    headers: webControlHeaders,
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          const normalizedId = id.replace(/\\/g, '/')
          if (!normalizedId.includes('node_modules')) {
            return
          }

          if (normalizedId.includes('@element-plus/icons-vue')) {
            return 'element-plus-icons'
          }
          if (normalizedId.includes('element-plus/theme-chalk')) {
            return 'element-plus-style'
          }
          if (normalizedId.includes('element-plus/es/components/')) {
            if (/element-plus\/es\/components\/(table|table-column|pagination|scrollbar|virtual-list)\//.test(normalizedId)) {
              return 'element-plus-table'
            }
            if (/element-plus\/es\/components\/(form|form-item|input|input-number|select|option|option-group|date-picker|time-picker|time-select|checkbox|checkbox-group|checkbox-button|radio|radio-group|radio-button|switch|slider|upload)\//.test(normalizedId)) {
              return 'element-plus-form'
            }
            if (/element-plus\/es\/components\/(dialog|drawer|message|message-box|notification|popper|popover|tooltip|overlay|focus-trap)\//.test(normalizedId)) {
              return 'element-plus-overlay'
            }
            if (/element-plus\/es\/components\/(button|button-group|icon|tag|empty|card|tabs|tab-pane|menu|menu-item|sub-menu|dropdown|dropdown-menu|dropdown-item)\//.test(normalizedId)) {
              return 'element-plus-basic'
            }
            return 'element-plus-components'
          }
          if (normalizedId.includes('element-plus')) {
            return 'element-plus-core'
          }
          if (normalizedId.includes('zrender')) {
            return 'echarts-zrender'
          }
          if (normalizedId.includes('echarts/lib/chart') || normalizedId.includes('echarts/charts')) {
            return 'echarts-charts'
          }
          if (normalizedId.includes('echarts/lib/component') || normalizedId.includes('echarts/components')) {
            return 'echarts-components'
          }
          if (normalizedId.includes('echarts/lib/renderer') || normalizedId.includes('echarts/renderers')) {
            return 'echarts-renderers'
          }
          if (normalizedId.includes('echarts/lib/coord') || normalizedId.includes('echarts/lib/scale')) {
            return 'echarts-coord'
          }
          if (normalizedId.includes('echarts/lib/data') || normalizedId.includes('echarts/lib/model')) {
            return 'echarts-data'
          }
          if (normalizedId.includes('echarts/lib/util') || normalizedId.includes('echarts/lib/core')) {
            return 'echarts-core'
          }
          if (normalizedId.includes('echarts')) {
            return 'echarts-core'
          }
          if (normalizedId.includes('hls.js')) {
            return 'video-hls'
          }
          if (normalizedId.includes('hikvideoctrl')) {
            return 'video-hik-webcontrol'
          }
          if (normalizedId.includes('vue')) {
            return 'vue-vendor'
          }
          if (normalizedId.includes('axios')) {
            return 'http-vendor'
          }
        },
      },
    },
  },
})
