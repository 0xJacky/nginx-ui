import type { ModelBase, UpdateOrderRequest } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

// Post-sync action types
export const PostSyncAction = {
  None: 'none',
  ReloadNginx: 'reload_nginx',
}

// Upstream test types
export const UpstreamTestType = {
  Local: 'local',
  Remote: 'remote',
  Mirror: 'mirror',
}

// Deploy mode types
export const DeployMode = {
  Local: 'local',
  Remote: 'remote',
} as const

export interface Namespace extends ModelBase {
  name: string
  sync_node_ids: number[]
  post_sync_action?: string
  upstream_test_type?: string
  deploy_mode?: string
}

const baseUrl = '/namespaces'

const namespace = extendCurdApi(useCurdApi<Namespace>(baseUrl), {
  updateOrder(data: UpdateOrderRequest) {
    return http.post('/namespaces/order', data)
  },
})

export default namespace
