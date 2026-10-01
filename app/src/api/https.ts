import type { AutoCertChallengeMethod } from '@/api/auto_cert'
import type { PrivateKeyType } from '@/constants'
import { http } from '@uozi-admin/request'

// Contract for the backend-orchestrated HTTPS enablement of a site:
// WebSocket `GET /api/sites/:name/https` plus the side-effect free
// `POST /api/sites/:name/https/check`.

export type HTTPSChallengeMethod = keyof typeof AutoCertChallengeMethod

// `delegate` is reported by the client while the main node issues the
// certificate for the selected node, before the run on the node starts.
export type HTTPSStep = 'delegate' | 'plan' | 'stage' | 'probe' | 'issue' | 'finalize' | 'rollback'

export type HTTPSStepStatus = 'running' | 'success' | 'warning' | 'error' | 'skipped'

export type HTTPSCheckStatus = Exclude<HTTPSStepStatus, 'running'>

// Translation arguments attached to server-side English source strings.
// eslint-disable-next-line ts/no-explicit-any
export type HTTPSMessageArgs = Record<string, any>

// The single message the client sends after the socket opens.
export interface HTTPSRequest {
  domains: string[]
  challenge_method: HTTPSChallengeMethod
  dns_credential_id: number
  redirect_http_to_https: boolean
  key_type: PrivateKeyType | string
  acme_user_id: number
  profile: string
  must_staple: boolean
  /** Free-form payload owned by the plugin that implements the challenge method. */
  challenge_config?: Record<string, unknown>
  enable_common_name: boolean
  revoke_old: boolean
  // A certificate record from the certificate manager to use instead of
  // issuing one; 0 or absent issues a new certificate with ACME.
  certificate_id?: number
}

export interface HTTPSHint {
  // Stable machine code, e.g. `dns_points_elsewhere`.
  code: string
  // English sentence; `params` are its translation arguments.
  message: string
  params?: HTTPSMessageArgs
}

export interface HTTPSDiagnostic {
  level: 'info' | 'warning' | 'error'
  code: string
  message: string
  params?: HTTPSMessageArgs
}

export interface HTTPSStepEvent {
  type: 'step'
  step: HTTPSStep
  status: HTTPSStepStatus
  message?: string
  args?: HTTPSMessageArgs
}

export interface HTTPSLogEvent {
  type: 'log'
  message: string
  args?: HTTPSMessageArgs
}

export interface HTTPSDiagnosticEvent extends HTTPSDiagnostic {
  type: 'diagnostic'
}

export interface HTTPSResult {
  ssl_certificate: string
  ssl_certificate_key: string
  key_type: string
  profile?: string
  cert_id?: number
}

export interface HTTPSDoneSuccessEvent extends HTTPSResult {
  type: 'done'
  status: 'success'
}

export interface HTTPSDoneErrorEvent {
  type: 'done'
  status: 'error'
  step?: HTTPSStep
  message: string
  args?: HTTPSMessageArgs
  hint?: HTTPSHint
}

export type HTTPSDoneEvent = HTTPSDoneSuccessEvent | HTTPSDoneErrorEvent

export type HTTPSEvent = HTTPSStepEvent | HTTPSLogEvent | HTTPSDiagnosticEvent | HTTPSDoneEvent

export interface HTTPSCheckRequest {
  domains: string[]
  challenge_method: HTTPSChallengeMethod
  // Checks coverage and expiry of an existing certificate instead of the
  // challenge route and DNS.
  certificate_id?: number
}

export interface HTTPSCheck {
  // e.g. `config_parse`, `challenge_route`, `dns`, `certificate` (an existing
  // certificate; params `name`, `not_after`, `uncovered`).
  code: string
  status: HTTPSCheckStatus
  message: string
  params?: HTTPSMessageArgs
  // Optional raw detail (resolved addresses, probe response, ...).
  detail?: string
}

export interface HTTPSCheckResponse {
  checks: HTTPSCheck[]
}

export function httpsWebSocketPath(name: string): string {
  return `/api/sites/${encodeURIComponent(name)}/https`
}

const https = {
  check(name: string, payload: HTTPSCheckRequest): Promise<HTTPSCheckResponse> {
    return http.post(`/sites/${encodeURIComponent(name)}/https/check`, payload)
  },
}

export default https
