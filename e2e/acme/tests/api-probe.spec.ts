import type { AcmeApi } from '../lib/api'
import type { IssueResult } from '../lib/issue'
import { randomBytes } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { createApi } from '../lib/api'
import { clearDomain, pointDomainTo } from '../lib/dns'
import { httpGet, httpsGet, inContainer, readSiteConfig, servedCertificateDNSNames, servedCertificateIssuer, waitForHTTP, waitForMarker } from '../lib/docker'
import { addresses } from '../lib/env'
import { describeIssue, enableHTTPSViaWebSocket, finalStepStatuses, issueViaWebSocket, renderedMessages } from '../lib/issue'
import { challengeLocation, httpServer, okLocation, tlsServer, uniqueDomain } from '../lib/sites'

// The HTTP-01 route probe that runs before nginx-ui contacts the CA.
//
// - It finds the local endpoints of a domain from the effective Nginx config
//   (the server block Nginx picks for the name), not from a site file named
//   like the certificate. A certificate's name is only a description.
// - The legacy socket (GET /api/domain/:name/cert: certificate manager renew,
//   site editor, auto-renew) never stops on the probe; it logs the result and
//   lets the CA decide.
// - The HTTPS orchestrator (GET /api/sites/:name/https) still stops when the
//   local Nginx answered the challenge path with something other than the
//   token.
//
// Every test owns its domains and sites; none depends on another's state.

let api: AcmeApi

test.beforeAll(async () => {
  api = await createApi()
})

test.afterAll(async () => {
  await api?.dispose()
})

function randomLabel() {
  return randomBytes(3).toString('hex')
}

function expectIssued(result: IssueResult) {
  expect(result.status, describeIssue(result)).toBe('success')
  expect(result.sslCertificate, describeIssue(result)).toBeTruthy()
  expect(result.sslCertificateKey, describeIssue(result)).toBeTruthy()
}

// 50058 is the local "HTTP-01 challenge route check failed" block.
function expectNotBlockedByProbe(result: IssueResult) {
  expect(result.message, describeIssue(result)).not.toMatch(/challenge route check failed/i)
  expect(result.error?.code, describeIssue(result)).not.toBe(50058)
}

/** A site with the challenge location on port 80, enabled and serving. */
async function createChallengeSite(label: string) {
  const domain = uniqueDomain(label)
  const name = `probe-site-${randomLabel()}`
  const marker = `${label}-http`
  await api.createEnabledSite(name, httpServer(domain, challengeLocation(), okLocation(marker)))
  await waitForMarker(domain, marker)
  return { domain, name, marker }
}

test.describe('certificate name that matches no site', () => {
  // Field report: renewing from the certificate manager sends the
  // certificate's name, which no site file carries.
  test('a certificate named after no site issues for the site serving its domain', async ({ page }) => {
    const site = await createChallengeSite('described')

    const result = await issueViaWebSocket(page, `cert-description-${randomLabel()}.e2e.test`, [site.domain])
    expectIssued(result)
  })

  test('one certificate for the domains of two sites issues and both sites serve it', async ({ page }) => {
    const first = await createChallengeSite('bundle-a')
    const second = await createChallengeSite('bundle-b')

    const result = await issueViaWebSocket(page, `bundle-${randomLabel()}`, [first.domain, second.domain])
    expectIssued(result)

    for (const site of [first, second]) {
      await api.saveSite(site.name, [
        httpServer(site.domain, challengeLocation(), okLocation(site.marker)),
        tlsServer(site.domain, result.sslCertificate!, result.sslCertificateKey!, okLocation(`${site.marker}s`)),
      ].join('\n'))
    }

    const expected = [first.domain, second.domain].sort()
    for (const site of [first, second]) {
      await expect.poll(() => httpsGet(site.domain).body).toContain(`${site.marker}s`)
      expect(servedCertificateIssuer(site.domain)).toContain('Pebble')
      expect(servedCertificateDNSNames(site.domain)).toEqual(expected)
    }
  })
})

