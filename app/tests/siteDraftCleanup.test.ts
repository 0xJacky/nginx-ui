import { describe, expect, test } from 'bun:test'
import { ConfigStatus } from '@/constants'
import { staleDraftAction, staleDraftName } from '@/views/site/site_add/draftCleanup'

describe('stale draft detection', () => {
  test('a rename after a draft save leaves the old draft behind', () => {
    expect(staleDraftName('app.example.com', 'www.example.com')).toBe('app.example.com')
  })

  test('no draft, an unchanged name or an empty name leave nothing behind', () => {
    expect(staleDraftName('', 'www.example.com')).toBeUndefined()
    expect(staleDraftName(undefined, 'www.example.com')).toBeUndefined()
    expect(staleDraftName('app.example.com', 'app.example.com')).toBeUndefined()
    expect(staleDraftName(' app.example.com ', 'app.example.com')).toBeUndefined()
    expect(staleDraftName('app.example.com', '')).toBeUndefined()
    expect(staleDraftName('app.example.com', undefined)).toBeUndefined()
  })
})

describe('stale draft action', () => {
  test('only a disabled draft is deleted', () => {
    expect(staleDraftAction(ConfigStatus.Disabled)).toBe('delete')
  })

  test('an enabled, maintained or unknown draft is kept', () => {
    expect(staleDraftAction(ConfigStatus.Enabled)).toBe('keep')
    expect(staleDraftAction(ConfigStatus.Maintenance)).toBe('keep')
    expect(staleDraftAction(undefined)).toBe('keep')
    expect(staleDraftAction('')).toBe('keep')
  })
})
