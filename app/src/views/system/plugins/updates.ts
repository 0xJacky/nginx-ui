import type { PluginInfo } from '@/api/plugin'
import type { CatalogEntry, PluginUpdateInfo } from '@/api/plugin_marketplace'

/**
 * Builds a minimal catalog entry from an update notice when the full catalog
 * has not loaded yet, so the release-notes drawer can still open and fetch its
 * details from the source.
 */
export function fallbackCatalogEntry(plugin: PluginInfo, update: PluginUpdateInfo): CatalogEntry {
  return {
    id: plugin.id,
    name: plugin.name_i18n ?? { en: plugin.name },
    description: plugin.description_i18n ?? (plugin.description
      ? { en: plugin.description }
      : undefined),
    icon_url: plugin.icon_url,
    trust: update.trust,
    releases: update.release
      ? [update.release]
      : [],
    source: update.source,
    installable_release: update.release,
    installed_version: plugin.version,
    update_available: true,
    installed_yanked: update.installed_yanked,
  }
}

/**
 * Returns the single installed plugin with an available update when there is
 * exactly one, so the update banner can offer direct actions instead of a
 * redundant filter switch.
 */
export function findSingleUpdate(
  plugins: readonly PluginInfo[],
  updates: readonly PluginUpdateInfo[],
): { plugin: PluginInfo, update: PluginUpdateInfo } | undefined {
  const byId = new Map(updates.map(item => [item.id, item]))
  const matches: { plugin: PluginInfo, update: PluginUpdateInfo }[] = []
  for (const plugin of plugins) {
    const update = byId.get(plugin.id)
    if (update)
      matches.push({ plugin, update })
    if (matches.length > 1)
      return undefined
  }
  return matches.length === 1
    ? matches[0]
    : undefined
}

/**
 * Returns the catalog entry for the sole available update, preferring the
 * current marketplace list and falling back to the full cached catalog.
 */
export function findSingleUpdateEntry(
  updates: readonly PluginUpdateInfo[],
  entries: readonly CatalogEntry[],
  catalog: readonly CatalogEntry[],
): CatalogEntry | undefined {
  if (updates.length !== 1)
    return undefined
  const id = updates[0].id
  return entries.find(entry => entry.id === id && entry.update_available)
    ?? catalog.find(entry => entry.id === id && entry.update_available)
}
