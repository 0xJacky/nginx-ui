import { http } from '@uozi-admin/request'

export interface INginxLogData {
  type: string
  log_path?: string
}

const nginx_log = {
  page(page = 0, data: INginxLogData | undefined = undefined) {
    return http.post(`/nginx_log?page=${page}`, data)
  },

  getList(params: {
    type?: string
    name?: string
    path?: string
  }) {
    return http.get(`/nginx_logs`, { params })
  },
}

export default nginx_log
