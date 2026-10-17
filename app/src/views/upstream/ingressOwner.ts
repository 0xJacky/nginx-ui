import type { ManagedUpstreamDetail } from '@/api/upstream'
import { managedIngressName } from '@/views/site/managedSite'

// The upstream API refuses an unforced change to a group an ingress
// generates with this code; its params are [upstream, ingress].
const managedByIngressCode = 40908

// The name of the ingress that generates the upstream, or '' for a group the
// operator owns.
export function upstreamIngressName(record: Pick<ManagedUpstreamDetail, 'managed_by'>): string {
  return managedIngressName(record.managed_by)
}

// The ingress named by a refused change to an upstream it generates, or ''
// for any other error. Lets the page lock a group whose owner it learnt of
// only from the refusal, for example one claimed after the list was loaded.
export function refusingIngress(error: unknown): string {
  if (!error || typeof error !== 'object')
    return ''
  const { scope, code, params } = error as { scope?: unknown, code?: unknown, params?: unknown }
  if (scope !== 'upstream' || Number(code) !== managedByIngressCode || !Array.isArray(params))
    return ''
  return String(params[1] ?? '')
}
