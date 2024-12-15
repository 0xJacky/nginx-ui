import http from '@/lib/http'

export interface ICP {
  icp_number: string
  public_security_number: string
}

const publicApi = {
  getICP<ICP>() {
    return http.get('/icp_settings')
  },
}

export default publicApi