// The bundled conf.d/nginx-ui.conf listens on `80` (every address) and is the
// default server there, so a loopback request for this domain reaches
// nginx-ui's own UI instead of the site. Only the container address routes it.
//
// Any `listen <address>:80` makes Nginx pick servers for that address from the
// blocks listening on it alone, and the first one becomes its default server.
// The published UI port arrives on the container address too, so the site
// first declares a default server there that proxies to the UI exactly like
// conf.d/nginx-ui.conf; otherwise the site would answer every UI request.
//
// For the same reason, while this site is enabled every request on the
// container address, including Pebble's validation of every other test
// domain, only sees the servers listening on that address. The site is
// disabled again when the test ends so it cannot break later tests.
test.describe('site on the container address only', () => {
  let enabled: { name: string, domain: string, marker: string } | undefined

  test.afterEach(async () => {
    if (!enabled)
      return
    const { name, domain, marker } = enabled
    enabled = undefined
    await api.disableSite(name)
    // Wait out the reload so the next test's validation sees the wildcard
    // servers on the container address again.
    await waitForHTTP(domain, probe => !probe.body.includes(marker), '/', 15_000, addresses.nginxUI)
  })

  test('a site listening only on the container address issues and the probe reaches it there', async ({ page }) => {
    const domain = uniqueDomain('ip-only')
    const name = `probe-site-${randomLabel()}`
    const marker = 'ip-only-http'
    enabled = { name, domain, marker }
    await api.createEnabledSite(name, `server {
    listen ${addresses.nginxUI}:80 default_server;
    server_name _;
    client_max_body_size 128M;

    location / {
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $forwarded_proto;
        proxy_set_header X-Forwarded-Host $forwarded_host;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_pass http://127.0.0.1:9000/;
    }
}

server {
    listen ${addresses.nginxUI}:80;
    server_name ${domain};

${challengeLocation()}

${okLocation(marker)}
}
`)
    await waitForMarker(domain, marker, addresses.nginxUI)
    // The premise: loopback does not reach this site.
    expect(httpGet(domain).body).not.toContain(marker)

    // A certificate name that is no site file, as in the field report, so the
    // endpoint can only come from the effective config.
    const result = await issueViaWebSocket(page, `cert-description-${randomLabel()}.e2e.test`, [domain])
    expectIssued(result)

    const viaContainerAddress = renderedMessages(result).filter(line =>
      line.includes(domain)
      && line.includes(addresses.nginxUI)
      && !/fail|cannot|unable|refused|error/i.test(line))
    expect(viaContainerAddress, describeIssue(result)).not.toHaveLength(0)
  })
})

test.describe('legacy socket never stops on the probe', () => {
  const overridden: string[] = []

  test.afterEach(async () => {
    while (overridden.length > 0)
      await clearDomain(overridden.pop()!)
  })

  test('without a challenge location and DNS at an unused address, the error comes from the CA', async ({ page }) => {
    const domain = uniqueDomain('no-route-unreachable')
    await api.createEnabledSite(domain, httpServer(domain, okLocation('no-route-unreachable-http')))
    await waitForMarker(domain, 'no-route-unreachable-http')
    overridden.push(domain)
    await pointDomainTo(domain, addresses.unused)

    const result = await issueViaWebSocket(page, domain, [domain])
    expect(result.status, describeIssue(result)).toBe('error')
    expectNotBlockedByProbe(result)
    expect(result.message, describeIssue(result)).toMatch(/connection/i)
  })

  test('without a challenge location, the local route failure is logged and the CA still decides', async ({ page }) => {
    const domain = uniqueDomain('no-route')
    await api.createEnabledSite(domain, httpServer(domain, okLocation('no-route-http')))
    await waitForMarker(domain, 'no-route-http')

    const result = await issueViaWebSocket(page, domain, [domain])
    expect(result.status, describeIssue(result)).toBe('error')
    expectNotBlockedByProbe(result)
    // The CA fetched the challenge and got the site's page instead.
    expect(result.message, describeIssue(result)).toMatch(/unauthorized|urn:ietf:params:acme:error/i)

    // The probe result for the domain is in the log (not only the "Checking
    // ..." line that starts it) and reports the failure.
    const routeResults = renderedMessages(result).filter(line =>
      line.includes(domain)
      && /challenge route/i.test(line)
      && !/\bchecking\b/i.test(line))
    expect(routeResults, describeIssue(result)).not.toHaveLength(0)
    expect(routeResults.join('\n'), describeIssue(result)).toMatch(/fail|unexpected|mismatch/i)
  })
})

