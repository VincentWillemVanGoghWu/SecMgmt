import { defineStore } from "pinia"

import { AlarmRealtimeClient } from "../services/realtime/alarmRealtimeClient"
import type { AlarmRealtimeEvent, RealtimeConnectionStatus } from "../types/realtime"

const SOUND_ENABLED_KEY = "steel-monitor-realtime-sound-enabled"

type AlarmListener = (event: AlarmRealtimeEvent) => void

const listeners = new Set<AlarmListener>()
let realtimeClient: AlarmRealtimeClient | null = null

const readSoundEnabled = () => window.localStorage.getItem(SOUND_ENABLED_KEY) !== "false"

export const useRealtimeStore = defineStore("realtime", {
  state: () => ({
    connectionStatus: "idle" as RealtimeConnectionStatus,
    lastError: "" as string,
    soundEnabled: readSoundEnabled(),
    eventSequence: 0,
    lastAlarmEvent: null as AlarmRealtimeEvent | null,
    recentEvents: [] as AlarmRealtimeEvent[],
    activeToken: "" as string,
  }),
  getters: {
    connectionLabel: (state) => {
      if (state.connectionStatus === "connected") return "实时已连接"
      if (state.connectionStatus === "connecting") return "实时连接中"
      if (state.connectionStatus === "reconnecting") return "实时重连中"
      return "实时未连接"
    },
  },
  actions: {
    start(token: string) {
      if (!token) {
        this.stop()
        return
      }
      if (this.activeToken === token && realtimeClient) {
        return
      }

      this.stop()
      this.activeToken = token
      realtimeClient = new AlarmRealtimeClient({
        token,
        onAlarm: (event) => this.handleAlarmEvent(event),
        onStateChange: (status, error) => {
          this.connectionStatus = status
          this.lastError = error ?? ""
        },
      })
      realtimeClient.start()
    },
    stop() {
      realtimeClient?.stop()
      realtimeClient = null
      this.activeToken = ""
      this.connectionStatus = "idle"
      this.lastError = ""
    },
    subscribe(listener: AlarmListener) {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    setSoundEnabled(value: boolean) {
      this.soundEnabled = value
      window.localStorage.setItem(SOUND_ENABLED_KEY, String(value))
    },
    handleAlarmEvent(event: AlarmRealtimeEvent) {
      this.lastAlarmEvent = event
      this.eventSequence += 1
      this.recentEvents = [event, ...this.recentEvents.filter((item) => item.alarm_id !== event.alarm_id)].slice(0, 20)
      listeners.forEach((listener) => listener(event))
      if (event.alarm_level === "critical" || event.alarm_level === "high") {
        this.playAlertSound()
      }
    },
    playAlertSound() {
      if (!this.soundEnabled) return
      const AudioContextClass = window.AudioContext || (window as typeof window & { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
      if (!AudioContextClass) return

      try {
        const audioContext = new AudioContextClass()
        const oscillator = audioContext.createOscillator()
        const gainNode = audioContext.createGain()
        oscillator.type = "sine"
        oscillator.frequency.value = 880
        gainNode.gain.value = 0.0001
        oscillator.connect(gainNode)
        gainNode.connect(audioContext.destination)
        const now = audioContext.currentTime
        gainNode.gain.exponentialRampToValueAtTime(0.08, now + 0.02)
        gainNode.gain.exponentialRampToValueAtTime(0.0001, now + 0.35)
        oscillator.start(now)
        oscillator.stop(now + 0.38)
        oscillator.onended = () => {
          void audioContext.close()
        }
      } catch {
        this.lastError = "当前浏览器阻止了声音播放"
      }
    },
  },
})
