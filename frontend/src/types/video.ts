export type StreamType = "http-flv" | "hls" | "webrtc" | "hik-sdk"
export type StreamProfile = "main" | "sub"
export type ConnectionMode = "standard" | "hik-sdk"
export type PlaybackMode = "hik" | "hls" | "native"
export type PreviewBrowseMode = "browser" | "webcontrol"

export interface LiveVideoPayload {
  cameraId?: number | null
  channelId?: number | null
  streamType: StreamType
  connectionMode: ConnectionMode
  playUrl: string
  expiresIn: number
  isMock: boolean
  playableInBrowser: boolean
  diagnosticMessage?: string | null
  sourceRtsp?: string | null
}

export interface StopLiveVideoPayload {
  cameraId?: number | null
  channelId?: number | null
  stopped: boolean
  message: string
}

export interface LiveWebControlConfig {
  sourceType: "camera" | "channel"
  cameraId?: number | null
  channelId?: number | null
  deviceName: string
  host: string
  port: number
  protocol: "http" | "https"
  username: string
  password: string
  channelNo: number
  streamType: 1 | 2
  streamProfile: StreamProfile
  zeroChannel: boolean
  useProxy: boolean
  webSocketPort?: number | null
  rtspPort?: number | null
  supported: boolean
  message?: string | null
}

export interface SnapshotPayload {
  cameraId?: number | null
  channelId?: number | null
  snapshotUrl: string
  expiresIn: number
}

export interface PlaybackSegmentRecord {
  startTime: string
  endTime: string
  channelId: number
  channelName: string
  recorderId: number
  recorderName: string
  cameraId?: number | null
  cameraName?: string | null
  recordType: string
  available: boolean
}

export interface PlaybackTimelineSpan {
  startTime: string
  endTime: string
  actualStartTime?: string
  actualEndTime?: string
  recordType: string
  available: boolean
  playbackUri?: string
  fileName?: string
}

export interface PlaybackUrlPayload {
  streamType: StreamType
  streamProfile?: StreamProfile
  playbackMode?: PlaybackMode
  playUrl: string
  startTime: string
  endTime: string
  expiresIn: number
  isMock: boolean
  playableInBrowser: boolean
  diagnosticMessage?: string | null
  sourceRtsp?: string | null
}

export interface StopPlaybackPayload {
  channelId: number
  stopped: boolean
  message: string
}
