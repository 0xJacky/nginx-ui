import type { CatalogEntry, CatalogSource } from '@/api/plugin_marketplace'
import { catalogSourceName, getMarketplaceSources, officialIdPrefix } from '@/api/plugin_marketplace'
import gettext from '@/gettext'

// The configured sources, shared by the pages that name a source.
const known = ref<CatalogSource[]>([])
// The official catalog URL, empty when this node has none.
const official = ref<string>()
let pending: Promise<void> | undefined

/** Keeps the sources a response listed, so every view names them alike. */
export function rememberSources(sources: CatalogSource[], officialUrl?: string) {
  known.value = sources
  if (officialUrl !== undefined)
    official.value = officialUrl
}

/**
 * Whether an official plugin is offered by another source, a mirror placed
 * above the official catalog.
 */
export function useOfficialElsewhere() {
  ensureSources()
  return (entry: Pick<CatalogEntry, 'id' | 'source'>) => Boolean(official.value
    && entry.source
    && entry.source !== official.value
    && entry.id.startsWith(officialIdPrefix))
}

/** Name of a source URL, read from the configured sources once if needed. */
export function useSourceName() {
  ensureSources()
  return (url: string | undefined) => {
    if (!url)
      return ''
    return catalogSourceName(sourceOf(url), url, gettext.current)
  }
}

/** Icon a source URL declares, empty when it has none. */
export function useSourceIcon() {
  ensureSources()
  return (url: string | undefined) => (url ? sourceOf(url)?.catalog_icon ?? '' : '')
}

function sourceOf(url: string) {
  return known.value.find(item => item.url === url)
}

function ensureSources() {
  if ((known.value.length === 0 || official.value === undefined) && !pending) {
    pending = getMarketplaceSources()
      .then(response => {
        if (known.value.length === 0)
          known.value = response.sources
        official.value = response.default
      })
      .catch(() => {})
      .finally(() => {
        pending = undefined
      })
  }
}
