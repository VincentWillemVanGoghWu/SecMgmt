<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { RouterView, useRoute } from 'vue-router'

import AppHeader from '../components/layout/AppHeader.vue'
import AppMainContent from '../components/layout/AppMainContent.vue'
import RealtimeAlarmNotifier from '../components/realtime/RealtimeAlarmNotifier.vue'
import AppSidebar from '../components/layout/AppSidebar.vue'
import { useAppStore, useAuthStore, useRealtimeStore } from '../stores'

const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const realtimeStore = useRealtimeStore()
const pageTitle = computed(() => String(route.meta.title ?? '首页驾驶舱'))

watch(
  () => authStore.token,
  (token) => {
    if (token) {
      realtimeStore.start(token)
      return
    }
    realtimeStore.stop()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  realtimeStore.stop()
})
</script>

<template>
  <div class="app-layout" :class="{ 'app-layout--sidebar-collapsed': appStore.sidebarCollapsed }">
    <AppSidebar />
    <div class="app-layout__main">
      <AppHeader :title="pageTitle" />
      <AppMainContent>
        <RouterView />
      </AppMainContent>
    </div>
    <RealtimeAlarmNotifier />
  </div>
</template>
