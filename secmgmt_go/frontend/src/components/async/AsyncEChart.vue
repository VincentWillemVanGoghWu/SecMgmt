<script setup lang="ts">
import { defineAsyncComponent } from "vue"
import type { EChartsCoreOption } from "echarts/core"

const props = withDefaults(
  defineProps<{
    option: EChartsCoreOption
    height?: string
  }>(),
  {
    height: "280px",
  },
)

const EChartRenderer = defineAsyncComponent(() => import("./EChartRenderer.vue"))
</script>

<template>
  <Suspense>
    <component :is="EChartRenderer" :option="props.option" :height="props.height" />
    <template #fallback>
      <div class="async-panel async-panel--chart" :style="{ height: props.height }">
        <span>图表组件异步加载中...</span>
      </div>
    </template>
  </Suspense>
</template>

<style scoped>
.async-panel {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  border: 1px dashed #c9d8e6;
  border-radius: 12px;
  background: linear-gradient(180deg, #f9fbfd 0%, #f2f7fb 100%);
  color: #6c7f93;
  font-size: 13px;
}

.async-panel--chart {
  min-height: 220px;
}
</style>
