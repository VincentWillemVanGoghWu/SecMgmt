<script setup lang="ts">
import { onMounted, reactive, ref, watch } from "vue"
import { useRoute } from "vue-router"
import { ElMessage } from "element-plus"
import { RefreshRight, Search, View } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import AlarmDetailDialog from "../../components/alarm/AlarmDetailDialog.vue"
import AlarmPlaybackDialog from "../../components/alarm/AlarmPlaybackDialog.vue"
import { getAlarmDetailApi, listAlarmsApi, processAlarmApi } from "../../api/alarm"
import type { AlarmDetail, AlarmRecord } from "../../types/alarm"
import { formatDateTime } from "../../utils/datetime"

const loading = ref(false)
const detailLoading = ref(false)
const records = ref<AlarmRecord[]>([])
const detailVisible = ref(false)
const activeDetail = ref<AlarmDetail | null>(null)
const playbackVisible = ref(false)
const activePlaybackAlarm = ref<AlarmRecord | null>(null)
const processingId = ref<number | null>(null)
const hasSearched = ref(false)
const route = useRoute()
const pagination = reactive({
  page: 1,
  pageSize: 12,
  total: 0,
})

const formatDateValue = (value: Date) => {
  const year = value.getFullYear()
  const month = String(value.getMonth() + 1).padStart(2, "0")
  const day = String(value.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

const getDefaultDateRange = () => {
  const now = new Date()
  const startDate = new Date(now.getFullYear(), now.getMonth(), 1)
  const endDate = new Date(now.getFullYear(), now.getMonth() + 1, 0)
  return [formatDateValue(startDate), formatDateValue(endDate)]
}

const queryForm = reactive({
  keyword: "",
  status: "",
  level: "",
  range: getDefaultDateRange(),
})

const statusOptions = [
  { label: "未处理", value: "pending" },
  { label: "处理中", value: "processing" },
  { label: "已处理", value: "done" },
  { label: "误报", value: "false_alarm" },
]

const levelOptions = [
  { label: "严重", value: "critical" },
  { label: "高", value: "high" },
  { label: "中", value: "medium" },
  { label: "低", value: "low" },
]

const getStatusText = (value?: string | null) => {
  if (value === "pending") return "未处理"
  if (value === "processing") return "处理中"
  if (value === "done") return "已处理"
  if (value === "false_alarm") return "误报"
  return value || "-"
}

const getStatusTone = (value?: string | null) => {
  if (value === "pending") return "danger"
  if (value === "processing") return "warning"
  if (value === "done") return "success"
  if (value === "false_alarm") return "default"
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

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const buildParams = () => {
  const [startDate, endDate] = queryForm.range ?? []
  return {
    keyword: queryForm.keyword || undefined,
    status: queryForm.status || undefined,
    level: queryForm.level || undefined,
    start_at: startDate ? `${startDate}T00:00:00` : undefined,
    end_at: endDate ? `${endDate}T23:59:59` : undefined,
    page: pagination.page,
    page_size: pagination.pageSize,
  }
}

const normalizeDateValue = (value: string) => value.trim().slice(0, 10)

const clearRecords = () => {
  records.value = []
  pagination.total = 0
}

const loadRecords = async () => {
  if (!hasSearched.value) {
    clearRecords()
    return
  }
  loading.value = true
  try {
    const result = await listAlarmsApi(buildParams())
    records.value = result.items
    pagination.total = result.total
    pagination.page = result.page
    pagination.pageSize = result.pageSize
  } finally {
    loading.value = false
  }
}

const applyRouteQuery = () => {
  queryForm.keyword = typeof route.query.keyword === "string" ? route.query.keyword : ""
  queryForm.status = typeof route.query.status === "string" ? route.query.status : ""
  queryForm.level = typeof route.query.level === "string" ? route.query.level : ""
  const startDate =
    typeof route.query.startAt === "string"
      ? route.query.startAt
      : typeof route.query.start_at === "string"
        ? route.query.start_at
        : ""
  const endDate =
    typeof route.query.endAt === "string"
      ? route.query.endAt
      : typeof route.query.end_at === "string"
        ? route.query.end_at
        : ""
  queryForm.range = startDate && endDate ? [normalizeDateValue(startDate), normalizeDateValue(endDate)] : getDefaultDateRange()
  hasSearched.value = Boolean(queryForm.keyword || queryForm.status || queryForm.level || (startDate && endDate))
  pagination.page = 1
}

const handleSearch = async () => {
  hasSearched.value = true
  pagination.page = 1
  await loadRecords()
}

const resetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  queryForm.level = ""
  queryForm.range = getDefaultDateRange()
  hasSearched.value = false
  pagination.page = 1
  clearRecords()
}

const openDetail = async (record: AlarmRecord) => {
  detailLoading.value = true
  try {
    activeDetail.value = await getAlarmDetailApi(record.id)
    detailVisible.value = true
  } finally {
    detailLoading.value = false
  }
}

const handleDialogRefreshed = async () => {
  if (activeDetail.value) {
    activeDetail.value = await getAlarmDetailApi(activeDetail.value.id)
  }
  await loadRecords()
}

const handleMarkDone = async (record: AlarmRecord) => {
  processingId.value = record.id
  try {
    await processAlarmApi(record.id, {
      status: "done",
      remark: "列表快捷标记为已处理",
    })
    await loadRecords()
    ElMessage.success("已标记为已处理")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "标记已处理失败"))
  } finally {
    processingId.value = null
  }
}

