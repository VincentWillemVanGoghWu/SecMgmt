import type { ApiResponse } from "../types/auth"
import type {
  SmartAiCallbackPayload,
  SmartAiReviewSubmitPayload,
  SmartAiTaskRecord,
  SmartBindingDetailRecord,
  SmartBindingRecord,
  SmartBindingRuleRecord,
  SmartBindingRuleSubmitPayload,
  SmartBindingSubmitPayload,
  SmartBindingTestResult,
  SmartCapabilityRecord,
  SmartEventDetailRecord,
  SmartEventIngestResponse,
  SmartEventPageRecord,
  SmartProviderRecord,
  SmartProviderSubmitPayload,
  SmartProviderTestResult,
  SmartRawEventRecord,
} from "../types/smart-interface"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listSmartProvidersApi = async (params?: Record<string, unknown>): Promise<SmartProviderRecord[]> =>
  unwrap(await http.get<ApiResponse<SmartProviderRecord[]>>("/smart/providers", { params }))

export const createSmartProviderApi = async (payload: SmartProviderSubmitPayload): Promise<SmartProviderRecord> =>
  unwrap(await http.post<ApiResponse<SmartProviderRecord>>("/smart/providers", payload))

export const updateSmartProviderApi = async (
  id: number,
  payload: SmartProviderSubmitPayload,
): Promise<SmartProviderRecord> => unwrap(await http.put<ApiResponse<SmartProviderRecord>>(`/smart/providers/${id}`, payload))

export const testSmartProviderApi = async (id: number): Promise<SmartProviderTestResult> =>
  unwrap(await http.post<ApiResponse<SmartProviderTestResult>>(`/smart/providers/${id}/test`))

export const listSmartCapabilitiesApi = async (params?: Record<string, unknown>): Promise<SmartCapabilityRecord[]> =>
  unwrap(await http.get<ApiResponse<SmartCapabilityRecord[]>>("/smart/capabilities", { params }))

export const listSmartBindingsApi = async (params?: Record<string, unknown>): Promise<SmartBindingRecord[]> =>
  unwrap(await http.get<ApiResponse<SmartBindingRecord[]>>("/smart/bindings", { params }))

export const createSmartBindingApi = async (payload: SmartBindingSubmitPayload): Promise<SmartBindingRecord> =>
  unwrap(await http.post<ApiResponse<SmartBindingRecord>>("/smart/bindings", payload))

export const updateSmartBindingApi = async (
  id: number,
  payload: SmartBindingSubmitPayload,
): Promise<SmartBindingRecord> => unwrap(await http.put<ApiResponse<SmartBindingRecord>>(`/smart/bindings/${id}`, payload))

export const deleteSmartBindingApi = async (id: number): Promise<void> => {
  await http.delete(`/smart/bindings/${id}`)
}

export const getSmartBindingDetailApi = async (id: number): Promise<SmartBindingDetailRecord> =>
  unwrap(await http.get<ApiResponse<SmartBindingDetailRecord>>(`/smart/bindings/${id}`))

export const testSmartBindingApi = async (id: number): Promise<SmartBindingTestResult> =>
  unwrap(await http.post<ApiResponse<SmartBindingTestResult>>(`/smart/bindings/${id}/test`))

export const createSmartBindingRuleApi = async (
  bindingId: number,
  payload: SmartBindingRuleSubmitPayload,
): Promise<SmartBindingRuleRecord> =>
  unwrap(await http.post<ApiResponse<SmartBindingRuleRecord>>(`/smart/bindings/${bindingId}/rules`, payload))

export const updateSmartBindingRuleApi = async (
  bindingId: number,
  ruleId: number,
  payload: SmartBindingRuleSubmitPayload,
): Promise<SmartBindingRuleRecord> =>
  unwrap(await http.put<ApiResponse<SmartBindingRuleRecord>>(`/smart/bindings/${bindingId}/rules/${ruleId}`, payload))

export const deleteSmartBindingRuleApi = async (bindingId: number, ruleId: number): Promise<void> => {
  await http.delete(`/smart/bindings/${bindingId}/rules/${ruleId}`)
}

export const ingestSmartProviderEventApi = async (
  providerCode: string,
  payload: Record<string, unknown> | unknown[] | string,
  headers?: Record<string, string>,
): Promise<SmartEventIngestResponse> =>
  unwrap(await http.post<ApiResponse<SmartEventIngestResponse>>(`/smart/events/ingest/${providerCode}`, payload, { headers }))

export const listSmartRawEventsApi = async (params?: Record<string, unknown>): Promise<SmartRawEventRecord[]> =>
  unwrap(await http.get<ApiResponse<SmartRawEventRecord[]>>("/smart/raw-events", { params }))

export const listSmartEventsApi = async (params?: Record<string, unknown>): Promise<SmartEventPageRecord> =>
  unwrap(await http.get<ApiResponse<SmartEventPageRecord>>("/smart/events", { params }))

export const getSmartEventDetailApi = async (id: number): Promise<SmartEventDetailRecord> =>
  unwrap(await http.get<ApiResponse<SmartEventDetailRecord>>(`/smart/events/${id}`))

export const submitSmartAiReviewApi = async (
  eventId: number,
  payload: SmartAiReviewSubmitPayload,
): Promise<SmartAiTaskRecord> =>
  unwrap(await http.post<ApiResponse<SmartAiTaskRecord>>(`/smart/events/${eventId}/submit-ai-review`, payload))

export const listSmartAiTasksApi = async (params?: Record<string, unknown>): Promise<SmartAiTaskRecord[]> =>
  unwrap(await http.get<ApiResponse<SmartAiTaskRecord[]>>("/smart/ai-tasks", { params }))

export const getSmartAiTaskApi = async (taskId: number): Promise<SmartAiTaskRecord> =>
  unwrap(await http.get<ApiResponse<SmartAiTaskRecord>>(`/smart/ai-tasks/${taskId}`))

export const retrySmartAiTaskApi = async (taskId: number): Promise<SmartAiTaskRecord> =>
  unwrap(await http.post<ApiResponse<SmartAiTaskRecord>>(`/smart/ai-tasks/${taskId}/retry`))

export const handleSmartAiCallbackApi = async (payload: SmartAiCallbackPayload): Promise<SmartAiTaskRecord> =>
  unwrap(await http.post<ApiResponse<SmartAiTaskRecord>>("/smart/ai/callback", payload))
