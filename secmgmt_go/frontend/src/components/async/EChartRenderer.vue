<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue"
import { BarChart, LineChart, PieChart } from "echarts/charts"
import { GridComponent, LegendComponent, TooltipComponent } from "echarts/components"
import { init, use } from "echarts/core"
import type { ECharts, EChartsCoreOption } from "echarts/core"
import { CanvasRenderer } from "echarts/renderers"

use([PieChart, BarChart, LineChart, GridComponent, TooltipComponent, LegendComponent, CanvasRenderer])

const props = withDefaults(
  defineProps<{
    option: EChartsCoreOption
    height?: string
  }>(),
  {
    height: "280px",
  },
)

const containerRef = ref<HTMLDivElement | null>(null)
let chartInstance: ECharts | null = null

const resizeChart = () => {
  chartInstance?.resize()
}

const renderChart = () => {
  if (!containerRef.value) {
    return
  }

  if (!chartInstance) {
    chartInstance = init(containerRef.value)
  }

  chartInstance.setOption(props.option, true)
}

onMounted(() => {
  renderChart()
  window.addEventListener("resize", resizeChart)
})

watch(
  () => props.option,
  () => {
    renderChart()
  },
  { deep: true },
)

onBeforeUnmount(() => {
  window.removeEventListener("resize", resizeChart)
  chartInstance?.dispose()
  chartInstance = null
})
</script>

<template>
  <div ref="containerRef" class="chart-container" :style="{ height: props.height }" />
</template>

<style scoped>
.chart-container {
  width: 100%;
  min-height: 220px;
}
</style>
