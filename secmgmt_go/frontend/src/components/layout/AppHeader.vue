<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowRight, Bell, Connection, Expand, Fold, SwitchButton, UserFilled } from '@element-plus/icons-vue'

import { useAppStore, useAuthStore, useRealtimeStore } from '../../stores'

const props = defineProps<{
  title: string
}>()

const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const realtimeStore = useRealtimeStore()

const currentTitle = computed(() => props.title || '首页驾驶舱')
const breadcrumbItems = computed(() =>
  currentTitle.value
    .split('/')
    .map((item) => item.trim())
    .filter(Boolean),
)
const moduleTitle = computed(() => breadcrumbItems.value.at(-1) ?? currentTitle.value)
const realtimeLabel = computed(() => realtimeStore.connectionLabel)
const userName = computed(() => authStore.currentUser?.realName ?? '未登录')
const roleName = computed(() => authStore.primaryRoleName)
const toggleIcon = computed(() => (appStore.sidebarCollapsed ? Expand : Fold))

const handleLogout = async () => {
  await authStore.logout()
  await router.replace({ name: 'login' })
}
</script>

<template>
  <header class="app-header">
    <div class="app-header__left">
      <button class="app-header__toggle" type="button" @click="appStore.toggleSidebar()">
        <el-icon><component :is="toggleIcon" /></el-icon>
      </button>
      <div class="app-header__heading">
        <div class="app-header__title-row">
          <h1 class="app-header__title">{{ moduleTitle }}</h1>
          <nav class="app-header__breadcrumb" aria-label="页面路径">
            <span v-for="(item, index) in breadcrumbItems" :key="`${item}-${index}`" class="app-header__breadcrumb-item">
              <el-icon v-if="index > 0" class="app-header__breadcrumb-separator"><ArrowRight /></el-icon>
              <span>{{ item }}</span>
            </span>
          </nav>
        </div>
      </div>
    </div>

    <div class="app-header__actions">
      <div class="app-header__panel app-header__panel--status">
        <span class="app-header__panel-icon">
          <el-icon><Connection /></el-icon>
        </span>
        <div class="app-header__panel-content">
          <span class="app-header__realtime-status" :class="`app-header__realtime-status--${realtimeStore.connectionStatus}`">
            <i class="app-header__status-dot" />
            {{ realtimeLabel }}
          </span>
        </div>
      </div>

      <div class="app-header__panel app-header__panel--sound">
        <span class="app-header__panel-icon">
          <el-icon><Bell /></el-icon>
        </span>
        <div class="app-header__panel-content">
          <div class="app-header__sound-control">
            <span>{{ realtimeStore.soundEnabled ? '已开启' : '已关闭' }}</span>
            <el-switch
              :model-value="realtimeStore.soundEnabled"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @update:model-value="realtimeStore.setSoundEnabled"
            />
          </div>
        </div>
      </div>

      <div class="app-header__panel app-header__panel--user">
        <span class="app-header__panel-icon">
          <el-icon><UserFilled /></el-icon>
        </span>
        <div class="app-header__panel-content">
          <div class="app-header__user-meta">
            <strong>{{ roleName }}</strong>
            <span>{{ userName }}</span>
          </div>
        </div>
      </div>

      <button class="app-button app-button--secondary app-header__logout" type="button" @click="handleLogout">
        <el-icon><SwitchButton /></el-icon>
        <span>退出登录</span>
      </button>
    </div>
  </header>
</template>

<style scoped>
.app-header__left {
  min-width: 0;
  gap: 14px;
}

.app-header__heading {
  min-width: 0;
}

.app-header__title-row {
  display: flex;
  align-items: baseline;
  gap: 16px;
  min-width: 0;
}

.app-header__title {
  margin: 0;
  white-space: nowrap;
}

.app-header__breadcrumb {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}

.app-header__breadcrumb-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #7f92a8;
  font-size: 13px;
  white-space: nowrap;
}

.app-header__breadcrumb-separator {
  color: #9eb0c3;
  font-size: 11px;
}

.app-header__actions {
  display: flex;
  align-items: stretch;
  gap: 12px;
}

.app-header__panel {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  min-height: 42px;
  padding: 8px 12px;
  border: 1px solid rgba(132, 154, 180, 0.16);
  border-radius: 14px;
  background: rgba(248, 251, 255, 0.9);
  box-shadow: 0 8px 20px rgba(17, 43, 74, 0.05);
}

.app-header__panel-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border-radius: 10px;
  background: rgba(36, 125, 255, 0.08);
  color: var(--color-primary);
  font-size: 16px;
}

.app-header__panel-content {
  display: flex;
  align-items: center;
  min-height: 30px;
}

.app-header__realtime-status,
.app-header__sound-control,
.app-header__user-meta {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #1d3045;
  font-size: 13px;
  font-weight: 600;
}

.app-header__realtime-status {
  min-height: 24px;
}

.app-header__status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #97a6b8;
  box-shadow: 0 0 0 4px rgba(151, 166, 184, 0.14);
}

.app-header__realtime-status--connected .app-header__status-dot {
  background: var(--color-success);
  box-shadow: 0 0 0 4px rgba(32, 178, 107, 0.14);
}

.app-header__realtime-status--connecting .app-header__status-dot,
.app-header__realtime-status--reconnecting .app-header__status-dot {
  background: var(--color-warning);
  box-shadow: 0 0 0 4px rgba(255, 159, 47, 0.14);
}

.app-header__sound-control,
.app-header__user-meta {
  gap: 10px;
}

.app-header__user-meta {
  align-items: baseline;
}

.app-header__user-meta strong {
  color: #17314d;
  font-size: 13px;
}

.app-header__user-meta span,
.app-header__sound-control span {
  color: #6e849b;
  font-size: 12px;
  font-weight: 500;
}

.app-header__logout {
  min-width: 112px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 1380px) {
  .app-header {
    align-items: flex-start;
    padding-top: 12px;
    padding-bottom: 12px;
  }

  .app-header__title-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }

  .app-header__actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }
}

@media (max-width: 960px) {
  .app-header {
    flex-direction: column;
    gap: 12px;
  }

  .app-header__left,
  .app-header__actions {
    width: 100%;
  }

  .app-header__actions {
    justify-content: flex-start;
  }
}
</style>
