import { describe, expect, test } from 'bun:test'
import { createSharedConnection } from '@/lib/websocket/sharedConnection'

function tracker(lingerMs = 0) {
  const calls = { open: 0, close: 0 }
  const connection = createSharedConnection({
    open: () => {
      calls.open++
    },
    close: () => {
      calls.close++
    },
    lingerMs,
  })

  return { calls, connection }
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

describe('shared connection reference counting', () => {
  test('keeps the connection open while another subscriber remains', () => {
    const { calls, connection } = tracker()

    const page = connection.acquire()
    const otherPage = connection.acquire()

    connection.release(page)

    expect(calls.close).toBe(0)
    expect(connection.subscriberCount).toBe(1)

    connection.release(otherPage)

    expect(calls.close).toBe(1)
    expect(connection.hasSubscribers).toBe(false)
  })

  test('ignores a double release instead of dropping other subscribers', () => {
    const { calls, connection } = tracker()

    const page = connection.acquire()
    connection.acquire()

    connection.release(page)
    connection.release(page)

    expect(calls.close).toBe(0)
    expect(connection.subscriberCount).toBe(1)
  })

  test('reopens for a late subscriber after the connection was closed', () => {
    const { calls, connection } = tracker()

    connection.release(connection.acquire())
    expect(calls.close).toBe(1)

    connection.acquire()
    expect(calls.open).toBe(2)
  })

  test('defers the close and cancels it when a page subscribes again', async () => {
    const { calls, connection } = tracker(30)

    connection.release(connection.acquire())
    expect(calls.close).toBe(0)

    // Navigating back before the linger window elapses reuses the socket.
    const page = connection.acquire()
    await sleep(60)
    expect(calls.close).toBe(0)

    connection.release(page)
    expect(calls.close).toBe(0)
    await sleep(60)
    expect(calls.close).toBe(1)
  })

  test('shutdown closes immediately and forgets pending subscribers', async () => {
    const { calls, connection } = tracker(30)

    const page = connection.acquire()
    connection.acquire()

    connection.shutdown()

    expect(calls.close).toBe(1)
    expect(connection.hasSubscribers).toBe(false)

    // A scope disposed after the shutdown must not trigger a second close.
    connection.release(page)
    await sleep(60)
    expect(calls.close).toBe(1)
  })
})
