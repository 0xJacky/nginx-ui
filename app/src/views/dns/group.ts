import type { DNSDomain, DNSRecord, RecordPayload } from '@/api/dns'
import { dnsApi } from '@/api/dns'

const pageSize = 200

export async function listAllDNSDomains() {
  const domains: DNSDomain[] = []
  let page = 1
  let hasMorePages = true

  while (hasMorePages) {
    const response = await dnsApi.getList({ page, per_page: pageSize })
    const pageDomains = response.data ?? []
    domains.push(...pageDomains)
    hasMorePages = response.pagination
      ? page < response.pagination.total_pages
      : pageDomains.length === pageSize
    page++
  }

  return domains
}

export async function listAllDNSRecords(domainId: number) {
  const records: DNSRecord[] = []
  let page = 1
  let hasMorePages = true

  while (hasMorePages) {
    const response = await dnsApi.listRecords(domainId, { page, per_page: pageSize })
    const pageRecords = response.data ?? []
    records.push(...pageRecords)
    hasMorePages = response.pagination
      ? page < response.pagination.total_pages
      : pageRecords.length === pageSize
    page++
  }

  return records
}

export function getRecordKey(record: Pick<DNSRecord, 'name' | 'type'>) {
  return `${record.name.trim().toLowerCase()}\u0000${record.type.trim().toUpperCase()}`
}

export function getCommonRecordPayload(record: DNSRecord): RecordPayload {
  return {
    type: record.type.toUpperCase(),
    name: record.name,
    content: record.content,
    ttl: record.ttl,
    priority: record.priority,
    weight: record.weight,
  }
}

export function mergeRecordPayload(record: DNSRecord, commonPayload: RecordPayload): RecordPayload {
  return {
    ...commonPayload,
    line: record.line,
    proxied: record.proxied,
    comment: record.comment,
  }
}

export function hasSameCommonRecordValues(record: DNSRecord, payload: RecordPayload) {
  return record.type.toUpperCase() === payload.type.toUpperCase()
    && record.name.trim().toLowerCase() === payload.name.trim().toLowerCase()
    && record.content === payload.content
    && record.ttl === payload.ttl
    && record.priority === payload.priority
    && record.weight === payload.weight
}

export function hasSameRecordValues(record: DNSRecord, payload: RecordPayload) {
  return hasSameCommonRecordValues(record, payload)
    && (payload.line === undefined || record.line === payload.line)
    && (payload.proxied === undefined || record.proxied === payload.proxied)
    && (payload.comment === undefined || (record.comment ?? '') === payload.comment)
}

export function getErrorMessage(error: unknown) {
  if (typeof error === 'object' && error !== null && 'message' in error && typeof error.message === 'string')
    return error.message
  if (typeof error === 'string')
    return error
  return $gettext('Unknown error')
}
