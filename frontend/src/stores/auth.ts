import { defineStore } from 'pinia'

import { getMeApi, loginApi, logoutApi } from '../api/auth'
import type { AuthUser, RoleInfo } from '../types/auth'
import { usePermissionStore } from './permission'
import { useRealtimeStore } from './realtime'

const TOKEN_KEY = 'steel-monitor-access-token'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: window.localStorage.getItem(TOKEN_KEY) ?? '',
    currentUser: null as AuthUser | null,
    roles: [] as RoleInfo[],
    initialized: false,
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token),
    primaryRoleName: (state) => state.roles[0]?.roleName ?? '未分配角色',
  },
  actions: {
    setToken(token: string) {
      this.token = token
      window.localStorage.setItem(TOKEN_KEY, token)
    },
    clearAuthState() {
      const permissionStore = usePermissionStore()
      const realtimeStore = useRealtimeStore()
      this.token = ''
      this.currentUser = null
      this.roles = []
      this.initialized = false
      permissionStore.clearPermissions()
      realtimeStore.stop()
      window.localStorage.removeItem(TOKEN_KEY)
    },
    async login(payload: { username: string; password: string }) {
      const loginData = await loginApi(payload)
      this.setToken(loginData.access_token)
      await this.fetchProfile()
      return loginData
    },
    async fetchProfile() {
      const permissionStore = usePermissionStore()
      const meData = await getMeApi()
      this.currentUser = meData.user
      this.roles = meData.roles
      permissionStore.setPermissionData({
        menus: meData.menus,
        buttonPermissions: meData.buttonPermissions,
        dataScopes: meData.dataScopes,
      })
      this.initialized = true
      return meData
    },
    async logout() {
      try {
        if (this.token) {
          await logoutApi()
        }
      } finally {
        this.clearAuthState()
      }
    },
    handleTokenExpired() {
      this.clearAuthState()
      if (window.location.pathname !== '/login') {
        window.location.replace('/login')
      }
    },
    async initializeAuth() {
      if (!this.token || this.initialized) {
        return
      }
      await this.fetchProfile()
    },
  },
})
