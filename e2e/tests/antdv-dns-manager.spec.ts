import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import {
  expectTableRows,
  gotoRoute,
  tableRows,
  waitForApiResponse,
} from './helpers'

function isExpectedNetworkConsoleError(text: string) {
  return /Failed to load resource|Failed to fetch|net::ERR_/i.test(text)
}

function captureBrowserErrors(page: Page) {
  const errors: string[] = []

  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedNetworkConsoleError(message.text()))
      errors.push(`console.error: ${message.text()}`)
  })
  page.on('pageerror', error => {
    errors.push(`pageerror: ${error.message}`)
  })

  return errors
}

async function expectVisibleSelectsToRenderOptions(page: Page, scope: Locator | Page = page) {
  const selects = scope.locator('.ant-select:not(.ant-select-auto-complete):visible')
  const selectCount = await selects.count()
  expect(selectCount, 'Expected at least one reachable select in this scope').toBeGreaterThan(0)

  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const trigger = select.locator('.ant-select-content')
    await expect(trigger, `Select ${index} has no human-readable closed label`).toHaveText(/\S/)

    const combobox = select.getByRole('combobox').first()
    const isDisabled = await combobox.isDisabled() || await combobox.getAttribute('aria-disabled') === 'true'
    if (isDisabled)
      continue

    await combobox.click()

    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown, `Select ${index} did not open a dropdown`).toBeVisible()
    await expect(dropdown, `Select ${index} rendered an empty dropdown`).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect.poll(
      () => dropdown.locator('.ant-select-item-option').count(),
      { message: `Select ${index} rendered no options` },
    ).toBeGreaterThan(0)
    await expect.poll(async () => {
      const labels = await dropdown.locator('.ant-select-item-option').allTextContents()
      return labels.filter(label => label.trim()).length
    }, { message: `Select ${index} rendered only blank options` }).toBeGreaterThan(0)

    await page.keyboard.press('Escape')
    await expect(combobox).toHaveAttribute('aria-expanded', 'false')
    await expect(trigger, `Select ${index} lost its closed label`).toHaveText(/\S/)
  }
}

async function expectRenderedCells(row: Locator, indices: number[], description: string) {
  const cells = row.locator('td')
  await expect.poll(() => cells.count()).toBeGreaterThan(Math.max(...indices))

  for (const index of indices)
    await expect(cells.nth(index), `${description} cell ${index} is empty`).toHaveText(/\S/)
}

async function expectDrawer(page: Page, minimumWidth: number) {
  const drawer = page.locator('.ant-drawer:visible').last()
  await expect(drawer).toBeVisible()

  const wrapper = drawer.locator('.ant-drawer-content-wrapper')
  await expect(wrapper).toBeVisible()
  await expect(drawer).toHaveText(/\S/)

  const width = await wrapper.evaluate(element => element.getBoundingClientRect().width)
  expect(width, 'Drawer content wrapper has zero width').toBeGreaterThan(0)
  expect(width, 'Drawer size prop was not reflected in its content wrapper').toBeGreaterThanOrEqual(minimumWidth)

  return drawer
}

async function expectNoBrowserErrors(errors: string[]) {
  expect(errors, `Unexpected browser errors:\n${errors.join('\n')}`).toEqual([])
}

async function visitCredentials(page: Page) {
  const responsePromise = waitForApiResponse(page, '/api/dns_credentials', 'GET')
  await gotoRoute(page, '/dns/credentials')
  const response = await responsePromise
  expect(response.ok()).toBe(true)
  await expectTableRows(page.locator('.ant-table').last(), 1)
}

test('DNS credentials keep provider filters and custom table cells rendered', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors = captureBrowserErrors(page)

  await visitCredentials(page)

  const table = page.locator('.ant-table').last()
  const row = (await expectTableRows(table, 1)).first()
  await expectRenderedCells(row, [0, 1, 2, 3], 'DNS credential')
  await expectVisibleSelectsToRenderOptions(page)
  await expectNoBrowserErrors(browserErrors)
})

