import { http } from '@uozi-admin/request'
import ws from '@/lib/websocket'

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

const upstream = {
  // HTTP GET interface to get all upstream availability results
  getAvailability(): Promise<UpstreamAvailabilityResponse> {
    return http.get('/upstream/availability')
  },

  // WebSocket interface for real-time availability updates
  availabilityWebSocket() {
    return ws('/api/upstream/availability_ws')
  },
}

export default upstream
