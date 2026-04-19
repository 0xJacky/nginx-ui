import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface NotificationDetails {
  response?: string | Record<string, unknown>
  [key: string]: unknown
}

export interface Notification extends ModelBase {
  type: string
  title: string
  content: string
  details: string | NotificationDetails | null
}

const baseUrl = '/notifications'

const notification = extendCurdApi(useCurdApi<Notification>(baseUrl), {
  clear: () => http.delete(baseUrl),
})

export default notification
