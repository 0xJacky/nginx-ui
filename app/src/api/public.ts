import { http } from '@uozi-admin/request'

export interface ICP {
  icp_number: string
  public_security_number: string
}

const publicApi = {
  getICP(): Promise<ICP> {
    return http.get('/icp_settings')
  },
}

export default publicApi
