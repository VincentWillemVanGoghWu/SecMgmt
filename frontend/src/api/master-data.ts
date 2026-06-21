import type { ApiResponse } from "../types/auth"
import type {
  DeptRecord,
  DictItemRecord,
  DictTypeRecord,
  FactoryRecord,
  StatusPayload,
  ZoneRecord,
} from "../types/master-data"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listFactoriesApi = async (params?: Record<string, unknown>): Promise<FactoryRecord[]> =>
  unwrap(await http.get<ApiResponse<FactoryRecord[]>>("/factories", { params }))

export const createFactoryApi = async (payload: Omit<FactoryRecord, "id">): Promise<FactoryRecord> =>
  unwrap(await http.post<ApiResponse<FactoryRecord>>("/factories", payload))

export const updateFactoryApi = async (id: number, payload: Omit<FactoryRecord, "id">): Promise<FactoryRecord> =>
  unwrap(await http.put<ApiResponse<FactoryRecord>>(`/factories/${id}`, payload))

export const updateFactoryStatusApi = async (id: number, payload: StatusPayload): Promise<FactoryRecord> =>
  unwrap(await http.patch<ApiResponse<FactoryRecord>>(`/factories/${id}/status`, payload))

export const deleteFactoryApi = async (id: number): Promise<void> => {
  await http.delete(`/factories/${id}`)
}

export const listZonesApi = async (params?: Record<string, unknown>): Promise<ZoneRecord[]> =>
  unwrap(await http.get<ApiResponse<ZoneRecord[]>>("/zones", { params }))

export const createZoneApi = async (payload: Omit<ZoneRecord, "id" | "factoryName">): Promise<ZoneRecord> =>
  unwrap(await http.post<ApiResponse<ZoneRecord>>("/zones", payload))

export const updateZoneApi = async (
  id: number,
  payload: Omit<ZoneRecord, "id" | "factoryName">,
): Promise<ZoneRecord> => unwrap(await http.put<ApiResponse<ZoneRecord>>(`/zones/${id}`, payload))

export const updateZoneStatusApi = async (id: number, payload: StatusPayload): Promise<ZoneRecord> =>
  unwrap(await http.patch<ApiResponse<ZoneRecord>>(`/zones/${id}/status`, payload))

export const deleteZoneApi = async (id: number): Promise<void> => {
  await http.delete(`/zones/${id}`)
}

export const listDeptsApi = async (params?: Record<string, unknown>): Promise<DeptRecord[]> =>
  unwrap(await http.get<ApiResponse<DeptRecord[]>>("/depts", { params }))

export const createDeptApi = async (payload: Omit<DeptRecord, "id" | "parentName" | "factoryName" | "zoneName">): Promise<DeptRecord> =>
  unwrap(await http.post<ApiResponse<DeptRecord>>("/depts", payload))

export const updateDeptApi = async (
  id: number,
  payload: Omit<DeptRecord, "id" | "parentName" | "factoryName" | "zoneName">,
): Promise<DeptRecord> => unwrap(await http.put<ApiResponse<DeptRecord>>(`/depts/${id}`, payload))

export const updateDeptStatusApi = async (id: number, payload: StatusPayload): Promise<DeptRecord> =>
  unwrap(await http.patch<ApiResponse<DeptRecord>>(`/depts/${id}/status`, payload))

export const deleteDeptApi = async (id: number): Promise<void> => {
  await http.delete(`/depts/${id}`)
}

export const listDictTypesApi = async (params?: Record<string, unknown>): Promise<DictTypeRecord[]> =>
  unwrap(await http.get<ApiResponse<DictTypeRecord[]>>("/dicts", { params }))

export const createDictTypeApi = async (payload: Omit<DictTypeRecord, "id" | "items">): Promise<DictTypeRecord> =>
  unwrap(await http.post<ApiResponse<DictTypeRecord>>("/dicts/types", payload))

export const updateDictTypeApi = async (
  id: number,
  payload: Omit<DictTypeRecord, "id" | "items">,
): Promise<DictTypeRecord> => unwrap(await http.put<ApiResponse<DictTypeRecord>>(`/dicts/types/${id}`, payload))

export const updateDictTypeStatusApi = async (id: number, payload: StatusPayload): Promise<DictTypeRecord> =>
  unwrap(await http.patch<ApiResponse<DictTypeRecord>>(`/dicts/types/${id}/status`, payload))

export const deleteDictTypeApi = async (id: number): Promise<void> => {
  await http.delete(`/dicts/types/${id}`)
}

export const createDictItemApi = async (payload: Omit<DictItemRecord, "id">): Promise<DictItemRecord> =>
  unwrap(await http.post<ApiResponse<DictItemRecord>>("/dicts/items", payload))

export const updateDictItemApi = async (id: number, payload: Omit<DictItemRecord, "id">): Promise<DictItemRecord> =>
  unwrap(await http.put<ApiResponse<DictItemRecord>>(`/dicts/items/${id}`, payload))

export const updateDictItemStatusApi = async (id: number, payload: StatusPayload): Promise<DictItemRecord> =>
  unwrap(await http.patch<ApiResponse<DictItemRecord>>(`/dicts/items/${id}/status`, payload))

export const deleteDictItemApi = async (id: number): Promise<void> => {
  await http.delete(`/dicts/items/${id}`)
}
