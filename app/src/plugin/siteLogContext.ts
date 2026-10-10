import type { SlotContext } from './types'

/** Context the host passes to `site.log.actions`. */
export interface SiteLogContext extends SlotContext {
  accessLogPath: string
  accessLogInherited: boolean
  errorLogPath: string
  errorLogInherited: boolean
  siteName: string
}

/** What a site list row or the editor knows about the logs of a site. */
export interface SiteLogSource {
  siteName?: string
  accessLogPath?: string
  accessLogInherited?: boolean
  errorLogPath?: string
  errorLogInherited?: boolean
}

/**
 * Normalizes the log fields of a site into the slot context: a missing path is
 * the empty string, and a flag is only ever true next to a path.
 */
export function siteLogContext(source: SiteLogSource): SiteLogContext {
  const accessLogPath = source.accessLogPath ?? ''
  const errorLogPath = source.errorLogPath ?? ''

  return {
    accessLogPath,
    accessLogInherited: accessLogPath !== '' && source.accessLogInherited === true,
    errorLogPath,
    errorLogInherited: errorLogPath !== '' && source.errorLogInherited === true,
    siteName: source.siteName ?? '',
  }
}

/** The nginx default logs a site without a directive of its own falls back to. */
export interface DefaultSiteLogs {
  access?: string
  error?: string
}

/**
 * Context of a site being edited: the paths of its own directives, and for each
 * kind without one the default log when `inherits` is set.
 */
export function editorSiteLogContext(
  siteName: string | undefined,
  own: { access?: string, error?: string },
  defaults: DefaultSiteLogs,
  inherits: boolean,
): SiteLogContext {
  const access = own.access ?? ''
  const error = own.error ?? ''
  const inheritedAccess = inherits && !access ? defaults.access ?? '' : ''
  const inheritedError = inherits && !error ? defaults.error ?? '' : ''

  return siteLogContext({
    siteName,
    accessLogPath: access || inheritedAccess,
    accessLogInherited: inheritedAccess !== '',
    errorLogPath: error || inheritedError,
    errorLogInherited: inheritedError !== '',
  })
}
