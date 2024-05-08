import http from '@/lib/http'

const settings = {
  get() {
    return http.get('/settings')
  },
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  save(data: any) {
    return http.post('/settings', data)
  },

  get_server_name() {
    return http.get('/settings/server/name')
  },
}

export default settings
