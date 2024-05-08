import http from '@/lib/http'
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

class EnvironmentCurd extends Curd<Environment> {
  constructor() {
    super('/environment')
  }

  load_from_settings() {
    return http.post(`${this.plural}/load_from_settings`)
  }
}

const environment: EnvironmentCurd = new EnvironmentCurd()

export default environment
