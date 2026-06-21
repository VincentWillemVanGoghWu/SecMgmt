export interface CameraRecord {
  id: number
  deviceCode: string
  name: string
  ip: string
  sdkPort: number
  httpPort: number
  rtspPort: number
  username: string
  factoryId: number
  factoryName: string
  zoneId: number
  zoneName: string
  installLocation?: string | null
  supportAi: boolean
  status: string
  lastOnlineAt?: string | null
  remark?: string | null
  passwordConfigured: boolean
}

export interface CameraSubmitPayload {
  deviceCode: string
  name: string
  ip: string
  sdkPort: number
  httpPort: number
  rtspPort: number
  username: string
  password?: string
  factoryId: number
  zoneId: number
  installLocation?: string | null
  supportAi: boolean
  status: string
  remark?: string | null
}

export interface CameraConnectionTestData {
  success: boolean
  status: string
  message: string
  rtspUrl: string
}

export interface CameraBrowserLoginPayload {
  ip: string
  port: number
  protocol: string
  username: string
  password: string
  loginUrl: string
}

export interface CameraStatusCheckData {
  status: string
  lastOnlineAt?: string | null
  message: string
}

export interface CameraTabConfigBase {
  supported: boolean
  message?: string | null
}

export interface CameraNetworkConfig extends CameraTabConfigBase {
  ip: string
  subnetMask?: string | null
  gateway?: string | null
  primaryDns?: string | null
  secondaryDns?: string | null
  dhcpEnabled: boolean
}

export interface CameraImageConfig extends CameraTabConfigBase {
  resolution?: string | null
  frameRate?: number | null
  bitrate?: number | null
  exposureMode?: string | null
  exposureTime?: string | null
  whiteBalanceMode?: string | null
}

export interface CameraRecordingConfig extends CameraTabConfigBase {
  scheduleMode?: string | null
  storageMode?: string | null
  overwriteEnabled?: boolean | null
  weeklyPlan: CameraRecordingScheduleDay[]
}

export interface CameraRecordingScheduleSlot {
  startTime: string
  endTime: string
  recordType: string
}

export interface CameraRecordingScheduleDay {
  dayOfWeek: string
  enabled: boolean
  slots: CameraRecordingScheduleSlot[]
}

export interface CameraPtzPreset {
  presetId: number
  name: string
}

export interface CameraPtzConfig extends CameraTabConfigBase {
  presetCount: number
  cruiseEnabled?: boolean | null
  trackEnabled?: boolean | null
  presets: CameraPtzPreset[]
}

export interface CameraUserAccount {
  userId: number
  username: string
  role?: string | null
  enabled: boolean
}

export interface CameraUserConfig extends CameraTabConfigBase {
  items: CameraUserAccount[]
}

export interface CameraSdkConfig {
  deviceName?: string | null
  deviceModel?: string | null
  deviceSerialNo?: string | null
  network: CameraNetworkConfig
  image: CameraImageConfig
  recording: CameraRecordingConfig
  ptz: CameraPtzConfig
  users: CameraUserConfig
}

export interface CameraDeviceIdentityPayload {
  cameraId?: number | null
  ip: string
  sdkPort: number
  httpPort: number
  rtspPort: number
  username: string
  password?: string | null
  factoryId: number
  zoneId: number
}

export interface CameraDeviceIdentity {
  deviceName?: string | null
  deviceModel?: string | null
  deviceSerialNo?: string | null
}

export interface CameraPtzPresetActionResult {
  success: boolean
  message: string
  presetId: number
}

export interface CameraPtzLensActionResult {
  success: boolean
  message: string
  action: "in" | "out" | string
}

export interface CameraUserAccountUpsert {
  userId?: number | null
  username: string
  password?: string | null
  role?: string | null
  enabled: boolean
}
