/**
 * Encode a filesystem path so a WAF does not read it as a traversal attempt.
 *
 * Managed WAF rulesets block requests whose query string looks like a path:
 * `?log_path=/var/log/nginx/access.log` is refused at the edge before the
 * application sees it. Percent-encoding does not help, because WAFs normalise
 * it before matching.
 *
 * base64url output uses only [A-Za-z0-9_-], so nothing path-shaped survives
 * into the URL. The `b64_` prefix is what lets the server tell an encoded value
 * from a raw one; the Go side is internal/helper/encoded_path.go and the two
 * implementations must stay in agreement.
 */

export const ENCODED_PATH_PREFIX = 'b64_'

export function encodePathParam(path: string): string {
  // btoa operates on binary strings, so encode to UTF-8 bytes first — a path
  // containing non-ASCII characters would otherwise throw.
  const bytes = new TextEncoder().encode(path)
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }

  const base64 = btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')

  return ENCODED_PATH_PREFIX + base64
}

/**
 * Query parameters that carry a filesystem path, keyed by request path.
 *
 * An allowlist rather than "any parameter named path": GET /settings/protected
 * also takes `path`, but its value is a settings key such as `app.jwt_secret`,
 * which must reach the server unchanged.
 */
export const PATH_PARAMS_BY_URL: Record<string, string[]> = {
  '/config': ['path'],
  '/configs': ['dir'],
  '/config_histories': ['filepath'],
  '/llm_messages': ['path'],
  '/llm_sessions': ['path'],
  '/nginx_logs': ['path'],
  '/nginx_log/preflight': ['log_path'],
}

/** Normalize a request URL to the key shape used by PATH_PARAMS_BY_URL. */
export function normalizeRequestUrl(url: string | undefined): string {
  if (!url) {
    return ''
  }
  const withoutQuery = url.split('?')[0]
  const trimmed = withoutQuery.replace(/\/+$/, '')
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}
