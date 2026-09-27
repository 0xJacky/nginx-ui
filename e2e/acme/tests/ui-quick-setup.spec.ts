import { expect, test } from '@playwright/test'
import { httpGet, httpsGet, readSiteConfig, servedCertificateIssuer, waitForHTTP } from '../lib/docker'
import { addresses } from '../lib/env'
import { uniqueDomain } from '../lib/sites'
import { SiteAddPage } from '../pages/siteAdd'

test('Quick Setup reverse proxy with TLS and redirect issues a certificate and serves HTTPS', async ({ page }) => {
  const domain = uniqueDomain('wizard')
  const wizard = new SiteAddPage(page)

  await wizard.open()
  await wizard.quickSetupReverseProxy({
    name: domain,
    domains: [domain],
    upstreamHost: addresses.backend,
    upstreamPort: '80',
    enableTLS: true,
    redirectHTTPToHTTPS: true,
  })
  await wizard.skipDNSStep()

  const outcome = await wizard.issueCertificate()
  expect(outcome.status, JSON.stringify(outcome.frames, null, 2)).toBe('success')

  await wizard.finish()

  // HTTPS reaches the backend with a Pebble certificate.
  await expect.poll(() => httpsGet(domain).status, { message: readSiteConfig(domain) }).toBe(200)
  expect(httpsGet(domain).body).toContain('Welcome to nginx')
  expect(servedCertificateIssuer(domain)).toContain('Pebble')

  // HTTP redirects to HTTPS...
  const redirect = await waitForHTTP(domain, probe => probe.status === 301)
  expect(redirect.location).toBe(`https://${domain}/`)

  // ...except the challenge path, which still goes to the HTTP-01 listener so
  // renewals keep working. Nothing listens there between issuances, so Nginx
  // answers 502 from the proxy rather than 301 or the backend's 404.
  const challenge = httpGet(domain, '/.well-known/acme-challenge/renewal-probe')
  expect(challenge.status, `${challenge.status} ${challenge.location}\n${readSiteConfig(domain)}`).toBe(502)
})
