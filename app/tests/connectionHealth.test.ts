import { describe, expect, test } from 'bun:test'
import { reconnectBackoffMs, resolveConnectionRecovery } from '@/lib/websocket/connectionHealth'

const NOW = 1_700_000_000_000
const STALE_AFTER_MS = 20_000

function health(overrides: Partial<Parameters<typeof resolveConnectionRecovery>[0]> = {}) {
  return resolveConnectionRecovery({
    hasSubscribers: true,
    readyState: WebSocket.OPEN,
    lastMessageAt: NOW - 1000,
    now: NOW,
    staleAfterMs: STALE_AFTER_MS,
    ...overrides,
  })
}

describe('shared connection health', () => {
  test('leaves the connection alone when nobody subscribes', () => {
    expect(health({ hasSubscribers: false, readyState: undefined })).toBe('none')
    expect(health({ hasSubscribers: false, lastMessageAt: NOW - 60_000 })).toBe('none')
  })

  test('waits for a handshake in flight', () => {
    expect(health({ readyState: WebSocket.CONNECTING, lastMessageAt: 0 })).toBe('none')
  })

  test('reconnects a socket that died under its subscribers', () => {
    // autoReconnect exhausted its retries, or the store never dialled.
    expect(health({ readyState: undefined, lastMessageAt: 0 })).toBe('connect')
    expect(health({ readyState: WebSocket.CLOSED })).toBe('connect')
    expect(health({ readyState: WebSocket.CLOSING })).toBe('connect')
  })

  test('keeps a socket that is still delivering', () => {
    expect(health({ lastMessageAt: NOW - STALE_AFTER_MS })).toBe('none')
  })

  test('rebuilds a half-open socket that went silent', () => {
    // readyState still reads OPEN: only the missing traffic gives it away.
    expect(health({ lastMessageAt: NOW - STALE_AFTER_MS - 1 })).toBe('reopen')
  })

  test('does not judge a socket that has not reported yet', () => {
    expect(health({ lastMessageAt: 0 })).toBe('none')
  })
})

describe('supervised reconnect backoff', () => {
  test('retries immediately before any failure', () => {
    expect(reconnectBackoffMs(0, 15_000, 300_000)).toBe(0)
  })

  test('doubles per failed attempt up to the cap', () => {
    expect(reconnectBackoffMs(1, 15_000, 300_000)).toBe(15_000)
    expect(reconnectBackoffMs(2, 15_000, 300_000)).toBe(30_000)
    expect(reconnectBackoffMs(5, 15_000, 300_000)).toBe(240_000)
    expect(reconnectBackoffMs(6, 15_000, 300_000)).toBe(300_000)
    expect(reconnectBackoffMs(50, 15_000, 300_000)).toBe(300_000)
  })
})
