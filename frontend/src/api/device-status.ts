import type { ApiResponse } from "../types/auth"
import type { DeviceStatusCheckAllData, DeviceStatusLogPageRecord } from "../types/device-status"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listDeviceStatusLogsApi = async (params?: Record<string, unknown>): Promise<DeviceStatusLogPageRecord> =>
  unwrap(await http.get<ApiResponse<DeviceStatusLogPageRecord>>("/device-status/logs", { params }))

export const checkAllDevicesStatusApi = async (): Promise<DeviceStatusCheckAllData> =>
  unwrap(await http.post<ApiResponse<DeviceStatusCheckAllData>>("/devices/status/check-all"))
