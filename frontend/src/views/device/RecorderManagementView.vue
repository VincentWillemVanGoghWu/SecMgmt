<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import {
  Connection,
  Delete,
  EditPen,
  Plus,
  RefreshRight,
  Search,
  SwitchButton,
} from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { listFactoriesApi } from "../../api/master-data"
import {
  checkRecorderStatusApi,
  createRecorderApi,
  deleteRecorderApi,
  getRecorderApi,
  listRecordersApi,
  syncRecorderChannelsApi,
  testRecorderConnectionApi,
  updateRecorderApi,
} from "../../api/recorder"
import { checkAllDevicesStatusApi } from "../../api/device-status"
import type { FactoryRecord } from "../../types/master-data"
import type { RecorderRecord, RecorderSubmitPayload } from "../../types/recorder"

const DEFAULT_DEVICE_PASSWORD = "bhcd2017"

interface RecorderFormState extends RecorderSubmitPayload {
  password: string
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const testingId = ref<number | null>(null)
const checkingId = ref<number | null>(null)
const syncingId = ref<number | null>(null)
const checkingAll = ref(false)
const formRef = ref<FormInstance>()

const records = ref<RecorderRecord[]>([])
const factories = ref<FactoryRecord[]>([])

const queryForm = reactive({
  keyword: "",
  factoryId: "",
  status: "",
})

const formState = reactive<RecorderFormState>({
  deviceCode: "",
  name: "",
  ip: "",
  sdkPort: 8000,
  httpPort: 80,
  username: "admin",
  password: DEFAULT_DEVICE_PASSWORD,
  channelCount: 0,
  factoryId: 0,
  status: "offline",
})

const statusOptions = [
  { label: "在线", value: "online" },
  { label: "离线", value: "offline" },
  { label: "异常", value: "exception" },
  { label: "停用", value: "disabled" },
]

const rules: FormRules<RecorderFormState> = {
  deviceCode: [{ required: true, message: "请输入设备编码", trigger: "blur" }],
  name: [{ required: true, message: "请输入录像机名称", trigger: "blur" }],
  ip: [{ required: true, message: "请输入设备 IP", trigger: "blur" }],
  username: [{ required: true, message: "请输入登录账号", trigger: "blur" }],
  factoryId: [{ required: true, message: "请选择所属厂区", trigger: "change" }],
  password: [
    {
      validator: (_, value, callback) => {
        if (!editingId.value && !String(value || "").trim()) {
          callback(new Error("请输入设备密码"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
}

const metrics = computed(() => {
  const total = records.value.length
  const online = records.value.filter((item) => item.status === "online").length
  const abnormal = records.value.filter((item) => item.status === "exception").length
  const totalChannels = records.value.reduce((sum, item) => sum + item.channelCount, 0)
  return [
    { label: "录像机总数", value: total, accent: "primary" },
    { label: "在线录像机", value: online, accent: "success" },
    { label: "异常录像机", value: abnormal, accent: "danger" },
    { label: "已同步通道", value: totalChannels, accent: "info" },
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

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", { hour12: false })
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) {
    return detail
  }
  if (error instanceof Error && error.message) {
    return error.message
  }
  return fallback
}

const loadFactories = async () => {
  factories.value = await listFactoriesApi()
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listRecordersApi({
      keyword: queryForm.keyword || undefined,
      factory_id: queryForm.factoryId ? Number(queryForm.factoryId) : undefined,
      status: queryForm.status || undefined,
    })
  } finally {
    loading.value = false
  }
}

const resetFormState = () => {
  editingId.value = null
  formState.deviceCode = ""
  formState.name = ""
  formState.ip = ""
  formState.sdkPort = 8000
  formState.httpPort = 80
  formState.username = "admin"
  formState.password = DEFAULT_DEVICE_PASSWORD
  formState.channelCount = 0
  formState.factoryId = factories.value[0]?.id ?? 0
  formState.status = "offline"
}

const openCreateDialog = () => {
  resetFormState()
  dialogVisible.value = true
}

const openEditDialog = async (record: RecorderRecord) => {
  try {
    const detail = await getRecorderApi(record.id)
    editingId.value = detail.id
    formState.deviceCode = detail.deviceCode
    formState.name = detail.name
    formState.ip = detail.ip
    formState.sdkPort = detail.sdkPort
    formState.httpPort = detail.httpPort
    formState.username = detail.username
    formState.password = ""
    formState.channelCount = detail.channelCount
    formState.factoryId = detail.factoryId
    formState.status = detail.status
    dialogVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "加载录像机详情失败"))
  }
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  const payload: RecorderSubmitPayload = {
    deviceCode: formState.deviceCode.trim(),
    name: formState.name.trim(),
    ip: formState.ip.trim(),
    sdkPort: Number(formState.sdkPort),
    httpPort: Number(formState.httpPort),
    username: formState.username.trim(),
    channelCount: Number(formState.channelCount),
    factoryId: Number(formState.factoryId),
    status: formState.status,
  }

  submitting.value = true
  try {
    if (editingId.value) {
      await updateRecorderApi(editingId.value, {
        ...payload,
        password: formState.password.trim() || undefined,
      })
      ElMessage.success("录像机更新成功")
    } else {
      await createRecorderApi({
        ...payload,
        password: formState.password.trim(),
      })
      ElMessage.success("录像机创建成功")
    }
    dialogVisible.value = false
    resetFormState()
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存录像机失败"))
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (record: RecorderRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除录像机“${record.name}”吗？`, "删除确认", { type: "warning" })
    await deleteRecorderApi(record.id)
    ElMessage.success("录像机删除成功")
    await loadRecords()
  } catch (error) {
    if ((error as { message?: string })?.message === "cancel") return
    ElMessage.error(resolveErrorMessage(error, "删除录像机失败"))
  }
}

const handleTestConnection = async (record: RecorderRecord) => {
  testingId.value = record.id
  try {
    const result = await testRecorderConnectionApi(record.id)
    ElMessage.success(result.message)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "录像机连接测试失败"))
  } finally {
    testingId.value = null
  }
}

const handleSyncChannels = async (record: RecorderRecord) => {
  syncingId.value = record.id
  try {
    const result = await syncRecorderChannelsApi(record.id)
    ElMessage.success(`已同步 ${result.channelCount} 个通道`)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "同步通道失败"))
  } finally {
    syncingId.value = null
  }
}

const handleCheckStatus = async (record: RecorderRecord) => {
  checkingId.value = record.id
  try {
    const result = await checkRecorderStatusApi(record.id)
    ElMessage.success(result.message)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "录像机状态检测失败"))
  } finally {
    checkingId.value = null
  }
}

const handleCheckAll = async () => {
  checkingAll.value = true
  try {
    const result = await checkAllDevicesStatusApi()
    ElMessage.success(result.message)
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "执行全部检测失败"))
  } finally {
    checkingAll.value = false
  }
}

const handleToggleStatus = async (record: RecorderRecord) => {
  try {
    await updateRecorderApi(record.id, {
      deviceCode: record.deviceCode,
      name: record.name,
      ip: record.ip,
      sdkPort: record.sdkPort,
      httpPort: record.httpPort,
      username: record.username,
      channelCount: record.channelCount,
      factoryId: record.factoryId,
      status: record.status === "disabled" ? "offline" : "disabled",
      password: undefined,
    })
    ElMessage.success(record.status === "disabled" ? "录像机已启用" : "录像机已停用")
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新录像机状态失败"))
  }
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.factoryId = ""
  queryForm.status = ""
  await loadRecords()
}

onMounted(async () => {
  await loadFactories()
  resetFormState()
  await loadRecords()
})
</script>

<template>
  <div class="device-page unified-list-page">
    <PageCard class="device-page__filters-card">
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.factoryId">
            <option value="">所属厂区</option>
            <option v-for="item in factories" :key="item.id" :value="String(item.id)">{{ item.factoryName }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.status">
            <option value="">状态</option>
            <option v-for="item in statusOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field device-page__keyword">
          <input v-model="queryForm.keyword" type="text" placeholder="输入录像机名称、编码或 IP" />
        </div>
        <template #actions>
          <button class="app-button app-button--primary device-page__button unified-list-page__button unified-list-page__search-button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary device-page__button unified-list-page__button unified-list-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'device:status:check'"
            class="app-button app-button--warning device-page__button unified-list-page__button unified-list-page__search-button"
            :disabled="checkingAll"
            @click="handleCheckAll"
          >
            <el-icon><RefreshRight /></el-icon>
            <span>{{ checkingAll ? "检测中..." : "全部检测" }}</span>
          </button>
          <button
            v-permission="'device:recorder:create'"
            class="app-button app-button--success device-page__button unified-list-page__button unified-list-page__search-button"
            @click="openCreateDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增录像机</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="device-page__summary unified-list-page__summary">
      <article v-for="card in metrics" :key="card.label" class="device-page__metric unified-list-page__metric" :class="[`device-page__metric--${card.accent}`, `unified-list-page__metric--${card.accent}`]">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <PageCard >
      <table class="app-table device-page__table unified-list-page__table">
        <colgroup>
          <col class="device-page__col-code" />
          <col class="device-page__col-name" />
          <col class="device-page__col-factory" />
          <col class="device-page__col-ip" />
          <col class="device-page__col-account" />
          <col class="device-page__col-channel" />
          <col class="device-page__col-status" />
          <col class="device-page__col-time" />
          <col class="device-page__col-actions" />
        </colgroup>
        <thead>
          <tr>
            <th>设备编码</th>
            <th>录像机名称</th>
            <th>厂区</th>
            <th>IP</th>
            <th>账号</th>
            <th>通道数</th>
            <th>状态</th>
            <th>最后在线时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="9" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>{{ record.deviceCode }}</td>
            <td>
              <div class="device-page__name-cell unified-list-page__name-cell">
                <strong>{{ record.name }}</strong>
              </div>
            </td>
            <td>{{ record.factoryName }}</td>
            <td>{{ record.ip }}</td>
            <td>{{ record.username }}</td>
            <td>{{ record.channelCount }}</td>
            <td><StatusTag :text="getStatusText(record.status)" :tone="getStatusTone(record.status)" /></td>
            <td>{{ formatDateTime(record.lastOnlineAt) }}</td>
            <td>
              <div class="table-actions">
                <button
                  v-permission="'device:recorder:update'"
                  class="app-button app-button--secondary device-page__button device-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
                <button
                  v-permission="'device:recorder:update'"
                  class="app-button app-button--warning device-page__button device-page__table-button unified-list-page__button unified-list-page__table-button"
                  @click="handleToggleStatus(record)"
                >
                  <el-icon><SwitchButton /></el-icon>
                  <span>{{ record.status === "disabled" ? "启用" : "停用" }}</span>
                </button>
                <button
                  v-permission="'device:recorder:test'"
                  class="app-button app-button--primary device-page__button device-page__table-button unified-list-page__button unified-list-page__table-button"
                  :disabled="testingId === record.id"
                  @click="handleTestConnection(record)"
                >
                  <el-icon><Connection /></el-icon>
                  <span>{{ testingId === record.id ? "测试中" : "测试" }}</span>
                </button>
                <button
                  v-permission="'device:recorder:test'"
                  class="app-button app-button--secondary device-page__button device-page__table-button unified-list-page__button unified-list-page__table-button"
                  :disabled="checkingId === record.id"
                  @click="handleCheckStatus(record)"
                >
                  <el-icon><RefreshRight /></el-icon>
                  <span>{{ checkingId === record.id ? "检测中" : "检测" }}</span>
                </button>
                <button
                  v-permission="'device:recorder:sync'"
                  class="app-button app-button--success device-page__button device-page__table-button unified-list-page__button unified-list-page__table-button"
                  :disabled="syncingId === record.id"
                  @click="handleSyncChannels(record)"
                >
                  <el-icon><RefreshRight /></el-icon>
                  <span>{{ syncingId === record.id ? "同步中" : "同步" }}</span>
                </button>
                <button
                  v-permission="'device:recorder:delete'"
                  class="app-button app-button--danger device-page__button device-page__table-button unified-list-page__button unified-list-page__table-button"
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
    </PageCard>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑录像机' : '新增录像机'" width="720px" destroy-on-close>
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="100px" class="device-form">
        <el-form-item label="设备编码" prop="deviceCode">
          <el-input v-model="formState.deviceCode" placeholder="例如 nvr-steel-001" />
        </el-form-item>
        <el-form-item label="录像机名称" prop="name">
          <el-input v-model="formState.name" placeholder="例如 炼钢一区 NVR" />
        </el-form-item>
        <el-form-item label="设备 IP" prop="ip">
          <el-input v-model="formState.ip" placeholder="例如 192.168.2.18" />
        </el-form-item>
        <el-form-item label="SDK 端口">
          <el-input-number v-model="formState.sdkPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
        </el-form-item>
        <el-form-item label="HTTP 端口">
          <el-input-number v-model="formState.httpPort" :min="1" :max="65535" controls-position="right" style="width: 100%" />
        </el-form-item>
        <el-form-item label="登录账号" prop="username">
          <el-input v-model="formState.username" />
        </el-form-item>
        <el-form-item label="设备密码" prop="password">
          <el-input v-model="formState.password" type="password" show-password :placeholder="editingId ? '留空表示保持原密码' : '请输入设备密码'" />
        </el-form-item>
        <el-form-item label="所属厂区" prop="factoryId">
          <el-select v-model="formState.factoryId" style="width: 100%">
            <el-option v-for="item in factories" :key="item.id" :label="item.factoryName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="通道数">
          <el-input-number v-model="formState.channelCount" :min="0" :max="128" controls-position="right" style="width: 100%" />
        </el-form-item>
        <el-form-item label="运行状态">
          <el-select v-model="formState.status" style="width: 100%">
            <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
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
  gap: 14px;
}

.device-page__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.device-page__metric {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.device-page__metric span {
  color: #667b91;
  font-size: 12px;
}

.device-page__metric strong {
  color: #0e2b4b;
  font-size: 26px;
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

.device-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.device-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px 180px minmax(260px, 1fr);
  gap: 10px;
  align-items: end;
}

.device-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.device-page__filters-card :deep(.app-field select),
.device-page__filters-card :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.device-page__filters-card .device-page__button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.device-page__keyword {
  grid-column: auto;
}

.device-page__name-cell,
.device-page__port-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.device-page__name-cell strong {
  color: #163657;
  font-size: 13px;
  line-height: 1.35;
}

.device-page__name-cell span,
.device-page__port-cell small {
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

.device-page__table td:nth-child(1),
.device-page__table td:nth-child(3),
.device-page__table td:nth-child(5),
.device-page__table td:nth-child(6),
.device-page__table td:nth-child(7) {
  white-space: nowrap;
}

.device-page__table td:nth-child(8) {
  font-size: 12px;
  line-height: 1.4;
}

.device-page__col-code {
  width: 72px;
}

.device-page__col-name {
  width: 128px;
}

.device-page__col-factory {
  width: 82px;
}

.device-page__col-ip {
  width: 140px;
}

.device-page__col-account {
  width: 58px;
}

.device-page__col-channel {
  width: 58px;
}

.device-page__col-status {
  width: 72px;
}

.device-page__col-time {
  width: 132px;
}

.device-page__col-actions {
  width: 332px;
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

  .device-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .device-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr 1fr;
  }

  .device-page__keyword {
    grid-column: auto;
  }

  .device-page__table .table-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 768px) {
  .device-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
