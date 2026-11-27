import type { http } from '@uozi-admin/request'

/** Permission a plugin needs before `registry.coreHttp` reaches the host API. */
export const CORE_API_PERMISSION = 'core_api'

/**
 * Whether a plugin without `core_api` is refused the host API.
 *
 * Off during the deprecation period: such a plugin still gets the real client
 * and a warning, so a plugin released before the check, the official DNS-01
 * among them, keeps working until its update is out. Turning it on also has to
 * drop `http` from `window.NginxUI.shared`, see `shared.ts`.
 */
export const ENFORCE_CORE_API = false

type CoreHttp = typeof http

/**
 * Wraps every method of `client` so that the first call logs `message`. When
 * `reject` is set, calls reject with that message instead of reaching the
 * client; the warning is logged too because a bundle may swallow the
 * rejection.
 */
function guardedCoreHttp(client: CoreHttp, message: string, reject: boolean): CoreHttp {
  let warned = false

  function guard<Method extends keyof CoreHttp>(method: Method): CoreHttp[Method] {
    return ((...args: Parameters<CoreHttp[Method]>) => {
      if (!warned) {
        warned = true
        console.warn(message)
      }
      if (reject)
        return Promise.reject(new Error(message))
      return (client[method] as (...callArgs: unknown[]) => Promise<unknown>)(...args)
    }) as CoreHttp[Method]
  }

  return {
    get: guard('get'),
    post: guard('post'),
    put: guard('put'),
    delete: guard('delete'),
    patch: guard('patch'),
  }
}

/**
 * The host API client for a plugin whose approved permissions are
 * `permissions`.
 *
 * Bundles run in the host page, so this keeps honest plugins to what they
 * declared; it is not a security boundary against a malicious one.
 */
export function coreHttpFor(
  pluginId: string,
  permissions: readonly string[] | undefined,
  client: CoreHttp,
  enforce = ENFORCE_CORE_API,
): CoreHttp {
  if (permissions?.includes(CORE_API_PERMISSION))
    return client

  const message = enforce
    ? `[plugin] ${pluginId}: registry.coreHttp needs the ${CORE_API_PERMISSION} permission`
    : `[plugin] ${pluginId}: registry.coreHttp without the ${CORE_API_PERMISSION} permission is deprecated, `
      + `a later Nginx UI release rejects these calls`

  return guardedCoreHttp(client, message, enforce)
}

/**
 * The host API client as `window.NginxUI.shared.http`, kept for the
 * deprecation period. The first call logs a warning; it never rejects.
 */
export function deprecatedSharedHttp(client: CoreHttp): CoreHttp {
  return guardedCoreHttp(
    client,
    `[plugin] window.NginxUI.shared.http is deprecated and a later Nginx UI release removes it, `
    + `declare the ${CORE_API_PERMISSION} permission and use registry.coreHttp`,
    false,
  )
}
