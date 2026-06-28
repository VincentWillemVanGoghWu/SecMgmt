<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import { Download, RefreshRight, Search } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { exportDeviceStatusApi } from "../../api/export"
import { checkAllDevicesStatusApi, listDeviceStatusLogsApi } from "../../api/device-status"
import type { DeviceStatusCheckAllData, DeviceStatusLogRecord } from "../../types/device-status"

const loading = ref(false)
const checkingAll = ref(false)
const exporting = ref(false)
const records = ref<DeviceStatusLogRecord[]>([])
const lastCheckSummary = ref<DeviceStatusCheckAllData | null>(null)
const pagination = reactive({
  page: 1,
  pageSize: 30,
  total: 0,
})

const queryForm = reactive({
  deviceType: "",
  deviceName: "",
  status: "",
  range: [] as string[],
})

const deviceTypeOptions = [
  { label: "全部", value: "" },
  { label: "摄像机", value: "camera" },
  { label: "录像机", value: "recorder" },
  { label: "通道", value: "channel" },
]

const statusOptions = [
  { label: "全部", value: "" },
  { label: "在线", value: "online" },
  { label: "离线", value: "offline" },
  { label: "异常", value: "exception" },
  { label: "停用", value: "disabled" },
]

const metrics = computed(() => [
  { label: "日志条数", value: pagination.total, accent: "primary" },
  { label: "在线变化", value: records.value.filter((item) => item.newStatus === "online").length, accent: "success" },
  { label: "离线/异常", value: records.value.filter((item) => ["offline", "exception"].includes(item.newStatus)).length, accent: "danger" },
  { label: "最近检测设备", value: lastCheckSummary.value?.checkedDevices ?? 0, accent: "info" },
])

const getStatusText = (status: string | null | undefined) => {
  if (!status) return "-"
  if (status === "online") return "在线"
  if (status === "offline") return "离线"
  if (status === "exception") return "异常"
  if (status === "disabled") return "停用"
  return status
}

const getStatusTone = (status: string | null | undefined) => {
  if (status === "online") return "success"
  if (status === "exception") return "danger"
  if (status === "disabled") return "warning"
  if (status === "offline") return "default"
  return "default"
}

const getDeviceTypeText = (value: string) => {
  if (value === "camera") return "摄像机"
  if (value === "recorder") return "录像机"
  if (value === "channel") return "通道"
  return value
}

const formatDateTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", { hour12: false })
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
    const [startAt, endAt] = queryForm.range ?? []
    const result = await listDeviceStatusLogsApi({
      device_type: queryForm.deviceType || undefined,
      device_name: queryForm.deviceName || undefined,
      status: queryForm.status || undefined,
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
  queryForm.deviceType = ""
  queryForm.deviceName = ""
  queryForm.status = ""
  queryForm.range = []
  pagination.page = 1
  await loadRecords()
}

const handleCheckAll = async () => {
  checkingAll.value = true
  try {
    lastCheckSummary.value = await checkAllDevicesStatusApi()
    ElMessage.success(lastCheckSummary.value.message)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "执行全部检测失败"))
  } finally {
    checkingAll.value = false
  }
}

