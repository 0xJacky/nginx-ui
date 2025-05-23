import type { EnvGroup } from './env_group'
import type { NgxConfig } from '@/api/ngx'
import type { ChatComplicationMessage } from '@/api/openai'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

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

const baseUrl = '/streams'

const stream = extendCurdApi(useCurdApi<Stream>(baseUrl), {
  enable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/enable`),
  disable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/disable`),
  duplicate: (name: string, data: { name: string }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/duplicate`, data),
  advance_mode: (name: string, data: { advanced: boolean }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/advance`, data),
  rename: (name: string, newName: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/rename`, { new_name: newName }),
})

export default stream
