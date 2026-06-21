import type { ApiResponse, LoginRequest, LoginTokenData, MeData } from "../types/auth"
import type { MenuItem } from "../types/navigation"
import { http } from "./http"

export const loginApi = async (payload: LoginRequest): Promise<LoginTokenData> => {
  const response = await http.post<ApiResponse<LoginTokenData>>("/auth/login", payload)
  return response.data.data
}

export const logoutApi = async (): Promise<void> => {
  await http.post("/auth/logout")
}

export const getMeApi = async (): Promise<MeData> => {
  const response = await http.get<ApiResponse<MeData>>("/auth/me")
  return response.data.data
}

export const getMenusApi = async (): Promise<MenuItem[]> => {
  const response = await http.get<ApiResponse<MenuItem[]>>("/menus")
  return response.data.data
}
