import type { NgxLocation, NgxServer } from '@/api/ngx'
import { describe, expect, it } from 'vitest'
import {
  buildTLSServerFromHTTPServer,
  isHTTPSRedirectReturn,
  serveTLSAppOverHTTP,
  stageServersForPendingTLS,
  stripHTTPSRedirectReturns,
} from '@/views/site/site_edit/composables/useHTTPSRedirect'

const challengeLocation: NgxLocation = {
  path: '~ /.well-known/acme-challenge',
  content: 'proxy_pass http://127.0.0.1:9180;',
  comments: '',
}

const redirectLocation: NgxLocation = {
  path: '/',
  content: 'return 301 https://$host$request_uri;',
  comments: '',
}

const appLocation: NgxLocation = {
  path: '/',
  content: 'proxy_pass http://127.0.0.1:9000;',
  comments: '',
}

// The shape the quick setup produces for a reverse proxy with "redirect HTTP
// to HTTPS": the port-80 server only redirects, the TLS server has the app.
function quickSetupServers(): NgxServer[] {
  return [
    {
      directives: [
        { directive: 'listen', params: '80' },
        { directive: 'listen', params: '[::]:80' },
        { directive: 'server_name', params: 'app.example.com' },
      ],
      locations: [{ ...redirectLocation }, { ...challengeLocation }],
    },
    {
      directives: [
        { directive: 'listen', params: '443 ssl' },
        { directive: 'listen', params: '[::]:443 ssl' },
        { directive: 'http2', params: 'on' },
        { directive: 'server_name', params: 'app.example.com' },
        { directive: 'ssl_certificate', params: '' },
        { directive: 'ssl_certificate_key', params: '' },
        { directive: 'ssl_protocols', params: 'TLSv1.2 TLSv1.3' },
        { directive: 'client_max_body_size', params: '128m' },
      ],
      locations: [{ ...appLocation }, { ...challengeLocation }],
    },
  ]
}

describe('isHTTPSRedirectReturn', () => {
  it('detects redirects to https on the same host', () => {
    expect(isHTTPSRedirectReturn('301 https://$host$request_uri')).toBe(true)
    expect(isHTTPSRedirectReturn('308 https://$host$request_uri')).toBe(true)
    expect(isHTTPSRedirectReturn('https://$host$request_uri')).toBe(true)
  })

  it('ignores other returns', () => {
    expect(isHTTPSRedirectReturn('404')).toBe(false)
    expect(isHTTPSRedirectReturn('301 https://other.example.com$request_uri')).toBe(false)
    expect(isHTTPSRedirectReturn('301 http://$host$request_uri')).toBe(false)
    expect(isHTTPSRedirectReturn('301 https://$hostname$request_uri')).toBe(false)
  })
})

describe('stripHTTPSRedirectReturns', () => {
  it('drops a line holding only the redirect', () => {
    expect(stripHTTPSRedirectReturns('return 301 https://$host$request_uri;\nproxy_pass http://127.0.0.1:9000;'))
      .toBe('proxy_pass http://127.0.0.1:9000;')
    expect(stripHTTPSRedirectReturns('    return 301 https://$host$request_uri;')).toBe('')
  })

  it('keeps unrelated content untouched', () => {
    const content = 'return 404;\nproxy_pass http://127.0.0.1:9000;'
    expect(stripHTTPSRedirectReturns(content)).toBe(content)
  })
})

describe('buildTLSServerFromHTTPServer', () => {
  it('drops a location / that only redirects to https', () => {
    const server: NgxServer = {
      directives: [
        { directive: 'listen', params: '80' },
        { directive: 'listen', params: '[::]:80' },
        { directive: 'server_name', params: 'app.example.com' },
      ],
      locations: [{ ...redirectLocation }, { ...challengeLocation }],
    }

    const tlsServer = buildTLSServerFromHTTPServer(server)

    expect(tlsServer.directives).toEqual([
      { directive: 'listen', params: '443 ssl' },
      { directive: 'listen', params: '[::]:443 ssl' },
      { directive: 'server_name', params: 'app.example.com' },
    ])
    expect(tlsServer.locations).toEqual([challengeLocation])
    // The source server is not modified.
    expect(server.locations).toHaveLength(2)
    expect(server.directives).toHaveLength(3)
  })

  it('drops a server-level https redirect and keeps the app', () => {
    const server: NgxServer = {
      directives: [
        { directive: 'listen', params: '80' },
        { directive: 'server_name', params: 'app.example.com' },
        { directive: 'return', params: '301 https://$host$request_uri' },
        { directive: 'root', params: '/var/www/html' },
      ],
      locations: [
        { path: '/', content: 'return 301 https://$host$request_uri;\ntry_files $uri $uri/ =404;', comments: '' },
      ],
    }

    const tlsServer = buildTLSServerFromHTTPServer(server)

    expect(tlsServer.directives).toEqual([
      { directive: 'listen', params: '443 ssl' },
      { directive: 'listen', params: '[::]:443 ssl' },
      { directive: 'server_name', params: 'app.example.com' },
      { directive: 'root', params: '/var/www/html' },
    ])
    expect(tlsServer.locations).toEqual([
      { path: '/', content: 'try_files $uri $uri/ =404;', comments: '' },
    ])
  })

  it('keeps unrelated returns', () => {
    const server: NgxServer = {
      directives: [{ directive: 'listen', params: '80' }],
      locations: [{ path: '/old', content: 'return 301 https://example.org/new;', comments: '' }],
    }

    expect(buildTLSServerFromHTTPServer(server).locations).toEqual(server.locations)
  })
})

