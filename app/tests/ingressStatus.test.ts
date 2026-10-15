import type { Ingress } from '@/api/ingress'
import { describe, expect, test } from 'bun:test'
import {
  backendHealth,
  backendHealthColor,
  certExpiry,
  filterIngresses,
  hostUrl,
  isGenerationBehind,
  ownersOf,
  phaseColor,
} from '@/views/ingress/ingressStatus'

function makeIngress(overrides: Partial<Ingress> = {}): Ingress {
  return {
    id: 1,
    name: 'tm-prod-ocr',
    owner: 'telemetry-prod',
    external_id: 'mac-service:42',
    labels: {},
    namespace_id: 0,
    generation: 3,
    site_path: 'ingress-tm-prod-ocr',
    spec: {
      hosts: ['ocr.example.com'],
      backends: [{
        name: 'app',
        servers: [
          { address: '127.0.0.1:24001', down: false, backup: false },
          { address: '127.0.0.1:24002', down: true, backup: false },
        ],
      }],
      routes: [{ path_prefix: '/', backend: 'app', websocket: true }],
      tls: { mode: 'acme', redirect_https: true, hsts: false },
      policy: {},
    },
    status: {
      observed_generation: 3,
      phase: 'Ready',
      conditions: [],
      cert_not_after: '2027-01-13T08:01:00Z',
      servers: [
        { backend: 'app', address: '127.0.0.1:24001', down: false, healthy: true },
        { backend: 'app', address: '127.0.0.1:24002', down: true, healthy: false },
      ],
    },
    created_at: '2026-10-15T07:59:58Z',
    updated_at: '2026-10-15T08:01:30Z',
    ...overrides,
  }
}

describe('phaseColor', () => {
  test('maps every phase and falls back to default', () => {
    expect(phaseColor('Ready')).toBe('success')
    expect(phaseColor('Degraded')).toBe('warning')
    expect(phaseColor('Error')).toBe('error')
    expect(phaseColor('Applying')).toBe('processing')
    expect(phaseColor('Pending')).toBe('default')
    expect(phaseColor('Unknown')).toBe('default')
    expect(phaseColor(undefined)).toBe('default')
  })
})

describe('isGenerationBehind', () => {
  test('is true until the latest generation is applied', () => {
    expect(isGenerationBehind(makeIngress())).toBe(false)
    expect(isGenerationBehind(makeIngress({ generation: 4 }))).toBe(true)
  })
})

describe('backendHealth', () => {
  test('counts healthy servers that are not drained', () => {
    const summary = backendHealth(makeIngress())
    expect(summary).toEqual({ healthy: 1, total: 2, down: 1, known: true })
    expect(backendHealthColor(summary)).toBe('success')
  })

  test('falls back to the spec before the checker reports', () => {
    const ingress = makeIngress()
    ingress.status.servers = null
    const summary = backendHealth(ingress)
    expect(summary).toEqual({ healthy: 0, total: 2, down: 1, known: false })
    expect(backendHealthColor(summary)).toBe('default')
  })

  test('flags partial and total outages', () => {
    expect(backendHealthColor({ healthy: 1, total: 3, down: 0, known: true })).toBe('warning')
    expect(backendHealthColor({ healthy: 0, total: 2, down: 0, known: true })).toBe('error')
    expect(backendHealthColor({ healthy: 0, total: 2, down: 2, known: true })).toBe('default')
  })
})

describe('certExpiry', () => {
  const now = new Date('2026-10-15T00:00:00Z').getTime()

  test('is none for plain HTTP', () => {
    const ingress = makeIngress()
    ingress.spec.tls.mode = 'none'
    expect(certExpiry(ingress, now).state).toBe('none')
  })

  test('is pending until the certificate exists', () => {
    const ingress = makeIngress()
    ingress.status.cert_not_after = null
    expect(certExpiry(ingress, now).state).toBe('pending')
  })

  test('reports the days left', () => {
    expect(certExpiry(makeIngress(), now)).toMatchObject({ state: 'valid', daysLeft: 90 })

    const expiring = makeIngress()
    expiring.status.cert_not_after = '2026-10-20T00:00:00Z'
    expect(certExpiry(expiring, now)).toMatchObject({ state: 'expiring', daysLeft: 5 })

    const expired = makeIngress()
    expired.status.cert_not_after = '2026-10-01T00:00:00Z'
    expect(certExpiry(expired, now).state).toBe('expired')
  })
})

describe('hostUrl', () => {
  test('uses HTTPS once the certificate is in place', () => {
    expect(hostUrl(makeIngress(), 'ocr.example.com')).toBe('https://ocr.example.com')

    const pending = makeIngress()
    pending.status.cert_not_after = null
    expect(hostUrl(pending, 'ocr.example.com')).toBe('http://ocr.example.com')
  })

  test('drops the wildcard label', () => {
    expect(hostUrl(makeIngress(), '*.example.com')).toBe('https://example.com')
  })
})

describe('filterIngresses', () => {
  const list = [
    makeIngress(),
    makeIngress({ name: 'tm-dev-api', owner: 'telemetry-dev', external_id: 'svc:7', spec: { ...makeIngress().spec, hosts: ['api.dev.example.com'] } }),
  ]

  test('lists owners once, sorted', () => {
    expect(ownersOf([...list, makeIngress()])).toEqual(['telemetry-dev', 'telemetry-prod'])
  })

  test('filters by owner and by name, host or external ID', () => {
    expect(filterIngresses(list, 'telemetry-dev', '').map(i => i.name)).toEqual(['tm-dev-api'])
    expect(filterIngresses(list, '', 'OCR.example').map(i => i.name)).toEqual(['tm-prod-ocr'])
    expect(filterIngresses(list, '', 'svc:7').map(i => i.name)).toEqual(['tm-dev-api'])
    expect(filterIngresses(list, 'telemetry-prod', 'api')).toEqual([])
    expect(filterIngresses(list, '', '  ')).toHaveLength(2)
  })
})
