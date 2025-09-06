import { http } from '@uozi-admin/request'

export interface ProcessStats {
  pid: number
}

export interface RestartResponse {
  message: string
}

const system = {
  getProcessStats(): Promise<ProcessStats> {
    return http.get('/system/stats')
  },

  restart(): Promise<RestartResponse> {
    return http.post('/system/restart')
  },
}

export default system
