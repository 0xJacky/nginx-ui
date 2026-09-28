// Helpers for pointing a location's `proxy_pass` at a named upstream group.

import type { NgxConfig, NgxServer } from '@/api/ngx'

// Matches the first uncommented proxy_pass directive and captures its target.
const proxyPassPattern = /^([ \t]*)proxy_pass[ \t]+([^;\s]+)[ \t]*;/m

// scheme://host[:port][uri]
const targetPattern = /^([a-z][\w+.-]*):\/\/(\[[^\]]+\]|[^/:?]+)(:\d+)?([/?].*)?$/i

interface ProxyPassTarget {
  scheme: string
  host: string
  port: string
  uri: string
}

function parseTarget(target: string): ProxyPassTarget | null {
  const match = targetPattern.exec(target)
  if (!match)
    return null
  return {
    scheme: match[1],
    host: match[2],
    port: match[3] ?? '',
    uri: match[4] ?? '',
  }
}

/**
 * Returns the upstream group a location proxies to, or undefined when its
 * proxy_pass points somewhere that is not one of the given upstream names.
 */
export function getProxyPassUpstream(content: string, names: Set<string>): string | undefined {
  const match = proxyPassPattern.exec(content)
  if (!match)
    return undefined

  const target = parseTarget(match[2])
  if (!target || target.port || !names.has(target.host))
    return undefined

  return target.host
}

/**
 * Points the first proxy_pass of a location at the upstream group `name`,
 * keeping its scheme and URI part. A location without proxy_pass gets one.
 */
export function setProxyPassUpstream(content: string, name: string): string {
  const match = proxyPassPattern.exec(content)
  if (!match) {
    const separator = content === '' || content.endsWith('\n') ? '' : '\n'
    return `${content}${separator}proxy_pass http://${name};\n`
  }

  const [directive, indent, rawTarget] = match
  const target = parseTarget(rawTarget)
  // Variables or unusual targets cannot be rewritten safely; replace them.
  const scheme = target?.scheme ?? 'http'
  const uri = target?.uri ?? ''
  const replacement = `${indent}proxy_pass ${scheme}://${name}${uri};`

  return content.slice(0, match.index) + replacement + content.slice(match.index + directive.length)
}

/**
 * Returns the target of the first proxy_pass of a location, or undefined when
 * the location does not proxy anywhere.
 */
export function getProxyPassTarget(content: string): string | undefined {
  return proxyPassPattern.exec(content)?.[2]
}

/** A location of the site that can be pointed at an upstream group. */
export interface ProxyLocation {
  /** `<server index>:<location index>` */
  key: string
  serverIdx: number
  locationIdx: number
  /** The server_name of the server block; empty when it has none. */
  serverName: string
  path: string
  /** The current proxy_pass target; undefined when the location has none. */
  target?: string
}

/** A location changed by applyUpstreamToLocations. */
export interface ProxyLocationChange extends ProxyLocation {
  /** The proxy_pass target after the change. */
  after: string
}

function serverNameOf(server: NgxServer) {
  return (server.directives ?? [])
    .filter(directive => directive.directive === 'server_name')
    .map(directive => (directive.params ?? '').trim())
    .filter(Boolean)
    .join(' ')
}

/** Lists every location of every server block of the config, in file order. */
export function listProxyLocations(config: Pick<NgxConfig, 'servers'>): ProxyLocation[] {
  return (config.servers ?? []).flatMap((server, serverIdx) => {
    const serverName = serverNameOf(server)
    return (server.locations ?? []).map((location, locationIdx) => ({
      key: `${serverIdx}:${locationIdx}`,
      serverIdx,
      locationIdx,
      serverName,
      path: location.path,
      target: getProxyPassTarget(location.content ?? ''),
    }))
  })
}

/**
 * The locations preselected when a site switches to an upstream group: every
 * location that already proxies somewhere, or the only location of a site
 * that has exactly one.
 */
export function defaultProxyLocationKeys(locations: ProxyLocation[]): string[] {
  const proxied = locations.filter(location => location.target !== undefined)
  if (proxied.length > 0)
    return proxied.map(location => location.key)
  if (locations.length === 1)
    return [locations[0].key]
  return []
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/**
 * Returns the upstream blocks of the site, among `names`, that no uncommented
 * pass directive of any location refers to anymore.
 */
export function findUnreferencedUpstreams(config: Pick<NgxConfig, 'servers'>, names: string[]): string[] {
  const contents = (config.servers ?? []).flatMap(server => (server.locations ?? []).map(location => location.content ?? ''))
  return names.filter(name => {
    const pattern = new RegExp(`^[^#\\n]*?\\b(?:proxy|grpc|fastcgi|uwsgi|scgi|memcached)_pass\\s+(?:[a-z][\\w+.-]*://)?${escapeRegExp(name)}(?::\\d+)?(?:[/;\\s?]|$)`, 'im')
    return !contents.some(content => pattern.test(content))
  })
}

/**
 * Points the selected locations at the upstream group `name` through
 * setProxyPassUpstream, which keeps the scheme and URI of an existing
 * proxy_pass and adds `proxy_pass http://<name>;` where there is none. Returns
 * the locations whose content changed; ones that already use the group are
 * left alone.
 */
export function applyUpstreamToLocations(
  config: Pick<NgxConfig, 'servers'>,
  name: string,
  keys: Iterable<string>,
): ProxyLocationChange[] {
  const selected = new Set(keys)
  const changes: ProxyLocationChange[] = []
  for (const entry of listProxyLocations(config)) {
    if (!selected.has(entry.key))
      continue
    const location = config.servers[entry.serverIdx].locations![entry.locationIdx]
    const content = location.content ?? ''
    const next = setProxyPassUpstream(content, name)
    if (next === content)
      continue
    location.content = next
    changes.push({ ...entry, after: getProxyPassTarget(next)! })
  }
  return changes
}
