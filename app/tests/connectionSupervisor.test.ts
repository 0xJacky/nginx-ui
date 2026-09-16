import type { ConnectionSupervisor } from '@/lib/websocket/useConnectionSupervisor'
import { afterEach, describe, expect, test } from 'bun:test'
import { effectScope, nextTick, shallowRef } from 'vue'
import { useConnectionSupervisor } from '@/lib/websocket/useConnectionSupervisor'

const INTERVAL_MS = 15_000
const STALE_AFTER_MS = 20_000
// A timestamp of 0 means "nothing received yet", so the fake clock starts at a
// realistic epoch and tests move it by offsets.
const EPOCH = 1_700_000_000_000

class FakeWebSocket extends EventTarget {
  constructor(public readyState: number) {
    super()
  }
}

const scopes: ReturnType<typeof effectScope>[] = []

afterEach(() => {
  scopes.splice(0).forEach(scope => scope.stop())
})

function setup(staleAfterMs = STALE_AFTER_MS) {
  let clock = EPOCH
  const calls = { connect: 0, open: 0 }
  const ws = shallowRef<WebSocket | undefined>()

  const scope = effectScope()
  const supervisor = scope.run(() => useConnectionSupervisor({
    socket: {
      ws,
      open: () => {
        calls.open++
      },
    },
    connect: () => {
      calls.connect++
    },
    staleAfterMs,
    intervalMs: INTERVAL_MS,
    maxBackoffMs: 60_000,
    now: () => clock,
  })) as ConnectionSupervisor

  scopes.push(scope)

  return {
    calls,
    supervisor,
    setClock: (offset: number) => {
      clock = EPOCH + offset
    },
    async useSocket(readyState: number) {
      const socket = new FakeWebSocket(readyState)
      ws.value = socket as unknown as WebSocket
      // The supervisor attaches its listeners in a watcher.
      await nextTick()
      return socket
    },
  }
}

describe('connection supervisor', () => {
  test('leaves the socket alone until started', () => {
    const { calls, supervisor } = setup()

    supervisor.check()

    expect(calls.connect).toBe(0)
  })

  test('reconnects a dead socket and backs off while it stays down', () => {
    const { calls, supervisor, setClock } = setup()
    supervisor.start()

    supervisor.check()
    expect(calls.connect).toBe(1)

    // Inside the first backoff window (one interval).
    setClock(INTERVAL_MS - 1)
    supervisor.check()
    expect(calls.connect).toBe(1)

    setClock(INTERVAL_MS)
    supervisor.check()
    expect(calls.connect).toBe(2)

    // The window doubles after a second failed attempt.
    setClock(INTERVAL_MS * 2)
    supervisor.check()
    expect(calls.connect).toBe(2)

    setClock(INTERVAL_MS * 3)
    supervisor.check()
    expect(calls.connect).toBe(3)
  })

  test('a user-driven check skips the backoff', () => {
    const { calls, supervisor } = setup()
    supervisor.start()

    supervisor.check()
    supervisor.check(true)

    expect(calls.connect).toBe(2)
  })

  test('a socket that opens again resets the backoff', async () => {
    const { calls, supervisor, setClock, useSocket } = setup()
    supervisor.start()

    supervisor.check()
    setClock(INTERVAL_MS)
    supervisor.check()
    expect(calls.connect).toBe(2)

    const socket = await useSocket(WebSocket.OPEN)
    socket.dispatchEvent(new Event('open'))

    // It drops again right away: no leftover backoff delays the retry.
    socket.readyState = WebSocket.CLOSED
    supervisor.check()
    expect(calls.connect).toBe(3)
  })

  test('rebuilds an open socket that went silent', async () => {
    const { calls, supervisor, setClock, useSocket } = setup()
    supervisor.start()

    const socket = await useSocket(WebSocket.OPEN)
    socket.dispatchEvent(new Event('message'))

    setClock(STALE_AFTER_MS)
    supervisor.check()
    expect(calls.open).toBe(0)

    setClock(STALE_AFTER_MS + 1)
    supervisor.check()
    expect(calls.open).toBe(1)
    expect(calls.connect).toBe(0)
  })

  test('ignores activity from a socket that was already replaced', async () => {
    const { calls, supervisor, setClock, useSocket } = setup()
    supervisor.start()

    const previous = await useSocket(WebSocket.OPEN)
    previous.dispatchEvent(new Event('message'))

    await useSocket(WebSocket.OPEN)

    setClock(STALE_AFTER_MS + 1)
    previous.dispatchEvent(new Event('message'))
    supervisor.check()

    expect(calls.open).toBe(1)
  })

  test('never rebuilds a quiet event stream', async () => {
    const { calls, supervisor, setClock, useSocket } = setup(Number.POSITIVE_INFINITY)
    supervisor.start()

    const socket = await useSocket(WebSocket.OPEN)
    socket.dispatchEvent(new Event('open'))

    setClock(24 * 60 * 60_000)
    supervisor.check()

    expect(calls.open).toBe(0)
    expect(calls.connect).toBe(0)
  })

  test('stops acting once stopped', () => {
    const { calls, supervisor } = setup()
    supervisor.start()
    supervisor.stop()

    supervisor.check(true)

    expect(calls.connect).toBe(0)
  })
})