// The planner removes the draft's own challenge locations and appends
// `location ~ /.well-known/acme-challenge` (template/block/letsencrypt.conf).
// A `^~` prefix location that matches the challenge path wins over every
// regex location, so the injected route is shadowed and Nginx answers 404:
// HTTP evidence that the CA would fail the same way.
test('the HTTPS orchestrator stops at the probe when the challenge route is shadowed', async ({ page }) => {
  const domain = uniqueDomain('shadowed')
  await api.saveSite(domain, httpServer(domain, `    location ^~ /.well-known/ {
        return 404;
    }`, okLocation('shadowed-app')))

  const result = await enableHTTPSViaWebSocket(page, domain, { domains: [domain] })

  expect(result.status, describeIssue(result)).toBe('error')
  const steps = finalStepStatuses(result)
  expect(steps.probe, describeIssue(result)).toBe('error')
  expect(steps.issue, describeIssue(result)).toBeUndefined()
  expect(result.hint?.code, describeIssue(result)).toBe('challenge_route_unavailable')
  expect(`${result.message} ${JSON.stringify(result.hint)}`, describeIssue(result)).toContain('404')

  // The staged config did carry the injected route; the shadowing, not a
  // missing location, is what failed.
  const staged = readSiteConfig(domain)
  expect(staged).toContain('/.well-known/acme-challenge')
  expect(staged).toMatch(/location \^~ \/\.well-known\//)
})

// The quick-setup wizard writes `listen [::]:80`, so after it ran the IPv6
// socket has a default server that routes the challenge. A domain whose own
// (IPv4-only) server does not route it would then pass a probe that accepts a
// success on any socket, although the CA (IPv4 here: challtestsrv has no AAAA
// records) reaches the domain's own server. This group builds that condition
// on purpose instead of depending on test order.
test.describe('an unrelated IPv6 default server that routes the challenge', () => {
  let ipv6Default = ''

  test.beforeEach(async () => {
    ipv6Default = `probe-v6-default-${randomLabel()}`
    await api.createEnabledSite(ipv6Default, `server {
    listen [::]:80 default_server;
    server_name _;

${challengeLocation()}

    location / {
        return 404;
    }
}
`)
    await expect.poll(() => inContainer(`curl -s -o /dev/null -w '%{http_code}' -H 'Host: ${uniqueDomain('v6-check')}' http://[::1]/`).stdout).toBe('404')
  })

  test.afterEach(async () => {
    if (ipv6Default)
      await api.disableSite(ipv6Default)
    ipv6Default = ''
  })

  test('does not make the legacy probe report a route the domain lacks as reachable', async ({ page }) => {
    const domain = uniqueDomain('v6-masked')
    await api.createEnabledSite(domain, httpServer(domain, okLocation('v6-masked-http')))
    await waitForMarker(domain, 'v6-masked-http')

    const result = await issueViaWebSocket(page, domain, [domain])
    expect(result.status, describeIssue(result)).toBe('error')
    expectNotBlockedByProbe(result)
    expect(result.message, describeIssue(result)).toMatch(/unauthorized|urn:ietf:params:acme:error/i)

    const lines = renderedMessages(result).filter(line => line.includes(domain) && /challenge route/i.test(line))
    expect(lines.filter(line => /reachable/i.test(line) && !/not reachable|unreachable/i.test(line)), describeIssue(result)).toHaveLength(0)
    expect(lines.join('\n'), describeIssue(result)).toMatch(/fail|unexpected|mismatch/i)
  })

  test('does not let the HTTPS orchestrator pass a shadowed challenge route', async ({ page }) => {
    const domain = uniqueDomain('v6-shadowed')
    await api.saveSite(domain, httpServer(domain, `    location ^~ /.well-known/ {
        return 404;
    }`, okLocation('v6-shadowed-app')))

    const result = await enableHTTPSViaWebSocket(page, domain, { domains: [domain] })

    expect(result.status, describeIssue(result)).toBe('error')
    expect(finalStepStatuses(result).probe, describeIssue(result)).toBe('error')
    expect(finalStepStatuses(result).issue, describeIssue(result)).toBeUndefined()
    expect(result.hint?.code, describeIssue(result)).toBe('challenge_route_unavailable')
  })
})
