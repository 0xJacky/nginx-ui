import type { EnvGroup } from './env_group'
import type { NgxConfig } from '@/api/ngx'
import type { ChatComplicationMessage } from '@/api/openai'
import Curd from '@/api/curd'
import http from '@/lib/http'

export interface Stream {
  modified_at: string
  advanced: boolean
  enabled: boolean
  name: string
  filepath: string
  config: string
  chatgpt_messages: ChatComplicationMessage[]
  tokenized?: NgxConfig
  env_group_id: number
  env_group?: EnvGroup
  sync_node_ids: number[]
}

class StreamCurd extends Curd<Stream> {
  // eslint-disable-next-line ts/no-explicit-any
  enable(name: string, config?: any) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/enable`, undefined, config)
  }

  disable(name: string) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/disable`)
  }

  duplicate(name: string, data: { name: string }): Promise<{ dst: string }> {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/duplicate`, data)
  }

  advance_mode(name: string, data: { advanced: boolean }) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/advance`, data)
  }

  rename(name: string, newName: string) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/rename`, { new_name: newName })
  }
}

const stream = new StreamCurd('/streams')

export default stream
