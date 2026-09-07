import type { DNSRecord } from '@/api/dns'

function normalizeDnsName(name: string): string {
  return name.trim().replace(/\.+$/, '').toLowerCase()
}

function getRecordDnsName(recordName: string, domain: string): string {
  const normalizedDomain = normalizeDnsName(domain)
  const normalizedRecord = normalizeDnsName(recordName)

  if (normalizedRecord === '@' || normalizedRecord === normalizedDomain)
    return normalizedDomain

  if (normalizedRecord.endsWith(`.${normalizedDomain}`))
    return normalizedRecord

  return `${normalizedRecord}.${normalizedDomain}`
}

/** Return all record IDs that exactly serve the first configured server name. */
export function findMatchingDNSRecordIds(
  serverName: string,
  domain: string,
  records: DNSRecord[],
): string[] {
  const targetName = normalizeDnsName(serverName.split(/\s+/)[0] ?? '')
  if (!targetName)
    return []

  return records
    .filter(record => getRecordDnsName(record.name, domain) === targetName)
    .map(record => record.id)
}
