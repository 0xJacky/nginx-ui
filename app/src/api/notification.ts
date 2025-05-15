import type { ModelBase } from '@/api/curd'
import { http, useCurdApi } from '@uozi-admin/request'

export interface Notification extends ModelBase {
  type: string
  title: string
  details: string
}

const baseUrl = '/notifications'

const notification = useCurdApi<Notification>(baseUrl, {
  clear: () => http.delete(baseUrl),
})

export default notification
