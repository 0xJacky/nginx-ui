import http from '@/lib/http'

const ngx = {
    build_config(ngxConfig: any) {
        return http.post('/ngx/build_config', ngxConfig)
    },

    tokenize_config(content: string) {
        return http.post('/ngx/tokenize_config', {content})
    },

    format_code(content: string) {
        return http.post('/ngx/format_code', {content})
    },

    status() {
        return http.get('/nginx/status')
    },

    reload() {
        return http.post('/nginx/reload')
    },

    restart() {
        return http.post('/nginx/restart')
    },

    test() {
        return http.post('/nginx/test')
    }
}

export default ngx
