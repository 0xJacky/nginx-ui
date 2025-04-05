import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

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

export default new Curd<EnvGroup>('/env_groups')
