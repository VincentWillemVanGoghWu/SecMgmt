import type { ApiResponse } from "../types/auth"
import type {
  DeviceCheckRunPageRecord,
  DeviceCheckSchedulePayload,
  DeviceCheckScheduleRecord,
  DeviceStatusCheckAllData,
  DeviceStatusLogPageRecord,
} from "../types/device-status"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listDeviceStatusLogsApi = async (params?: Record<string, unknown>): Promise<DeviceStatusLogPageRecord> =>
  unwrap(await http.get<ApiResponse<DeviceStatusLogPageRecord>>("/device-status/logs", { params }))

export const checkAllDevicesStatusApi = async (): Promise<DeviceStatusCheckAllData> =>
  unwrap(await http.post<ApiResponse<DeviceStatusCheckAllData>>("/devices/status/check-all"))

export const listDeviceCheckSchedulesApi = async (): Promise<DeviceCheckScheduleRecord[]> =>
  unwrap(await http.get<ApiResponse<DeviceCheckScheduleRecord[]>>("/device-check/schedules"))

export const createDeviceCheckScheduleApi = async (payload: DeviceCheckSchedulePayload): Promise<DeviceCheckScheduleRecord> =>
  unwrap(await http.post<ApiResponse<DeviceCheckScheduleRecord>>("/device-check/schedules", payload))

export const updateDeviceCheckScheduleApi = async (
  id: number,
  payload: DeviceCheckSchedulePayload,
): Promise<DeviceCheckScheduleRecord> =>
  unwrap(await http.put<ApiResponse<DeviceCheckScheduleRecord>>(`/device-check/schedules/${id}`, payload))

export const updateDeviceCheckScheduleStatusApi = async (id: number, enabled: boolean): Promise<DeviceCheckScheduleRecord> =>
  unwrap(await http.patch<ApiResponse<DeviceCheckScheduleRecord>>(`/device-check/schedules/${id}/status`, { enabled }))

export const deleteDeviceCheckScheduleApi = async (id: number): Promise<void> => {
  await http.delete(`/device-check/schedules/${id}`)
}

export const runDeviceCheckScheduleApi = async (id: number): Promise<DeviceStatusCheckAllData> =>
  unwrap(await http.post<ApiResponse<DeviceStatusCheckAllData>>(`/device-check/schedules/${id}/run`))

export const listDeviceCheckRunsApi = async (params?: Record<string, unknown>): Promise<DeviceCheckRunPageRecord> =>
  unwrap(await http.get<ApiResponse<DeviceCheckRunPageRecord>>("/device-check/runs", { params }))
