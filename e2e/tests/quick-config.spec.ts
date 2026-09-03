import { expect, test } from '@playwright/test'
import { authHeaders, gotoRoute, waitForApiResponse } from './helpers'

interface QuickConfigResponse {
  template: string
  tokenized: {
    name?: string
  }
}

async function fillFormItem(page: import('@playwright/test').Page, label: string, value: string) {
  const item = page.locator('.ant-form-item').filter({ has: page.getByText(label, { exact: true }) }).first()
  await item.locator('input, textarea').first().fill(value)
}

async function toggleSwitch(page: import('@playwright/test').Page, label: string) {
  const item = page.locator('.ant-form-item').filter({ has: page.getByText(label, { exact: true }) }).first()
  await item.locator('.ant-switch').click()
}

async function selectType(page: import('@playwright/test').Page, type: 'Reverse Proxy' | 'Static Site' | 'Redirect') {
  await page.locator('.ant-radio-button-wrapper').filter({ hasText: type }).click()
}

async function submitQuickConfig(page: import('@playwright/test').Page): Promise<QuickConfigResponse> {
  const responsePromise = waitForApiResponse(page, '/api/templates/quick_config', 'POST')
  await page.getByRole('button', { name: 'Next', exact: true }).click()
  const response = await responsePromise
  expect(response.ok()).toBe(true)
  return await response.json() as QuickConfigResponse
}

test('quick setup requires name and domains before enabling Next', async ({ page }) => {
  await gotoRoute(page, '/sites/add')

  await expect(page.getByText('Quick Setup', { exact: true })).toBeVisible()
  const next = page.getByRole('button', { name: 'Next', exact: true })
  await expect(next).toBeDisabled()

  await fillFormItem(page, 'Configuration Name', 'e2e-empty-domains')
  await expect(next).toBeDisabled()

  await fillFormItem(page, 'Domains', 'e2e-empty-domains.example.com')
  await expect(next).toBeEnabled()
})

test('quick setup reverse proxy without TLS saves a site end to end', async ({ page }) => {
  const name = 'e2e-rp-plain'
  const domain = `${name}.example.com`

  await gotoRoute(page, '/sites/add')
  await fillFormItem(page, 'Configuration Name', name)
  await fillFormItem(page, 'Domains', domain)

  const result = await submitQuickConfig(page)
  expect(result.tokenized.name).toBe(domain)
  expect(result.template).toContain('proxy_pass http://127.0.0.1:9000/')
  expect(result.template).toContain('client_max_body_size 1000m')
  expect(result.template).not.toContain('return 301')
  expect(result.template).not.toContain('listen 443 ssl')

  await expect(page.locator('.ant-steps-item-active')).toContainText('DNS Record')

  await page.getByRole('button', { name: 'Next', exact: true }).click()
  await expect(page.locator('.ant-steps-item-active')).toContainText('Configure SSL')
  await expect(page.getByText('Issue a certificate to enable TLS before continuing.')).not.toBeVisible()

  const savePromise = waitForApiResponse(page, `/api/sites/${name}`, 'POST')
  await page.getByRole('button', { name: 'Next', exact: true }).click()
  const saveResponse = await savePromise
  expect(saveResponse.ok()).toBe(true)

  await expect(page.getByText('Site Config Created Successfully', { exact: true })).toBeVisible()

  const headers = await authHeaders(page)
  const disableResponse = await page.request.post(`/api/sites/${name}/disable`, { headers })
  expect(disableResponse.ok()).toBe(true)
  const deleteResponse = await page.request.delete(`/api/sites/${name}`, { headers })
  expect(deleteResponse.ok()).toBe(true)
})