const canViewPlayback = (record: AlarmRecord) =>
  Boolean(record.channelId && record.recordStartTime && record.recordEndTime)

const openPlayback = (record: AlarmRecord) => {
  if (!canViewPlayback(record)) {
    ElMessage.warning("当前告警缺少关联录像通道或时间段")
    return
  }
  activePlaybackAlarm.value = record
  playbackVisible.value = true
}

const handlePageChange = async (page: number) => {
  pagination.page = page
  await loadRecords()
}

onMounted(async () => {
  applyRouteQuery()
  await loadRecords()
})

watch(
  () => route.query,
  async () => {
    applyRouteQuery()
    await loadRecords()
  },
)
</script>

<template>
  <div class="alarm-query-page unified-list-page">
    <PageCard
      class="alarm-query-page__filters-card"
    >
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <input v-model="queryForm.keyword" type="text" placeholder="告警编号 / 类型 / 设备 / 区域" />
        </div>
        <div class="app-field">
          <select v-model="queryForm.status">
            <option value="" hidden>告警状态</option>
            <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.level">
            <option value="" hidden>告警等级</option>
            <option v-for="item in levelOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field alarm-query-page__field--range">
          <el-date-picker
            v-model="queryForm.range"
            type="daterange"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            value-format="YYYY-MM-DD"
            class="alarm-query-page__date-range"
          />
        </div>
        <div class="alarm-query-page__filter-actions">
          <button class="app-button app-button--primary alarm-query-page__button unified-list-page__button unified-list-page__search-button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary alarm-query-page__button unified-list-page__button unified-list-page__search-button" @click="resetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
        </div>
      </SearchForm>
    </PageCard>

    <PageCard>
      <table class="app-table alarm-query-page__table unified-list-page__table">
        <colgroup>
          <col class="alarm-query-page__col-no" />
          <col class="alarm-query-page__col-type" />
          <col class="alarm-query-page__col-level" />
          <col class="alarm-query-page__col-status" />
          <col class="alarm-query-page__col-device" />
          <col class="alarm-query-page__col-area" />
          <col class="alarm-query-page__col-count" />
          <col class="alarm-query-page__col-time" />
          <col class="alarm-query-page__col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>告警编号</th>
            <th>类型</th>
            <th>等级</th>
            <th>状态</th>
            <th>设备</th>
            <th>区域</th>
            <th>重复次数</th>
            <th>告警时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="9" class="app-table__empty">{{ loading ? "加载中..." : "暂无告警" }}</td>
          </tr>
          <tr v-for="item in records" :key="item.id">
            <td>{{ item.alarmNo }}</td>
            <td>
              <div class="alarm-query-page__name-cell unified-list-page__name-cell">
                <strong>{{ item.alarmType }}</strong>
              </div>
            </td>
            <td><StatusTag :text="getLevelText(item.alarmLevel)" :tone="getLevelTone(item.alarmLevel)" /></td>
            <td><StatusTag :text="getStatusText(item.status)" :tone="getStatusTone(item.status)" /></td>
            <td>
              <div class="alarm-query-page__name-cell unified-list-page__name-cell">
                <strong>{{ item.cameraName || item.channelName || "-" }}</strong>
              </div>
            </td>
            <td>
              <div class="alarm-query-page__name-cell unified-list-page__name-cell">
                <strong>{{ item.factoryName || "-" }}</strong>
              </div>
            </td>
            <td>{{ item.occurrenceCount }}</td>
            <td>{{ formatDateTime(item.alarmTime) }}</td>
            <td>
              <div class="table-actions">
                <button class="app-button app-button--primary alarm-query-page__button alarm-query-page__table-button unified-list-page__button unified-list-page__table-button" @click="openDetail(item)">
                  <el-icon><View /></el-icon>
                  <span>详情</span>
                </button>
                <button
                  v-permission="'alarm:process'"
                  class="app-button app-button--success alarm-query-page__button alarm-query-page__table-button unified-list-page__button unified-list-page__table-button"
                  :disabled="processingId === item.id || item.status === 'done'"
                  @click="handleMarkDone(item)"
                >
                  <span>{{ processingId === item.id ? "处理中..." : "标记已处理" }}</span>
                </button>
                <button
                  class="app-button app-button--secondary alarm-query-page__button alarm-query-page__table-button unified-list-page__button unified-list-page__table-button"
                  :disabled="!canViewPlayback(item)"
                  @click="openPlayback(item)"
                >
                  <span>录像查看</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
      <div class="alarm-query-page__pagination">
        <el-pagination
          layout="total, prev, pager, next"
          :current-page="pagination.page"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          @current-change="handlePageChange"
        />
      </div>
    </PageCard>

    <AlarmDetailDialog
      v-model="detailVisible"
      :detail="activeDetail"
      :loading="detailLoading"
      @refreshed="handleDialogRefreshed"
    />
    <AlarmPlaybackDialog v-model="playbackVisible" :alarm="activePlaybackAlarm" />
  </div>
