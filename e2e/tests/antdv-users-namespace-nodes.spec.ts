import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

interface ApiPayload {
  data?: Array<Record<string, unknown>>
  pagination?: {
    total?: number
    per_page?: number
    current_page?: number
    total_pages?: number
    [key: string]: unknown
  }
  [key: string]: unknown
}

const legacyNodeFixture = {
  id: 9104,
  name: 'e2e-legacy-node',
  url: 'https://node.example.test:9000',
  version: 'v2.5.9',
  status: true,
  enabled: true,
  auth_method: 'legacy_secret',
  credential_status: 'active',
  has_credential: true,
  auth_upgrade_status: 'failed',
  auth_upgrade_step: 'verify',
  auth_upgrade_attempt_count: 2,
  auth_upgrade_attempted_at: '2026-08-17T09:15:06Z',
  auth_upgrade_next_retry_at: '2026-08-17T10:15:06Z',
  auth_upgrade_error_code: 'invalid_confirmation',
  auth_upgrade_error: 'The target node returned an invalid upgrade confirmation.',
}

const namespaceFixture = {
  id: 7001,
  name: 'e2e-namespace',
  sync_node_ids: [legacyNodeFixture.id],
  post_sync_action: 'none',
  upstream_test_type: 'local',
  deploy_mode: 'local',
  sync_strategy: 'manual',
  sync_interval_minutes: 30,
  created_at: '2026-08-17T08:00:00Z',
  updated_at: '2026-08-17T09:00:00Z',
}

const passkeyFixture = {
  id: 8001,
  name: 'e2e-passkey',
  user_id: '1',
  raw_id: 'e2e-passkey-raw-id',
  created_at: '2026-08-17T08:00:00Z',
  updated_at: '2026-08-17T09:00:00Z',
  last_used_at: 1_755_423_600,
}

function collectDiagnostics(page: Page) {
  const browserErrors: string[] = []
  const mutatingRequests: string[] = []

  page.on('console', message => {
    if (message.type() !== 'error')
      return

    const text = message.text()
    if (/failed to load resource|net::err_|network(?:error|failure)|websocket.*(?:failed|closed)/i.test(text))
      return

    browserErrors.push(`console.error: ${text}`)
  })
  page.on('pageerror', error => {
    browserErrors.push(`pageerror: ${error.message}`)
  })
  page.on('request', request => {
    const url = new URL(request.url())
    if (url.pathname.startsWith('/api/')
      && url.pathname !== '/api/token/short'
      && !['GET', 'HEAD', 'OPTIONS'].includes(request.method()))
      mutatingRequests.push(`${request.method()} ${url.pathname}`)
  })

  return { browserErrors, mutatingRequests }
}

async function installNodeFixture(page: Page) {
  await page.route('**/api/nodes?**', async route => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }

    const response = await route.fetch()
    const payload = await response.json() as ApiPayload
    if (Array.isArray(payload.data) && !payload.data.some(node => node.id === legacyNodeFixture.id)) {
      payload.data = [legacyNodeFixture, ...payload.data]
      if (payload.pagination && typeof payload.pagination.total === 'number')
        payload.pagination.total += 1
    }

    await route.fulfill({ response, json: payload })
  })
}

async function installNamespaceFixture(page: Page) {
  await page.route(`**/api/namespaces/${namespaceFixture.id}`, async route => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }

    await route.fulfill({ status: 200, contentType: 'application/json', json: namespaceFixture })
  })

  await page.route('**/api/namespaces?**', async route => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }

    const response = await route.fetch()
    const payload = await response.json() as ApiPayload
    if (Array.isArray(payload.data) && !payload.data.some(namespace => namespace.id === namespaceFixture.id)) {
      payload.data = [namespaceFixture, ...payload.data]
      if (payload.pagination && typeof payload.pagination.total === 'number')
        payload.pagination.total += 1
    }

    await route.fulfill({ response, json: payload })
  })
}

async function installPasskeyFixture(page: Page) {
  await page.route('**/api/passkeys**', async route => {
    const url = new URL(route.request().url())
    if (url.pathname !== '/api/passkeys' || route.request().method() !== 'GET') {
      await route.continue()
      return
    }

    await route.fulfill({ status: 200, contentType: 'application/json', json: [passkeyFixture] })
  })
}

