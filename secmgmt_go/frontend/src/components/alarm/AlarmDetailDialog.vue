<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue"
import { useRouter } from "vue-router"
import { ElDialog, ElMessage } from "element-plus"

import PageCard from "../common/PageCard.vue"
import StatusTag from "../common/StatusTag.vue"
import { falseAlarmApi, processAlarmApi, repushAlarmApi } from "../../api/alarm"
import { http } from "../../api/http"
import type { AlarmDetail } from "../../types/alarm"
import { formatDateTime } from "../../utils/datetime"

const props = defineProps<{
  modelValue: boolean
  detail: AlarmDetail | null
  loading?: boolean
}>()

const emit = defineEmits<{
  "update:modelValue": [value: boolean]
  refreshed: []
}>()

const router = useRouter()
const actionLoading = computed(() => props.loading ?? false)
const imageState = ref<"idle" | "loading" | "loaded" | "error">("idle")
const displayImageUrl = ref<string | null>(null)
const imagePreviewVisible = ref(false)
let imageRequestId = 0

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

const getPushChannelText = (value?: string | null) => {
  if (value === "dingtalk") return "钉钉"
  if (value === "wechat") return "微信公众号"
  return value || "-"
}

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const detail = (error as { response?: { data?: { detail?: string } } })?.response?.data?.detail
  if (typeof detail === "string" && detail) return detail
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const closeDialog = () => emit("update:modelValue", false)

const closeImagePreview = () => {
  imagePreviewVisible.value = false
}

const revokeDisplayImageUrl = () => {
  if (displayImageUrl.value?.startsWith("blob:")) {
    URL.revokeObjectURL(displayImageUrl.value)
  }
  displayImageUrl.value = null
}

const handleImageLoaded = () => {
  imageState.value = "loaded"
}

const handleImageError = () => {
  imageState.value = "error"
}

const handleOpenImagePreview = () => {
  if (!displayImageUrl.value || imageState.value !== "loaded") return
  imagePreviewVisible.value = true
}

const loadImage = async () => {
  const imageUrl = props.detail?.imageUrl
  imageRequestId += 1
  const currentRequestId = imageRequestId
  revokeDisplayImageUrl()

  if (!props.modelValue || !imageUrl) {
    imageState.value = "idle"
    imagePreviewVisible.value = false
    return
  }

  imageState.value = "loading"

  try {
    const response = await http.get<Blob>(imageUrl, {
      responseType: "blob",
    })
    if (currentRequestId !== imageRequestId) return
    displayImageUrl.value = URL.createObjectURL(response.data)
  } catch {
    if (currentRequestId !== imageRequestId) return
    imageState.value = "error"
    imagePreviewVisible.value = false
  }
}

const refreshAfterAction = async (action: Promise<unknown>, successMessage: string) => {
  try {
    await action
    ElMessage.success(successMessage)
    emit("refreshed")
  } catch (error) {
    ElMessage.error(resolveErrorMessage(error, successMessage.replace("成功", "失败")))
  }
}

const handleProcess = async (nextStatus: "processing" | "done") => {
  if (!props.detail) return
  await refreshAfterAction(
    processAlarmApi(props.detail.id, {
      status: nextStatus,
      remark: nextStatus === "processing" ? "进入处理中" : "已完成处置",
    }),
    nextStatus === "processing" ? "已标记处理中" : "已标记为已处理",
  )
}

const handleFalseAlarm = async () => {
  if (!props.detail) return
  await refreshAfterAction(falseAlarmApi(props.detail.id, { remark: "手动标记误报" }), "已标记为误报")
}

const handleRepush = async () => {
  if (!props.detail) return
  await refreshAfterAction(repushAlarmApi(props.detail.id), "已重新推送告警")
}

const handleViewPlayback = async () => {
  if (!props.detail) return
  await router.push({
    name: "monitor-playback",
    query: {
      recorderId: props.detail.recorderId ?? undefined,
      channelId: props.detail.channelId ?? undefined,
      cameraId: props.detail.cameraId ?? undefined,
      startTime: props.detail.recordStartTime ?? undefined,
      endTime: props.detail.recordEndTime ?? undefined,
      recordType: "alarm",
    },
  })
  closeDialog()
}

watch(
  () => [props.modelValue, props.detail?.id, props.detail?.imageUrl],
  () => {
    void loadImage()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  imageRequestId += 1
  revokeDisplayImageUrl()
})
</script>

