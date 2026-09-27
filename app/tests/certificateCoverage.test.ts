import { describe, expect, test } from 'bun:test'
import {
  certificateCoverage,
  certificateNameCovers,
  isCertificateExpired,
} from '@/views/site/site_edit/components/HTTPS/certificateCoverage'

describe('certificateNameCovers', () => {
  test('an exact name covers itself, ignoring case and a trailing dot', () => {
    expect(certificateNameCovers('app.example.com', 'app.example.com')).toBe(true)
    expect(certificateNameCovers('App.Example.com.', 'app.example.COM')).toBe(true)
    expect(certificateNameCovers('app.example.com', 'www.example.com')).toBe(false)
  })

  test('a wildcard covers exactly one extra label', () => {
    expect(certificateNameCovers('*.example.com', 'app.example.com')).toBe(true)
    expect(certificateNameCovers('*.example.com', 'www.example.com')).toBe(true)
    expect(certificateNameCovers('*.example.com', 'example.com')).toBe(false)
    expect(certificateNameCovers('*.example.com', 'a.b.example.com')).toBe(false)
    expect(certificateNameCovers('*.example.com', 'app.example.org')).toBe(false)
    expect(certificateNameCovers('*.example.com', 'appexample.com')).toBe(false)
  })

  test('a requested wildcard is only covered by the same wildcard', () => {
    expect(certificateNameCovers('*.example.com', '*.example.com')).toBe(true)
    expect(certificateNameCovers('app.example.com', '*.example.com')).toBe(false)
    expect(certificateNameCovers('*.example.com', '*.app.example.com')).toBe(false)
  })

  test('IP addresses match exactly', () => {
    expect(certificateNameCovers('192.0.2.10', '192.0.2.10')).toBe(true)
    expect(certificateNameCovers('192.0.2.10', '192.0.2.11')).toBe(false)
  })

  test('punycode and Unicode forms of an IDN are the same name', () => {
    expect(certificateNameCovers('xn--mnchen-3ya.example', 'münchen.example')).toBe(true)
    expect(certificateNameCovers('*.xn--mnchen-3ya.example', 'www.münchen.example')).toBe(true)
  })

  test('empty names never cover anything', () => {
    expect(certificateNameCovers('', 'app.example.com')).toBe(false)
    expect(certificateNameCovers('app.example.com', ' ')).toBe(false)
  })
})

describe('certificateCoverage', () => {
  test('splits the requested domains into covered and uncovered ones', () => {
    expect(certificateCoverage(['example.com', '*.example.com'], ['example.com', 'www.example.com', 'a.b.example.com', 'other.org'])).toEqual({
      known: true,
      covered: ['example.com', 'www.example.com'],
      uncovered: ['a.b.example.com', 'other.org'],
    })
  })

  test('full coverage leaves nothing uncovered', () => {
    expect(certificateCoverage(['*.example.com'], ['app.example.com', 'www.example.com'])).toEqual({
      known: true,
      covered: ['app.example.com', 'www.example.com'],
      uncovered: [],
    })
  })

  test('a record without names cannot be judged client-side', () => {
    const unknown = { known: false, covered: [], uncovered: [] }
    expect(certificateCoverage([], ['app.example.com'])).toEqual(unknown)
    expect(certificateCoverage(undefined, ['app.example.com'])).toEqual(unknown)
    expect(certificateCoverage(['', ' '], ['app.example.com'])).toEqual(unknown)
  })
})

describe('isCertificateExpired', () => {
  const now = new Date('2026-09-27T12:00:00Z')

  test('a past expiry is expired, a future one is not', () => {
    expect(isCertificateExpired('2026-09-01T00:00:00Z', now)).toBe(true)
    expect(isCertificateExpired('2026-12-01T00:00:00Z', now)).toBe(false)
  })

  test('an unknown expiry is not treated as expired', () => {
    expect(isCertificateExpired(undefined, now)).toBe(false)
    expect(isCertificateExpired('', now)).toBe(false)
    expect(isCertificateExpired('not a date', now)).toBe(false)
    // Go's zero time.Time: the backend could not read the certificate file.
    expect(isCertificateExpired('0001-01-01T00:00:00Z', now)).toBe(false)
  })
})
