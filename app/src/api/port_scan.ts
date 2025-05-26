import { http } from '@uozi-admin/request'

export interface PortScanRequest {
  start_port: number
  end_port: number
  page: number
  page_size: number
}

export interface PortInfo {
  port: number
  status: string
  process: string
}

export interface PortScanResponse {
  data: PortInfo[]
  total: number
  page: number
  page_size: number
}

const portScan = {
  scan: (data: PortScanRequest) => http.post<PortScanResponse>('/system/port_scan', data),
}

export default portScan
