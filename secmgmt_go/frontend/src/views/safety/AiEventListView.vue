<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { ElDialog, ElMessage } from "element-plus"
import { RefreshRight, Search, View } from "@element-plus/icons-vue"

import PageCard from "../../components/common/PageCard.vue"
import SearchForm from "../../components/common/SearchForm.vue"
import StatusTag from "../../components/common/StatusTag.vue"
import { getAiEventDetailApi, listAiEventsApi } from "../../api/ai-event"
import type { AiEventRecord } from "../../types/ai-event"
import { formatDateTime } from "../../utils/datetime"

const loading = ref(false)
const records = ref<AiEventRecord[]>([])
const detailVisible = ref(false)
const activeEvent = ref<AiEventRecord | null>(null)

const queryForm = reactive({
  keyword: "",
  sourceType: "",
  eventType: "",
  eventLevel: "",
})

const sourceOptions = ["hikvision-camera", "hikvision-isapi", "third-party-ai", "frame-model"]
const eventLevelOptions = [
  { label: "全部", value: "" },
  { label: "严重", value: "critical" },
  { label: "高", value: "high" },
  { label: "中", value: "medium" },
  { label: "低", value: "low" },
]

const getLevelText = (value: string) => {
  if (value === "critical") return "严重"
  if (value === "high") return "高"
  if (value === "medium") return "中"
  if (value === "low") return "低"
  return value
}

const getLevelTone = (value: string) => {
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
    records.value = await listAiEventsApi({
      keyword: queryForm.keyword || undefined,
      source_type: queryForm.sourceType || undefined,
      event_type: queryForm.eventType || undefined,
      event_level: queryForm.eventLevel || undefined,
    })
  } finally {
    loading.value = false
  }
}

const resetQuery = async () => {
  queryForm.keyword = ""
  queryForm.sourceType = ""
  queryForm.eventType = ""
  queryForm.eventLevel = ""
  await loadRecords()
}

const openDetail = async (record: AiEventRecord) => {
  try {
    activeEvent.value = await getAiEventDetailApi(record.id)
    detailVisible.value = true
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, "获取事件详情失败"))
  }
}

onMounted(async () => {
  await loadRecords()
})
</script>

<template>
  <div class="ai-event-list-page unified-list-page">
    <PageCard
      class="ai-event-list-page__filters-card"
      title="智能事件查询"
      description="集中查看海康设备、ISAPI、第三方 AI 或抽帧模型回传的智能分析事件。"
    >
      <SearchForm class="unified-list-page__search-form">
        <div class="app-field">
          <label>关键字</label>
          <input v-model="queryForm.keyword" type="text" placeholder="事件编号 / 类型 / 去重键" />
        </div>
        <div class="app-field">
          <label>事件来源</label>
          <select v-model="queryForm.sourceType">
            <option value="">全部</option>
            <option v-for="item in sourceOptions" :key="item" :value="item">{{ item }}</option>
          </select>
        </div>
        <div class="app-field">
          <label>事件类型</label>
          <input v-model="queryForm.eventType" type="text" placeholder="如 helmet_missing / fire" />
        </div>
        <div class="app-field">
          <label>告警等级</label>
          <select v-model="queryForm.eventLevel">
            <option v-for="item in eventLevelOptions" :key="item.value" :value="item.value">{{ item.label }}</option>
          </select>
        </div>
        <template #actions>
          <button class="app-button app-button--primary ai-event-list-page__button unified-list-page__button unified-list-page__search-button" @click="loadRecords">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </button>
          <button class="app-button app-button--secondary ai-event-list-page__button unified-list-page__button unified-list-page__search-button" @click="resetQuery">
            <el-icon><RefreshRight /></el-icon>
            <span>重置</span>
          </button>
        </template>
      </SearchForm>
    </PageCard>

    <PageCard title="事件列表" description="支持查看设备匹配结果、置信度、原始 JSON 和事件发生位置。">
      <table class="app-table unified-list-page__table">
        <thead>
          <tr>
            <th>事件编号</th>
            <th>事件类型</th>
            <th>等级</th>
            <th>来源</th>
            <th>设备</th>
            <th>厂区/区域</th>
            <th>置信度</th>
            <th>事件时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!records.length">
            <td colspan="9" class="app-table__empty">{{ loading ? "加载中..." : "暂无事件" }}</td>
          </tr>
          <tr v-for="item in records" :key="item.id">
            <td>{{ item.eventNo }}</td>
            <td>
              <div class="unified-list-page__name-cell">
                <strong>{{ item.eventType }}</strong>
                <span>{{ item.eventNo }}</span>
              </div>
            </td>
            <td><StatusTag :text="getLevelText(item.eventLevel)" :tone="getLevelTone(item.eventLevel)" /></td>
            <td>{{ item.sourceType }}</td>
            <td>
              <div class="unified-list-page__name-cell">
                <strong>{{ item.cameraName || item.channelName || "-" }}</strong>
                <span>{{ item.sourceType }}</span>
              </div>
            </td>
            <td>{{ item.factoryName || "-" }} / {{ item.zoneName || "-" }}</td>
            <td>{{ item.confidence ?? "-" }}</td>
            <td>{{ formatDateTime(item.eventTime) }}</td>
            <td>
              <button class="app-button app-button--secondary ai-event-list-page__button unified-list-page__button unified-list-page__table-button" @click="openDetail(item)">
                <el-icon><View /></el-icon>
                <span>原始JSON</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </PageCard>

    <el-dialog v-model="detailVisible" title="智能事件详情" width="760px">
      <div class="ai-event-detail">
        <div class="ai-event-detail__header">
          <strong>{{ activeEvent?.eventNo }}</strong>
          <span>{{ activeEvent?.cameraName || activeEvent?.channelName || "未匹配设备" }}</span>
        </div>
        <pre>{{ activeEvent?.rawJson }}</pre>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.ai-event-list-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.ai-event-list-page__button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 14px;
  font-size: 13px;
  white-space: nowrap;
}

.ai-event-list-page__filters-card :deep(.page-card__body) {
  padding: 14px 16px;
}

.ai-event-list-page__filters-card :deep(.search-form) {
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 10px;
  align-items: end;
}

.ai-event-list-page__filters-card :deep(.search-form__fields) {
  grid-template-columns: 180px minmax(220px, 1fr) minmax(220px, 1fr) 160px;
  gap: 10px;
  align-items: end;
}

.ai-event-list-page__filters-card :deep(.search-form__actions) {
  flex-wrap: nowrap;
  justify-content: flex-end;
  align-items: end;
}

.ai-event-list-page__filters-card :deep(.app-field label) {
  margin-bottom: 6px;
  font-size: 12px;
}

.ai-event-list-page__filters-card :deep(.app-field input),
.ai-event-list-page__filters-card :deep(.app-field select) {
  height: 36px;
  font-size: 13px;
}

.ai-event-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ai-event-detail__header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.ai-event-detail pre {
  margin: 0;
  padding: 14px;
  max-height: 420px;
  overflow: auto;
  border-radius: 10px;
  background: #0f2237;
  color: #dbeeff;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 1100px) {
  .ai-event-list-page__filters-card :deep(.search-form) {
    grid-template-columns: 1fr;
  }

  .ai-event-list-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 768px) {
  .ai-event-list-page__filters-card :deep(.search-form__fields) {
    grid-template-columns: 1fr;
  }
}
</style>
