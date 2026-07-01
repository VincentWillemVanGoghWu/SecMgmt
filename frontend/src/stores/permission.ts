import { defineStore } from 'pinia'

import type { DataScopeInfo } from '../types/auth'
import type { MenuItem } from '../types/navigation'

const findFirstRouteName = (items: MenuItem[]): string | null => {
  for (const item of items) {
    if (item.routeName) {
      return item.routeName
    }
    if (item.children?.length) {
      const childRouteName = findFirstRouteName(item.children)
      if (childRouteName) {
        return childRouteName
      }
    }
  }
  return null
}

export const usePermissionStore = defineStore('permission', {
  state: () => ({
    menuGroups: [] as MenuItem[],
    buttonPermissions: [] as string[],
    roleCodes: [] as string[],
    dataScopes: [] as DataScopeInfo[],
    loaded: false,
  }),
  getters: {
    allMenuItems: (state) => state.menuGroups,
    hasPermission: (state) => (permissionCode: string) =>
      state.roleCodes.some((code) => code.trim().toLowerCase() === 'admin') || state.buttonPermissions.includes(permissionCode),
    defaultRouteName: (state) => findFirstRouteName(state.menuGroups),
  },
  actions: {
    setPermissionData(payload: {
      menus: MenuItem[]
      buttonPermissions: string[]
      roleCodes?: string[]
      dataScopes: DataScopeInfo[]
    }) {
      this.menuGroups = payload.menus
      this.buttonPermissions = payload.buttonPermissions
      this.roleCodes = payload.roleCodes ?? []
      this.dataScopes = payload.dataScopes
      this.loaded = true
    },
    clearPermissions() {
      this.menuGroups = []
      this.buttonPermissions = []
      this.roleCodes = []
      this.dataScopes = []
      this.loaded = false
    },
  },
})
