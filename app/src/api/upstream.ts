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
  global_health_check_enabled: boolean
}

export interface UpstreamHealthCheckStatusResponse {
  enabled: boolean
  interval_seconds: number
}

export interface UpdateSocketConfigRequest {
  enabled: boolean
}

export type UpstreamMethod = '' | 'least_conn' | 'ip_hash' | 'hash' | 'random'

export interface ManagedUpstreamServer {
  address: string
  weight?: number | null
  max_fails?: number | null
  fail_timeout?: string
  backup: boolean
  down: boolean
  // Server parameters the form does not model, kept for round trips.
  params?: string
}

export interface ManagedUpstream {
  name: string
  method: UpstreamMethod
  hash_key?: string
  consistent: boolean
  keepalive: number
  // Shared memory zone named after the group: `zone <name> <zone_size>;`
  zone: boolean
  zone_size?: string
  servers: ManagedUpstreamServer[]
  extra_directives?: string
}

export interface UpstreamReference {
  type: 'site' | 'config'
  name: string
  path: string
}

export interface ManagedUpstreamDetail extends ManagedUpstream {
  path: string
  content: string
  references: UpstreamReference[]
}

export type UpstreamSourceType = 'managed' | 'site' | 'stream' | 'config'

// The file an upstream block lives in: a site or stream name, the managed
// group name, or a path relative to the nginx configuration directory.
export interface UpstreamSource {
  type: UpstreamSourceType
  name: string
}

// State of one `server` line of an upstream block.
export interface UpstreamServerState {
  address: string
  // host:port key of the availability results
  socket: string
  down: boolean
  backup: boolean
  weight?: number
  params?: string
}

export interface UpstreamGroupState {
  name: string
  config_path: string
  source: UpstreamSource
  servers: UpstreamServerState[]
}

export interface ExternalUpstream extends UpstreamGroupState {
  // Set when the file could not be read back; its servers cannot be toggled.
  read_only: boolean
}

export interface UpstreamServerStateRequest {
  upstream: string
  config_path: string
  address: string
  enabled: boolean
}

export interface ManagedUpstreamListResponse {
  data: ManagedUpstreamDetail[]
  external: ExternalUpstream[]
  dir: string
}

export interface UpstreamPreviewResponse {
  content: string
  file_name: string
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

  getHealthCheckStatus(): Promise<UpstreamHealthCheckStatusResponse> {
    return http.get('/upstream/health_check/status')
  },

  // Update socket configuration
  updateSocketConfig(socket: string, data: UpdateSocketConfigRequest) {
    return http.put(`/upstream/socket/${encodeURIComponent(socket)}`, data)
  },

  // Standalone upstream groups managed by Nginx UI (conf.d/upstream-<name>.conf)
  getManagedList(): Promise<ManagedUpstreamListResponse> {
    return http.get('/upstreams')
  },

  getManaged(name: string): Promise<ManagedUpstreamDetail> {
    return http.get(`/upstreams/${encodeURIComponent(name)}`)
  },

  createManaged(data: ManagedUpstream): Promise<ManagedUpstreamDetail> {
    return http.post('/upstreams', data)
  },

  updateManaged(name: string, data: ManagedUpstream): Promise<ManagedUpstreamDetail> {
    return http.post(`/upstreams/${encodeURIComponent(name)}`, data)
  },

  deleteManaged(name: string) {
    return http.delete(`/upstreams/${encodeURIComponent(name)}`)
  },

  // Switch one server of any upstream block on or off (`down` parameter). The
  // file is saved like its own editor saves it: nginx -t, reload, rollback.
  setServerState(data: UpstreamServerStateRequest): Promise<UpstreamGroupState> {
    return http.post('/upstream/server_state', data)
  },

  // Validation errors are shown inline by the form, so the global error toast
  // is skipped here.
  previewManaged(data: ManagedUpstream): Promise<UpstreamPreviewResponse> {
    return http.post('/upstream/preview', data, { skipErrHandling: true })
  },
}

export default upstream
