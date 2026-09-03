import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

interface BrowserDiagnostics {
  consoleErrors: string[]
  pageErrors: string[]
}

type LocatorRoot = Locator | Page

function collectBrowserDiagnostics(page: Page): BrowserDiagnostics {
  const diagnostics: BrowserDiagnostics = {
    consoleErrors: [],
    pageErrors: [],
  }

  page.on('console', message => {
    if (message.type() !== 'error')
      return

    // The demo can report failed network resources while its intentionally
    // unavailable auxiliary services are probed. Keep application errors strict.
    if (!/(?:Failed to load resource|net::ERR_|WebSocket connection.*failed)/i.test(message.text()))
      diagnostics.consoleErrors.push(message.text())
  })
  page.on('pageerror', error => {
    diagnostics.pageErrors.push(error.message)
  })

  return diagnostics
}

async function expectNoBrowserDiagnostics(diagnostics: BrowserDiagnostics) {
  expect(diagnostics.consoleErrors, 'Unexpected browser console errors').toEqual([])
  expect(diagnostics.pageErrors, 'Unexpected uncaught page errors').toEqual([])
}

function selectPopup(page: Page, controlsId: string | null): Locator {
  if (!controlsId)
    return page.locator('.ant-select-dropdown:visible').last()

  return page.locator('.ant-select-dropdown').filter({
    has: page.locator(`[id="${controlsId}"]`),
  }).last()
}

async function expectVisibleSelectsHaveOptions(page: Page, scope: LocatorRoot = page) {
  const selects = scope.locator('.ant-select:visible')
  const selectCount = await selects.count()
  expect(selectCount, 'Expected at least one reachable Select').toBeGreaterThan(0)

  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const trigger = select.getByRole('combobox')
    await expect(trigger, `Select ${index} has no accessible trigger`).toBeVisible()

    const closedText = (await select.locator('.ant-select-content').innerText()).trim()
    expect(closedText, `Select ${index} has no human-readable closed label`).toMatch(/\S/)
    expect(closedText, `Select ${index} exposes a function instead of a label`).not.toMatch(/=>|function\s*\(/)
    expect(closedText, `Select ${index} exposes a raw enum instead of a label`).not.toMatch(/^(?:access|error|not_indexed|queued|indexing|indexed)$/i)

    const controlsId = await trigger.getAttribute('aria-controls')
    await trigger.click()

    const popup = selectPopup(page, controlsId)
    await expect(popup, `Select ${index} popup did not open`).toBeVisible()
    await expect(popup, `Select ${index} popup rendered an empty state`).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect.poll(
      () => popup.locator('.ant-select-item-option').count(),
      { message: `Select ${index} rendered no options` },
    ).toBeGreaterThan(0)
    await expect(popup.locator('.ant-select-item-option').first()).toContainText(/\S/)

    await page.keyboard.press('Escape')
    await expect(popup).toBeHidden()
  }
}

async function expectDropdownHasContent(page: Page, trigger: Locator) {
  await expect(trigger).toBeVisible()
  await trigger.click()

  const dropdown = page.locator('.ant-dropdown:visible').last()
  await expect(dropdown).toBeVisible()
  await expect.poll(() => dropdown.locator('.ant-dropdown-menu-item').count()).toBeGreaterThan(0)
  await expect(dropdown).toContainText(/\S/)

  const box = await dropdown.boundingBox()
  expect(box?.width ?? 0, 'Dropdown has no measurable width').toBeGreaterThan(0)
  expect(box?.height ?? 0, 'Dropdown has no measurable height').toBeGreaterThan(0)

  await page.keyboard.press('Escape')
  await expect(dropdown).toBeHidden()
}

