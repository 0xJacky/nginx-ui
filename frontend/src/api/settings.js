import http from '@/lib/http'

const settings = {
    get() {
        return http.get('/settings')
    }
}

export default settings
