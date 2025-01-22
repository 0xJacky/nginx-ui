import type { CosyError } from '@/lib/http'
import http from '@/lib/http'

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
}

export default selfCheck
