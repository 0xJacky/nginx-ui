const LOOPBACK_HOSTNAMES = new Set(['localhost', '127.0.0.1', '[::1]'])

/**
 * Whether `value` is an absolute http(s) URL on a loopback host. The dev
 * plugin URL is limited to these, so a pasted remote address never turns into
 * a script the browser loads on every visit.
 */
export function isLoopbackUrl(value: string): boolean {
  let url: URL
  try {
    url = new URL(value)
  }
  catch {
    return false
  }

  if (url.protocol !== 'http:' && url.protocol !== 'https:')
    return false

  // URL lowercases the host and keeps an IPv6 address in brackets.
  return LOOPBACK_HOSTNAMES.has(url.hostname) || url.hostname.endsWith('.localhost')
}
