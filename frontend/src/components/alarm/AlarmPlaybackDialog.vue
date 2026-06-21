<script setup lang="ts">
import { ElDialog, ElMessage } from "element-plus"
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue"

import PageCard from "../common/PageCard.vue"
import HikWebControlPlaybackPlayer from "../video/HikWebControlPlaybackPlayer.vue"
import {
  downloadPlaybackFileApi,
  getChannelLiveWebControlConfigApi,
  searchPlaybackSegmentsApi,
} from "../../api/video"
import type { AlarmRecord } from "../../types/alarm"
import type { LiveWebControlConfig, PlaybackTimelineSpan, StreamProfile } from "../../types/video"
import { formatDateTime } from "../../utils/datetime"

const props = defineProps<{
  modelValue: boolean
  alarm: AlarmRecord | null
}>()

const emit = defineEmits<{
  "update:modelValue": [value: boolean]
}>()

type HikPlaybackPlayerExpose = {
  startPlayback: (params: { startTime: string; endTime: string; streamType?: 1 | 2 }) => Promise<void>
  stopPlayback: (options?: { silent?: boolean }) => Promise<void>
  destroyPlayer: () => Promise<void>
  pausePlayback: () => Promise<void>
  resumePlayback: () => Promise<void>
}

const loading = ref(false)
const downloading = ref(false)
const hikPlaybackPlayerRef = ref<HikPlaybackPlayerExpose | null>(null)
const hikPlaybackConfig = ref<LiveWebControlConfig | null>(null)
const recordedSpans = ref<PlaybackTimelineSpan[]>([])
const currentOffsetSeconds = ref(0)
const currentPlayableEndOffsetSeconds = ref(0)
const fullscreenTargetRef = ref<HTMLElement | null>(null)
let playbackClockTimer: number | null = null
let playbackClockAnchorOffsetSeconds = 0
let playbackClockAnchorStartedAt = 0

const playback = reactive({
  isPlaying: false,
  isPaused: false,
  streamProfile: "main" as StreamProfile,
  message: "请选择告警后查看录像",
})

const canPlay = computed(
  () =>
    Boolean(
      props.alarm?.channelId
      && props.alarm?.recordStartTime
      && props.alarm?.recordEndTime,
    ),
)

const playbackTitle = computed(() => props.alarm?.alarmNo || "录像查看")
const playbackLocation = computed(() => {
  if (!props.alarm) return ""
  return [
    props.alarm.recorderName || "",
    props.alarm.channelName || "",
    props.alarm.cameraName || "",
  ]
    .filter(Boolean)
    .join(" / ")
})
const playerMessage = computed(() => "")

const closeDialog = () => emit("update:modelValue", false)

const pad = (value: number) => String(value).padStart(2, "0")

