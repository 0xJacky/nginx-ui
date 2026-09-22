/**
 * Liveness policy for a shared connection.
 *
 * Reference counting answers "does anyone still want this data", never "is the
 * connection still delivering it". A socket can die under its subscribers —
 * backend restart, laptop sleep, a proxy dropping an idle tunnel, or VueUse's
 * autoReconnect giving up after its retry budget — and the reference count will
 * happily stay above zero while the UI quietly freezes. Something therefore has
 * to watch the connection itself and bring it back while subscribers remain.
 */

export type ConnectionRecovery
  // The connection is healthy, or nobody is watching it.
  = | 'none'
  // There is no live socket: open one.
    | 'connect'
  // The socket still reads as open but has gone quiet: tear it down and
  // reconnect. Half-open TCP connections never surface as a close event.
    | 'reopen'

export interface ConnectionHealthInput {
  /** Whether any consumer still needs the data. */
  hasSubscribers: boolean
  /** WebSocket.readyState, or undefined when no socket exists. */
  readyState: number | undefined
  /** Timestamp of the last frame received (or of the last connect), 0 when unknown. */
  lastMessageAt: number
  /** Current timestamp. */
  now: number
  /** An open socket silent for longer than this is treated as half-open. */
  staleAfterMs: number
}

// Local copies of the WebSocket readyState values keep this module usable
// outside a browser (unit tests, SSR).
const READY_STATE_CONNECTING = 0
const READY_STATE_OPEN = 1

export function resolveConnectionRecovery(input: ConnectionHealthInput): ConnectionRecovery {
  const { hasSubscribers, readyState, lastMessageAt, now, staleAfterMs } = input

  // Without subscribers the connection is supposed to be closed.
  if (!hasSubscribers) {
    return 'none'
  }

  // A handshake in flight is not a failure; give it time to finish.
  if (readyState === READY_STATE_CONNECTING) {
    return 'none'
  }

  if (readyState !== READY_STATE_OPEN) {
    return 'connect'
  }

  // Nothing received yet and no connect timestamp: too early to judge.
  if (lastMessageAt <= 0) {
    return 'none'
  }

  return now - lastMessageAt > staleAfterMs ? 'reopen' : 'none'
}

/**
 * Delay before the next supervised reconnect after `failures` attempts that
 * did not bring the connection back.
 *
 * Each supervised attempt restarts VueUse's own retry burst, so retrying at a
 * fixed interval would keep hammering a backend that is down for good. Doubling
 * from `baseMs` up to `maxMs` keeps recovery quick after a short outage while
 * an hour-long one costs a handful of attempts.
 */
export function reconnectBackoffMs(failures: number, baseMs: number, maxMs: number): number {
  if (failures <= 0) {
    return 0
  }

  return Math.min(baseMs * 2 ** (failures - 1), maxMs)
}
