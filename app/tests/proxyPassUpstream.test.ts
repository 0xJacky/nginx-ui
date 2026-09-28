import { describe, expect, test } from 'bun:test'
import { getProxyPassUpstream, setProxyPassUpstream } from '@/components/NgxConfigEditor/proxyPassUpstream'

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
