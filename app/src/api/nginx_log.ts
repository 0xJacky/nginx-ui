import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface NginxLogData {
  type?: string
  path?: string
}

export interface AnalyticsRequest {
  path: string
  start_time?: string
  end_time?: string
  limit?: number
}

export interface AccessLogEntry {
  timestamp: string
  ip: string
  location: string
  method: string
  path: string
  protocol: string
  status: number
  bytes_sent: number
  referer: string
  user_agent: string
  browser: string
  browser_version: string
  os: string
  os_version: string
  device_type: string
  request_time?: number
  upstream_time?: number
  raw: string
}

export interface LogStats {
  total_requests: number
  unique_ips: number
  avg_request_time: number
  total_bytes: number
  error_rate: number
  date_range: string
}

export interface IPStat {
  ip: string
  country: string
  requests: number
  bytes: number
}

export interface PathStat {
  path: string
  requests: number
  avg_time: number
  bytes: number
}

export interface StatusStat {
  status: number
  requests: number
}

export interface CountryStat {
  country: string
  requests: number
}

export interface BrowserStat {
  browser: string
  version: string
  requests: number
}

export interface OSStat {
  os: string
  version: string
  requests: number
}

export interface DeviceStat {
  device_type: string
  requests: number
}

export interface LogAnalyticsResponse {
  entries: AccessLogEntry[]
  stats: LogStats
  top_ips: IPStat[]
  top_paths: PathStat[]
  status_distribution: StatusStat[]
  countries: CountryStat[]
  browsers: BrowserStat[]
  os_stats: OSStat[]
  devices: DeviceStat[]
}

export interface SearchFilters {
  query: string
  ip: string
  method: string
  status: string[]
  path: string
  user_agent: string
  referer: string
  browser: string[]
  os: string[]
  device: string[]
}

export interface AdvancedSearchRequest {
  start_time?: string
  end_time?: string
  query?: string
  ip?: string
  method?: string
  status?: number[]
  path?: string
  user_agent?: string
  referer?: string
  browser?: string
  os?: string
  device?: string
  limit?: number
  offset?: number
  sort_by?: string
  sort_order?: string
  log_path?: string
}

export interface SummaryStats {
  uv: number // Unique Visitors (unique IPs)
  pv: number // Page Views (total requests)
  total_traffic: number // Total bytes sent
  unique_pages: number // Unique pages visited
  avg_traffic_per_pv: number // Average traffic per page view
}

export interface AdvancedSearchResponse {
  entries: AccessLogEntry[]
  total: number
  took: number
  query: string
  summary: SummaryStats
}

export interface PreflightResponse {
  start_time: string
  end_time: string
  available: boolean
  index_status: string
}

// Index status related interfaces
export interface FileStatus {
  path: string
  last_modified: string
  last_size: number
  last_indexed: string
  is_compressed: boolean
  has_timerange: boolean
  timerange_start?: string
  timerange_end?: string
}

export interface IndexStatus {
  document_count: number
  log_paths: string[]
  log_paths_count: number
  total_files: number
  files: FileStatus[]
}

const nginx_log = extendCurdApi(useCurdApi('/nginx_logs'), {
  page(page = 0, data: NginxLogData | undefined = undefined) {
    return http.post(`/nginx_log/page?page=${page}`, data)
  },

  analytics(data: AnalyticsRequest): Promise<LogAnalyticsResponse> {
    return http.post('/nginx_log/analytics', data)
  },

  search(data: AdvancedSearchRequest): Promise<AdvancedSearchResponse> {
    return http.post('/nginx_log/search', data)
  },

  getPreflight(logPath?: string): Promise<PreflightResponse> {
    const params = logPath ? { log_path: logPath } : {}
    return http.get('/nginx_log/preflight', { params })
  },

  // Note: getIndexStatus removed - index status is now included in the log list response
  // The nginx_logs endpoint now returns comprehensive information including:
  // - Basic log file information (path, type, name)
  // - Index status for each file (is_indexed, last_indexed)
  // - File metadata (size, modification time)
  // - Time ranges for indexed files
  // - Summary statistics (total files, indexed count, document count)

  // Index management APIs
  rebuildIndex(): Promise<{ message: string }> {
    return http.post('/nginx_log/index/rebuild')
  },

  rebuildFileIndex(path: string): Promise<{ message: string }> {
    return http.post('/nginx_log/index/rebuild', { path })
  },
})

export default nginx_log
