import type { NgxConfig, NgxDirective, NgxLocation, NgxServer, NgxUpstream } from '@/api/ngx'
import { hasSSLListen, isCompleteTLSServer } from '../../composables/useHTTPSRedirect'

// Pure helpers that decide where the HTTPS onboarding card shows and whether
// the configuration the backend is about to rewrite matches what is on screen.
// No Vue reactivity so they can be unit-tested in isolation.

type ServersOf = Pick<NgxConfig, 'servers'> | undefined | null

function trimmed(value?: string | null) {
  return (value ?? '').trim()
}

/** A TLS server that still waits for its certificate (no non-empty ssl_certificate/_key). */
export function isPendingTLSServer(server?: NgxServer | null): boolean {
  return !!server && hasSSLListen(server) && !isCompleteTLSServer(server)
}

export function hasTLSServer(config: ServersOf): boolean {
  return config?.servers?.some(server => hasSSLListen(server)) ?? false
}

export function hasPendingTLSServer(config: ServersOf): boolean {
  return config?.servers?.some(isPendingTLSServer) ?? false
}

function normalizeServerName(value: string): string {
  let name = value.trim().replace(/;$/, '').replace(/\.$/, '').toLowerCase()
  // `.example.com` is nginx shorthand for example.com and *.example.com.
  if (name.startsWith('.'))
    name = name.slice(1)
  return name
}

// server_name values that can never be a certificate identifier: the catch-all
// `_`, regular expressions, suffix wildcards (`www.*`), variables and local names.
function isCertificateCandidate(name: string): boolean {
  if (!name || name === '_' || name === 'localhost')
    return false
  if (name.startsWith('~') || name.includes('$'))
    return false
  if (name.endsWith('.*') || name.slice(1).includes('*'))
    return false
  return true
}

/** Certificate identifier candidates from the server_name directives of the given servers. */
export function extractServerDomains(servers: (NgxServer | undefined | null)[]): string[] {
  const names = servers.flatMap(server => (server?.directives ?? [])
    .filter(directive => directive.directive === 'server_name')
    .flatMap(directive => directive.params?.split(/\s+/) ?? []))

  return [...new Set(names.map(normalizeServerName).filter(isCertificateCandidate))]
}

/**
 * Domains for a whole site: the names of its pending TLS servers when it has
 * any (they are what the certificate is for), otherwise every server's names.
 */
export function extractSiteDomains(config: ServersOf): string[] {
  const servers = config?.servers ?? []
  const pending = servers.filter(isPendingTLSServer)
  const domains = pending.length ? extractServerDomains(pending) : []

  return domains.length ? domains : extractServerDomains(servers)
}

function isCertificateDirective(directive: NgxDirective) {
  return directive.directive === 'ssl_certificate' || directive.directive === 'ssl_certificate_key'
}

/**
 * Regenerating a site that already serves HTTPS (e.g. quick setup on the edit
 * page) yields a TLS server with empty certificate placeholders. This copies
 * the certificate directives of the previous config's first working TLS server
 * into every pending TLS server of `next`, in place of the placeholders, so the
 * site keeps its certificate. Mutates `next`; returns whether it changed.
 */
export function carryOverCertificate(next: ServersOf, previous: ServersOf): boolean {
  const source = previous?.servers?.find(isCompleteTLSServer)
  const certificate = (source?.directives ?? [])
    .filter(directive => isCertificateDirective(directive) && trimmed(directive.params) !== '')
    .map(({ directive, params }) => ({ directive, params }))

  const targets = next?.servers?.filter(isPendingTLSServer) ?? []
  if (!certificate.length || !targets.length)
    return false

  targets.forEach(server => {
    const directives = server.directives ?? []
    const at = directives.findIndex(isCertificateDirective)
    const kept = directives.filter(directive => !isCertificateDirective(directive))
    const copies = certificate.map(directive => ({ ...directive }))
    const insertAt = at === -1 ? kept.length : at

    server.directives = [...kept.slice(0, insertAt), ...copies, ...kept.slice(insertAt)]
  })

  return true
}

/** Order-sensitive string list equality, used to keep computed domain lists stable. */
export function sameStringList(a: readonly string[] | undefined, b: readonly string[] | undefined): boolean {
  if (!a || !b || a.length !== b.length)
    return false
  return a.every((value, index) => value === b[index])
}

// BuildConfig drops directives without params (the quick setup's placeholder
// ssl_certificate/_key), so they never reach the file and must not count as a change.
function normalizeDirectives(directives?: NgxDirective[]) {
  return (directives ?? [])
    .filter(directive => trimmed(directive.params) !== '')
    .map(directive => [trimmed(directive.directive), trimmed(directive.params), trimmed(directive.comments)])
}

function normalizeLocations(locations?: NgxLocation[]) {
  return (locations ?? []).map(location => [trimmed(location.path), trimmed(location.content), trimmed(location.comments)])
}

function normalizeServer(server: NgxServer) {
  return {
    d: normalizeDirectives(server.directives),
    l: normalizeLocations(server.locations),
    c: trimmed(server.comments),
  }
}

function normalizeUpstream(upstream: NgxUpstream) {
  return {
    n: trimmed(upstream.name),
    d: normalizeDirectives(upstream.directives),
    c: trimmed(upstream.comments),
  }
}

export interface SerializeOptions {
  // Which servers take part: all of them, all but the pending TLS servers, or only those.
  servers?: 'all' | 'without-pending-tls' | 'pending-tls'
}

/**
 * A canonical string of the parts of a config that end up in the site file,
 * ignoring editor bookkeeping (directive `idx`, whitespace, empty directives).
 */
export function serializeNgxConfig(config: Partial<NgxConfig> | undefined | null, options: SerializeOptions = {}): string {
  const mode = options.servers ?? 'all'
  const servers = (config?.servers ?? []).filter(server => {
    if (mode === 'all')
      return true
    return mode === 'pending-tls' ? isPendingTLSServer(server) : !isPendingTLSServer(server)
  })

  return JSON.stringify({
    custom: mode === 'pending-tls' ? '' : trimmed(config?.custom),
    upstreams: mode === 'pending-tls' ? [] : (config?.upstreams ?? []).map(normalizeUpstream),
    servers: servers.map(normalizeServer),
  })
}

/**
 * Reports whether the on-screen config differs from the saved one in anything
 * but its pending TLS servers. HTTPS onboarding rewrites the saved file, so
 * such edits would be lost; pending TLS servers are compared separately
 * because an enabled site can never save them (nginx -t rejects a TLS server
 * without certificate).
 */
export function hasUnsavedChanges(current: Partial<NgxConfig> | undefined | null, saved: Partial<NgxConfig> | undefined | null): boolean {
  if (!current || !saved)
    return false

  return serializeNgxConfig(current, { servers: 'without-pending-tls' })
    !== serializeNgxConfig(saved, { servers: 'without-pending-tls' })
}

/** Reports whether the on-screen pending TLS servers differ from the saved ones. */
export function pendingTLSServersDiffer(current: Partial<NgxConfig> | undefined | null, saved: Partial<NgxConfig> | undefined | null): boolean {
  if (!current || !saved)
    return false

  return serializeNgxConfig(current, { servers: 'pending-tls' })
    !== serializeNgxConfig(saved, { servers: 'pending-tls' })
}
