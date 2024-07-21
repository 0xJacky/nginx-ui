import http from '@/lib/http'

export interface BannedIP {
  ip: string
  attempts: number
  expired_at: string
}

const settings = {
  get<T>(): Promise<T> {
    return http.get('/settings')
  },
  save<T>(data: T) {
    return http.post('/settings', data)
  },
  get_server_name(): Promise<{ name: string }> {
    return http.get('/settings/server/name')
  },
  get_banned_ips(): Promise<BannedIP[]> {
    return http.get('/settings/auth/banned_ips')
  },
  remove_banned_ip(ip: string) {
    return http.delete('/settings/auth/banned_ip', { data: { ip } })
  },
}

export default settings
