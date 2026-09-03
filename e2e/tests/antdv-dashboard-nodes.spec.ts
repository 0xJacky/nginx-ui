import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute } from './helpers'

const expectedNetworkFailure = /failed to load resource:\s*net::|net::ERR_|failed to fetch|network error|networkerror/i

async function assertVisibleSelectsHaveOptions(page: Page, root: Locator | Page) {
  const selects = root.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)

  const selectCount = await selects.count()
  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const combobox = select.getByRole('combobox')
    const triggerLabel = select.locator('.ant-select-content')

    await expect(combobox).toBeVisible()
    await expect(triggerLabel).toBeVisible()
    const closedLabel = (await triggerLabel.innerText()).trim()
    expect(closedLabel, `Select ${index} has no rendered closed label`).toMatch(/\S/)

    await combobox.click()

    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)
    await expect(options.first()).toContainText(/\S/)

    const selectedValues = dropdown.locator('[role="option"][aria-selected="true"]')
    if (await selectedValues.count() > 0) {
      const selectedValue = (await selectedValues.first().innerText()).trim()

      // HTTP method labels are intentionally self-describing uppercase tokens. For
      // enum-like values, the trigger must expose the option label instead of the
      // raw value (for example, "200 OK" instead of "200").
      if (selectedValue && !/^[A-Z]+$/.test(selectedValue))
        expect(closedLabel, `Select ${index} exposes its raw enum value`).not.toBe(selectedValue)
    }

    await page.keyboard.press('Escape')
    await expect(dropdown).toBeHidden()
  }
}

test('dashboard nodes and site cards retain migrated component content', async ({ page }) => {
  test.setTimeout(120_000)

  const browserErrors: string[] = []
  page.on('console', message => {
    if (message.type() === 'error' && !expectedNetworkFailure.test(message.text()))
      browserErrors.push(`console: ${message.text()}`)
  })
  page.on('pageerror', error => {
    if (!expectedNetworkFailure.test(error.message))
      browserErrors.push(`pageerror: ${error.message}`)
  })

  await gotoRoute(page, '/dashboard/server')
  await expect(page.locator('.env-list-card')).toBeVisible()
  await assertVisibleSelectsHaveOptions(page, page)

  const indicator = page.locator('.indicator:visible').first()
  await expect(indicator).toBeVisible()
  await expect(indicator.locator('.node-name')).toHaveText(/\S/)
  await expect(indicator.locator('.ant-tag')).toBeVisible()

  const nodeItems = page.locator('.env-list-card .env-list-item')
  await expect.poll(() => nodeItems.count()).toBeGreaterThan(0)
  const nodeCount = await nodeItems.count()

  await expect(nodeItems.locator('.ant-avatar')).toHaveCount(nodeCount)
  await expect(nodeItems.locator('.env-tags .ant-tag')).toHaveCount(nodeCount)
  for (let index = 0; index < nodeCount; index++) {
    const item = nodeItems.nth(index)
    await expect(item).toBeVisible()
    await expect(item.locator('.ant-avatar')).toBeVisible()
    await expect(item.locator('.env-tags .ant-tag')).toContainText(/\S/)
    await expect(item.locator('.env-name')).toHaveText(/\S/)
    await expect(item.locator('.env-meta-wrapper')).toContainText(/\S/)

    const analytics = item.locator('.node-analytic')
    await expect(analytics).toBeVisible()
    await expect(analytics).toContainText(/\S/)
    await expect(analytics.locator('.link-btn')).toBeVisible()
    await expect(analytics.locator('.link-btn')).toContainText(/\S/)
  }

  const namespaceTabs = page.locator('.env-list-card .ant-tabs').first()
  const tabs = namespaceTabs.getByRole('tab')
  await expect.poll(() => tabs.count()).toBeGreaterThan(0)
  const tabCount = await tabs.count()
  for (let index = 0; index < tabCount; index++) {
    const tab = tabs.nth(index)
    await expect(tab).toBeVisible()
    await expect(tab).toContainText(/\S/)
    await tab.click()
    await expect(tab).toHaveAttribute('aria-selected', 'true')

    // NamespaceTabs uses the tab as a filter; the rendered panel is the list below it.
    await expect(page.locator('.env-list .nui-list-items')).toContainText(/\S/)
    await expect.poll(() => nodeItems.count()).toBeGreaterThan(0)
  }

  await gotoRoute(page, '/dashboard/sites')
  const siteCards = page.locator('.site-card')
  await expect.poll(() => siteCards.count()).toBeGreaterThan(0)
  const siteCount = await siteCards.count()

  for (let index = 0; index < siteCount; index++) {
    const card = siteCards.nth(index)
    await expect(card).toBeVisible()
    await expect(card.locator('.site-title')).toHaveText(/\S/)
    await expect(card.locator('.site-url')).toHaveText(/\S/)
    await expect(card.locator('.site-icon')).toBeVisible()

    const fallbackAvatar = card.locator('.avatar-fallback')
    const favicon = card.locator('.site-icon img')
    expect(await fallbackAvatar.count() + await favicon.count()).toBe(1)
    if (await fallbackAvatar.count() > 0)
      await expect(fallbackAvatar).toHaveText(/\S/)
    else
      await expect(favicon).toHaveAttribute('alt', /\S/)

    const status = card.locator('.site-status .status-indicator, .site-status .ant-tag')
    await expect(status).toHaveCount(1)
    await expect(status).toBeVisible()
  }

  const errorDetails = page.locator('[data-testid="site-health-check-error"]:visible')
  if (await errorDetails.count() > 0) {
    await errorDetails.first().hover()
    const tooltip = page.locator('.ant-tooltip:visible').last()
    await expect(tooltip).toBeVisible()
    await expect(tooltip).toContainText(/\S/)
    await expect.poll(async () => (await tooltip.boundingBox())?.width ?? 0).toBeGreaterThan(0)
  }

  const settingsButton = page.locator('.action .ant-btn').filter({ has: page.locator('.anticon-setting') }).first()
  await expect(settingsButton).toBeVisible()
  await settingsButton.click()
  await expect(page.locator('.site-card.settings-mode')).toHaveCount(siteCount)

  const configButton = page.locator('.site-card.settings-mode .site-card-config .ant-btn').first()
  await expect(configButton).toBeVisible()
  await configButton.click()

  const modal = page.locator('.ant-modal:visible').last()
  await expect(modal).toBeVisible()
  const modalContainer = page.locator('.ant-modal-container:visible').last()
  await expect(modalContainer).toBeVisible()
  await expect(modalContainer).toContainText(/\S/)
  await expect.poll(async () => (await modalContainer.boundingBox())?.width ?? 0).toBeGreaterThan(0)

  await assertVisibleSelectsHaveOptions(page, modal)

  const collapse = modal.locator('.ant-collapse')
  await expect(collapse).toHaveCount(1)
  const collapseHeader = collapse.locator('.ant-collapse-header')
  await collapseHeader.click()
  await expect(collapseHeader).toHaveAttribute('aria-expanded', 'true')
  const collapseContent = collapse.locator('.ant-collapse-panel:visible')
  await expect(collapseContent).toBeVisible()
  await expect(collapseContent).toContainText(/\S/)

  expect(browserErrors, browserErrors.join('\n')).toEqual([])
})
