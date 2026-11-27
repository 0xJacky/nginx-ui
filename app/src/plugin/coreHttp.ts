import type { http } from '@uozi-admin/request'

/** Permission a plugin needs before `registry.coreHttp` reaches the host API. */
export const CORE_API_PERMISSION = 'core_api'

type CoreHttp = typeof http

/**
 * Client handed to a plugin without `core_api`: every call rejects. The
 * warning is logged too because a bundle may swallow the rejection.
 */
function deniedCoreHttp(pluginId: string): CoreHttp {
  let warned = false

  function deny(): Promise<never> {
    const message = `[plugin] ${pluginId}: registry.coreHttp needs the ${CORE_API_PERMISSION} permission`
    if (!warned) {
      warned = true
      console.warn(message)
    }
    return Promise.reject(new Error(message))
  }

  return {
    get: deny,
    post: deny,
    put: deny,
    delete: deny,
    patch: deny,
  }
}

/**
 * The host API client for a plugin whose approved permissions are
 * `permissions`.
 *
 * Bundles run in the host page, so this keeps honest plugins to what they
 * declared; it is not a security boundary against a malicious one.
 */
export function coreHttpFor(pluginId: string, permissions: readonly string[] | undefined, client: CoreHttp): CoreHttp {
  return permissions?.includes(CORE_API_PERMISSION)
    ? client
    : deniedCoreHttp(pluginId)
}
