<script setup lang="ts">
import { defineAsyncComponent } from "vue"

const props = defineProps<{
  title?: string
  playUrl?: string | null
  message?: string
  status?: string
}>()

const VideoPlayerRenderer = defineAsyncComponent(() => import("./VideoPlayerRenderer.vue"))
</script>

<template>
  <Suspense>
    <component
      :is="VideoPlayerRenderer"
      :title="props.title"
      :play-url="props.playUrl"
      :message="props.message"
      :status="props.status"
    />
    <template #fallback>
      <div class="async-video-placeholder">
        <span>播放器组件异步加载中...</span>
      </div>
    </template>
  </Suspense>
</template>

<style scoped>
.async-video-placeholder {
  min-height: 280px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  border: 1px dashed rgba(97, 127, 158, 0.28);
  background: linear-gradient(180deg, #102640 0%, #143558 100%);
  color: #d9ebff;
  font-size: 13px;
}
</style>
