import type { NgxDirective, NgxLocation, NgxServer } from '@/api/ngx'

// Matches a `return` statement that sends the client to HTTPS on the same host,
// e.g. `return 301 https://$host$request_uri;`. The leading group keeps the
// character in front of the statement so a replacement leaves it intact.
const httpsRedirectStatement = /(^|[;{}\s])return\s+(?:\d{3}\s+)?["']?https:\/\/\$host\b[^;]*;/g

// Directives that only make sense on the TLS listener and must not be copied
// into a plain HTTP server.
const tlsOnlyDirectives = new Set(['listen', 'server_name', 'http2', 'ssl'])

function cloneServer(server: NgxServer): NgxServer {
  return JSON.parse(JSON.stringify(server))
}

export function hasSSLListen(server?: NgxServer) {
  return server?.directives?.some(v => v.directive === 'listen' && v.params?.includes('ssl')) ?? false
}

export function hasDirectiveWithValue(server: NgxServer | undefined, directive: string) {
  return server?.directives?.some(v => v.directive === directive && v.params?.trim()) ?? false
}

/** A TLS server that already references a certificate and its key. */
export function isCompleteTLSServer(server?: NgxServer) {
  return hasSSLListen(server)
    && hasDirectiveWithValue(server, 'ssl_certificate')
    && hasDirectiveWithValue(server, 'ssl_certificate_key')
}

export function isHTTPChallengeLocation(location: NgxLocation) {
  return location.path.includes('/.well-known/acme-challenge')
}

function isRootLocation(location: NgxLocation) {
  return location.path.trim() === '/'
}

/** Reports whether the params of a `return` directive redirect to HTTPS on the same host. */
export function isHTTPSRedirectReturn(params: string) {
  return /^(?:\d{3}\s+)?["']?https:\/\/\$host\b/.test(params.trim())
}

/**
 * Removes every `return ... https://$host...;` statement from a location body.
 * Lines that held nothing but such a statement are dropped entirely.
 */
export function stripHTTPSRedirectReturns(content: string) {
  return content
    .split('\n')
    .flatMap(line => {
      const stripped = line.replace(httpsRedirectStatement, '$1')
      if (stripped === line)
        return [line]

      return stripped.trim() ? [stripped.trimEnd()] : []
    })
    .join('\n')
}

function hasEffectiveContent(content: string) {
  return content
    .split('\n')
    .some(line => {
      const trimmed = line.trim()
      return trimmed !== '' && !trimmed.startsWith('#')
    })
}

/**
 * Removes HTTPS self-redirects from a server in place: server-level
 * `return https://$host...` directives, the same statements inside location
 * bodies, and locations that are left empty afterwards.
 * Returns true when anything was removed.
 */
function removeHTTPSRedirects(server: NgxServer) {
  let removed = false

  if (server.directives) {
    const directives = server.directives.filter(v => !(v.directive === 'return' && isHTTPSRedirectReturn(v.params ?? '')))
    removed = directives.length !== server.directives.length
    server.directives = directives
  }

  if (server.locations) {
    server.locations = server.locations.flatMap(location => {
      const content = stripHTTPSRedirectReturns(location.content ?? '')
      if (content === location.content)
        return [location]

      removed = true
      return hasEffectiveContent(content) ? [{ ...location, content }] : []
    })
  }

  return removed
}

/**
 * Builds the 443 server for "Enable TLS" from a plain HTTP server. The copy
 * listens on 443 (IPv4 and IPv6) and carries no redirect to
 * `https://$host...`, otherwise HTTPS would redirect to itself forever.
 */
export function buildTLSServerFromHTTPServer(server: NgxServer): NgxServer {
  const tlsServer = cloneServer(server)

  removeHTTPSRedirects(tlsServer)

  tlsServer.directives = [
    { directive: 'listen', params: '443 ssl' },
    { directive: 'listen', params: '[::]:443 ssl' },
    ...(tlsServer.directives ?? []).filter(v => v.directive !== 'listen'),
  ]

  return tlsServer
}

function serverNamesKey(server: NgxServer) {
  return (server.directives ?? [])
    .filter(v => v.directive === 'server_name')
    .flatMap(v => v.params?.trim().split(/\s+/) ?? [])
    .filter(Boolean)
    .sort()
    .join(' ')
}

function isTLSOnlyDirective(directive: NgxDirective) {
  return tlsOnlyDirectives.has(directive.directive) || directive.directive.startsWith('ssl_')
}

/**
 * Makes a port-80 server that only redirects to HTTPS serve the application of
 * a TLS server that is still waiting for its certificate. The port-80 server
 * keeps its own directives and locations (notably the ACME challenge one) and
 * gains the TLS server's app directives and locations. Servers that do not
 * redirect are returned unchanged.
 */
export function serveTLSAppOverHTTP(httpServer: NgxServer, tlsServer: NgxServer): NgxServer {
  const server = cloneServer(httpServer)

  if (!removeHTTPSRedirects(server))
    return httpServer

  server.directives ??= []
  server.locations ??= []

  if (server.locations.some(isRootLocation))
    return server

  const directives = server.directives
  const locationPaths = new Set(server.locations.map(location => location.path.trim()))
  const source = cloneServer(tlsServer)

  source.directives?.forEach(directive => {
    if (isTLSOnlyDirective(directive))
      return
    if (directives.some(v => v.directive === directive.directive && v.params === directive.params))
      return

    directives.push({ directive: directive.directive, params: directive.params, comments: directive.comments })
  })

  source.locations?.forEach(location => {
    if (isHTTPChallengeLocation(location) || locationPaths.has(location.path.trim()))
      return

    server.locations!.push(location)
  })

  return server
}

/**
 * Returns the servers that can be written to disk before a certificate is
 * issued: TLS servers without a certificate are left out, and a port-80
 * server with the same server_name that would only redirect to the missing
 * HTTPS server serves the pending server's application instead.
 */
export function stageServersForPendingTLS(servers: NgxServer[]): NgxServer[] {
  const pending = servers.filter(server => hasSSLListen(server) && !isCompleteTLSServer(server))
  const completeNames = new Set(servers.filter(isCompleteTLSServer).map(serverNamesKey))

  return servers
    .filter(server => !pending.includes(server))
    .map(server => {
      if (hasSSLListen(server))
        return cloneServer(server)

      const names = serverNamesKey(server)
      const source = completeNames.has(names)
        ? undefined
        : pending.find(tlsServer => serverNamesKey(tlsServer) === names)

      return source ? serveTLSAppOverHTTP(cloneServer(server), source) : cloneServer(server)
    })
}
