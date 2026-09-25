import { describe, expect, test } from 'bun:test'
import { collectChangedPaths, copyPaths } from '../src/utils/changedPaths'

describe('collectChangedPaths', () => {
  test('returns nothing for identical objects', () => {
    const settings = { server: { port: 9000, enable_https: false }, cert: { recursive_nameservers: ['1.1.1.1'] } }
    expect(collectChangedPaths(settings, structuredClone(settings))).toEqual([])
  })

  test('lists nested leaf paths that changed', () => {
    const before = { server: { port: 9000, enable_https: false }, app: { page_size: 10 } }
    const after = { server: { port: 9000, enable_https: true }, app: { page_size: 20 } }
    expect(collectChangedPaths(before, after)).toEqual(['app.page_size', 'server.enable_https'])
  })

  test('compares arrays as a single value', () => {
    const before = { cert: { recursive_nameservers: ['1.1.1.1'] } }
    const after = { cert: { recursive_nameservers: ['1.1.1.1', '8.8.8.8'] } }
    expect(collectChangedPaths(before, after)).toEqual(['cert.recursive_nameservers'])
    expect(collectChangedPaths(after, structuredClone(after))).toEqual([])
  })

  test('treats missing, null and empty string as the same blank value', () => {
    expect(collectChangedPaths({ nginx: {} }, { nginx: { maintenance_host: '' } })).toEqual([])
    expect(collectChangedPaths({ nginx: { maintenance_host: null } }, { nginx: { maintenance_host: '' } })).toEqual([])
    expect(collectChangedPaths({ nginx: {} }, { nginx: { maintenance_host: 'a' } })).toEqual(['nginx.maintenance_host'])
  })

  test('does not treat false or zero as blank', () => {
    expect(collectChangedPaths({ a: { b: false } }, { a: { b: undefined } })).toEqual(['a.b'])
    expect(collectChangedPaths({ a: { b: 0 } }, { a: { b: '' } })).toEqual(['a.b'])
  })

  test('reports a whole section added or removed', () => {
    expect(collectChangedPaths({}, { listener: { unix_socket: '/tmp/a.sock' } })).toEqual(['listener.unix_socket'])
  })
})

describe('copyPaths', () => {
  test('copies only the listed paths', () => {
    const target = { nginx: { host_mode: '', container_name: '', stub_status_port: 1 } }
    const source = { nginx: { host_mode: 'ssh', container_name: 'nginx', stub_status_port: 2 } }
    copyPaths(target, source, ['nginx.host_mode', 'nginx.container_name'])
    expect(target).toEqual({ nginx: { host_mode: 'ssh', container_name: 'nginx', stub_status_port: 1 } })
  })

  test('copies arrays by value', () => {
    const target = { cert: { recursive_nameservers: [] as string[] } }
    const source = { cert: { recursive_nameservers: ['1.1.1.1'] } }
    copyPaths(target, source, ['cert.recursive_nameservers'])
    source.cert.recursive_nameservers.push('8.8.8.8')
    expect(target.cert.recursive_nameservers).toEqual(['1.1.1.1'])
  })
})
