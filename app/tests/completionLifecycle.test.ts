import { describe, expect, test } from 'bun:test'
import { createLazyResource, createLifecycle } from '@/components/CodeEditor/completionLifecycle'

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

function collectingLifecycle() {
  const errors: unknown[] = []
  const lifecycle = createLifecycle({ onError: error => errors.push(error) })
  return { errors, lifecycle }
}

function socketTracker() {
  const sockets: { id: number, alive: boolean, closed: boolean }[] = []
  const options = {
    create: () => {
      const socket = { id: sockets.length + 1, alive: true, closed: false }
      sockets.push(socket)
      return socket
    },
    isAlive: (socket: { alive: boolean }) => socket.alive,
    destroy: (socket: { closed: boolean }) => {
      socket.closed = true
    },
  }

  return { sockets, options }
}

describe('completion lifecycle sessions', () => {
  test('runs teardowns once, newest first', () => {
    const { lifecycle } = collectingLifecycle()
    const order: string[] = []

    const session = lifecycle.begin()
    session.onDispose(() => order.push('socket'))
    session.onDispose(() => order.push('listeners'))

    lifecycle.dispose()
    lifecycle.dispose()

    expect(session.active).toBe(false)
    expect(order).toEqual(['listeners', 'socket'])
  })

  test('releases a resource acquired after the session was disposed', async () => {
    const { lifecycle } = collectingLifecycle()
    let closed = 0

    // init() awaits the enabled status, the editor unmounts meanwhile.
    const session = lifecycle.begin()
    const status = sleep(10).then(() => true)
    lifecycle.dispose()
    await status

    expect(session.active).toBe(false)
    session.onDispose(() => closed++)
    expect(closed).toBe(1)
  })

  test('never fires a timer after dispose', async () => {
    const { lifecycle } = collectingLifecycle()
    let fired = 0

    const session = lifecycle.begin()
    session.setTimeout(() => fired++, 10)
    lifecycle.dispose()

    // Scheduling from a late async step is a no-op too.
    session.setTimeout(() => fired++, 0)

    await sleep(30)
    expect(fired).toBe(0)
  })

  test('fires a timer while the session is active', async () => {
    const { lifecycle } = collectingLifecycle()
    let fired = 0

    lifecycle.begin().setTimeout(() => fired++, 5)

    await sleep(20)
    expect(fired).toBe(1)
  })

  test('a repeated begin disposes the previous session', () => {
    const { lifecycle } = collectingLifecycle()
    let closed = 0

    const first = lifecycle.begin()
    first.onDispose(() => closed++)

    const second = lifecycle.begin()

    expect(first.active).toBe(false)
    expect(second.active).toBe(true)
    expect(closed).toBe(1)
  })

  test('a failing teardown does not skip the others', () => {
    const { errors, lifecycle } = collectingLifecycle()
    let closed = 0

    const session = lifecycle.begin()
    session.onDispose(() => closed++)
    session.onDispose(() => {
      throw new Error('editor already destroyed')
    })

    lifecycle.dispose()

    expect(closed).toBe(1)
    expect(errors).toHaveLength(1)
  })
})

describe('lazy completion socket', () => {
  test('is not opened until the first acquire', () => {
    const { lifecycle } = collectingLifecycle()
    const { sockets, options } = socketTracker()

    const connection = createLazyResource(lifecycle.begin(), options)

    expect(sockets).toHaveLength(0)
    expect(connection.current).toBeUndefined()

    const socket = connection.acquire()
    expect(connection.acquire()).toBe(socket)
    expect(sockets).toHaveLength(1)
  })

  test('replaces a socket that died', () => {
    const { lifecycle } = collectingLifecycle()
    const { sockets, options } = socketTracker()

    const connection = createLazyResource(lifecycle.begin(), options)

    connection.acquire()!.alive = false
    const replacement = connection.acquire()

    expect(sockets).toHaveLength(2)
    expect(sockets[0].closed).toBe(true)
    expect(replacement).toBe(sockets[1])
  })

  test('closes the socket with the session and refuses to reopen it', () => {
    const { lifecycle } = collectingLifecycle()
    const { sockets, options } = socketTracker()

    const connection = createLazyResource(lifecycle.begin(), options)
    connection.acquire()

    lifecycle.dispose()

    expect(sockets[0].closed).toBe(true)
    expect(connection.current).toBeUndefined()

    // A debounced completion request that slipped past cleanup.
    expect(connection.acquire()).toBeUndefined()
    expect(sockets).toHaveLength(1)
  })

  test('closes the socket of a superseded init', () => {
    const { lifecycle } = collectingLifecycle()
    const { sockets, options } = socketTracker()

    createLazyResource(lifecycle.begin(), options).acquire()
    const connection = createLazyResource(lifecycle.begin(), options)

    expect(sockets[0].closed).toBe(true)
    expect(connection.acquire()).toBe(sockets[1])
    expect(sockets[1].closed).toBe(false)
  })

  test('destroys a socket whose creation tore the session down', () => {
    const { lifecycle } = collectingLifecycle()
    const { sockets, options } = socketTracker()

    const connection = createLazyResource(lifecycle.begin(), {
      ...options,
      create: () => {
        const socket = options.create()
        lifecycle.dispose()
        return socket
      },
    })

    expect(connection.acquire()).toBeUndefined()
    expect(sockets[0].closed).toBe(true)
  })
})
