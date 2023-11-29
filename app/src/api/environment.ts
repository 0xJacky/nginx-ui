import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

export interface Environment extends ModelBase {
  name: string
  url: string
  token: string
}

const environment: Curd<Environment> = new Curd('/environment')

export default environment