describe('serveTLSAppOverHTTP', () => {
  it('returns the server unchanged when it does not redirect', () => {
    const [, tlsServer] = quickSetupServers()
    const httpServer: NgxServer = {
      directives: [{ directive: 'listen', params: '80' }],
      locations: [{ ...appLocation }, { ...challengeLocation }],
    }

    expect(serveTLSAppOverHTTP(httpServer, tlsServer)).toBe(httpServer)
  })

  it('keeps the root location left after stripping a mixed redirect', () => {
    const [, tlsServer] = quickSetupServers()
    const httpServer: NgxServer = {
      directives: [{ directive: 'listen', params: '80' }],
      locations: [
        { path: '/', content: 'return 301 https://$host$request_uri;\nproxy_pass http://127.0.0.1:8000;', comments: '' },
        { ...challengeLocation },
      ],
    }

    expect(serveTLSAppOverHTTP(httpServer, tlsServer).locations).toEqual([
      { path: '/', content: 'proxy_pass http://127.0.0.1:8000;', comments: '' },
      challengeLocation,
    ])
  })
})

describe('stageServersForPendingTLS', () => {
  it('serves the pending TLS app over HTTP instead of redirecting', () => {
    const servers = quickSetupServers()

    const staged = stageServersForPendingTLS(servers)

    expect(staged).toHaveLength(1)
    expect(staged[0].directives).toEqual([
      { directive: 'listen', params: '80' },
      { directive: 'listen', params: '[::]:80' },
      { directive: 'server_name', params: 'app.example.com' },
      { directive: 'client_max_body_size', params: '128m' },
    ])
    expect(staged[0].locations).toEqual([challengeLocation, appLocation])
    // The editor state is not modified.
    expect(servers).toEqual(quickSetupServers())
  })

  it('keeps the redirect when the TLS server already has a certificate', () => {
    const servers = quickSetupServers()
    const tlsDirectives = servers[1].directives!
    tlsDirectives.find(v => v.directive === 'ssl_certificate')!.params = '/etc/nginx/ssl/app/fullchain.cer'
    tlsDirectives.find(v => v.directive === 'ssl_certificate_key')!.params = '/etc/nginx/ssl/app/private.key'

    expect(stageServersForPendingTLS(servers)).toEqual(servers)
  })

  it('keeps the redirect when a working TLS server with the same names exists', () => {
    const servers = quickSetupServers()
    servers.push({
      directives: [
        { directive: 'listen', params: '8443 ssl' },
        { directive: 'server_name', params: 'app.example.com' },
        { directive: 'ssl_certificate', params: '/etc/nginx/ssl/app/fullchain.cer' },
        { directive: 'ssl_certificate_key', params: '/etc/nginx/ssl/app/private.key' },
      ],
      locations: [],
    })

    const staged = stageServersForPendingTLS(servers)

    expect(staged).toHaveLength(2)
    expect(staged[0]).toEqual(servers[0])
  })

  it('leaves port-80 servers for other names alone', () => {
    const servers = quickSetupServers()
    servers[0].directives![2].params = 'other.example.com'

    const staged = stageServersForPendingTLS(servers)

    expect(staged).toEqual([servers[0]])
  })

  it('matches server names regardless of order', () => {
    const servers = quickSetupServers()
    servers[0].directives![2].params = 'www.example.com app.example.com'
    servers[1].directives![3].params = 'app.example.com www.example.com'

    expect(stageServersForPendingTLS(servers)[0].locations).toEqual([challengeLocation, appLocation])
  })
})
