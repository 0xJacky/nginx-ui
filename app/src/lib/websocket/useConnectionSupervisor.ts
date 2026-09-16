// Imports are explicit (no auto-import) so the supervisor runs under bun:test.
import type { UseWebSocketReturn } from '@vueuse/core'
import { useDocumentVisibility, useIntervalFn, useOnline } from '@vueuse/core'
import { watch } from 'vue'
import { reconnectBackoffMs, resolveConnectionRecovery } from './connectionHealth'

export interface ConnectionSupervisorOptions {
  /** The socket to watch. Only its current WebSocket and open() are used. */
  socket: Pick<UseWebSocketReturn<unknown>, 'ws' | 'open'>
  /** Opens the socket unless it is already open or connecting. */
  connect: () => void
  /**
   * An open socket silent for longer than this is rebuilt. Only meaningful for
   * endpoints that push on a fixed schedule; pass Infinity for event streams
   * where silence is normal.
   */
  staleAfterMs: number
  /** How often to check while running; also the base reconnect backoff. */
  intervalMs: number
  /** Upper bound for the reconnect backoff. */
  maxBackoffMs?: number
  /** Clock override for tests. */
  now?: () => number
}

export interface ConnectionSupervisor {
  /** Start supervising: the connection is wanted from now on (idempotent). */
  start: () => void
  /** Stop supervising and forget liveness and backoff state. */
  stop: () => void
  /**
   * Run one check now. `force` skips the reconnect backoff, for user-driven
   * signals such as the tab becoming visible.
   */
  check: (force?: boolean) => void
}

const DEFAULT_MAX_BACKOFF_MS = 5 * 60_000

/**
 * Keeps a long-lived WebSocket alive while it is wanted.
 *
 * Ownership (reference counts, start/stop monitoring) says whether the data is
 * wanted; it cannot say the connection still delivers it. VueUse's
 * autoReconnect gives up after a short burst, which is not enough for a laptop
 * sleep or a backend restart, and a half-open connection never reports a close
 * at all. While started, the supervisor reconnects dead sockets with backoff,
 * rebuilds silent ones, and re-checks immediately when the tab becomes visible
 * or the network returns.
 */
export function useConnectionSupervisor(options: ConnectionSupervisorOptions): ConnectionSupervisor {
  const {
    socket,
    connect,
    staleAfterMs,
    intervalMs,
    maxBackoffMs = DEFAULT_MAX_BACKOFF_MS,
    now = Date.now,
  } = options

  let isRunning = false
  let lastMessageAt = 0
  let failedAttempts = 0
  let nextAttemptAt = 0

  function markAlive() {
    lastMessageAt = now()
    failedAttempts = 0
    nextAttemptAt = 0
  }

  // Liveness comes straight from the socket instead of the owner's callbacks,
  // so every owner gets it without extra wiring. VueUse creates a new
  // WebSocket per connection attempt, hence the watch.
  watch(socket.ws, (ws, _previous, onCleanup) => {
    if (!ws) {
      return
    }

    ws.addEventListener('open', markAlive)
    ws.addEventListener('message', markAlive)
    onCleanup(() => {
      ws.removeEventListener('open', markAlive)
      ws.removeEventListener('message', markAlive)
    })
  }, { immediate: true })

  function check(force = false) {
    const currentTime = now()
    const action = resolveConnectionRecovery({
      hasSubscribers: isRunning,
      readyState: socket.ws.value?.readyState,
      lastMessageAt,
      now: currentTime,
      staleAfterMs,
    })

    if (action === 'none') {
      return
    }

    if (!force && currentTime < nextAttemptAt) {
      return
    }

    // Counted as failed until the socket opens or delivers again.
    failedAttempts++
    nextAttemptAt = currentTime + reconnectBackoffMs(failedAttempts, intervalMs, maxBackoffMs)

    if (action === 'connect') {
      connect()
      return
    }

    // open() closes the silent socket, resets VueUse's retry budget and dials again.
    lastMessageAt = 0
    socket.open()
  }

  const interval = useIntervalFn(() => check(), intervalMs, { immediate: false })

  const visibility = useDocumentVisibility()
  const online = useOnline()

  watch(visibility, (value, previous) => {
    if (value === 'visible' && previous !== 'visible') {
      check(true)
    }
  })

  watch(online, (value, previous) => {
    if (value && !previous) {
      check(true)
    }
  })

  return {
    start() {
      isRunning = true
      // resume() restarts the timer, so repeated starts would keep pushing the
      // next tick back.
      if (!interval.isActive.value) {
        interval.resume()
      }
    },
    stop() {
      isRunning = false
      interval.pause()
      lastMessageAt = 0
      failedAttempts = 0
      nextAttemptAt = 0
    },
    check,
  }
}
