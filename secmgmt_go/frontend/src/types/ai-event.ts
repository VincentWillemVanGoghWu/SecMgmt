export interface AiIntegrationConfig {
  callbackUrl: string
  signatureSecret: string
  signingEnabled: boolean
  eventSources: string[]
  minConfidence: number
  ignoreBelowThreshold: boolean
  dedupWindowSeconds: number
  eventTypeMappings: Record<string, string>
}

export interface AiEventRecord {
  id: number
  eventNo: string
  sourceType: string
  eventType: string
  eventLevel: string
  eventTime: string
  cameraId?: number | null
  cameraName?: string | null
  recorderId?: number | null
  recorderName?: string | null
  channelId?: number | null
  channelName?: string | null
  factoryId?: number | null
  factoryName?: string | null
  zoneId?: number | null
  zoneName?: string | null
  imageUrl?: string | null
  videoUrl?: string | null
  confidence?: number | null
  rawJson: string
  dedupKey: string
  createdAt: string
}

export interface AiCallbackResult {
  accepted: boolean
  stored: boolean
  ignored: boolean
  reason: string
  eventId?: number | null
  eventNo?: string | null
}

export interface AiCallbackPayload {
  deviceCode: string
  eventType: string
  eventTime: string
  confidence?: number | null
  imageUrl?: string | null
  raw?: Record<string, unknown> | unknown[] | string | null
  sourceType?: string
  videoUrl?: string | null
}
