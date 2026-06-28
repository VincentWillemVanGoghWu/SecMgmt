<script setup lang="ts">
import { ElNotification } from "element-plus"
import { computed, watch } from "vue"
import { useRouter } from "vue-router"

import { useRealtimeStore } from "../../stores"

const router = useRouter()
const realtimeStore = useRealtimeStore()
const criticalLevels = new Set(["critical", "high"])
const latestAlarm = computed(() => realtimeStore.lastAlarmEvent)

watch(
  () => realtimeStore.eventSequence,
  () => {
    const event = latestAlarm.value
    if (!event || !criticalLevels.has(event.alarm_level)) return

    ElNotification({
      title: event.alarm_level === "critical" ? "紧急告警" : "重要告警",
      type: "error",
      duration: 8000,
      position: "top-right",
      message: `${event.factory_name || "未绑定厂区"} / ${event.zone_name || "未绑定区域"} / ${event.camera_name || "未知设备"} 出现 ${event.alarm_type}`,
      onClick: () => {
        void router.push({ name: "safety-realtime-alarms" })
      },
    })
  },
)
</script>

<template>
  <span style="display: none" />
</template>
