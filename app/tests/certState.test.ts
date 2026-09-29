import { describe, expect, test } from 'bun:test'

function fakeGettext(message: string, args?: Record<string, unknown>) {
  return message.replace(/%\{(\w+)\}/g, (_, key: string) => String(args?.[key] ?? `%{${key}}`))
}
Object.assign(globalThis, { $gettext: fakeGettext })

const { certRenewalMethodLabel, certStateLabel, certStateTone, isAcmeCert, splitVisible }
  = await import('../src/views/certificate/certState')

describe('certificate state', () => {
  test('labels carry the remaining days', () => {
    expect(certStateLabel({ state: 'valid', days_left: 62, certificate_info: undefined as never })).toBe('Valid · 62 days left')
    expect(certStateLabel({ state: 'expiring', days_left: 12, certificate_info: undefined as never })).toBe('Expires in 12 days')
  })

  test('a certificate that never finished issuing is not called expired', () => {
    expect(certStateLabel({ state: 'not_issued', certificate_info: undefined as never })).toBe('Not issued yet')
    expect(certStateLabel({ state: 'failed', certificate_info: undefined as never })).toBe('Issuance failed')
    expect(certStateLabel({ state: 'failed', certificate_info: { not_after: '2026-10-01' } as never })).toBe('Renewal failed')
  })

  test('tones follow the severity', () => {
    expect(certStateTone('valid')).toBe('success')
    expect(certStateTone('expiring')).toBe('warning')
    expect(certStateTone('failed')).toBe('error')
    expect(certStateTone('expired')).toBe('error')
    expect(certStateTone('manual')).toBe('default')
    expect(certStateTone(undefined)).toBe('default')
  })

  test('renewal method names the DNS provider', () => {
    expect(certRenewalMethodLabel('dns01', 'Cloudflare')).toBe('DNS validation · Cloudflare')
    expect(certRenewalMethodLabel('dns01')).toBe('DNS validation')
    expect(certRenewalMethodLabel('http01')).toBe('HTTP validation')
    expect(certRenewalMethodLabel('upload')).toBe('Upload')
    expect(certRenewalMethodLabel(undefined)).toBe('Upload')
  })

  test('only certificates with a challenge method are ACME certificates', () => {
    expect(isAcmeCert({ auto_cert: 1, challenge_method: 'dns01' })).toBe(true)
    expect(isAcmeCert({ auto_cert: -1, challenge_method: 'http01' })).toBe(true)
    expect(isAcmeCert({ auto_cert: -1, challenge_method: '' as never })).toBe(false)
    expect(isAcmeCert({ auto_cert: 3, challenge_method: 'http01' })).toBe(false)
  })

  test('splitVisible keeps the first items and counts the rest', () => {
    expect(splitVisible(['a', 'b', 'c', 'd'], 2)).toEqual({ visible: ['a', 'b'], hidden: 2 })
    expect(splitVisible(['a'], 2)).toEqual({ visible: ['a'], hidden: 0 })
  })
})
