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
  message: string
}
