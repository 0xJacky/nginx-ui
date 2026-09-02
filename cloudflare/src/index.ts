import type { StopParams } from '@cloudflare/containers'
import type { WedgeState } from './recovery'
import { Container, getContainer } from '@cloudflare/containers'
import { loadingPage } from './loading'
import { clearUnserved, decideRecovery, freshWedgeState, markServing } from './recovery'

interface Env {
  NGINX_UI_DEMO: DurableObjectNamespace<NginxUiDemo>
  /**
   * Shared secret for the manual recycle endpoint. Unset means the endpoint is
   * disabled, which is the right default: no secret, no lever.
   *   bunx wrangler secret put DEMO_ADMIN_TOKEN
   */
  DEMO_ADMIN_TOKEN?: string
}

/**
 * The demo runs as a single shared instance rather than one per visitor.
 *
 * Everyone must see the same nginx configuration: a visitor who edits a site
 * and reloads has to land back on the container that holds the edit. getRandom
 * would scatter them across instances and make the demo look broken.
 */
// Durable Object state persists across container-application deletes, and a DO
// that still references a deleted application never binds a new instance. Bump
// this to hand the Worker a clean Durable Object when that happens.
const INSTANCE = 'demo-v2'

/** Worker-owned path, never forwarded to the container. */
const STATUS_PATH = '/__demo/status'

/**
 * Container-served path that must reach nginx even while the container is
 * still reported as unhealthy — it is how a failed boot gets diagnosed, so
 * gating it behind readiness would defeat it.
 */
const BOOT_LOG_PATH = '/__demo/bootlog'

/** Worker-owned, secret-gated: stop the container so the next request starts it fresh. */
const RECYCLE_PATH = '/__demo/recycle'

/**
 * Minimum gap between probes of the application behind nginx.
 *
 * Every unanswered probe costs a platform-level error event, and the loading
 * page polls once a second at first, so an unprobed cache window is what keeps
 * a down container from generating thousands of errors an hour.
 */
const PROBE_INTERVAL_MS = 2_000

export class NginxUiDemo extends Container<Env> {
  // nginx inside the container listens here; it proxies to nginx-ui on 9000.
  defaultPort = 8080

  // Long enough that a visitor reading the docs mid-session does not get
  // dropped, short enough that an idle demo is not billed all night.
  sleepAfter = '20m'

  // The SPA's own health endpoint (router/routers.go). The default "ping" path
  // would 404 and delay readiness.
  pingEndpoint = 'localhost/healthz'

  // Note: the WebSocket origin allowlist lives in resources/demo/app.ini, not
  // in envVars here. envVars are applied at container start, so they do not
  // reach an already-running instance on deploy — and a value baked into the
  // image ships and rolls out with it. Serving this Worker on a hostname other
  // than the one in app.ini means adding that origin there too, or every
  // WebSocket upgrade will be rejected.

  /**
   * Boot in progress, if any. Held on the instance so concurrent requests
   * during a cold start share one startup rather than racing several.
   */
  private booting?: Promise<void>

  /**
   * Whether the application behind nginx has been seen serving.
   *
   * Container "healthy" is not enough. The platform's port check counts any
   * HTTP response as success, and nginx binds 8080 the moment it starts —
   * several seconds before nginx-ui is up on 9000. During that window nginx
   * answers 502, the container reads as healthy, the loading page is dismissed
   * and the visitor gets the raw 502. Cached because once nginx-ui is serving
   * it stays up for the container's life; cleared whenever the container stops.
   */
  private applicationServing = false

  /** When the probe last ran, so a busy loading page cannot drive one per request. */
  private lastProbeAt = 0

  /** Tracks a container that claims to be healthy but will not serve. */
  private wedge: WedgeState = freshWedgeState()

  /**
   * Probe the application itself rather than the port in front of it.
   *
   * Rate-limited: the answer only changes when nginx-ui finishes starting, so
   * probing more than once every couple of seconds buys nothing and costs an
   * error event per attempt while the container is down.
   */
  private async applicationReady(): Promise<boolean> {
    if (this.applicationServing) {
      return true
    }

    const now = Date.now()
    if (now - this.lastProbeAt < PROBE_INTERVAL_MS) {
      return false
    }
    this.lastProbeAt = now

    try {
      const probe = await this.containerFetch('http://localhost/healthz')
      if (probe.ok) {
        this.applicationServing = true
        return true
      }

      if (await isStaleContainerError(probe)) {
        // Not "still starting". The platform is saying nothing is bound to the
        // port at all, which on a container it also calls healthy means its
        // view of the container is stale.
        console.log('container reports healthy but nothing is listening on its port')
      }
      else {
        // 502 while nginx-ui is still starting is the case this exists for.
        console.log(`nginx is up but the app is not serving yet: ${probe.status}`)
      }
    }
    catch (err: unknown) {
      console.log(`health probe failed: ${err}`)
    }

    return false
  }

