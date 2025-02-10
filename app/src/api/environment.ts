import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'
import http from '@/lib/http'

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
    super('/environments')
  }

  load_from_settings() {
    return http.post(`${this.baseUrl}/load_from_settings`)
  }
}

const environment: EnvironmentCurd = new EnvironmentCurd()

export default environment
