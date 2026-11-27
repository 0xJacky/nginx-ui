/**
 * What a plugin may know about a stored DNS credential: enough to list it and
 * to reference it, never its configuration or secrets.
 */
export interface DnsCredentialSummary {
  id: number
  name: string
  code: string
  provider?: string
  provider_code?: string
}

/**
 * One page of the DNS credential list, as the REST API returns it. Rows carry
 * more fields, the configuration among them, which are dropped.
 */
export interface DnsCredentialPage {
  data?: DnsCredentialSummary[]
  pagination?: { per_page?: number }
}

function toSummary(credential: DnsCredentialSummary): DnsCredentialSummary {
  return {
    id: credential.id,
    name: credential.name,
    code: credential.code,
    provider: credential.provider,
    provider_code: credential.provider_code,
  }
}

/**
 * Reads every page of the DNS credential list and keeps only the summary
 * fields. A failing page rejects instead of returning a partial list.
 */
export async function collectDnsCredentialSummaries(
  fetchPage: (page: number) => Promise<DnsCredentialPage>,
): Promise<DnsCredentialSummary[]> {
  const summaries: DnsCredentialSummary[] = []

  for (let page = 1; ; page++) {
    const response = await fetchPage(page)
    const rows = response?.data ?? []
    summaries.push(...rows.map(toSummary))

    const perPage = response?.pagination?.per_page ?? 0
    if (!perPage || rows.length < perPage)
      return summaries
  }
}
