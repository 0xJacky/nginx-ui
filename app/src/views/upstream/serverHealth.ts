export interface ServerProbeResult {
  online: boolean
  latency: number
}

export type ServerHealthState = 'disabled' | 'unknown' | 'online' | 'offline'

export interface ServerHealth {
  state: ServerHealthState
  // The probe result behind an online or offline state.
  result?: ServerProbeResult
}

/**
 * Health of one upstream server from the shared availability results.
 *
 * The results are keyed by the socket the health checker probes, which the
 * backend resolves for every server (`web.internal` -> `web.internal:80`,
 * `[::1]` -> `[::1]:80`, `unix:/run/app.sock` unchanged). The lookup therefore
 * always goes through that socket and never through the address as written,
 * which misses every server without an explicit port. A disabled server is
 * reported as such whatever its last probe said.
 */
export function serverHealth(
  isDown: boolean,
  socket: string | undefined,
  results: Record<string, ServerProbeResult | undefined>,
): ServerHealth {
  if (isDown)
    return { state: 'disabled' }
  const result = socket ? results[socket] : undefined
  if (!result)
    return { state: 'unknown' }
  return { state: result.online ? 'online' : 'offline', result }
}
