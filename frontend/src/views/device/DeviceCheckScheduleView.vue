<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus"
import { Delete, Edit, Plus, RefreshRight, VideoPlay } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import {
  createDeviceCheckScheduleApi,
  deleteDeviceCheckScheduleApi,
  listDeviceCheckRunsApi,
  listDeviceCheckSchedulesApi,
  runDeviceCheckScheduleApi,
  updateDeviceCheckScheduleApi,
  updateDeviceCheckScheduleStatusApi,
} from "../../api/device-status"
import { listPushConfigsApi } from "../../api/push"
import type {
  DeviceCheckNotifyMode,
  DeviceCheckRunRecord,
  DeviceCheckSchedulePayload,
  DeviceCheckScheduleRecord,
} from "../../types/device-status"
import type { StatusTone } from "../../types/navigation"
import type { PushConfigRecord } from "../../types/push"

interface ScheduleFormState {
  name: string
  enabled: boolean
  frequencyPerDay: number
  notifyEnabled: boolean
  pushConfigIds: number[]
  notifyMode: DeviceCheckNotifyMode
}

interface MetricCard {
  label: string
  value: number
  note: string
  tone: StatusTone
}

const loading = ref(false)
const saving = ref(false)
const runningId = ref<number | null>(null)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()
const schedules = ref<DeviceCheckScheduleRecord[]>([])
const runs = ref<DeviceCheckRunRecord[]>([])
const pushConfigs = ref<PushConfigRecord[]>([])
const runPagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0,
})

const formState = reactive<ScheduleFormState>({
  name: "",
  enabled: true,
  frequencyPerDay: 1,
  notifyEnabled: true,
  pushConfigIds: [],
  notifyMode: "offline_changed",
})

const rules: FormRules<ScheduleFormState> = {
  name: [{ required: true, message: "请输入计划名称", trigger: "blur" }],
  frequencyPerDay: [{ required: true, message: "请选择每天检测次数", trigger: "change" }],
}

const frequencyOptions = [
  { label: "每天 1 次", value: 1 },
  { label: "每天 2 次", value: 2 },
  { label: "每天 4 次", value: 4 },
  { label: "每天 6 次", value: 6 },
  { label: "每天 8 次", value: 8 },
  { label: "每天 12 次", value: 12 },
  { label: "每天 24 次", value: 24 },
]

const notifyModeOptions = [
  { label: "只推送新离线", value: "offline_changed" },
  { label: "每次离线都推送", value: "offline_each_run" },
]

const emailPushConfigs = computed(() => pushConfigs.value.filter((item) => item.providerType === "email" && item.enabled))
const enabledSchedules = computed(() => schedules.value.filter((item) => item.enabled))
const latestRun = computed(() => runs.value[0] ?? null)
const nextSchedule = computed(() => {
  return [...enabledSchedules.value]
    .filter((item) => item.nextRunAt)
    .sort((left, right) => new Date(left.nextRunAt || 0).getTime() - new Date(right.nextRunAt || 0).getTime())[0]
})

const metrics = computed<MetricCard[]>(() => [
  {
    label: "巡检计划",
    value: schedules.value.length,
    note: `${enabledSchedules.value.length} 个启用`,
    tone: "info",
  },
  {
    label: "启用计划",
    value: enabledSchedules.value.length,
    note: schedules.value.length ? "按计划自动执行" : "等待创建计划",
    tone: "success",
  },
  {
    label: "最近离线",
    value: latestRun.value?.offlineTotal ?? 0,
    note: latestRun.value ? formatDateTime(latestRun.value.startedAt) : "暂无记录",
    tone: latestRun.value?.offlineTotal ? "danger" : "default",
  },
  {
    label: "最近变化",
    value: latestRun.value?.changedTotal ?? 0,
    note: latestRun.value ? `${latestRun.value.checkedTotal} 个设备已检测` : "暂无记录",
    tone: latestRun.value?.changedTotal ? "warning" : "default",
  },
])

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  })
}

const formatFrequency = (value: number) => `每天 ${value} 次`

const formatInterval = (frequency: number) => {
  if (!frequency) return "-"
  const minutes = Math.round((24 * 60) / frequency)
  if (minutes < 60) return `${minutes} 分钟/次`
  const hours = Math.floor(minutes / 60)
  const restMinutes = minutes % 60
  return restMinutes ? `${hours} 小时 ${restMinutes} 分钟/次` : `${hours} 小时/次`
}

const getRunStatusText = (status: string) => {
  if (status === "success") return "成功"
  if (status === "failed") return "失败"
  if (status === "running") return "运行中"
  return status
}

