import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

export interface SiteCategory extends ModelBase {
  name: string
  sync_node_ids: number[]
}

const site_category = new Curd<SiteCategory>('site_categories')

export default site_category
