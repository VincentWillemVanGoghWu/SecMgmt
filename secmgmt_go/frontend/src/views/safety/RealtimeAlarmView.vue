<script setup lang="ts">
import { onMounted, onUnmounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import { RefreshRight, Search, View } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import AlarmDetailDialog from "../../components/alarm/AlarmDetailDialog.vue"
import AlarmPlaybackDialog from "../../components/alarm/AlarmPlaybackDialog.vue"
import { getAlarmDetailApi, listRealtimeAlarmsApi, processAlarmApi } from "../../api/alarm"
import { useRealtimeStore } from "../../stores"
import type { AlarmDetail, AlarmRecord } from "../../types/alarm"
import type { AlarmRealtimeEvent } from "../../types/realtime"
import { formatDateTime } from "../../utils/datetime"
import { mapRealtimeEventToAlarmRecord, prependRealtimeAlarm } from "../../utils/realtime"

const loading = ref(false)
const detailLoading = ref(false)
const records = ref<AlarmRecord[]>([])
const detailVisible = ref(false)
const activeDetail = ref<AlarmDetail | null>(null)
const playbackVisible = ref(false)
const activePlaybackAlarm = ref<AlarmRecord | null>(null)
const processingId = ref<number | null>(null)
const realtimeStore = useRealtimeStore()
let unsubscribe: (() => void) | null = null
const pagination = reactive({
  page: 1,
  pageSize: 12,
  total: 0,
})

const queryForm = reactive({
  keyword: "",
  status: "",
  level: "",
})

const statusOptions = [
  { label: "未处理", value: "pending" },
  { label: "处理中", value: "processing" },
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

const loadRecords = async () => {
  loading.value = true
  try {
    const result = await listRealtimeAlarmsApi({
      keyword: queryForm.keyword || undefined,
      status: queryForm.status || undefined,
      level: queryForm.level || undefined,
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

const matchesRealtimeFilter = (record: AlarmRecord) => {
  const keyword = queryForm.keyword.trim().toLowerCase()
  const matchesKeyword =
    !keyword ||
    [
      record.alarmNo,
      record.alarmType,
      record.cameraName,
      record.channelName,
      record.factoryName,
      record.zoneName,
      record.message,
    ]
      .filter(Boolean)
      .some((value) => String(value).toLowerCase().includes(keyword))
  const matchesStatus = !queryForm.status || record.status === queryForm.status
  const matchesLevel = !queryForm.level || record.alarmLevel === queryForm.level
  return matchesKeyword && matchesStatus && matchesLevel
}

const handleRealtimeAlarm = (event: AlarmRealtimeEvent) => {
  const nextRecord = mapRealtimeEventToAlarmRecord(event)
  if (pagination.page !== 1 || !matchesRealtimeFilter(nextRecord)) {
    return
  }
  records.value = prependRealtimeAlarm(records.value, event).slice(0, pagination.pageSize)
  pagination.total += 1
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
    records.value = records.value.filter((item) => item.id !== record.id)
    pagination.total = Math.max(0, pagination.total - 1)
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

const handleSearch = async () => {
  pagination.page = 1
  await loadRecords()
}

const handlePageChange = async (page: number) => {
  pagination.page = page
  await loadRecords()
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.status = ""
  queryForm.level = ""
  pagination.page = 1
  await loadRecords()
}

onMounted(async () => {
  await loadRecords()
  unsubscribe = realtimeStore.subscribe(handleRealtimeAlarm)
})

onUnmounted(() => {
  unsubscribe?.()
  unsubscribe = null
})
</script>

<template>
  <div class="realtime-alarm-page unified-list-page">
    <PageCard
      class="realtime-alarm-page__filters-card"
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
        <template #actions>
          <button class="app-button app-button--primary realtime-alarm-page__button unified-list-page__button unified-list-page__search-button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary realtime-alarm-page__button unified-list-page__button unified-list-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button class="app-button app-button--warning realtime-alarm-page__button unified-list-page__button unified-list-page__search-button" @click="loadRecords">
            <el-icon><RefreshRight /></el-icon>
            <span>{{ loading ? "刷新中..." : "手动刷新" }}</span>
          </button>
          <span class="realtime-alarm-page__status" :class="`realtime-alarm-page__status--${realtimeStore.connectionStatus}`">
            {{ realtimeStore.connectionLabel }}
          </span>
        </template>
      </SearchForm>
    </PageCard>

    <PageCard>
      <table class="app-table realtime-alarm-page__table unified-list-page__table">
        <colgroup>
          <col class="realtime-alarm-page__col-no" />
          <col class="realtime-alarm-page__col-type" />
          <col class="realtime-alarm-page__col-level" />
          <col class="realtime-alarm-page__col-status" />
          <col class="realtime-alarm-page__col-device" />
          <col class="realtime-alarm-page__col-area" />
          <col class="realtime-alarm-page__col-count" />
          <col class="realtime-alarm-page__col-time" />
          <col class="realtime-alarm-page__col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>告警编号</th>
            <th >类型</th>
            <th>等级</th>
            <th>状态</th>
            <th>设备</th>
            <th>区域</th>
            <th>次数</th>
            <th>时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="9" class="app-table__empty">{{ loading ? "加载中..." : "暂无实时告警" }}</td>
          </tr>
          <tr v-for="item in records" :key="item.id">
            <td>{{ item.alarmNo }}</td>
            <td>
              <div class="realtime-alarm-page__name-cell unified-list-page__name-cell">
                <strong>{{ item.alarmType }}</strong>
              </div>
            </td>
            <td><StatusTag :text="getLevelText(item.alarmLevel)" :tone="getLevelTone(item.alarmLevel)" /></td>
            <td><StatusTag :text="getStatusText(item.status)" :tone="getStatusTone(item.status)" /></td>
            <td>
              <div class="realtime-alarm-page__name-cell unified-list-page__name-cell">
                <strong>{{ item.cameraName || item.channelName || "-" }}</strong>
              </div>
            </td>
            <td>
              <div class="realtime-alarm-page__name-cell unified-list-page__name-cell">
                <strong>{{ item.factoryName || "-" }}</strong>
              </div>
            </td>
            <td>{{ item.occurrenceCount }}</td>
            <td>{{ formatDateTime(item.alarmTime) }}</td>
            <td>
              <div class="table-actions">
                <button class="app-button app-button--primary realtime-alarm-page__button realtime-alarm-page__table-button unified-list-page__button unified-list-page__table-button" @click="openDetail(item)">
                  <el-icon><View /></el-icon>
                  <span>详情</span>
                </button>
                <button
                  v-permission="'alarm:process'"
                  class="app-button app-button--success realtime-alarm-page__button realtime-alarm-page__table-button unified-list-page__button unified-list-page__table-button"
                  :disabled="processingId === item.id || item.status === 'done'"
                  @click="handleMarkDone(item)"
                >
                  <span>{{ processingId === item.id ? "处理中..." : "标记已处理" }}</span>
                </button>
                <button
                  class="app-button app-button--secondary realtime-alarm-page__button realtime-alarm-page__table-button unified-list-page__button unified-list-page__table-button"
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
      <div class="realtime-alarm-page__pagination">
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
.realtime-alarm-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.realtime-alarm-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.realtime-alarm-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.realtime-alarm-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: minmax(240px, 1.2fr) 160px 160px;
  gap: 10px;
  align-items: end;
}

.realtime-alarm-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.realtime-alarm-page__filters-card :deep(.app-field input),
.realtime-alarm-page__filters-card :deep(.app-field select) {
  height: 36px;
  font-size: 13px;
}

.realtime-alarm-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.realtime-alarm-page__status {
  display: inline-flex;
  align-items: center;
  min-height: 36px;
  padding: 0 12px;
  border-radius: 999px;
  background: #eef4fb;
  color: #466179;
  font-size: 12px;
}

.realtime-alarm-page__status--connected {
  background: #e6f7ee;
  color: #157347;
}

.realtime-alarm-page__status--connecting,
.realtime-alarm-page__status--reconnecting {
  background: #fff4de;
  color: #9a6700;
}

.realtime-alarm-page__name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.realtime-alarm-page__name-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.realtime-alarm-page__name-cell span {
  color: #708398;
  font-size: 11px;
  line-height: 1.4;
}

.realtime-alarm-page__table {
  table-layout: fixed;
}

.realtime-alarm-page__table th,
.realtime-alarm-page__table td {
  padding: 9px 10px;
  font-size: 12px;
  vertical-align: middle;
}

.realtime-alarm-page__table th {
  font-size: 12px;
  white-space: nowrap;
}

.realtime-alarm-page__table td:nth-child(1),
.realtime-alarm-page__table td:nth-child(3),
.realtime-alarm-page__table td:nth-child(4),
.realtime-alarm-page__table td:nth-child(7) {
  white-space: nowrap;
}

.realtime-alarm-page__col-no {
  width: 148px;
}

.realtime-alarm-page__col-type {
  width: 180px;
}

.realtime-alarm-page__col-level {
  width: 74px;
}

.realtime-alarm-page__col-status {
  width: 82px;
}

.realtime-alarm-page__col-device {
  width: 140px;
}

.realtime-alarm-page__col-area {
  width: 140px;
}

.realtime-alarm-page__col-count {
  width: 72px;
}

.realtime-alarm-page__col-time {
  width: 150px;
}

.realtime-alarm-page__col-actions {
  width: 236px;
}

.realtime-alarm-page__table .table-actions {
  flex-wrap: nowrap;
  gap: 4px;
}

.realtime-alarm-page__table-button {
  min-height: 30px;
  padding: 0 9px;
  font-size: 11px;
  line-height: 1;
  white-space: nowrap;
}

.realtime-alarm-page__table-button :deep(.el-icon) {
  font-size: 11px;
}

.realtime-alarm-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

@media (max-width: 1100px) {
  .realtime-alarm-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .realtime-alarm-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr 1fr;
  }

  .realtime-alarm-page__table .table-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 768px) {
  .realtime-alarm-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
