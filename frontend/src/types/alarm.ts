import type { AiEventRecord } from "./ai-event"

export interface AlarmRecord {
  id: number
  alarmNo: string
  aiEventId?: number | null
  alarmType: string
  alarmLevel: string
  alarmTime: string
  status: "pending" | "processing" | "done" | "false_alarm"
  cameraId?: number | null
  cameraName?: string | null
  recorderId?: number | null
  recorderName?: string | null
  channelId?: number | null
  channelName?: string | null
  factoryId?: number | null
  factoryName?: string | null
  zoneId?: number | null
  zoneName?: string | null
  message?: string | null
  imageUrl?: string | null
  videoUrl?: string | null
  recordStartTime?: string | null
  recordEndTime?: string | null
  occurrenceCount: number
  lastEventTime?: string | null
  createdAt: string
}

export interface AlarmRealtimePageRecord {
  items: AlarmRecord[]
  total: number
  page: number
  pageSize: number
}

export interface AlarmPageRecord {
  items: AlarmRecord[]
  total: number
  page: number
  pageSize: number
}

export interface AlarmPushRecord {
  time: string
  channel: string
  status: string
  message: string
  operatorName?: string | null
}

export interface AlarmProcessLog {
  id: number
  action: string
  fromStatus?: string | null
  toStatus?: string | null
  operatorId?: number | null
  operatorName?: string | null
  remark?: string | null
  createdAt: string
}

export interface AlarmDetail extends AlarmRecord {
  aiEvent?: AiEventRecord | null
  cameraInfo?: Record<string, unknown> | null
  areaInfo?: Record<string, unknown> | null
  pushRecords: AlarmPushRecord[]
  processLogs: AlarmProcessLog[]
}
