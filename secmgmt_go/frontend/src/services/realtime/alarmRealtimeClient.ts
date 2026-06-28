import { buildApiUrl } from "../../api/http"
import type { AlarmRealtimeEvent, RealtimeConnectionStatus } from "../../types/realtime"

interface AlarmRealtimeClientOptions {
  token: string
  onAlarm: (event: AlarmRealtimeEvent) => void
  onStateChange: (status: RealtimeConnectionStatus, error?: string | null) => void
}

export class AlarmRealtimeClient {
  private eventSource: EventSource | null = null
  private reconnectTimer: number | null = null
  private reconnectAttempts = 0
  private stopped = false
  private readonly options: AlarmRealtimeClientOptions

  constructor(options: AlarmRealtimeClientOptions) {
    this.options = options
  }

  start() {
    this.stopped = false
    this.connect()
  }

  stop() {
    this.stopped = true
    this.clearReconnectTimer()
    this.eventSource?.close()
    this.eventSource = null
    this.options.onStateChange("idle", null)
  }

  private connect() {
    if (this.stopped) return

    this.clearReconnectTimer()
    this.options.onStateChange(this.reconnectAttempts > 0 ? "reconnecting" : "connecting", null)

    const url = buildApiUrl(`/sse/alarms?token=${encodeURIComponent(this.options.token)}`)
    const source = new EventSource(url)
    this.eventSource = source

    source.addEventListener("connected", () => {
      this.reconnectAttempts = 0
      this.options.onStateChange("connected", null)
    })

    source.addEventListener("alarm", (event) => {
      try {
        const payload = JSON.parse((event as MessageEvent<string>).data) as AlarmRealtimeEvent
        this.options.onAlarm(payload)
      } catch {
        this.options.onStateChange("reconnecting", "实时告警消息解析失败")
      }
    })

    source.onerror = () => {
      if (this.stopped) return
      source.close()
      if (this.eventSource === source) {
        this.eventSource = null
      }
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect() {
    this.reconnectAttempts += 1
    const delay = Math.min(10000, 2000 * this.reconnectAttempts)
    this.options.onStateChange("reconnecting", `连接中断，${Math.round(delay / 1000)} 秒后重连`)
    this.reconnectTimer = window.setTimeout(() => {
      this.connect()
    }, delay)
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }
}
