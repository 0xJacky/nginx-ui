import type { APIRequestContext, APIResponse } from '@playwright/test'
import { request } from '@playwright/test'
import { inContainer, mustInContainer, writeContainerFile } from './docker'
import { baseURL } from './env'

// The ingress API as an external controller drives it: every request carries
// the node secret in X-Node-Secret, never an admin token. See
// docs/guide/ingress.md.

export interface IngressServer {
  address: string
  weight?: number
  max_fails?: number
  fail_timeout?: string
  down?: boolean
  backup?: boolean
}

export interface IngressSpec {
  hosts: string[]
  backends: { name: string, lb?: string, keepalive?: number, servers: IngressServer[] }[]
  routes: { path_prefix: string, backend: string, websocket?: boolean, read_timeout?: string }[]
  tls: { mode: 'none' | 'acme' | 'cert', challenge?: 'http01' | 'dns01', key_type?: string, redirect_https?: boolean, hsts?: boolean, cert_id?: number }
  policy?: { extra_headers?: { name: string, value: string }[] }
}

export interface IngressCondition {
  type: string
  status: boolean
  reason: string
  message: string
}

export interface Ingress {
  id: number
  name: string
  owner: string
  generation: number
  site_path: string
  spec: IngressSpec
  status: {
    observed_generation: number
    phase: string
    conditions: IngressCondition[] | null
    config_hash: string
    cert_id: number
    cert_not_after: string | null
    servers: { backend: string, address: string, down: boolean, healthy: boolean, checked: boolean }[] | null
  }
}

/** The cosy error body: { scope, code, message, params }. */
export interface ApiError {
  scope?: string
  code?: number
  message?: string
}

/** The node secret nginx-ui generated at boot, from the [node] section of app.ini. */
export function readNodeSecret(): string {
  let section = ''
  for (const raw of mustInContainer('cat /etc/nginx-ui/app.ini').split('\n')) {
    const line = raw.trim()
    const header = /^\[(.+)\]$/.exec(line)
    if (header) {
      section = header[1]!.toLowerCase()
      continue
    }
    const pair = /^Secret\s*=\s*(.+)$/.exec(line)
    if (section === 'node' && pair)
      return pair[1]!.trim()
  }
  throw new Error('no [node] Secret in /etc/nginx-ui/app.ini')
}

export async function createNodeApi(): Promise<APIRequestContext> {
  return request.newContext({
    baseURL,
    extraHTTPHeaders: { 'X-Node-Secret': readNodeSecret() },
  })
}

export async function body<T>(response: APIResponse): Promise<T> {
  return await response.json() as T
}

export async function expectStatus(response: APIResponse, status: number, what: string) {
  if (response.status() !== status)
    throw new Error(`${what} answered ${response.status()}, want ${status}: ${await response.text()}`)
  return response
}

export function condition(ing: Ingress, type: string): IngressCondition | undefined {
  return ing.status.conditions?.find(c => c.type === type)
}

/** A short description of the status for assertion messages. */
export function describeStatus(ing: Ingress): string {
  const conditions = (ing.status.conditions ?? [])
    .map(c => `  ${c.type}=${c.status} ${c.reason}${c.message ? `: ${c.message}` : ''}`)
    .join('\n')
  return `phase=${ing.status.phase} generation=${ing.generation} observed=${ing.status.observed_generation}\n${conditions}`
}

// Two loopback backends served by the container's own nginx. Each names
// itself and echoes the headers the ingress proxy sets.
export const backendPorts = { one: 24101, two: 24102 } as const
const backendsConf = '/etc/nginx/conf.d/e2e-ingress-backends.conf'

function backendServer(label: string, port: number) {
  return `server {
    listen 127.0.0.1:${port};
    location / {
        default_type text/plain;
        return 200 "backend=${label} host=$http_host xff=$http_x_forwarded_for proto=$http_x_forwarded_proto\\n";
    }
}
`
}

/** Install the two backends (idempotent) and wait until both answer. */
export async function startBackends() {
  writeContainerFile(backendsConf, `# Ingress E2E backends (tests/api-ingress.spec.ts)\n${backendServer('one', backendPorts.one)}${backendServer('two', backendPorts.two)}`)
  mustInContainer('nginx -t && nginx -s reload')
  const deadline = Date.now() + 15_000
  for (const port of Object.values(backendPorts)) {
    while (!inContainer(`curl -fsS --max-time 2 http://127.0.0.1:${port}/`).stdout.includes('backend=')) {
      if (Date.now() > deadline)
        throw new Error(`backend 127.0.0.1:${port} did not answer`)
      await new Promise(resolve => setTimeout(resolve, 250))
    }
  }
}

function shellQuote(value: string) {
  return `'${value.replaceAll('\'', `'\\''`)}'`
}

/**
 * GET https://<domain>/ through the local Nginx with an X-Forwarded-For the
 * client already set. Returns the body, empty when the request failed.
 */
export function httpsBody(domain: string): string {
  return inContainer(
    `curl -sS -k --max-time 10 -H 'X-Forwarded-For: 203.0.113.7' --resolve ${shellQuote(`${domain}:443:127.0.0.1`)} ${shellQuote(`https://${domain}/`)}`,
  ).stdout
}

/** GET http://<domain><path> through the local Nginx without following redirects; the body. */
export function httpBody(domain: string, path = '/'): string {
  return inContainer(
    `curl -sS --max-time 10 --resolve ${shellQuote(`${domain}:80:127.0.0.1`)} ${shellQuote(`http://${domain}${path}`)}`,
  ).stdout
}

/** The backend labels that answered `count` HTTPS requests, sorted and unique. */
export function answeringBackends(domain: string, count = 8): string[] {
  const seen = new Set<string>()
  for (let i = 0; i < count; i++) {
    const match = /backend=(\w+)/.exec(httpsBody(domain))
    seen.add(match ? match[1]! : 'none')
  }
  return [...seen].sort()
}

export function fileExists(path: string): boolean {
  return inContainer(`test -e ${shellQuote(path)}`).code === 0
}

export function readFile(path: string): string {
  return mustInContainer(`cat ${shellQuote(path)}`)
}

export function sitePaths(name: string) {
  return {
    available: `/etc/nginx/sites-available/ingress-${name}`,
    enabled: `/etc/nginx/sites-enabled/ingress-${name}`,
    upstream: (backend: string) => `/etc/nginx/conf.d/upstream-ig-${name}-${backend}.conf`,
  }
}
