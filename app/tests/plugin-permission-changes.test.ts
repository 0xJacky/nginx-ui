import { describe, expect, test } from 'bun:test'
import { asksForLess, asksForMore, permissionChanges } from '../src/views/system/plugins/permissionChanges'

describe('permissionChanges', () => {
  test('lists permissions added and dropped', () => {
    const changes = permissionChanges({ permissions: ['kv', 'cron'] }, { permissions: ['kv', 'notify'] })
    expect(changes.added).toEqual(['notify'])
    expect(changes.removed).toEqual(['cron'])
    expect(asksForMore(changes)).toBe(true)
    expect(asksForLess(changes)).toBe(true)
  })

  test('lists addresses added to a limited network permission', () => {
    const changes = permissionChanges(
      { permissions: ['network'], network_hosts: ['api.example.com'] },
      { permissions: ['network'], network_hosts: ['api.example.com', 'cdn.example.com'] },
    )
    expect(changes).toEqual({ added: [], removed: [], addedHosts: ['cdn.example.com'], removedHosts: [], anyHost: false })
  })

  test('flags a limited network permission that becomes any address', () => {
    const changes = permissionChanges(
      { permissions: ['network'], network_hosts: ['api.example.com'] },
      { permissions: ['network'] },
    )
    expect(changes.anyHost).toBe(true)
    expect(asksForMore(changes)).toBe(true)
  })

  test('takes a list where any address was allowed as less access', () => {
    const changes = permissionChanges({ permissions: ['network'] }, { permissions: ['network'], network_hosts: ['api.example.com'] })
    expect(asksForMore(changes)).toBe(false)
    expect(changes.addedHosts).toEqual([])
  })

  test('shows a new network permission once, not again for its addresses', () => {
    const changes = permissionChanges({ permissions: [] }, { permissions: ['network'], network_hosts: ['api.example.com'] })
    expect(changes).toEqual({ added: ['network'], removed: [], addedHosts: [], removedHosts: [], anyHost: false })
  })
})
