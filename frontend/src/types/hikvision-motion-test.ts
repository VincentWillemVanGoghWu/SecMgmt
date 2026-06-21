export interface HikvisionMotionTestEvent {
  eventTime: string
  deviceIp?: string | null
  command: number
  alarmType: number
  channels: number[]
  message: string
  snapshotUrl?: string | null
}

export interface HikvisionMotionTestState {
  running: boolean
  sdkAvailable: boolean
  sessionId?: string | null
  sourceType?: "camera" | "channel" | null
  sourceId?: number | null
  sourceName?: string | null
  cameraId?: number | null
  cameraName?: string | null
  cameraIp?: string | null
  recorderId?: number | null
  recorderName?: string | null
  recorderIp?: string | null
  channelId?: number | null
  channelName?: string | null
  selectedChannelNo?: number | null
  startedAt?: string | null
  lastEventAt?: string | null
  eventCount: number
  message: string
  lastError?: string | null
  lastSnapshotUrl?: string | null
  lastSnapshotAt?: string | null
  recentEvents: HikvisionMotionTestEvent[]
}

export interface HikvisionManualSnapshot {
  sourceType: "camera" | "channel"
  sourceId: number
  sourceName: string
  cameraId?: number | null
  cameraName?: string | null
  cameraIp?: string | null
  recorderId?: number | null
  recorderName?: string | null
  recorderIp?: string | null
  channelId?: number | null
  channelName?: string | null
  channelNo: number
  capturedAt: string
  snapshotUrl: string
  message: string
}
