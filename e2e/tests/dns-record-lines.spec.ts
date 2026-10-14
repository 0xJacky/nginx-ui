import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute } from './helpers'

test.use({ storageState: { cookies: [], origins: [] } })

const domains = [
  {
    id: 2017,
    domain: 'tencent.example.com',
    dns_credential_id: 1,
    dns_credential: { id: 1, name: 'Tencent test', provider: 'Tencent Cloud DNS', provider_code: 'tencentcloud' },
  },
  {
    id: 2018,
    domain: 'ali.example.com',
    dns_credential_id: 2,
    dns_credential: { id: 2, name: 'Ali test', provider: 'Alibaba Cloud DNS', provider_code: 'alidns' },
  },
]

interface Mutation {
  domainId: number
  method: string
  payload: { name: string, content: string, line?: string }
}

async function mockDNS(page: Page, theme: 'light' | 'dark') {
  const mutations: Mutation[] = []
  const browserErrors: string[] = []
  page.on('pageerror', error => browserErrors.push(error.message))

  await page.addInitScript(({ theme, domainIds }) => {
    localStorage.setItem('user', JSON.stringify({ token: 'dns-fixture-token', info: { id: 1, name: 'Test' } }))
    localStorage.setItem('settings', JSON.stringify({ language: 'en', theme, preference_theme: theme }))
    localStorage.setItem('nginx-ui-dns-groups-v1', JSON.stringify({ groupsByScope: {
      '1:0': [{ id: 'dns-lines', name: 'DNS line regression', domainIds, description: '', createdAt: '', updatedAt: '' }],
    } }))
  }, { theme, domainIds: domains.map(domain => domain.id) })

  await page.routeWebSocket(/\/api\//, () => {})
  await page.route(url => url.pathname.startsWith('/api/'), async route => {
    const request = route.request()
    const path = new URL(request.url()).pathname
    const match = path.match(/^\/api\/dns\/domains\/(\d+)(?:\/(record-lines|records)(?:\/([^/]+))?)?$/)
    let response: unknown = { data: [], pagination: { total: 0, current_page: 1, per_page: 100 } }
    if (path === '/api/dns/domains') {
      response = { data: domains, pagination: { total: domains.length, current_page: 1, per_page: 100 } }
    }
    else if (match) {
      const domainId = Number(match[1])
      if (!match[2]) {
        response = domains.find(domain => domain.id === domainId)
      }
      else if (match[2] === 'record-lines') {
        response = { data: domainId === 2017
          ? ['默认', '电信', '联通', '移动', '办公网'].map(code => ({ code, name: code }))
          : [{ code: 'default', name: 'Default' }] }
      }
      else if (request.method() === 'GET') {
        response = {
          data: [{ id: '42', type: 'A', name: 'www', content: '192.0.2.1', ttl: 600, line: domainId === 2017 ? '电信' : 'default' }],
          pagination: { total: 1, current_page: 1, per_page: 100 },
        }
      }
      else {
        const payload = request.postDataJSON()
        mutations.push({ domainId, method: request.method(), payload })
        response = { id: match[3] ?? '43', ...payload }
      }
    }
    else if (path === '/api/nginx/modules') {
      response = []
    }
    else if (path === '/api/user') {
      response = { id: 1, name: 'Test', language: 'en' }
    }
    else if (path === '/api/token/short') {
      response = { short_token: 'fixture-short-token' }
    }
    else if (path === '/api/settings/server/name') {
      response = { name: 'DNS regression' }
    }
    else if (path === '/api/2fa_status') {
      response = { enabled: false, otp_status: false, passkey_status: false }
    }
    await route.fulfill({ json: response })
  })
  return { mutations, browserErrors }
}

function formItem(scope: Locator, label: string) {
  return scope.locator('.ant-form-item').filter({ hasText: new RegExp(`^${label}`) }).first()
}

async function selectLine(page: Page, scope: Locator, line: string) {
  await formItem(scope, 'Resolution Line').getByRole('combobox').click()
  await page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: new RegExp(`^${line}$`) }).click()
}

