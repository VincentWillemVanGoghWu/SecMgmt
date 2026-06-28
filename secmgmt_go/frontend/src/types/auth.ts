import type { MenuItem } from "./navigation"

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginTokenData {
  access_token: string
  token_type: string
  expires_in: number
}

export interface AuthUser {
  id: number
  username: string
  realName: string
  status: string
}

export interface RoleInfo {
  id: number
  roleCode: string
  roleName: string
  status: string
}

export interface DataScopeInfo {
  roleCode: string
  scopeType: string
  scopeValue?: string | null
}

export interface MeData {
  user: AuthUser
  roles: RoleInfo[]
  menus: MenuItem[]
  buttonPermissions: string[]
  dataScopes: DataScopeInfo[]
}

export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}
