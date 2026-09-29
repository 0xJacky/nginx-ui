import type { NginxUIPlugin } from './types'

/** The part of a webapp entry that identifies one build of a bundle. */
export interface BundleRef {
  id: string
  version: string
}

/**
 * Evaluates the bundle of `entry` and returns the definition it registered.
 * A bundle cannot run twice on one page, so definitions are remembered here.
 */
export type BundleFetcher<T extends BundleRef> = (entry: T) => Promise<NginxUIPlugin | undefined>

export interface BundleSource<T extends BundleRef> {
  /** Reuses the remembered definition of the same version, else fetches the bundle. */
  acquire: (entry: T) => Promise<NginxUIPlugin | undefined>
  /** Drops the remembered definition of a plugin. */
  forget: (pluginId: string) => void
  /** The remembered definition of a plugin, if any. */
  peek: (pluginId: string) => NginxUIPlugin | undefined
}

export function createBundleSource<T extends BundleRef>(fetchBundle: BundleFetcher<T>): BundleSource<T> {
  const remembered = new Map<string, { version: string, definition: NginxUIPlugin }>()

  async function acquire(entry: T) {
    const known = remembered.get(entry.id)
    if (known && known.version === entry.version)
      return known.definition

    const definition = await fetchBundle(entry)
    if (definition)
      remembered.set(entry.id, { version: entry.version, definition })
    else
      remembered.delete(entry.id)

    return definition
  }

  function forget(pluginId: string) {
    remembered.delete(pluginId)
  }

  function peek(pluginId: string) {
    return remembered.get(pluginId)?.definition
  }

  return { acquire, forget, peek }
}
