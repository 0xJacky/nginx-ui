import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

export interface EnvGroup extends ModelBase {
  name: string
  sync_node_ids: number[]
}

const env_group = new Curd<EnvGroup>('env_groups')

export default env_group
