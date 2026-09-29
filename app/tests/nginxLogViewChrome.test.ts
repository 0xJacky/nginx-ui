import { describe, expect, test } from 'bun:test'
import { showsHostViewChrome } from '../src/views/nginx_log/viewChrome'

describe('showsHostViewChrome', () => {
  test('the built-in views but the structured one get the header', () => {
    expect(showsHostViewChrome('raw', undefined)).toBe(true)
    expect(showsHostViewChrome('dashboard', undefined)).toBe(true)
    expect(showsHostViewChrome('structured', undefined)).toBe(false)
  })

  test('a plugin view gets the header whatever its key', () => {
    expect(showsHostViewChrome('search', 'search')).toBe(true)
    expect(showsHostViewChrome('structured', 'structured')).toBe(true)
  })

  test('nothing is decided yet while plugins load', () => {
    expect(showsHostViewChrome('', undefined)).toBe(true)
  })
})
