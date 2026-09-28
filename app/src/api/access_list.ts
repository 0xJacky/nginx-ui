import type { ModelBase } from '@/api/curd'
import type { NgxConfig } from '@/api/ngx'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export type AccessRuleType = 'allow' | 'deny' | 'ref'
export type AccessFallback = 'allow' | 'deny'

export interface AccessRule {
  type: AccessRuleType
  value?: string
  ref_id?: number
  note?: string
}

export interface AccessListWarning {
  code: 'shadowed' | 'allows_all' | 'host_bits' | 'empty'
  rule: number
}

export interface AccessList extends ModelBase {
  name: string
  slug: string
  fallback: AccessFallback
  rules: AccessRule[]
  warnings?: AccessListWarning[]
  used_by?: number
}

export interface AccessListPayload {
  id?: number
  name: string
  slug?: string
  fallback: AccessFallback
  rules: AccessRule[]
}

export interface AccessListPreview {
  content: string
  warnings: AccessListWarning[]
}

export interface AccessReference {
  kind: 'site' | 'stream'
  name: string
  server: number
  server_name: string
  location?: string
  slug: string
}

export interface AccessListUsage {
  lists: AccessList[]
  references: AccessReference[]
}

// Server blocks are public or use a list; locations inherit the server, are
// public, use their own list or are kept reachable for ACME challenges.
// Manual blocks contain allow/deny rules Nginx UI did not write.
export type AccessMode = 'public' | 'inherit' | 'list' | 'acme' | 'manual'

// The list pages summarise all server blocks of a file; mixed means they
// differ.
export type AccessSummaryMode = 'public' | 'list' | 'manual' | 'mixed'

export interface AccessLocationState {
  index: number
  path: string
  mode: AccessMode
  slug?: string
  acme: boolean
}

export interface AccessServerState {
  index: number
  server_name: string
  mode: AccessMode
  slug?: string
  locations: AccessLocationState[]
}

export interface AccessChange {
  server: number
  location?: number
  mode: 'public' | 'inherit' | 'list'
  slug?: string
}

export interface AccessControlSource {
  content?: string
  config?: NgxConfig
}

export interface AccessControlResult extends AccessControlSource {
  servers: AccessServerState[]
}

export interface AccessBatchResult {
  name: string
  success: boolean
  error?: string
}

const baseUrl = '/access_lists'

const accessList = extendCurdApi(useCurdApi<AccessList>(baseUrl), {
  getAll: (): Promise<{ data: AccessList[] }> => http.get(baseUrl),
  preview: (data: AccessListPayload): Promise<AccessListPreview> =>
    http.post(`${baseUrl}/preview`, data, { skipErrHandling: true }),
  getUsage: (id: number): Promise<AccessListUsage> => http.get(`${baseUrl}/${id}/usage`),
  // The editor refreshes the state while the operator types, so an
  // unparsable intermediate state must not raise an error toast.
  getState: (source: AccessControlSource): Promise<AccessControlResult> =>
    http.post('/access_control/state', source, { skipErrHandling: true }),
  apply: (source: AccessControlSource, changes: AccessChange[]): Promise<AccessControlResult> =>
    http.post('/access_control/apply', { ...source, changes }),
  batchApply: (data: {
    kind: 'site' | 'stream'
    names: string[]
    mode: 'public' | 'list'
    slug?: string
  }): Promise<{ results: AccessBatchResult[] }> => http.post('/access_control/batch', data),
})

export default accessList
