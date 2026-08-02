import { http } from '@uozi-admin/request'

export interface PublicNodeInfo {
  icp_number: string
  public_security_number: string
  /** Demo instances fabricate state and refuse anything with outside effects. */
  demo: boolean
}

/** @deprecated use PublicNodeInfo */
export type ICP = PublicNodeInfo

const publicApi = {
  getICP(): Promise<PublicNodeInfo> {
    return http.get('/icp_settings')
  },
}

export default publicApi
