import { http } from '@uozi-admin/request'

export interface InstallRequest {
  email: string
  username: string
  password: string
}

function installSecretHeaders(installSecret: string) {
  return {
    'X-Install-Secret': installSecret,
  }
}

export interface InstallLockResponse {
  lock: boolean
  timeout: boolean
}

const install = {
  get_lock() {
    return http.get<InstallLockResponse>('/install')
  },
  install_nginx_ui(data: InstallRequest, installSecret: string) {
    return http.post('/setup/install', data, {
      crypto: true,
      headers: installSecretHeaders(installSecret),
      skipAuthRedirect: true,
    })
  },
}

export default install
