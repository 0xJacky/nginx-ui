import type { CertificateInfo } from '@/api/cert'
import type { ModelBase } from '@/api/curd'
import type { Namespace } from '@/api/namespace'
import type { NgxConfig } from '@/api/ngx'
import type { ConfigStatus, PrivateKeyType } from '@/constants'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export type SiteStatus = ConfigStatus.Enabled | ConfigStatus.Disabled | ConfigStatus.Maintenance

export interface ProxyTarget {
  host: string
  port: string
  type: string // "proxy_pass" or "upstream"
}

export interface SiteDNSRecord {
  id: string
  name: string
  type: string
  exists: boolean
}

export interface Site extends ModelBase {
  modified_at: string
  path: string
  advanced: boolean
  name: string
  filepath: string
  config: string
  auto_cert: boolean
  tokenized?: NgxConfig
  cert_info?: Record<number, CertificateInfo[]>
  namespace_id: number
  namespace?: Namespace
  sync_node_ids: number[]
  urls?: string[]
  proxy_targets?: ProxyTarget[]
  status: SiteStatus
  dns_domain_id?: number | null
  dns_records?: SiteDNSRecord[] | null
  // Legacy single-record fields are kept for backward compatibility.
  dns_record_id?: string | null
  dns_record_name?: string | null
  dns_record_type?: string | null
  dns_record_exists?: boolean | null
}

export interface SiteLog {
  type: 'access' | 'error'
  path: string
  inherited: boolean
  valid: boolean
}

export interface AutoCertRequest {
  dns_credential_id: number | null
  challenge_method: string
  profile?: string
  domains: string[]
  key_type: PrivateKeyType
  acme_user_id?: number
  must_staple?: boolean
  lego_disable_cname_support?: boolean
  enable_common_name?: boolean
  revoke_old?: boolean
}

const baseUrl = '/sites'

const site = extendCurdApi(useCurdApi<Site>(baseUrl), {
  enable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/enable`),
  disable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/disable`),
  batchEnable: (names: string[]) => http.post(`${baseUrl}/batch/enable`, { names }),
  batchDisable: (names: string[]) => http.post(`${baseUrl}/batch/disable`, { names }),
  rename: (oldName: string, newName: string) => http.post(`${baseUrl}/${encodeURIComponent(oldName)}/rename`, { new_name: newName }),
  get_default_template: () => http.get('default_site_template'),
  add_auto_cert: (domain: string, data: AutoCertRequest) => http.post(`auto_cert/${encodeURIComponent(domain)}`, data),
  remove_auto_cert: (domain: string) => http.delete(`auto_cert/${encodeURIComponent(domain)}`),
  duplicate: (name: string, data: { name: string }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/duplicate`, data),
  advance_mode: (name: string, data: { advanced: boolean }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/advance`, data),
  enableMaintenance: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/maintenance`),
  getLogs: (name: string) => http.get<{ logs: SiteLog[] }>(`${baseUrl}/${encodeURIComponent(name)}/logs`),
})

export default site