test('nginx log list keeps tabs, table renderers, and Select popups populated', async ({ page }) => {
  test.setTimeout(120_000)
  const diagnostics = collectBrowserDiagnostics(page)

  await gotoRoute(page, '/nginx_log/list')

  const listTable = page.locator('.ant-table').first()
  const initialRows = await expectTableRows(page, 1)
  await expect(listTable.locator('thead th').first()).toBeVisible()
  await expect(listTable).toContainText(/\S/)

  // These class checks prove the columns are using their custom renderers,
  // instead of falling back to raw enum/data values.
  await expect.poll(() => initialRows.locator('.ant-tag').count()).toBeGreaterThan(0)
  await expect.poll(() => initialRows.locator('.ant-badge-status').count()).toBeGreaterThan(0)
  await expect.poll(() => initialRows.locator('.anticon-check-circle').count()).toBeGreaterThan(0)

  const logTypeTabs = page.locator('.tab-filter .ant-tabs-tab')
  await expect(logTypeTabs).toHaveCount(2)

  for (let index = 0; index < await logTypeTabs.count(); index++) {
    const tab = logTypeTabs.nth(index)
    const tabButton = tab.locator('[role="tab"]')
    await expect(tabButton).toBeVisible()
    await expect(tabButton).toContainText(/\S/)
    await tabButton.click()
    await expect(tab).toHaveClass(/ant-tabs-tab-active/)
    await expect(listTable).toBeVisible()
    await expect(listTable.locator('thead th').first()).toBeVisible()
    await expect(listTable.locator('.ant-table-tbody')).toContainText(/\S/)
  }

  // Return to the populated access-log table before checking its access-only
  // filter and before opening a row in the detail view.
  await logTypeTabs.nth(0).locator('[role="tab"]').click()
  const accessRows = await expectTableRows(page, 1)
  await expect.poll(() => accessRows.locator('.ant-badge-status').count()).toBeGreaterThan(0)

  await expectVisibleSelectsHaveOptions(page)
  await expectNoBrowserDiagnostics(diagnostics)
})

test('an access log detail view keeps every mode, dropdown, filter Select, and rendered row alive', async ({ page }) => {
  test.setTimeout(180_000)
  const diagnostics = collectBrowserDiagnostics(page)

  await gotoRoute(page, '/nginx_log/list')
  const accessRows = await expectTableRows(page, 1)
  const accessRow = accessRows.filter({ has: page.locator('.ant-badge-status') }).first()
  await expect(accessRow).toBeVisible()

  const actionCell = accessRow.locator('td').last()
  const viewButton = actionCell.getByRole('button').first()
  await expect(viewButton).toBeVisible()
  await viewButton.click()

  const detailCard = page.locator('.ant-card').filter({
    has: page.locator('.ant-segmented'),
  }).last()
  await expect(detailCard).toBeVisible({ timeout: 90_000 })
  await expect(detailCard.locator('.font-mono').first()).toContainText(/\S/)

  const viewModes = detailCard.locator('.ant-segmented-item')
  await expect(viewModes).toHaveCount(3)

  // Structured mode is the highest-value path: it contains both the custom
  // table cells and all of the advanced-search Select controls.
  await expect(detailCard.locator('.log-table-container')).toBeVisible({ timeout: 90_000 })
  const structuredRows = await expectTableRows(detailCard.locator('.log-table-container'), 1)
  await expect(structuredRows.first().locator('td').first()).toContainText(/\d{4}/)
  await expect.poll(() => structuredRows.locator('.ant-tag').count()).toBeGreaterThan(0)

  const searchFilters = detailCard.locator('.bg-gray-50.border-gray-200').first()
  await expect(searchFilters).toBeVisible()
  await searchFilters.locator('.cursor-pointer').first().click()
  await expect(searchFilters.locator('input').first()).toBeVisible()
  await expectVisibleSelectsHaveOptions(page, detailCard)

  await expectDropdownHasContent(page, detailCard.locator('.ant-dropdown-trigger').first())

  // Exercise each segmented panel and require meaningful rendered content in
  // the panel selected by the user.
  await viewModes.nth(0).click()
  await expect(viewModes.nth(0)).toHaveClass(/ant-segmented-item-selected/)
  await expectTableRows(detailCard.locator('.log-table-container'), 1)

  await viewModes.nth(1).click()
  await expect(viewModes.nth(1)).toHaveClass(/ant-segmented-item-selected/)
  const dashboard = detailCard.locator('.dashboard-viewer')
  await expect(dashboard).toBeVisible({ timeout: 90_000 })
  await expect(dashboard.locator('.ant-statistic').first()).toBeVisible({ timeout: 90_000 })
  await expect(dashboard).toContainText(/\S/)
  await expectDropdownHasContent(page, dashboard.locator('.ant-dropdown-trigger').first())

  await viewModes.nth(2).click()
  await expect(viewModes.nth(2)).toHaveClass(/ant-segmented-item-selected/)
  const rawViewer = detailCard.locator('.nginx-log-container')
  await expect(rawViewer).toBeVisible({ timeout: 90_000 })
  await expect(rawViewer.locator('.nginx-log-line').first()).toBeVisible({ timeout: 90_000 })
  await expect(rawViewer).toContainText(/\S/)

  await expectNoBrowserDiagnostics(diagnostics)
})
