import http from '@/lib/http'

interface IData {
    type: string
    conf_name: string
    server_idx: number
    directive_idx: number
}

const nginx_log = {
    page(page = 0, data: IData) {
        return http.post('/nginx_log?page=' + page, data)
    }
}

export default nginx_log
