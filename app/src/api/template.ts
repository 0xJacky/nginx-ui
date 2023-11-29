import Curd from '@/api/curd'
import http from '@/lib/http'
import type { NgxServer } from '@/api/ngx'

export interface Variable {
  type?: string
  name?: { [key: string]: string }
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  value?: any
}

export interface Template extends NgxServer {
  name: string
  description: { [key: string]: string }
  author: string
  filename: string
  variables: {
    [key: string]: Variable
  }
  custom: string
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
