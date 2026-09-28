import { describe, expect, test } from 'bun:test'
import { kbToZoneSize, zoneSizeToKb } from '@/views/upstream/zoneSize'

describe('zoneSizeToKb', () => {
  test('reads k, m and byte sizes', () => {
    expect(zoneSizeToKb('64k')).toBe(64)
    expect(zoneSizeToKb('64K')).toBe(64)
    expect(zoneSizeToKb('1m')).toBe(1024)
    expect(zoneSizeToKb('65536')).toBe(64)
    expect(zoneSizeToKb('1000')).toBe(1)
  })

  test('returns null for anything nginx would not accept', () => {
    expect(zoneSizeToKb('')).toBeNull()
    expect(zoneSizeToKb(undefined)).toBeNull()
    expect(zoneSizeToKb('1g')).toBeNull()
    expect(zoneSizeToKb('64 k')).toBeNull()
  })
})

describe('kbToZoneSize', () => {
  test('formats whole KB values', () => {
    expect(kbToZoneSize(64)).toBe('64k')
    expect(kbToZoneSize(1024)).toBe('1024k')
    expect(kbToZoneSize(64.7)).toBe('64k')
    expect(kbToZoneSize(null)).toBe('')
  })
})
