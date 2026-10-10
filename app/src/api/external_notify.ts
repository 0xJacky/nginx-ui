import type { ModelBase } from '@/api/curd'
import type { ConfigurationField } from '@/api/plugin'
import { http, useCurdApi } from '@uozi-admin/request'

export interface ExternalNotify extends ModelBase {
  type: string
  description: string
  language: string
  config: Record<string, string>
  enabled: boolean
}

export interface TestMessageRequest {
  type: string
  language: string
  config: Record<string, string>
}

export interface CreateExternalNotifyRequest {
  type: string
  description?: string
  language: string
  config: Record<string, string>
  enabled: boolean
}

/** A notifier type a plugin offers next to the built-in ones. */
export interface ExternalNotifierChannel {
  /** Value stored in ExternalNotify.type, e.g. "plugin:mychat". */
  type: string
  name: string
  plugin_id?: string
  fields: ConfigurationField[]
}

const baseUrl = '/external_notifies'

const externalNotify = useCurdApi<ExternalNotify>(baseUrl)

// Add test message API with direct parameters
export function testMessage(params: TestMessageRequest): Promise<{ message: string }> {
  return http.post(`${baseUrl}/test`, params)
}

export function listExternalNotifies(): Promise<{ data: ExternalNotify[] }> {
  return http.get(baseUrl, { params: { page: 1, per_page: 1000 } })
}

export function createExternalNotify(params: CreateExternalNotifyRequest): Promise<ExternalNotify> {
  return http.post(baseUrl, params)
}

/** Lists the notifier types enabled plugins provide, with their form schema. */
export function listExternalNotifierChannels(): Promise<{ data: ExternalNotifierChannel[] }> {
  return http.get(`${baseUrl}/channels`)
}

export default externalNotify
