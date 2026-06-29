import type { ApiResponse } from "../types/auth"
import type {
  AlarmReportData,
  CategoryChart,
  DashboardDeviceStatus,
  OperationDashboardStats,
  DashboardSummary,
  DeviceReportData,
  NameValueChart,
  PushReportData,
  ZoneRankingChart,
} from "../types/dashboard"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const getDashboardSummaryApi = async (params?: Record<string, unknown>): Promise<DashboardSummary> =>
  unwrap(await http.get<ApiResponse<DashboardSummary>>("/dashboard/summary", { params }))

export const getDashboardAlarmTrendApi = async (params?: Record<string, unknown>): Promise<CategoryChart> =>
  unwrap(await http.get<ApiResponse<CategoryChart>>("/dashboard/alarm-trend", { params }))

export const getDashboardAlarmTypesApi = async (params?: Record<string, unknown>): Promise<NameValueChart> =>
  unwrap(await http.get<ApiResponse<NameValueChart>>("/dashboard/alarm-types", { params }))

export const getDashboardOperationStatsApi = async (params?: Record<string, unknown>): Promise<OperationDashboardStats> =>
  unwrap(await http.get<ApiResponse<OperationDashboardStats>>("/dashboard/operation-stats", { params }))

export const getDashboardZoneRankingApi = async (params?: Record<string, unknown>): Promise<ZoneRankingChart> =>
  unwrap(await http.get<ApiResponse<ZoneRankingChart>>("/dashboard/zone-ranking", { params }))

export const getDashboardDeviceStatusApi = async (): Promise<DashboardDeviceStatus> =>
  unwrap(await http.get<ApiResponse<DashboardDeviceStatus>>("/dashboard/device-status"))

export const getAlarmReportApi = async (params?: Record<string, unknown>): Promise<AlarmReportData> =>
  unwrap(await http.get<ApiResponse<AlarmReportData>>("/reports/alarms", { params }))

export const getDeviceReportApi = async (params?: Record<string, unknown>): Promise<DeviceReportData> =>
  unwrap(await http.get<ApiResponse<DeviceReportData>>("/reports/devices", { params }))

export const getPushReportApi = async (params?: Record<string, unknown>): Promise<PushReportData> =>
  unwrap(await http.get<ApiResponse<PushReportData>>("/reports/push", { params }))

