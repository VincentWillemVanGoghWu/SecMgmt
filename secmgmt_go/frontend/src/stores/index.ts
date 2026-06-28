import { createPinia } from 'pinia'

export { useAppStore } from './app'
export { useAuthStore } from './auth'
export { usePermissionStore } from './permission'
export { useRealtimeStore } from './realtime'

export const pinia = createPinia()
