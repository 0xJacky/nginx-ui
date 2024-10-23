import type { NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'
import Curd from '@/api/curd'
import http from '@/lib/http'

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

class TemplateApi extends Curd<Template> {
  get_config_list() {
    return http.get('template/configs')
  }

  get_block_list() {
    return http.get('template/blocks')
  }

  get_config(name: string) {
    return http.get(`template/config/${name}`)
  }

  get_block(name: string) {
    return http.get(`template/block/${name}`)
  }

  build_block(name: string, data: Variable) {
    return http.post(`template/block/${name}`, data)
  }
}

const template = new TemplateApi('/template')

export default template
