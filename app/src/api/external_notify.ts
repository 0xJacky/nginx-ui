import type { ModelBase } from '@/api/curd'
import { http, useCurdApi } from '@uozi-admin/request'

export interface ExternalNotify extends ModelBase {
  type: string
  language: string
  config: Record<string, string>
}

export interface TestMessageRequest {
  type: string
  language: string
  config: Record<string, string>
}

const baseUrl = '/external_notifies'

const externalNotify = useCurdApi<ExternalNotify>(baseUrl)

// Add test message API with direct parameters
export function testMessage(params: TestMessageRequest): Promise<{ message: string }> {
  return http.post(`${baseUrl}/test`, params)
}

export default externalNotify
