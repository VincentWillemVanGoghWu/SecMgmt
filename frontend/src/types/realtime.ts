export interface AlarmRealtimeEvent {
  alarm_id: number
  alarm_no: string
  alarm_type: string
  alarm_level: string
  alarm_time: string
  status: "pending" | "processing" | "done" | "false_alarm"
  occurrence_count: number
  factory_id?: number | null
  factory_name?: string | null
  zone_id?: number | null
  zone_name?: string | null
  camera_id?: number | null
  camera_name?: string | null
  recorder_id?: number | null
  recorder_name?: string | null
  channel_id?: number | null
  channel_name?: string | null
  message?: string | null
  image_url?: string | null
  video_url?: string | null
  record_start_time?: string | null
  record_end_time?: string | null
  last_event_time?: string | null
  created_at: string
}

export type RealtimeConnectionStatus = "idle" | "connecting" | "connected" | "reconnecting"
