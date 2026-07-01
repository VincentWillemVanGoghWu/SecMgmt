export interface DeviceStatusLogRecord {
  id: number
  deviceType: string
  deviceId: number
  deviceName: string
  oldStatus?: string | null
  newStatus: string
  message?: string | null
  checkedAt: string
}

export interface DeviceStatusLogPageRecord {
  items: DeviceStatusLogRecord[]
  total: number
  page: number
  pageSize: number
}

export interface DeviceStatusCheckAllData {
  checkedDevices: number
  changedDevices: number
  checkedCameras: number
  checkedRecorders: number
  checkedChannels: number
  onlineDevices?: number
  offlineDevices?: number
  disabledDevices?: number
  message: string
}

export type DeviceCheckNotifyMode = "offline_changed" | "offline_each_run"

export interface DeviceCheckScheduleRecord {
  id: number
  name: string
  enabled: boolean
  frequencyPerDay: number
  notifyEnabled: boolean
  pushConfigIds: number[]
  notifyMode: DeviceCheckNotifyMode
  lastRunAt?: string | null
  nextRunAt?: string | null
  lastSuccessAt?: string | null
  lastError?: string | null
  createdAt: string
  updatedAt: string
}

export interface DeviceCheckSchedulePayload {
  name: string
  enabled: boolean
  frequencyPerDay: number
  notifyEnabled: boolean
  pushConfigIds: number[]
  notifyMode: DeviceCheckNotifyMode
}

export interface DeviceCheckRunRecord {
  id: number
  scheduleId?: number | null
  startedAt: string
  finishedAt?: string | null
  status: string
  checkedTotal: number
  onlineTotal: number
  offlineTotal: number
  disabledTotal: number
  changedTotal: number
  notified: boolean
  errorMessage?: string | null
  createdAt: string
}

export interface DeviceCheckRunPageRecord {
  items: DeviceCheckRunRecord[]
  total: number
  page: number
  pageSize: number
}
