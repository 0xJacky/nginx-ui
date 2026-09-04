// Client-side expiry is a UX safeguard. The server still validates every token.
export function getTokenExpiration(token: string): number | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3)
      return null
    const payload = parts[1].replaceAll('-', '+').replaceAll('_', '/')
    const { exp } = JSON.parse(atob(payload.padEnd(Math.ceil(payload.length / 4) * 4, '=')))
    return typeof exp === 'number' && Number.isFinite(exp) && Number.isFinite(exp * 1000)
      ? exp * 1000
      : null
  }
  catch {
    // Opaque or malformed credentials must be rejected by the server, not guessed here.
    return null
  }
}

export function isTokenExpired(token: string, now = Date.now()): boolean {
  const expiresAt = getTokenExpiration(token)
  return expiresAt !== null && expiresAt <= now
}

interface ExpiryClock {
  now: () => number
  schedule: (callback: () => void, delay: number) => () => void
}

const defaultClock: ExpiryClock = {
  now: () => Date.now(),
  schedule(callback, delay) {
    const timer = setTimeout(callback, delay)
    return () => clearTimeout(timer)
  },
}

export function createTokenExpiryMonitor(
  getToken: () => string,
  onExpired: (token: string) => void,
  clock: ExpiryClock = defaultClock,
) {
  let cancelTimer: (() => void) | undefined
  let stopped = false

  function check() {
    cancelTimer?.()
    cancelTimer = undefined
    if (stopped)
      return

    const token = getToken()
    const expiresAt = getTokenExpiration(token)
    if (expiresAt === null)
      return

    const remaining = expiresAt - clock.now()
    if (remaining <= 0) {
      onExpired(token)
      return
    }

    // Periodically recheck the wall clock and avoid the browser timer's 32-bit limit.
    cancelTimer = clock.schedule(check, Math.min(remaining, 60_000))
  }

  function stop() {
    stopped = true
    cancelTimer?.()
    cancelTimer = undefined
  }

  return { check, stop }
}
