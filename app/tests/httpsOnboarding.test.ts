import type { HTTPSOnboardingEvent, HTTPSOnboardingState } from '@/views/site/site_edit/components/HTTPS/httpsOnboardingState'
import { describe, expect, test } from 'bun:test'
import {
  CONNECTION_CLOSED_MESSAGE,
  createHTTPSOnboardingState,
  diagnosticsWithoutHint,
  isValidHostname,
  parseHTTPSEvent,
  reduceHTTPSEvent,
} from '@/views/site/site_edit/components/HTTPS/httpsOnboardingState'

function run(events: HTTPSOnboardingEvent[], initial = createHTTPSOnboardingState('running')): HTTPSOnboardingState {
  return events.reduce(reduceHTTPSEvent, initial)
}

describe('reduceHTTPSEvent', () => {
  test('a successful HTTP-01 run records steps, logs and the result', () => {
    const state = run([
      { type: 'step', step: 'plan', status: 'running', message: 'Planning' },
      { type: 'step', step: 'plan', status: 'success' },
      { type: 'step', step: 'stage', status: 'running' },
      { type: 'step', step: 'stage', status: 'success' },
      { type: 'step', step: 'probe', status: 'running' },
      { type: 'step', step: 'probe', status: 'success' },
      { type: 'step', step: 'issue', status: 'running' },
      { type: 'log', message: '[INFO] acme: Obtaining bundled SAN certificate' },
      { type: 'step', step: 'issue', status: 'success' },
      { type: 'step', step: 'finalize', status: 'running' },
      { type: 'step', step: 'finalize', status: 'success' },
      {
        type: 'done',
        status: 'success',
        ssl_certificate: '/etc/nginx/ssl/app/fullchain.cer',
        ssl_certificate_key: '/etc/nginx/ssl/app/private.key',
        key_type: 'P256',
        profile: '',
        cert_id: 1,
      },
      // The server closes the socket after `done`; that must not turn into a failure.
      { type: 'socket_closed' },
    ])

    expect(state.phase).toBe('success')
    expect(state.error).toBeUndefined()
    expect(state.result).toEqual({
      ssl_certificate: '/etc/nginx/ssl/app/fullchain.cer',
      ssl_certificate_key: '/etc/nginx/ssl/app/private.key',
      key_type: 'P256',
      profile: '',
      cert_id: 1,
    })
    expect(state.steps.plan?.status).toBe('success')
    expect(state.steps.finalize?.status).toBe('success')
    expect(state.steps.rollback).toBeUndefined()
    expect(state.currentStep).toBe('finalize')
    expect(state.logs).toEqual([{ message: '[INFO] acme: Obtaining bundled SAN certificate', args: undefined }])
  })

  test('a probe failure keeps the hint and marks the failed step', () => {
    const hint = {
      code: 'dns_points_elsewhere',
      message: 'The domain %{domain} resolves to %{resolved}, not to this server (%{local}).',
      params: { domain: 'app.example.com', resolved: '10.0.0.2', local: '10.0.0.1' },
    }

    const state = run([
      { type: 'step', step: 'plan', status: 'success' },
      { type: 'step', step: 'stage', status: 'success' },
      { type: 'step', step: 'probe', status: 'running' },
      {
        type: 'diagnostic',
        level: 'warning',
        code: 'aaaa_without_ipv6_listen',
        message: 'AAAA record without an IPv6 listen',
        params: { domain: 'app.example.com', ip: '2001:db8::1' },
      },
      { type: 'done', status: 'error', step: 'probe', message: 'challenge route returned 404', hint },
      { type: 'socket_closed' },
    ])

    expect(state.phase).toBe('error')
    expect(state.steps.probe?.status).toBe('error')
    expect(state.steps.issue).toBeUndefined()
    expect(state.error).toEqual({
      step: 'probe',
      message: 'challenge route returned 404',
      args: undefined,
      hint,
    })
    expect(state.diagnostics).toEqual([{
      level: 'warning',
      code: 'aaaa_without_ipv6_listen',
      message: 'AAAA record without an IPv6 listen',
      params: { domain: 'app.example.com', ip: '2001:db8::1' },
    }])
  })

  test('done/error without a step falls back to the running step', () => {
    const state = run([
      { type: 'step', step: 'issue', status: 'running' },
      { type: 'done', status: 'error', message: 'acme: error: 403' },
    ])

    expect(state.error?.step).toBe('issue')
    expect(state.steps.issue?.status).toBe('error')
  })

  test('a server-reported rollback step is kept alongside the failure', () => {
    const state = run([
      { type: 'step', step: 'stage', status: 'error' },
      { type: 'step', step: 'rollback', status: 'success' },
      { type: 'done', status: 'error', step: 'stage', message: 'nginx -t failed' },
    ])

    expect(state.steps.stage?.status).toBe('error')
    expect(state.steps.rollback?.status).toBe('success')
  })

  test('a socket closed before done is a failure of the current step', () => {
    const state = run([
      { type: 'step', step: 'plan', status: 'success' },
      { type: 'step', step: 'stage', status: 'running' },
      { type: 'socket_closed' },
    ])

    expect(state.phase).toBe('error')
    expect(state.steps.stage?.status).toBe('error')
    expect(state.error).toEqual({
      code: 'connection_closed',
      step: 'stage',
      message: CONNECTION_CLOSED_MESSAGE,
    })
  })

  test('a socket closed before any step fails without a step', () => {
    const state = run([{ type: 'socket_closed' }])

    expect(state.phase).toBe('error')
    expect(state.error?.code).toBe('connection_closed')
    expect(state.error?.step).toBeUndefined()
    expect(state.steps).toEqual({})
  })

  test('events are ignored unless the run is in progress', () => {
    const idle = createHTTPSOnboardingState()

    expect(reduceHTTPSEvent(idle, { type: 'step', step: 'plan', status: 'running' })).toBe(idle)
    expect(reduceHTTPSEvent(idle, { type: 'socket_closed' })).toBe(idle)
  })

  test('unknown steps and statuses are ignored', () => {
    const initial = createHTTPSOnboardingState('running')
    const bogus = { type: 'step', step: 'deploy', status: 'running' } as unknown as HTTPSOnboardingEvent

    expect(reduceHTTPSEvent(initial, bogus)).toBe(initial)
  })

  test('the reducer does not mutate the previous state', () => {
    const initial = createHTTPSOnboardingState('running')
    reduceHTTPSEvent(initial, { type: 'log', message: 'hello' })
    reduceHTTPSEvent(initial, { type: 'step', step: 'plan', status: 'running' })

    expect(initial.logs).toEqual([])
    expect(initial.steps).toEqual({})
  })
})

