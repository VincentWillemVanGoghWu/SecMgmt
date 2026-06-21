import type { ApiResponse } from "../types/auth"
import type {
  AiCallbackPayload,
  AiCallbackResult,
  AiEventRecord,
  AiIntegrationConfig,
} from "../types/ai-event"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const getAiIntegrationConfigApi = async (): Promise<AiIntegrationConfig> =>
  unwrap(await http.get<ApiResponse<AiIntegrationConfig>>("/ai/config"))

export const createAiEventCallbackApi = async (payload: AiCallbackPayload): Promise<AiCallbackResult> =>
  unwrap(await http.post<ApiResponse<AiCallbackResult>>("/ai/events/callback", payload))

export const listAiEventsApi = async (params?: Record<string, unknown>): Promise<AiEventRecord[]> =>
  unwrap(await http.get<ApiResponse<AiEventRecord[]>>("/ai/events", { params }))

export const getAiEventDetailApi = async (id: number): Promise<AiEventRecord> =>
  unwrap(await http.get<ApiResponse<AiEventRecord>>(`/ai/events/${id}`))
