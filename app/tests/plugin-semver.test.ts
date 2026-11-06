import { describe, expect, test } from 'bun:test'
import { compareVersions, parseVersion, satisfies } from '../src/plugin/semver'

describe('parseVersion', () => {
  test('reads major, minor and patch with sane defaults', () => {
    expect(parseVersion('1.2.3')).toEqual({ major: 1, minor: 2, patch: 3 })
    expect(parseVersion('v2.0')).toEqual({ major: 2, minor: 0, patch: 0 })
    expect(parseVersion('3')).toEqual({ major: 3, minor: 0, patch: 0 })
  })

  test('ignores prerelease and build metadata', () => {
    expect(parseVersion('1.2.3-beta.1')).toEqual({ major: 1, minor: 2, patch: 3 })
    expect(parseVersion('1.2.3+build.5')).toEqual({ major: 1, minor: 2, patch: 3 })
  })

  test('rejects anything that is not a version', () => {
    expect(parseVersion('')).toBeNull()
    expect(parseVersion('latest')).toBeNull()
    expect(parseVersion('1.2.3.4')).toBeNull()
  })
})

describe('compareVersions', () => {
  test('orders by major, then minor, then patch', () => {
    expect(compareVersions({ major: 1, minor: 0, patch: 0 }, { major: 2, minor: 0, patch: 0 })).toBeLessThan(0)
    expect(compareVersions({ major: 1, minor: 3, patch: 0 }, { major: 1, minor: 2, patch: 9 })).toBeGreaterThan(0)
    expect(compareVersions({ major: 1, minor: 2, patch: 3 }, { major: 1, minor: 2, patch: 3 })).toBe(0)
  })
})

describe('satisfies', () => {
  test('treats an empty range or a wildcard as always satisfied', () => {
    expect(satisfies('1.2.3', '')).toBe(true)
    expect(satisfies('1.2.3', '*')).toBe(true)
    expect(satisfies('1.2.3', '  ')).toBe(true)
  })

  test('matches exact versions', () => {
    expect(satisfies('1.2.3', '1.2.3')).toBe(true)
    expect(satisfies('1.2.3', '=1.2.3')).toBe(true)
    expect(satisfies('1.2.4', '1.2.3')).toBe(false)
  })

  test('treats a partial exact version as a wildcard on the missing parts', () => {
    expect(satisfies('1.2.9', '1.2')).toBe(true)
    expect(satisfies('1.3.0', '1.2')).toBe(false)
    expect(satisfies('1.9.9', '1')).toBe(true)
    expect(satisfies('2.0.0', '1')).toBe(false)
  })

  test('handles the comparison operators', () => {
    expect(satisfies('1.2.3', '>=1.2.3')).toBe(true)
    expect(satisfies('1.2.2', '>=1.2.3')).toBe(false)
    expect(satisfies('1.2.4', '>1.2.3')).toBe(true)
    expect(satisfies('1.2.3', '>1.2.3')).toBe(false)
    expect(satisfies('1.2.3', '<=1.2.3')).toBe(true)
    expect(satisfies('1.2.4', '<=1.2.3')).toBe(false)
    expect(satisfies('1.2.2', '<1.2.3')).toBe(true)
    expect(satisfies('1.2.3', '<1.2.3')).toBe(false)
  })

  test('tolerates a space between the operator and the version', () => {
    expect(satisfies('3.5.42', '>= 3.5.0')).toBe(true)
    expect(satisfies('3.4.0', '>= 3.5.0')).toBe(false)
  })

  test('applies the caret rules', () => {
    expect(satisfies('1.9.0', '^1.2.3')).toBe(true)
    expect(satisfies('2.0.0', '^1.2.3')).toBe(false)
    expect(satisfies('1.2.2', '^1.2.3')).toBe(false)
    expect(satisfies('0.2.9', '^0.2.3')).toBe(true)
    expect(satisfies('0.3.0', '^0.2.3')).toBe(false)
    expect(satisfies('0.0.3', '^0.0.3')).toBe(true)
    expect(satisfies('0.0.4', '^0.0.3')).toBe(false)
    expect(satisfies('1.5.0', '^1.2')).toBe(true)
    expect(satisfies('0.9.0', '^0')).toBe(true)
    expect(satisfies('1.0.0', '^0')).toBe(false)
  })

  test('applies the tilde rules', () => {
    expect(satisfies('1.2.9', '~1.2.3')).toBe(true)
    expect(satisfies('1.3.0', '~1.2.3')).toBe(false)
    expect(satisfies('1.2.0', '~1.2')).toBe(true)
    expect(satisfies('1.3.0', '~1.2')).toBe(false)
    expect(satisfies('1.9.9', '~1')).toBe(true)
    expect(satisfies('2.0.0', '~1')).toBe(false)
  })

  test('combines space separated comparators with AND', () => {
    expect(satisfies('3.5.42', '>=3.5.42 <4')).toBe(true)
    expect(satisfies('4.0.0', '>=3.5.42 <4')).toBe(false)
    expect(satisfies('3.5.41', '>=3.5.42 <4')).toBe(false)
  })

  test('combines groups with OR', () => {
    expect(satisfies('1.5.0', '^1.0.0 || ^2.0.0')).toBe(true)
    expect(satisfies('2.5.0', '^1.0.0 || ^2.0.0')).toBe(true)
    expect(satisfies('3.0.0', '^1.0.0 || ^2.0.0')).toBe(false)
  })

  test('ignores prerelease on the version under test', () => {
    expect(satisfies('1.2.3-rc.1', '^1.2.3')).toBe(true)
    expect(satisfies('1.5.4-beta', '~1.5.4')).toBe(true)
  })

  test('rejects an unusable version or comparator', () => {
    expect(satisfies('not-a-version', '^1.0.0')).toBe(false)
    expect(satisfies('1.0.0', '>=abc')).toBe(false)
  })
})
