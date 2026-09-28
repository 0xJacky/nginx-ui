// Helpers for pointing a location's `proxy_pass` at a named upstream group.

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
