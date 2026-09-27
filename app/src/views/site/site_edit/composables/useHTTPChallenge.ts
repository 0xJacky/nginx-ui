import type { NgxLocation, NgxServer } from '@/api/ngx'

function returnDirectiveContent(params: string) {
  return `return ${params.trim().replace(/;$/, '')};`
}

/**
 * Installs the ACME HTTP-01 proxy location without leaving a server-level
 * return in front of it. Nginx evaluates a server-level return before any
 * location, so older quick-config files could otherwise answer the challenge
 * with a redirect or 404.
 */
export function ensureHTTPChallengeLocation(server: NgxServer, locations: NgxLocation[]) {
  const existingLocations = server.locations ?? []
  server.locations = existingLocations.filter(location => !location.path.includes('/.well-known/acme-challenge'))

  const returnDirectives = (server.directives ?? []).filter(directive => directive.directive === 'return')
  if (returnDirectives.length > 0) {
    server.directives = (server.directives ?? []).filter(directive => directive.directive !== 'return')
    const rootLocation = server.locations.find(location => location.path.trim() === '/')
    const returnContent = `${returnDirectives.map(directive => returnDirectiveContent(directive.params)).join('\n')}\n`
    if (rootLocation) {
      rootLocation.content = `${returnContent}${rootLocation.content}`
    }
    else {
      server.locations.push({ path: '/', content: returnContent, comments: '' })
    }
  }

  server.locations.push(...locations)
}