async function expectSelectHasOptions(page: Page, select: Locator) {
  await expect(select).toBeVisible()

  const trigger = select.getByRole('combobox')
  const closedLabel = (await select.locator('.ant-select-content').innerText()).trim()
  expect(closedLabel, 'The closed Select should show its rendered label').not.toBe('')

  await trigger.click()
  const dropdown = page.locator('.ant-select-dropdown:visible').last()
  await expect(dropdown).toBeVisible()
  await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
  await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)

  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count()).toBeGreaterThan(0)
  const optionLabels = (await options.allTextContents()).map(label => label.trim()).filter(Boolean)
  expect(optionLabels.length, 'The Select options should contain rendered labels').toBeGreaterThan(0)
  expect(optionLabels).toContain(closedLabel)

  await page.keyboard.press('Escape')
  await expect(dropdown).toBeHidden()
}

async function expectVisibleSelectsHaveOptions(page: Page) {
  const selects = page.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)

  const count = await selects.count()
  for (let index = 0; index < count; index++)
    await expectSelectHasOptions(page, selects.nth(index))
}

async function expectSensibleWidth(locator: Locator, minimumWidth = 200) {
  await expect.poll(async () => (await locator.boundingBox())?.width ?? 0).toBeGreaterThan(minimumWidth)
}

async function closeOverlay(overlay: Locator) {
  await overlay.getByRole('button').first().click()
  await expect(overlay).toBeHidden()
}

test('users and namespaces keep table renderers, overlays, and form controls populated', async ({ page }) => {
  const diagnostics = collectDiagnostics(page)
  await installNodeFixture(page)
  await installNamespaceFixture(page)
  await installPasskeyFixture(page)

  await gotoRoute(page, '/users')
  const usersTable = page.locator('.ant-table').last()
  const userRows = await expectTableRows(usersTable, 1)
  const userRow = userRows.first()
  await expect(userRow.locator('.ant-tag')).toHaveCount(1)
  await expect(userRow.locator('.ant-tag')).not.toHaveText(/^\s*$/)
  expect(await userRow.innerText()).not.toMatch(/\b(?:true|false)\b/)
  await expectVisibleSelectsHaveOptions(page)

  const userActions = userRow.getByRole('button')
  await expect.poll(() => userActions.count()).toBeGreaterThanOrEqual(2)
  await userActions.first().click()
  const userModal = page.locator('.ant-modal:visible').last()
  await expect(userModal).toBeVisible()
  await expect(userModal.locator('.ant-modal-container')).toBeVisible()
  await expect(userModal.locator('.ant-modal-body')).not.toHaveText(/^\s*$/)
  await expectSensibleWidth(userModal.locator('.ant-modal-container'))
  await closeOverlay(userModal)

  await gotoRoute(page, '/namespaces')
  const namespaceTable = page.locator('.ant-table').last()
  const namespaceRows = await expectTableRows(namespaceTable, 1)
  const namespaceRow = namespaceRows.first()
  const namespaceCells = namespaceRow.locator('td')
  await expect.poll(() => namespaceCells.count()).toBeGreaterThan(8)
  const syncNodeCellText = (await namespaceCells.nth(2).innerText()).trim()
  expect(syncNodeCellText, 'Sync-node cells should render a node label').not.toBe('')
  expect(syncNodeCellText, 'Sync-node cells should not expose the numeric node id').not.toBe(String(legacyNodeFixture.id))
  for (const index of [3, 4, 5, 6]) {
    const cellText = (await namespaceCells.nth(index).innerText()).trim()
    expect(cellText, `Namespace column ${index} should render content`).not.toBe('')
    expect(cellText, `Namespace column ${index} should render a label instead of an enum`).not.toMatch(/^(?:none|reload_nginx|local|remote|mirror|manual|auto)$/)
  }

  const namespaceActions = namespaceRow.getByRole('button')
  await expect.poll(() => namespaceActions.count()).toBeGreaterThanOrEqual(4)
  await namespaceActions.nth(2).click()
  const namespaceModal = page.locator('.ant-modal:visible').last()
  await expect(namespaceModal).toBeVisible()
  await expect(namespaceModal.locator('.ant-modal-container')).toBeVisible()
  await expect(namespaceModal.locator('.ant-modal-body')).not.toHaveText(/^\s*$/)
  await expectSensibleWidth(namespaceModal.locator('.ant-modal-container'))
  const namespaceSelects = namespaceModal.locator('.ant-select:visible')
  await expect.poll(() => namespaceSelects.count()).toBeGreaterThan(0)
  for (let index = 0; index < await namespaceSelects.count(); index++)
    await expectSelectHasOptions(page, namespaceSelects.nth(index))
  await expect.poll(() => namespaceModal.getByRole('checkbox').count()).toBeGreaterThan(0)
  await closeOverlay(namespaceModal)

  await gotoRoute(page, '/profile')
  await expect.poll(() => page.locator('main h2').count()).toBeGreaterThan(0)
  expect((await page.locator('main').innerText()).trim().length).toBeGreaterThan(0)
  await expectSelectHasOptions(page, page.locator('.ant-select:visible').first())
  const passkeyList = page.locator('.nui-list').first()
  await expect(passkeyList).toBeVisible()
  const passkeyItems = passkeyList.locator('.nui-list-item')
  await expect.poll(() => passkeyItems.count()).toBeGreaterThan(0)
  await expect(passkeyItems.first().locator('.nui-list-item-meta-title')).toContainText(passkeyFixture.name)
  await expect(passkeyItems.first().locator('.nui-list-item-meta-description')).not.toHaveText(/^\s*$/)

  expect(diagnostics.mutatingRequests, 'The users and namespaces flow must stay read-only').toEqual([])
  expect(diagnostics.browserErrors, 'Unexpected browser errors were reported').toEqual([])
})

