import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute, tableRows, waitForApiResponse } from './helpers'

interface BrowserErrors {
  consoleErrors: string[]
  pageErrors: string[]
}

function collectBrowserErrors(page: Page): BrowserErrors {
  const errors: BrowserErrors = {
    consoleErrors: [],
    pageErrors: [],
  }

  page.on('console', message => {
    if (message.type() === 'error') {
      errors.consoleErrors.push(message.text())
    }
  })
  page.on('pageerror', error => {
    errors.pageErrors.push(error.message)
  })

  return errors
}

function isDemoNetworkFailure(message: string): boolean {
  return /Failed to load resource|net::ERR_|NetworkError|Load failed|WebSocket connection .* failed/i.test(message)
}

function expectNoUnexpectedBrowserErrors(errors: BrowserErrors) {
  const unexpectedConsoleErrors = errors.consoleErrors.filter(error => !isDemoNetworkFailure(error))
  const unexpectedErrors = [
    ...unexpectedConsoleErrors.map(error => `console: ${error}`),
    ...errors.pageErrors.map(error => `pageerror: ${error}`),
  ]

  expect(unexpectedErrors, 'Unexpected browser errors').toEqual([])
}

async function expectVisibleSelectsToHaveOptions(
  page: Page,
  root: Locator,
  description: string,
  rawEnumPattern?: RegExp,
) {
  const selects = root.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)

  const selectCount = await selects.count()
  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const content = select.locator('.ant-select-content')
    await expect(content, `${description} ${index + 1} has no visible closed label`).toBeVisible()

    const closedLabel = (await content.innerText()).trim()
    expect(closedLabel, `${description} ${index + 1} has an empty closed label`).not.toBe('')
    if (rawEnumPattern) {
      expect(closedLabel, `${description} ${index + 1} shows a raw enum value`).not.toMatch(rawEnumPattern)
    }

    await select.getByRole('combobox').click()

    // antdv-next options are classed elements, not role=option elements.
    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown, `${description} ${index + 1} did not open a dropdown`).toBeVisible()
    await expect(dropdown, `${description} ${index + 1} rendered an empty dropdown`).not.toHaveClass(/ant-select-dropdown-empty/)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)
    await expect(options.first()).toContainText(/\S/)

    const selectedOption = dropdown.locator('.ant-select-item-option-selected').first()
    await expect(selectedOption, `${description} ${index + 1} has no rendered selected option`).toBeVisible()
    const selectedLabel = (await selectedOption.locator('.ant-select-item-option-content').innerText()).trim()
    expect(closedLabel, `${description} ${index + 1} does not show the selected option label`).toBe(selectedLabel)

    await page.keyboard.press('Escape')
    await expect(dropdown).toBeHidden()
  }
}

async function expectTabPanelsToRender(tabs: Locator, requireTableRows: boolean) {
  const tabRoots = tabs.locator('.ant-tabs-tab')
  await expect.poll(() => tabRoots.count()).toBeGreaterThan(0)

  const tabCount = await tabRoots.count()
  for (let index = 0; index < tabCount; index++) {
    const tab = tabRoots.nth(index)
    const tabButton = tab.getByRole('tab')
    await tabButton.click()
    await expect(tabButton).toHaveAttribute('aria-selected', 'true')

    const panel = tabs.locator('[role="tabpanel"][aria-hidden="false"]').first()
    await expect(panel).toBeVisible()
    await expect(panel).toContainText(/\S/)

    if (requireTableRows) {
      await expect.poll(() => tableRows(panel).count()).toBeGreaterThan(0)
    }
  }
}

async function expectCardBodiesToRender(cards: Locator) {
  await expect.poll(() => cards.count()).toBeGreaterThan(0)

  const cardCount = await cards.count()
  for (let index = 0; index < cardCount; index++) {
    const card = cards.nth(index)
    await expect(card).toBeVisible()
    const body = card.locator('.ant-card-body').first()
    await expect(body).toBeVisible()
    await expect(body).toContainText(/\S/)
  }
}

