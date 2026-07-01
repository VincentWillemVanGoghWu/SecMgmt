import type { ApiResponse } from "../types/auth"
import type {
  RecorderChannelRecord,
  RecorderChannelSyncData,
  RecorderChannelUpdatePayload,
  RecorderConnectionTestData,
  RecorderRecord,
  RecorderSubmitPayload,
} from "../types/recorder"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listRecordersApi = async (params?: Record<string, unknown>): Promise<RecorderRecord[]> =>
  unwrap(await http.get<ApiResponse<RecorderRecord[]>>("/recorders", { params }))

export const getRecorderApi = async (id: number): Promise<RecorderRecord> =>
  unwrap(await http.get<ApiResponse<RecorderRecord>>(`/recorders/${id}`))

export const createRecorderApi = async (
  payload: RecorderSubmitPayload & { password: string },
): Promise<RecorderRecord> => unwrap(await http.post<ApiResponse<RecorderRecord>>("/recorders", payload))

export const updateRecorderApi = async (
  id: number,
  payload: RecorderSubmitPayload & { password?: string },
): Promise<RecorderRecord> => unwrap(await http.put<ApiResponse<RecorderRecord>>(`/recorders/${id}`, payload))

export const deleteRecorderApi = async (id: number): Promise<void> => {
  await http.delete(`/recorders/${id}`)
}

export const testRecorderConnectionApi = async (id: number): Promise<RecorderConnectionTestData> =>
  unwrap(await http.post<ApiResponse<RecorderConnectionTestData>>(`/recorders/${id}/test`))

export const syncRecorderChannelsApi = async (id: number): Promise<RecorderChannelSyncData> =>
  unwrap(await http.post<ApiResponse<RecorderChannelSyncData>>(`/recorders/${id}/sync-channels`))

export const listRecorderChannelsApi = async (recorderId: number): Promise<RecorderChannelRecord[]> =>
  unwrap(await http.get<ApiResponse<RecorderChannelRecord[]>>(`/recorders/${recorderId}/channels`))

export const listChannelsApi = async (params?: Record<string, unknown>): Promise<RecorderChannelRecord[]> =>
  unwrap(await http.get<ApiResponse<RecorderChannelRecord[]>>("/channels", { params }))

export const updateChannelApi = async (
  id: number,
  payload: RecorderChannelUpdatePayload,
): Promise<RecorderChannelRecord> => unwrap(await http.put<ApiResponse<RecorderChannelRecord>>(`/channels/${id}`, payload))
