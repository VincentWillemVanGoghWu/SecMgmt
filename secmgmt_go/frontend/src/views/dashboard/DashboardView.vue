<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { useRouter } from "vue-router"
import type { EChartsCoreOption } from "echarts/core"
import { ElMessage } from "element-plus"
import { DataAnalysis, RefreshRight, Warning } from "@element-plus/icons-vue"

import AsyncEChart from "../../components/async/AsyncEChart.vue"
import {
  getDashboardAlarmTrendApi,
  getDashboardAlarmTypesApi,
  getDashboardSummaryApi,
} from "../../api/dashboard"
import type {
  CategoryChart,
  DashboardSummary,
  NameValueChart,
} from "../../types/dashboard"

const router = useRouter()

const loading = ref(false)
const summary = ref<DashboardSummary | null>(null)
const alarmTrend = ref<CategoryChart>({ categories: [], series: [] })
const alarmTypes = ref<NameValueChart>({ items: [] })

const queryForm = reactive({
  range: [] as string[],
})

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const formatPercent = (value?: number) => `${Number(value ?? 0).toFixed(2)}%`

const metricCards = computed(() => {
  const data = summary.value
  return [
    {
      label: "今日告警",
      value: data?.todayAlarmCount ?? 0,
      subtext: "点击查看告警报表",
      accent: "danger",
      action: () => router.push({ name: "safety-alarm-stats" }),
    },
    {
      label: "待处理告警",
      value: data?.pendingAlarmCount ?? 0,
      subtext: "点击查看待处理明细",
      accent: "warning",
      action: () => router.push({ name: "safety-alarm-list", query: { status: "pending" } }),
    },
    {
      label: "紧急告警",
      value: data?.criticalAlarmCount ?? 0,
      subtext: "点击查看严重告警",
      accent: "primary",
      action: () => router.push({ name: "safety-alarm-list", query: { level: "critical" } }),
    },
    {
      label: "推送成功率",
      value: formatPercent(data?.pushSuccessRate),
      subtext: "点击查看推送日志",
      accent: "success",
      action: () => router.push({ name: "push-logs" }),
    },
  ]
})

const trendOption = computed<EChartsCoreOption>(() => ({
  backgroundColor: "transparent",
  tooltip: { trigger: "axis" },
  legend: { top: 0, textStyle: { color: "#bfd0ea" } },
  grid: { left: 16, right: 16, bottom: 20, top: 40, containLabel: true },
  xAxis: {
    type: "category",
    data: alarmTrend.value.categories,
    axisLabel: { color: "#b6c9e6" },
    axisLine: { lineStyle: { color: "rgba(191, 208, 234, 0.25)" } },
  },
  yAxis: {
    type: "value",
    axisLabel: { color: "#b6c9e6" },
    splitLine: { lineStyle: { color: "rgba(191, 208, 234, 0.12)" } },
  },
  series: alarmTrend.value.series.map((item, index) => ({
    name: item.name,
    type: "line",
    smooth: true,
    data: item.data,
    symbolSize: 8,
    lineStyle: { width: 3, color: index === 0 ? "#49a5ff" : "#ff7f50" },
    itemStyle: { color: index === 0 ? "#49a5ff" : "#ff7f50" },
    areaStyle: {
      color:
        index === 0
          ? "rgba(73, 165, 255, 0.18)"
          : "rgba(255, 127, 80, 0.12)",
    },
  })),
}))

const alarmTypeOption = computed<EChartsCoreOption>(() => ({
  backgroundColor: "transparent",
  tooltip: { trigger: "item" },
  legend: {
    bottom: 0,
    textStyle: { color: "#bfd0ea" },
  },
  series: [
    {
      name: "告警类型",
      type: "pie",
      radius: ["40%", "68%"],
      center: ["50%", "44%"],
      label: { color: "#d5e3f7" },
      data: alarmTypes.value.items.map((item) => ({ name: item.name, value: item.value })),
    },
  ],
}))


const buildParams = () => {
  const [startAt, endAt] = queryForm.range ?? []
  return {
    start_at: startAt || undefined,
    end_at: endAt || undefined,
  }
}

const loadDashboard = async () => {
  loading.value = true
  try {
    const params = buildParams()
    const [summaryData, trendData, typeData] = await Promise.all([
      getDashboardSummaryApi(params),
      getDashboardAlarmTrendApi(params),
      getDashboardAlarmTypesApi(params),
    ])
    summary.value = summaryData
    alarmTrend.value = trendData
    alarmTypes.value = typeData
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载驾驶舱数据失败"))
  } finally {
    loading.value = false
  }
}

const resetQuery = async () => {
  queryForm.range = []
  await loadDashboard()
}

onMounted(async () => {
  await loadDashboard()
})
</script>

