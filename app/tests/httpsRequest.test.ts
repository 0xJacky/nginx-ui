import type { AutoCertOptions } from '@/api/auto_cert'
import type { HTTPSRequestForm } from '@/views/site/site_edit/components/HTTPS/httpsRequest'
import { describe, expect, test } from 'bun:test'
import {
  buildHTTPSCheckRequest,
  buildHTTPSRequest,
  isChallengeMethod,
} from '@/views/site/site_edit/components/HTTPS/httpsRequest'

function options(overrides: Partial<AutoCertOptions> = {}): AutoCertOptions {
  return {
    domains: [],
    challenge_method: 'http01',
    key_type: 'P256',
    dns_credential_id: 7,
    acme_user_id: 3,
    profile: '',
    must_staple: false,
    challenge_config: { disable_authoritative_ns_propagation: true },
    enable_common_name: false,
    revoke_old: false,
    ...overrides,
  }
}

function form(overrides: Partial<HTTPSRequestForm> = {}): HTTPSRequestForm {
  return {
    method: 'http01',
    domains: ['app.example.com'],
    redirectHTTPToHTTPS: true,
    options: options(),
    ...overrides,
  }
}

describe('buildHTTPSRequest', () => {
  test('HTTP-01 drops the DNS-only fields and never sends a certificate id', () => {
    const request = buildHTTPSRequest(form({ certificateId: 5 }))

    expect(request.challenge_method).toBe('http01')
    expect(request.dns_credential_id).toBe(0)
    expect(request.challenge_config).toBeUndefined()
    expect(request.acme_user_id).toBe(3)
    expect('certificate_id' in request).toBe(false)
  })

  test('DNS-01 keeps the credential and the propagation option', () => {
    const request = buildHTTPSRequest(form({ method: 'dns01' }))

    expect(request.challenge_method).toBe('dns01')
    expect(request.dns_credential_id).toBe(7)
    expect(request.challenge_config).toEqual({ disable_authoritative_ns_propagation: true })
  })

  test('an existing certificate sends its id with a neutral challenge', () => {
    const request = buildHTTPSRequest(form({ method: 'existing', certificateId: 5, redirectHTTPToHTTPS: false }))

    expect(request.certificate_id).toBe(5)
    expect(request.challenge_method).toBe('http01')
    expect(request.dns_credential_id).toBe(0)
    expect(request.redirect_http_to_https).toBe(false)
  })

  test('the existing method without a picked certificate sends no id', () => {
    expect('certificate_id' in buildHTTPSRequest(form({ method: 'existing' }))).toBe(false)
  })

  test('the domains are copied, not shared', () => {
    const domains = ['app.example.com']
    const request = buildHTTPSRequest(form({ domains }))
    domains.push('www.example.com')

    expect(request.domains).toEqual(['app.example.com'])
  })
})

describe('buildHTTPSCheckRequest', () => {
  test('carries the certificate id only when one is used', () => {
    expect(buildHTTPSCheckRequest(buildHTTPSRequest(form({ method: 'existing', certificateId: 9 })))).toEqual({
      domains: ['app.example.com'],
      challenge_method: 'http01',
      certificate_id: 9,
    })
    expect(buildHTTPSCheckRequest(buildHTTPSRequest(form({ method: 'dns01' })))).toEqual({
      domains: ['app.example.com'],
      challenge_method: 'dns01',
    })
  })
})

describe('isChallengeMethod', () => {
  test('only ACME challenges are challenge methods', () => {
    expect(isChallengeMethod('http01')).toBe(true)
    expect(isChallengeMethod('dns01')).toBe(true)
    expect(isChallengeMethod('existing')).toBe(false)
    expect(isChallengeMethod('skip')).toBe(false)
  })
})
