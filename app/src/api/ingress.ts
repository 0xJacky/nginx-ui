import type { AxiosRequestConfig } from 'axios'
import { http } from '@uozi-admin/request'

// Declarative reverse proxy entries published by external controllers.
// The contract is documented in docs/guide/ingress.md.

export type IngressLB = 'round_robin' | 'least_conn' | 'ip_hash'

export type IngressTLSMode = 'none' | 'acme' | 'cert'

export type IngressChallenge = 'http01' | 'dns01'

export type IngressPhase = 'Pending' | 'Applying' | 'Ready' | 'Degraded' | 'Error'

export type IngressConditionType = 'ConfigApplied' | 'CertReady' | 'BackendsHealthy'

export interface IngressServer {
  // host:port inside the allowed backend ranges
  address: string
  weight?: number
  max_fails?: number
  fail_timeout?: string
  down: boolean
  backup: boolean
}

export interface IngressBackend {
  name: string
  lb?: IngressLB
  servers: IngressServer[]
  keepalive?: number
}

export interface IngressRoute {
  path_prefix: string
  backend: string
  websocket: boolean
  read_timeout?: string
  send_timeout?: string
  client_max_body_size?: string
  // Replaces the policy list for this route; empty means the policy applies.
  allow_cidrs?: string[]
}

export interface IngressTLS {
  mode: IngressTLSMode
  cert_id?: number
  key_type?: string
  challenge?: IngressChallenge
  dns_credential_id?: number
  acme_user_id?: number
  redirect_https: boolean
  hsts: boolean
}

export interface IngressRateLimit {
  rps: number
  burst: number
}

export interface IngressHeader {
  name: string
  value: string
}

export interface IngressPolicy {
  allow_cidrs?: string[]
  rate_limit?: IngressRateLimit | null
  client_max_body_size?: string
  extra_headers?: IngressHeader[]
}

export interface IngressSpec {
  hosts: string[]
  backends: IngressBackend[]
  routes: IngressRoute[]
  tls: IngressTLS
  policy: IngressPolicy
}

export interface IngressCondition {
  type: IngressConditionType | string
  status: boolean
  reason?: string
  message?: string
  last_transition_time: string
}

export interface IngressServerStatus {
  backend: string
  address: string
  down: boolean
  // Raw probe result; only meaningful when checked is true.
  healthy: boolean
  // False until the availability checker has reported on this server.
  checked: boolean
}

export interface IngressStatus {
  observed_generation: number
  phase: IngressPhase | ''
  conditions: IngressCondition[] | null
  config_hash?: string
  cert_id?: number
  cert_not_after?: string | null
  last_applied_at?: string | null
  servers?: IngressServerStatus[] | null
}

export interface Ingress {
  id: number
  name: string
  owner: string
  external_id: string
  labels: Record<string, string> | null
  namespace_id: number
  generation: number
  // Generated site file name under sites-available.
  site_path: string
  spec: IngressSpec
  status: IngressStatus
  created_at: string
  updated_at: string
}

export interface IngressUpsertRequest {
  owner: string
  external_id?: string
  labels?: Record<string, string>
  namespace_id?: number
  // Transfers the ingress from another owner.
  force?: boolean
  spec: IngressSpec
}

export interface IngressSyncItem {
  name: string
  external_id?: string
  labels?: Record<string, string>
  namespace_id?: number
  spec: IngressSpec
}

export interface IngressSyncRequest {
  items: IngressSyncItem[]
}

export interface IngressServerPatch {
  owner: string
  address: string
  down: boolean
}

export interface IngressRendered {
  site_name: string
  site_content: string
  upstreams: Record<string, string>
  hash: string
}

export interface IngressApplyResult {
  name: string
  changed: boolean
  rendered: IngressRendered
  test_output: string
  error: string
}

export interface IngressWriteOptions {
  // Go duration, e.g. 30s: block until the new generation is applied.
  wait?: string
}

export interface IngressDeleteOptions {
  owner?: string
  // Delete even when the owner does not match.
  force?: boolean
  // Also remove the certificate record and files.
  purge_cert?: boolean
}

export interface IngressSyncOptions extends IngressWriteOptions {
  owner: string
  prune?: boolean
}

const baseUrl = '/ingresses'

function path(name: string) {
  return `${baseUrl}/${encodeURIComponent(name)}`
}

function flag(value?: boolean) {
  return value ? 1 : undefined
}

const ingress = {
  getList(owner?: string, config?: AxiosRequestConfig): Promise<{ data: Ingress[] }> {
    return http.get(baseUrl, { ...config, params: { owner: owner || undefined } })
  },

  getItem(name: string, config?: AxiosRequestConfig): Promise<Ingress> {
    return http.get(path(name), config)
  },

  upsert(name: string, data: IngressUpsertRequest, options: IngressWriteOptions = {}): Promise<Ingress> {
    return http.put(path(name), data, { params: { wait: options.wait } })
  },

  dryRun(name: string, data: IngressUpsertRequest): Promise<IngressApplyResult> {
    return http.put(path(name), data, { params: { dry_run: 1 } })
  },

  deleteItem(name: string, options: IngressDeleteOptions = {}) {
    return http.delete(path(name), {
      params: {
        owner: options.owner || undefined,
        force: flag(options.force),
        purge_cert: flag(options.purge_cert),
      },
    })
  },

  sync(data: IngressSyncRequest, options: IngressSyncOptions): Promise<{ data: Ingress[] }> {
    return http.put(baseUrl, data, {
      params: {
        owner: options.owner,
        prune: flag(options.prune),
        wait: options.wait,
      },
    })
  },

  patchServer(name: string, backend: string, data: IngressServerPatch, options: IngressWriteOptions = {}): Promise<Ingress> {
    return http.patch(`${path(name)}/backends/${encodeURIComponent(backend)}/servers`, data, {
      params: { wait: options.wait },
    })
  },

  reconcile(name: string): Promise<Ingress> {
    return http.post(`${path(name)}/reconcile`)
  },
}

export default ingress
