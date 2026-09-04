import { describe, expect, test } from 'bun:test'
import { createTokenExpiryMonitor, getTokenExpiration, isTokenExpired } from '../src/lib/auth/tokenExpiry'

function token(exp: unknown) {
  return `header.${btoa(JSON.stringify({ exp, name: 'test' })).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')}.signature`
}

function fixture(initialToken: string, initialTime = 100_000) {
  let currentToken = initialToken
  let now = initialTime
  const expired: string[] = []
  const jobs: { callback: () => void, delay: number, cancelled: boolean }[] = []
  const monitor = createTokenExpiryMonitor(() => currentToken, value => {
    expired.push(value)
    currentToken = ''
  }, {
    now: () => now,
    schedule(callback, delay) {
      const job = { callback, delay, cancelled: false }
      jobs.push(job)
      return () => {
        job.cancelled = true
      }
    },
  })
  return {
    monitor,
    jobs,
    expired,
    setToken: (value: string) => { currentToken = value },
    setTime: (value: number) => { now = value },
  }
}

describe('JWT expiry', () => {
  test('reads seconds as milliseconds and expires exactly at the deadline', () => {
    expect(getTokenExpiration(token(101))).toBe(101_000)
    expect(isTokenExpired(token(101), 100_999)).toBe(false)
    expect(isTokenExpired(token(101), 101_000)).toBe(true)
  })

  test('leaves opaque, malformed and missing-exp credentials to server validation', () => {
    for (const value of ['', 'opaque-token', 'x.?.y', 'x.bnVsbA.y', token(undefined), token('101'), token(1e308)]) {
      expect(getTokenExpiration(value)).toBeNull()
      expect(isTokenExpired(value)).toBe(false)
    }
  })

  test('expires a persisted token on the initial check', () => {
    const f = fixture(token(99))
    f.monitor.check()
    expect(f.expired).toEqual([token(99)])
    expect(f.jobs).toHaveLength(0)
  })

  test('expires an idle session without another HTTP request', () => {
    const f = fixture(token(101))
    f.monitor.check()
    expect(f.jobs[0].delay).toBe(1000)
    f.setTime(101_000)
    f.jobs[0].callback()
    expect(f.expired).toEqual([token(101)])
    f.monitor.check()
    expect(f.expired).toHaveLength(1)
  })

  test('resume checks catch expiry even if the timer never ran', () => {
    const f = fixture(token(200))
    f.monitor.check()
    f.setTime(300_000)
    f.monitor.check()
    expect(f.jobs[0].cancelled).toBe(true)
    expect(f.expired).toEqual([token(200)])
  })

  test('a queued old timer cannot expire a replacement session', () => {
    const f = fixture(token(101))
    f.monitor.check()
    const oldTimer = f.jobs[0]
    f.setToken(token(300))
    f.monitor.check()
    expect(oldTimer.cancelled).toBe(true)
    f.setTime(102_000)
    oldTimer.callback()
    expect(f.expired).toHaveLength(0)
  })

  test('logout and disposal cancel pending work', () => {
    const f = fixture(token(200))
    f.monitor.check()
    f.setToken('')
    f.monitor.check()
    expect(f.jobs[0].cancelled).toBe(true)
    f.setToken(token(200))
    f.monitor.check()
    f.monitor.stop()
    f.setTime(300_000)
    f.jobs.at(-1)!.callback()
    expect(f.expired).toHaveLength(0)
  })

  test('bounds timers for distant expirations and clock adjustments', () => {
    const f = fixture(token(5_000_000_000))
    f.monitor.check()
    expect(f.jobs[0].delay).toBe(60_000)
    f.setTime(50_000)
    f.jobs[0].callback()
    expect(f.jobs.at(-1)!.delay).toBe(60_000)
    expect(f.expired).toHaveLength(0)
  })
})
