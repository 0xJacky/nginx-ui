import type { CosyError } from '@/lib/http'
import http from '@/lib/http'
import ws from '@/lib/websocket'

export interface Report {
  name: string
  err?: CosyError
}

const selfCheck = {
  run(): Promise<Report[]> {
    return http.get('/self_check')
  },
  fix(taskName: string) {
    return http.post(`/self_check/${taskName}/fix`)
  },
  websocket() {
    return ws('/api/self_check/websocket', false)
  },
}

export default selfCheck
