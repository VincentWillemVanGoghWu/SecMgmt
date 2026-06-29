<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import type { FormInstance, FormRules } from "element-plus"
import { ElMessage, ElMessageBox } from "element-plus"
import { Connection, Delete, EditPen, Plus, RefreshRight, Search, SwitchButton } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { listFactoriesApi, listZonesApi } from "../../api/master-data"
import {
  createPushConfigApi,
  deletePushConfigApi,
  listPushConfigsApi,
  testPushConfigApi,
  updatePushConfigApi,
  updatePushConfigStatusApi,
} from "../../api/push"
import type { FactoryRecord, ZoneRecord } from "../../types/master-data"
import type { PushConfigRecord, PushConfigSubmitPayload } from "../../types/push"

interface PushConfigFormState {
  configName: string
  providerType: "dingtalk" | "wechat"
  webhook: string
  secret: string
  appId: string
  appSecret: string
  templateId: string
  receiverOpenIds: string[]
  receiverOpenIdsText: string
  factoryIds: number[]
  zoneIds: number[]
  alarmTypes: string[]
  alarmLevels: string[]
  enabled: boolean
  rateLimitWindowSeconds: number
  rateLimitMaxCount: number
  retryMaxCount: number
  retryIntervalSeconds: number
  remark: string
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const testingId = ref<number | null>(null)
const formRef = ref<FormInstance>()

const records = ref<PushConfigRecord[]>([])
const factories = ref<FactoryRecord[]>([])
const zones = ref<ZoneRecord[]>([])
const timeRangeValue = ref<string[]>([])
const editingSecretConfigured = ref(false)
const editingAppSecretConfigured = ref(false)

const queryForm = reactive({
  keyword: "",
  enabled: "",
  providerType: "",
})

const formState = reactive<PushConfigFormState>({
  configName: "",
  providerType: "dingtalk",
  webhook: "mock://dingtalk/success",
  secret: "",
  appId: "",
  appSecret: "",
  templateId: "",
  receiverOpenIds: [],
  receiverOpenIdsText: "",
  factoryIds: [],
  zoneIds: [],
  alarmTypes: [],
  alarmLevels: ["high", "critical"],
  enabled: true,
  rateLimitWindowSeconds: 300,
  rateLimitMaxCount: 1,
  retryMaxCount: 2,
  retryIntervalSeconds: 1,
  remark: "",
})

const statusOptions = [
  { label: "全部", value: "" },
  { label: "启用", value: "true" },
  { label: "停用", value: "false" },
]

const providerOptions = [
  { label: "钉钉群机器人", value: "dingtalk" },
  { label: "微信公众号", value: "wechat" },
]

const alarmTypeOptions = [
  { label: "未戴安全帽", value: "helmet_missing" },
  { label: "区域入侵", value: "intrusion" },
  { label: "移动侦测", value: "移动侦测" },
  { label: "烟雾", value: "smoke" },
  { label: "明火", value: "fire" },
  { label: "人员跌倒", value: "person_fall" },
  { label: "人群聚集", value: "crowd" },
]

const alarmLevelOptions = [
  { label: "严重", value: "critical" },
  { label: "高", value: "high" },
  { label: "中", value: "medium" },
  { label: "低", value: "low" },
]

const rules: FormRules<PushConfigFormState> = {
  configName: [{ required: true, message: "请输入配置名称", trigger: "blur" }],
  webhook: [
    {
      validator: (_, value, callback) => {
        if (formState.providerType === "dingtalk" && !String(value || "").trim()) {
          callback(new Error("请输入钉钉 Webhook"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
  appId: [
    {
      validator: (_, value, callback) => {
        if (formState.providerType === "wechat" && !String(value || "").trim()) {
          callback(new Error("请输入微信公众号 AppID"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
  appSecret: [
    {
      validator: (_, value, callback) => {
        if (formState.providerType !== "wechat") {
          callback()
          return
        }
        if (!editingId.value && !String(value || "").trim()) {
          callback(new Error("请输入微信公众号 AppSecret"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
  templateId: [
    {
      validator: (_, value, callback) => {
        if (formState.providerType === "wechat" && !String(value || "").trim()) {
          callback(new Error("请输入模板ID"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
  receiverOpenIdsText: [
    {
      validator: (_, value, callback) => {
        if (formState.providerType === "wechat" && !String(value || "").trim()) {
          callback(new Error("请至少维护一个接收人 OpenID"))
          return
        }
        callback()
      },
      trigger: "blur",
    },
  ],
}

const filteredZoneOptions = computed(() => {
  if (!formState.factoryIds.length) {
    return zones.value
  }
  return zones.value.filter((item) => formState.factoryIds.includes(item.factoryId))
})

const metrics = computed(() => [
  { label: "配置总数", value: records.value.length, accent: "primary" },
  { label: "启用配置", value: records.value.filter((item) => item.enabled).length, accent: "success" },
  { label: "微信配置", value: records.value.filter((item) => item.providerType === "wechat").length, accent: "info" },
  { label: "高危策略", value: records.value.filter((item) => item.alarmLevels.includes("critical") || item.alarmLevels.includes("high")).length, accent: "danger" },
])

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

const formatDateTime = (value?: string | null) => {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString("zh-CN", { hour12: false })
}

const getEnabledText = (enabled: boolean) => (enabled ? "启用" : "停用")
const getEnabledTone = (enabled: boolean) => (enabled ? "success" : "default")
const getProviderText = (providerType: PushConfigRecord["providerType"]) =>
  providerType === "wechat" ? "微信公众号" : "钉钉群机器人"
const getProviderTone = (providerType: PushConfigRecord["providerType"]) =>
  providerType === "wechat" ? "success" : "info"

const getFactoryNames = (ids: number[]) =>
  ids.length ? factories.value.filter((item) => ids.includes(item.id)).map((item) => item.factoryName).join(" / ") : "全部厂区"

const getZoneNames = (ids: number[]) =>
  ids.length ? zones.value.filter((item) => ids.includes(item.id)).map((item) => item.zoneName).join(" / ") : "全部区域"

const getAlarmTypeText = (values: string[]) =>
  values.length
    ? alarmTypeOptions.filter((item) => values.includes(item.value)).map((item) => item.label).join(" / ")
    : "全部类型"

const getAlarmLevelText = (values: string[]) =>
  values.length
    ? alarmLevelOptions.filter((item) => values.includes(item.value)).map((item) => item.label).join(" / ")
    : "全部等级"

const getTimeRangeText = (record: PushConfigRecord) =>
  record.activeTimeRanges.length ? record.activeTimeRanges.map((item) => `${item.start}-${item.end}`).join(" / ") : "全天"

const getCredentialText = (record: PushConfigRecord) => {
  if (record.providerType === "wechat") {
    return record.appSecretConfigured ? "已配置 AppSecret" : "未配置 AppSecret"
  }
  return record.secretConfigured ? "已配置 Secret 加签" : "未配置 Secret"
}

const getReceiverText = (record: PushConfigRecord) => {
  if (record.providerType === "wechat") {
    return record.receiverOpenIds.length ? `已配置 ${record.receiverOpenIds.length} 个接收人` : "未配置接收人"
  }
  return record.webhook || "-"
}

const normalizeOpenIds = (value: string) =>
  value
    .split(/[\n,，;；]+/)
    .map((item) => item.trim())
    .filter(Boolean)

const loadLookups = async () => {
  ;[factories.value, zones.value] = await Promise.all([listFactoriesApi(), listZonesApi()])
}

const loadRecords = async () => {
  loading.value = true
  try {
    records.value = await listPushConfigsApi({
      keyword: queryForm.keyword || undefined,
      enabled: queryForm.enabled === "" ? undefined : queryForm.enabled === "true",
      provider_type: queryForm.providerType || undefined,
    })
  } finally {
    loading.value = false
  }
}

const resetFormState = () => {
  editingId.value = null
  editingSecretConfigured.value = false
  editingAppSecretConfigured.value = false
  formState.configName = ""
  formState.providerType = "dingtalk"
  formState.webhook = "mock://dingtalk/success"
  formState.secret = ""
  formState.appId = ""
  formState.appSecret = ""
  formState.templateId = ""
  formState.receiverOpenIds = []
  formState.receiverOpenIdsText = ""
  formState.factoryIds = []
  formState.zoneIds = []
  formState.alarmTypes = []
  formState.alarmLevels = ["high", "critical"]
  formState.enabled = true
  formState.rateLimitWindowSeconds = 300
  formState.rateLimitMaxCount = 1
  formState.retryMaxCount = 2
  formState.retryIntervalSeconds = 1
  formState.remark = ""
  timeRangeValue.value = []
}

const openCreateDialog = () => {
  resetFormState()
  dialogVisible.value = true
}

const openEditDialog = (record: PushConfigRecord) => {
  editingId.value = record.id
  editingSecretConfigured.value = record.secretConfigured
  editingAppSecretConfigured.value = record.appSecretConfigured
  formState.configName = record.configName
  formState.providerType = record.providerType
  formState.webhook = record.webhook ?? (record.providerType === "dingtalk" ? "mock://dingtalk/success" : "")
  formState.secret = ""
  formState.appId = record.appId ?? ""
  formState.appSecret = ""
  formState.templateId = record.templateId ?? ""
  formState.receiverOpenIds = [...record.receiverOpenIds]
  formState.receiverOpenIdsText = record.receiverOpenIds.join("\n")
  formState.factoryIds = [...record.factoryIds]
  formState.zoneIds = [...record.zoneIds]
  formState.alarmTypes = [...record.alarmTypes]
  formState.alarmLevels = [...record.alarmLevels]
  formState.enabled = record.enabled
  formState.rateLimitWindowSeconds = record.rateLimitWindowSeconds
  formState.rateLimitMaxCount = record.rateLimitMaxCount
  formState.retryMaxCount = record.retryMaxCount
  formState.retryIntervalSeconds = record.retryIntervalSeconds
  formState.remark = record.remark ?? ""
  timeRangeValue.value = record.activeTimeRanges.length
    ? [record.activeTimeRanges[0].start, record.activeTimeRanges[0].end]
    : []
  dialogVisible.value = true
}

const handleFactoryChange = () => {
  formState.zoneIds = formState.zoneIds.filter((item) => filteredZoneOptions.value.some((zone) => zone.id === item))
}

const handleProviderChange = (value: PushConfigFormState["providerType"]) => {
  if (value === "dingtalk") {
    formState.appId = ""
    formState.appSecret = ""
    formState.templateId = ""
    formState.receiverOpenIds = []
    formState.receiverOpenIdsText = ""
    if (!formState.webhook) {
      formState.webhook = "mock://dingtalk/success"
    }
    return
  }
  formState.webhook = ""
  formState.secret = ""
  if (!formState.appId) {
    formState.appId = "mock://wechat/success-app"
  }
}

const buildPayload = (): PushConfigSubmitPayload => {
  const receiverOpenIds = normalizeOpenIds(formState.receiverOpenIdsText)
  return {
    configName: formState.configName.trim(),
    providerType: formState.providerType,
    webhook: formState.providerType === "dingtalk" ? formState.webhook.trim() || null : null,
    appId: formState.providerType === "wechat" ? formState.appId.trim() || null : null,
    appSecret: formState.providerType === "wechat" ? formState.appSecret.trim() || undefined : undefined,
    templateId: formState.providerType === "wechat" ? formState.templateId.trim() || null : null,
    receiverOpenIds: formState.providerType === "wechat" ? receiverOpenIds : [],
    factoryIds: [...formState.factoryIds],
    zoneIds: [...formState.zoneIds],
    alarmTypes: [...formState.alarmTypes],
    alarmLevels: [...formState.alarmLevels],
    activeTimeRanges:
      timeRangeValue.value.length === 2
        ? [{ start: timeRangeValue.value[0], end: timeRangeValue.value[1] }]
        : [],
    enabled: formState.enabled,
    rateLimitWindowSeconds: Number(formState.rateLimitWindowSeconds),
    rateLimitMaxCount: Number(formState.rateLimitMaxCount),
    retryMaxCount: Number(formState.retryMaxCount),
    retryIntervalSeconds: Number(formState.retryIntervalSeconds),
    remark: formState.remark.trim() || null,
    secret: formState.providerType === "dingtalk" ? formState.secret.trim() || undefined : undefined,
  }
}

const handleSubmit = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) {
    return
  }

  submitting.value = true
  try {
    const payload = buildPayload()
    if (editingId.value) {
      await updatePushConfigApi(editingId.value, payload)
      ElMessage.success("推送配置更新成功")
    } else {
      await createPushConfigApi(payload)
      ElMessage.success("推送配置创建成功")
    }
    dialogVisible.value = false
    resetFormState()
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "保存推送配置失败"))
  } finally {
    submitting.value = false
  }
}

const handleResetQuery = async () => {
  queryForm.keyword = ""
  queryForm.enabled = ""
  queryForm.providerType = ""
  await loadRecords()
}

const handleToggleStatus = async (record: PushConfigRecord) => {
  try {
    await updatePushConfigStatusApi(record.id, !record.enabled)
    ElMessage.success(record.enabled ? "推送配置已停用" : "推送配置已启用")
    await loadRecords()
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "更新推送配置状态失败"))
  }
}

const handleDelete = async (record: PushConfigRecord) => {
  try {
    await ElMessageBox.confirm(`确认删除推送配置“${record.configName}”吗？`, "删除确认", { type: "warning" })
    await deletePushConfigApi(record.id)
    ElMessage.success("推送配置删除成功")
    await loadRecords()
  } catch (error) {
    if ((error as { message?: string })?.message === "cancel") {
      return
    }
    ElMessage.error(resolveErrorMessage(error, "删除推送配置失败"))
  }
}

const handleTestPush = async (record: PushConfigRecord) => {
  testingId.value = record.id
  try {
    const result = await testPushConfigApi(record.id)
    ElMessage[result.success ? "success" : "error"](`${result.message}，时间 ${formatDateTime(result.pushedAt)}`)
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "测试推送失败"))
  } finally {
    testingId.value = null
  }
}

onMounted(async () => {
  await loadLookups()
  await loadRecords()
})
</script>

<template>
  <div class="push-page unified-list-page">
    <PageCard
      class="push-page__filters-card"
     >
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <select v-model="queryForm.enabled" v-refresh-on-empty="loadRecords">
            <option value="">状态</option>
            <option v-for="item in statusOptions.slice(1)" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field">
          <select v-model="queryForm.providerType" v-refresh-on-empty="loadRecords">
            <option value="">渠道</option>
            <option v-for="item in providerOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <div class="app-field push-page__keyword">
          <ClearableSearchInput v-model="queryForm.keyword" placeholder="输入配置名称、Webhook、AppID 或模板ID" @clear="loadRecords" />
        </div>
        <template #actions>
          <button class="app-button app-button--primary push-page__button unified-list-page__button unified-list-page__search-button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary push-page__button unified-list-page__button unified-list-page__search-button" @click="handleResetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
          <button
            v-permission="'push:config:create'"
            class="app-button app-button--success push-page__button unified-list-page__button unified-list-page__search-button"
            @click="openCreateDialog"
          >
            <el-icon><Plus /></el-icon>
            <span>新增配置</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <section class="push-page__summary unified-list-page__summary">
      <article v-for="card in metrics" :key="card.label" class="push-page__metric unified-list-page__metric" :class="[`push-page__metric--${card.accent}`, `unified-list-page__metric--${card.accent}`]">
        <span>{{ card.label }}</span>
        <strong>{{ card.value }}</strong>
      </article>
    </section>

    <PageCard>
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th>配置名称</th>
            <th>渠道</th>
            <th>凭证/接收人</th>
            <th>厂区 / 区域</th>
            <th>告警类型 / 等级</th>
            <th>生效时段</th>
            <th>限流规则</th>
            <th>重试策略</th>
            <th>状态</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="11" class="app-table__empty">{{ loading ? "加载中..." : "暂无数据" }}</td>
          </tr>
          <tr v-for="record in records" :key="record.id">
            <td>
              <div class="push-page__name-cell unified-list-page__name-cell">
                <strong>{{ record.configName }}</strong>
                <span>{{ getCredentialText(record) }}</span>
              </div>
            </td>
            <td><StatusTag :text="getProviderText(record.providerType)" :tone="getProviderTone(record.providerType)" /></td>
            <td>
              <div class="push-page__stack-cell unified-list-page__stack-cell">
                <strong>{{ getCredentialText(record) }}</strong>
                <span>{{ getReceiverText(record) }}</span>
              </div>
            </td>
            <td>
              <div class="push-page__stack-cell unified-list-page__stack-cell">
                <strong>{{ getFactoryNames(record.factoryIds) }}</strong>
                <span>{{ getZoneNames(record.zoneIds) }}</span>
              </div>
            </td>
            <td>
              <div class="push-page__stack-cell unified-list-page__stack-cell">
                <strong>{{ getAlarmTypeText(record.alarmTypes) }}</strong>
                <span>{{ getAlarmLevelText(record.alarmLevels) }}</span>
              </div>
            </td>
            <td>{{ getTimeRangeText(record) }}</td>
            <td>{{ record.rateLimitWindowSeconds }} 秒内最多 {{ record.rateLimitMaxCount }} 次</td>
            <td>{{ record.retryMaxCount }} 次重试 / 间隔 {{ record.retryIntervalSeconds }} 秒</td>
            <td><StatusTag :text="getEnabledText(record.enabled)" :tone="getEnabledTone(record.enabled)" /></td>
            <td>{{ formatDateTime(record.updatedAt) }}</td>
            <td>
              <div class="table-actions">
                <button
                  v-permission="'push:config:update'"
                  class="app-button app-button--secondary push-page__button unified-list-page__button unified-list-page__table-button"
                  @click="openEditDialog(record)"
                >
                  <el-icon><EditPen /></el-icon>
                  <span>编辑</span>
                </button>
                <button
                  v-permission="'push:config:test'"
                  class="app-button app-button--primary push-page__button unified-list-page__button unified-list-page__table-button"
                  :disabled="testingId === record.id"
                  @click="handleTestPush(record)"
                >
                  <el-icon><Connection /></el-icon>
                  <span>{{ testingId === record.id ? "测试中..." : "测试推送" }}</span>
                </button>
                <button
                  v-permission="'push:config:update'"
                  class="app-button app-button--warning push-page__button unified-list-page__button unified-list-page__table-button"
                  @click="handleToggleStatus(record)"
                >
                  <el-icon><SwitchButton /></el-icon>
                  <span>{{ record.enabled ? "停用" : "启用" }}</span>
                </button>
                <button
                  v-permission="'push:config:delete'"
                  class="app-button app-button--danger push-page__button unified-list-page__button unified-list-page__table-button"
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

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑推送配置' : '新增推送配置'"
      width="860px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formState" :rules="rules" label-width="112px" class="push-form">
        <el-form-item label="配置名称" prop="configName">
          <el-input v-model="formState.configName" placeholder="例如 高危告警值班群" />
        </el-form-item>
        <el-form-item label="推送渠道">
          <el-select v-model="formState.providerType" style="width: 100%" @change="handleProviderChange">
            <el-option v-for="item in providerOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="formState.providerType === 'dingtalk'" label="Webhook" prop="webhook">
          <el-input v-model="formState.webhook" placeholder="支持真实钉钉地址或 mock://dingtalk/success" />
        </el-form-item>
        <el-form-item v-if="formState.providerType === 'dingtalk'" label="Secret">
          <el-input
            v-model="formState.secret"
            type="password"
            show-password
            :placeholder="editingSecretConfigured ? '留空表示保持原 Secret' : '可选，用于启用钉钉加签'"
          />
        </el-form-item>
        <el-form-item v-if="formState.providerType === 'wechat'" label="AppID" prop="appId">
          <el-input v-model="formState.appId" placeholder="支持真实 AppID 或 mock://wechat/success-app" />
        </el-form-item>
        <el-form-item v-if="formState.providerType === 'wechat'" label="AppSecret" prop="appSecret">
          <el-input
            v-model="formState.appSecret"
            type="password"
            show-password
            :placeholder="editingAppSecretConfigured ? '留空表示保持原 AppSecret' : '请输入微信公众号 AppSecret'"
          />
        </el-form-item>
        <el-form-item v-if="formState.providerType === 'wechat'" label="模板ID" prop="templateId">
          <el-input v-model="formState.templateId" placeholder="例如 tmpl-alarm-001" />
        </el-form-item>
        <el-form-item v-if="formState.providerType === 'wechat'" label="接收人 OpenID" prop="receiverOpenIdsText">
          <el-input
            v-model="formState.receiverOpenIdsText"
            type="textarea"
            :rows="3"
            placeholder="每行一个 OpenID，或用逗号分隔多个接收人"
          />
        </el-form-item>
        <el-form-item label="关联厂区">
          <el-select v-model="formState.factoryIds" multiple clearable style="width: 100%" @change="handleFactoryChange">
            <el-option v-for="item in factories" :key="item.id" :label="item.factoryName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="关联区域">
          <el-select v-model="formState.zoneIds" multiple clearable style="width: 100%">
            <el-option v-for="item in filteredZoneOptions" :key="item.id" :label="item.zoneName" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="告警类型">
          <el-select v-model="formState.alarmTypes" multiple clearable style="width: 100%">
            <el-option v-for="item in alarmTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="告警等级">
          <el-select v-model="formState.alarmLevels" multiple clearable style="width: 100%">
            <el-option v-for="item in alarmLevelOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </el-form-item>
        <el-form-item label="生效时段">
          <el-time-picker
            v-model="timeRangeValue"
            is-range
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="HH:mm"
            format="HH:mm"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="是否启用">
          <el-switch v-model="formState.enabled" />
        </el-form-item>
        <el-form-item label="限流窗口">
          <el-input-number v-model="formState.rateLimitWindowSeconds" :min="0" :max="86400" style="width: 100%" />
        </el-form-item>
        <el-form-item label="窗口最大次数">
          <el-input-number v-model="formState.rateLimitMaxCount" :min="1" :max="100" style="width: 100%" />
        </el-form-item>
        <el-form-item label="重试次数">
          <el-input-number v-model="formState.retryMaxCount" :min="0" :max="10" style="width: 100%" />
        </el-form-item>
        <el-form-item label="重试间隔秒">
          <el-input-number v-model="formState.retryIntervalSeconds" :min="0" :max="60" style="width: 100%" />
        </el-form-item>
        <el-form-item label="备注" class="push-form__full">
          <el-input
            v-model="formState.remark"
            type="textarea"
            :rows="3"
            :placeholder="formState.providerType === 'wechat' ? '记录模板用途、负责人和接收规则' : '记录值班群说明、机器人用途或值守策略'"
          />
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
.push-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.push-page__summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.push-page__metric {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 18px 20px;
  border-radius: 12px;
  border: 1px solid #dbe6f0;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(17, 43, 74, 0.05);
}

.push-page__metric span {
  color: #667b91;
  font-size: 13px;
}

.push-page__metric strong {
  color: #0e2b4b;
  font-size: 30px;
}

.push-page__metric--success strong {
  color: #1d9b52;
}

.push-page__metric--danger strong {
  color: #d64f5a;
}

.push-page__metric--info strong {
  color: #1d7ad9;
}

.push-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.push-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.push-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.push-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px 180px minmax(320px, 1fr);
  gap: 10px;
  align-items: end;
}

.push-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.push-page__filters-card :deep(.app-field select),
.push-page__filters-card :deep(.app-field input) {
  height: 36px;
  font-size: 13px;
}

.push-page__filters-card .push-page__button {
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.push-page__keyword {
  grid-column: auto;
}

.push-page__name-cell,
.push-page__stack-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.push-page__name-cell strong,
.push-page__stack-cell strong {
  color: #163657;
}

.push-page__name-cell span,
.push-page__stack-cell span {
  color: #708398;
  font-size: 12px;
  line-height: 1.5;
}

.push-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px 16px;
}

.push-form :deep(.el-form-item) {
  margin-bottom: 18px;
}

.push-form__full {
  grid-column: 1 / -1;
}

@media (max-width: 1280px) {
  .push-page__summary {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 960px) {
  .push-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .push-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }

  .push-page__summary,
  .push-form {
    grid-template-columns: 1fr;
  }

  .push-page__keyword {
    grid-column: auto;
  }
}

@media (max-width: 768px) {
  .push-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
