/**
 * Lifecycle bookkeeping for an editor integration that is set up asynchronously.
 *
 * Code completion only learns whether it is enabled after an HTTP round trip,
 * attaches its editor listeners on a timer, and opens its socket on demand. Any
 * of those steps can finish after the editor was unmounted or re-initialised,
 * so each init() runs in a session: every resource it acquires registers a
 * teardown with that session, and every late step checks that the session is
 * still active before acquiring anything. Disposing the session (cleanUp or the
 * next init) then releases exactly what was acquired, whenever it was acquired.
 */

export interface LifecycleSession {
  /** False once the session was disposed or superseded by a newer one. */
  readonly active: boolean
  /**
   * Registers a teardown. Teardowns run in reverse registration order; one that
   * is registered after the session is gone runs immediately.
   */
  onDispose: (teardown: () => void) => void
  /** Schedules a callback that is cancelled when the session is disposed. */
  setTimeout: (callback: () => void, delayMs: number) => void
}

export interface Lifecycle {
  /** Disposes the current session, if any, and starts a new one. */
  begin: () => LifecycleSession
  /** Disposes the current session. Safe to call repeatedly. */
  dispose: () => void
}

export interface LifecycleOptions {
  /**
   * Receives errors thrown by teardowns. A failing teardown never stops the
   * remaining ones, otherwise one broken editor call would leak the socket.
   */
  onError?: (error: unknown) => void
}

function createSession(onError: (error: unknown) => void) {
  const teardowns: (() => void)[] = []
  let active = true

  function runTeardown(teardown: () => void) {
    try {
      teardown()
    }
    catch (error) {
      onError(error)
    }
  }

  const session: LifecycleSession = {
    get active() {
      return active
    },
    onDispose(teardown) {
      if (!active) {
        runTeardown(teardown)
        return
      }
      teardowns.push(teardown)
    },
    setTimeout(callback, delayMs) {
      if (!active) {
        return
      }

      const timer = setTimeout(() => {
        if (active) {
          callback()
        }
      }, delayMs)
      session.onDispose(() => clearTimeout(timer))
    },
  }

  function dispose() {
    if (!active) {
      return
    }
    active = false

    // Release in reverse order so later resources, which may depend on earlier
    // ones, go first.
    while (teardowns.length > 0) {
      runTeardown(teardowns.pop()!)
    }
  }

  return { session, dispose }
}

export function createLifecycle(options: LifecycleOptions = {}): Lifecycle {
  const { onError = error => console.error(error) } = options
  let current: ReturnType<typeof createSession> | undefined

  function dispose() {
    const previous = current
    current = undefined
    previous?.dispose()
  }

  function begin() {
    dispose()
    current = createSession(onError)
    return current.session
  }

  return { begin, dispose }
}

export interface LazyResourceOptions<T> {
  /** Creates the resource. Only called while the session is active. */
  create: () => T
  /** Whether an existing resource can still be used; a dead one is replaced. */
  isAlive: (resource: T) => boolean
  /** Releases the resource. */
  destroy: (resource: T) => void
}

export interface LazyResource<T> {
  /**
   * Returns a usable resource, creating it on first use or after the previous
   * one died. Returns undefined once the session is gone, so nothing is created
   * for an editor that has already been torn down.
   */
  acquire: () => T | undefined
  /** The resource created so far, without creating one. */
  readonly current: T | undefined
}

/**
 * Binds an on-demand resource to a session: nothing is created until the first
 * acquire(), and whatever was created is destroyed with the session.
 */
export function createLazyResource<T>(session: LifecycleSession, options: LazyResourceOptions<T>): LazyResource<T> {
  const { create, isAlive, destroy } = options
  let resource: T | undefined

  function release() {
    if (resource === undefined) {
      return
    }

    const stale = resource
    resource = undefined
    destroy(stale)
  }

  session.onDispose(release)

  return {
    acquire() {
      if (!session.active) {
        return undefined
      }

      if (resource !== undefined && isAlive(resource)) {
        return resource
      }

      release()
      const created = create()

      // create() tore the session down (e.g. a callback ran cleanUp): the
      // session teardown has already run, so release the new resource here.
      if (!session.active) {
        destroy(created)
        return undefined
      }

      resource = created
      return resource
    },
    get current() {
      return resource
    },
  }
}