const getRunStatusTone = (status: string): StatusTone => {
  if (status === "success") return "success"
  if (status === "failed") return "danger"
  if (status === "running") return "warning"
  return "default"
}

const getNotifyModeText = (mode: DeviceCheckNotifyMode) =>
  notifyModeOptions.find((item) => item.value === mode)?.label ?? mode

const getScheduleStatusText = (record: DeviceCheckScheduleRecord) => {
  if (!record.enabled) return "停用"
  if (record.lastError) return "异常"
  return "启用"
}

const getScheduleStatusTone = (record: DeviceCheckScheduleRecord): StatusTone => {
  if (!record.enabled) return "default"
  if (record.lastError) return "danger"
  return "success"
}

const getPushConfigNames = (record: DeviceCheckScheduleRecord) => {
  if (!record.notifyEnabled) return "未启用邮件"
  const names = record.pushConfigIds
    .map((id) => emailPushConfigs.value.find((item) => item.id === id)?.configName)
    .filter((name): name is string => Boolean(name))
  return names.length ? names.join("、") : "未选择配置"
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string; message?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  const message = (error as { response?: { data?: { message?: string } } })?.response?.data?.message
  if (typeof message === "string" && message) return message
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const resetForm = () => {
  editingId.value = null
  formState.name = "全局设备巡检"
  formState.enabled = true
  formState.frequencyPerDay = 1
  formState.notifyEnabled = true
  formState.pushConfigIds = emailPushConfigs.value.slice(0, 1).map((item) => item.id)
  formState.notifyMode = "offline_changed"
  formRef.value?.clearValidate()
}

const loadSchedules = async () => {
  schedules.value = await listDeviceCheckSchedulesApi()
}

const loadRuns = async () => {
  const result = await listDeviceCheckRunsApi({
    page: runPagination.page,
    page_size: runPagination.pageSize,
  })
  runs.value = result.items
  runPagination.total = result.total
  runPagination.page = result.page
  runPagination.pageSize = result.pageSize
}

const loadPage = async () => {
  loading.value = true
  try {
    const [configItems] = await Promise.all([
      listPushConfigsApi({ provider_type: "email", enabled: true }),
      loadSchedules(),
      loadRuns(),
    ])
    pushConfigs.value = configItems
  } finally {
    loading.value = false
  }
}

const openCreateDialog = () => {
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = (record: DeviceCheckScheduleRecord) => {
  editingId.value = record.id
  formState.name = record.name
  formState.enabled = record.enabled
  formState.frequencyPerDay = record.frequencyPerDay
  formState.notifyEnabled = record.notifyEnabled
  formState.pushConfigIds = [...record.pushConfigIds]
  formState.notifyMode = record.notifyMode
  formRef.value?.clearValidate()
  dialogVisible.value = true
}

const buildPayload = (): DeviceCheckSchedulePayload => ({
  name: formState.name.trim(),
  enabled: formState.enabled,
  frequencyPerDay: formState.frequencyPerDay,
  notifyEnabled: formState.notifyEnabled,
  pushConfigIds: formState.notifyEnabled ? formState.pushConfigIds : [],
  notifyMode: formState.notifyMode,
})

const handleSave = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (formState.notifyEnabled && formState.pushConfigIds.length === 0) {
    ElMessage.warning("请选择邮件推送配置")
    return
  }

  saving.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await updateDeviceCheckScheduleApi(editingId.value, payload)
      ElMessage.success("巡检计划已更新")
    } else {
      await createDeviceCheckScheduleApi(payload)
      ElMessage.success("巡检计划已创建")
    }
    dialogVisible.value = false
    await loadSchedules()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存巡检计划失败"))
  } finally {
    saving.value = false
  }
}

const handleToggleStatus = async (record: DeviceCheckScheduleRecord) => {
  try {
    await updateDeviceCheckScheduleStatusApi(record.id, !record.enabled)
    ElMessage.success(record.enabled ? "巡检计划已停用" : "巡检计划已启用")
    await loadSchedules()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新状态失败"))
  }
}

const handleRunNow = async (record: DeviceCheckScheduleRecord) => {
  runningId.value = record.id
  try {
    const result = await runDeviceCheckScheduleApi(record.id)
    ElMessage.success(result.message || "巡检执行完成")
    await Promise.all([loadSchedules(), loadRuns()])
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "执行巡检失败"))
  } finally {
    runningId.value = null
  }
}

const handleDelete = async (record: DeviceCheckScheduleRecord) => {
  try {
    await ElMessageBox.confirm(`确定删除巡检计划「${record.name}」吗？`, "删除确认", { type: "warning" })
    await deleteDeviceCheckScheduleApi(record.id)
    ElMessage.success("巡检计划已删除")
    await loadSchedules()
  } catch (error) {
    if (error !== "cancel") {
      ElMessage.error(resolveErrorMessage(error, "删除巡检计划失败"))
    }
  }
}

