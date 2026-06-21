import { defineStore } from 'pinia'

import type { DataScopeInfo } from '../types/auth'
import type { MenuItem } from '../types/navigation'

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
