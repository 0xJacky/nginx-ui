import type { Ingress, IngressCondition, IngressPhase, IngressServerStatus } from '@/api/ingress'

// Pure helpers for the ingress page. They take the API objects as they come
// and never translate, so they can be unit tested without the app.

export type PhaseColor = 'success' | 'warning' | 'error' | 'processing' | 'default'

const phaseColors: Record<IngressPhase, PhaseColor> = {
  Ready: 'success',
  Degraded: 'warning',
  Error: 'error',
  Applying: 'processing',
  Pending: 'default',
}

export function phaseColor(phase?: string): PhaseColor {
  return phaseColors[phase as IngressPhase] ?? 'default'
}

// The spec changed after the last apply that finished.
export function isGenerationBehind(ingress: Ingress): boolean {
  return (ingress.status?.observed_generation ?? 0) < ingress.generation
}

export function failingConditions(ingress: Ingress): IngressCondition[] {
  return (ingress.status?.conditions ?? []).filter(condition => !condition.status)
}

export interface BackendHealthSummary {
  // Servers that take traffic and passed the last check.
  healthy: number
  total: number
  // Servers drained with `down`.
  down: number
  // False until the availability checker has reported on any server.
  known: boolean
}

export function backendHealth(ingress: Ingress): BackendHealthSummary {
  const statuses = ingress.status?.servers ?? []
  if (statuses.length > 0) {
    return {
      healthy: statuses.filter(server => server.healthy && !server.down).length,
      total: statuses.length,
      down: statuses.filter(server => server.down).length,
      known: true,
    }
  }

  const servers = (ingress.spec?.backends ?? []).flatMap(backend => backend.servers ?? [])
  return {
    healthy: 0,
    total: servers.length,
    down: servers.filter(server => server.down).length,
    known: false,
  }
}

export function backendHealthColor(summary: BackendHealthSummary): 'success' | 'warning' | 'error' | 'default' {
  const serving = summary.total - summary.down
  if (!summary.known || serving <= 0)
    return 'default'
  if (summary.healthy === 0)
    return 'error'
  return summary.healthy < serving ? 'warning' : 'success'
}

export function serverStatus(ingress: Ingress, backend: string, address: string): IngressServerStatus | undefined {
  return (ingress.status?.servers ?? []).find(server => server.backend === backend && server.address === address)
}

export type CertState = 'none' | 'pending' | 'valid' | 'expiring' | 'expired'

export interface CertExpiry {
  state: CertState
  notAfter?: string
  daysLeft?: number
}

const dayMs = 24 * 60 * 60 * 1000

// Certificates closer than this to their expiry are flagged.
export const certExpiringDays = 14

export function certExpiry(ingress: Ingress, now: number = Date.now()): CertExpiry {
  if ((ingress.spec?.tls?.mode ?? 'none') === 'none')
    return { state: 'none' }

  const notAfter = ingress.status?.cert_not_after
  if (!notAfter)
    return { state: 'pending' }

  const daysLeft = Math.floor((new Date(notAfter).getTime() - now) / dayMs)
  if (Number.isNaN(daysLeft))
    return { state: 'pending' }

  let state: CertState = 'valid'
  if (daysLeft < 0)
    state = 'expired'
  else if (daysLeft < certExpiringDays)
    state = 'expiring'

  return { state, notAfter, daysLeft }
}

// The URL a host is served on: HTTPS once the certificate is in place.
export function hostUrl(ingress: Ingress, host: string): string {
  const isHttps = (ingress.spec?.tls?.mode ?? 'none') !== 'none' && !!ingress.status?.cert_not_after
  return `${isHttps ? 'https' : 'http'}://${host.replace(/^\*\./, '')}`
}

export function ownersOf(ingresses: Ingress[]): string[] {
  return [...new Set(ingresses.map(ingress => ingress.owner).filter(Boolean))].sort()
}

export function filterIngresses(ingresses: Ingress[], owner: string, keyword: string): Ingress[] {
  const needle = keyword.trim().toLowerCase()
  return ingresses.filter(ingress => {
    if (owner && ingress.owner !== owner)
      return false
    if (!needle)
      return true
    return ingress.name.toLowerCase().includes(needle)
      || (ingress.external_id ?? '').toLowerCase().includes(needle)
      || (ingress.spec?.hosts ?? []).some(host => host.toLowerCase().includes(needle))
  })
}
