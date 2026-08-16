import type { NgxConfig, NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Variable {
  type?: string
  name?: Record<string, string>
  // eslint-disable-next-line ts/no-explicit-any
  value?: any
  mask?: Record<string, Record<string, string>>
}

export interface Template extends NgxServer {
  name: string
  description: Record<string, string>
  author: string
  filename: string
  variables: Record<string, Variable>
  custom: string
  locations?: NgxLocation[]
  directives?: NgxDirective[]
}

export type QuickConfigType = 'reverse_proxy' | 'static' | 'redirect'

export interface QuickConfigRequest {
  type: QuickConfigType
  domains: string[]
  enable_tls?: boolean
  redirect_http_to_https?: boolean
  // reverse_proxy
  scheme?: 'http' | 'https'
  host?: string
  port?: string
  enable_websocket?: boolean
  client_max_body_size?: string
  // static
  web_root?: string
  index?: string
  spa_fallback?: boolean
  // redirect
  target_url?: string
  redirect_status?: '301' | '302' | '308'
}

export interface QuickConfigResponse {
  template: string
  tokenized: NgxConfig
}

const baseUrl = '/templates'

const template = extendCurdApi(useCurdApi<Template>(baseUrl), {
  get_config_list: () => http.get(`${baseUrl}/configs`),
  get_block_list: () => http.get(`${baseUrl}/blocks`),
  get_config: (name: string) => http.get(`${baseUrl}/config/${name}`),
  get_block: (name: string) => http.get(`${baseUrl}/block/${name}`),
  build_block: (name: string, data: Variable) => http.post(`${baseUrl}/block/${name}`, data),
  get_quick_config: (data: QuickConfigRequest): Promise<QuickConfigResponse> => http.post(`${baseUrl}/quick_config`, data),
  analyze_quick_config: (config: string): Promise<{ request: QuickConfigRequest }> => http.post(`${baseUrl}/quick_config/analyze`, { config }),
})

export default template