const handleRunPageChange = async (page: number) => {
  runPagination.page = page
  await loadRuns()
}

onMounted(loadPage)
</script>

<template>
  <div class="device-check-page">
    <section class="device-check-page__metrics">
      <article
        v-for="card in metrics"
        :key="card.label"
        class="device-check-page__metric"
        :class="`device-check-page__metric--${card.tone}`"
      >
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
        <small>{{ card.note }}</small>
      </article>
    </section>

    <section class="device-check-page__content-grid">
      <PageCard class="device-check-page__schedule-card" title="巡检计划" :description="`当前共 ${schedules.length} 个计划`">
        <template #headerActions>
          <button v-permission="'device:check-plan:create'" class="app-button app-button--primary device-check-page__button" @click="openCreateDialog">
            <el-icon><Plus /></el-icon>
            <span>新增计划</span>
          </button>
          <button class="app-button app-button--secondary device-check-page__button" :disabled="loading" @click="loadPage">
            <el-icon><RefreshRight /></el-icon>
            <span>刷新</span>
          </button>
        </template>

        <div class="device-check-page__table-wrap">
          <table class="app-table device-check-page__table">
            <colgroup>
              <col class="device-check-page__col-plan-name" />
              <col class="device-check-page__col-frequency" />
              <col class="device-check-page__col-notify" />
              <col class="device-check-page__col-status" />
              <col class="device-check-page__col-time" />
              <col class="device-check-page__col-time" />
              <col class="device-check-page__col-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>计划名称</th>
                <th>执行频率</th>
                <th>邮件推送</th>
                <th>状态</th>
                <th>最近执行</th>
                <th>下次执行</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!schedules.length">
                <td colspan="7" class="app-table__empty">{{ loading ? "加载中..." : "暂无巡检计划" }}</td>
              </tr>
              <tr v-for="record in schedules" :key="record.id">
                <td>
                  <div class="device-check-page__name-cell">
                    <strong>{{ record.name }}</strong>
                    <span>{{ getNotifyModeText(record.notifyMode) }}</span>
                  </div>
                </td>
                <td>
                  <div class="device-check-page__stack-cell">
                    <strong>{{ formatFrequency(record.frequencyPerDay) }}</strong>
                    <span>{{ formatInterval(record.frequencyPerDay) }}</span>
                  </div>
                </td>
                <td>
                  <div class="device-check-page__stack-cell">
                    <StatusTag :text="record.notifyEnabled ? '已启用' : '未启用'" :tone="record.notifyEnabled ? 'success' : 'default'" />
                    <span>{{ getPushConfigNames(record) }}</span>
                  </div>
                </td>
                <td>
                  <div class="device-check-page__stack-cell">
                    <StatusTag :text="getScheduleStatusText(record)" :tone="getScheduleStatusTone(record)" />
                    <span v-if="record.lastError">{{ record.lastError }}</span>
                  </div>
                </td>
                <td>{{ formatDateTime(record.lastRunAt) }}</td>
                <td>{{ formatDateTime(record.nextRunAt) }}</td>
                <td>
                  <div class="table-actions device-check-page__actions">
                    <button
                      v-permission="'device:check-plan:run'"
                      class="app-button app-button--secondary device-check-page__table-button"
                      :disabled="runningId === record.id"
                      @click="handleRunNow(record)"
                    >
                      <el-icon><VideoPlay /></el-icon>
                      <span>{{ runningId === record.id ? "执行中" : "测试" }}</span>
                    </button>
                    <button
                      v-permission="'device:check-plan:update'"
                      class="app-button app-button--secondary device-check-page__table-button"
                      @click="openEditDialog(record)"
                    >
                      <el-icon><Edit /></el-icon>
                      <span>编辑</span>
                    </button>
                    <button
                      v-permission="'device:check-plan:update'"
                      class="app-button app-button--warning device-check-page__table-button"
                      @click="handleToggleStatus(record)"
                    >
                      <span>{{ record.enabled ? "停用" : "启用" }}</span>
                    </button>
                    <button
                      v-permission="'device:check-plan:delete'"
                      class="app-button app-button--danger device-check-page__table-button"
                      @click="handleDelete(record)"
                    >
                      <el-icon><Delete /></el-icon>
                      <span>删除</span>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </PageCard>

      <PageCard class="device-check-page__side-card" title="运行策略" description="当前调度与通知摘要">
        <div class="device-check-page__policy-list">
          <div class="device-check-page__policy-item">
            <span>调度方式</span>
            <strong>后端定时任务</strong>
          </div>
          <div class="device-check-page__policy-item">
            <span>邮件配置</span>
            <strong>{{ emailPushConfigs.length }} 个可用</strong>
          </div>
          <div class="device-check-page__policy-item">
            <span>默认推送</span>
            <strong>新离线设备</strong>
          </div>
          <div class="device-check-page__policy-item">
            <span>下一计划</span>
            <strong>{{ nextSchedule?.name ?? "-" }}</strong>
          </div>
        </div>
        <div class="device-check-page__latest-run">
          <span>最近巡检结果</span>
          <div>
            <strong>{{ latestRun?.checkedTotal ?? 0 }}</strong>
            <small>检测总数</small>
          </div>
          <div>
            <strong>{{ latestRun?.onlineTotal ?? 0 }}</strong>
            <small>在线</small>
          </div>
          <div>
            <strong>{{ latestRun?.offlineTotal ?? 0 }}</strong>
            <small>离线</small>
          </div>
        </div>
      </PageCard>
    </section>

    <PageCard title="执行记录">
      <div class="device-check-page__table-wrap">
        <table class="app-table device-check-page__run-table">
          <colgroup>
            <col class="device-check-page__col-run-time" />
            <col class="device-check-page__col-run-status" />
            <col class="device-check-page__col-run-count" />
            <col class="device-check-page__col-run-count" />
            <col class="device-check-page__col-run-count" />
            <col class="device-check-page__col-run-count" />
            <col class="device-check-page__col-run-count" />
            <col class="device-check-page__col-run-notified" />
          </colgroup>
          <thead>
            <tr>
              <th>开始时间</th>
              <th>状态</th>
              <th>检测总数</th>
              <th>在线</th>
              <th>离线</th>
              <th>停用</th>
              <th>变化</th>
              <th>已推送</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!runs.length">
              <td colspan="8" class="app-table__empty">{{ loading ? "加载中..." : "暂无执行记录" }}</td>
            </tr>
            <tr v-for="record in runs" :key="record.id">
              <td>{{ formatDateTime(record.startedAt) }}</td>
              <td><StatusTag :text="getRunStatusText(record.status)" :tone="getRunStatusTone(record.status)" /></td>
              <td>{{ record.checkedTotal }}</td>
              <td>{{ record.onlineTotal }}</td>
              <td>{{ record.offlineTotal }}</td>
              <td>{{ record.disabledTotal }}</td>
              <td>{{ record.changedTotal }}</td>
              <td>{{ record.notified ? "是" : "否" }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="device-check-page__pagination">
        <el-pagination
          layout="total, prev, pager, next"
          :current-page="runPagination.page"
          :page-size="runPagination.pageSize"
          :total="runPagination.total"
          @current-change="handleRunPageChange"
        />
      </div>
    </PageCard>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑巡检计划' : '新增巡检计划'" width="640px">
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="118px">
        <el-form-item label="计划名称" prop="name">
          <el-input v-model="formState.name" maxlength="100" />
        </el-form-item>
        <el-form-item label="是否启用">
          <el-switch v-model="formState.enabled" />
        </el-form-item>
        <el-form-item label="每天检测次数" prop="frequencyPerDay">
          <el-select v-model="formState.frequencyPerDay" style="width: 100%">
            <el-option v-for="item in frequencyOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="邮件推送">
          <el-switch v-model="formState.notifyEnabled" />
        </el-form-item>
        <el-form-item v-if="formState.notifyEnabled" label="推送配置">
          <el-select v-model="formState.pushConfigIds" multiple style="width: 100%" placeholder="选择邮件推送配置">
            <el-option v-for="item in emailPushConfigs" :key="item.id" :label="item.configName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="formState.notifyEnabled" label="推送策略">
          <el-segmented v-model="formState.notifyMode" :options="notifyModeOptions" />
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="dialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="saving" @click="handleSave">
          {{ saving ? "保存中..." : "保存" }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.device-check-page {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.device-check-page__overview {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(320px, 420px);
  gap: 18px;
  padding: 22px 24px;
  border: 1px solid #dbe6f0;
  border-radius: 12px;
  background:
    linear-gradient(135deg, rgba(36, 125, 255, 0.08), rgba(255, 255, 255, 0.8)),
    #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.device-check-page__overview-main {
  min-width: 0;
}

.device-check-page__eyebrow {
  color: #247dff;
  font-size: 13px;
  font-weight: 700;
}

.device-check-page__overview h2 {
  margin: 8px 0;
  color: #0e2b4b;
  font-size: 22px;
  line-height: 1.35;
}

.device-check-page__overview p {
  margin: 0;
  color: #667b91;
  font-size: 13px;
  line-height: 1.7;
}

.device-check-page__overview-meta {
  display: grid;
  grid-template-columns: 1fr;
  gap: 10px;
}

.device-check-page__overview-meta div,
.device-check-page__policy-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
  padding: 10px 12px;
  border: 1px solid rgba(132, 154, 180, 0.16);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.75);
}

.device-check-page__overview-meta span,
.device-check-page__policy-item span {
  color: #667b91;
  font-size: 12px;
}

.device-check-page__overview-meta strong,
.device-check-page__policy-item strong {
  min-width: 0;
  color: #163657;
  font-size: 13px;
  text-align: right;
  word-break: break-word;
}

.device-check-page__metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.device-check-page__metric {
  display: flex;
  min-height: 112px;
  flex-direction: column;
  gap: 8px;
  padding: 18px 20px;
  border: 1px solid #dbe6f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.device-check-page__metric span,
.device-check-page__metric small {
  color: #667b91;
  font-size: 13px;
}

.device-check-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
  line-height: 1;
}

.device-check-page__metric--success strong {
  color: #1d9b52;
}

.device-check-page__metric--danger strong {
  color: #d64f5a;
}

.device-check-page__metric--warning strong {
  color: #d19a1b;
}

.device-check-page__metric--info strong {
  color: #1d7ad9;
}

.device-check-page__content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 18px;
  align-items: start;
}