test('nodes keep auth Steps, Select options, and batch-upgrade overlays populated', async ({ page }) => {
  const diagnostics = collectDiagnostics(page)
  await installNodeFixture(page)

  await gotoRoute(page, '/nodes')
  const nodesTable = page.locator('.ant-table').last()
  const nodeRows = await expectTableRows(nodesTable, 1)
  const authRow = nodeRows.filter({ has: page.locator('button[aria-label]') }).first()
  await expect(authRow).toBeVisible()
  await expect(authRow).toContainText(legacyNodeFixture.name)
  await expect(authRow.locator('.ant-tag')).toHaveCount(2)
  await expect(authRow.locator('.ant-badge')).toHaveCount(1)
  expect(await authRow.innerText()).not.toContain('legacy_secret')

  const authTrigger = authRow.locator('button[aria-label]').first()
  await authTrigger.click()
  const authPopover = page.locator('.ant-popover:visible').last()
  await expect(authPopover).toBeVisible()
  await expect(authPopover.locator('.ant-alert')).toBeVisible()
  await expect(authPopover.locator('.ant-steps')).toBeVisible()
  const stepTitles = authPopover.locator('.ant-steps-item-title')
  await expect(stepTitles).toHaveCount(4)
  for (let index = 0; index < await stepTitles.count(); index++)
    await expect(stepTitles.nth(index)).not.toHaveText(/^\s*$/)
  await expectSensibleWidth(authPopover, 200)
  await authTrigger.click()
  await expect(authPopover).toBeHidden()

  const autoRefreshSelect = page.locator('.ant-select.w-16')
  await expect(autoRefreshSelect).toHaveClass(/ant-select-disabled/)
  await page.locator('.ant-switch').click()
  await expect(autoRefreshSelect).not.toHaveClass(/ant-select-disabled/)
  await expectVisibleSelectsHaveOptions(page)

  await authRow.getByRole('checkbox').check()
  const footerButtons = page.locator('.ant-pro-footer-toolbar').getByRole('button')
  await expect.poll(() => footerButtons.count()).toBeGreaterThanOrEqual(4)
  await footerButtons.first().click()

  const batchModal = page.locator('.ant-modal:visible').last()
  await expect(batchModal).toBeVisible()
  await expect(batchModal.locator('.ant-modal-container')).toBeVisible()
  await expect(batchModal.locator('.ant-modal-body')).not.toHaveText(/^\s*$/)
  await expectSensibleWidth(batchModal.locator('.ant-modal-container'), 500)
  await expectSelectHasOptions(page, batchModal.locator('.ant-select').first())
  await closeOverlay(batchModal)

  await footerButtons.nth(1).click()
  const syncModal = page.locator('.ant-modal:visible').last()
  await expect(syncModal).toBeVisible()
  await expect(syncModal.locator('.ant-modal-container')).toBeVisible()
  await expect(syncModal.locator('.ant-alert')).toBeVisible()
  await expect(syncModal.locator('.ant-modal-body')).not.toHaveText(/^\s*$/)
  await expect.poll(() => syncModal.getByRole('checkbox').count()).toBeGreaterThan(0)
  await expectSensibleWidth(syncModal.locator('.ant-modal-container'))
  await closeOverlay(syncModal)

  expect(diagnostics.mutatingRequests, 'The node flow must stay read-only').toEqual([])
  expect(diagnostics.browserErrors, 'Unexpected browser errors were reported').toEqual([])
})
