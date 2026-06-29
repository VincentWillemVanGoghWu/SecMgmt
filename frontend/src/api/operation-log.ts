import type { ApiResponse } from "../types/auth"
import type {
  OperationDashboardStats,
  OperationLogDetail,
  OperationLogPageData,
} from "../types/operation-log"
import { downloadFile, http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listOperationLogsApi = async (params?: Record<string, unknown>): Promise<OperationLogPageData> =>
  unwrap(await http.get<ApiResponse<OperationLogPageData>>("/operation-logs", { params }))

export const getOperationLogDetailApi = async (id: number): Promise<OperationLogDetail> =>
  unwrap(await http.get<ApiResponse<OperationLogDetail>>(`/operation-logs/${id}`))

export const exportOperationLogsApi = async (params?: Record<string, unknown>): Promise<void> =>
  downloadFile("/export/operation-logs", params)

export const getDashboardOperationStatsApi = async (params?: Record<string, unknown>): Promise<OperationDashboardStats> =>
  unwrap(await http.get<ApiResponse<OperationDashboardStats>>("/dashboard/operation-stats", { params }))
