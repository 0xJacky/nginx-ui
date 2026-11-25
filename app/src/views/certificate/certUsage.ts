import type { CertificateUsage } from '@/api/cert'
import type { ConfigUsageItem } from '@/components/ConfigUsage'
import ConfigUsage from '@/components/ConfigUsage'

export function toConfigUsages(usedBy: CertificateUsage[] = []): ConfigUsageItem[] {
  return usedBy.map(usage => ({
    key: `${usage.kind}:${usage.name}`,
    label: usage.name,
    kind: usage.kind,
    to: `/${usage.kind}s/${encodeURIComponent(usage.name)}`,
    status: usage.status,
  }))
}

// The table cell that lists the sites and streams loading a certificate.
export function renderCertUsage(usedBy?: CertificateUsage[]) {
  const usages = toConfigUsages(usedBy)
  return h(ConfigUsage, {
    usages,
    summary: $ngettext('Used by %{count} configuration', 'Used by %{count} configurations', usages.length, {
      count: String(usages.length),
    }),
  })
}
