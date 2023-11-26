import http from '@/lib/http'

const analytic = {
  init() {
    return http.get('/analytic/init')
  }
}

export default analytic
