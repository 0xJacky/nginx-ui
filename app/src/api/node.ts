import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Node extends ModelBase {
  name: string
  url: string
  token: string
  status?: boolean
  response_at?: Date
}

export interface NodeInfo {
  id: number
  name: string
  token: string
  response_at?: Date
}

const baseUrl = '/nodes'

function reloadNginx(nodeIds: number[]) {
  return http.post('/nodes/reload_nginx', { node_ids: nodeIds })
}

function restartNginx(nodeIds: number[]) {
  return http.post('/nodes/restart_nginx', { node_ids: nodeIds })
}

const nodeApi = extendCurdApi(useCurdApi<Node>(baseUrl), {
  load_from_settings: () => http.post(`${baseUrl}/load_from_settings`),
  reloadNginx,
  restartNginx,
})

export default nodeApi