<template>
  <div class="dashboard-view">
    <section class="dashboard-hero">
      <div class="dashboard-hero__heading">
        <div>
          <h1>固定污染源监测</h1>
        </div>
        <div class="dashboard-hero__actions">
          <el-date-picker
            v-model="queryForm.range"
            type="datetimerange"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ss"
            class="dashboard-hero__picker"
          />
          <button
            v-permission="'dashboard:refresh'"
            class="app-button app-button--primary dashboard-hero__button"
            :disabled="loading"
            @click="loadDashboard"
          >
            <el-icon><RefreshRight /></el-icon>
            <span>{{ loading ? "刷新中..." : "刷新数据" }}</span>
          </button>
          <button class="app-button app-button--secondary dashboard-hero__button" :disabled="loading" @click="resetQuery">
            重置范围
          </button>
        </div>
      </div>

      <section class="dashboard-metrics">
        <article
          v-for="card in metricCards"
          :key="card.label"
          class="dashboard-metric-card"
          :class="`dashboard-metric-card--${card.accent}`"
          @click="card.action"
        >
          <span>{{ card.label }}</span>
          <strong>{{ card.value }}</strong>
          <small>{{ card.subtext }}</small>
        </article>
      </section>
    </section>

    <section class="dashboard-grid">
      <article class="dashboard-card">
        <header class="dashboard-card__header">
          <div>
            <h3>告警趋势</h3>
            <p>统计时间内告警总量与紧急告警走势。</p>
          </div>
          <el-icon class="dashboard-card__icon"><DataAnalysis /></el-icon>
        </header>
        <div class="dashboard-card__chart">
          <AsyncEChart :option="trendOption" height="100%" />
        </div>
      </article>

      <article class="dashboard-card">
        <header class="dashboard-card__header">
          <div>
            <h3>告警类型分布</h3>
            <p>按告警类型查看近期结构占比。</p>
          </div>
          <el-icon class="dashboard-card__icon"><Warning /></el-icon>
        </header>
        <div class="dashboard-card__chart">
          <AsyncEChart :option="alarmTypeOption" height="100%" />
        </div>
      </article>
    </section>
  </div>
</template>

<style scoped>
.dashboard-view {
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: calc(100vh - var(--layout-header-height) - (var(--layout-page-padding) * 2));
  min-height: 0;
  overflow: hidden;
  padding: 0;
  color: #e7f0ff;
}

.dashboard-hero {
  padding: 18px 20px;
  border-radius: 20px;
  background:
    radial-gradient(circle at top right, rgba(58, 134, 255, 0.2), transparent 32%),
    radial-gradient(circle at left bottom, rgba(42, 209, 255, 0.18), transparent 28%),
    linear-gradient(180deg, #091e3b 0%, #0d2c53 100%);
  box-shadow: 0 18px 40px rgba(8, 25, 46, 0.28);
}

.dashboard-hero__heading {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 14px;
}

.dashboard-hero__heading h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
}

.dashboard-hero__heading p {
  margin: 6px 0 0;
  color: rgba(231, 240, 255, 0.72);
  font-size: 13px;
}

.dashboard-hero__actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dashboard-hero__picker {
  width: 320px;
}

.dashboard-hero__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.dashboard-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.dashboard-metric-card {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid rgba(159, 190, 230, 0.16);
  background: linear-gradient(180deg, rgba(16, 52, 93, 0.82), rgba(10, 34, 63, 0.92));
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.dashboard-metric-card:hover {
  transform: translateY(-2px);
  border-color: rgba(73, 165, 255, 0.44);
}

.dashboard-metric-card span,
.dashboard-metric-card small {
  color: rgba(231, 240, 255, 0.72);
}

.dashboard-metric-card strong {
  font-size: 26px;
  color: #ffffff;
}

.dashboard-metric-card--danger strong {
  color: #ff7f8f;
}

.dashboard-metric-card--warning strong {
  color: #ffc857;
}

.dashboard-metric-card--primary strong {
  color: #64b2ff;
}

.dashboard-metric-card--success strong {
  color: #54e2a0;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
  flex: 1;
  min-height: 0;
  align-items: stretch;
}

.dashboard-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  border-radius: 16px;
  background: linear-gradient(180deg, #102e56 0%, #0b2444 100%);
  box-shadow: 0 18px 40px rgba(8, 25, 46, 0.22);
  min-height: 0;
}

.dashboard-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.dashboard-card__header h3 {
  margin: 0;
  color: #f5f9ff;
  font-size: 16px;
}

.dashboard-card__header p {
  margin: 4px 0 0;
  color: rgba(231, 240, 255, 0.68);
  font-size: 12px;
}

.dashboard-card__icon {
  font-size: 20px;
  color: #4fb4ff;
}

.dashboard-card__chart {
  flex: 1;
  min-height: 0;
}

@media (max-width: 1280px) {
  .dashboard-metrics,
  .dashboard-grid {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 960px) {
  .dashboard-hero__heading {
    flex-direction: column;
  }

  .dashboard-hero__actions {
    flex-wrap: wrap;
  }

  .dashboard-hero__picker {
    width: 100%;
  }

  .dashboard-metrics,
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}
</style>