.device-check-page__table-wrap {
  width: 100%;
  overflow-x: auto;
}

.device-check-page__table,
.device-check-page__run-table {
  min-width: 960px;
  table-layout: fixed;
}

.device-check-page__table th,
.device-check-page__table td,
.device-check-page__run-table th,
.device-check-page__run-table td {
  padding: 9px 10px;
  font-size: 12px;
  white-space: nowrap;
  vertical-align: middle;
}

.device-check-page__table th,
.device-check-page__run-table th {
  font-size: 12px;
  white-space: nowrap;
}

.device-check-page__name-cell,
.device-check-page__stack-cell {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 5px;
}

.device-check-page__name-cell strong,
.device-check-page__stack-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.device-check-page__name-cell span,
.device-check-page__stack-cell span {
  max-width: 220px;
  overflow: hidden;
  color: #708398;
  font-size: 11px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-check-page__button,
.device-check-page__table-button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.device-check-page__table-button {
  min-height: 32px;
  padding: 0 10px;
  font-size: 12px;
}

.device-check-page__actions {
  flex-wrap: nowrap;
  gap: 4px;
}

.device-check-page__col-plan-name {
  width: 150px;
}

.device-check-page__col-frequency {
  width: 112px;
}

.device-check-page__col-notify {
  width: 150px;
}

.device-check-page__col-status {
  width: 90px;
}

.device-check-page__col-time,
.device-check-page__col-run-time {
  width: 155px;
}

.device-check-page__col-actions {
  width: 230px;
}

.device-check-page__col-run-status {
  width: 88px;
}

.device-check-page__col-run-count,
.device-check-page__col-run-notified {
  width: 84px;
}

.device-check-page__policy-list {
  display: grid;
  gap: 10px;
}

.device-check-page__latest-run {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #edf2f7;
}

.device-check-page__latest-run > span {
  grid-column: 1 / -1;
  color: #4e6177;
  font-size: 13px;
  font-weight: 700;
}

.device-check-page__latest-run div {
  display: flex;
  min-height: 68px;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
  padding: 10px;
  border: 1px solid rgba(132, 154, 180, 0.16);
  border-radius: 8px;
  background: #f7faff;
}

.device-check-page__latest-run strong {
  color: #0e2b4b;
  font-size: 22px;
  line-height: 1;
}

.device-check-page__latest-run small {
  color: #708398;
  font-size: 12px;
}

.device-check-page__pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

@media (max-width: 1280px) {
  .device-check-page__overview,
  .device-check-page__content-grid {
    grid-template-columns: 1fr;
  }

  .device-check-page__side-card {
    order: -1;
  }
}

@media (max-width: 920px) {
  .device-check-page__metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .device-check-page__overview {
    padding: 18px;
  }

  .device-check-page__overview-meta div,
  .device-check-page__policy-item {
    align-items: flex-start;
    flex-direction: column;
  }

  .device-check-page__overview-meta strong,
  .device-check-page__policy-item strong {
    text-align: left;
  }

  .device-check-page__metrics,
  .device-check-page__latest-run {
    grid-template-columns: 1fr;
  }
}
</style>
