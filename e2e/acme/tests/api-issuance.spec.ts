import type { AcmeApi } from '../lib/api'
import type { IssueResult } from '../lib/issue'
import { expect, test } from '@playwright/test'
import { createApi } from '../lib/api'
import { clearDomain, pointDomainTo } from '../lib/dns'
import { httpsGet, servedCertificateIssuer, waitForHTTP, waitForMarker, writeContainerFile } from '../lib/docker'
import { addresses } from '../lib/env'
import { describeIssue, issueViaWebSocket } from '../lib/issue'
import { challengeLocation, challengePort, httpServer, okLocation, tlsServer, uniqueDomain } from '../lib/sites'

// Each test owns a unique domain and site, so none depends on another's state.
// They are written against the intended behavior: a valid challenge route
// must issue no matter how the config spells it, and failures must carry the
// CA's reason (plus an actionable hint when the backend attaches one).
//
// Except for the first test, every spec waits until Nginx really serves a
// saved config before issuing: `nginx -s reload` returns before the new
// workers take over. The first test covers that window on purpose, because
// the site wizard enables a site and starts issuance right away.

let api: AcmeApi

test.beforeAll(async () => {
  api = await createApi()
})

test.afterAll(async () => {
  await api?.dispose()
})

function expectIssued(result: IssueResult) {
  expect(result.status, describeIssue(result)).toBe('success')
  expect(result.sslCertificate, describeIssue(result)).toBeTruthy()
  expect(result.sslCertificateKey, describeIssue(result)).toBeTruthy()
}

function expectPebbleCertificate(domain: string) {
  expect(servedCertificateIssuer(domain)).toContain('Pebble')
}

test('issuance started right after enabling a site does not race the Nginx reload', async ({ page }) => {
  const domain = uniqueDomain('fresh')
  await api.createEnabledSite(domain, httpServer(domain, challengeLocation(), okLocation('fresh-http')))

  // No wait: this is what the site wizard does.
  const result = await issueViaWebSocket(page, domain, [domain])
  expectIssued(result)
})

test('a plain site with the challenge location issues and then serves a Pebble certificate over HTTPS', async ({ page }) => {
  const domain = uniqueDomain('plain')
  await api.createEnabledSite(domain, httpServer(domain, challengeLocation(), okLocation('plain-http')))
  await waitForMarker(domain, 'plain-http')

  const result = await issueViaWebSocket(page, domain, [domain])
  expectIssued(result)

  await api.saveSite(domain, [
    httpServer(domain, challengeLocation(), okLocation('plain-http')),
    tlsServer(domain, result.sslCertificate!, result.sslCertificateKey!, okLocation('plain-https')),
  ].join('\n'))

  await expect.poll(() => httpsGet(domain).body).toContain('plain-https')
  expect(httpsGet(domain).status).toBe(200)
  expectPebbleCertificate(domain)
})

test('a challenge location pulled in through an include snippet issues', async ({ page }) => {
  const domain = uniqueDomain('include')
  writeContainerFile('/etc/nginx/snippets/acme.conf', `${challengeLocation()}\n`)
  await api.createEnabledSite(domain, httpServer(domain, '    include /etc/nginx/snippets/acme.conf;', okLocation('include-http')))
  await waitForMarker(domain, 'include-http')

  const result = await issueViaWebSocket(page, domain, [domain])
  expectIssued(result)
})

test('a challenge location proxied to localhost instead of 127.0.0.1 issues', async ({ page }) => {
  const domain = uniqueDomain('localhost')
  await api.createEnabledSite(domain, httpServer(domain, challengeLocation(`http://localhost:${challengePort}`), okLocation('localhost-http')))
  await waitForMarker(domain, 'localhost-http')

  const result = await issueViaWebSocket(page, domain, [domain])
  expectIssued(result)
})

// Let's Encrypt, and Pebble's VA which mirrors it, follows redirects during
// HTTP-01 validation (Go's default client: up to 10 hops, TLS not verified),
// so a site whose port 80 only redirects to HTTPS can renew as long as the
// HTTPS server routes the challenge.
async function issueThenRedirectEverything(page: import('@playwright/test').Page, domain: string, challengeOnTLS: boolean) {
  await api.createEnabledSite(domain, httpServer(domain, challengeLocation(), okLocation('redirect-http')))
  await waitForMarker(domain, 'redirect-http')
  const first = await issueViaWebSocket(page, domain, [domain])
  expectIssued(first)

  const tlsBody = [okLocation('redirect-https')]
  if (challengeOnTLS) {
    tlsBody.unshift(`    location ^~ /.well-known/acme-challenge/ {
        proxy_set_header Host $host;
        proxy_pass http://127.0.0.1:${challengePort};
    }`)
  }
  await api.saveSite(domain, [
    httpServer(domain, '    return 301 https://$host$request_uri;'),
    tlsServer(domain, first.sslCertificate!, first.sslCertificateKey!, ...tlsBody),
  ].join('\n'))

  const redirect = await waitForHTTP(domain, probe => probe.status === 301, '/.well-known/acme-challenge/probe')
  expect(redirect.location).toBe(`https://${domain}/.well-known/acme-challenge/probe`)
  expectPebbleCertificate(domain)

  return issueViaWebSocket(page, domain, [domain])
}

test('a site whose port 80 only redirects to HTTPS reissues when the HTTPS server routes the challenge', async ({ page }) => {
  const result = await issueThenRedirectEverything(page, uniqueDomain('redirect-ok'), true)
  expectIssued(result)
})

test('a site whose port 80 only redirects to HTTPS fails to reissue when the HTTPS server lacks the challenge', async ({ page }) => {
  const result = await issueThenRedirectEverything(page, uniqueDomain('redirect-missing'), false)
  expect(result.status, describeIssue(result)).toBe('error')
})

test.describe('DNS overrides', () => {
  const overridden: string[] = []

  test.afterEach(async () => {
    while (overridden.length > 0)
      await clearDomain(overridden.pop()!)
  })

  test('a domain resolving to another web server fails as unauthorized', async ({ page }) => {
    const domain = uniqueDomain('elsewhere')
    await api.createEnabledSite(domain, httpServer(domain, challengeLocation(), okLocation('elsewhere-http')))
    await waitForMarker(domain, 'elsewhere-http')
    overridden.push(domain)
    await pointDomainTo(domain, addresses.backend)

    const result = await issueViaWebSocket(page, domain, [domain])
    expect(result.status, describeIssue(result)).toBe('error')
    expect(result.message, describeIssue(result)).toMatch(/unauthorized/i)
    if (result.hint)
      expect(['dns_points_elsewhere', 'challenge_not_served'], describeIssue(result)).toContain(result.hint.code)
  })

  test('a domain resolving to an unused address fails with a connection error', async ({ page }) => {
    const domain = uniqueDomain('unreachable')
    await api.createEnabledSite(domain, httpServer(domain, challengeLocation(), okLocation('unreachable-http')))
    await waitForMarker(domain, 'unreachable-http')
    overridden.push(domain)
    await pointDomainTo(domain, addresses.unused)

    const result = await issueViaWebSocket(page, domain, [domain])
    expect(result.status, describeIssue(result)).toBe('error')
    expect(result.message, describeIssue(result)).toMatch(/connection/i)
  })
})
