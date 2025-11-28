import type { SiteStatusType } from '@/constants/site-status'
import { http } from '@uozi-admin/request'

export interface SiteInfo {
  id: number // primary identifier for API operations
  health_check_enabled: boolean // whether health check is enabled
  host: string // host:port format
  port: number
  scheme: string // http, https, grpc, grpcs
  display_url: string // computed URL for display
  custom_order: number
  name: string
  status: SiteStatusType
  status_code: number
  response_time: number
  favicon_url: string
  favicon_data: string
  title: string
  last_checked: number
  error?: string
  // Legacy fields for backward compatibility
  url?: string // deprecated, use display_url instead
  health_check_protocol?: string // deprecated, use scheme instead
  host_port?: string // deprecated, use host instead
}

export interface HealthCheckConfig {
  health_check_enabled?: boolean
  check_interval?: number
  timeout?: number
  user_agent?: string
  max_redirects?: number
  follow_redirects?: boolean
  check_favicon?: boolean
  health_check_config?: {
    protocol?: string
    method?: string
    path?: string
    headers?: Record<string, string>
    body?: string
    expected_status?: number[]
    expected_text?: string
    not_expected_text?: string
    validate_ssl?: boolean
    verify_hostname?: boolean
    grpc_service?: string
    grpc_method?: string
    dns_resolver?: string
    source_ip?: string
    client_cert?: string
    client_key?: string
  }
}

export interface HeaderItem {
  name: string
  value: string
}

export interface EnhancedHealthCheckConfig {
  // Basic settings
  enabled: boolean
  interval: number
  timeout: number
  userAgent: string
  maxRedirects: number
  followRedirects: boolean
  checkFavicon: boolean

  // Protocol settings
  protocol: string
  method: string
  path: string
  headers: HeaderItem[]
  body: string

  // Response validation
  expectedStatus: number[]
  expectedText: string
  notExpectedText: string
  validateSSL: boolean
  verifyHostname: boolean

  // gRPC settings
  grpcService: string
  grpcMethod: string

  // Advanced settings
  dnsResolver: string
  sourceIP: string
  clientCert: string
  clientKey: string
}

export interface HealthCheckTestConfig {
  protocol: string
  method: string
  path: string
  headers: Record<string, string>
  body: string
  expected_status: number[]
  expected_text: string
  not_expected_text: string
  validate_ssl: boolean
  grpc_service: string
  grpc_method: string
  timeout: number
}

export interface SiteNavigationResponse {
  data: SiteInfo[]
}

export interface SiteNavigationStatusResponse {
  running: boolean
}

export const siteNavigationApi = {
  // Get all sites for navigation
  getSites(): Promise<SiteNavigationResponse> {
    return http.get('/site_navigation')
  },

  // Get service status
  getStatus(): Promise<SiteNavigationStatusResponse> {
    return http.get('/site_navigation/status')
  },

  // Update sites order
  updateOrder(orderedIds: number[]): Promise<{ message: string }> {
    return http.post('/site_navigation/order', { ordered_ids: orderedIds })
  },

  // Get health check configuration
  getHealthCheck(id: number): Promise<HealthCheckConfig> {
    return http.get(`/site_navigation/health_check/${id}`)
  },

  // Update health check configuration
  updateHealthCheck(id: number, config: HealthCheckConfig): Promise<{ message: string }> {
    return http.post(`/site_navigation/health_check/${id}`, config)
  },

  // Test health check configuration
  testHealthCheck(id: number, config: HealthCheckTestConfig): Promise<{ success: boolean, response_time?: number, error?: string }> {
    return http.post(`/site_navigation/test_health_check/${id}`, { config })
  },

  // WebSocket URL for real-time updates
  websocketUrl: '/api/site_navigation_ws',
}
