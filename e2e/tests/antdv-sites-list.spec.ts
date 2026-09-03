import type { Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

const rawConfigStatusValues = new Set(['enabled', 'disabled', 'maintenance'])

function isExpectedDemoNetworkFailure(message: string) {
  return message.startsWith('Failed to load resource:')
    || message.startsWith('Failed to fetch short token:')
}

function captureBrowserErrors(page: Page) {
  const errors: string[] = []

  page.on('console', message => {
    if (message.type() !== 'error')
      return

    // The demo intentionally emits failed-resource errors for guarded endpoints.
    if (isExpectedDemoNetworkFailure(message.text()))
      return

    errors.push(`console.error: ${message.text()}`)
  })

  page.on('pageerror', error => {
    errors.push(`pageerror: ${error.message}`)
  })

  return errors
}

async function expectReachableSelectsToHaveOptions(page: Page) {
  const selects = page.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)

  const selectCount = await selects.count()
  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const closedLabel = (await select.locator('.ant-select-content').innerText()).trim()

    expect(closedLabel, `Select ${index} has no rendered closed label`).not.toBe('')
    expect(
      rawConfigStatusValues.has(closedLabel),
      `Select ${index} exposes a raw enum value instead of a human label: ${closedLabel}`,
    ).toBe(false)

    const trigger = select.getByRole('combobox')
    await expect(trigger).toBeVisible()
    await trigger.click()

    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)
    await expect.poll(async () => (await options.first().innerText()).trim()).toMatch(/\S/)

    await page.keyboard.press('Escape')
    await expect(dropdown).toBeHidden()
  }
}

test('sites list renders custom cells and non-empty status/select controls', async ({ page }) => {
  const browserErrors = captureBrowserErrors(page)

  await gotoRoute(page, '/sites/list')

  const table = page.locator('.ant-table').first()
  await expect(table).toBeVisible()
  const rows = await expectTableRows(table, 1)
  const rowCount = await rows.count()
  expect(rowCount).toBeGreaterThan(0)

  const namespaceTabs = page.locator('.ant-tabs').first()
  const namespaceTabItems = namespaceTabs.getByRole('tab')
  await expect.poll(() => namespaceTabItems.count()).toBeGreaterThan(0)
  const namespaceTabCount = await namespaceTabItems.count()
  for (let index = 0; index < namespaceTabCount; index++) {
    const tab = namespaceTabItems.nth(index)
    await tab.click()
    await expect(tab).toHaveAttribute('aria-selected', 'true')
    await expectTableRows(table, 1)
  }

  for (let index = 0; index < rowCount; index++) {
    const row = rows.nth(index)
    const cells = row.locator('td.ant-table-cell')
    await expect(cells).toHaveCount(7)

    const nameCell = cells.nth(1)
    await expect(nameCell.locator('div').first()).toBeVisible()
    await expect(nameCell.locator('div').first()).not.toBeEmpty()

    const proxyCell = cells.nth(2)
    const proxyTargets = proxyCell.locator('.proxy-targets')
    if (await proxyTargets.count() > 0) {
      await expect(proxyTargets).toBeVisible()
      await expect.poll(() => proxyTargets.locator('.proxy-target-tag').count()).toBeGreaterThan(0)
    }
    else {
      const emptyProxyValue = proxyCell.locator('span').first()
      await expect(emptyProxyValue).toBeVisible()
      await expect(emptyProxyValue).not.toBeEmpty()
    }

    await expect(row.locator('.site-status-select')).toHaveCount(1)
    await expect(row.locator('.site-status-select .ant-select-content')).not.toBeEmpty()
  }

  await expect.poll(() => rows.locator('.proxy-targets .proxy-target-tag').count()).toBeGreaterThan(0)
  await expectReachableSelectsToHaveOptions(page)

  const actionCell = rows.first().locator('td.ant-table-cell-fix-end')
  const actionButtons = actionCell.getByRole('button')
  await expect.poll(() => actionButtons.count()).toBeGreaterThanOrEqual(2)

  // Duplicate is always immediately before the delete action; clicking it only opens a modal.
  await actionButtons.nth((await actionButtons.count()) - 2).click()

  const modal = page.locator('.ant-modal:visible').last()
  await expect(modal).toBeVisible()
  const modalContainer = modal.locator('.ant-modal-container')
  await expect(modalContainer).toBeVisible()
  await expect.poll(async () => (await modalContainer.boundingBox())?.width ?? 0).toBeGreaterThan(200)
  await expect(modal.locator('.ant-modal-body')).not.toBeEmpty()
  await expect(modal.locator('input')).toHaveCount(1)

  await modal.locator('.ant-modal-close').click()
  await expect(modal).toBeHidden()

  expect(browserErrors).toEqual([])
})

test('TabFilter tabs keep their rendered labels and switch to populated views', async ({ page }) => {
  const browserErrors = captureBrowserErrors(page)

  await gotoRoute(page, '/nginx_log/list')

  const tabFilter = page.locator('.tab-filter').first()
  await expect(tabFilter).toBeVisible()
  const tabs = tabFilter.getByRole('tab')
  await expect.poll(() => tabs.count()).toBeGreaterThan(0)

  const tabCount = await tabs.count()
  for (let index = 0; index < tabCount; index++) {
    const tab = tabs.nth(index)
    await tab.click()
    await expect(tab).toHaveAttribute('aria-selected', 'true')

    const label = tab.locator('.tab-content')
    await expect(label).toBeVisible()
    await expect(label).not.toBeEmpty()
    await expect.poll(async () => {
      const box = await label.boundingBox()
      return (box?.width ?? 0) * (box?.height ?? 0)
    }).toBeGreaterThan(0)

    // TabFilter is a filter-only ATabs instance; its selected tab drives the table below.
    await expect(tabFilter.locator('[role="tabpanel"][aria-hidden="false"]')).toBeAttached()
    await expectTableRows(page, 1)
  }

  await expectReachableSelectsToHaveOptions(page)
  expect(browserErrors).toEqual([])
})
