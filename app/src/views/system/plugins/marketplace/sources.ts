import type { CatalogSource } from '@/api/plugin_marketplace'
import { catalogSourceName, getMarketplaceSources } from '@/api/plugin_marketplace'
import gettext from '@/gettext'

// The configured sources, shared by the pages that name a source.
const known = ref<CatalogSource[]>([])
let pending: Promise<void> | undefined

/** Keeps the sources a response listed, so every view names them alike. */
export function rememberSources(sources: CatalogSource[]) {
  known.value = sources
}

/** Name of a source URL, read from the configured sources once if needed. */
export function useSourceName() {
  if (known.value.length === 0 && !pending) {
    pending = getMarketplaceSources()
      .then(response => {
        if (known.value.length === 0)
          known.value = response.sources
      })
      .catch(() => {})
      .finally(() => {
        pending = undefined
      })
  }

  return (url: string | undefined) => {
    if (!url)
      return ''
    return catalogSourceName(known.value.find(item => item.url === url), url, gettext.current)
  }
}
