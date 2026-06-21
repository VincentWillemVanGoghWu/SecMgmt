import type { ApiResponse } from "../types/auth"
import type {
  PushConfigRecord,
  PushConfigSubmitPayload,
  PushConfigTestResult,
  PushLogPageRecord,
  PushLogRecord,
} from "../types/push"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listPushConfigsApi = async (params?: Record<string, unknown>): Promise<PushConfigRecord[]> =>
  unwrap(await http.get<ApiResponse<PushConfigRecord[]>>("/push/configs", { params }))

export const createPushConfigApi = async (payload: PushConfigSubmitPayload): Promise<PushConfigRecord> =>
  unwrap(await http.post<ApiResponse<PushConfigRecord>>("/push/configs", payload))

export const updatePushConfigApi = async (id: number, payload: PushConfigSubmitPayload): Promise<PushConfigRecord> =>
  unwrap(await http.put<ApiResponse<PushConfigRecord>>(`/push/configs/${id}`, payload))

export const updatePushConfigStatusApi = async (id: number, enabled: boolean): Promise<PushConfigRecord> =>
  unwrap(await http.patch<ApiResponse<PushConfigRecord>>(`/push/configs/${id}/status`, { enabled }))

export const deletePushConfigApi = async (id: number): Promise<void> => {
  await http.delete(`/push/configs/${id}`)
}

export const testPushConfigApi = async (id: number): Promise<PushConfigTestResult> =>
  unwrap(await http.post<ApiResponse<PushConfigTestResult>>(`/push/configs/${id}/test`))

export const listPushLogsApi = async (params?: Record<string, unknown>): Promise<PushLogPageRecord> =>
  unwrap(await http.get<ApiResponse<PushLogPageRecord>>("/push/logs", { params }))

export const retryPushLogApi = async (id: number): Promise<PushLogRecord> =>
  unwrap(await http.post<ApiResponse<PushLogRecord>>(`/push/logs/${id}/retry`))

