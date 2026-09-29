import type { Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

interface BrowserDiagnostics {
  consoleErrors: string[]
  pageErrors: string[]
}

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

test('nginx log list keeps tabs and table renderers populated', async ({ page }) => {
  test.setTimeout(120_000)
  const diagnostics = collectBrowserDiagnostics(page)

  await gotoRoute(page, '/nginx_log/list')

  const listTable = page.locator('.ant-table').first()
  const initialRows = await expectTableRows(page, 1)
  await expect(listTable.locator('thead th').first()).toBeVisible()
  await expect(listTable).toContainText(/\S/)

  // The class check proves the type column uses its custom renderer, instead
  // of falling back to the raw enum value.
  await expect.poll(() => initialRows.locator('.ant-tag').count()).toBeGreaterThan(0)

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

  // Return to the populated access-log table.
  await logTypeTabs.nth(0).locator('[role="tab"]').click()
  await expectTableRows(page, 1)

  await expectNoBrowserDiagnostics(diagnostics)
})

test('an access log detail view opens the raw viewer', async ({ page }) => {
  test.setTimeout(180_000)
  const diagnostics = collectBrowserDiagnostics(page)

  await gotoRoute(page, '/nginx_log/list')
  const accessRows = await expectTableRows(page, 1)
  const accessRow = accessRows.first()
  await expect(accessRow).toBeVisible()

  const actionCell = accessRow.locator('td').last()
  const viewButton = actionCell.getByRole('button').first()
  await expect(viewButton).toBeVisible()
  await viewButton.click()

  const detailCard = page.locator('.ant-card').filter({
    has: page.locator('.font-mono'),
  }).last()
  await expect(detailCard).toBeVisible({ timeout: 90_000 })
  await expect(detailCard.locator('.font-mono').first()).toContainText(/\S/)

  const rawViewer = detailCard.locator('.nginx-log-container')
  await expect(rawViewer).toBeVisible({ timeout: 90_000 })
  await expect(rawViewer.locator('.nginx-log-line').first()).toBeVisible({ timeout: 90_000 })
  await expect(rawViewer).toContainText(/\S/)

  await expectNoBrowserDiagnostics(diagnostics)
})
