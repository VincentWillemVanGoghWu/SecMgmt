import type { AlarmRecord } from "../types/alarm"
import type { AlarmRealtimeEvent } from "../types/realtime"

export const mapRealtimeEventToAlarmRecord = (event: AlarmRealtimeEvent): AlarmRecord => ({
  id: event.alarm_id,
  alarmNo: event.alarm_no,
  alarmType: event.alarm_type,
  alarmLevel: event.alarm_level,
  alarmTime: event.alarm_time,
  status: event.status,
  cameraId: event.camera_id,
  cameraName: event.camera_name,
  recorderId: event.recorder_id,
  recorderName: event.recorder_name,
  channelId: event.channel_id,
  channelName: event.channel_name,
  factoryId: event.factory_id,
  factoryName: event.factory_name,
  zoneId: event.zone_id,
  zoneName: event.zone_name,
  message: event.message,
  imageUrl: event.image_url,
  videoUrl: event.video_url,
  recordStartTime: event.record_start_time,
  recordEndTime: event.record_end_time,
  occurrenceCount: event.occurrence_count,
  lastEventTime: event.last_event_time ?? event.alarm_time,
  createdAt: event.created_at,
})

export const prependRealtimeAlarm = (records: AlarmRecord[], event: AlarmRealtimeEvent, limit = 50): AlarmRecord[] => {
  const next = mapRealtimeEventToAlarmRecord(event)
  return [next, ...records.filter((item) => item.id !== next.id)].slice(0, limit)
}
