<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{
  title?: string
  playUrl?: string | null
  message?: string
  status?: string
}>()

const statusText = computed(() => {
  if (props.status === "online") {
    return "在线"
  }
  if (props.status === "offline") {
    return "离线"
  }
  if (props.status === "exception") {
    return "异常"
  }
  if (props.status === "disabled") {
    return "停用"
  }
  return "未选择"
})
</script>

<template>
  <section class="video-player-card">
    <header class="video-player-card__header">
      <div>
        <h4>{{ props.title || "预览播放器" }}</h4>
        <p>{{ props.message || "当前阶段使用异步播放器壳组件承接预览地址，后续可替换为真实播放内核。" }}</p>
      </div>
      <span class="video-player-card__status">{{ statusText }}</span>
    </header>

    <div class="video-player-card__screen">
      <div class="video-player-card__watermark">HIKVISION PREVIEW</div>
      <div class="video-player-card__hint">
        <strong>{{ props.playUrl ? "已生成 RTSP 地址" : "尚未生成播放地址" }}</strong>
        <span>{{ props.playUrl || "请先执行连接测试以生成模拟 RTSP 地址。" }}</span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.video-player-card {
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(97, 127, 158, 0.24);
  background: #0f243d;
}

.video-player-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  color: #d5e9ff;
  background: rgba(255, 255, 255, 0.04);
}

.video-player-card__header h4 {
  margin: 0 0 4px;
  font-size: 15px;
}

.video-player-card__header p {
  margin: 0;
  font-size: 12px;
  color: #aac8e7;
}

.video-player-card__status {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
  color: #d9ebff;
  background: rgba(90, 157, 224, 0.18);
  white-space: nowrap;
}

.video-player-card__screen {
  position: relative;
  min-height: 280px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background:
    radial-gradient(circle at top, rgba(51, 123, 197, 0.22), transparent 42%),
    linear-gradient(180deg, #102640 0%, #091727 100%);
}

.video-player-card__watermark {
  position: absolute;
  top: 16px;
  right: 18px;
  font-size: 12px;
  letter-spacing: 1px;
  color: rgba(217, 235, 255, 0.42);
}

.video-player-card__hint {
  max-width: 560px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  text-align: center;
  color: #eaf4ff;
}

.video-player-card__hint strong {
  font-size: 18px;
}

.video-player-card__hint span {
  font-size: 13px;
  line-height: 1.7;
  color: #b8d1e9;
  word-break: break-all;
}
</style>
