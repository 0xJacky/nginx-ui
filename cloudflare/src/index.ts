import { Container, getContainer } from '@cloudflare/containers'
import { loadingPage } from './loading'

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
   * Report whether the container can serve, kicking off a start if not.
   *
   * Deliberately does not await the boot: the caller returns a loading page
   * immediately instead of holding the request open for several seconds, which
   * is what produces a white screen.
   */
  /** Current container state, for the status endpoint and for debugging. */
  async status(): Promise<{ ready: boolean, status: string, exitCode?: number }> {
    const state = await this.getState()
    return {
      ready: state.status === 'healthy',
      status: state.status,
      // exitCode is only present on the stopped-with-code variant.
      ...('exitCode' in state ? { exitCode: state.exitCode as number } : {}),
    }
  }

  async ready(): Promise<boolean> {
    const state = await this.getState()
    if (state.status === 'healthy') {
      return true
    }

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
   * Return the demo to its pristine state.
   *
   * Container disk is ephemeral, so stopping IS the restore: the next start
   * comes up from the image with the seeded database and configs back in
   * place. SIGTERM rather than a kill, because s6-overlay shuts nginx and
   * nginx-ui down in order and nginx-ui holds an open SQLite handle.
   */
  /**
   * Stop the container so the next request starts it fresh.
   *
   * Needed after changing envVars: they are applied when the container starts,
   * so a deploy alone leaves the running instance on the old environment.
   */
  async recycle(): Promise<string> {
    const state = await this.getState()
    if (state.status === 'stopped' || state.status === 'stopped_with_code') {
      return 'already-stopped'
    }
    await this.stop()
    return 'stopped'
  }

  async restore(): Promise<'stopped' | 'already-stopped'> {
    const state = await this.getState()
    if (state.status === 'stopped' || state.status === 'stopped_with_code') {
      // Most days the idle timeout will already have done this.
      return 'already-stopped'
    }

    await this.stop()
    return 'stopped'
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
    // does not depend on the container being up.
    if (url.pathname === STATUS_PATH) {
      const status = await container.status()
      if (!status.ready) {
        // Kick off a boot so polling the status page actually makes progress.
        await container.ready()
      }
      return Response.json(status, {
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
