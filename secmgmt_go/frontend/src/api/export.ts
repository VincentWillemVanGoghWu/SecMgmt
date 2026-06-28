import { downloadFile } from "./http"

export const exportAlarmsApi = async (params?: Record<string, unknown>): Promise<void> =>
  downloadFile("/export/alarms", params)

export const exportDeviceStatusApi = async (params?: Record<string, unknown>): Promise<void> =>
  downloadFile("/export/device-status", params)

export const exportPushLogsApi = async (params?: Record<string, unknown>): Promise<void> =>
  downloadFile("/export/push-logs", params)

