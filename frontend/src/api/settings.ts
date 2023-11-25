import http from '@/lib/http'

const settings = {
  get() {
    return http.get('/settings')
  },
  save(data: any) {
    return http.post('/settings', data)
  }
}

export default settings