  /**
   * Current container state, for the status endpoint and for debugging.
   *
   * Goes through ready() rather than reading the state directly, so that
   * polling this endpoint also drives a start. The loading page polls it and
   * nothing else, so a status call that only observed would leave a sleeping
   * container asleep.
   */
  async status(): Promise<{ ready: boolean, status: string, exitCode?: number }> {
    const ready = await this.ready()
    const state = await this.getState()

    return {
      ready,
      status: state.status,
      // exitCode is only present on the stopped-with-code variant.
      ...('exitCode' in state ? { exitCode: state.exitCode as number } : {}),
    }
  }

  /**
   * Report whether the container can serve, kicking off a start if not.
   *
   * Deliberately does not await the boot: the caller returns a loading page
   * immediately instead of holding the request open for several seconds, which
   * is what produces a white screen.
   */
  async ready(): Promise<boolean> {
    const state = await this.getState()
    if (state.status === 'healthy') {
      if (await this.applicationReady()) {
        this.wedge = markServing()
        return true
      }

      await this.recoverIfWedged()
      return false
    }

    this.applicationServing = false
    this.wedge = clearUnserved(this.wedge)
    console.log(`container not ready yet: status=${state.status}`)

    this.booting ??= this.startAndWaitForPorts()
      .catch((err: unknown) => {
        console.error('demo container failed to start', err)
      })
      .finally(() => {
        this.booting = undefined
      })

    return false
  }

  /**
   * Break the deadlock where the platform reports the container healthy but it
   * never serves.
   *
   * getState() is not the truth. When a container goes away without the
   * platform reconciling its record, the state stays 'healthy' indefinitely —
   * so the start path above, which only runs for a non-healthy state, is never
   * reached and the demo stays down until someone intervenes by hand. This has
   * happened: an instance sat 'running' for nineteen hours with nothing bound
   * to 8080. Stopping the container puts the record into a state ready() knows
   * how to start from.
   *
   * Deliberately slow to trigger. Stopping a container that is merely slow to
   * boot is exactly how an earlier version of this file put the loading page
   * into a permanent refresh loop, so this waits out a full boot window first
   * and then acts at most once per cooldown.
   */
  private async recoverIfWedged(): Promise<void> {
    const decision = decideRecovery(this.wedge, Date.now())
    this.wedge = decision.next

    if (decision.action === 'wait') {
      return
    }

    this.applicationServing = false
    console.error(
      `container has reported healthy for ${Math.round(decision.unservedFor / 1000)}s `
      + `without serving; sending ${decision.action} so the next request starts a fresh one `
      + `(recovery attempt ${decision.next.attempts})`,
    )

    try {
      await (decision.action === 'destroy' ? this.destroy() : this.stop())
    }
    catch (err: unknown) {
      console.error('failed to recover the wedged container', err)
    }
  }

  override onStop(params: StopParams): void {
    // A restarted container serves 502 again until nginx-ui is back, so the
    // cached answer must not survive the stop.
    this.applicationServing = false
    this.wedge = clearUnserved(this.wedge)
    console.log(`container stopped: exitCode=${params.exitCode} reason=${params.reason}`)
  }

  /**
   * Stop the container so the next request brings up a fresh one.
   *
   * Container disk is ephemeral, so stopping IS the restore: the next start
   * comes up from the image with the seeded database and configs back in
   * place. SIGTERM rather than a kill, because the entrypoint shuts nginx and
   * nginx-ui down in order and nginx-ui holds an open SQLite handle.
   *
   * Used for both the scheduled restore and the manual recycle after a config
   * change that only takes effect at container start.
   */
  async recycle(): Promise<'stopped' | 'already-stopped'> {
    const state = await this.getState()
    if (state.status === 'stopped' || state.status === 'stopped_with_code') {
      // Most days the idle timeout will already have done this.
      return 'already-stopped'
    }

    await this.stop()
    return 'stopped'
  }

  /** Alias kept for the scheduled handler's intent to read clearly. */
  async restore(): Promise<'stopped' | 'already-stopped'> {
    return this.recycle()
  }

  override onError(error: unknown): Response {
    console.error('demo container error', error)
    return new Response('The demo container failed to start.', {
      status: 502,
      headers: { 'content-type': 'text/plain; charset=utf-8' },
    })
  }
}

/** A navigation request is one where showing a loading page makes sense. */
function wantsDocument(request: Request): boolean {
  if (request.method !== 'GET') {
    return false
  }
  const accept = request.headers.get('accept') ?? ''
  return accept.includes('text/html')
}

function isWebSocketUpgrade(request: Request): boolean {
  return (request.headers.get('upgrade') ?? '').toLowerCase() === 'websocket'
}

/**
 * Recognise the platform's own "the container went away" response.
 *
 * When the container is reaped between the readiness check and the proxy, the
 * runtime answers with a plain-text error naming the unreachable address rather
 * than throwing, so it cannot be caught — it has to be detected. Matching on
 * the message is unpleasant but there is no status code or header that
 * distinguishes it from an error the application itself produced.
 */
