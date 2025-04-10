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

export interface NginxPerformanceInfo {
  active: number // 活动连接数
  accepts: number // 总握手次数
  handled: number // 总连接次数
  requests: number // 总请求数
  reading: number // 读取客户端请求数
  writing: number // 响应数
  waiting: number // 驻留进程（等待请求）
  workers: number // 工作进程数
  master: number // 主进程数
  cache: number // 缓存管理进程数
  other: number // 其他Nginx相关进程数
  cpu_usage: number // CPU 使用率
  memory_usage: number // 内存使用率（MB）
  worker_processes: number // worker_processes 配置
  worker_connections: number // worker_connections 配置
}

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

  detail_status(): Promise<{ running: boolean, stub_status_enabled: boolean, info: NginxPerformanceInfo }> {
    return http.get('/nginx/detail_status')
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
