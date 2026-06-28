export interface RecorderRecord {
  id: number
  deviceCode: string
  name: string
  ip: string
  sdkPort: number
  httpPort: number
  username: string
  channelCount: number
  factoryId: number
  factoryName: string
  status: string
  lastOnlineAt?: string | null
  passwordConfigured: boolean
}

export interface RecorderSubmitPayload {
  deviceCode: string
  name: string
  ip: string
  sdkPort: number
  httpPort: number
  username: string
  channelCount: number
  factoryId: number
  status: string
  password?: string
}

export interface RecorderConnectionTestData {
  success: boolean
  status: string
  message: string
}

export interface RecorderStatusCheckData {
  status: string
  lastOnlineAt?: string | null
  message: string
}

export interface RecorderChannelRecord {
  id: number
  recorderId: number
  recorderName: string
  channelNo: number
  name: string
  cameraId?: number | null
  cameraName?: string | null
  factoryId: number
  factoryName: string
  zoneId?: number | null
  zoneName?: string | null
  enabled: boolean
  supportPlayback: boolean
  status: string
}

export interface RecorderChannelUpdatePayload {
  name: string
  cameraId?: number | null
  factoryId: number
  zoneId?: number | null
  enabled: boolean
  supportPlayback: boolean
  status: string
}

export interface RecorderChannelSyncData {
  recorderId: number
  recorderName: string
  channelCount: number
  channels: RecorderChannelRecord[]
}
