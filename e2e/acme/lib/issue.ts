import type { Page } from '@playwright/test'
import { readToken } from './api'
import { baseURL } from './env'

export interface IssueHint {
  code: string
  message?: string
  params?: Record<string, unknown>
}

/** The structured cosy error the legacy socket attaches to an error message. */
export interface IssueError {
  scope?: string
  code?: number
  message?: string
  params?: string[]
}

export interface IssueResult {
  /** success / error from the final message, or closed / timeout when none arrived. */
  status: 'success' | 'error' | 'closed' | 'timeout'
  message: string
  hint?: IssueHint
  /** Structured error of a failed legacy issuance, when the backend sends one. */
  error?: IssueError
  sslCertificate?: string
  sslCertificateKey?: string
  /** Certificate record id, reported by the HTTPS orchestrator. */
  certId?: number
  /** Every message the server sent, in order. */
  messages: Record<string, unknown>[]
}

export interface IssueOptions {
  challengeMethod?: 'http01' | 'dns01'
  keyType?: string
  timeoutMs?: number
}

/** URL-safe base64 without padding, as buildWebSocketUrl() sends the long token. */
function tokenParam(token: string) {
  return Buffer.from(token, 'utf8').toString('base64').replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
}

/**
 * Issue a certificate through GET /api/domain/:name/cert, the same websocket
 * the legacy site editor flow uses.
 */
export async function issueViaWebSocket(page: Page, name: string, domains: string[], options: IssueOptions = {}): Promise<IssueResult> {
  return runSocket(page, `/api/domain/${encodeURIComponent(name)}/cert`, {
    server_name: domains,
    challenge_method: options.challengeMethod ?? 'http01',
    key_type: options.keyType ?? 'P256',
  }, options.timeoutMs)
}

export interface HTTPSRequest {
  domains: string[]
  challenge_method?: 'http01' | 'dns01'
  redirect_http_to_https?: boolean
  key_type?: string
  certificate_id?: number
}

/**
 * Enable HTTPS through GET /api/sites/:name/https, the backend orchestrator the
 * HTTPS card drives (plan, stage, probe, issue, finalize).
 */
export async function enableHTTPSViaWebSocket(page: Page, name: string, request: HTTPSRequest, timeoutMs?: number): Promise<IssueResult> {
  return runSocket(page, `/api/sites/${encodeURIComponent(name)}/https`, {
    challenge_method: 'http01',
    redirect_http_to_https: true,
    key_type: 'P256',
    ...request,
  }, timeoutMs)
}

/** Step statuses reported by the orchestrator, e.g. { probe: 'skipped' }. */
export function finalStepStatuses(result: IssueResult): Record<string, string> {
  const statuses: Record<string, string> = {}
  for (const m of result.messages) {
    if (m.type === 'step' && typeof m.step === 'string')
      statuses[m.step] = String(m.status)
  }
  return statuses
}

/**
 * The socket runs inside the browser page so the Origin header is the app's
 * own origin, which CheckWebSocketOrigin accepts.
 */
async function runSocket(page: Page, path: string, payload: Record<string, unknown>, timeoutMs = 90_000): Promise<IssueResult> {
  if (!page.url().startsWith(baseURL)) {
    // Any same-origin document works; the JSON endpoint avoids booting the SPA.
    await page.goto('/api/install')
  }

  const url = new URL(path, baseURL)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.searchParams.set('token', tokenParam(readToken()))

  const raw = await page.evaluate(({ url, payload, timeoutMs }) => new Promise<{
    messages: Record<string, unknown>[]
    final: Record<string, unknown> | null
    ending: 'closed' | 'timeout' | 'final'
  }>(resolve => {
    const messages: Record<string, unknown>[] = []
    const ws = new WebSocket(url)
    let settled = false
    const finish = (final: Record<string, unknown> | null, ending: 'closed' | 'timeout' | 'final') => {
      if (settled)
        return
      settled = true
      clearTimeout(timer)
      resolve({ messages, final, ending })
      ws.close()
    }
    const timer = setTimeout(() => finish(null, 'timeout'), timeoutMs)
    ws.onopen = () => ws.send(JSON.stringify(payload))
    ws.onmessage = event => {
      const message = JSON.parse(String(event.data)) as Record<string, unknown>
      messages.push(message)
      // Legacy cert socket: {status:"error"} or {status:"success", ssl_certificate}.
      // HTTPS orchestrator socket: {type:"done", status}.
      const isFinal = message.type !== undefined
        ? message.type === 'done'
        : message.status === 'error' || (message.status === 'success' && typeof message.ssl_certificate === 'string')
      if (isFinal)
        finish(message, 'final')
    }
    ws.onclose = () => finish(null, 'closed')
  }), { url: url.toString(), payload, timeoutMs })

  const final = raw.final
  if (!final) {
    return {
      status: raw.ending === 'timeout' ? 'timeout' : 'closed',
      message: `socket ${raw.ending} before a final message`,
      messages: raw.messages,
    }
  }

  return {
    status: final.status === 'success' ? 'success' : 'error',
    message: String(final.message ?? ''),
    hint: final.hint as IssueHint | undefined,
    error: final.error as IssueError | undefined,
    sslCertificate: final.ssl_certificate as string | undefined,
    sslCertificateKey: final.ssl_certificate_key as string | undefined,
    certId: typeof final.cert_id === 'number' ? final.cert_id : undefined,
    messages: raw.messages,
  }
}

/**
 * The text of a socket message with its translation arguments filled in. Log
 * lines arrive as translation containers: {message: "... %{domain} ...",
 * args: {domain: "..."}}.
 */
export function renderMessage(message: Record<string, unknown>): string {
  let text = String(message.message ?? '')
  const args = message.args
  if (args && typeof args === 'object') {
    for (const [key, value] of Object.entries(args as Record<string, unknown>))
      text = text.replaceAll(`%{${key}}`, String(value))
  }
  return text
}

/** Every message of the transcript, rendered. */
export function renderedMessages(result: IssueResult): string[] {
  return result.messages.map(renderMessage)
}

/** A compact transcript for assertion messages. */
export function describeIssue(result: IssueResult): string {
  const log = result.messages.map(m => `  [${String(m.status ?? m.type ?? '')}] ${renderMessage(m)}`).join('\n')
  const error = result.error ? ` error=${JSON.stringify(result.error)}` : ''
  return `status=${result.status} message=${result.message} hint=${JSON.stringify(result.hint)}${error}\n${log}`
}
