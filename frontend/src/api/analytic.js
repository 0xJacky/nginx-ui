import http from '@/lib/http'

const analytic = {
    cpu_usage() {
        return http.get('/analytic/cpu')
    }
}

export default analytic
