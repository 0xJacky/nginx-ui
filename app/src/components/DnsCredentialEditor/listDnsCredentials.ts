import type { DnsCredentialSummary } from './dnsCredentialSummaries'
import dns_credential from '@/api/dns_credential'
import { collectDnsCredentialSummaries } from './dnsCredentialSummaries'

/**
 * Lists the stored DNS credentials of the selected node without their
 * configuration, so a certificate form can offer them for selection.
 *
 * Plugins reach it as `window.NginxUI.shared.ui.listDnsCredentials`, which
 * lets a DNS-01 form pick a credential without the `core_api` permission.
 */
export function listDnsCredentials(): Promise<DnsCredentialSummary[]> {
  return collectDnsCredentialSummaries(page => dns_credential.getList({ page }))
}
