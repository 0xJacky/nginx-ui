import http from '@/lib/http'
import { useUserStore } from '@/pinia'
import { SSE } from 'sse.js'

export interface INginxLogData {
  type: string
  log_path?: string
}

const nginx_log = {
  page(page = 0, data: INginxLogData | undefined = undefined) {
    return http.post(`/nginx_log?page=${page}`, data)
  },

  get_list(params: {
    type?: string
    name?: string
    path?: string
  }) {
    return http.get(`/nginx_logs`, { params })
  },

  logs_live() {
    const { token } = useUserStore()
    const url = `/api/nginx_logs/index_status`

    return new SSE(url, {
      headers: {
        Authorization: token,
      },
    })
  },
}

export default nginx_log
