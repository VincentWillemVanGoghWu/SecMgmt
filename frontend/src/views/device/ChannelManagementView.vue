<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage } from "element-plus"
import { EditPen, RefreshRight, Search } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { listFactoriesApi, listZonesApi } from "../../api/master-data"
import { listCamerasApi } from "../../api/camera"
import { listChannelsApi, updateChannelApi } from "../../api/recorder"
import type { CameraRecord } from "../../types/camera"
import type { FactoryRecord, ZoneRecord } from "../../types/master-data"
import type { RecorderChannelRecord, RecorderChannelUpdatePayload } from "../../types/recorder"

interface ChannelFormState extends RecorderChannelUpdatePayload {}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editingChannel = ref<RecorderChannelRecord | null>(null)
const formRef = ref<FormInstance>()

const records = ref<RecorderChannelRecord[]>([])
const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const cameras = ref<CameraRecord[]>([])

const queryForm = reactive({
  keyword: "",
  factoryId: "",
  zoneId: "",
  status: "",
})

const formState = reactive<ChannelFormState>({
  name: "",
  cameraId: null,
  factoryId: 0,
  zoneId: null,
  enabled: true,
  supportPlayback: true,
  status: "offline",
})

const statusOptions = [
  { label: "在线", value: "online" },
  { label: "离线", value: "offline" },
  { label: "异常", value: "exception" },
  { label: "停用", value: "disabled" },
]

const rules: FormRules<ChannelFormState> = {
  name: [{ required: true, message: "请输入通道名称", trigger: "blur" }],
  factoryId: [{ required: true, message: "请选择所属厂区", trigger: "change" }],
}

const queryZoneOptions = computed(() => {
  if (!queryForm.factoryId) return zones.value
  return zones.value.filter((item) => item.factoryId === Number(queryForm.factoryId))
})

const formZoneOptions = computed(() => zones.value.filter((item) => item.factoryId === formState.factoryId))
const formCameraOptions = computed(() => {
  if (!formState.factoryId) return cameras.value
  return cameras.value.filter((item) => item.factoryId === formState.factoryId)
})

const metrics = computed(() => {
  const total = records.value.length
  const boundCamera = records.value.filter((item) => item.cameraId).length
  const enabledCount = records.value.filter((item) => item.enabled).length
  const playbackCount = records.value.filter((item) => item.supportPlayback).length
  return [
    { label: "通道总数", value: total, accent: "primary" },
    { label: "已绑定摄像机", value: boundCamera, accent: "info" },
    { label: "启用通道", value: enabledCount, accent: "success" },
    { label: "支持回放", value: playbackCount, accent: "warning" },
  ]
})

const getStatusText = (status: string) => {
  if (status === "online") return "在线"
  if (status === "offline") return "离线"
  if (status === "exception") return "异常"
  if (status === "disabled") return "停用"
  return status
}

const getStatusTone = (status: string) => {
  if (status === "online") return "success"
  if (status === "exception") return "danger"
  if (status === "disabled") return "warning"
  return "default"
}

const getEnabledText = (enabled: boolean) => (enabled ? "启用" : "停用")
const getEnabledTone = (enabled: boolean) => (enabled ? "success" : "default")

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const loadLookups = async () => {
  ;[factories.value, zones.value, cameras.value] = await Promise.all([listFactoriesApi(), listZonesApi(), listCamerasApi()])
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listChannelsApi({
      keyword: queryForm.keyword || undefined,
      factory_id: queryForm.factoryId ? Number(queryForm.factoryId) : undefined,
      zone_id: queryForm.zoneId ? Number(queryForm.zoneId) : undefined,
      status: queryForm.status || undefined,
    })
  } finally {
    loading.value = false
  }
}

const resetFormState = () => {
  formState.name = ""
  formState.cameraId = null
  formState.factoryId = factories.value[0]?.id ?? 0
  formState.zoneId = null
  formState.enabled = true
  formState.supportPlayback = true
  formState.status = "offline"
}

const openEditDialog = (record: RecorderChannelRecord) => {
  editingChannel.value = record
  formState.name = record.name
  formState.cameraId = record.cameraId ?? null
  formState.factoryId = record.factoryId
  formState.zoneId = record.zoneId ?? null
  formState.enabled = record.enabled
  formState.supportPlayback = record.supportPlayback
  formState.status = record.status
  dialogVisible.value = true
}

const handleFactoryQueryChange = () => {
  if (!queryZoneOptions.value.some((item) => item.id === Number(queryForm.zoneId))) {
    queryForm.zoneId = ""
  }
}

const handleFormFactoryChange = () => {
  if (!formZoneOptions.value.some((item) => item.id === formState.zoneId)) {
    formState.zoneId = null
  }
  if (formState.cameraId && !formCameraOptions.value.some((item) => item.id === formState.cameraId)) {
    formState.cameraId = null
  }
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid || !editingChannel.value) return

  submitting.value = true
  try {
    await updateChannelApi(editingChannel.value.id, {
      name: formState.name.trim(),
      cameraId: formState.cameraId || null,
      factoryId: Number(formState.factoryId),
      zoneId: formState.zoneId || null,
      enabled: formState.enabled,
      supportPlayback: formState.supportPlayback,
      status: formState.status,
    })
    ElMessage.success("通道更新成功")
    dialogVisible.value = false
    editingChannel.value = null
    resetFormState()
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新通道失败"))
  } finally {
    submitting.value = false
  }
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.factoryId = ""
  queryForm.zoneId = ""
  queryForm.status = ""
  await loadRecords()
}

