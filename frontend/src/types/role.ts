import type { MenuItem } from "./navigation"

export type DataScopeType = "all" | "factory" | "zone" | "device" | "dept" | "self" | "custom"
export type RoleStatus = "enabled" | "disabled"

export interface DeviceScopeValue {
  cameraIds: number[]
  recorderIds: number[]
  channelIds: number[]
}

export interface CustomScopeValue extends DeviceScopeValue {
  factoryIds: number[]
  zoneIds: number[]
  deptIds: number[]
  userIds: number[]
}

export interface RoleDataScopeRecord {
  id: number
  roleCode: string
  roleName: string
  status: RoleStatus | string
  remark?: string | null
  dataScopeType: DataScopeType | string
  dataScopeValue?: string | null
  menuCodes: string[]
  permissionCodes: string[]
}

export interface RoleSubmitPayload {
  roleCode: string
  roleName: string
  status: RoleStatus
  remark?: string | null
}

export interface RoleDataScopeUpdatePayload {
  dataScopeType: DataScopeType
  dataScopeValue?: number[] | DeviceScopeValue | CustomScopeValue | null
}

export interface RoleMenuTreeItem extends MenuItem {}

export interface RoleMenuUpdatePayload {
  menuIds: number[]
}

export interface RolePermissionOption {
  id: number
  name: string
  code: string
  isButton: boolean
  moduleKey: string
  resourceKey?: string
}

export interface RolePermissionUpdatePayload {
  permissionIds: number[]
}

export interface RoleStatusUpdatePayload {
  status: RoleStatus
}
