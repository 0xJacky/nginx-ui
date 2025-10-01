import { http } from '@uozi-admin/request'

export interface AppSettings {
  page_size: number
  jwt_secret: string
}

export interface ServerSettings {
  host: string
  port: number
  run_mode: 'debug' | 'release'
  enable_https: boolean
  ssl_cert: string
  ssl_key: string
  // HTTP/2 and HTTP/3 protocol support (fixed priority: h3->h2->h1)
  enable_h2: boolean
  enable_h3: boolean
}

export interface DatabaseSettings {
  name: string
}

export interface AuthSettings {
  ip_white_list: string[]
  ban_threshold_minutes: number
  max_attempts: number
}

export interface CasdoorSettings {
  endpoint: string
  client_id: string
  client_secret: string
  certificate_path: string
  organization: string
  application: string
  redirect_uri: string
}

export interface CertSettings {
  email: string
  ca_dir: string
  renewal_interval: number
  recursive_nameservers: string[]
  http_challenge_port: string
}

export interface HTTPSettings {
  github_proxy: string
  insecure_skip_verify: boolean
}

export interface LogrotateSettings {
  enabled: boolean
  cmd: string
  interval: number
}

export interface NginxSettings {
  access_log_path: string
  error_log_path: string
  config_dir: string
  config_path: string
  log_dir_white_list: string[]
  pid_path: string
  test_config_cmd: string
  reload_cmd: string
  restart_cmd: string
  stub_status_port: number
  container_name: string
}

export interface NginxLogSettings {
  advanced_indexing_enabled: boolean
  index_path: string
}

export interface NodeSettings {
  name: string
  secret: string
  skip_installation: boolean
  demo: boolean
  icp_number: string
  public_security_number: string
}

export interface OpenaiSettings {
  model: string
  base_url: string
  proxy: string
  token: string
  api_type: string
  enable_code_completion: boolean
  code_completion_model: string
}

export interface TerminalSettings {
  start_cmd: string
}

export interface WebauthnSettings {
  rp_display_name: string
  rpid: string
  rp_origins: string[]
}

export interface BannedIP {
  ip: string
  attempts: number
  expired_at: string
}

export interface Settings {
  app: AppSettings
  server: ServerSettings
  database: DatabaseSettings
  auth: AuthSettings
  casdoor: CasdoorSettings
  cert: CertSettings
  http: HTTPSettings
  logrotate: LogrotateSettings
  nginx: NginxSettings
  nginx_log: NginxLogSettings
  node: NodeSettings
  openai: OpenaiSettings
  terminal: TerminalSettings
  webauthn: WebauthnSettings
}

const settings = {
  get(): Promise<Settings> {
    return http.get('/settings')
  },
  save(data: Settings) {
    return http.post('/settings', data)
  },
  get_server_name(): Promise<{ name: string }> {
    return http.get('/settings/server/name')
  },
  get_banned_ips(): Promise<BannedIP[]> {
    return http.get('/settings/auth/banned_ips')
  },
  remove_banned_ip(ip: string) {
    return http.delete('/settings/auth/banned_ip', { data: { ip } })
  },
}

export default settings
