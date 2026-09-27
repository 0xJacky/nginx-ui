import type { NgxConfig, NgxServer } from '@/api/ngx'
import { describe, expect, test } from 'bun:test'
import {
  carryOverCertificate,
  extractServerDomains,
  extractSiteDomains,
  hasPendingTLSServer,
  hasTLSServer,
  hasUnsavedChanges,
  isPendingTLSServer,
  pendingTLSServersDiffer,
  sameStringList,
  serializeNgxConfig,
} from '@/views/site/site_edit/components/HTTPS/siteHTTPSState'

function httpServer(names = 'app.example.com'): NgxServer {
  return {
    directives: [
      { directive: 'listen', params: '80' },
      { directive: 'listen', params: '[::]:80' },
      { directive: 'server_name', params: names },
    ],
    locations: [
      { path: '/', content: 'return 301 https://$host$request_uri;', comments: '' },
      { path: '~ /.well-known/acme-challenge', content: 'proxy_pass http://127.0.0.1:9180;', comments: '' },
    ],
  }
}

// The TLS server the quick setup generates: empty certificate placeholders.
function pendingTLSServer(names = 'app.example.com'): NgxServer {
  return {
    directives: [
      { directive: 'listen', params: '443 ssl' },
      { directive: 'listen', params: '[::]:443 ssl' },
      { directive: 'server_name', params: names },
      { directive: 'ssl_certificate', params: '' },
      { directive: 'ssl_certificate_key', params: '' },
    ],
    locations: [
      { path: '/', content: 'proxy_pass http://127.0.0.1:9000;', comments: '' },
    ],
  }
}

function certifiedTLSServer(names = 'app.example.com'): NgxServer {
  return {
    directives: [
      { directive: 'listen', params: '443 ssl' },
      { directive: 'server_name', params: names },
      { directive: 'ssl_certificate', params: '/etc/nginx/ssl/app/fullchain.cer' },
      { directive: 'ssl_certificate_key', params: '/etc/nginx/ssl/app/private.key' },
    ],
    locations: [],
  }
}

function config(servers: NgxServer[]): NgxConfig {
  return { name: 'app', servers, upstreams: [], custom: '' }
}

function clone<T>(value: T): T {
  return JSON.parse(JSON.stringify(value))
}

describe('pending TLS detection', () => {
  test('a TLS server with empty or missing certificate directives is pending', () => {
    expect(isPendingTLSServer(pendingTLSServer())).toBe(true)

    const withoutDirectives = pendingTLSServer()
    withoutDirectives.directives = withoutDirectives.directives!.filter(d => !d.directive.startsWith('ssl_'))
    expect(isPendingTLSServer(withoutDirectives)).toBe(true)

    const keyOnly = certifiedTLSServer()
    keyOnly.directives = keyOnly.directives!.filter(d => d.directive !== 'ssl_certificate')
    expect(isPendingTLSServer(keyOnly)).toBe(true)
  })

  test('plain HTTP and certified TLS servers are not pending', () => {
    expect(isPendingTLSServer(httpServer())).toBe(false)
    expect(isPendingTLSServer(certifiedTLSServer())).toBe(false)
    expect(isPendingTLSServer(undefined)).toBe(false)
  })

  test('site level checks', () => {
    expect(hasTLSServer(config([httpServer()]))).toBe(false)
    expect(hasPendingTLSServer(config([httpServer()]))).toBe(false)

    expect(hasTLSServer(config([httpServer(), pendingTLSServer()]))).toBe(true)
    expect(hasPendingTLSServer(config([httpServer(), pendingTLSServer()]))).toBe(true)

    expect(hasTLSServer(config([httpServer(), certifiedTLSServer()]))).toBe(true)
    expect(hasPendingTLSServer(config([httpServer(), certifiedTLSServer()]))).toBe(false)

    expect(hasTLSServer(undefined)).toBe(false)
    expect(hasPendingTLSServer({ servers: undefined as unknown as NgxServer[] })).toBe(false)
  })
})

describe('domain extraction', () => {
  test('splits, normalises and de-duplicates server_name values', () => {
    const server = httpServer('App.Example.com  www.example.com. app.example.com')
    server.directives!.push({ directive: 'server_name', params: '.example.org' })

    expect(extractServerDomains([server])).toEqual(['app.example.com', 'www.example.com', 'example.org'])
  })

  test('drops names that cannot be certificate identifiers', () => {
    const server = httpServer('_ localhost ~^(?<sub>.+)\\.example\\.com$ www.* $host app.example.com')

    expect(extractServerDomains([server])).toEqual(['app.example.com'])
  })

  test('keeps wildcards and IP addresses for the card to validate', () => {
    expect(extractServerDomains([httpServer('*.example.com 203.0.113.10')])).toEqual(['*.example.com', '203.0.113.10'])
  })

  test('a site prefers the names of its pending TLS servers', () => {
    const site = config([httpServer('legacy.example.com app.example.com'), pendingTLSServer('app.example.com')])

    expect(extractSiteDomains(site)).toEqual(['app.example.com'])
  })

  test('a site without pending TLS servers uses every server name', () => {
    const site = config([httpServer('a.example.com'), httpServer('b.example.com a.example.com')])

    expect(extractSiteDomains(site)).toEqual(['a.example.com', 'b.example.com'])
  })

  test('falls back to all names when the pending TLS server has none usable', () => {
    const site = config([httpServer('app.example.com'), pendingTLSServer('_')])

    expect(extractSiteDomains(site)).toEqual(['app.example.com'])
  })

  test('sameStringList compares order and content', () => {
    expect(sameStringList(['a', 'b'], ['a', 'b'])).toBe(true)
    expect(sameStringList(['a', 'b'], ['b', 'a'])).toBe(false)
    expect(sameStringList(['a'], ['a', 'b'])).toBe(false)
    expect(sameStringList(undefined, [])).toBe(false)
  })
})

