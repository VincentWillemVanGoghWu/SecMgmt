import { reactive } from "vue"

export const playbackDownloadState = reactive({
  active: false,
  preparing: false,
  progress: 0,
  message: "",
  estimatedRemainingText: "",
  startedAtMs: 0,
  transferStartedAtMs: 0,
  expectedPrepareSeconds: 0,
  segmentKey: "",
  controller: null as AbortController | null,
})
