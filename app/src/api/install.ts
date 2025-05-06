import http from '@/lib/http'

export interface InstallRequest {
  email: string
  username: string
  password: string
}

export interface InstallLockResponse {
  lock: boolean
  timeout: boolean
}

const install = {
  get_lock() {
    return http.get<InstallLockResponse>('/install')
  },
  install_nginx_ui(data: InstallRequest) {
    return http.post('/install', data, { crypto: true })
  },
}

export default install
