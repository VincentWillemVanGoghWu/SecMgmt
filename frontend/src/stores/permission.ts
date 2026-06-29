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
    dataScopes: [] as DataScopeInfo[],
    loaded: false,
  }),
  getters: {
    allMenuItems: (state) => state.menuGroups,
    hasPermission: (state) => (permissionCode: string) => state.buttonPermissions.includes(permissionCode),
    defaultRouteName: (state) => findFirstRouteName(state.menuGroups),
  },
  actions: {
    setPermissionData(payload: {
      menus: MenuItem[]
      buttonPermissions: string[]
      dataScopes: DataScopeInfo[]
    }) {
      this.menuGroups = payload.menus
      this.buttonPermissions = payload.buttonPermissions
      this.dataScopes = payload.dataScopes
      this.loaded = true
    },
    clearPermissions() {
      this.menuGroups = []
      this.buttonPermissions = []
      this.dataScopes = []
      this.loaded = false
    },
  },
})