test('quick setup reverse proxy with TLS emits redirect, websocket and acme-challenge blocks and gates on a missing certificate', async ({ page }) => {
  await gotoRoute(page, '/sites/add')
  await fillFormItem(page, 'Configuration Name', 'e2e-rp-tls')
  await fillFormItem(page, 'Domains', 'e2e-rp-tls.example.com www.e2e-rp-tls.example.com')
  await fillFormItem(page, 'Client Max Body Size', '100m')
  await toggleSwitch(page, 'Enable TLS')

  const result = await submitQuickConfig(page)
  expect(result.template).toContain('return 301 https://$host$request_uri')
  expect(result.template).toContain('listen 443 ssl')
  expect(result.template).toContain('proxy_set_header Upgrade $http_upgrade')
  expect(result.template).toContain('client_max_body_size 100m')
  expect(result.template).toContain('location ~ /.well-known/acme-challenge')
  expect(result.tokenized.name).toBe('e2e-rp-tls.example.com')

  await expect(page.locator('.ant-steps-item-active')).toContainText('DNS Record')

  await page.getByRole('button', { name: 'Next', exact: true }).click()
  await expect(page.locator('.ant-steps-item-active')).toContainText('Configure SSL')

  await expect(page.getByText('Issue a certificate to enable TLS before continuing.', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Next', exact: true })).toBeDisabled()
})

test('quick setup static site emits root, index and SPA fallback', async ({ page }) => {
  await gotoRoute(page, '/sites/add')
  await fillFormItem(page, 'Configuration Name', 'e2e-static')
  await selectType(page, 'Static Site')
  await fillFormItem(page, 'Domains', 'e2e-static.example.com')
  await fillFormItem(page, 'Web Root', '/var/www/e2e-static')
  await toggleSwitch(page, 'Single Page Application Fallback')

  const result = await submitQuickConfig(page)
  expect(result.template).toContain('root /var/www/e2e-static;')
  expect(result.template).toContain('index index.html;')
  expect(result.template).toContain('try_files $uri $uri/ /index.html;')
})

test('quick setup redirect emits the chosen status code', async ({ page }) => {
  await gotoRoute(page, '/sites/add')
  await fillFormItem(page, 'Configuration Name', 'e2e-redirect')
  await selectType(page, 'Redirect')
  await fillFormItem(page, 'Domains', 'e2e-redirect.example.com')
  await fillFormItem(page, 'Target URL', 'https://new.example.com')

  const statusItem = page.locator('.ant-form-item').filter({ has: page.getByText('Status Code', { exact: true }) }).first()
  await statusItem.getByRole('combobox').click()
  // antdv-next renders dropdown options without role="option" or aria-selected, so the
  // option itself still has to be matched by its class rather than by role.
  await page.locator('.ant-select-item-option').filter({ hasText: '308 Permanent Redirect' }).click()

  const result = await submitQuickConfig(page)
  expect(result.template).toContain('return 308 https://new.example.com;')
})

test('quick setup on the edit page prefills and regenerates an existing site', async ({ page }) => {
  const name = 'e2e-edit-rp'
  const domain = `${name}.example.com`

  // Create the site through the API using quick config generation.
  await gotoRoute(page, '/')
  const headers = await authHeaders(page)
  const genResponse = await page.request.post('/api/templates/quick_config', {
    headers,
    data: {
      type: 'reverse_proxy',
      domains: [domain],
      host: '127.0.0.1',
      port: '9000',
      enable_websocket: true,
      client_max_body_size: '1000m',
    },
  })
  expect(genResponse.ok()).toBe(true)
  const gen = await genResponse.json() as QuickConfigResponse
  const saveResponse = await page.request.post(`/api/sites/${name}`, {
    headers,
    data: { name, content: gen.template, overwrite: true },
  })
  expect(saveResponse.ok()).toBe(true)

  try {
    await gotoRoute(page, `/sites/${name}`)
    const quickSetupButton = page.getByRole('button', { name: /Quick Setup/ })
    await expect(quickSetupButton).toBeVisible()
    await quickSetupButton.click()

    // The modal prefills from the existing config and enables generation.
    const modal = page.locator('.ant-modal').filter({ has: page.getByText('Generate Config') })
    await expect(modal).toBeVisible()
    await expect(page.getByRole('button', { name: 'Generate Config', exact: true })).toBeEnabled()

    // Change the proxy port and regenerate.
    const portItem = modal.locator('.ant-form-item').filter({ has: page.getByText('Port', { exact: true }) }).first()
    await portItem.locator('input').fill('8080')

    const regenPromise = waitForApiResponse(page, '/api/templates/quick_config', 'POST')
    await page.getByRole('button', { name: 'Generate Config', exact: true }).click()
    const regen = await regenPromise
    expect(regen.ok()).toBe(true)

    // Confirm the destructive replace, then wait for the save before reading it back.
    await page.getByRole('button', { name: 'Replace', exact: true }).click()
    await expect(modal).toBeHidden()

    const updatePromise = waitForApiResponse(page, `/api/sites/${name}`, 'POST')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    const updateResponse = await updatePromise
    expect(updateResponse.ok()).toBe(true)

    const getResponse = await page.request.get(`/api/sites/${name}`, { headers })
    expect(getResponse.ok()).toBe(true)
    const site = await getResponse.json() as { config: string }
    expect(site.config).toContain('proxy_pass http://127.0.0.1:8080/')
  }
  finally {
    await page.request.post(`/api/sites/${name}/disable`, { headers })
    await page.request.delete(`/api/sites/${name}`, { headers })
  }
})
