import type { ApiResponse } from "../types/auth"
import type { UserCreatePayload, UserRecord, UserUpdatePayload } from "../types/user"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listUsersApi = async (params?: Record<string, unknown>): Promise<UserRecord[]> =>
  unwrap(await http.get<ApiResponse<UserRecord[]>>("/users", { params }))

export const createUserApi = async (payload: UserCreatePayload): Promise<UserRecord> =>
  unwrap(await http.post<ApiResponse<UserRecord>>("/users", payload))

export const updateUserApi = async (id: number, payload: UserUpdatePayload): Promise<UserRecord> =>
  unwrap(await http.put<ApiResponse<UserRecord>>(`/users/${id}`, payload))

export const deleteUserApi = async (id: number): Promise<void> => {
  await http.delete(`/users/${id}`)
}
