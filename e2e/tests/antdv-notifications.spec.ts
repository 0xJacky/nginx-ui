import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import {
  expectEmptyTable,
  expectTableRows,
  gotoRoute,
  waitForApiResponse,
} from './helpers'

interface NotificationListResponse {
  data?: unknown
}

function isExpectedNetworkFailure(message: string) {
  return /Failed to load resource|net::ERR_|NetworkError|Load failed/i.test(message)
}

async function assertSelectHasOptions(page: Page, select: Locator, index: number) {
  await expect(select).toBeVisible()

  const content = select.locator('.ant-select-content')
  await expect(content).toBeVisible()
  const closedLabel = (await content.innerText()).trim()
  expect(closedLabel, `Notification select ${index} has no closed label`).not.toBe('')
  expect(closedLabel, `Notification select ${index} exposes a raw enum value`).not.toMatch(/^\d+$/)

  const combobox = select.getByRole('combobox')
  await expect(combobox).toBeVisible()
  const listId = await combobox.getAttribute('aria-controls')
  expect(listId, `Notification select ${index} has no popup list id`).toBeTruthy()
  if (!listId)
    throw new Error(`Notification select ${index} has no popup list id`)

  await combobox.click()

  const dropdown = page.locator('.ant-select-dropdown').filter({
    has: page.locator(`[id="${listId}"]`),
  })
  await expect(dropdown).toBeVisible()
  await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
  await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)

  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count()).toBeGreaterThan(0)
  await expect(options.first()).toBeVisible()
  await expect(options.first()).toContainText(/\S/)

  await page.keyboard.press('Escape')
  await expect(dropdown).toBeHidden()
}

async function assertRenderedNotificationRow(row: Locator) {
  const cells = row.locator('td.ant-table-cell')
  await expect(cells).toHaveCount(5)

  for (const index of [0, 1, 2, 3]) {
    await expect(cells.nth(index)).toBeVisible()
    await expect(cells.nth(index)).toContainText(/\S/)
  }

  await expect(cells.nth(0)).not.toHaveText(/^\s*\d+\s*$/)
}

async function assertRenderedNotificationListItem(item: Locator) {
  await expect(item).toBeVisible()
  await expect(item.locator('.nui-list-item-meta-title')).toContainText(/\S/)
  await expect(item.locator('.nui-list-item-meta-description')).toContainText(/\S/)
}

test('notification page and header popover render migrated antdv-next content', async ({ page }) => {
  const browserErrors: string[] = []
  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedNetworkFailure(message.text()))
      browserErrors.push(`console.error: ${message.text()}`)
  })
  page.on('pageerror', error => {
    browserErrors.push(`pageerror: ${error.message}`)
  })

  const notificationResponsePromise = waitForApiResponse(page, '/api/notifications', 'GET')
  await gotoRoute(page, '/notifications')

  const notificationResponse = await notificationResponsePromise
  expect(notificationResponse.ok()).toBe(true)
  const responseBody = await notificationResponse.json() as NotificationListResponse
  expect(Array.isArray(responseBody.data), 'Notifications API did not return a data array').toBe(true)
  const notificationRecords = Array.isArray(responseBody.data) ? responseBody.data : []

  const main = page.getByRole('main')
  await expect(main).toBeVisible()

  const selects = main.locator('.ant-select')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)
  const selectCount = await selects.count()
  for (let index = 0; index < selectCount; index++)
    await assertSelectHasOptions(page, selects.nth(index), index)

  const table = main.locator('.ant-table')
  await expect(table).toBeVisible()
  const headers = table.locator('.ant-table-thead > tr').first().locator('th')
  await expect(headers).toHaveCount(5)
  for (const header of await headers.all())
    await expect(header).toContainText(/\S/)

  if (notificationRecords.length === 0) {
    await expectEmptyTable(table)
  }
  else {
    const rows = await expectTableRows(table, 1)
    await expect.poll(() => rows.count()).toBe(notificationRecords.length)
    await assertRenderedNotificationRow(rows.first())
  }

  const bell = page.locator('.header [aria-label="bell"]')
  await expect(bell).toBeVisible()
  await bell.click()

  const popover = page.locator('.ant-popover:visible').filter({
    has: page.locator('.nui-list'),
  }).first()
  await expect(popover).toBeVisible()
  await expect(popover.locator('.ant-popover-content')).toContainText(/\S/)
  await expect(popover.locator('h3')).toContainText(/\S/)
  await expect(popover).toHaveCSS('width', '400px')

  const popoverContainer = popover.locator('.ant-popover-container')
  await expect(popoverContainer).toBeVisible()
  await expect(popoverContainer).toHaveCSS('width', '400px')
  await expect.poll(async () => (await popoverContainer.boundingBox())?.width ?? 0).toBeGreaterThan(0)

  const list = popover.locator('.nui-list')
  await expect(list).toBeAttached()
  const listItems = list.locator('.nui-list-item')
  if (notificationRecords.length === 0) {
    await expect(listItems).toHaveCount(0)
  }
  else {
    await expect.poll(() => listItems.count()).toBeGreaterThan(0)
    await assertRenderedNotificationListItem(listItems.first())
  }

  const popoverLinks = popover.locator('a')
  await expect(popoverLinks).toHaveCount(2)
  for (const link of await popoverLinks.all()) {
    await expect(link).toBeVisible()
    await expect(link).toContainText(/\S/)
  }

  const clearTrigger = popover.locator('h3').locator('..').locator('a')
  await clearTrigger.click()

  const confirmation = page.locator('.ant-popconfirm:visible').first()
  await expect(confirmation).toBeVisible()
  await expect(confirmation.locator('.ant-popconfirm-message-text')).toContainText(/\S/)
  const confirmationButtons = confirmation.locator('.ant-popconfirm-buttons button')
  await expect(confirmationButtons).toHaveCount(2)
  for (const button of await confirmationButtons.all()) {
    await expect(button).toBeVisible()
    await expect(button).toContainText(/\S/)
  }

  await confirmationButtons.first().click()
  await expect(confirmation).toBeHidden()

  expect(browserErrors, browserErrors.join('\n')).toEqual([])
})
