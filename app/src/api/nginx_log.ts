import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface NginxLogData {
  type?: string
  path?: string
}

const nginx_log = extendCurdApi(useCurdApi('/nginx_logs'), {
  page(page = 0, data: NginxLogData | undefined = undefined) {
    return http.post(`/nginx_log?page=${page}`, data)
  },
})

export default nginx_log
