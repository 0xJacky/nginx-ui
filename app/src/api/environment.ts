import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

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
const environment: Curd<Environment> = new Curd('/environment')

export default environment
