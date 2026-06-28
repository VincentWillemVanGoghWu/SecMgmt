import type { ApiResponse } from "../types/auth"
import type {
  RoleDataScopeRecord,
  RoleDataScopeUpdatePayload,
  RoleStatusUpdatePayload,
  RoleSubmitPayload,
} from "../types/role"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listRolesApi = async (params?: Record<string, unknown>): Promise<RoleDataScopeRecord[]> =>
  unwrap(await http.get<ApiResponse<RoleDataScopeRecord[]>>("/roles", { params }))

export const createRoleApi = async (payload: RoleSubmitPayload): Promise<RoleDataScopeRecord> =>
  unwrap(await http.post<ApiResponse<RoleDataScopeRecord>>("/roles", payload))

export const updateRoleApi = async (id: number, payload: RoleSubmitPayload): Promise<RoleDataScopeRecord> =>
  unwrap(await http.put<ApiResponse<RoleDataScopeRecord>>(`/roles/${id}`, payload))

export const updateRoleStatusApi = async (
  id: number,
  payload: RoleStatusUpdatePayload,
): Promise<RoleDataScopeRecord> => unwrap(await http.patch<ApiResponse<RoleDataScopeRecord>>(`/roles/${id}/status`, payload))

export const deleteRoleApi = async (id: number): Promise<void> => {
  await http.delete(`/roles/${id}`)
}

export const updateRoleDataScopeApi = async (
  id: number,
  payload: RoleDataScopeUpdatePayload,
): Promise<RoleDataScopeRecord> =>
  unwrap(await http.put<ApiResponse<RoleDataScopeRecord>>(`/roles/${id}/data-scope`, payload))
