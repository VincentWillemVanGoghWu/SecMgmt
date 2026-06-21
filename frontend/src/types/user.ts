import type { RoleStatus } from "./role"

export type UserStatus = "enabled" | "disabled"

export interface UserRoleSummary {
  id: number
  roleCode: string
  roleName: string
  status: RoleStatus | string
}

export interface UserRecord {
  id: number
  username: string
  realName: string
  deptId?: number | null
  deptName?: string | null
  status: UserStatus | string
  roles: UserRoleSummary[]
  createdAt: string
}

export interface UserSubmitPayload {
  username: string
  realName: string
  deptId?: number | null
  status: UserStatus
  roleIds: number[]
}

export interface UserCreatePayload extends UserSubmitPayload {
  password: string
}

export interface UserUpdatePayload extends UserSubmitPayload {
  password?: string
}
