import type { APIRequestContext } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { createApi } from '../lib/api'
import { mustInContainer, upgradeNginxUI } from '../lib/docker'
import { addresses, baseURL, upgradeImage } from '../lib/env'
import { describeIssue, issueViaWebSocket } from '../lib/issue'
import { uniqueDomain } from '../lib/sites'

// Upgrade from a release that solved DNS-01 in the core (2.x) to one where the
// DNS providers live in the official DNS-01 plugin. Start the stack on the old
// image (NGINX_UI_IMAGE) and name the new one in E2E_ACME_UPGRADE_IMAGE:
//
//   NGINX_UI_IMAGE=uozi/nginx-ui:v2.9.5 sh e2e/acme/scripts/up.sh
//   E2E_ACME_UPGRADE_IMAGE=nginx-ui-acme-e2e:local bun run --cwd e2e test:acme -- upgrade-dns01
//
// The DNS credential uses lego's httpreq provider against the dnsapi service,
// which publishes the TXT record in challtestsrv, so Pebble validates a real
// DNS-01 challenge before and after the upgrade.

test.skip(!upgradeImage, 'set E2E_ACME_UPGRADE_IMAGE and start the stack on the previous release')

const dns01PluginID = 'com.nginxui.dns01'
// pebble-config.json: certificates of this profile are valid for 180 seconds,
// so they are due for renewal (half the lifetime) after 90.
const shortProfile = 'e2erenew'
const renewalDueMs = 100_000

interface CertRecord {
  id: number
  domains?: string[]
  challenge_method?: string
  certificate_info?: { not_before: string }
  last_renewal_error?: string
}

interface PluginInfo {
  id: string
  version: string
  enabled: boolean
  status: string
  trust: string
}

async function getJSON<T>(context: APIRequestContext, path: string): Promise<T> {
  const response = await context.get(path)
  if (!response.ok())
    throw new Error(`GET ${path} answered ${response.status()}: ${await response.text()}`)
  return await response.json() as T
}

async function findCert(context: APIRequestContext, domain: string): Promise<CertRecord> {
  const list = await getJSON<{ data: CertRecord[] }>(context, '/api/certs?page_size=100')
  const found = list.data.find(record => record.domains?.includes(domain))
  if (!found)
    throw new Error(`no certificate record for ${domain}`)
  return getJSON<CertRecord>(context, `/api/certs/${found.id}`)
}

test('a DNS-01 certificate issued on 2.x still renews after the upgrade', async ({ page }) => {
  test.setTimeout(900_000)
  const api = await createApi()
  const domain = uniqueDomain('dns01-upgrade')
  const name = domain

  // On the old release: a DNS credential and a DNS-01 certificate.
  const created = await api.context.post('/api/dns_credentials', {
    data: {
      name: 'e2e httpreq',
      provider: 'HTTP request',
      provider_code: 'httpreq',
      code: 'httpreq',
      configuration: {
        credentials: { HTTPREQ_ENDPOINT: `http://${addresses.dnsapi}:8080` },
        additional: { HTTPREQ_PROPAGATION_TIMEOUT: '60', HTTPREQ_POLLING_INTERVAL: '2' },
      },
    },
  })
  expect(created.ok(), await created.text()).toBe(true)
  const credentialID = (await created.json() as { id: number }).id

  const issued = await issueViaWebSocket(page, name, [domain], {
    challengeMethod: 'dns01',
    timeoutMs: 180_000,
    // challtestsrv answers no SOA queries, so lego cannot find the
    // authoritative nameservers; wait a fixed minute instead. 2.x keeps the
    // flag in a column, the upgrade moves it into challenge_config.
    extra: { dns_credential_id: credentialID, profile: shortProfile, disable_authoritative_ns_propagation: true },
  })
  expect(issued.status, describeIssue(issued)).toBe('success')
  const issuedAt = Date.now()

  const before = await findCert(api.context, domain)
  expect(before.challenge_method).toBe('dns01')
  const firstNotBefore = before.certificate_info!.not_before

  // Upgrade once the certificate is due, so the renewal check the new release
  // runs at boot is the one that has to renew it.
  await page.waitForTimeout(Math.max(0, issuedAt + renewalDueMs - Date.now()))
  await upgradeNginxUI(upgradeImage, baseURL)
  expect(mustInContainer('nginx-ui --version')).toMatch(/^nginx-ui 3\./)

  // The official DNS-01 plugin is installed and enabled without the user.
  await expect.poll(async () => {
    const plugins = await getJSON<PluginInfo[]>(api.context, '/api/plugins')
    const plugin = plugins.find(item => item.id === dns01PluginID)
    return plugin ? `${plugin.status} enabled=${plugin.enabled} trust=${plugin.trust}` : 'missing'
  }, { timeout: 120_000, message: 'the DNS-01 plugin after the upgrade' }).toMatch(/enabled=true trust=official$/)

  // The credential and the certificate came through the upgrade untouched.
  const credential = await getJSON<{ provider_code: string }>(api.context, `/api/dns_credentials/${credentialID}`)
  expect(credential.provider_code).toBe('httpreq')

  // The automatic renewal renews the certificate on its own.
  let lastRecord = before
  await expect.poll(async () => {
    lastRecord = await findCert(api.context, domain)
    return lastRecord.certificate_info?.not_before !== firstNotBefore
  }, {
    timeout: 240_000,
    intervals: [5_000],
    message: 'the automatic renewal after the upgrade',
  }).toBe(true)
  test.info().annotations.push({
    type: 'auto renewal',
    description: `not_before ${firstNotBefore} -> ${lastRecord.certificate_info?.not_before}; last_renewal_error=${lastRecord.last_renewal_error ?? ''}`,
  })

  // Issuing again through the plugin works, the way a user renews by hand.
  const reissued = await issueViaWebSocket(page, name, [domain], {
    challengeMethod: 'dns01',
    timeoutMs: 180_000,
    extra: { dns_credential_id: credentialID, challenge_config: { disable_authoritative_ns_propagation: true } },
  })
  expect(reissued.status, describeIssue(reissued)).toBe('success')

  await api.dispose()
})