const formatDownloadLabelTime = (value?: string | null) => {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value.replace(/[^\d]/g, "").slice(0, 14)
  }
  return `${date.getFullYear()}${pad(date.getMonth() + 1)}${pad(date.getDate())}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
}

const buildAlarmDownloadFilename = () => {
  const alarmNo = props.alarm?.alarmNo?.trim() || "alarm"
  const startLabel = formatDownloadLabelTime(props.alarm?.recordStartTime)
  const endLabel = formatDownloadLabelTime(props.alarm?.recordEndTime)
  if (startLabel && endLabel) {
    return `${alarmNo}(${startLabel}-${endLabel}).mp4`
  }
  return `${alarmNo}.mp4`
}

const toSdkSearchDateTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value.replace("T", " ").slice(0, 19)
  }
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const getPlaybackBounds = () => {
  const start = props.alarm?.recordStartTime ? new Date(props.alarm.recordStartTime) : null
  const end = props.alarm?.recordEndTime ? new Date(props.alarm.recordEndTime) : null
  if (!start || !end || Number.isNaN(start.getTime()) || Number.isNaN(end.getTime()) || end <= start) {
    return null
  }
  return { start, end }
}

const getPlaybackDurationSeconds = () => {
  const bounds = getPlaybackBounds()
  if (!bounds) return 0
  return Math.max(0, Math.round((bounds.end.getTime() - bounds.start.getTime()) / 1000))
}

const getCurrentPlaybackLimitSeconds = () => {
  const totalDuration = getPlaybackDurationSeconds()
  if (currentPlayableEndOffsetSeconds.value <= 0) {
    return totalDuration
  }
  return Math.min(totalDuration, currentPlayableEndOffsetSeconds.value)
}

const clampOffsetSeconds = (offsetSeconds: number, maxOffsetSeconds = getPlaybackDurationSeconds()) =>
  Math.min(Math.max(0, Math.round(offsetSeconds)), Math.max(0, Math.round(maxOffsetSeconds)))

const stopPlaybackClock = () => {
  if (playbackClockTimer !== null) {
    window.clearInterval(playbackClockTimer)
    playbackClockTimer = null
  }
}

const syncPlaybackClock = (offsetSeconds: number) => {
  playbackClockAnchorOffsetSeconds = clampOffsetSeconds(offsetSeconds, getCurrentPlaybackLimitSeconds())
  playbackClockAnchorStartedAt = Date.now()
  currentOffsetSeconds.value = playbackClockAnchorOffsetSeconds
}

const startPlaybackClock = (offsetSeconds: number, playableEndOffsetSeconds = getCurrentPlaybackLimitSeconds()) => {
  currentPlayableEndOffsetSeconds.value = playableEndOffsetSeconds
  syncPlaybackClock(offsetSeconds)
  stopPlaybackClock()
  playbackClockTimer = window.setInterval(() => {
    if (!playback.isPlaying || playback.isPaused) {
      return
    }
    const elapsedSeconds = Math.max(0, (Date.now() - playbackClockAnchorStartedAt) / 1000)
    const nextOffset = clampOffsetSeconds(playbackClockAnchorOffsetSeconds + elapsedSeconds, getCurrentPlaybackLimitSeconds())
    currentOffsetSeconds.value = nextOffset
    if (nextOffset >= getCurrentPlaybackLimitSeconds()) {
      stopPlaybackClock()
    }
  }, 250)
}

const buildFallbackSpan = (): PlaybackTimelineSpan[] => {
  if (!props.alarm?.recordStartTime || !props.alarm?.recordEndTime) {
    return []
  }
  return [{
    startTime: props.alarm.recordStartTime,
    endTime: props.alarm.recordEndTime,
    recordType: "alarm",
    available: true,
  }]
}

const buildPlayablePoint = (requestedOffsetSeconds = 0) => {
  const bounds = getPlaybackBounds()
  const spans = [...recordedSpans.value]
    .filter((item) => item.available)
    .sort((left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime())
  if (!bounds || !spans.length) {
    return null
  }

  const axisStartMs = bounds.start.getTime()
  const axisEndMs = bounds.end.getTime()
  const rawTargetMs = axisStartMs + Math.max(0, Math.floor(requestedOffsetSeconds)) * 1000
  const clampedTargetMs = Math.min(Math.max(axisStartMs, rawTargetMs), Math.max(axisStartMs, axisEndMs - 1000))

  let bestSpan = spans[0]
  let snappedMs = new Date(bestSpan.startTime).getTime()
  let bestDistance = Number.POSITIVE_INFINITY

  for (const span of spans) {
    const spanStartMs = new Date(span.startTime).getTime()
    const spanEndMs = new Date(span.endTime).getTime()
    if (Number.isNaN(spanStartMs) || Number.isNaN(spanEndMs) || spanEndMs <= spanStartMs) {
      continue
    }
    const insideMs = Math.min(Math.max(clampedTargetMs, spanStartMs), Math.max(spanStartMs, spanEndMs - 1000))
    const distance = clampedTargetMs < spanStartMs
      ? spanStartMs - clampedTargetMs
      : clampedTargetMs >= spanEndMs
        ? clampedTargetMs - Math.max(spanStartMs, spanEndMs - 1000)
        : 0
    if (distance < bestDistance) {
      bestDistance = distance
      bestSpan = span
      snappedMs = insideMs
    }
    if (distance === 0) {
      break
    }
  }

  const snappedOffsetSeconds = Math.max(0, Math.round((snappedMs - axisStartMs) / 1000))
  const spanStartMs = new Date(bestSpan.startTime).getTime()
  const spanEndMs = new Date(bestSpan.endTime).getTime()
  return {
    span: bestSpan,
    targetTime: new Date(snappedMs).toISOString(),
    targetOffsetSeconds: snappedOffsetSeconds,
    spanStartOffsetSeconds: Math.max(0, Math.round((spanStartMs - axisStartMs) / 1000)),
    spanEndOffsetSeconds: Math.max(0, Math.round((spanEndMs - axisStartMs) / 1000)),
  }
}

const resolvePreferredStartOffset = () => {
  const bounds = getPlaybackBounds()
  if (!bounds) return 0
  const alarmTime = props.alarm?.alarmTime ? new Date(props.alarm.alarmTime) : null
  if (alarmTime && !Number.isNaN(alarmTime.getTime())) {
    const rawOffset = Math.round((alarmTime.getTime() - bounds.start.getTime()) / 1000)
    if (rawOffset >= 0 && rawOffset <= getPlaybackDurationSeconds()) {
      return rawOffset
    }
  }
  const firstAvailableSpan = recordedSpans.value[0]
  if (!firstAvailableSpan) return 0
  const spanStart = new Date(firstAvailableSpan.startTime)
  if (Number.isNaN(spanStart.getTime())) return 0
  return clampOffsetSeconds((spanStart.getTime() - bounds.start.getTime()) / 1000)
}

const resetPlayback = () => {
  playback.isPlaying = false
  playback.isPaused = false
  hikPlaybackConfig.value = null
  recordedSpans.value = []
  currentOffsetSeconds.value = 0
  currentPlayableEndOffsetSeconds.value = 0
  stopPlaybackClock()
  playback.message = canPlay.value ? "点击刷新可重新拉取回放流" : "当前告警缺少关联录像通道或时间段"
}

const ensureHikPlaybackConfig = async () => {
  if (!props.alarm?.channelId) {
    throw new Error("当前告警缺少关联通道，无法使用 SDK 回放。")
  }
  if (hikPlaybackConfig.value?.channelId === props.alarm.channelId) {
    return hikPlaybackConfig.value
  }
  const config = await getChannelLiveWebControlConfigApi(props.alarm.channelId, {
    streamProfile: playback.streamProfile,
  })
  hikPlaybackConfig.value = {
    ...config,
    streamType: 1,
    streamProfile: playback.streamProfile,
  }
  return hikPlaybackConfig.value
}

const prepareDialogPlaybackSession = async () => {
  await nextTick()
  if (!hikPlaybackPlayerRef.value) {
    return
  }
  try {
    await hikPlaybackPlayerRef.value.destroyPlayer()
  } catch {
    // Ignore teardown failures before rebuilding the dialog playback session.
  }
}

const loadRecordedSpans = async () => {
  if (!props.alarm?.channelId || !props.alarm.recordStartTime || !props.alarm.recordEndTime) {
    recordedSpans.value = []
    return
  }
  try {
    const items = await searchPlaybackSegmentsApi({
      recorder_id: props.alarm.recorderId ?? undefined,
      channel_id: props.alarm.channelId,
      start_time: props.alarm.recordStartTime,
      end_time: props.alarm.recordEndTime,
    })
    recordedSpans.value = items.length
      ? items.map((item) => ({
        startTime: item.startTime,
        endTime: item.endTime,
        recordType: item.recordType,
        available: item.available,
      })).sort((left, right) => new Date(left.startTime).getTime() - new Date(right.startTime).getTime())
      : buildFallbackSpan()
  } catch {
    recordedSpans.value = buildFallbackSpan()
  }
}

const waitFor = async (ms: number) =>
  await new Promise<void>((resolve) => {
    window.setTimeout(resolve, ms)
  })

const resolveErrorMessage = (error: unknown, fallback: string) => {
  const responseData = (error as { response?: { data?: { detail?: string; message?: string } } })?.response?.data
  if (typeof responseData?.message === "string" && responseData.message) return responseData.message
  if (typeof responseData?.detail === "string" && responseData.detail) return responseData.detail
  if (typeof error === "string" && error && error !== "undefined") return error
  if (error instanceof Error && error.message) return error.message
  return fallback
}

const resolveAsyncErrorMessage = async (error: unknown, fallback: string) => {
  const blobData = (error as { response?: { data?: unknown } })?.response?.data
  if (blobData instanceof Blob) {
    try {
      const rawText = await blobData.text()
      if (rawText) {
        try {
          const parsed = JSON.parse(rawText) as { detail?: string; message?: string }
          if (typeof parsed.detail === "string" && parsed.detail) {
            return parsed.detail
          }
          if (typeof parsed.message === "string" && parsed.message) {
            return parsed.message
          }
        } catch {
          if (rawText !== "undefined") {
            return rawText
          }
        }
      }
    } catch {
      return fallback
    }
  }
  const directMessage = resolveErrorMessage(error, "")
  if (directMessage) {
    return directMessage
  }
  return fallback
}

const stopPlayback = async (options?: { silent?: boolean; preserveOffset?: boolean }) => {
  stopPlaybackClock()
  playback.isPlaying = false
  playback.isPaused = false
  if (!options?.preserveOffset) {
    currentOffsetSeconds.value = 0
  }
  if (!hikPlaybackPlayerRef.value) return
  try {
    await hikPlaybackPlayerRef.value.stopPlayback({ silent: options?.silent ?? true })
  } catch (error) {
    if (!options?.silent) {
      throw error
    }
  }
}

const handleDownload = async () => {
  if (!props.alarm || !canPlay.value) {
    ElMessage.warning("当前告警缺少有效的录像时间段，无法下载")
    return
  }
  downloading.value = true
  try {
    await downloadPlaybackFileApi({
      alarm_no: props.alarm.alarmNo,
      recorder_id: props.alarm.recorderId ?? undefined,
      channel_id: props.alarm.channelId ?? undefined,
      camera_id: props.alarm.cameraId ?? undefined,
      start_time: props.alarm.recordStartTime,
      end_time: props.alarm.recordEndTime,
      stream_profile: playback.streamProfile,
    }, {
      filename: buildAlarmDownloadFilename(),
    })
    ElMessage.success("已成功下载该告警录像")
  } catch (error) {
    ElMessage.error(await resolveAsyncErrorMessage(error, "下载录像失败"))
  } finally {
    downloading.value = false
  }
}

const shouldRetryDialogPlayback = (error: unknown) => {
  const message = resolveErrorMessage(error, "")
  return message.includes("设备请求失败") || message.includes("启动 HIK 回放")
}

const startDialogPlayback = async (offsetSeconds?: number) => {
  if (!props.alarm) return
  if (!canPlay.value) {
    playback.message = "当前告警缺少关联录像通道或时间段"
    ElMessage.warning(playback.message)
    return
  }
  playback.message = "正在使用 SDK 拉取关联录像，请稍候"
  await nextTick()
  if (!hikPlaybackPlayerRef.value) {
    throw new Error("SDK 播放器尚未就绪，请稍后重试。")
  }
  const [config] = await Promise.all([
    ensureHikPlaybackConfig(),
    loadRecordedSpans(),
  ])
  const requestedOffsetSeconds = offsetSeconds ?? resolvePreferredStartOffset()
  const targetPoint = buildPlayablePoint(requestedOffsetSeconds)
  if (!targetPoint) {
    throw new Error("当前告警时间范围内没有可播放的录像片段。")
  }
  await hikPlaybackPlayerRef.value.startPlayback({
    startTime: toSdkSearchDateTime(targetPoint.targetTime),
    endTime: toSdkSearchDateTime(targetPoint.span.endTime),
    streamType: config.streamType,
  })
  playback.isPlaying = true
  playback.isPaused = false
  startPlaybackClock(targetPoint.targetOffsetSeconds, targetPoint.spanEndOffsetSeconds)
  playback.message = targetPoint.targetOffsetSeconds !== clampOffsetSeconds(requestedOffsetSeconds)
    ? "该时段无录像，已跳转到最近录像位置"
    : "已切换为 SDK 回放，可直接拖动时间轴定位录像"
}

const openPlayback = async (offsetSeconds?: number) => {
  loading.value = true
  try {
    await startDialogPlayback(offsetSeconds)
  } catch (error) {
    if (shouldRetryDialogPlayback(error)) {
      try {
        playback.message = "正在重新初始化 SDK 回放会话，请稍候"
        await prepareDialogPlaybackSession()
        await waitFor(250)
        await startDialogPlayback(offsetSeconds)
        return
      } catch (retryError) {
        playback.isPlaying = false
        playback.isPaused = false
        playback.message = resolveErrorMessage(retryError, "拉取关联录像失败")
        ElMessage.error(playback.message)
        return
      }
    }
    playback.isPlaying = false
    playback.isPaused = false
    playback.message = resolveErrorMessage(error, "拉取关联录像失败")
    ElMessage.error(playback.message)
  } finally {
    loading.value = false
  }
}

const handlePauseToggle = async () => {
  if (!playback.isPlaying || !hikPlaybackPlayerRef.value) {
    playback.message = "当前没有正在播放的 SDK 回放"
    return
  }
  loading.value = true
  try {
    if (playback.isPaused) {
      await hikPlaybackPlayerRef.value.resumePlayback()
      playback.isPaused = false
      startPlaybackClock(currentOffsetSeconds.value, getCurrentPlaybackLimitSeconds())
      playback.message = "SDK 回放已继续"
    } else {
      await hikPlaybackPlayerRef.value.pausePlayback()
      playback.isPaused = true
      syncPlaybackClock(currentOffsetSeconds.value)
      stopPlaybackClock()
      playback.message = "SDK 回放已暂停"
    }
  } catch (error) {
    playback.message = resolveErrorMessage(error, playback.isPaused ? "SDK 回放继续失败" : "SDK 回放暂停失败")
    ElMessage.error(playback.message)
  } finally {
    loading.value = false
  }
}

const handleSeek = async (offsetSeconds: number) => {
  if (loading.value) return
  await openPlayback(offsetSeconds)
}

const handleSeekEnd = async (_offsetSeconds: number) => {
  await stopPlayback({ silent: true })
  currentOffsetSeconds.value = getCurrentPlaybackLimitSeconds()
  playback.message = "已到当前录像片段结束位置，SDK 回放已停止"
}

const handlePlaybackEnded = async () => {
  stopPlaybackClock()
  currentOffsetSeconds.value = getCurrentPlaybackLimitSeconds()
  playback.isPlaying = false
  playback.isPaused = false
  await stopPlayback({ silent: true, preserveOffset: true })
  playback.message = "当前录像片段已播放结束"
}

const handleFullscreen = async () => {
  const target = fullscreenTargetRef.value
  if (!target) return
  if (document.fullscreenElement === target) {
    await document.exitFullscreen()
    return
  }
  await target.requestFullscreen()
}

watch(
  () => props.modelValue,
  async (visible) => {
    if (visible) {
      resetPlayback()
      await prepareDialogPlaybackSession()
      await openPlayback()
      return
    }
    await stopPlayback({ silent: true })
    resetPlayback()
  },
)

watch(
  () => props.alarm?.id,
  async (nextId, previousId) => {
    if (!props.modelValue || !nextId || nextId === previousId) return
    if (previousId) {
      await stopPlayback({ silent: true })
    }
    resetPlayback()
    await prepareDialogPlaybackSession()
    await openPlayback()
  },
)

onBeforeUnmount(() => {
  stopPlaybackClock()
})
</script>

<template>
  <ElDialog
    :model-value="modelValue"
    title="录像查看"
    width="1100px"
    top="3vh"
    destroy-on-close
    append-to-body
    @close="closeDialog"
  >
    <div class="alarm-playback-dialog">
      <div class="alarm-playback-dialog__toolbar">
        <div class="alarm-playback-dialog__meta">
          <span>告警编号：{{ alarm?.alarmNo || "-" }}</span>
          <span>时间段：{{ formatDateTime(alarm?.recordStartTime) }} 至 {{ formatDateTime(alarm?.recordEndTime) }}</span>
        </div>
        <div class="alarm-playback-dialog__actions">
          <button class="app-button app-button--secondary alarm-playback-dialog__button" :disabled="loading" @click="handlePauseToggle">
            {{ playback.isPaused ? "继续回放" : "暂停回放" }}
          </button>
          <button class="app-button app-button--secondary alarm-playback-dialog__button" :disabled="downloading || !canPlay" @click="handleDownload">
            {{ downloading ? "下载中..." : "下载录像" }}
          </button>
          <button class="app-button app-button--secondary alarm-playback-dialog__button" :disabled="loading" @click="() => void stopPlayback()">
            停止
          </button>
          <button class="app-button app-button--secondary alarm-playback-dialog__button" @click="() => void handleFullscreen()">
            全屏
          </button>
          <button class="app-button app-button--secondary alarm-playback-dialog__button" :disabled="loading" @click="() => void openPlayback()">
            {{ loading ? "加载中..." : "刷新回放" }}
          </button>
        </div>
      </div>

      <PageCard>
        <div ref="fullscreenTargetRef" class="alarm-playback-dialog__player">
          <HikWebControlPlaybackPlayer
            ref="hikPlaybackPlayerRef"
            :config="hikPlaybackConfig"
            :message="playerMessage"
            :playback-start-time="alarm?.recordStartTime || ''"
            :playback-end-time="alarm?.recordEndTime || ''"
            :recorded-spans="recordedSpans"
            :current-offset-seconds="currentOffsetSeconds"
            :playback-ended-error-codes="[1011]"
            @seek="handleSeek"
            @seek-end="handleSeekEnd"
            @playback-end="handlePlaybackEnded"
            @toggle-fullscreen="handleFullscreen"
          />
        </div>
        <div class="alarm-playback-dialog__caption">
          <strong>{{ playbackTitle }}</strong>
          <span>{{ playbackLocation || "未识别位置信息" }}</span>
        </div>
      </PageCard>
    </div>
  </ElDialog>
</template>

<style scoped>
.alarm-playback-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.alarm-playback-dialog__toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.alarm-playback-dialog__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: #60778f;
  font-size: 12px;
}

.alarm-playback-dialog__actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.alarm-playback-dialog__button {
  min-height: 34px;
  padding: 0 14px;
  font-size: 13px;
}

.alarm-playback-dialog :deep(.page-card__body) {
  height: 680px;
}

.alarm-playback-dialog__player {
  height: 100%;
  min-height: 680px;
}

.alarm-playback-dialog__caption {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-top: 10px;
  color: #60778f;
  font-size: 12px;
}
</style>
