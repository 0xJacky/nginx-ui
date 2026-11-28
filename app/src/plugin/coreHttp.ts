import type { http } from '@uozi-admin/request'
import type { DnsCredentialSummary } from '@/components/DnsCredentialEditor/dnsCredentialSummaries'

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

export interface CoreHttpOptions {
  /** Defaults to `ENFORCE_CORE_API`. */
  enforce?: boolean
  /**
   * Answers `get('/dns_credentials')` of a plugin without `core_api` once the
   * check is enforced, see `dnsCredentialsShim`.
   */
  listDnsCredentials?: () => Promise<DnsCredentialSummary[]>
}

/** Whether `url` is the DNS credential list, with or without a query. */
function isDnsCredentialList(url: string): boolean {
  return url.split('?')[0].replace(/^\.?\/+/, '').replace(/\/+$/, '') === 'dns_credentials'
}

/**
 * Lets DNS-01 releases from before `listDnsCredentials` keep their credential
 * picker on a host that enforces `core_api`: such a release pages
 * `GET /dns_credentials` through `registry.coreHttp`. The list is answered
 * from `listDnsCredentials`, so it carries no secrets, and without pagination,
 * so the plugin stops after the first page. Every other call still rejects.
 */
function dnsCredentialsShim(
  denied: CoreHttp,
  listDnsCredentials: () => Promise<DnsCredentialSummary[]>,
): CoreHttp {
  return {
    ...denied,
    get: ((url: string, config?: Parameters<CoreHttp['get']>[1]) => isDnsCredentialList(url)
      ? listDnsCredentials().then(data => ({ data }))
      : denied.get(url, config)) as CoreHttp['get'],
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
  options: CoreHttpOptions = {},
): CoreHttp {
  if (permissions?.includes(CORE_API_PERMISSION))
    return client

  const enforce = options.enforce ?? ENFORCE_CORE_API
  if (!enforce) {
    return guardedCoreHttp(
      client,
      `[plugin] ${pluginId}: registry.coreHttp without the ${CORE_API_PERMISSION} permission is deprecated, `
      + `a later Nginx UI release rejects these calls`,
      false,
    )
  }

  const denied = guardedCoreHttp(
    client,
    `[plugin] ${pluginId}: registry.coreHttp needs the ${CORE_API_PERMISSION} permission`,
    true,
  )

  return options.listDnsCredentials
    ? dnsCredentialsShim(denied, options.listDnsCredentials)
    : denied
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
