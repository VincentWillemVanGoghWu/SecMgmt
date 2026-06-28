<script setup lang="ts">
import { computed } from "vue"

import { usePermissionStore } from "../../stores"

const permissionStore = usePermissionStore()

const parseCountSummary = (rawValue?: string | null) => {
  if (!rawValue || rawValue === "*") {
    return ""
  }
  try {
    const parsed = JSON.parse(rawValue)
    if (Array.isArray(parsed)) {
      return `${parsed.length} 项`
    }
    if (parsed && typeof parsed === "object") {
      return Object.entries(parsed)
        .filter(([, value]) => Array.isArray(value) && value.length > 0)
        .map(([key, value]) => `${key}:${(value as unknown[]).length}`)
        .join(" / ")
    }
  } catch {
    return rawValue
  }
  return rawValue
}

const scopeTypeLabel = (scopeType: string) => {
  if (scopeType === "all") return "全部数据"
  if (scopeType === "factory") return "指定厂区"
  if (scopeType === "zone") return "指定区域"
  if (scopeType === "device") return "指定设备"
  if (scopeType === "dept") return "本部门"
  if (scopeType === "self") return "本人"
  if (scopeType === "custom") return "自定义"
  return scopeType
}

const items = computed(() =>
  permissionStore.dataScopes.map((item) => ({
    key: `${item.roleCode}-${item.scopeType}`,
    roleCode: item.roleCode,
    label: scopeTypeLabel(item.scopeType),
    detail: parseCountSummary(item.scopeValue),
  })),
)
</script>

<template>
  <div v-if="items.length" class="data-scope-summary">
    <span class="data-scope-summary__label">当前数据范围</span>
    <div class="data-scope-summary__items">
      <span v-for="item in items" :key="item.key" class="data-scope-summary__tag">
        {{ item.roleCode }}: {{ item.label }}<template v-if="item.detail"> ({{ item.detail }})</template>
      </span>
    </div>
  </div>
</template>

<style scoped>
.data-scope-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 12px;
  align-items: center;
  padding: 14px 16px;
  border: 1px solid #dbe6f0;
  border-radius: 12px;
  background: #f8fbfe;
}

.data-scope-summary__label {
  color: #56708a;
  font-size: 13px;
  font-weight: 700;
}

.data-scope-summary__items {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.data-scope-summary__tag {
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(29, 122, 217, 0.1);
  color: #1b5f9f;
  font-size: 12px;
}
</style>
