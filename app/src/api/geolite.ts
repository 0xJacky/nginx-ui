import { http } from '@uozi-admin/request'

export interface GeoLiteStatus {
  exists: boolean
  path: string
  size: number
  last_modified: string
}

const geolite = {
  async getStatus() {
    return http.get<GeoLiteStatus>('geolite/status')
  },
}

export default geolite
