import { describe, expect, test } from 'bun:test'
import { decodeIdnLabel, isIdnDomain, toUnicodeDomain } from '../src/utils/idnDomain'

describe('toUnicodeDomain', () => {
  // Expected values cross-checked against golang.org/x/net/idna, which is what the
  // backend uses to produce the stored ASCII form.
  test.each([
    ['xn--fsq.example.com', '例.example.com'],
    ['xn--fsq.xn--fiqs8s', '例.中国'],
    ['xn--mnchen-3ya.de', 'münchen.de'],
    ['xn--fa-hia.de', 'faß.de'],
    ['xn--53h.example.com', '☕.example.com'],
    ['xn--4ca0bs.example.com', 'äöü.example.com'],
    ['www.xn--fsq.example.com', 'www.例.example.com'],
    ['*.xn--fsq.example.com', '*.例.example.com'],
    ['_acme-challenge.xn--fsq.example.com', '_acme-challenge.例.example.com'],
  ])('decodes %s to %s', (input, expected) => {
    expect(toUnicodeDomain(input)).toBe(expected)
  })

  test.each([
    ['example.com'],
    ['www.example.com'],
    ['ab--cd.example.com'],
    ['a--b.example.com'],
    ['@'],
    ['*'],
  ])('leaves %s untouched', input => {
    expect(toUnicodeDomain(input)).toBe(input)
  })

  test('returns an empty string for blank input', () => {
    expect(toUnicodeDomain('')).toBe('')
    expect(toUnicodeDomain(undefined)).toBe('')
    expect(toUnicodeDomain(null)).toBe('')
  })

  test('keeps labels that only look like punycode', () => {
    // Not decodable: the payload holds characters outside the punycode digit set.
    expect(toUnicodeDomain('xn--@@@.example.com')).toBe('xn--@@@.example.com')
    // An ACE prefix with an empty payload is not a valid encoding.
    expect(toUnicodeDomain('xn--.example.com')).toBe('xn--.example.com')
  })

  test('is case insensitive about the ACE prefix', () => {
    expect(toUnicodeDomain('XN--FSQ.example.com')).toBe('例.example.com')
  })

  test('round-trips every label of a multi-label IDN', () => {
    expect(toUnicodeDomain('xn--fsq.xn--fsq.xn--fiqs8s')).toBe('例.例.中国')
  })
})

describe('decodeIdnLabel', () => {
  test('decodes a single label', () => {
    expect(decodeIdnLabel('xn--fsq')).toBe('例')
  })

  test('passes through a plain label', () => {
    expect(decodeIdnLabel('www')).toBe('www')
  })
})

describe('isIdnDomain', () => {
  test('is true only when display differs from the stored form', () => {
    expect(isIdnDomain('xn--fsq.example.com')).toBe(true)
    expect(isIdnDomain('example.com')).toBe(false)
    expect(isIdnDomain('')).toBe(false)
    expect(isIdnDomain(undefined)).toBe(false)
  })
})
