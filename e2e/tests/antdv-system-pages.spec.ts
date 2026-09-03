import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

interface BrowserDiagnostics {
  consoleErrors: string[]
  pageErrors: string[]
}

function isExpectedDemoNetworkFailure(message: string): boolean {
  return /Failed to load resource|net::ERR_|NetworkError|WebSocket connection.*failed/i.test(message)
}

function collectBrowserDiagnostics(page: Page): BrowserDiagnostics {
  const diagnostics: BrowserDiagnostics = {
    consoleErrors: [],
    pageErrors: [],
  }

  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedDemoNetworkFailure(message.text()))
      diagnostics.consoleErrors.push(message.text())
  })
  page.on('pageerror', error => diagnostics.pageErrors.push(error.message))

  return diagnostics
}

async function openSystemPage(page: Page, route: string, selector: string): Promise<Locator> {
  await gotoRoute(page, route)
  const pageRoot = page.locator(selector)
  await expect(pageRoot).toBeVisible()
  return pageRoot
}

async function assertEveryVisibleSelectHasOptions(page: Page, rawValues: string[] = []) {
  const selects = page.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)

  const selectCount = await selects.count()
  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const trigger = select.getByRole('combobox')
    const closedLabel = (await select.locator('.ant-select-content').textContent() ?? '').trim()

    await expect(trigger).toBeVisible()
    await trigger.click()

    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)
    await expect(options.first()).not.toBeEmpty()

    const optionLabels = (await options.allTextContents())
      .map(label => label.trim())
      .filter(Boolean)

    expect(closedLabel, `Select ${index} has no visible closed label`).not.toBe('')
    expect(optionLabels, `Select ${index} has no non-empty option labels`).not.toHaveLength(0)
    expect(optionLabels, `Select ${index} does not display its selected label`).toContain(closedLabel)

    for (const rawValue of rawValues) {
      expect(closedLabel, `Select ${index} displays raw value ${rawValue}`).not.toBe(rawValue)
    }

    await page.keyboard.press('Escape')
    await expect(page.locator('.ant-select-dropdown:visible')).toHaveCount(0)
  }
}

const diagnosticsByPage = new WeakMap<Page, BrowserDiagnostics>()

test.beforeEach(({ page }) => {
  diagnosticsByPage.set(page, collectBrowserDiagnostics(page))
})

test.afterEach(({ page }) => {
  const diagnostics = diagnosticsByPage.get(page)
  expect(diagnostics?.consoleErrors ?? [], 'Unexpected browser console errors').toEqual([])
  expect(diagnostics?.pageErrors ?? [], 'Unexpected browser page errors').toEqual([])
})

test('about page renders and reaches the third-party components page', async ({ page }) => {
  const about = await openSystemPage(page, '/system/about', '.router-view > .ant-card')

  await expect(about).not.toBeEmpty()
  await expect(about.locator('img[alt="logo"]')).toBeVisible()
  await expect(about.locator('h2').first()).not.toBeEmpty()
  await assertEveryVisibleSelectHasOptions(page)

  const licensesButton = about.getByRole('button')
  await expect(licensesButton).toHaveCount(1)
  await licensesButton.click()

  await expect(page).toHaveURL(/#\/system\/licenses$/)
  await expect(page.locator('.router-view .ant-tabs')).toBeVisible()
})

test('upgrade page keeps its channel select populated and human-readable', async ({ page }) => {
  const upgrade = await openSystemPage(page, '/system/upgrade', '.router-view .upgrade-container')

  await expect(upgrade).not.toBeEmpty()
  await expect(upgrade.locator('.ant-select')).toHaveCount(1)
  await expect(upgrade.locator('h3').first()).not.toBeEmpty()
  await assertEveryVisibleSelectHasOptions(page, ['stable', 'prerelease', 'dev'])
})

test('license tabs render non-empty tables and preserve custom cell renderers', async ({ page }) => {
  const tabsPanel = await openSystemPage(page, '/system/licenses', '.router-view .ant-tabs')
  const table = tabsPanel.locator('.ant-table')
  const rows = await expectTableRows(table, 1)
  const firstRow = rows.first()

  await expect(firstRow).toBeVisible()
  await expect(firstRow.locator('td')).toHaveCount(4)
  for (let index = 0; index < 4; index++)
    await expect(firstRow.locator('td').nth(index)).not.toBeEmpty()

  await expect(firstRow.locator('.ant-typography')).toHaveCount(1)
  await expect(firstRow.locator('.ant-typography').first()).not.toBeEmpty()
  await expect(firstRow.locator('.ant-tag')).toHaveCount(1)
  await expect(firstRow.locator('.ant-tag').first()).not.toBeEmpty()
  const firstRowLink = firstRow.getByRole('link')
  await expect(firstRowLink).toHaveCount(1)
  await expect(firstRowLink.first()).toHaveAttribute('href', /.+/)
  await expect(firstRowLink.first()).not.toBeEmpty()

  await assertEveryVisibleSelectHasOptions(page)

  const tabs = tabsPanel.getByRole('tab')
  await expect(tabs).toHaveCount(3)
  for (let index = 0; index < 3; index++) {
    const tab = tabs.nth(index)
    await expect(tab).toBeVisible()
    await tab.click()
    await expect(tab).toHaveAttribute('aria-selected', 'true')

    const activePanel = tabsPanel.getByRole('tabpanel')
    await expect(activePanel).toBeVisible()
    await expect.poll(async () => (await activePanel.innerText()).trim().length).toBeGreaterThan(0)
    await expectTableRows(activePanel, 1)
  }
})

test('self-check renders local list items with titles and descriptions', async ({ page }) => {
  const selfCheck = await openSystemPage(page, '/system/self_check', '.router-view .nui-list')
  const items = selfCheck.locator('.nui-list-item')

  await expect(selfCheck.locator('.nui-list-items')).toBeVisible()
  await expect.poll(() => items.count()).toBeGreaterThan(0)

  const itemCount = await items.count()
  for (let index = 0; index < itemCount; index++) {
    const item = items.nth(index)
    await expect(item).toBeVisible()
    await expect(item.locator('.nui-list-item-meta-title')).toBeVisible()
    await expect(item.locator('.nui-list-item-meta-title')).not.toBeEmpty()
    await expect(item.locator('.nui-list-item-meta-description')).toBeVisible()
    await expect(item.locator('.nui-list-item-meta-description')).not.toBeEmpty()
  }

  await assertEveryVisibleSelectHasOptions(page)
})
