import type { ApiResponse } from "../types/auth"
import type { AxiosProgressEvent } from "axios"
import type {
  LiveVideoPayload,
  LiveWebControlConfig,
  PlaybackSegmentRecord,
  PlaybackUrlPayload,
  SnapshotPayload,
  StopPlaybackPayload,
  StopLiveVideoPayload,
  StreamProfile,
  StreamType,
} from "../types/video"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const getLiveVideoApi = async (
  cameraId: number,
  params?: { streamType?: StreamType; streamProfile?: StreamProfile },
): Promise<LiveVideoPayload> =>
  unwrap(
    await http.get<ApiResponse<LiveVideoPayload>>(`/video/live/${cameraId}`, {
      params: {
        stream_type: params?.streamType ?? "hik-sdk",
        stream_profile: params?.streamProfile ?? "main",
      },
      timeout: params?.streamType === "hls" ? 45000 : undefined,
    }),
  )

export const getChannelLiveVideoApi = async (
  channelId: number,
  params?: { streamType?: StreamType; streamProfile?: StreamProfile },
): Promise<LiveVideoPayload> =>
  unwrap(
    await http.get<ApiResponse<LiveVideoPayload>>(`/video/live/channel/${channelId}`, {
      params: {
        stream_type: params?.streamType ?? "hik-sdk",
        stream_profile: params?.streamProfile ?? "main",
      },
      timeout: params?.streamType === "hls" ? 45000 : undefined,
    }),
  )

export const stopLiveVideoApi = async (cameraId: number): Promise<StopLiveVideoPayload> =>
  unwrap(await http.post<ApiResponse<StopLiveVideoPayload>>(`/video/live/${cameraId}/stop`))

export const stopChannelLiveVideoApi = async (channelId: number): Promise<StopLiveVideoPayload> =>
  unwrap(await http.post<ApiResponse<StopLiveVideoPayload>>(`/video/live/channel/${channelId}/stop`))

export const getLiveWebControlConfigApi = async (
  cameraId: number,
  params?: { streamProfile?: StreamProfile },
): Promise<LiveWebControlConfig> =>
  unwrap(
    await http.get<ApiResponse<LiveWebControlConfig>>(`/video/live/${cameraId}/webcontrol-config`, {
      params: {
        stream_profile: params?.streamProfile ?? "main",
      },
    }),
  )

export const getChannelLiveWebControlConfigApi = async (
  channelId: number,
  params?: { streamProfile?: StreamProfile },
): Promise<LiveWebControlConfig> =>
  unwrap(
    await http.get<ApiResponse<LiveWebControlConfig>>(`/video/live/channel/${channelId}/webcontrol-config`, {
      params: {
        stream_profile: params?.streamProfile ?? "main",
      },
    }),
  )

export const createSnapshotApi = async (
  payload: {
    cameraId?: number
    channelId?: number
    channelNo?: number
    streamProfile?: StreamProfile
    preferDeviceSnapshot?: boolean
  },
): Promise<SnapshotPayload> =>
  unwrap(
    await http.post<ApiResponse<SnapshotPayload>>("/video/snapshot", {
      cameraId: payload.cameraId,
      channelId: payload.channelId,
      channelNo: payload?.channelNo ?? 1,
      streamProfile: payload?.streamProfile ?? "main",
      preferDeviceSnapshot: payload?.preferDeviceSnapshot ?? false,
    }),
  )

export const searchPlaybackSegmentsApi = async (params: Record<string, unknown>): Promise<PlaybackSegmentRecord[]> =>
  unwrap(await http.get<ApiResponse<PlaybackSegmentRecord[]>>("/video/playback/search", { params }))

export const getPlaybackUrlApi = async (params: Record<string, unknown>): Promise<PlaybackUrlPayload> =>
  unwrap(await http.get<ApiResponse<PlaybackUrlPayload>>("/video/playback/url", { params, timeout: 45000 }))

export const seekPlaybackApi = async (params: Record<string, unknown>): Promise<PlaybackUrlPayload> =>
  unwrap(await http.post<ApiResponse<PlaybackUrlPayload>>("/video/playback/seek", null, { params, timeout: 45000 }))

const resolveDownloadFilename = (headers: Record<string, unknown>, fallback: string) => {
  const disposition = headers["content-disposition"] as string | undefined
  const encodedMatch = disposition?.match(/filename\*=UTF-8''([^;]+)/i)
  const plainMatch = disposition?.match(/filename="?([^"]+)"?/i)
  return encodedMatch?.[1]
    ? decodeURIComponent(encodedMatch[1])
    : (plainMatch?.[1] ?? fallback)
}

const triggerBrowserDownload = (blob: Blob, filename: string) => {
  const link = document.createElement("a")
  const objectUrl = window.URL.createObjectURL(blob)
  link.href = objectUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(objectUrl)
}

export const downloadPlaybackFileApi = async (
  params: Record<string, unknown>,
  options?: {
    signal?: AbortSignal
    onProgress?: (event: AxiosProgressEvent) => void
    filename?: string
  },
): Promise<void> => {
  const response = await http.get<Blob>("/video/playback/download", {
    params,
    responseType: "blob",
    signal: options?.signal,
    timeout: 0,
    onDownloadProgress: options?.onProgress,
  })
  const filename = options?.filename
    || resolveDownloadFilename(
      response.headers as Record<string, unknown>,
      `playback_${Date.now()}.mp4`,
    )
  const blob = response.data instanceof Blob ? response.data : new Blob([response.data])
  triggerBrowserDownload(blob, filename)
}

export const stopPlaybackApi = async (channelId: number): Promise<StopPlaybackPayload> =>
  unwrap(await http.post<ApiResponse<StopPlaybackPayload>>("/video/playback/stop", null, { params: { channel_id: channelId } }))