test('server dashboard renders analytics cards and node list content', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors = collectBrowserErrors(page)

  const initResponsePromise = waitForApiResponse(page, '/api/analytic/init', 'GET')
  await gotoRoute(page, '/dashboard/server')
  const initResponse = await initResponsePromise
  expect(initResponse.ok()).toBe(true)

  await expectCardBodiesToRender(page.locator('.first-row .ant-card, .row-two .ant-card'))

  const statisticValues = page.locator(
    '.first-row .ant-statistic-content-value, .row-two .ant-statistic-content-value',
  )
  await expect.poll(() => statisticValues.count()).toBeGreaterThan(0)
  const statisticCount = await statisticValues.count()
  for (let index = 0; index < statisticCount; index++) {
    await expect(statisticValues.nth(index)).toHaveText(/\S/)
  }

  const nodeCard = page.locator('.env-list-card')
  await expect(nodeCard).toBeVisible()
  const nodeItems = nodeCard.locator('.env-list-item')
  await expect.poll(() => nodeItems.count()).toBeGreaterThan(0)
  const nodeItemCount = await nodeItems.count()
  for (let index = 0; index < nodeItemCount; index++) {
    const item = nodeItems.nth(index)
    await expect(item.locator('.nui-list-item-meta-title')).toContainText(/\S/)
    await expect(item.locator('.nui-list-item-meta-description')).toContainText(/\S/)
  }

  await expectVisibleSelectsToHaveOptions(
    page,
    page,
    'Dashboard language Select',
    /^(?:en|zh_CN|zh_TW)$/,
  )

  expectNoUnexpectedBrowserErrors(browserErrors)
})

test('Nginx performance dashboard renders cards, tabs, tables, and parameter selects', async ({ page }) => {
  test.setTimeout(120_000)
  const browserErrors = collectBrowserErrors(page)

  const detailResponsePromise = waitForApiResponse(page, '/api/nginx/detail_status', 'GET')
  await gotoRoute(page, '/dashboard/nginx')
  const detailResponse = await detailResponsePromise
  expect(detailResponse.ok()).toBe(true)

  const performanceDashboard = page.locator('.performance-dashboard')
  await expect(performanceDashboard).toBeVisible()
  await expectCardBodiesToRender(performanceDashboard.locator('.ant-card'))

  const performanceStatisticValues = performanceDashboard.locator('.ant-statistic-content-value')
  await expect.poll(() => performanceStatisticValues.count()).toBeGreaterThan(0)
  const performanceStatisticCount = await performanceStatisticValues.count()
  for (let index = 0; index < performanceStatisticCount; index++) {
    await expect(performanceStatisticValues.nth(index)).toHaveText(/\S/)
  }

  const performanceTabs = performanceDashboard.locator('.ant-tabs').first()
  await expect(performanceTabs).toBeVisible()
  await expectTabPanelsToRender(performanceTabs, true)

  const modulesPanel = performanceTabs.locator('[role="tabpanel"][aria-hidden="false"]').first()
  const moduleRow = tableRows(modulesPanel).first()
  const moduleCells = moduleRow.locator('td')
  await expect(moduleCells).toHaveCount(3)
  await expect(moduleCells.nth(0)).toContainText(/\S/)
  await expect(moduleCells.nth(0).locator('div').first()).toBeVisible()
  await expect(moduleCells.nth(1).locator('span').first()).toContainText(/\S/)
  await expect(moduleCells.nth(2).locator('span').first()).toContainText(/\S/)
  await expect(moduleCells.nth(1)).not.toHaveText(/^\s*(?:true|false)\s*$/)
  await expect(moduleCells.nth(2)).not.toHaveText(/^\s*(?:true|false)\s*$/)

  const performanceCard = performanceDashboard.locator('> .ant-card').first()
  const performanceRequestPromise = waitForApiResponse(page, '/api/nginx/performance', 'GET')
  await performanceCard.getByRole('button').first().click()
  const performanceResponse = await performanceRequestPromise
  expect(performanceResponse.ok()).toBe(true)

  const modalContainer = page.locator('.ant-modal:visible .ant-modal-container').last()
  await expect(modalContainer).toBeVisible()
  await expect(modalContainer).toContainText(/\S/)
  await expect.poll(async () => (await modalContainer.boundingBox())?.width ?? 0).toBeGreaterThan(0)

  const parameterTabs = modalContainer.locator('.ant-tabs').first()
  await expectTabPanelsToRender(parameterTabs, false)

  const performanceParameterTab = parameterTabs.locator('.ant-tabs-tab').first().getByRole('tab')
  await performanceParameterTab.click()
  const performanceParameterPanel = parameterTabs.locator('[role="tabpanel"][aria-hidden="false"]').first()
  await expectVisibleSelectsToHaveOptions(page, performanceParameterPanel, 'Performance parameter Select')

  const cacheTab = parameterTabs.locator('.ant-tabs-tab').nth(1).getByRole('tab')
  await cacheTab.click()
  const cachePanel = parameterTabs.locator('[role="tabpanel"][aria-hidden="false"]').first()
  await expect(cachePanel).toContainText(/\S/)

  const enableCacheSwitch = cachePanel.locator('.ant-switch').first()
  await expect(enableCacheSwitch).toBeVisible()
  await enableCacheSwitch.click()
  await expect.poll(() => cachePanel.locator('.ant-select:visible').count()).toBeGreaterThan(0)
  await expectVisibleSelectsToHaveOptions(page, cachePanel, 'Cache parameter Select')

  expectNoUnexpectedBrowserErrors(browserErrors)
})
