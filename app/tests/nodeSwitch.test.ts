import type { RouteLocationNormalizedLoaded } from 'vue-router'
import { describe, expect, test } from 'bun:test'
import {
  filterSwitchNodes,
  getNodeSwitchBlocker,
  getNodeVersionState,
  resolveSwitchLanding,
  sortSwitchNodes,
} from '@/lib/node/switch'

type LandingRoute = Parameters<typeof resolveSwitchLanding>[0]

function route(path: string, params: RouteLocationNormalizedLoaded['params'] = {}, hidden?: boolean | (() => boolean)): LandingRoute {
  return {
    path,
    params,
    matched: [
      { meta: {} },
      { meta: { hiddenInSidebar: hidden } },
    ] as unknown as LandingRoute['matched'],
  }
}

describe('node version state', () => {
  test('is compatible only on an exact version match', () => {
    expect(getNodeVersionState({ version: '2.7.0' }, '2.7.0')).toBe('compatible')
    expect(getNodeVersionState({ version: '2.6.3' }, '2.7.0')).toBe('mismatch')
  })

  test('is unknown when either side has no version', () => {
    expect(getNodeVersionState({}, '2.7.0')).toBe('unknown')
    expect(getNodeVersionState({ version: '2.7.0' }, '')).toBe('unknown')
  })
})

describe('node switch blocker', () => {
  test('allows online nodes on the local version', () => {
    expect(getNodeSwitchBlocker({ status: true, version: '2.7.0' }, '2.7.0')).toBeNull()
  })

  test('reports offline before any version problem', () => {
    expect(getNodeSwitchBlocker({ status: false, version: '2.6.3' }, '2.7.0')).toBe('offline')
  })

  test('blocks mismatched and unknown versions', () => {
    expect(getNodeSwitchBlocker({ status: true, version: '2.6.3' }, '2.7.0')).toBe('version_mismatch')
    expect(getNodeSwitchBlocker({ status: true }, '2.7.0')).toBe('version_unknown')
  })
})

describe('node switch list', () => {
  const nodes = [
    { name: 'tyo-lb', url: 'https://10.8.0.4:9000', status: false },
    { name: 'sg-edge', url: 'https://10.0.2.11:9000', status: true },
    { name: 'fra-edge', url: 'https://fra.example.net:9000', status: true },
  ]

  test('sorts online nodes first, then by name', () => {
    expect(sortSwitchNodes(nodes).map(n => n.name)).toEqual(['fra-edge', 'sg-edge', 'tyo-lb'])
  })

  test('does not reorder the input array', () => {
    sortSwitchNodes(nodes)
    expect(nodes[0].name).toBe('tyo-lb')
  })

  test('filters by name or URL, ignoring case and whitespace', () => {
    expect(filterSwitchNodes(nodes, ' EDGE ').map(n => n.name)).toEqual(['sg-edge', 'fra-edge'])
    expect(filterSwitchNodes(nodes, 'example.net').map(n => n.name)).toEqual(['fra-edge'])
    expect(filterSwitchNodes(nodes, '')).toHaveLength(3)
  })
})

describe('node switch landing', () => {
  test('stays on pages without route params', () => {
    expect(resolveSwitchLanding(route('/sites/list'))).toBe('/sites/list')
  })

  test('falls back to the dashboard for resource pages', () => {
    expect(resolveSwitchLanding(route('/sites/example.com', { name: 'example.com' }))).toBe('/dashboard')
    expect(resolveSwitchLanding(route('/config/conf.d/edit', { name: ['conf.d'] }))).toBe('/dashboard')
  })

  test('ignores empty params', () => {
    expect(resolveSwitchLanding(route('/config', { name: [] }))).toBe('/config')
    expect(resolveSwitchLanding(route('/streams', { name: '' }))).toBe('/streams')
  })

  test('falls back when the page is hidden for the new node', () => {
    expect(resolveSwitchLanding(route('/nodes', {}, () => true))).toBe('/dashboard')
    expect(resolveSwitchLanding(route('/nodes', {}, () => false))).toBe('/nodes')
  })

  test('keeps pages that are only statically hidden from the sidebar', () => {
    expect(resolveSwitchLanding(route('/sites/add', {}, true))).toBe('/sites/add')
  })
})