onMounted(async () => {
  await loadLookups()
  resetFormState()
  await loadRecords()
})
</script>

<template>
  <div class="device-page">
    <PageCard class="device-page__filters-card">
      <SearchForm class="device-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.factoryId" @change="handleFactoryQueryChange" v-refresh-on-empty="loadRecords">
            <option value="">厂区</option>
            <option v-for="item in factories" :key="item.id" :value="String(item.id)">{{ item.factoryName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.zoneId" v-refresh-on-empty="loadRecords">
            <option value="">区域</option>
            <option v-for="item in queryZoneOptions" :key="item.id" :value="String(item.id)">{{ item.zoneName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.status" v-refresh-on-empty="loadRecords">
            <option value="">状态</option>
            <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field device-page__keyword">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入通道名称" @clear="loadRecords" />
        </div>
        <template #actions>
          <button class="app-button app-button--primary device-page__button device-page__search-button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary device-page__button device-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
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

    <PageCard>
      <table class="app-table device-page__table">
        <colgroup>
          <col class="device-page__col-recorder" />
          <col class="device-page__col-name" />
          <col class="device-page__col-camera" />
          <col class="device-page__col-area" />
          <col class="device-page__col-enabled" />
          <col class="device-page__col-playback" />
          <col class="device-page__col-status" />
          <col class="device-page__col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>录像机 / 通道号</th>
            <th>通道名称</th>
            <th>绑定摄像机</th>
            <th>厂区 / 区域</th>
            <th>启停</th>
            <th>回放</th>
            <th>状态</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="8" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>
              <div class="device-page__name-cell">
                <strong>{{ record.recorderName }}</strong>
                <span>CH{{ String(record.channelNo).padStart(2, "0") }}</span>
              </div>
            </td>
            <td>
              <div class="device-page__name-cell">
                <strong>{{ record.name }}</strong>
              </div>
            </td>
            <td>{{ record.cameraName || "-" }}</td>
            <td>{{ record.factoryName }} / {{ record.zoneName || "-" }}</td>
            <td><StatusTag :text="getEnabledText(record.enabled)" :tone="getEnabledTone(record.enabled)" /></td>
            <td><StatusTag :text="record.supportPlayback ? '支持回放' : '仅预览'" :tone="record.supportPlayback ? 'info' : 'default'" /></td>
            <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td>
              <div class="table-actions">
                <button
                  v-permission="'device:channel:update'"
                  class="app-button app-button--secondary device-page__button device-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </PageCard>

    <el-dialog v-model="dialogVisible" :title="editingChannel ? '编辑通道' : '编辑通道'" width="760px" destroy-on-close>
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="100px" class="device-form">
        <el-form-item label="通道名称" prop="name">
          <el-input v-model="formState.name" />
        </el-form-item>
        <el-form-item label="所属厂区" prop="factoryId">
          <el-select v-model="formState.factoryId" style="width: 100%" @change="handleFormFactoryChange">
            <el-option v-for="item in factories" :key="item.id" :label="item.factoryName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属区域">
          <el-select v-model="formState.zoneId" clearable style="width: 100%">
            <el-option v-for="item in formZoneOptions" :key="item.id" :label="item.zoneName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="绑定摄像机">
          <el-select v-model="formState.cameraId" disabled style="width: 100%">
            <el-option v-for="item in formCameraOptions" :key="item.id" :label="`${item.name} / ${item.ip}`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="运行状态">
          <el-select v-model="formState.status" style="width: 100%">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="启用状态">
          <el-switch v-model="formState.enabled" />
        </el-form-item>
        <el-form-item label="支持回放">
          <el-switch v-model="formState.supportPlayback" />
        </el-form-item>
      </el-form>
      <template #footer>
        <button class="app-button app-button--secondary" @click="dialogVisible = false">取消</button>
        <button class="app-button app-button--primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? "保存中..." : "保存" }}
        </button>
      </template>
    </el-dialog>
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

.device-page__metric--warning strong {
  color: #d19a1b;
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
  grid-template-columns: 140px 140px 140px minmax(220px, 300px);
  gap: 10px;
  align-items: end;
}

.device-page__search-form :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.device-page__search-form :deep(.app-field select),
.device-page__search-form :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.device-page__search-button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
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

.device-page__col-recorder {
  width: 150px;
}

.device-page__col-name {
  width: 130px;
}

.device-page__col-camera {
  width: 120px;
}

.device-page__col-area {
  width: 150px;
}

.device-page__col-enabled {
  width: 72px;
}

.device-page__col-playback {
  width: 90px;
}

.device-page__col-status {
  width: 72px;
}

.device-page__col-actions {
  width: 80px;
}

.device-page__table .table-actions {
  flex-wrap: nowrap;
  gap: 4px;
}

.device-page__table-button {
  min-height: 30px;
  padding: 0 7px;
  font-size: 11px;
  gap: 3px;
  white-space: nowrap;
}

.device-page__table-button :deep(.el-icon) {
  font-size: 11px;
}

.device-page__keyword {
  grid-column: auto;
}

.device-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 16px;
}

.device-form :deep(.el-form-item) {
  margin-bottom: 18px;
}

@media (max-width: 1100px) {
  .device-page__summary,
  .device-form {
    grid-template-columns: 1fr;
  }

  .device-page__search-form {
    grid-template-columns: 1fr;
  }

  .device-page__search-form :deep(.search-form__fields) {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .device-page__keyword {
    grid-column: auto;
  }
}

@media (max-width: 768px) {
  .device-page__search-form :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
