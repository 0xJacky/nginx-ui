import type { ModelBase, UpdateOrderRequest } from '@/api/curd'
import { http, useCurdApi } from '@uozi-admin/request'
// Post-sync action types
export const PostSyncAction = {
  None: 'none',
  ReloadNginx: 'reload_nginx',
}

export interface EnvGroup extends ModelBase {
  name: string
  sync_node_ids: number[]
  post_sync_action?: string
}

const baseUrl = '/env_groups'

const env_group = useCurdApi<EnvGroup>(baseUrl, {
  updateOrder(data: UpdateOrderRequest) {
    return http.post('/env_groups/order', data)
  },
})

export default env_group
