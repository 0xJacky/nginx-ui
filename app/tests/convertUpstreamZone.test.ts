import { describe, expect, test } from 'bun:test'
import { convertZoneChoice, findDeclaredZone } from '@/components/NgxConfigEditor/convertZone'

describe('findDeclaredZone', () => {
  test('returns the parameters of the zone directive', () => {
    expect(findDeclaredZone([
      { directive: 'least_conn', params: '' },
      { directive: 'zone', params: 'backend 128k' },
      { directive: 'server', params: '127.0.0.1:8080' },
    ])).toBe('backend 128k')
    expect(findDeclaredZone([{ directive: 'zone', params: ' shared; ' }])).toBe('shared')
  })

  test('is undefined for a block without a zone', () => {
    expect(findDeclaredZone([{ directive: 'server', params: '127.0.0.1:8080' }])).toBeUndefined()
    expect(findDeclaredZone([])).toBeUndefined()
    expect(findDeclaredZone(undefined)).toBeUndefined()
  })
})

describe('convertZoneChoice', () => {
  test('sends the switch and the size for a block without a zone', () => {
    expect(convertZoneChoice(undefined, true, 64)).toEqual({ zone: true, zone_size: '64k' })
    expect(convertZoneChoice(undefined, true, 1024)).toEqual({ zone: true, zone_size: '1024k' })
    expect(convertZoneChoice(undefined, true, null)).toEqual({ zone: true })
  })

  test('sends an explicit false when the switch is off', () => {
    expect(convertZoneChoice(undefined, false, 64)).toEqual({ zone: false })
  })

  test('sends nothing when the block keeps its own zone', () => {
    expect(convertZoneChoice('backend 128k', true, 64)).toEqual({})
    expect(convertZoneChoice('shared', false, 64)).toEqual({})
  })
})
