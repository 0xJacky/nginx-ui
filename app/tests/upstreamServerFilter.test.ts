import { describe, expect, test } from 'bun:test'
import { disablesLastPrimary, filterUpstreams, normalizeKeyword } from '@/views/upstream/serverFilter'

const groups = [
  {
    name: 'api_pool',
    servers: [
      { address: '10.0.0.1:8080', socket: '10.0.0.1:8080' },
      { address: '10.0.0.2:8080', socket: '10.0.0.2:8080' },
    ],
  },
  {
    name: 'Web_Pool',
    servers: [
      { address: 'web.internal', socket: 'web.internal:80' },
      { address: '10.0.1.5:80', socket: '10.0.1.5:80' },
    ],
  },
]

describe('upstream search filter', () => {
  test('returns every group for an empty keyword', () => {
    expect(filterUpstreams(groups, '')).toBe(groups)
    expect(filterUpstreams(groups, '   ')).toBe(groups)
    expect(filterUpstreams(groups, undefined)).toBe(groups)
  })

  test('keeps all servers of a group whose name matches, ignoring case', () => {
    const result = filterUpstreams(groups, 'web_pool')
    expect(result).toHaveLength(1)
    expect(result[0]).toBe(groups[1])
    expect(result[0].servers).toHaveLength(2)
  })

  test('narrows groups to the servers whose address matches', () => {
    const result = filterUpstreams(groups, '10.0.0.2')
    expect(result).toHaveLength(1)
    expect(result[0].name).toBe('api_pool')
    expect(result[0].servers.map(s => s.address)).toEqual(['10.0.0.2:8080'])
    // The source list is left untouched.
    expect(groups[0].servers).toHaveLength(2)
  })

  test('matches an ip:port across groups', () => {
    const result = filterUpstreams(groups, ':80')
    expect(result.map(g => g.name)).toEqual(['api_pool', 'Web_Pool'])
    expect(result[1].servers.map(s => s.address)).toEqual(['web.internal', '10.0.1.5:80'])
  })

  test('matches the implicit default port through the socket', () => {
    const result = filterUpstreams(groups, 'web.internal:80')
    expect(result).toHaveLength(1)
    expect(result[0].servers.map(s => s.address)).toEqual(['web.internal'])
  })

  test('drops groups without a match', () => {
    expect(filterUpstreams(groups, '192.168.')).toEqual([])
  })

  test('normalizes the keyword', () => {
    expect(normalizeKeyword('  API_Pool ')).toBe('api_pool')
    expect(normalizeKeyword(null)).toBe('')
  })
})

describe('last primary server guard', () => {
  test('flags disabling the only enabled primary server', () => {
    const servers = [
      { address: 'a:80' },
      { address: 'b:80', down: true },
      { address: 'c:80', backup: true },
    ]
    expect(disablesLastPrimary(servers, 'a:80')).toBe(true)
  })

  test('allows disabling while another primary stays up', () => {
    const servers = [{ address: 'a:80' }, { address: 'b:80' }]
    expect(disablesLastPrimary(servers, 'a:80')).toBe(false)
  })

  test('never flags backup, already disabled or unknown servers', () => {
    const servers = [{ address: 'a:80', down: true }, { address: 'c:80', backup: true }]
    expect(disablesLastPrimary(servers, 'a:80')).toBe(false)
    expect(disablesLastPrimary(servers, 'c:80')).toBe(false)
    expect(disablesLastPrimary(servers, 'z:80')).toBe(false)
  })
})
