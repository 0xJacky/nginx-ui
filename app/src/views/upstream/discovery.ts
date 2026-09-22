import type { DiscoveryProvider } from '@/api/upstream_discovery'
import { listDiscoveryProviders } from '@/api/upstream_discovery'

/** The default refresh interval of a binding, in seconds. */
export const defaultRefreshSeconds = 60

/** The shortest refresh interval of a binding, in seconds. */
export const minRefreshSeconds = 10

// Providers enabled upstream.discovery plugins offer, with their forms.
export const discoveryProviders = ref<DiscoveryProvider[]>([])

let pendingProviders: Promise<void> | undefined

/** Loads the providers once; pass force to refresh them. */
export function loadDiscoveryProviders(force = false): Promise<void> {
  if (pendingProviders && !force)
    return pendingProviders

  pendingProviders = listDiscoveryProviders()
    .then(res => {
      discoveryProviders.value = res.data ?? []
    })
    .catch(error => {
      console.error('Failed to load discovery providers:', error)
      pendingProviders = undefined
    })
  return pendingProviders
}

export function findDiscoveryProvider(kind?: string): DiscoveryProvider | undefined {
  if (!kind)
    return undefined
  return discoveryProviders.value.find(item => item.kind === kind)
}

/** Display name of a provider, falling back to the stored value. */
export function discoveryProviderLabel(kind?: string): string {
  return findDiscoveryProvider(kind)?.name ?? kind ?? ''
}

/** Drops the values the provider form does not declare. */
export function sanitizeDiscoveryConfig(kind: string | undefined, config: Record<string, string> | undefined): Record<string, string> {
  const found = findDiscoveryProvider(kind)
  if (!found)
    return { ...config }
  const allowed = new Set(found.fields.map(field => field.key))
  return Object.fromEntries(Object.entries(config ?? {}).filter(([key]) => allowed.has(key)))
}
