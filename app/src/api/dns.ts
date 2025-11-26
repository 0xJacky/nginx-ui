import type { ModelBase, Pagination } from '@/api/curd'
import type { DnsCredential } from '@/api/dns_credential'
import { http, useCurdApi } from '@uozi-admin/request'

export interface DNSDomain extends ModelBase {
  domain: string
  description?: string
  dns_credential_id: number
  credential?: {
    id: number
    name: string
    provider: string
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

const baseUrl = '/dns/domains'

const domainApi = useCurdApi<DNSDomain>(baseUrl)

export const dnsApi = {
  ...domainApi,
  listRecords(domainId: number, params?: RecordListParams) {
    return http.get<{ data: DNSRecord[], pagination: Pagination }>(`${baseUrl}/${domainId}/records`, { params })
  },
  createRecord(domainId: number, payload: RecordPayload) {
    return http.post<DNSRecord>(`${baseUrl}/${domainId}/records`, payload)
  },
  updateRecord(domainId: number, recordId: string, payload: RecordPayload) {
    return http.put<DNSRecord>(`${baseUrl}/${domainId}/records/${recordId}`, payload)
  },
  deleteRecord(domainId: number, recordId: string) {
    return http.delete(`${baseUrl}/${domainId}/records/${recordId}`)
  },
}

export type { DnsCredential }
