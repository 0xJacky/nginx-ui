import http from '@/lib/http'

export interface NgxConfig {
  file_name?: string
  name: string
  upstreams?: NgxUpstream[]
  servers: NgxServer[]
  custom?: string
}

export interface NgxServer {
  directives?: NgxDirective[]
  locations?: NgxLocation[]
  comments?: string
}

export interface NgxUpstream {
  name: string
  directives: NgxDirective[]
  comments?: string
}

export interface NgxDirective {
  idx?: number
  directive: string
  params: string
  comments?: string
}

export interface NgxLocation {
  path: string
  content: string
  comments: string
}

export type DirectiveMap = Record<string, { links: string[] }>

const ngx = {
  build_config(ngxConfig: NgxConfig) {
    return http.post('/ngx/build_config', ngxConfig)
  },

  tokenize_config(content: string) {
    return http.post('/ngx/tokenize_config', { content })
  },

  format_code(content: string) {
    return http.post('/ngx/format_code', { content })
  },

  status(): Promise<{ running: boolean, message: string, level: number }> {
    return http.get('/nginx/status')
  },

  reload() {
    return http.post('/nginx/reload')
  },

  restart() {
    return http.post('/nginx/restart')
  },

  test() {
    return http.post('/nginx/test')
  },

  get_directives(): Promise<DirectiveMap> {
    return http.get('/nginx/directives')
  },
}

export default ngx