</template>

<style scoped>
.alarm-query-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.alarm-query-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.alarm-query-page__name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.alarm-query-page__name-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.alarm-query-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.alarm-query-page__filters-card :deep(.search-form) {
  display: block;
}

.alarm-query-page__filters-card :deep(.search-form__fields) {
  width: 100%;
  grid-template-columns: minmax(240px, 1.3fr) 140px 140px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.alarm-query-page__filters-card :deep(.search-form__fields > *) {
  min-width: 0;
}

.alarm-query-page__filter-actions {
  display: flex;
  flex-wrap: nowrap;
  justify-content: flex-end;
  gap: 10px;
  align-items: end;
}

.alarm-query-page__filters-card :deep(.app-field input),
.alarm-query-page__filters-card :deep(.app-field select) {
  height: 36px;
  font-size: 13px;
}

.alarm-query-page__date-range {
  width: 100% !important;
}

.alarm-query-page__filters-card :deep(.alarm-query-page__date-range.el-date-editor) {
  width: 100% !important;
}

.alarm-query-page__field--range {
  min-width: 0;
}

.alarm-query-page__table {
  table-layout: fixed;
}

.alarm-query-page__table th,
.alarm-query-page__table td {
  padding: 9px 10px;
  font-size: 12px;
  vertical-align: middle;
}

.alarm-query-page__table th {
  font-size: 12px;
  white-space: nowrap;
}

.alarm-query-page__table td:nth-child(1),
.alarm-query-page__table td:nth-child(3),
.alarm-query-page__table td:nth-child(4),
.alarm-query-page__table td:nth-child(7) {
  white-space: nowrap;
}

.alarm-query-page__col-no {
  width: 148px;
}

.alarm-query-page__col-type {
  width: 180px;
}

.alarm-query-page__col-level {
  width: 74px;
}

.alarm-query-page__col-status {
  width: 82px;
}

.alarm-query-page__col-device {
  width: 140px;
}

.alarm-query-page__col-area {
  width: 140px;
}

.alarm-query-page__col-count {
  width: 82px;
}

.alarm-query-page__col-time {
  width: 150px;
}

.alarm-query-page__col-actions {
  width: 236px;
}

.alarm-query-page__table .table-actions {
  flex-wrap: nowrap;
  gap: 4px;
}

.alarm-query-page__table-button {
  min-height: 30px;
  padding: 0 9px;
  font-size: 11px;
  line-height: 1;
  white-space: nowrap;
}

.alarm-query-page__table-button :deep(.el-icon) {
  font-size: 11px;
}

.alarm-query-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

@media (max-width: 1100px) {
  .alarm-query-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }

  .alarm-query-page__table .table-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 768px) {
  .alarm-query-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }

  .alarm-query-page__filter-actions {
    justify-content: flex-start;
  }
}
</style>
