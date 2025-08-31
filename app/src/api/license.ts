import { http } from '@uozi-admin/request'

export interface License {
  name: string
  license: string
  url: string
  version: string
}

export interface ComponentInfo {
  backend: License[]
  frontend: License[]
}

export interface LicenseStats {
  total_backend: number
  total_frontend: number
  total: number
  license_distribution: Record<string, number>
}

const license = {
  getAll(): Promise<ComponentInfo> {
    return http.get('/licenses')
  },
  getBackend(): Promise<License[]> {
    return http.get('/licenses/backend')
  },
  getFrontend(): Promise<License[]> {
    return http.get('/licenses/frontend')
  },
  getStats(): Promise<LicenseStats> {
    return http.get('/licenses/stats')
  },
}

export default license
