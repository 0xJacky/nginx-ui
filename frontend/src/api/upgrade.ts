import http from '@/lib/http'

const upgrade = {
    get_latest_release() {
        return http.get('/upgrade/release')
    },
    current_version() {
        return http.get('/upgrade/current')
    }
}

export default upgrade
