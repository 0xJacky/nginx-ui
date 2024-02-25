export interface Settings {
  server: {
    http_host: string
    http_port: string
    run_mode: string
    jwt_secret: string
    node_secret: string
    start_cmd: string
    http_challenge_port: string
    github_proxy: string
    email: string
    ca_dir: string
  }
  nginx: {
    access_log_path: string
    error_log_path: string
    config_dir: string
    pid_path: string
    reload_cmd: string
    restart_cmd: string
  }
  openai: {
    model: string
    base_url: string
    proxy: string
    token: string
  }
  logrotate: {
    enabled: boolean
    cmd: string
    interval: number
  }
}