describe('parseHTTPSEvent', () => {
  test('parses event frames and rejects everything else', () => {
    expect(parseHTTPSEvent('{"type":"log","message":"x"}')).toEqual({ type: 'log', message: 'x' })
    expect(parseHTTPSEvent('not json')).toBeUndefined()
    expect(parseHTTPSEvent('{"message":"no type"}')).toBeUndefined()
    expect(parseHTTPSEvent(new ArrayBuffer(1))).toBeUndefined()
  })
})

describe('isValidHostname', () => {
  test('accepts hostnames and rejects malformed ones', () => {
    expect(isValidHostname('example.com')).toBe(true)
    expect(isValidHostname('ab--cd.example.com')).toBe(true)
    expect(isValidHostname('例子.测试')).toBe(true)
    expect(isValidHostname('example.com.')).toBe(true)
    expect(isValidHostname('-bad.example.com')).toBe(false)
    expect(isValidHostname('bad..example.com')).toBe(false)
    expect(isValidHostname('bad_host.example.com')).toBe(false)
    expect(isValidHostname('https://example.com')).toBe(false)
  })

  test('accepts a leading wildcard only when allowed', () => {
    expect(isValidHostname('*.example.com')).toBe(false)
    expect(isValidHostname('*.example.com', true)).toBe(true)
    expect(isValidHostname('a.*.example.com', true)).toBe(false)
  })
})

describe('diagnosticsWithoutHint', () => {
  const diagnostics = [
    { level: 'warning', code: 'dns_points_elsewhere', message: 'The domain resolves elsewhere' },
    { level: 'warning', code: 'aaaa_without_ipv6_listen', message: 'AAAA without IPv6 listen' },
  ] as const

  test('drops the diagnostic the failure hint already explains', () => {
    const kept = diagnosticsWithoutHint([...diagnostics], 'dns_points_elsewhere')
    expect(kept.map(d => d.code)).toEqual(['aaaa_without_ipv6_listen'])
  })

  test('keeps every diagnostic without a hint', () => {
    expect(diagnosticsWithoutHint([...diagnostics])).toHaveLength(2)
    expect(diagnosticsWithoutHint([...diagnostics], 'port80_unreachable')).toHaveLength(2)
  })
})
