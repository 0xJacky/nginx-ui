import type { ModelBase, Pagination } from '@/api/curd'
import type { DnsCredential } from '@/api/dns_credential'
import { http, useCurdApi } from '@uozi-admin/request'

export interface DNSDomain extends ModelBase {
  domain: string
  description?: string
  dns_credential_id: number
  dns_credential?: {
    id: number
    name: string
    provider: string
    provider_code?: string
  }
}

export interface DNSRecord {
  id: string
  type: string
  name: string
  content: string
  ttl: number
  priority?: number
  weight?: number
  proxied?: boolean
}

export interface DDNSRecordTarget {
  id: string
  name: string
  type: string
}

export interface DDNSConfig {
  enabled: boolean
  interval_seconds: number
  targets: DDNSRecordTarget[]
  last_ipv4?: string
  last_ipv6?: string
  last_run_at?: string
  last_error?: string
}

export interface DDNSDomainItem {
  id: number
  domain: string
  credential_name?: string
  credential_provider?: string
  config: DDNSConfig
}

export interface UpdateDDNSPayload {
  enabled: boolean
  interval_seconds: number
  record_ids: string[]
}

export interface DomainListParams {
  keyword?: string
  credential_id?: number
  page?: number
  per_page?: number
}

export interface RecordListParams {
  type?: string
  name?: string
  page?: number
  per_page?: number
}

export interface RecordPayload {
  type: string
  name: string
  content: string
  ttl: number
  priority?: number
  weight?: number
  proxied?: boolean
}

const baseDomainUrl = '/dns/domains'

const domainApi = useCurdApi<DNSDomain>(baseDomainUrl)

export const dnsApi = {
  ...domainApi,
  listRecords(domainId: number, params?: RecordListParams) {
    return http.get<{ data: DNSRecord[], pagination: Pagination }>(`${baseDomainUrl}/${domainId}/records`, { params })
  },
  createRecord(domainId: number, payload: RecordPayload) {
    return http.post<DNSRecord>(`${baseDomainUrl}/${domainId}/records`, payload)
  },
  updateRecord(domainId: number, recordId: string, payload: RecordPayload) {
    return http.put<DNSRecord>(`${baseDomainUrl}/${domainId}/records/${recordId}`, payload)
  },
  deleteRecord(domainId: number, recordId: string) {
    return http.delete(`${baseDomainUrl}/${domainId}/records/${recordId}`)
  },
  getDDNSConfig(domainId: number) {
    return http.get<DDNSConfig>(`${baseDomainUrl}/${domainId}/ddns`)
  },
  updateDDNSConfig(domainId: number, payload: UpdateDDNSPayload) {
    return http.put<DDNSConfig>(`${baseDomainUrl}/${domainId}/ddns`, payload)
  },
  listDDNS() {
    return http.get<{ data: DDNSDomainItem[] }>(`/dns/ddns`)
  },
}

export type { DnsCredential }
