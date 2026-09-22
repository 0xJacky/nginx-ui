/**
 * Reference counting for a connection that is shared by several pages.
 *
 * A store-level WebSocket outlives the component that happened to open it, so
 * no single route may close it on unmount: that is how leaving one page freezes
 * the data every other page still renders. Each consumer instead acquires a
 * token while it needs the data and releases it when its scope is disposed, and
 * the connection is only closed once the last token is gone.
 */

export type SubscriptionToken = symbol

export interface SharedConnectionOptions {
  /**
   * Opens the underlying connection. Called on every acquire, so it must be
   * idempotent — that also makes a late subscriber revive a socket that died
   * while nobody was watching.
   */
  open: () => void
  /** Closes the underlying connection. Must be idempotent. */
  close: () => void
  /**
   * How long to keep the connection open after the last subscriber leaves.
   * Navigating through a page that does not need the data would otherwise
   * close and immediately reopen the socket. Set to 0 to close synchronously.
   */
  lingerMs?: number
}

export interface SharedConnection {
  /** Registers a subscriber and makes sure the connection is open. */
  acquire: () => SubscriptionToken
  /** Unregisters a subscriber; closes the connection when it was the last one. */
  release: (token: SubscriptionToken) => void
  /** Drops every subscriber and closes the connection immediately (logout, unload). */
  shutdown: () => void
  readonly subscriberCount: number
  readonly hasSubscribers: boolean
}

const DEFAULT_LINGER_MS = 5000

export function createSharedConnection(options: SharedConnectionOptions): SharedConnection {
  const { open, close, lingerMs = DEFAULT_LINGER_MS } = options

  const subscribers = new Set<SubscriptionToken>()
  let lingerTimer: ReturnType<typeof setTimeout> | undefined

  function clearLingerTimer() {
    if (lingerTimer !== undefined) {
      clearTimeout(lingerTimer)
      lingerTimer = undefined
    }
  }

  function acquire(): SubscriptionToken {
    const token = Symbol('shared-connection-subscriber')
    subscribers.add(token)
    clearLingerTimer()
    open()
    return token
  }

  function release(token: SubscriptionToken) {
    // An unknown token means a double release (StrictMode-style remounts, HMR,
    // a manual release followed by scope disposal). Ignoring it keeps the
    // counter from drifting below the number of live subscribers, which would
    // close the connection under the pages that are still using it.
    if (!subscribers.delete(token)) {
      return
    }

    if (subscribers.size > 0) {
      return
    }

    clearLingerTimer()

    if (lingerMs <= 0) {
      close()
      return
    }

    lingerTimer = setTimeout(() => {
      lingerTimer = undefined
      if (subscribers.size === 0) {
        close()
      }
    }, lingerMs)
  }

  function shutdown() {
    subscribers.clear()
    clearLingerTimer()
    close()
  }

  return {
    acquire,
    release,
    shutdown,
    get subscriberCount() {
      return subscribers.size
    },
    get hasSubscribers() {
      return subscribers.size > 0
    },
  }
}
