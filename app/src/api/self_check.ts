import type { Container } from '@/language'
import type { CosyError } from '@/lib/http'
import { http } from '@uozi-admin/request'

export const ReportStatus = {
  Success: 'success',
  Warning: 'warning',
  Error: 'error',
} as const

export type ReportStatusType = typeof ReportStatus[keyof typeof ReportStatus]

export interface TaskReport {
  key: string
  name: Container
  description: Container
  fixable?: boolean
  err?: CosyError
  status: ReportStatusType
}

const selfCheck = {
  run(): Promise<TaskReport[]> {
    return http.get('/self_check')
  },
  fix(taskName: string) {
    return http.post(`/self_check/${taskName}/fix`)
  },
  websocketUrl: '/api/self_check/websocket',
  timeoutCheck() {
    return http.get('/self_check/timeout')
  },
}

export default selfCheck
