import type { ApiResponse } from "../types/auth"
import type {
  CameraBrowserLoginPayload,
  CameraConnectionTestData,
  CameraDeviceIdentity,
  CameraDeviceIdentityPayload,
  CameraImageConfig,
  CameraPtzLensActionResult,
  CameraPtzConfig,
  CameraPtzPresetActionResult,
  CameraRecord,
  CameraRecordingConfig,
  CameraSdkConfig,
  CameraStatusCheckData,
  CameraSubmitPayload,
  CameraNetworkConfig,
  CameraUserAccountUpsert,
  CameraUserConfig,
} from "../types/camera"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const listCamerasApi = async (params?: Record<string, unknown>): Promise<CameraRecord[]> =>
  unwrap(await http.get<ApiResponse<CameraRecord[]>>("/cameras", { params }))

export const getCameraApi = async (id: number): Promise<CameraRecord> =>
  unwrap(await http.get<ApiResponse<CameraRecord>>(`/cameras/${id}`))

export const getCameraBrowserLoginApi = async (id: number): Promise<CameraBrowserLoginPayload> =>
  unwrap(await http.get<ApiResponse<CameraBrowserLoginPayload>>(`/cameras/${id}/browser-login`))

export const createCameraApi = async (payload: CameraSubmitPayload & { password: string }): Promise<CameraRecord> =>
  unwrap(await http.post<ApiResponse<CameraRecord>>("/cameras", payload))

export const fetchCameraDeviceIdentityApi = async (payload: CameraDeviceIdentityPayload): Promise<CameraDeviceIdentity> =>
  unwrap(await http.post<ApiResponse<CameraDeviceIdentity>>("/cameras/sdk-device-identity", payload))

export const updateCameraApi = async (
  id: number,
  payload: CameraSubmitPayload & { password?: string },
): Promise<CameraRecord> => unwrap(await http.put<ApiResponse<CameraRecord>>(`/cameras/${id}`, payload))

export const updateCameraStatusApi = async (id: number, status: string): Promise<CameraRecord> =>
  unwrap(await http.patch<ApiResponse<CameraRecord>>(`/cameras/${id}/status`, { status }))

export const deleteCameraApi = async (id: number): Promise<void> => {
  await http.delete(`/cameras/${id}`)
}

export const testCameraConnectionApi = async (id: number): Promise<CameraConnectionTestData> =>
  unwrap(await http.post<ApiResponse<CameraConnectionTestData>>(`/cameras/${id}/test`))

export const checkCameraStatusApi = async (id: number): Promise<CameraStatusCheckData> =>
  unwrap(await http.post<ApiResponse<CameraStatusCheckData>>(`/cameras/${id}/status/check`))

export const getCameraSdkConfigApi = async (id: number): Promise<CameraSdkConfig> =>
  unwrap(await http.get<ApiResponse<CameraSdkConfig>>(`/cameras/${id}/sdk-config`))

export const updateCameraNetworkConfigApi = async (
  id: number,
  payload: Omit<CameraNetworkConfig, "supported" | "message">,
): Promise<CameraNetworkConfig> =>
  unwrap(await http.put<ApiResponse<CameraNetworkConfig>>(`/cameras/${id}/sdk-config/network`, payload))

export const updateCameraImageConfigApi = async (
  id: number,
  payload: Omit<CameraImageConfig, "supported" | "message">,
): Promise<CameraImageConfig> =>
  unwrap(await http.put<ApiResponse<CameraImageConfig>>(`/cameras/${id}/sdk-config/image`, payload))

export const updateCameraRecordingConfigApi = async (
  id: number,
  payload: Pick<CameraRecordingConfig, "storageMode" | "overwriteEnabled" | "weeklyPlan">,
): Promise<CameraRecordingConfig> =>
  unwrap(await http.put<ApiResponse<CameraRecordingConfig>>(`/cameras/${id}/sdk-config/recording`, payload))

export const setCameraPtzPresetApi = async (
  id: number,
  payload: { presetId: number; name: string },
): Promise<CameraPtzConfig> =>
  unwrap(await http.put<ApiResponse<CameraPtzConfig>>(`/cameras/${id}/sdk-config/ptz/presets`, payload))

export const deleteCameraPtzPresetApi = async (id: number, presetId: number): Promise<CameraPtzConfig> =>
  unwrap(await http.delete<ApiResponse<CameraPtzConfig>>(`/cameras/${id}/sdk-config/ptz/presets/${presetId}`))

export const gotoCameraPtzPresetApi = async (id: number, presetId: number): Promise<CameraPtzPresetActionResult> =>
  unwrap(await http.put<ApiResponse<CameraPtzPresetActionResult>>(`/cameras/${id}/sdk-config/ptz/presets/${presetId}/goto`))

export const updateCameraPtzConfigApi = async (
  id: number,
  payload: Pick<CameraPtzConfig, "cruiseEnabled" | "trackEnabled">,
): Promise<CameraPtzConfig> => unwrap(await http.put<ApiResponse<CameraPtzConfig>>(`/cameras/${id}/sdk-config/ptz`, payload))

export const controlCameraPtzZoomApi = async (
  id: number,
  action: "in" | "out",
): Promise<CameraPtzLensActionResult> =>
  unwrap(await http.put<ApiResponse<CameraPtzLensActionResult>>(`/cameras/${id}/sdk-config/ptz/zoom/${action}`))

export const upsertCameraUserApi = async (id: number, payload: CameraUserAccountUpsert): Promise<CameraUserConfig> =>
  unwrap(await http.put<ApiResponse<CameraUserConfig>>(`/cameras/${id}/sdk-config/users`, payload))

export const deleteCameraUserApi = async (id: number, userId: number): Promise<CameraUserConfig> =>
  unwrap(await http.delete<ApiResponse<CameraUserConfig>>(`/cameras/${id}/sdk-config/users/${userId}`))
