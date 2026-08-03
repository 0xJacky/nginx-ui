/**
 * When to tear down a container that the platform insists is healthy.
 *
 * Kept separate from the Durable Object because getting this wrong is
 * expensive in both directions — too eager and the demo is stopped mid-boot
 * every cooldown, too reluctant and it stays down for as long as nobody
 * happens to look — and because a pure decision can be tested without a
 * container binding.
 */

/**
 * How long a container that reports healthy gets to actually serve before it
 * is treated as wedged rather than merely slow.
 *
 * Generous on purpose. nginx binds 8080 within a second of container start, so
 * 'healthy' arrives long before nginx-ui has opened its database and finished
 * indexing; a fresh boot has been seen to take most of a minute.
 */
export const APP_BOOT_GRACE_MS = 75_000

/** Minimum gap between forced recoveries, so a container that dies on boot is not thrashed. */
export const RECOVERY_COOLDOWN_MS = 120_000

export interface WedgeState {
  /** When the container first reported healthy without serving, if it has. */
  notServingSince?: number
  /** When the last recovery was issued. 0 means none has been. */
  lastRecoveryAt: number
  /** Recoveries issued since the container was last seen serving. */
  attempts: number
}

/**
 * - `wait`    — inside the boot window, or inside the cooldown after a recovery
 * - `stop`    — SIGTERM, the ordered shutdown a container that is merely stuck deserves
 * - `destroy` — SIGKILL, for when a stop has already been sent and changed nothing
 */
export type RecoveryAction = 'wait' | 'stop' | 'destroy'

export interface RecoveryDecision {
  action: RecoveryAction
  next: WedgeState
  /** How long the container has reported healthy without serving, in ms. */
  unservedFor: number
}

export function freshWedgeState(): WedgeState {
  return { lastRecoveryAt: 0, attempts: 0 }
}

/**
 * Forget that the container was ever unserved, without forgetting the
 * cooldown. Used when the container stops: the clock restarts, but a recovery
 * issued moments ago must still hold off the next one.
 */
export function clearUnserved(state: WedgeState): WedgeState {
  return { ...state, notServingSince: undefined }
}

/** The container served. Nothing about the previous failure applies any more. */
export function markServing(): WedgeState {
  return freshWedgeState()
}

/**
 * Decide what to do about a container reporting healthy that is not serving.
 *
 * Call once per observation. The returned state must be stored, because the
 * first call is what starts the boot-window clock.
 */
export function decideRecovery(state: WedgeState, now: number): RecoveryDecision {
  const notServingSince = state.notServingSince ?? now
  const unservedFor = now - notServingSince

  if (unservedFor < APP_BOOT_GRACE_MS) {
    return { action: 'wait', unservedFor, next: { ...state, notServingSince } }
  }

  // A recovery was issued recently and the container has not come back yet.
  // Sending another signal now would only interrupt the start it is already
  // attempting — which is how an earlier version of this turned a slow boot
  // into a page that refreshed forever.
  if (state.lastRecoveryAt !== 0 && now - state.lastRecoveryAt < RECOVERY_COOLDOWN_MS) {
    return { action: 'wait', unservedFor, next: { ...state, notServingSince } }
  }

  const attempts = state.attempts + 1

  return {
    // A second round means the SIGTERM went nowhere, so the process it was
    // aimed at is already gone and only a kill will reconcile the record.
    action: attempts > 1 ? 'destroy' : 'stop',
    unservedFor,
    next: { notServingSince: undefined, lastRecoveryAt: now, attempts },
  }
}
