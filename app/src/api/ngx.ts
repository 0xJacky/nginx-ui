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

export interface ProxyCacheConfig {
  enabled: boolean
  path: string
  levels: string
  use_temp_path: string
  keys_zone: string
  inactive: string
  max_size: string
  min_free: string
  manager_files: string
  manager_sleep: string
  manager_threshold: string
  loader_files: string
  loader_sleep: string
  loader_threshold: string
  purger: string
  purger_files: string
  purger_sleep: string
  purger_threshold: string
}

export interface NginxPerformanceInfo {
  active: number // Number of active connections
  accepts: number // Total number of accepted connections
  handled: number // Total number of handled connections
  requests: number // Total number of requests
  reading: number // Number of connections reading request data
  writing: number // Number of connections writing response data
  waiting: number // Number of idle connections waiting for requests
  workers: number // Number of worker processes
  master: number // Number of master processes
  cache: number // Number of cache manager processes
  other: number // Number of other Nginx-related processes
  cpu_usage: number // CPU usage percentage
  memory_usage: number // Memory usage in MB
  worker_processes: number // worker_processes configuration
  worker_connections: number // worker_connections configuration
  process_mode: string // Worker process configuration mode: 'auto' or 'manual'
}

export interface NginxConfigInfo {
  worker_processes: string
  worker_connections: number
  process_mode: string
  keepalive_timeout: string
  gzip: string
  gzip_min_length: number
  gzip_comp_level: number
  client_max_body_size: string
  server_names_hash_bucket_size: string
  client_header_buffer_size: string
  client_body_buffer_size: string
  proxy_cache: ProxyCacheConfig
}

export interface NginxPerfOpt {
  worker_processes: string
  worker_connections: string
  keepalive_timeout: string
  gzip: string
  gzip_min_length: string
  gzip_comp_level: string
  client_max_body_size: string
  server_names_hash_bucket_size: string
  client_header_buffer_size: string
  client_body_buffer_size: string
  proxy_cache: ProxyCacheConfig
}

export interface NgxModule {
  name: string
  params?: string
  dynamic: boolean
  loaded: boolean
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

  toggle_stub_status(enable: boolean): Promise<{ stub_status_enabled: boolean, error: string }> {
    return http.post('/nginx/stub_status', { enable })
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

  get_performance(): Promise<NginxConfigInfo> {
    return http.get('/nginx/performance')
  },

  update_performance(params: NginxPerfOpt): Promise<NginxConfigInfo> {
    return http.post('/nginx/performance', params)
  },

  get_modules(): Promise<NgxModule[]> {
    return http.get('/nginx/modules')
  },
}

export default ngx
