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
}

export interface AutoCertRequest {
  dns_credential_id: number | null
  challenge_method: string
  domains: string[]
  key_type: PrivateKeyType
}

const baseUrl = '/sites'

const site = extendCurdApi(useCurdApi<Site>(baseUrl), {
  enable: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/enable`),
  disable: (name: string) => http.post(`${baseUrl}/${name}/disable`),
  rename: (oldName: string, newName: string) => http.post(`${baseUrl}/${encodeURIComponent(oldName)}/rename`, { new_name: newName }),
  get_default_template: () => http.get('default_site_template'),
  add_auto_cert: (domain: string, data: AutoCertRequest) => http.post(`auto_cert/${encodeURIComponent(domain)}`, data),
  remove_auto_cert: (domain: string) => http.delete(`auto_cert/${encodeURIComponent(domain)}`),
  duplicate: (name: string, data: { name: string }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/duplicate`, data),
  advance_mode: (name: string, data: { advanced: boolean }) => http.post(`${baseUrl}/${encodeURIComponent(name)}/advance`, data),
  enableMaintenance: (name: string) => http.post(`${baseUrl}/${encodeURIComponent(name)}/maintenance`),
})

export default site
