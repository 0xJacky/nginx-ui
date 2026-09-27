import { spawnSync } from 'node:child_process'
import { composeEnv, composeFile, project } from './env'

export interface ExecResult {
  code: number
  stdout: string
  stderr: string
}

/**
 * Run a shell command inside a compose service (nginx-ui by default) through
 * `docker compose exec`. `input` is piped to the command's stdin.
 */
export function inContainer(command: string, options: { service?: string, input?: string } = {}): ExecResult {
  const result = spawnSync('docker', [
    'compose',
    '-p',
    project,
    '-f',
    composeFile,
    'exec',
    '-T',
    options.service ?? 'nginx-ui',
    'sh',
    '-c',
    command,
  ], {
    env: composeEnv(),
    input: options.input,
    encoding: 'utf8',
    timeout: 120_000,
  })

  if (result.error)
    throw result.error

  return {
    code: result.status ?? -1,
    stdout: result.stdout ?? '',
    stderr: result.stderr ?? '',
  }
}

/** Like inContainer, but throws with the command output when it fails. */
export function mustInContainer(command: string, options: { service?: string, input?: string } = {}): string {
  const result = inContainer(command, options)
  if (result.code !== 0)
    throw new Error(`"${command}" exited with ${result.code}\nstdout: ${result.stdout}\nstderr: ${result.stderr}`)
  return result.stdout
}

function shellQuote(value: string) {
  return `'${value.replaceAll('\'', `'\\''`)}'`
}

/** Write a file inside the nginx-ui container, creating its parent directory. */
export function writeContainerFile(path: string, content: string) {
  const dir = path.slice(0, path.lastIndexOf('/')) || '/'
  mustInContainer(`mkdir -p ${shellQuote(dir)} && cat > ${shellQuote(path)}`, { input: content })
}

export function readInstallSecret(): string {
  return mustInContainer('cat /etc/nginx-ui/.install_secret').trim()
}

export interface HTTPProbe {
  status: number
  location: string
  body: string
}

// All probes run inside the nginx-ui container against its own Nginx, so the
// host never needs to resolve the test domains.

/** GET http://<domain><path> through the local Nginx without following redirects. */
export function httpGet(domain: string, path = '/'): HTTPProbe {
  return curlProbe(`--resolve ${shellQuote(`${domain}:80:127.0.0.1`)} ${shellQuote(`http://${domain}${path}`)}`)
}

/** GET https://<domain><path> through the local Nginx, ignoring trust (Pebble roots are random). */
export function httpsGet(domain: string, path = '/'): HTTPProbe {
  return curlProbe(`-k --resolve ${shellQuote(`${domain}:443:127.0.0.1`)} ${shellQuote(`https://${domain}${path}`)}`)
}

function curlProbe(args: string): HTTPProbe {
  const marker = '__E2E_CURL__'
  const result = inContainer(
    `curl -sS --max-time 15 -o /tmp/e2e-curl-body -w '${marker}%{http_code} %{redirect_url}\n' ${args}; cat /tmp/e2e-curl-body 2>/dev/null; rm -f /tmp/e2e-curl-body`,
  )
  const [meta, ...rest] = result.stdout.split(marker)[1]?.split('\n') ?? ['']
  const [status, location = ''] = (meta ?? '').split(' ')
  return {
    status: Number(status) || 0,
    location: location.trim(),
    body: rest.join('\n') + (result.stderr ? `\n[stderr] ${result.stderr}` : ''),
  }
}

/** Issuer of the certificate the local Nginx presents for <domain> on :443. */
export function servedCertificateIssuer(domain: string): string {
  return inContainer(
    `openssl s_client -connect 127.0.0.1:443 -servername ${shellQuote(domain)} </dev/null 2>/dev/null | openssl x509 -noout -issuer`,
  ).stdout.trim()
}

export function readSiteConfig(name: string): string {
  return inContainer(`cat ${shellQuote(`/etc/nginx/sites-available/${name}`)}`).stdout
}

/**
 * Poll http://<domain><path> until `accept` passes. `nginx -s reload` returns
 * before the new workers take over, so a request sent right after a save or
 * enable can still reach the old configuration.
 */
export async function waitForHTTP(domain: string, accept: (probe: HTTPProbe) => boolean, path = '/', timeoutMs = 15_000): Promise<HTTPProbe> {
  const deadline = Date.now() + timeoutMs
  let probe = httpGet(domain, path)
  while (!accept(probe)) {
    if (Date.now() > deadline)
      throw new Error(`http://${domain}${path} never matched; last answer ${probe.status} ${probe.location}\n${probe.body}`)
    await new Promise(resolve => setTimeout(resolve, 250))
    probe = httpGet(domain, path)
  }
  return probe
}

/** Wait until the local Nginx answers <domain> with a body containing <marker>. */
export function waitForMarker(domain: string, marker: string) {
  return waitForHTTP(domain, probe => probe.status === 200 && probe.body.includes(marker))
}