test('DNS domains and record manager keep rows, custom cells, selects, and drawer content rendered', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors = captureBrowserErrors(page)

  await visitCredentials(page)

  const domainResponsePromise = waitForApiResponse(page, '/api/dns/domains', 'GET')
  await gotoRoute(page, '/dns/domains')
  const domainResponse = await domainResponsePromise
  expect(domainResponse.ok()).toBe(true)

  const domainCard = page.locator('.ant-card:visible').last()
  await expect(domainCard).toBeVisible()
  const domainTable = domainCard.locator('.ant-table')
  const domainRow = (await expectTableRows(domainTable, 1)).first()
  await expectRenderedCells(domainRow, [0, 1, 2], 'DNS domain')
  await expectVisibleSelectsToRenderOptions(page, domainCard)

  const domainActionButtons = domainRow.getByRole('button')
  await expect(domainActionButtons).toHaveCount(3)
  await domainActionButtons.nth(1).click()

  const modal = page.locator('.ant-modal:visible').last()
  await expect(modal).toBeVisible()
  const modalContainer = modal.locator('.ant-modal-container')
  await expect(modalContainer).toBeVisible()
  await expect(modalContainer).toHaveText(/\S/)
  const modalBox = await modalContainer.boundingBox()
  expect(modalBox?.width ?? 0, 'Modal container has zero width').toBeGreaterThan(0)
  await expectVisibleSelectsToRenderOptions(page, modal)
  await modal.locator('.ant-modal-footer .ant-btn').first().click()
  await expect(modal).toBeHidden()

  const recordsResponsePromise = page.waitForResponse(response => {
    const url = new URL(response.url())
    return /^\/api\/dns\/domains\/\d+\/records$/.test(url.pathname)
      && response.request().method() === 'GET'
  })
  await domainRow.getByRole('button').first().click()
  const recordsResponse = await recordsResponsePromise
  expect(recordsResponse.ok()).toBe(true)
  await expect(page).toHaveURL(/#\/dns\/domains\/\d+\/records$/)

  const recordManager = page.locator('.record-manager')
  const recordTable = recordManager.locator('.dns-record-table')
  const recordRow = (await expectTableRows(recordTable, 1)).first()
  await expect(recordRow).toHaveText(/\S/)
  await expect(recordRow.locator('.record-value').first()).toHaveText(/\S/)
  await expect.poll(() => recordRow.locator('.ant-tag').count()).toBeGreaterThan(0)
  await expectVisibleSelectsToRenderOptions(page, recordManager)

  const recordButtons = recordRow.getByRole('button')
  const recordButtonCount = await recordButtons.count()
  expect(recordButtonCount, 'Record row has no action control').toBeGreaterThan(1)

  if (recordButtonCount >= 3) {
    await recordButtons.nth(recordButtonCount - 2).click()
  }
  else {
    await recordButtons.last().click()
    const members = recordTable.locator('.record-group-members:visible').last()
    await expect(members).toBeVisible()
    await members.locator('tbody tr').first().getByRole('button').first().click()
  }

  const drawer = await expectDrawer(page, 400)
  await expectVisibleSelectsToRenderOptions(page, drawer)
  await expect(drawer.locator('.ant-form-item').first()).toBeVisible()
  await expectNoBrowserErrors(browserErrors)
})

test('DNS groups expose populated multi-select options in the read-only creation drawer', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors = captureBrowserErrors(page)

  const domainsResponsePromise = waitForApiResponse(page, '/api/dns/domains', 'GET')
  await gotoRoute(page, '/dns/groups')
  const domainsResponse = await domainsResponsePromise
  expect(domainsResponse.ok()).toBe(true)

  const groupPage = page.locator('.dns-groups-page')
  await expect(groupPage).toBeVisible()
  const groupTable = groupPage.locator('.ant-table')
  await expect(groupTable.locator('.ant-table-placeholder')).toHaveText(/\S/)

  const createButton = groupPage.locator('.ant-btn-primary').first()
  await expect(createButton).toBeVisible()
  await createButton.click()

  const drawer = await expectDrawer(page, 500)
  await expectVisibleSelectsToRenderOptions(page, drawer)
  await expect(drawer.locator('.ant-form-item').first()).toBeVisible()
  await expectNoBrowserErrors(browserErrors)
})

test('DDNS keeps custom cells and editable drawer selects populated without submitting changes', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors = captureBrowserErrors(page)

  const ddnsResponsePromise = waitForApiResponse(page, '/api/dns/ddns', 'GET')
  await gotoRoute(page, '/dns/ddns')
  const ddnsResponse = await ddnsResponsePromise
  expect(ddnsResponse.ok()).toBe(true)

  const ddnsPage = page.locator('.ddns-page')
  const table = ddnsPage.locator('.ant-table')
  const rows = await expectTableRows(table, 1)
  const row = rows.first()
  await expectRenderedCells(row, [0, 1, 2, 3, 4, 5, 6], 'DDNS')
  await expect(row.locator('.ant-tag').first()).toHaveText(/\S/)
  await expectVisibleSelectsToRenderOptions(page)

  let enabledDeleteButton: Locator | undefined
  const rowCount = await rows.count()
  for (let index = 0; index < rowCount; index++) {
    const buttons = rows.nth(index).getByRole('button')
    const candidate = buttons.last()
    if (await candidate.isEnabled()) {
      enabledDeleteButton = candidate
      break
    }
  }
  expect(enabledDeleteButton, 'Demo did not expose a reachable DDNS confirmation popover').toBeDefined()
  if (!enabledDeleteButton)
    throw new Error('Demo did not expose a reachable DDNS confirmation popover')

  await enabledDeleteButton.click()
  const popover = page.locator('.ant-popover:visible').last()
  await expect(popover).toBeVisible()
  await expect(popover).toHaveText(/\S/)
  const popoverBox = await popover.boundingBox()
  expect(popoverBox?.width ?? 0, 'Popover has zero width').toBeGreaterThan(0)
  await page.keyboard.press('Escape')
  await expect(popover).toBeHidden()

  await row.getByRole('button').first().click()
  const drawer = await expectDrawer(page, 500)
  await expect(drawer.locator('.ant-skeleton')).toBeHidden()
  await expect(drawer.locator('.ant-form-item').first()).toBeVisible()
  await expectVisibleSelectsToRenderOptions(page, drawer)
  await expect(drawer.locator('.ant-select').first().locator('.ant-select-content'))
    .not.toHaveText(/^(?:ipv4|ipv6|ipv4_ipv6|ipv6_ipv4)$/i)
  await expectNoBrowserErrors(browserErrors)
})
