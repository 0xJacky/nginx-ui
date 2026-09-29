import type { Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { authHeaders, gotoRoute, waitForApiResponse } from './helpers'

// Needs a writable instance (NGINX_UI_NODE_DEMO=false): it creates
// certificates and a site, and enables HTTPS on it.

interface CreatedCert {
  id: number
  name: string
}

interface RecommendationResponse {
  certificate: { id: number, name: string } | null
}

const siteName = 'e2e-cert-recommend'
const siteDomain = 'www.e2e-abc.cn'

async function fillFormItem(page: Page, label: string, value: string) {
  const item = page.locator('.ant-form-item').filter({ has: page.getByText(label, { exact: true }) }).first()
  await item.locator('input, textarea').first().fill(value)
}

async function createSelfSignedCert(page: Page, name: string, domain: string): Promise<CreatedCert> {
  const response = await page.request.post('/api/self_signed_cert', {
    headers: await authHeaders(page),
    data: { name, domains: [domain], key_type: 'P256' },
  })
  expect(response.ok(), await response.text()).toBe(true)
  return await response.json() as CreatedCert
}

async function openSSLStep(page: Page) {
  await gotoRoute(page, '/sites/add')
  await fillFormItem(page, 'Configuration Name', siteName)
  await fillFormItem(page, 'Domains', siteDomain)

  const quickConfig = waitForApiResponse(page, '/api/templates/quick_config', 'POST')
  await page.getByRole('button', { name: 'Next', exact: true }).click()
  expect((await quickConfig).ok()).toBe(true)
  await expect(page.locator('.ant-steps-item-active')).toContainText('DNS Record')

  const recommendation = waitForApiResponse(page, '/api/cert_recommendation', 'POST')
  await page.getByRole('button', { name: 'Next', exact: true }).click()
  await expect(page.locator('.ant-steps-item-active')).toContainText('Configure SSL')
  return recommendation
}

function domainTag(page: Page, domain: string) {
  return page.locator('.https-card .ant-tag').filter({ hasText: domain })
}

function methodSegment(page: Page) {
  return page.locator('.ant-segmented').filter({ hasText: 'Existing certificate' }).first()
}

test('the SSL step preselects the certificate that covers the site', async ({ page }) => {
  await gotoRoute(page, '/')
  const headers = await authHeaders(page)
  const created: CreatedCert[] = []

  try {
    created.push(await createSelfSignedCert(page, 'e2e-recommend-def', '*.e2e-def.cn'))
    const abc = await createSelfSignedCert(page, 'e2e-recommend-abc', '*.e2e-abc.cn')
    created.push(abc)

    const recommendation = await (await openSSLStep(page)).json() as RecommendationResponse
    expect(recommendation.certificate?.id).toBe(abc.id)

    // The card starts on "Existing certificate" with *.e2e-abc.cn selected.
    await expect(methodSegment(page).locator('.ant-segmented-item-selected')).toHaveText('Existing certificate')
    const selected = page.locator('.https-certificate')
    await expect(selected).toContainText('e2e-recommend-abc')
    await expect(selected.locator('.ant-tag').filter({ hasText: 'Recommended' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Enable HTTPS and finish', exact: true })).toBeEnabled()

    // No certificate covers both domains: the recommendation is withdrawn.
    const withdrawn = waitForApiResponse(page, '/api/cert_recommendation', 'POST')
    const domainInput = page.getByPlaceholder('e.g. example.com www.example.com')
    await domainInput.fill('www.e2e-def.cn')
    await domainInput.press('Enter')
    await expect(domainTag(page, 'www.e2e-def.cn')).toBeVisible()
    expect((await (await withdrawn).json() as RecommendationResponse).certificate).toBeNull()
    await expect(methodSegment(page).locator('.ant-segmented-item-selected')).not.toHaveText('Existing certificate')
    await expect(page.locator('.https-certificate')).toHaveCount(0)

    // Back to the site's own domain: the recommendation returns.
    const restored = waitForApiResponse(page, '/api/cert_recommendation', 'POST')
    await domainTag(page, 'www.e2e-def.cn').locator('.ant-tag-close-icon').click()
    expect((await (await restored).json() as RecommendationResponse).certificate?.id).toBe(abc.id)
    await expect(methodSegment(page).locator('.ant-segmented-item-selected')).toHaveText('Existing certificate')

    // Finishing installs the recommended certificate on the site.
    await page.getByRole('button', { name: 'Enable HTTPS and finish', exact: true }).click()
    await expect(page.getByText('Site Config Created Successfully', { exact: true })).toBeVisible({ timeout: 60_000 })

    const site = await page.request.get(`/api/sites/${siteName}`, { headers })
    expect(site.ok()).toBe(true)
    const config = (await site.json() as { config: string }).config
    expect(config).toContain('listen 443 ssl')
    const certRecord = await page.request.get(`/api/certs/${abc.id}`, { headers })
    const certPath = (await certRecord.json() as { ssl_certificate_path: string }).ssl_certificate_path
    expect(config).toContain(`ssl_certificate ${certPath};`)
  }
  finally {
    await page.request.post(`/api/sites/${siteName}/disable`, { headers })
    await page.request.delete(`/api/sites/${siteName}`, { headers })
    for (const cert of created)
      await page.request.delete(`/api/certs/${cert.id}`, { headers })
  }
})

test('a method chosen by hand is not replaced by a recommendation', async ({ page }) => {
  await gotoRoute(page, '/')
  const headers = await authHeaders(page)
  const abc = await createSelfSignedCert(page, 'e2e-recommend-lock', '*.e2e-abc.cn')

  try {
    await (await openSSLStep(page)).finished()
    await expect(methodSegment(page).locator('.ant-segmented-item-selected')).toHaveText('Existing certificate')

    await methodSegment(page).getByText('HTTP-01', { exact: true }).click()
    await expect(methodSegment(page).locator('.ant-segmented-item-selected')).toHaveText('HTTP-01')

    // A later domain change asks nothing more and keeps the user's method.
    const domainInput = page.getByPlaceholder('e.g. example.com www.example.com')
    await domainInput.fill('e2e-abc.cn')
    await domainInput.press('Enter')
    await page.waitForTimeout(1_000)
    await expect(methodSegment(page).locator('.ant-segmented-item-selected')).toHaveText('HTTP-01')
  }
  finally {
    await page.request.delete(`/api/sites/${siteName}`, { headers })
    await page.request.delete(`/api/certs/${abc.id}`, { headers })
  }
})
