import type { ApiResponse } from "../types/auth"
import type { AlarmDetail, AlarmPageRecord, AlarmRealtimePageRecord, AlarmRecord } from "../types/alarm"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listRealtimeAlarmsApi = async (params?: Record<string, unknown>): Promise<AlarmRealtimePageRecord> =>
  unwrap(await http.get<ApiResponse<AlarmRealtimePageRecord>>("/alarms/realtime", { params }))

export const listAlarmsApi = async (params?: Record<string, unknown>): Promise<AlarmPageRecord> =>
  unwrap(await http.get<ApiResponse<AlarmPageRecord>>("/alarms", { params }))

export const getAlarmDetailApi = async (id: number): Promise<AlarmDetail> =>
  unwrap(await http.get<ApiResponse<AlarmDetail>>(`/alarms/${id}`))

export const processAlarmApi = async (
  id: number,
  payload: { status: "processing" | "done"; remark?: string },
): Promise<AlarmRecord> => unwrap(await http.post<ApiResponse<AlarmRecord>>(`/alarms/${id}/process`, payload))

export const falseAlarmApi = async (
  id: number,
  payload?: { remark?: string },
): Promise<AlarmRecord> => unwrap(await http.post<ApiResponse<AlarmRecord>>(`/alarms/${id}/false-alarm`, payload ?? {}))

export const repushAlarmApi = async (id: number): Promise<AlarmRecord> =>
  unwrap(await http.post<ApiResponse<AlarmRecord>>(`/alarms/${id}/repush`))
