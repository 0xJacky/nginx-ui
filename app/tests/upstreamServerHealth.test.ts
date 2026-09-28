import { describe, expect, test } from 'bun:test'
import { serverHealth } from '@/views/upstream/serverHealth'

// Availability results as the health checker reports them: keyed by the socket
// it probes, with port 80 filled in for servers written without one.
const results = {
  'web.internal:80': { online: true, latency: 1.5 },
  '10.0.0.1:80': { online: true, latency: 0.8 },
  '10.0.0.2:8080': { online: false, latency: 0 },
  '[::1]:80': { online: true, latency: 0.2 },
  '[::1]:8080': { online: true, latency: 0.3 },
  'unix:/run/app.sock': { online: true, latency: 0.1 },
  'nohost.invalid:80': { online: false, latency: 0 },
}

describe('serverHealth', () => {
  test('looks servers without a port up by their resolved socket', () => {
    expect(serverHealth(false, 'web.internal:80', results)).toEqual({ state: 'online', result: results['web.internal:80'] })
    expect(serverHealth(false, '10.0.0.1:80', results).state).toBe('online')
    expect(serverHealth(false, '[::1]:80', results).state).toBe('online')
  })

  test('keeps explicit ports and unix sockets as they are', () => {
    expect(serverHealth(false, '10.0.0.2:8080', results).state).toBe('offline')
    expect(serverHealth(false, '[::1]:8080', results).state).toBe('online')
    expect(serverHealth(false, 'unix:/run/app.sock', results).state).toBe('online')
  })

  test('reports a hostname the checker cannot resolve as offline, not unknown', () => {
    expect(serverHealth(false, 'nohost.invalid:80', results).state).toBe('offline')
  })

  test('never matches the address as written when it lacks a port', () => {
    // The bug this guards against: `web.internal` is not a key of the results.
    expect(serverHealth(false, 'web.internal', results).state).toBe('unknown')
    expect(serverHealth(false, '[::1]', results).state).toBe('unknown')
  })

  test('reports unknown when the backend resolved no socket', () => {
    expect(serverHealth(false, undefined, results).state).toBe('unknown')
    expect(serverHealth(false, '', results).state).toBe('unknown')
    expect(serverHealth(false, '10.9.9.9:80', results).state).toBe('unknown')
  })

  test('reports a disabled server as disabled whatever its probe says', () => {
    expect(serverHealth(true, 'web.internal:80', results)).toEqual({ state: 'disabled' })
    expect(serverHealth(true, 'nohost.invalid:80', results)).toEqual({ state: 'disabled' })
  })
})
