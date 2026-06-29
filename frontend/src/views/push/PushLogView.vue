<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import { Download, RefreshRight, Search } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { exportPushLogsApi } from "../../api/export"
import { listPushLogsApi, retryPushLogApi } from "../../api/push"
import type { PushLogRecord } from "../../types/push"

const loading = ref(false)
const retryingId = ref<number | null>(null)
const exporting = ref(false)
const records = ref<PushLogRecord[]>([])
const pagination = reactive({
  page: 1,
  pageSize: 30,
  total: 0,
})

const queryForm = reactive({
  channel: "",
  status: "",
  alarmType: "",
  range: [] as string[],
})

const channelOptions = [
  { label: "全部", value: "" },
  { label: "钉钉", value: "dingtalk" },
  { label: "微信公众号", value: "wechat" },
]

const statusOptions = [
  { label: "全部", value: "" },
  { label: "成功", value: "success" },
  { label: "失败", value: "failed" },
  { label: "限流", value: "rate_limited" },
]

const alarmTypeOptions = [
  { label: "全部", value: "" },
  { label: "未戴安全帽", value: "helmet_missing" },
  { label: "区域入侵", value: "intrusion" },
  { label: "移动侦测", value: "移动侦测" },
  { label: "烟雾", value: "smoke" },
  { label: "明火", value: "fire" },
  { label: "人员跌倒", value: "person_fall" },
]

const metrics = computed(() => [
  { label: "日志总数", value: pagination.total, accent: "primary" },
  { label: "成功推送", value: records.value.filter((item) => item.status === "success").length, accent: "success" },
  { label: "失败推送", value: records.value.filter((item) => item.status === "failed").length, accent: "danger" },
  { label: "限流跳过", value: records.value.filter((item) => item.status === "rate_limited").length, accent: "info" },
])

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", { hour12: false })
}

const getStatusText = (status: string) => {
  if (status === "success") return "成功"
  if (status === "failed") return "失败"
  if (status === "rate_limited") return "限流"
  return status
}

const getStatusTone = (status: string) => {
  if (status === "success") return "success"
  if (status === "failed") return "danger"
  if (status === "rate_limited") return "warning"
  return "default"
}

const getLevelText = (value?: string | null) => {
  if (value === "critical") return "严重"
  if (value === "high") return "高"
  if (value === "medium") return "中"
  if (value === "low") return "低"
  return value || "-"
}

const getLevelTone = (value?: string | null) => {
  if (value === "critical" || value === "high") return "danger"
  if (value === "medium") return "warning"
  if (value === "low") return "info"
  return "default"
}

const getTriggeredByText = (value: string) => {
  if (value === "auto") return "自动"
  if (value === "manual") return "手动"
  if (value === "test") return "测试"
  return value
}

const getChannelText = (value: string) => {
  if (value === "dingtalk") return "钉钉"
  if (value === "wechat") return "微信公众号"
  return value
}

const loadRecords = async () => {
  loading.value = true
  try {
    const [startAt, endAt] = queryForm.range ?? []
    const result = await listPushLogsApi({
      channel: queryForm.channel || undefined,
      status: queryForm.status || undefined,
      alarm_type: queryForm.alarmType || undefined,
      start_at: startAt || undefined,
      end_at: endAt || undefined,
      page: pagination.page,
      page_size: pagination.pageSize,
    })
    records.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.pageSize = result.pageSize
  } finally {
    loading.value = false
  }
}

const handleSearch = async () => {
  pagination.page = 1
  await loadRecords()
}

const handleResetQuery = async () => {
  queryForm.channel = ""
  queryForm.status = ""
  queryForm.alarmType = ""
  queryForm.range = []
  pagination.page = 1
  await loadRecords()
}

const handleRetry = async (record: PushLogRecord) => {
  retryingId.value = record.id
  try {
    const result = await retryPushLogApi(record.id)
    ElMessage.success(`重试完成：${result.message}`)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "重试推送失败"))
  } finally {
    retryingId.value = null
  }
}

const handleExport = async () => {
  exporting.value = true
  try {
    const [startAt, endAt] = queryForm.range ?? []
    await exportPushLogsApi({
      channel: queryForm.channel || undefined,
      status: queryForm.status || undefined,
      alarm_type: queryForm.alarmType || undefined,
      start_at: startAt || undefined,
      end_at: endAt || undefined,
    })
  } finally {
    exporting.value = false
  }
}

const handlePageChange = async (page: number) => {
  pagination.page = page
  await loadRecords()
}

onMounted(async () => {
  await loadRecords()
})
</script>

