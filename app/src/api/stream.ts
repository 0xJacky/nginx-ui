import type { Namespace } from './namespace'
import type { ChatComplicationMessage } from '@/api/llm'
import type { NgxConfig } from '@/api/ngx'
import type { ProxyTarget, SiteStatus } from '@/api/site'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Stream {
  modified_at: string
  advanced: boolean
  status: SiteStatus
  name: string
  filepath: string
  config: string
  llm_messages: ChatComplicationMessage[]
  tokenized?: NgxConfig
  namespace_id: number
  namespace?: Namespace
  sync_node_ids: number[]
  proxy_targets?: ProxyTarget[]
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
