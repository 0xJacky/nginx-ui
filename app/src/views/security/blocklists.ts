import type { BlocklistKind } from '@/api/blocklist'
import { listBlocklistKinds } from '@/api/blocklist'

/** The shortest refresh interval of a blocklist source, in seconds. */
export const minRefreshSeconds = 60

// Source kinds enabled security.blocklist plugins offer, with their forms.
export const blocklistKinds = ref<BlocklistKind[]>([])

let pendingKinds: Promise<void> | undefined

/** Loads the source kinds once; pass force to refresh them. */
export function loadBlocklistKinds(force = false): Promise<void> {
  if (pendingKinds && !force)
    return pendingKinds

  pendingKinds = listBlocklistKinds()
    .then(res => {
      blocklistKinds.value = res.data ?? []
    })
    .catch(error => {
      console.error('Failed to load blocklist source kinds:', error)
      pendingKinds = undefined
    })
  return pendingKinds
}

export function findBlocklistKind(kind?: string): BlocklistKind | undefined {
  if (!kind)
    return undefined
  return blocklistKinds.value.find(item => item.kind === kind)
}

/** Display name of a kind, falling back to the stored value. */
export function blocklistKindLabel(kind?: string): string {
  return findBlocklistKind(kind)?.name ?? kind ?? ''
}

/** Drops the values the kind form does not declare. */
export function sanitizeBlocklistConfig(kind: string | undefined, config: Record<string, string> | undefined): Record<string, string> {
  const found = findBlocklistKind(kind)
  if (!found)
    return { ...config }
  const allowed = new Set(found.fields.map(field => field.key))
  return Object.fromEntries(Object.entries(config ?? {}).filter(([key]) => allowed.has(key)))
}
