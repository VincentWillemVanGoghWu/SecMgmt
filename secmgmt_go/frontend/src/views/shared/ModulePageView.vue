<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { RefreshRight, Search } from '@element-plus/icons-vue'

import PageCard from '../../components/common/PageCard.vue'
import SearchForm from '../../components/common/SearchForm.vue'
import TableCellValue from '../../components/common/TableCellValue.vue'
import { modulePageMap } from '../../mock/pages'

const route = useRoute()

const pageConfig = computed(() => {
  const pageKey = String(route.meta.pageKey ?? 'linkage')
  return modulePageMap[pageKey] ?? modulePageMap.linkage
})
</script>

<template>
  <div class="module-page">
    <PageCard class="module-page__filters-card" :title="pageConfig.title" :description="pageConfig.description">
      <SearchForm>
        <div v-for="filter in pageConfig.filters" :key="filter" class="app-field">
          <label>{{ filter }}</label>
          <input :placeholder="`请输入${filter}`" type="text" />
        </div>
        <template #actions>
          <button class="app-button app-button--primary module-page__button">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary module-page__button">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <PageCard title="Mock 数据列表" description="阶段 B 先用 Mock 数据承接页面切换和统一风格。">
      <table class="app-table">
        <thead>
          <tr>
            <th v-for="column in pageConfig.columns" :key="column.key">{{ column.label }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, rowIndex) in pageConfig.rows" :key="rowIndex">
            <td v-for="column in pageConfig.columns" :key="column.key">
              <TableCellValue :value="row[column.key]" />
            </td>
          </tr>
        </tbody>
      </table>
    </PageCard>
  </div>
</template>

<style scoped>
.module-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.module-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.module-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.module-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.module-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: repeat(4, minmax(160px, 1fr));
  gap: 10px;
  align-items: end;
}

.module-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.module-page__filters-card :deep(.app-field label) {
  margin-bottom: 6px;
  font-size: 12px;
}

.module-page__filters-card :deep(.app-field input),
.module-page__filters-card :deep(.app-field select) {
  height: 36px;
  font-size: 13px;
}

@media (max-width: 1100px) {
  .module-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .module-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 768px) {
  .module-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