<template>
  <div class="push-log-page unified-list-page">
    <PageCard
      class="push-log-page__filters-card"
    >
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.channel" v-refresh-on-empty="handleSearch">
            <option value="">渠道</option>
            <option v-for="item in channelOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.status" v-refresh-on-empty="handleSearch">
            <option value="">状态</option>
            <option v-for="item in statusOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.alarmType" v-refresh-on-empty="handleSearch">
            <option value="">告警类型</option>
            <option v-for="item in alarmTypeOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field push-log-page__range">
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
          <button class="app-button app-button--primary push-log-page__button unified-list-page__button unified-list-page__search-button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary push-log-page__button unified-list-page__button unified-list-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button class="app-button app-button--success push-log-page__button unified-list-page__button unified-list-page__search-button" :disabled="exporting" @click="handleExport">
            <el-icon><Download /></el-icon>
            <span>{{ exporting ? "导出中..." : "导出 Excel" }}</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="push-log-page__summary unified-list-page__summary">
      <article v-for="card in metrics" :key="card.label" class="push-log-page__metric unified-list-page__metric" :class="[`push-log-page__metric--${card.accent}`, `unified-list-page__metric--${card.accent}`]">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <PageCard>
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th style="width: 160px">配置名称</th>
            <th style="width: 250px">告警信息</th>
            <th style="width: 160px">厂区 / 区域</th>
            <th style="width: 80px">状态</th>
            <th style="width: 60px">触发方式</th>
            <th>消息</th>
            <th style="width: 70px">重试次数</th>
            <th style="width: 160px">推送时间</th>
            <th style="width: 80px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="9" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td style="width: 160px">
              <div class="push-log-page__stack unified-list-page__stack-cell">
                <strong>{{ record.configName || "-" }}</strong>
                <span>{{ getChannelText(record.channel) }} / {{ record.providerType }}</span>
              </div>
            </td>
            <td style="width: 180px">
              <div class="push-log-page__stack unified-list-page__stack-cell">
                <strong>{{ record.alarmType || "测试消息" }}</strong>
                <span>{{ record.alarmNo || "无关联告警" }}</span>
                <StatusTag v-if="record.alarmLevel" :text="getLevelText(record.alarmLevel)" :tone="getLevelTone(record.alarmLevel)" />
              </div>
            </td>
            <td style="width: 160px">
              <div class="push-log-page__stack unified-list-page__stack-cell">
                <strong>{{ record.factoryName || "-" }}</strong>
                <span>{{ record.zoneName || "-" }}</span>
              </div>
            </td>
            <td style="width: 100px"><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td style="width: 100px">{{ getTriggeredByText(record.triggeredBy) }}</td>
            <td style="width: 320px">
              <div class="push-log-page__message unified-list-page__message">
                <strong>{{ record.message }}</strong>
                <span v-if="record.errorMessage">错误：{{ record.errorMessage }}</span>
                <span v-else-if="record.responseBody">响应：{{ record.responseBody }}</span>
              </div>
            </td>
            <td style="width: 90px">{{ record.retryCount }}</td>
            <td style="width: 160px">{{ formatDateTime(record.pushedAt) }}</td>
            <td style="width: 120px">
              <button
                v-if="record.status !== 'success'"
                v-permission="'push:log:retry'"
                class="app-button app-button--warning push-log-page__button unified-list-page__button unified-list-page__table-button"
                :disabled="retryingId === record.id"
                @click="handleRetry(record)"
              >
                <el-icon><RefreshRight /></el-icon>
                <span>{{ retryingId === record.id ? "重试中..." : "重试" }}</span>
              </button>
              <span v-else>-</span>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="push-log-page__pagination">
        <el-pagination
          layout="total, prev, pager, next"
          :current-page="pagination.page"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          @current-change="handlePageChange"
        />
      </div>
    </PageCard>
  </div>
</template>

<style scoped>
.push-log-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.push-log-page__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.push-log-page__metric {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.push-log-page__metric span {
  color: #667b91;
  font-size: 13px;
}

.push-log-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
}

.push-log-page__metric--success strong {
  color: #1d9b52;
}

.push-log-page__metric--danger strong {
  color: #d64f5a;
}

.push-log-page__metric--info strong {
  color: #1d7ad9;
}

.push-log-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.push-log-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.push-log-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.push-log-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 160px 160px 180px minmax(320px, 1fr);
  gap: 10px;
  align-items: end;
}

.push-log-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.push-log-page__filters-card :deep(.app-field select),
.push-log-page__filters-card :deep(.app-field input),
.push-log-page__filters-card :deep(.el-input__wrapper) {
  font-size: 13px;
}

.push-log-page__filters-card :deep(.el-date-editor) {
  min-height: 36px;
}

.push-log-page__filters-card .push-log-page__button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.push-log-page__range {
  grid-column: auto;
}

.push-log-page__stack,
.push-log-page__message {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.push-log-page__stack strong,
.push-log-page__message strong {
  color: #163657;
}

.push-log-page__stack span,
.push-log-page__message span {
  color: #708398;
  font-size: 12px;
  line-height: 1.5;
}

.push-log-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

@media (max-width: 1100px) {
  .push-log-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .push-log-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }

  .push-log-page__summary {
    grid-template-columns: 1fr;
  }

  .push-log-page__range {
    grid-column: auto;
  }
}

@media (max-width: 768px) {
  .push-log-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
