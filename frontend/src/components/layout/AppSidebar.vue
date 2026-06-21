<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown } from '@element-plus/icons-vue'

import type { MenuItem } from '../../types/navigation'
import { useAppStore, usePermissionStore } from '../../stores'
import { resolveMenuIcon } from '../../utils/menu-icon'

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const permissionStore = usePermissionStore()

const menuGroups = computed(() => permissionStore.allMenuItems)
const isExpanded = (key: string) => appStore.expandedMenuKeys.includes(key)
const isRouteActive = (routeName?: string) => String(route.name ?? '') === String(routeName ?? '')
const isChildActive = (item: MenuItem) => item.children?.some((child) => isRouteActive(child.routeName)) ?? false
const isItemActive = (item: MenuItem) => isRouteActive(item.routeName) || isChildActive(item)
const getIcon = (icon?: string) => resolveMenuIcon(icon)

const syncExpandedMenuByRoute = () => {
  const activeParent = menuGroups.value.find((item) => isChildActive(item))
  if (activeParent) {
    appStore.setExpandedMenuGroup(activeParent.key)
    return
  }
  const activeLeaf = menuGroups.value.find((item) => isRouteActive(item.routeName))
  if (activeLeaf) {
    appStore.setExpandedMenuGroup(activeLeaf.key)
  }
}

const handleGroupClick = (item: MenuItem) => {
  if (item.children?.length) {
    appStore.toggleMenuGroup(item.key)
    return
  }
  if (item.routeName) {
    router.push({ name: item.routeName })
  }
}

watch([menuGroups, () => route.name], syncExpandedMenuByRoute, { immediate: true })
</script>

<template>
  <aside class="app-sidebar" :class="{ 'app-sidebar--collapsed': appStore.sidebarCollapsed }">
    <div class="app-sidebar__brand">
      <img class="app-sidebar__brand-logo" src="/bgLogo.png" alt="SmartLink" />
    </div>

    <nav class="app-sidebar__nav">
      <section v-for="item in menuGroups" :key="item.key" class="app-sidebar__section">
        <button
          type="button"
          class="app-sidebar__title"
          :class="{ 'app-sidebar__title--active': isItemActive(item) }"
          @click="handleGroupClick(item)"
        >
          <span class="app-sidebar__title-left">
            <span class="app-sidebar__icon">
              <component :is="getIcon(item.icon)" />
            </span>
            <span class="app-sidebar__label">{{ item.label }}</span>
          </span>
          <span v-if="item.children?.length" class="app-sidebar__arrow" :class="{ 'app-sidebar__arrow--expanded': isExpanded(item.key) }">
            <el-icon><ArrowDown /></el-icon>
          </span>
        </button>

        <div v-if="item.children?.length && isExpanded(item.key)" class="app-sidebar__children">
          <RouterLink
            v-for="child in item.children"
            :key="child.key"
            :to="{ name: child.routeName }"
            class="app-sidebar__item"
            :class="{ 'app-sidebar__item--active': isRouteActive(child.routeName) }"
          >
            <span class="app-sidebar__item-icon">
              <component :is="getIcon(child.icon)" />
            </span>
            <span>{{ child.label }}</span>
          </RouterLink>
        </div>
      </section>
    </nav>
  </aside>
</template>

<style scoped>
.app-sidebar {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: 240px;
  height: 100vh;
  background: #30455f;
  color: #edf3f9;
  border-radius: 0 22px 22px 0;
  box-shadow: 0 12px 36px rgba(15, 32, 54, 0.22);
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  z-index: 30;
}

.app-sidebar__nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 10px 18px;
}

.app-sidebar__brand {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 86px;
  padding: 14px 16px 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.app-sidebar__brand-logo {
  display: block;
  width: auto;
  height: auto;
  max-width: 100%;
  max-height: 58px;
  flex-shrink: 0;
  object-fit: contain;
}

.app-sidebar__section {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.app-sidebar__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  min-height: 50px;
  padding: 0 14px 0 18px;
  border: 0;
  border-radius: 14px;
  background: transparent;
  color: #eef4fb;
  cursor: pointer;
  font-family: 'Microsoft YaHei', 'PingFang SC', sans-serif;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.app-sidebar__title:hover {
  background: rgba(255, 255, 255, 0.04);
}

.app-sidebar__title--active {
  color: #ffffff;
  background: linear-gradient(135deg, rgba(28, 103, 255, 0.28), rgba(52, 181, 255, 0.18));
  box-shadow: 0 8px 18px rgba(20, 95, 221, 0.18);
}

.app-sidebar__title-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.app-sidebar__icon,
.app-sidebar__item-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.app-sidebar__icon {
  width: 18px;
  height: 18px;
  color: #d6e0eb;
  font-size: 16px;
}

.app-sidebar__label {
  overflow: hidden;
  font-size: 15px;
  font-weight: 500;
  letter-spacing: 0.2px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-sidebar__arrow {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: inherit;
  font-size: 12px;
  opacity: 0.9;
  transition: transform 0.2s ease;
}

.app-sidebar__arrow--expanded {
  transform: rotate(180deg);
}

.app-sidebar__children {
  display: flex;
  flex-direction: column;
  gap: 0;
  padding: 2px 0 8px;
}

.app-sidebar__item {
  display: flex;
  align-items: center;
  min-height: 46px;
  margin: 2px 0;
  padding: 0 18px 0 40px;
  border-radius: 12px;
  border-left: 0;
  color: #eef4fb;
  font-family: 'Microsoft YaHei', 'PingFang SC', sans-serif;
  font-size: 15px;
  font-weight: 400;
  text-decoration: none;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.app-sidebar__item:hover {
  background: rgba(255, 255, 255, 0.04);
  color: #ffffff;
}

.app-sidebar__item--active {
  border-left: 0;
  background: linear-gradient(135deg, rgba(31, 124, 255, 0.28), rgba(64, 205, 255, 0.12));
  box-shadow: 0 10px 20px rgba(14, 79, 191, 0.16);
  color: #54adff;
  font-weight: 500;
}

.app-sidebar__item-icon {
  display: none;
}

.app-sidebar--collapsed .app-sidebar__label,
.app-sidebar--collapsed .app-sidebar__arrow,
.app-sidebar--collapsed .app-sidebar__children {
  display: none;
}

.app-sidebar--collapsed.app-sidebar {
  width: 84px;
}

.app-sidebar--collapsed .app-sidebar__brand {
  justify-content: center;
  min-height: 74px;
  padding: 12px 10px;
}

.app-sidebar--collapsed .app-sidebar__brand-logo {
  width: auto;
  height: auto;
  max-width: 52px;
  max-height: 40px;
}

.app-sidebar--collapsed .app-sidebar__title {
  justify-content: center;
  padding: 12px 10px;
}

.app-sidebar--collapsed .app-sidebar__title-left {
  justify-content: center;
}

@media (max-width: 960px) {
  .app-sidebar {
    position: static;
    height: auto;
    min-height: auto;
  }
}
</style>
