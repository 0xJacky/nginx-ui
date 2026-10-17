import { describe, expect, test } from 'bun:test'

Object.assign(globalThis, { location: { protocol: 'https:' }, window: { location: { href: 'https://ui.example.com/app/' } } })
const { buildPluginWebSocketUrl } = await import('../src/plugin/wsUrl')

const session = { token: '', shortToken: 'abcdefghijklmnop', nodeId: 0 }

describe('buildPluginWebSocketUrl', () => {
  test('points at the plugin http capability with a wss address', () => {
    const url = new URL(buildPluginWebSocketUrl('com.example.demo', '/events', session))
    expect(url.protocol).toBe('wss:')
    expect(url.host).toBe('ui.example.com')
    expect(url.pathname).toBe('/app/api/plugins/com.example.demo/http/events')
    expect(url.searchParams.get('token')).toBe('abcdefghijklmnop')
    expect(url.searchParams.has('x_node_id')).toBe(false)
  })

  test('falls back to the URL safe long token', () => {
    const token = 'header.payload?>>>>.signature'
    const url = new URL(buildPluginWebSocketUrl('com.example.demo', 'events', { ...session, token, shortToken: '' }))
    const expected = btoa(token).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
    expect(url.searchParams.get('token')).toBe(expected)
  })

  test('carries the selected node for cluster proxying', () => {
    const url = new URL(buildPluginWebSocketUrl('com.example.demo', 'events', { ...session, nodeId: 4 }))
    expect(url.searchParams.get('x_node_id')).toBe('4')
  })

  test('keeps the query of the path and encodes the plugin id', () => {
    const url = new URL(buildPluginWebSocketUrl('a b', '//geolite/download?force=1', session))
    expect(url.pathname).toBe('/app/api/plugins/a%20b/http/geolite/download')
    expect(url.searchParams.get('force')).toBe('1')
    expect(url.searchParams.get('token')).toBe('abcdefghijklmnop')
  })
})
