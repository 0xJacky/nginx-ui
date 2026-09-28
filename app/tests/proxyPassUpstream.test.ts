import type { NgxConfig } from '@/api/ngx'
import { describe, expect, test } from 'bun:test'
import {
  applyUpstreamToLocations,
  defaultProxyLocationKeys,
  findUnreferencedUpstreams,
  getProxyPassTarget,
  getProxyPassUpstream,
  listProxyLocations,
  setProxyPassUpstream,
} from '@/components/NgxConfigEditor/proxyPassUpstream'

const names = new Set(['backend_pool', 'api'])

describe('getProxyPassUpstream', () => {
  test('finds a managed upstream with or without a URI', () => {
    expect(getProxyPassUpstream('proxy_pass http://backend_pool;', names)).toBe('backend_pool')
    expect(getProxyPassUpstream('  proxy_set_header Host $host;\n  proxy_pass https://api/v1/;', names)).toBe('api')
  })

  test('ignores hosts that are not upstreams, ports and comments', () => {
    expect(getProxyPassUpstream('proxy_pass http://127.0.0.1:9000/;', names)).toBeUndefined()
    expect(getProxyPassUpstream('proxy_pass http://backend_pool:8080;', names)).toBeUndefined()
    expect(getProxyPassUpstream('# proxy_pass http://backend_pool;', names)).toBeUndefined()
    expect(getProxyPassUpstream('return 200;', names)).toBeUndefined()
  })
})

describe('setProxyPassUpstream', () => {
  test('replaces the host and keeps scheme, URI and indentation', () => {
    const content = '    proxy_set_header Host $host;\n    proxy_pass https://127.0.0.1:9000/app/;\n'
    expect(setProxyPassUpstream(content, 'backend_pool'))
      .toBe('    proxy_set_header Host $host;\n    proxy_pass https://backend_pool/app/;\n')
  })

  test('switches between upstreams', () => {
    expect(setProxyPassUpstream('proxy_pass http://api/;', 'backend_pool')).toBe('proxy_pass http://backend_pool/;')
  })

  test('adds proxy_pass to a location that has none', () => {
    expect(setProxyPassUpstream('proxy_set_header Host $host;', 'api'))
      .toBe('proxy_set_header Host $host;\nproxy_pass http://api;\n')
    expect(setProxyPassUpstream('', 'api')).toBe('proxy_pass http://api;\n')
  })

  test('replaces a variable target entirely', () => {
    expect(setProxyPassUpstream('proxy_pass $backend;', 'api')).toBe('proxy_pass http://api;')
  })
})

function siteConfig(): Pick<NgxConfig, 'servers'> {
  return {
    servers: [
      {
        directives: [
          { directive: 'listen', params: '80' },
          { directive: 'server_name', params: 'old.test www.old.test' },
        ],
        locations: [
          { path: '/', content: '    proxy_set_header Host $host;\n    proxy_pass http://old_pool/;\n', comments: '' },
          { path: '/static/', content: '    root /var/www;\n', comments: '' },
          { path: '/api/', content: '    proxy_pass https://127.0.0.1:9000/v1/;\n', comments: '' },
        ],
      },
      {
        directives: [{ directive: 'listen', params: '8080' }],
        locations: [
          { path: '/', content: 'return 204;', comments: '' },
        ],
      },
    ],
  }
}

describe('listProxyLocations', () => {
  test('lists every location with its server name and proxy target', () => {
    expect(listProxyLocations(siteConfig())).toEqual([
      { key: '0:0', serverIdx: 0, locationIdx: 0, serverName: 'old.test www.old.test', path: '/', target: 'http://old_pool/' },
      { key: '0:1', serverIdx: 0, locationIdx: 1, serverName: 'old.test www.old.test', path: '/static/', target: undefined },
      { key: '0:2', serverIdx: 0, locationIdx: 2, serverName: 'old.test www.old.test', path: '/api/', target: 'https://127.0.0.1:9000/v1/' },
      { key: '1:0', serverIdx: 1, locationIdx: 0, serverName: '', path: '/', target: undefined },
    ])
  })

  test('handles servers without directives or locations', () => {
    expect(listProxyLocations({ servers: [{}] })).toEqual([])
    expect(getProxyPassTarget('# proxy_pass http://a;')).toBeUndefined()
  })
})

describe('defaultProxyLocationKeys', () => {
  test('preselects every location that proxies somewhere', () => {
    expect(defaultProxyLocationKeys(listProxyLocations(siteConfig()))).toEqual(['0:0', '0:2'])
  })

  test('preselects the only location of a site without proxy_pass', () => {
    const config = { servers: [{ locations: [{ path: '/', content: 'root /srv;', comments: '' }] }] }
    expect(defaultProxyLocationKeys(listProxyLocations(config))).toEqual(['0:0'])
  })

  test('preselects nothing when several locations have no proxy_pass', () => {
    const config = {
      servers: [{
        locations: [
          { path: '/', content: 'root /srv;', comments: '' },
          { path: '/a/', content: 'return 204;', comments: '' },
        ],
      }],
    }
    expect(defaultProxyLocationKeys(listProxyLocations(config))).toEqual([])
  })
})

describe('applyUpstreamToLocations', () => {
  test('rewrites the selected locations and reports the changes', () => {
    const config = siteConfig()
    const changes = applyUpstreamToLocations(config, 'shared_pool', ['0:0', '0:1', '0:2'])

    expect(changes.map(change => [change.key, change.target, change.after])).toEqual([
      ['0:0', 'http://old_pool/', 'http://shared_pool/'],
      ['0:1', undefined, 'http://shared_pool'],
      ['0:2', 'https://127.0.0.1:9000/v1/', 'https://shared_pool/v1/'],
    ])
    const locations = config.servers[0].locations!
    expect(locations[0].content).toBe('    proxy_set_header Host $host;\n    proxy_pass http://shared_pool/;\n')
    // A location without proxy_pass gets one added.
    expect(locations[1].content).toBe('    root /var/www;\nproxy_pass http://shared_pool;\n')
    expect(locations[2].content).toBe('    proxy_pass https://shared_pool/v1/;\n')
    // Unselected locations stay as they were.
    expect(config.servers[1].locations![0].content).toBe('return 204;')
  })

  test('skips locations that already use the group', () => {
    const config = siteConfig()
    applyUpstreamToLocations(config, 'shared_pool', ['0:0'])
    expect(applyUpstreamToLocations(config, 'shared_pool', ['0:0'])).toEqual([])
  })

  test('ignores unknown keys', () => {
    expect(applyUpstreamToLocations(siteConfig(), 'shared_pool', ['7:7'])).toEqual([])
  })
})

describe('findUnreferencedUpstreams', () => {
  test('reports site upstreams no location proxies to', () => {
    const config = siteConfig()
    expect(findUnreferencedUpstreams(config, ['old_pool', 'spare'])).toEqual(['spare'])

    applyUpstreamToLocations(config, 'shared_pool', ['0:0'])
    expect(findUnreferencedUpstreams(config, ['old_pool'])).toEqual(['old_pool'])
  })

  test('counts every pass directive and ignores comments and longer names', () => {
    const config = {
      servers: [{
        locations: [
          { path: '/grpc', content: 'grpc_pass grpc://grpc_pool;', comments: '' },
          { path: '/old', content: '# proxy_pass http://old_pool;\nproxy_pass http://old_pool_v2;', comments: '' },
        ],
      }],
    }
    expect(findUnreferencedUpstreams(config, ['grpc_pool', 'old_pool'])).toEqual(['old_pool'])
  })
})
