import http from '@/lib/http'

const ngx = {
    build_config(ngxConfig: any) {
        return http.post('/ngx/build_config', ngxConfig)
    },

    tokenize_config(content: string) {
        return http.post('/ngx/tokenize_config', {content})
    }
}

export default ngx
