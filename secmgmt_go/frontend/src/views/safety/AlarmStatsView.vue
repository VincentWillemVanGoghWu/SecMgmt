<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { useRouter } from "vue-router"
import type { EChartsCoreOption } from "echarts/core"
import { ElMessage } from "element-plus"
import { RefreshRight, Search } from "@element-plus/icons-vue"

import AsyncEChart from "../../components/async/AsyncEChart.vue"
import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { getAlarmReportApi, getDeviceReportApi, getPushReportApi } from "../../api/dashboard"
import type { AlarmReportData, DeviceReportData, PushReportData } from "../../types/dashboard"

const router = useRouter()

const loading = ref(false)
const alarmReport = ref<AlarmReportData | null>(null)
const deviceReport = ref<DeviceReportData | null>(null)
const pushReport = ref<PushReportData | null>(null)
const zonePage = reactive({
  page: 1,
  pageSize: 30,
  total: 0,
})
const factoryPage = reactive({
  page: 1,
  pageSize: 30,
  total: 0,
})

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

const getStatusText = (value: string) => {
  if (value === "pending") return "未处理"
  if (value === "processing") return "处理中"
  if (value === "done") return "已处理"
  if (value === "false_alarm") return "误报"
  if (value === "success") return "成功"
  if (value === "failed") return "失败"
  if (value === "rate_limited") return "限流"
  return value
}

const getStatusTone = (value: string) => {
  if (value === "pending") return "danger"
  if (value === "processing") return "warning"
  if (value === "done") return "success"
  if (value === "false_alarm") return "default"
  if (value === "success") return "success"
  if (value === "failed") return "danger"
  if (value === "rate_limited") return "warning"
  return "default"
}

const buildParams = () => {
  const [startAt, endAt] = queryForm.range ?? []
  return {
    start_at: startAt || undefined,
    end_at: endAt || undefined,
  }
}

const buildAlarmReportParams = () => ({
  ...buildParams(),
  zone_page: zonePage.page,
  zone_page_size: zonePage.pageSize,
})

const buildDeviceReportParams = () => ({
  ...buildParams(),
  factory_page: factoryPage.page,
  factory_page_size: factoryPage.pageSize,
})

const alarmMetricCards = computed(() => {
  const summary = alarmReport.value?.summary
  return [
    {
      label: "今日告警",
      value: summary?.todayAlarmCount ?? 0,
      action: () => router.push({ name: "safety-alarm-list" }),
    },
    {
      label: "待处理",
      value: summary?.pendingAlarmCount ?? 0,
      action: () => router.push({ name: "safety-alarm-list", query: { status: "pending" } }),
    },
    {
      label: "紧急告警",
      value: summary?.criticalAlarmCount ?? 0,
      action: () => router.push({ name: "safety-alarm-list", query: { level: "critical" } }),
    },
    {
      label: "推送成功率",
      value: formatPercent(summary?.pushSuccessRate),
      action: () => router.push({ name: "push-logs" }),
    },
  ]
})

const alarmTrendOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: "axis" },
  legend: { top: 0 },
  grid: { left: 16, right: 16, bottom: 20, top: 40, containLabel: true },
  xAxis: { type: "category", data: alarmReport.value?.trend.categories ?? [] },
  yAxis: { type: "value" },
  series: (alarmReport.value?.trend.series ?? []).map((item, index) => ({
    name: item.name,
    type: "line",
    smooth: true,
    data: item.data,
    itemStyle: { color: index === 0 ? "#2f80ed" : "#ff7f50" },
    areaStyle: { color: index === 0 ? "rgba(47,128,237,0.10)" : "rgba(255,127,80,0.10)" },
  })),
}))

const alarmTypeOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: "item" },
  legend: { bottom: 0 },
  series: [
    {
      type: "pie",
      radius: ["42%", "70%"],
      data: alarmReport.value?.alarmTypes.items.map((item) => ({ name: item.name, value: item.value })) ?? [],
    },
  ],
}))

const statusOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: "axis" },
  grid: { left: 16, right: 16, bottom: 20, top: 16, containLabel: true },
  xAxis: { type: "category", data: alarmReport.value?.statusSummary.map((item) => getStatusText(item.name)) ?? [] },
  yAxis: { type: "value" },
  series: [
    {
      type: "bar",
      barWidth: 24,
      itemStyle: { color: "#2ec7c9", borderRadius: [8, 8, 0, 0] },
      data: alarmReport.value?.statusSummary.map((item) => item.value) ?? [],
    },
  ],
}))

const deviceTrendOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: "axis" },
  legend: { top: 0 },
  grid: { left: 16, right: 16, bottom: 20, top: 40, containLabel: true },
  xAxis: { type: "category", data: deviceReport.value?.statusTrend.categories ?? [] },
  yAxis: { type: "value" },
  series: (deviceReport.value?.statusTrend.series ?? []).map((item, index) => ({
    name: item.name,
    type: "bar",
    barGap: "20%",
    data: item.data,
    itemStyle: { color: index === 0 ? "#4caf50" : "#7c4dff" },
  })),
}))

const pushTrendOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: "axis" },
  legend: { top: 0 },
  grid: { left: 16, right: 16, bottom: 20, top: 40, containLabel: true },
  xAxis: { type: "category", data: pushReport.value?.trend.categories ?? [] },
  yAxis: { type: "value" },
  series: (pushReport.value?.trend.series ?? []).map((item, index) => ({
    name: item.name,
    type: "line",
    smooth: true,
    data: item.data,
    itemStyle: { color: index === 0 ? "#2fdd92" : "#ef5350" },
    lineStyle: { width: 3 },
  })),
}))

const pushChannelOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: "item" },
  legend: { bottom: 0 },
  series: [
    {
      type: "pie",
      radius: ["35%", "65%"],
      data: pushReport.value?.channelDistribution.items.map((item) => ({ name: item.name, value: item.value })) ?? [],
    },
  ],
}))

const loadAlarmReport = async () => {
  const alarmData = await getAlarmReportApi(buildAlarmReportParams())
  alarmReport.value = alarmData
  zonePage.total = alarmData.zoneRanking.total
  zonePage.page = alarmData.zoneRanking.page
  zonePage.pageSize = alarmData.zoneRanking.pageSize
}

const loadDeviceReport = async () => {
  const deviceData = await getDeviceReportApi(buildDeviceReportParams())
  deviceReport.value = deviceData
  factoryPage.total = deviceData.factoryStats.total
  factoryPage.page = deviceData.factoryStats.page
  factoryPage.pageSize = deviceData.factoryStats.pageSize
}

const loadPushReport = async () => {
  pushReport.value = await getPushReportApi(buildParams())
}

