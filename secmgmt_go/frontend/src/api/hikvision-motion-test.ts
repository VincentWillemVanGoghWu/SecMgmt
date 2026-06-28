import type { ApiResponse } from "../types/auth"
import type { HikvisionManualSnapshot, HikvisionMotionTestState } from "../types/hikvision-motion-test"
import { http } from "./http"

const unwrap = <T>(response: { data: ApiResponse<T> }) => response.data.data

export const getHikvisionMotionTestStateApi = async (): Promise<HikvisionMotionTestState> =>
  unwrap(await http.get<ApiResponse<HikvisionMotionTestState>>("/ai/hikvision-motion-test"))

export const startHikvisionMotionTestApi = async (
  payload: { sourceType: "camera" | "channel"; sourceId: number; channelNo?: number },
): Promise<HikvisionMotionTestState> =>
  unwrap(
    await http.post<ApiResponse<HikvisionMotionTestState>>("/ai/hikvision-motion-test/start", {
      sourceType: payload.sourceType,
      sourceId: payload.sourceId,
      channelNo: payload.channelNo ?? 1,
    }),
  )

export const stopHikvisionMotionTestApi = async (): Promise<HikvisionMotionTestState> =>
  unwrap(await http.post<ApiResponse<HikvisionMotionTestState>>("/ai/hikvision-motion-test/stop"))

export const createHikvisionManualSnapshotApi = async (
  payload: { sourceType: "camera" | "channel"; sourceId: number; channelNo?: number },
): Promise<HikvisionManualSnapshot> =>
  unwrap(
    await http.post<ApiResponse<HikvisionManualSnapshot>>("/ai/hikvision-motion-test/snapshot", {
      sourceType: payload.sourceType,
      sourceId: payload.sourceId,
      channelNo: payload.channelNo ?? 1,
    }),
  )
