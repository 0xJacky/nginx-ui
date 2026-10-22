/**
 * Route removers per plugin id. `router.addRoute` hands back a function that
 * takes the record out again, which is how a disabled plugin loses its pages.
 */
const removers = new Map<string, Array<() => void>>()

export function trackPluginRoute(pluginId: string, remove: () => void) {
  const list = removers.get(pluginId) ?? []
  list.push(remove)
  removers.set(pluginId, list)
}

/** Removes every route the plugin added and forgets the removers. */
export function removePluginRoutes(pluginId: string) {
  const list = removers.get(pluginId) ?? []
  removers.delete(pluginId)

  for (const remove of list)
    remove()
}