<template>
  <el-dialog :model-value="modelValue" title="告警详情" width="960px" top="6vh" @close="closeDialog">
    <div v-if="detail" class="alarm-detail">
      <PageCard title="告警信息" description="展示告警基础信息、状态和关联录像时间。">
        <div class="alarm-detail__grid">
          <div class="alarm-detail__item"><span>告警编号</span><strong>{{ detail.alarmNo }}</strong></div>
          <div class="alarm-detail__item"><span>告警类型</span><strong>{{ detail.alarmType }}</strong></div>
          <div class="alarm-detail__item">
            <span>告警等级</span>
            <StatusTag :text="getLevelText(detail.alarmLevel)" :tone="getLevelTone(detail.alarmLevel)" />
          </div>
          <div class="alarm-detail__item">
            <span>状态</span>
            <StatusTag :text="getStatusText(detail.status)" :tone="getStatusTone(detail.status)" />
          </div>
          <div class="alarm-detail__item"><span>告警时间</span><strong>{{ formatDateTime(detail.alarmTime) }}</strong></div>
          <div class="alarm-detail__item"><span>重复次数</span><strong>{{ detail.occurrenceCount }}</strong></div>
          <div class="alarm-detail__item"><span>关联录像开始</span><strong>{{ formatDateTime(detail.recordStartTime) }}</strong></div>
          <div class="alarm-detail__item"><span>关联录像结束</span><strong>{{ formatDateTime(detail.recordEndTime) }}</strong></div>
        </div>
      </PageCard>

      <div class="alarm-detail__split">
        <PageCard title="设备与区域" description="返回摄像机、录像机、通道、厂区和区域信息。">
          <div class="alarm-detail__stack">
            <div class="alarm-detail__row"><span>摄像机</span><strong>{{ detail.cameraName || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>录像机</span><strong>{{ detail.recorderName || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>通道</span><strong>{{ detail.channelName || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>厂区</span><strong>{{ detail.factoryName || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>区域</span><strong>{{ detail.zoneName || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>告警说明</span><strong>{{ detail.message || "-" }}</strong></div>
          </div>
        </PageCard>

        <PageCard title="抓拍图" description="没有真实抓拍图时显示图片地址或占位。">
          <div class="alarm-detail__media">
            <div class="alarm-detail__media-preview">
              <img
                v-if="displayImageUrl"
                v-show="imageState !== 'error'"
                :src="displayImageUrl"
                alt="抓拍图"
                title="双击查看大图"
                @dblclick="handleOpenImagePreview"
                @load="handleImageLoaded"
                @error="handleImageError"
              />
              <div v-if="detail.imageUrl && imageState === 'loading'" class="alarm-detail__media-state">
                抓拍图加载中...
              </div>
              <div v-else-if="detail.imageUrl && imageState === 'error'" class="alarm-detail__media-state alarm-detail__media-state--error">
                抓拍图加载失败，请检查图片地址是否可访问
              </div>
              <div v-else-if="!detail.imageUrl" class="alarm-detail__media-empty">暂无抓拍图</div>
            </div>
            <code>{{ detail.imageUrl || "暂无图片地址" }}</code>
          </div>
        </PageCard>
      </div>

      <div class="alarm-detail__split">
        <PageCard title="AI事件信息" description="返回原始 AI 事件概要信息。">
          <div class="alarm-detail__stack">
            <div class="alarm-detail__row"><span>事件编号</span><strong>{{ detail.aiEvent?.eventNo || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>事件类型</span><strong>{{ detail.aiEvent?.eventType || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>事件来源</span><strong>{{ detail.aiEvent?.sourceType || "-" }}</strong></div>
            <div class="alarm-detail__row"><span>置信度</span><strong>{{ detail.aiEvent?.confidence ?? "-" }}</strong></div>
          </div>
        </PageCard>

        <PageCard title="推送记录" description="展示告警自动推送、限流跳过和人工重推的历史记录。">
          <div class="alarm-detail__list">
            <div v-if="!detail.pushRecords.length" class="alarm-detail__empty">暂无推送记录</div>
            <div v-for="item in detail.pushRecords" :key="`${item.time}-${item.channel}`" class="alarm-detail__list-item">
              <strong>{{ getPushChannelText(item.channel) }}</strong>
              <span>{{ item.status }} / {{ formatDateTime(item.time) }}</span>
              <p>{{ item.message }}</p>
            </div>
          </div>
        </PageCard>
      </div>

      <PageCard title="处理记录" description="所有状态流转和重新推送动作都会记录在这里。">
        <div class="alarm-detail__list">
          <div v-if="!detail.processLogs.length" class="alarm-detail__empty">暂无处理记录</div>
          <div v-for="item in detail.processLogs" :key="item.id" class="alarm-detail__list-item">
            <strong>{{ item.action }}</strong>
            <span>{{ formatDateTime(item.createdAt) }} / {{ item.operatorName || "系统" }}</span>
            <p>{{ item.fromStatus || "-" }} -> {{ item.toStatus || "-" }}{{ item.remark ? ` / ${item.remark}` : "" }}</p>
          </div>
        </div>
      </PageCard>

      <div class="alarm-detail__actions">
        <button
          v-permission="'alarm:process'"
          class="app-button app-button--warning"
          :disabled="actionLoading"
          @click="handleProcess('processing')"
        >
          标记处理中
        </button>
        <button
          v-permission="'alarm:process'"
          class="app-button app-button--success"
          :disabled="actionLoading"
          @click="handleProcess('done')"
        >
          标记已处理
        </button>
        <button
          v-permission="'alarm:process'"
          class="app-button app-button--secondary"
          :disabled="actionLoading"
          @click="handleFalseAlarm"
        >
          标记误报
        </button>
        <button
          v-permission="'alarm:repush'"
          class="app-button app-button--secondary"
          :disabled="actionLoading"
          @click="handleRepush"
        >
          重新推送
        </button>
        <button class="app-button app-button--primary" @click="handleViewPlayback">查看录像</button>
      </div>
    </div>
  </el-dialog>
  <el-dialog
    :model-value="imagePreviewVisible"
    title="抓拍图预览"
    width="80vw"
    append-to-body
    destroy-on-close
    @close="closeImagePreview"
  >
    <div class="alarm-detail__image-preview-dialog">
      <img v-if="displayImageUrl" :src="displayImageUrl" alt="抓拍图大图预览" class="alarm-detail__image-preview-full" />
    </div>
  </el-dialog>
</template>

<style scoped>
.alarm-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.alarm-detail__grid,
.alarm-detail__split {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.alarm-detail__item,
.alarm-detail__row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  padding: 10px 0;
  border-bottom: 1px solid #edf3f8;
}

.alarm-detail__item span,
.alarm-detail__row span {
  color: #6b8097;
  font-size: 13px;
  font-weight: 700;
}

.alarm-detail__item strong,
.alarm-detail__row strong {
  color: #123654;
  text-align: right;
}

.alarm-detail__stack,
.alarm-detail__list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.alarm-detail__list-item {
  padding: 12px 14px;
  border: 1px solid #dbe6f0;
  border-radius: 10px;
  background: #f8fbfe;
}

.alarm-detail__list-item strong {
  color: #14395b;
}

.alarm-detail__list-item span {
  display: block;
  margin-top: 4px;
  color: #6a8097;
  font-size: 12px;
}

.alarm-detail__list-item p {
  margin: 8px 0 0;
  color: #41596f;
  font-size: 13px;
}

.alarm-detail__empty,
.alarm-detail__media-empty {
  padding: 18px;
  text-align: center;
  color: #73879c;
  background: #f5f8fb;
  border-radius: 10px;
}

.alarm-detail__media {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.alarm-detail__media-preview {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 280px;
  max-height: 280px;
  padding: 12px;
  border: 1px solid #dbe6f0;
  border-radius: 10px;
  background: #f8fbfe;
  overflow: hidden;
}

.alarm-detail__media img {
  width: 100%;
  height: 100%;
  max-height: 256px;
  object-fit: contain;
  border-radius: 10px;
  cursor: zoom-in;
}

.alarm-detail__media-state,
.alarm-detail__media-empty {
  width: 100%;
  min-height: 120px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.alarm-detail__media-state {
  padding: 18px;
  text-align: center;
  color: #73879c;
}

.alarm-detail__media-state--error {
  color: #b5475a;
  background: #fff5f6;
  border-radius: 10px;
}

.alarm-detail__media code {
  padding: 10px 12px;
  border-radius: 10px;
  background: #f5f8fb;
  color: #164064;
  word-break: break-all;
}

.alarm-detail__image-preview-dialog {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
}

.alarm-detail__image-preview-full {
  max-width: 100%;
  max-height: 70vh;
  object-fit: contain;
  border-radius: 10px;
}

.alarm-detail__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
}

@media (max-width: 960px) {
  .alarm-detail__grid,
  .alarm-detail__split {
    grid-template-columns: 1fr;
  }
}
</style>