async function isStaleContainerError(response: Response): Promise<boolean> {
  if (response.status < 500 || response.webSocket) {
    return false
  }
  if (!(response.headers.get('content-type') ?? '').startsWith('text/plain')) {
    return false
  }

  // Peek without consuming: the body is still needed if this turns out to be a
  // genuine application error.
  const body = await response.clone().text().catch(() => '')
  return body.includes('Error proxying request to container')
    || body.includes('is not listening in the TCP address')
}

/**
 * Stamp the public scheme and host onto a request before it reaches the
 * container.
 *
 * The Worker-to-container hop is plain HTTP, so without this nginx derives
 * `http://<host>` while the browser sent `Origin: https://<host>`. Nginx UI's
 * WebSocket origin check compares the two and rejects every upgrade — the
 * terminal, the log stream and the cluster monitor all fail with nothing in the
 * logs to explain it.
 */
function withForwardedHeaders(request: Request, url: URL): Request {
  // A WebSocket upgrade is passed through untouched. Reconstructing the
  // request drops the upgrade in the Workers runtime, and the handshake then
  // fails with no diagnostic anywhere. The origin problem those headers would
  // have solved is handled by NGINX_UI_HTTP_WEBSOCKET_TRUSTED_ORIGINS instead.
  if (isWebSocketUpgrade(request)) {
    return request
  }

  const headers = new Headers(request.headers)
  headers.set('X-Forwarded-Proto', url.protocol.replace(':', ''))
  headers.set('X-Forwarded-Host', url.host)
  return new Request(request, { headers })
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url)
    const container = getContainer(env.NGINX_UI_DEMO, INSTANCE)

    // Answered by the Worker so the loading page has something to poll that
    // does not depend on the container being up. status() kicks off a boot
    // itself, so polling makes progress without a second round trip — which
    // used to double the platform error events a down container produced.
    if (url.pathname === STATUS_PATH) {
      return Response.json(await container.status(), {
        headers: { 'cache-control': 'no-store' },
      })
    }

    // Force a fresh container, for when a config change only takes effect at
    // container start (envVars) rather than at deploy.
    if (url.pathname === RECYCLE_PATH) {
      const supplied = request.headers.get('x-demo-admin-token')
      if (!env.DEMO_ADMIN_TOKEN || supplied !== env.DEMO_ADMIN_TOKEN) {
        return new Response('Not found', { status: 404 })
      }
      return Response.json({ outcome: await container.recycle() })
    }

    if (url.pathname === BOOT_LOG_PATH) {
      // Deliberately unguarded: nginx answers this one even when nginx-ui is
      // down, which is precisely when it is worth reading.
      await container.ready()
      return container.fetch(withForwardedHeaders(request, url))
    }

    let ready = await container.ready()

    if (ready) {
      // fetch(), not containerFetch(): only fetch() carries WebSocket upgrades,
      // which the terminal, log stream and cluster monitor all rely on.
      const response = await container.fetch(withForwardedHeaders(request, url))

      // The readiness check and the proxy are not atomic. The idle timeout can
      // reap the container in between, leaving the Durable Object reporting a
      // state that is no longer true; the platform then answers with its own
      // "not listening on ..." error, which used to reach the visitor verbatim.
      // Treat that as not-ready, ask for a fresh container, and fall through to
      // the loading page the rest of this handler already knows how to serve.
      if (await isStaleContainerError(response)) {
        // Do NOT stop the container here. It is already gone, and a stop lands
        // on whatever boot ready() has since kicked off — every subsequent
        // request would kill the container that is trying to start, and the
        // loading page would refresh forever. Just re-arm the boot and fall
        // through.
        console.log('container went away mid-request; waiting for the restart')
        await container.ready()
        ready = false
      }
      else {
        return response
      }
    }

    if (wantsDocument(request) && !isWebSocketUpgrade(request)) {
      // 200 at the originally requested URL, never a redirect: the visitor's
      // deep link survives, and there is structurally no redirect loop.
      return new Response(loadingPage(), {
        status: 200,
        headers: {
          'content-type': 'text/html; charset=utf-8',
          'cache-control': 'no-store',
        },
      })
    }

    // XHR, assets and WebSocket upgrades get a normal retry signal instead of
    // an HTML page they cannot parse.
    return new Response('The demo is starting up.', {
      status: 503,
      headers: {
        'retry-after': '3',
        'cache-control': 'no-store',
        'content-type': 'text/plain; charset=utf-8',
      },
    })
  },

  async scheduled(_controller: ScheduledController, env: Env): Promise<void> {
    const container = getContainer(env.NGINX_UI_DEMO, INSTANCE)
    const outcome = await container.restore()
    console.log(`scheduled demo restore: ${outcome}`)
  },
}
