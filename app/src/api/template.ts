import type { NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'
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

const baseUrl = '/templates'

const template = extendCurdApi(useCurdApi<Template>(baseUrl), {
  get_config_list: () => http.get(`${baseUrl}/configs`),
  get_block_list: () => http.get(`${baseUrl}/blocks`),
  get_config: (name: string) => http.get(`${baseUrl}/config/${name}`),
  get_block: (name: string) => http.get(`${baseUrl}/block/${name}`),
  build_block: (name: string, data: Variable) => http.post(`${baseUrl}/block/${name}`, data),
})

export default template
