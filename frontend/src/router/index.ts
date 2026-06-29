import { createRouter, createWebHistory } from 'vue-router'

import { pinia, useAuthStore, usePermissionStore } from '../stores'
import { appRoutes } from './routes'

export const router = createRouter({
  history: createWebHistory(),
  routes: appRoutes,
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore(pinia)
  const permissionStore = usePermissionStore(pinia)

  if (to.meta.guestOnly && authStore.isAuthenticated) {
    if (!permissionStore.loaded) {
      try {
        await authStore.initializeAuth()
      } catch {
        authStore.clearAuthState()
        return true
      }
    }
    return { name: permissionStore.defaultRouteName ?? 'dashboard' }
  }

  if (!to.meta.requiresAuth) {
    return true
  }

  if (!authStore.isAuthenticated) {
    return {
      name: 'login',
      query: { redirect: to.fullPath },
    }
  }

  if (!permissionStore.loaded) {
    try {
      await authStore.initializeAuth()
    } catch {
      authStore.clearAuthState()
      return {
        name: 'login',
        query: { redirect: to.fullPath },
      }
    }
  }

  if (
    to.name === 'dashboard' &&
    permissionStore.defaultRouteName &&
    permissionStore.defaultRouteName !== 'dashboard'
  ) {
    return { name: permissionStore.defaultRouteName }
  }

  return true
})
