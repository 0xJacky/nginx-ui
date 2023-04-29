import http from '@/lib/http'

export interface INginxLogData {
    type: string
    conf_name: string
    server_idx: number
    directive_idx: number
}

const nginx_log = {
    page(page = 0, data: INginxLogData) {
        return http.post('/nginx_log?page=' + page, data)
    }
}
export default nginx_log
