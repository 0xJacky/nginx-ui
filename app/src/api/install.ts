import http from '@/lib/http'

export interface InstallRequest {
  email: string
  username: string
  password: string
  database: string
}

const install = {
  get_lock() {
    return http.get('/install')
  },
  install_nginx_ui(data: InstallRequest) {
    return http.post('/install', data, { crypto: true })
  },
}

export default install
