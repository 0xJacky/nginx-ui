import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Notification extends ModelBase {
  type: string
  title: string
  details: string
}

const baseUrl = '/notifications'

const notification = extendCurdApi(useCurdApi<Notification>(baseUrl), {
  clear: () => http.delete(baseUrl),
})

export default notification
