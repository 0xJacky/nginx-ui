import type { AccessServerState } from '@/api/access_list'
import { describe, expect, test } from 'bun:test'
import { PublicValue, sharedServerAccessValue } from '@/components/AccessControl/options'

function server(mode: AccessServerState['mode'], slug?: string): Pick<AccessServerState, 'mode' | 'slug'> {
  return { mode, slug }
}

describe('sharedServerAccessValue', () => {
  test('servers using the same list share its slug', () => {
    expect(sharedServerAccessValue([server('list', 'intranet')])).toBe('intranet')
    expect(sharedServerAccessValue([server('list', 'intranet'), server('list', 'intranet')])).toBe('intranet')
  })

  test('public servers share the public value', () => {
    expect(sharedServerAccessValue([server('public'), server('public')])).toBe(PublicValue)
  })

  test('servers that differ share nothing', () => {
    expect(sharedServerAccessValue([server('list', 'intranet'), server('public')])).toBeUndefined()
    expect(sharedServerAccessValue([server('list', 'intranet'), server('list', 'office')])).toBeUndefined()
  })

  test('custom rules share nothing', () => {
    expect(sharedServerAccessValue([server('manual')])).toBeUndefined()
    expect(sharedServerAccessValue([server('manual'), server('manual')])).toBeUndefined()
  })

  test('no servers share nothing', () => {
    expect(sharedServerAccessValue([])).toBeUndefined()
  })
})