const loadReports = async () => {
  loading.value = true
  try {
    await Promise.all([loadAlarmReport(), loadDeviceReport(), loadPushReport()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载统计报表失败"))
  } finally {
    loading.value = false
  }
}

const resetQuery = async () => {
  queryForm.range = []
  zonePage.page = 1
  factoryPage.page = 1
  await loadReports()
}

const handleSearch = async () => {
  zonePage.page = 1
  factoryPage.page = 1
  await loadReports()
}

const handleZonePageChange = async (page: number) => {
  loading.value = true
  zonePage.page = page
  try {
    await loadAlarmReport()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载区域告警排行失败"))
  } finally {
    loading.value = false
  }
}

const handleFactoryPageChange = async (page: number) => {
  loading.value = true
  factoryPage.page = page
  try {
    await loadDeviceReport()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载厂区设备统计失败"))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadReports()
})
</script>

<template>
  <div class="alarm-stats-page">
    <PageCard
      class="alarm-stats-page__filters-card"
    >
      <SearchForm>
        <div class="app-field alarm-stats-page__range">
          <label>统计时间</label>
          <el-date-picker
            v-model="queryForm.range"
            type="datetimerange"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ss"
            style="width: 100%"
          />
        </div>
        <template #actions>
          <button class="app-button app-button--primary alarm-stats-page__button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>{{ loading ? "查询中..." : "查询" }}</span>
          </button>
          <button class="app-button app-button--secondary alarm-stats-page__button" @click="resetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="alarm-stats-page__metrics">
      <article
        v-for="card in alarmMetricCards"
        :key="card.label"
        class="alarm-stats-page__metric"
        @click="card.action"
      >
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <section class="alarm-stats-page__grid">
      <PageCard title="告警趋势" description="展示统计时间内告警总量和紧急告警的变化趋势。">
        <AsyncEChart :option="alarmTrendOption" height="300px" />
      </PageCard>

      <PageCard title="告警类型分布" description="按事件类型统计告警数量。">
        <AsyncEChart :option="alarmTypeOption" height="300px" />
      </PageCard>

      <PageCard title="告警状态分布" description="查看未处理、处理中、已处理和误报的数量。">
        <AsyncEChart :option="statusOption" height="300px" />
      </PageCard>

      <PageCard title="区域告警排行" description="支持点击区域定位到告警查询列表。">
        <table class="app-table">
          <thead>
            <tr>
              <th>区域</th>
              <th>厂区</th>
              <th>告警数</th>
              <th>待处理</th>
              <th>紧急</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!(alarmReport?.zoneRanking.items.length)">
              <td colspan="5" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
            </tr>
            <tr
              v-for="item in alarmReport?.zoneRanking.items ?? []"
              :key="`${item.factoryId}-${item.zoneId}`"
              class="alarm-stats-page__click-row"
              @click="router.push({ name: 'safety-alarm-list', query: { keyword: item.zoneName || undefined } })"
            >
              <td>{{ item.zoneName || "未绑定区域" }}</td>
              <td>{{ item.factoryName || "-" }}</td>
              <td>{{ item.alarmCount }}</td>
              <td><StatusTag text="待处理" tone="warning" /> {{ item.pendingCount }}</td>
              <td><StatusTag text="紧急" tone="danger" /> {{ item.criticalCount }}</td>
            </tr>
          </tbody>
        </table>
        <div class="alarm-stats-page__pagination">
          <el-pagination
            layout="total, prev, pager, next"
            :current-page="zonePage.page"
            :page-size="zonePage.pageSize"
            :total="zonePage.total"
            @current-change="handleZonePageChange"
          />
        </div>
      </PageCard>

      <PageCard title="设备状态变化趋势" description="基于设备状态日志统计摄像机和录像机变化次数。">
        <AsyncEChart :option="deviceTrendOption" height="300px" />
      </PageCard>

      <PageCard title="厂区设备统计" description="按厂区查看摄像机和录像机在线情况。">
        <table class="app-table">
          <thead>
            <tr>
              <th>厂区</th>
              <th>摄像机</th>
              <th>录像机</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!(deviceReport?.factoryStats.items.length)">
              <td colspan="3" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
            </tr>
            <tr v-for="item in deviceReport?.factoryStats.items ?? []" :key="item.factoryId">
              <td>{{ item.factoryName }}</td>
              <td>{{ item.cameraOnline }} / {{ item.cameraTotal }}</td>
              <td>{{ item.recorderOnline }} / {{ item.recorderTotal }}</td>
            </tr>
          </tbody>
        </table>
        <div class="alarm-stats-page__pagination">
          <el-pagination
            layout="total, prev, pager, next"
            :current-page="factoryPage.page"
            :page-size="factoryPage.pageSize"
            :total="factoryPage.total"
            @current-change="handleFactoryPageChange"
          />
        </div>
      </PageCard>

      <PageCard title="推送趋势" description="统计成功与失败的变化趋势，可跳转到推送日志查看明细。">
        <template #actions>
          <button class="app-button app-button--secondary alarm-stats-page__button" @click="router.push({ name: 'push-logs' })">
            查看推送日志
          </button>
        </template>
        <AsyncEChart :option="pushTrendOption" height="300px" />
      </PageCard>

      <PageCard title="推送渠道分布" description="查看钉钉与微信公众号等渠道的使用情况。">
        <AsyncEChart :option="pushChannelOption" height="300px" />
      </PageCard>

      <PageCard title="推送状态汇总" description="展示推送成功、失败和限流次数。">
        <section class="alarm-stats-page__push-overview">
          <article class="alarm-stats-page__push-card">
            <span>总次数</span>
            <strong>{{ pushReport?.overview.total ?? 0 }}</strong>
          </article>
          <article class="alarm-stats-page__push-card">
            <span>成功</span>
            <strong>{{ pushReport?.overview.success ?? 0 }}</strong>
          </article>
          <article class="alarm-stats-page__push-card">
            <span>失败</span>
            <strong>{{ pushReport?.overview.failed ?? 0 }}</strong>
          </article>
          <article class="alarm-stats-page__push-card">
            <span>成功率</span>
            <strong>{{ formatPercent(pushReport?.overview.successRate) }}</strong>
          </article>
        </section>
        <table class="app-table">
          <thead>
            <tr>
              <th>状态</th>
              <th>数量</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in pushReport?.statusDistribution.items ?? []" :key="item.name">
              <td><StatusTag :text="getStatusText(item.name)" :tone="getStatusTone(item.name)" /></td>
              <td>{{ item.value }}</td>
            </tr>
          </tbody>
        </table>
      </PageCard>
    </section>
  </div>
</template>

<style scoped>
.alarm-stats-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.alarm-stats-page__range {
  grid-column: auto;
}

.alarm-stats-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.alarm-stats-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.alarm-stats-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.alarm-stats-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: minmax(360px, 1fr);
  gap: 10px;
  align-items: end;
}

.alarm-stats-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.alarm-stats-page__filters-card :deep(.app-field label) {
  margin-bottom: 6px;
  font-size: 12px;
}

.alarm-stats-page__filters-card :deep(.app-field input),
.alarm-stats-page__filters-card :deep(.app-field select),
.alarm-stats-page__filters-card :deep(.el-input__wrapper) {
  font-size: 13px;
}

.alarm-stats-page__filters-card :deep(.el-date-editor) {
  min-height: 36px;
}

.alarm-stats-page__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.alarm-stats-page__metric {
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.alarm-stats-page__metric span {
  color: #667b91;
  font-size: 13px;
}

.alarm-stats-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
}

.alarm-stats-page__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.alarm-stats-page__click-row {
  cursor: pointer;
}

.alarm-stats-page__click-row:hover {
  background: rgba(73, 165, 255, 0.05);
}

.alarm-stats-page__push-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 14px;
}

.alarm-stats-page__push-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 14px 16px;
  border-radius: 12px;
  background: #f4f8fb;
}

.alarm-stats-page__push-card span {
  color: #667b91;
  font-size: 12px;
}

.alarm-stats-page__push-card strong {
  color: #0e2b4b;
  font-size: 22px;
}

.alarm-stats-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

@media (max-width: 1200px) {
  .alarm-stats-page__metrics,
  .alarm-stats-page__grid,
  .alarm-stats-page__push-overview {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 960px) {
  .alarm-stats-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .alarm-stats-page__metrics,
  .alarm-stats-page__grid,
  .alarm-stats-page__push-overview {
    grid-template-columns: 1fr;
  }

}
</style>
