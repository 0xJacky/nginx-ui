import http from '@/lib/http'

const ngx = {
    build_config(ngxConfig) {
        return http.post('/ngx/build_config', ngxConfig)
    },

    tokenize_config(content) {
        return http.post('/ngx/tokenize_config', {content})
    }
}

export default ngx
