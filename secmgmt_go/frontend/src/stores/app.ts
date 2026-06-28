import { defineStore } from 'pinia'

const defaultExpandedKeys = ['dashboard']

export const useAppStore = defineStore('app', {
  state: () => ({
    sidebarCollapsed: false,
    expandedMenuKeys: defaultExpandedKeys as string[],
  }),
  actions: {
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    },
    setExpandedMenuGroup(key: string | null) {
      this.expandedMenuKeys = key ? [key] : []
    },
    toggleMenuGroup(key: string) {
      if (this.expandedMenuKeys.includes(key)) {
        this.expandedMenuKeys = []
        return
      }
      this.expandedMenuKeys = [key]
    },
  },
})
