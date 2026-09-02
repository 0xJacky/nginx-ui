import type { RecoveryAction } from './recovery'
import { expect, test } from 'bun:test'
import {
  APP_BOOT_GRACE_MS,
  clearUnserved,
  decideRecovery,
  freshWedgeState,
  markServing,
  RECOVERY_COOLDOWN_MS,
} from './recovery'

// A clock that starts small on purpose: it catches a cooldown check that
// mistakes "never recovered" (lastRecoveryAt === 0) for "recovered at time 0".
const T0 = 1_000

test('the first observation only starts the clock', () => {
  const { action, next } = decideRecovery(freshWedgeState(), T0)

  expect(action).toBe('wait')
  expect(next.notServingSince).toBe(T0)
  expect(next.attempts).toBe(0)
})

test('a container inside the boot window is left alone', () => {
  const started = decideRecovery(freshWedgeState(), T0).next
  const { action } = decideRecovery(started, T0 + APP_BOOT_GRACE_MS - 1)

  // Stopping a container that is merely slow to boot is the failure mode this
  // whole grace period exists to prevent.
  expect(action).toBe('wait')
})

test('a container past the boot window is stopped, not killed', () => {
  const started = decideRecovery(freshWedgeState(), T0).next
  const { action, next, unservedFor } = decideRecovery(started, T0 + APP_BOOT_GRACE_MS)

  expect(action).toBe('stop')
  expect(unservedFor).toBe(APP_BOOT_GRACE_MS)
  expect(next.attempts).toBe(1)
  expect(next.lastRecoveryAt).toBe(T0 + APP_BOOT_GRACE_MS)
  // The clock restarts, so the replacement container gets a full boot window.
  expect(next.notServingSince).toBeUndefined()
})

test('the cooldown outlasts the boot window, so a restart is never interrupted', () => {
  const stoppedAt = T0 + APP_BOOT_GRACE_MS
  const afterStop = decideRecovery(decideRecovery(freshWedgeState(), T0).next, stoppedAt).next

  // The replacement container comes up and is still not serving a full boot
  // window later. Signalling it again here would cut short the start it is
  // already attempting.
  const during = decideRecovery(afterStop, stoppedAt + APP_BOOT_GRACE_MS)
  expect(during.action).toBe('wait')

  expect(APP_BOOT_GRACE_MS).toBeLessThan(RECOVERY_COOLDOWN_MS)
})

/**
 * Replay the loading page's polling loop against a container that never
 * serves, and report every recovery it would issue.
 *
 * Testing the real call pattern rather than one decision at a time is what
 * catches a rate limit that looks right in isolation but lets a poll every few
 * seconds slip past it.
 */
function replay(durationMs: number, pollMs = 4_000) {
  let state = freshWedgeState()
  const issued: Array<{ at: number, action: RecoveryAction }> = []

  for (let now = T0; now <= T0 + durationMs; now += pollMs) {
    const decision = decideRecovery(state, now)
    state = decision.next
    if (decision.action !== 'wait') {
      issued.push({ at: now - T0, action: decision.action })
    }
  }

  return issued
}

test('a stop that changed nothing escalates to a kill', () => {
  const [first, second] = replay(10 * 60_000)

  expect(first.action).toBe('stop')
  expect(second.action).toBe('destroy')
})

test('every recovery after the first is a kill', () => {
  const issued = replay(30 * 60_000)

  expect(issued.length).toBeGreaterThan(2)
  expect(issued.slice(1).every(r => r.action === 'destroy')).toBe(true)
})

test('the first recovery waits out one boot window and no longer', () => {
  const [first] = replay(10 * 60_000)

  expect(first.at).toBeGreaterThanOrEqual(APP_BOOT_GRACE_MS)
  expect(first.at).toBeLessThan(APP_BOOT_GRACE_MS + 4_000)
})

test('polling every few seconds cannot outrun the rate limit', () => {
  // The property that matters: whatever the poll rate, consecutive recoveries
  // stay a full cooldown apart — which is longer than a boot window, so a
  // container that is starting is never signalled part-way through.
  const WINDOW_MS = 30 * 60_000

  for (const pollMs of [250, 1_000, 4_000]) {
    const issued = replay(WINDOW_MS, pollMs)

    for (let i = 1; i < issued.length; i++) {
      expect(issued[i].at - issued[i - 1].at).toBeGreaterThanOrEqual(RECOVERY_COOLDOWN_MS)
    }

    expect(issued.length).toBeLessThanOrEqual(Math.ceil(WINDOW_MS / RECOVERY_COOLDOWN_MS))
  }
})

test('serving again forgets the cooldown as well as the clock', () => {
  const served = markServing()

  expect(served).toEqual(freshWedgeState())

  // So the next wedge, whenever it comes, starts over with a polite stop.
  const started = decideRecovery(served, T0).next
  expect(decideRecovery(started, T0 + APP_BOOT_GRACE_MS).action).toBe('stop')
})

test('a stopping container clears the clock but stays inside its cooldown', () => {
  const stoppedAt = T0 + APP_BOOT_GRACE_MS
  const afterStop = decideRecovery(decideRecovery(freshWedgeState(), T0).next, stoppedAt).next

  const cleared = clearUnserved(afterStop)

  expect(cleared.notServingSince).toBeUndefined()
  expect(cleared.lastRecoveryAt).toBe(stoppedAt)
  expect(cleared.attempts).toBe(1)
})

test('a wedge detected long after the last recovery is acted on immediately', () => {
  // The Durable Object is evicted between incidents, but when it is not, a
  // recovery from hours ago must not be mistaken for one still in flight.
  const stale = { lastRecoveryAt: T0, attempts: 1 }
  const started = decideRecovery(stale, T0 + 3_600_000).next

  expect(decideRecovery(started, T0 + 3_600_000 + APP_BOOT_GRACE_MS).action).toBe('destroy')
})
