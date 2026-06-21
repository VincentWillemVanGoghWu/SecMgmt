import { pinia, usePermissionStore } from "../stores"

export const hasPermission = (permissionCode: string): boolean => {
  const permissionStore = usePermissionStore(pinia)
  return permissionStore.hasPermission(permissionCode)
}
