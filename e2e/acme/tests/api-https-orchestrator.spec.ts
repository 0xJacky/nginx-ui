import type { AcmeApi } from '../lib/api'
import type { IssueResult } from '../lib/issue'
import { expect, test } from '@playwright/test'
import { createApi } from '../lib/api'
import { httpGet, httpsGet, readSiteConfig, servedCertificateIssuer } from '../lib/docker'
import { describeIssue, enableHTTPSViaWebSocket, finalStepStatuses } from '../lib/issue'
import { httpServer, okLocation, uniqueDomain } from '../lib/sites'

// These specs drive GET /api/sites/:name/https, the orchestrator behind the
// HTTPS card: it stages the challenge route, verifies it, issues (or takes an
// existing certificate) and writes the final HTTPS config in one run. Sites
// start as disabled drafts, the way the add-site wizard saves them.

let api: AcmeApi

test.beforeAll(async () => {
  api = await createApi()
})

test.afterAll(async () => {
  await api?.dispose()
})

function expectEnabled(result: IssueResult) {
  expect(result.status, describeIssue(result)).toBe('success')
  expect(result.sslCertificate, describeIssue(result)).toBeTruthy()
  expect(result.certId, describeIssue(result)).toBeGreaterThan(0)
}

async function expectServesHTTPS(domain: string, marker: string) {
  await expect.poll(() => httpsGet(domain).body).toContain(marker)
  expect(httpsGet(domain).status).toBe(200)
  expect(servedCertificateIssuer(domain)).toContain('Pebble')
}

test('a plain HTTP draft gets a certificate, an HTTPS server and a redirect in one run', async ({ page }) => {
  const domain = uniqueDomain('draft')
  await api.saveSite(domain, httpServer(domain, okLocation('draft-app')))

  const result = await enableHTTPSViaWebSocket(page, domain, { domains: [domain] })
  expectEnabled(result)

  // The HTTPS server is built from the port-80 app content.
  await expectServesHTTPS(domain, 'draft-app')
  const http = httpGet(domain)
  expect(http.status).toBe(301)
  expect(http.location).toBe(`https://${domain}/`)
})

test('an existing certificate enables HTTPS without contacting the CA', async ({ page }) => {
  // Issue a certificate for the domain through a first site, then disable that
  // site so a second one can take the domain over. Disabling only pauses the
  // certificate's renewal; the record stays available for reuse.
  const domain = uniqueDomain('reuse')
  const owner = `${domain}.owner`
  await api.saveSite(owner, httpServer(domain, okLocation('owner-app')))
  const issued = await enableHTTPSViaWebSocket(page, owner, { domains: [domain] })
  expectEnabled(issued)
  await api.disableSite(owner)
  const pausedCert = await (await api.context.get(`/api/certs/${issued.certId}`)).json()
  expect(pausedCert.auto_cert).toBe(-2)

  await api.saveSite(domain, httpServer(domain, okLocation('reuse-app')))
  const result = await enableHTTPSViaWebSocket(page, domain, { domains: [domain], certificate_id: issued.certId })

  expectEnabled(result)
  expect(result.certId).toBe(issued.certId)
  expect(result.sslCertificate).toBe(issued.sslCertificate)
  const steps = finalStepStatuses(result)
  expect(steps.probe, describeIssue(result)).toBe('skipped')
  expect(steps.issue, describeIssue(result)).toBe('skipped')

  await expectServesHTTPS(domain, 'reuse-app')
  expect(readSiteConfig(domain)).toContain(`ssl_certificate ${issued.sslCertificate};`)

  // Enabling the original site again resumes the renewal it had.
  await api.saveSite(owner, httpServer(uniqueDomain('moved'), okLocation('owner-app')))
  await api.enableSite(owner)
  const resumedCert = await (await api.context.get(`/api/certs/${issued.certId}`)).json()
  expect(resumedCert.auto_cert).toBe(1)
})

test('an existing certificate that covers none of the domains is refused and nothing is written', async ({ page }) => {
  const certified = uniqueDomain('covered')
  const owner = `${certified}.owner`
  await api.saveSite(owner, httpServer(certified, okLocation('covered-app')))
  const issued = await enableHTTPSViaWebSocket(page, owner, { domains: [certified] })
  expectEnabled(issued)

  const domain = uniqueDomain('uncovered')
  const draft = httpServer(domain, okLocation('uncovered-app'))
  await api.saveSite(domain, draft)
  const before = readSiteConfig(domain)

  const result = await enableHTTPSViaWebSocket(page, domain, { domains: [domain], certificate_id: issued.certId })

  expect(result.status, describeIssue(result)).toBe('error')
  expect(result.hint?.code, describeIssue(result)).toBe('certificate_does_not_cover_domains')
  expect(readSiteConfig(domain)).toBe(before)
})