const handleExport = async () => {
  exporting.value = true
  try {
    const [startAt, endAt] = queryForm.range ?? []
    await exportDeviceStatusApi({
      device_type: queryForm.deviceType || undefined,
      device_name: queryForm.deviceName || undefined,
      status: queryForm.status || undefined,
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
  <div class="device-page">
    <PageCard class="device-page__filters-card">
      <SearchForm class="device-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.deviceType">
            <option value="">设备类型</option>
            <option v-for="item in deviceTypeOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.status">
            <option value="">状态</option>
            <option v-for="item in statusOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field device-page__keyword">
          <input v-model="queryForm.deviceName" type="text" placeholder="输入设备名称关键字" />
        </div>
        <div class="app-field device-page__range">
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
          <button class="app-button app-button--primary device-page__button device-page__search-button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary device-page__button device-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'device:status:check'"
            class="app-button app-button--success device-page__button device-page__search-button"
            :disabled="checkingAll"
            @click="handleCheckAll"
          >
            <el-icon><RefreshRight /></el-icon>
            <span>{{ checkingAll ? "检测中..." : "全部检测" }}</span>
          </button>
          <button class="app-button app-button--warning device-page__button device-page__search-button" :disabled="exporting" @click="handleExport">
            <el-icon><Download /></el-icon>
            <span>{{ exporting ? "导出中..." : "导出 Excel" }}</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="device-page__summary">
      <article v-for="card in metrics" :key="card.label" class="device-page__metric" :class="`device-page__metric--${card.accent}`">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <PageCard :description="`当前查询到 ${pagination.total} 条设备状态日志，每页显示 ${pagination.pageSize} 条。`">
      <table class="app-table device-page__table">
        <colgroup>
          <col class="device-page__col-type" />
          <col class="device-page__col-name" />
          <col class="device-page__col-status" />
          <col class="device-page__col-status" />
          <col class="device-page__col-message" />
          <col class="device-page__col-time" />
        </colgroup>
        <thead>
          <tr>
            <th>设备类型</th>
            <th>设备名称</th>
            <th>原状态</th>
            <th>新状态</th>
            <th>检测说明</th>
            <th>检测时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="6" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>{{ getDeviceTypeText(record.deviceType) }}</td>
            <td>
              <div class="device-page__name-cell">
                <strong>{{ record.deviceName }}</strong>
                <span>ID {{ record.deviceId }}</span>
              </div>
            </td>
            <td><StatusTag :text="getStatusText(record.oldStatus)" :tone="getStatusTone(record.oldStatus)" /></td>
            <td><StatusTag :text="getStatusText(record.newStatus)" :tone="getStatusTone(record.newStatus)" /></td>
            <td class="device-page__message">{{ record.message || "-" }}</td>
            <td>{{ formatDateTime(record.checkedAt) }}</td>
          </tr>
        </tbody>
      </table>
      <div class="device-page__pagination">
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
.device-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}


.device-page__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.device-page__metric {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.device-page__metric span {
  color: #667b91;
  font-size: 13px;
}

.device-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
}

.device-page__metric--success strong {
  color: #1d9b52;
}

.device-page__metric--danger strong {
  color: #d64f5a;
}

.device-page__metric--info strong {
  color: #1d7ad9;
}

.device-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.device-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.device-page__search-form {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.device-page__search-form :deep(.search-form__fields) {
  grid-template-columns: 140px 140px minmax(220px, 300px) minmax(280px, 1fr);
  gap: 10px;
  align-items: end;
}

.device-page__search-form :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.device-page__search-form :deep(.app-field select),
.device-page__search-form :deep(.app-field input),
.device-page__search-form :deep(.el-date-editor) {
  width: 100%;
  height: 36px;
  font-size: 13px;
}

.device-page__search-button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.device-page__table {
  table-layout: fixed;
}

.device-page__table th,
.device-page__table td {
  padding: 9px 10px;
  font-size: 12px;
  vertical-align: middle;
}

.device-page__table th {
  font-size: 12px;
  white-space: nowrap;
}

.device-page__name-cell {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.device-page__name-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.device-page__name-cell span {
  color: #708398;
  font-size: 11px;
  line-height: 1.4;
}

.device-page__col-type {
  width: 80px;
}

.device-page__col-name {
  width: 170px;
}

.device-page__col-status {
  width: 84px;
}

.device-page__col-message {
  width: auto;
}

.device-page__col-time {
  width: 165px;
}

.device-page__message {
  color: #48607a;
  line-height: 1.4;
}

.device-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.device-page__keyword,
.device-page__range {
  grid-column: auto;
}

@media (max-width: 1100px) {
  .device-page__summary {
    grid-template-columns: 1fr;
  }

  .device-page__search-form {
    grid-template-columns: 1fr;
  }

  .device-page__search-form :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }

  .device-page__search-form :deep(.search-form__actions) {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}

@media (max-width: 768px) {
  .device-page__search-form :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
