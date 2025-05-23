import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Environment extends ModelBase {
  name: string
  url: string
  token: string
  status?: boolean
}

export interface Node {
  id: number
  name: string
  token: string
  response_at?: Date
}

const baseUrl = '/environments'

const environment = extendCurdApi(useCurdApi<Environment>(baseUrl), {
  load_from_settings: () => http.post(`${baseUrl}/load_from_settings`),
})

export default environment
