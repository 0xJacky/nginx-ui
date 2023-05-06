import http from '@/lib/http'

const upgrade = {
    get_latest_release(channel: string) {
        return http.get('/upgrade/release', {
            params: {
                channel
            }
        })
    },
    current_version() {
        return http.get('/upgrade/current')
    }
}

export default upgrade