for (const theme of ['light', 'dark'] as const) {
  test(`Tencent DNS create and edit retain selectable resolution lines (${theme})`, async ({ page }) => {
    const { mutations, browserErrors } = await mockDNS(page, theme)
    await gotoRoute(page, '/dns/domains/2017/records')
    await expect(page.locator('body')).toHaveClass(theme)
    await expect(page.getByRole('columnheader', { name: 'Resolution Line' })).toBeVisible()

    await page.getByRole('button', { name: /Add Record$/ }).click()
    const drawer = page.locator('.ant-drawer:visible').last()
    await expect(formItem(drawer, 'Resolution Line')).toContainText('默认')
    await page.screenshot({ path: test.info().outputPath(`tencent-dns-${theme}.png`), animations: 'disabled' })
    await selectLine(page, drawer, '联通')
    await formItem(drawer, 'Value').locator('textarea').fill('192.0.2.2')
    await drawer.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(drawer).not.toHaveClass(/ant-drawer-open/)
    expect(mutations[0]).toMatchObject({ domainId: 2017, method: 'POST', payload: { line: '联通' } })

    await page.locator('.dns-record-table tbody tr.ant-table-row').first().getByRole('button', { name: 'Edit', exact: true }).click()
    await expect(drawer).toBeVisible()
    await expect(formItem(drawer, 'Resolution Line')).toContainText('电信')
    await formItem(drawer, 'Value').locator('textarea').fill('192.0.2.3')
    await drawer.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(drawer).not.toHaveClass(/ant-drawer-open/)
    expect(mutations[1]).toMatchObject({ domainId: 2017, method: 'PUT', payload: { line: '电信' } })

    await page.locator('.dns-record-table tbody tr.ant-table-row').first().getByRole('button', { name: 'Edit', exact: true }).click()
    await selectLine(page, drawer, '办公网')
    await drawer.getByRole('button', { name: 'Save', exact: true }).click()
    await expect(drawer).not.toHaveClass(/ant-drawer-open/)
    expect(mutations[2]).toMatchObject({ domainId: 2017, method: 'PUT', payload: { line: '办公网' } })
    expect(browserErrors).toEqual([])
  })
}

test('mixed DNS domain groups send resolution lines to the matching providers', async ({ page }) => {
  const { mutations, browserErrors } = await mockDNS(page, 'light')
  await gotoRoute(page, '/dns/groups/dns-lines/records')
  await page.getByRole('button', { name: /Add group record$/ }).first().click()
  const drawer = page.locator('.ant-drawer:visible').last()
  const tencentFields = drawer.locator('.provider-domain-fields').filter({ hasText: 'tencent.example.com' })
  await expect(formItem(tencentFields, 'Resolution Line')).toContainText('默认')
  await selectLine(page, tencentFields, '移动')
  await formItem(drawer, 'Name').locator('input').fill('new')
  await formItem(drawer, 'Value').locator('textarea').fill('192.0.2.4')
  await drawer.getByRole('button', { name: 'Preview changes', exact: true }).click()
  const preview = page.locator('.ant-modal:visible').last()
  await expect(preview).toContainText('移动')
  await preview.getByRole('button', { name: 'Apply changes', exact: true }).click()
  await expect.poll(() => mutations.length).toBe(2)
  expect(mutations.find(mutation => mutation.domainId === 2017)?.payload.line).toBe('移动')
  expect(mutations.find(mutation => mutation.domainId === 2018)?.payload.line).toBe('default')
  expect(browserErrors).toEqual([])
})

test('group edits preserve Tencent lines unless explicitly overridden', async ({ page }) => {
  const { mutations, browserErrors } = await mockDNS(page, 'dark')
  await gotoRoute(page, '/dns/groups/dns-lines/records')
  await page.getByRole('button', { name: /Edit and sync$/ }).first().click()
  const drawer = page.locator('.ant-drawer:visible').last()
  await formItem(drawer, 'Value').locator('textarea').fill('192.0.2.5')
  await drawer.getByRole('button', { name: 'Preview changes', exact: true }).click()
  const preview = page.locator('.ant-modal:visible').last()
  await expect(preview).toContainText('电信')
  await preview.getByRole('button', { name: 'Apply changes', exact: true }).click()
  await expect.poll(() => mutations.length).toBe(2)
  expect(mutations.find(mutation => mutation.domainId === 2017)).toMatchObject({ method: 'PUT', payload: { line: '电信' } })
  await page.locator('.ant-modal:visible').last().locator('.ant-modal-footer').getByRole('button', { name: 'Close', exact: true }).click()

  await page.getByRole('button', { name: /Edit and sync$/ }).first().click()
  const tencentFields = drawer.locator('.provider-domain-fields').filter({ hasText: 'tencent.example.com' })
  await selectLine(page, tencentFields, '联通')
  await drawer.getByRole('button', { name: 'Preview changes', exact: true }).click()
  await preview.getByRole('button', { name: 'Apply changes', exact: true }).click()
  await expect.poll(() => mutations.length).toBe(3)
  expect(mutations[2]).toMatchObject({ domainId: 2017, method: 'PUT', payload: { line: '联通' } })
  expect(browserErrors).toEqual([])
})