describe('unsaved change detection', () => {
  test('editor bookkeeping and whitespace are not changes', () => {
    const saved = config([httpServer(), certifiedTLSServer()])
    const current = clone(saved)
    current.servers[0].directives!.forEach((d, idx) => {
      d.idx = idx
    })
    current.servers[0].directives![2].params = '  app.example.com '
    current.servers[0].locations![0].content = '\nreturn 301 https://$host$request_uri;\n'

    expect(serializeNgxConfig(current)).toBe(serializeNgxConfig(saved))
    expect(hasUnsavedChanges(current, saved)).toBe(false)
  })

  test('empty placeholder directives do not count, they never reach the file', () => {
    const inMemory = config([httpServer(), pendingTLSServer()])
    const fromFile = clone(inMemory)
    fromFile.servers[1].directives = fromFile.servers[1].directives!.filter(d => d.params !== '')

    expect(serializeNgxConfig(inMemory)).toBe(serializeNgxConfig(fromFile))
  })

  test('edits outside pending TLS servers are unsaved changes', () => {
    const saved = config([httpServer(), certifiedTLSServer()])

    const location = clone(saved)
    location.servers[0].locations!.push({ path: '/api', content: 'proxy_pass http://127.0.0.1:9001;', comments: '' })
    expect(hasUnsavedChanges(location, saved)).toBe(true)

    const custom = clone(saved)
    custom.custom = 'map $a $b { default 1; }'
    expect(hasUnsavedChanges(custom, saved)).toBe(true)

    const certificate = clone(saved)
    certificate.servers[1].directives![2].params = '/etc/nginx/ssl/other/fullchain.cer'
    expect(hasUnsavedChanges(certificate, saved)).toBe(true)
  })

  test('a pending TLS server added on screen is tracked separately', () => {
    const saved = config([httpServer()])
    const current = config([httpServer(), pendingTLSServer()])

    expect(hasUnsavedChanges(current, saved)).toBe(false)
    expect(pendingTLSServersDiffer(current, saved)).toBe(true)
    expect(pendingTLSServersDiffer(saved, clone(saved))).toBe(false)
  })

  test('nothing to compare without a saved config', () => {
    expect(hasUnsavedChanges(config([httpServer()]), undefined)).toBe(false)
    expect(pendingTLSServersDiffer(config([httpServer()]), undefined)).toBe(false)
  })
})

describe('certificate carry-over on regeneration', () => {
  test('a working certificate replaces the placeholders in place', () => {
    const next = config([httpServer(), pendingTLSServer()])
    const previous = config([httpServer(), certifiedTLSServer()])

    expect(carryOverCertificate(next, previous)).toBe(true)
    expect(isPendingTLSServer(next.servers[1])).toBe(false)
    expect(next.servers[1].directives!.map(d => d.directive)).toEqual([
      'listen',
      'listen',
      'server_name',
      'ssl_certificate',
      'ssl_certificate_key',
    ])
    expect(next.servers[1].directives![3].params).toBe('/etc/nginx/ssl/app/fullchain.cer')
    expect(next.servers[1].directives![4].params).toBe('/etc/nginx/ssl/app/private.key')
    // The source config is not touched.
    expect(previous.servers[1].directives![2].params).toBe('/etc/nginx/ssl/app/fullchain.cer')
  })

  test('keeps every certificate pair of the source server', () => {
    const source = certifiedTLSServer()
    source.directives!.push(
      { directive: 'ssl_certificate', params: '/etc/nginx/ssl/app_rsa/fullchain.cer' },
      { directive: 'ssl_certificate_key', params: '/etc/nginx/ssl/app_rsa/private.key' },
    )
    const next = config([pendingTLSServer()])

    expect(carryOverCertificate(next, config([source]))).toBe(true)
    expect(next.servers[0].directives!.filter(d => d.directive === 'ssl_certificate').map(d => d.params)).toEqual([
      '/etc/nginx/ssl/app/fullchain.cer',
      '/etc/nginx/ssl/app_rsa/fullchain.cer',
    ])
  })

  test('appends when the pending server has no placeholders', () => {
    const pending = pendingTLSServer()
    pending.directives = pending.directives!.filter(d => !d.directive.startsWith('ssl_certificate'))
    const next = config([pending])

    expect(carryOverCertificate(next, config([certifiedTLSServer()]))).toBe(true)
    expect(isPendingTLSServer(next.servers[0])).toBe(false)
    expect(next.servers[0].directives!.at(-1)!.directive).toBe('ssl_certificate_key')
  })

  test('nothing to carry without a working certificate or a pending server', () => {
    const next = config([httpServer(), pendingTLSServer()])
    expect(carryOverCertificate(next, config([httpServer(), pendingTLSServer()]))).toBe(false)
    expect(carryOverCertificate(next, config([httpServer()]))).toBe(false)
    expect(carryOverCertificate(next, undefined)).toBe(false)
    expect(isPendingTLSServer(next.servers[1])).toBe(true)

    const plain = config([httpServer()])
    expect(carryOverCertificate(plain, config([certifiedTLSServer()]))).toBe(false)
    expect(plain.servers[0].directives).toEqual(httpServer().directives)
  })
})
