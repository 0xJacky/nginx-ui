import { http } from '@uozi-admin/request'

export interface UpstreamStatus {
  online: boolean
  latency: number
}

export interface UpstreamAvailabilityResponse {
  results: Record<string, UpstreamStatus>
  targets: Array<{
    host: string
    port: string
    type: string
    config_path: string
    last_seen: string
  }>
  last_update_time: string
  target_count: number
}

export interface SocketInfo {
  socket: string
  host: string
  port: string
  type: string
  is_consul: boolean
  upstream_name: string
  last_check: string
  status: UpstreamStatus | null
  enabled: boolean
}

export interface SocketListResponse {
  data: SocketInfo[]
}

export interface UpdateSocketConfigRequest {
  enabled: boolean
}

const upstream = {
  // HTTP GET interface to get all upstream availability results
  getAvailability(): Promise<UpstreamAvailabilityResponse> {
    return http.get('/upstream/availability')
  },

  // WebSocket URL for real-time availability updates
  availabilityWebSocketUrl: '/api/upstream/availability_ws',

  // Get all sockets with their configuration and health status
  getSocketList(): Promise<SocketListResponse> {
    return http.get('/upstream/sockets')
  },

  // Update socket configuration
  updateSocketConfig(socket: string, data: UpdateSocketConfigRequest) {
    return http.put(`/upstream/socket/${encodeURIComponent(socket)}`, data)
  },
}

export default upstream
