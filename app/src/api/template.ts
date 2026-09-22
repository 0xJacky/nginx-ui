import type { NgxConfig, NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface Variable {
  type?: string
  name?: Record<string, string>
  // eslint-disable-next-line ts/no-explicit-any
  value?: any
  mask?: Record<string, Record<string, string>>
}

/**
 * Where a template comes from: built into Nginx UI, a snippet of the user or
 * an enabled plugin.
 */
export type TemplateOrigin = 'builtin' | 'custom' | 'plugin'

export interface Template extends NgxServer {
  name: string
  /** The name per language, when the template has one. */
  name_i18n?: Record<string, string>
  description: Record<string, string>
  author: string
  filename: string
  origin?: TemplateOrigin
  variables: Record<string, Variable>
  custom: string
  locations?: NgxLocation[]
  directives?: NgxDirective[]
  /** The plugin a template of origin "plugin" comes from. */
  plugin_id?: string
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
  // Name of a managed upstream group; replaces host and port when set.
  upstream?: string
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

/** What identifies where a block template comes from. */
export type TemplateSource = Pick<Template, 'origin' | 'plugin_id'>

const baseUrl = '/templates'

// A snippet is asked for by its origin, a plugin template by its plugin.
function blockParams(source?: TemplateSource) {
  if (source?.origin === 'custom')
    return { origin: source.origin }
  if (source?.plugin_id)
    return { plugin_id: source.plugin_id }
  return undefined
}

const template = extendCurdApi(useCurdApi<Template>(baseUrl), {
  get_config_list: () => http.get(`${baseUrl}/configs`),
  get_block_list: () => http.get(`${baseUrl}/blocks`),
  get_config: (name: string) => http.get(`${baseUrl}/config/${name}`),
  get_block: (name: string, source?: TemplateSource) => http.get(`${baseUrl}/block/${name}`, { params: blockParams(source) }),
  build_block: (name: string, data: Variable, source?: TemplateSource) => http.post(`${baseUrl}/block/${name}`, data, { params: blockParams(source) }),
  get_quick_config: (data: QuickConfigRequest): Promise<QuickConfigResponse> => http.post(`${baseUrl}/quick_config`, data),
  analyze_quick_config: (config: string): Promise<{ request: QuickConfigRequest }> => http.post(`${baseUrl}/quick_config/analyze`, { config }),
})

export default template
