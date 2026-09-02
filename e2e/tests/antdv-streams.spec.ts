import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import {
  expectEmptyTable,
  gotoRoute,
  tableRows,
  waitForApiResponse,
} from './helpers'

interface StreamListResponse {
  data?: unknown
}

function isIgnorableDemoNetworkError(message: string) {
  return /Failed to load resource|net::ERR_|NetworkError|Network Error|Load failed/i.test(message)
}

async function assertSelectsHaveOptions(page: Page, selects: Locator) {
  const selectCount = await selects.count()
  expect(selectCount, 'Expected at least one reachable Select control').toBeGreaterThan(0)

  for (let index = 0; index < selectCount; index++) {
    const select = selects.nth(index)
    const trigger = select.getByRole('combobox')

    await expect(trigger).toBeVisible()
    await trigger.click()

    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)

    const renderedLabel = (await options.first().innerText()).trim()
    expect(renderedLabel).not.toBe('')

    // Selecting an option only changes the local filter/editor state and closes
    // the popup, which lets the assertion verify the rendered human label.
    await options.first().click()
    await expect(page.locator('.ant-select-dropdown:visible')).toHaveCount(0)
    await expect(select.locator('.ant-select-content')).toHaveText(renderedLabel, { exact: true })
  }
}

async function assertNonEmptyRegion(region: Locator) {
  await expect(region).toBeVisible()
  await expect.poll(async () => {
    const textLength = (await region.innerText()).trim().length
    const renderedControls = await region.locator(
      'input, textarea, button, [role="textbox"], .ant-empty, .ant-form-item, .ant-table, .nui-list-item',
    ).count()
    return textLength + renderedControls
  }).toBeGreaterThan(0)
}

async function assertTabsHaveContent(tabs: Locator) {
  const tabItems = tabs.locator('.ant-tabs-tab:visible')
  const tabCount = await tabItems.count()
  expect(tabCount, 'Expected a visible Tabs navigation').toBeGreaterThan(0)

  for (let index = 0; index < tabCount; index++) {
    const tab = tabItems.nth(index)
    const tabButton = tab.getByRole('tab')
    expect((await tabButton.innerText()).trim()).not.toBe('')

    await tabButton.click()
    await expect(tabButton).toHaveAttribute('aria-selected', 'true')

    const panel = tabs.locator('.ant-tabs-content-active:visible').last()
    await assertNonEmptyRegion(panel)
  }
}

async function assertCollapsePanelsHaveContent(collapseRoots: Locator) {
  const rootCount = await collapseRoots.count()
  expect(rootCount, 'Expected a reachable Collapse').toBeGreaterThan(0)

  for (let rootIndex = 0; rootIndex < rootCount; rootIndex++) {
    const root = collapseRoots.nth(rootIndex)
    const items = root.locator('.ant-collapse-item')
    const itemCount = await items.count()
    expect(itemCount, 'Expected Collapse items').toBeGreaterThan(0)

    for (let itemIndex = 0; itemIndex < itemCount; itemIndex++) {
      const item = items.nth(itemIndex)
      const header = item.locator('.ant-collapse-header')
      const isOpen = await item.evaluate(element =>
        element.classList.contains('ant-collapse-item-active')
        || Boolean(element.querySelector('.ant-collapse-content-active')),
      )

      if (!isOpen)
        await header.click()

      const content = item.locator('.ant-collapse-content')
      await assertNonEmptyRegion(content)
    }
  }
}

async function assertRenderedListsHaveItems(root: Locator) {
  const lists = root.locator('.nui-list:visible')

  for (let index = 0; index < await lists.count(); index++) {
    const list = lists.nth(index)
    await expect(list).toBeVisible()

    const items = list.locator('.nui-list-item:visible')
    if (await items.count() === 0)
      continue

    for (let itemIndex = 0; itemIndex < await items.count(); itemIndex++) {
      await expect.poll(async () => (await items.nth(itemIndex).innerText()).trim().length)
        .toBeGreaterThan(0)
    }
  }
}

async function assertEditorOverlays(page: Page) {
  const historyButton = page.locator('.ant-card-extra .ant-btn:visible').filter({
    has: page.locator('.anticon-history'),
  }).first()

  if (await historyButton.count() > 0) {
    await historyButton.click()
    const historyModal = page.locator('.ant-modal:visible').last()
    await assertNonEmptyRegion(historyModal.locator('.ant-modal-body'))
    await expect.poll(async () => (await historyModal.locator('.ant-modal-container').boundingBox())?.width ?? 0)
      .toBeGreaterThan(0)
    await historyModal.locator('.ant-modal-close').click()
    await expect(historyModal).toBeHidden()
  }

  const syncStrategyTrigger = page.locator('.right-settings-container .text-trueGray-600:visible').first()
  if (await syncStrategyTrigger.count() > 0) {
    await syncStrategyTrigger.click()
    const popover = page.locator('.ant-popover:visible').last()
    await assertNonEmptyRegion(popover)
    await expect.poll(async () => (await popover.locator('.ant-popover-container').boundingBox())?.width
      ?? (await popover.boundingBox())?.width ?? 0).toBeGreaterThan(0)
  }

  const menuTrigger = page.locator('.domain-edit-container .ant-tabs-tab .anticon-more:visible').first()
  if (await menuTrigger.count() > 0) {
    await menuTrigger.click()
    const dropdown = page.locator('.ant-dropdown:visible').last()
    await assertNonEmptyRegion(dropdown)
    await expect.poll(() => dropdown.locator('.ant-menu-item:visible').count()).toBeGreaterThan(0)
  }
}

test('streams list and editor keep migrated controls populated', async ({ page }) => {
  test.setTimeout(120_000)

  const consoleErrors: string[] = []
  const pageErrors: string[] = []
  page.on('console', message => {
    if (message.type() === 'error')
      consoleErrors.push(message.text())
  })
  page.on('pageerror', error => pageErrors.push(error.message))

  const streamsResponsePromise = waitForApiResponse(page, '/api/streams', 'GET')
  await gotoRoute(page, '/streams')
  const streamsResponse = await streamsResponsePromise
  expect(streamsResponse.ok()).toBe(true)
  const streamsBody = await streamsResponse.json() as StreamListResponse

  await expect(page.locator('.ant-table:visible')).toBeVisible()
  const listTabs = page.locator('.ant-tabs:visible').first()
  const navigationTabs = listTabs.locator('.ant-tabs-tab:visible')
  await expect.poll(() => navigationTabs.count()).toBeGreaterThan(0)

  for (let index = 0; index < await navigationTabs.count(); index++) {
    const tab = navigationTabs.nth(index).getByRole('tab')
    await tab.click()
    await expect(tab).toHaveAttribute('aria-selected', 'true')
    await expect(page.locator('.ant-table:visible')).toBeVisible()
  }

  const rows = tableRows(page)
  const hasStreams = Array.isArray(streamsBody.data) && streamsBody.data.length > 0
  let streamName = ''

  if (hasStreams) {
    await expect.poll(() => rows.count()).toBeGreaterThan(0)
    const firstRow = rows.first()
    await expect(firstRow).toBeVisible()
    await expect(firstRow.locator('td').nth(1)).not.toBeEmpty()
    await expect(firstRow.locator('td').nth(2)).not.toBeEmpty()
    await expect(firstRow.locator('.stream-status-select')).toBeVisible()
    streamName = (await firstRow.locator('td').nth(1).innerText()).trim()
    expect(streamName).not.toBe('')
  }
  else {
    await expectEmptyTable(page)
  }

  await assertSelectsHaveOptions(page, page.locator('.ant-select:not(.ant-select-disabled):visible'))

  const addAction = page.locator('.ant-card-extra a').first()
  await expect(addAction).toBeVisible()
  await addAction.click()

  const addModal = page.locator('.ant-modal:visible').last()
  await assertNonEmptyRegion(addModal.locator('.ant-modal-body'))
  await expect(addModal.locator('.ant-modal-container')).toBeVisible()
  await expect(addModal.locator('.ant-modal-body input')).toBeVisible()
  await expect(addModal.locator('.ant-modal-footer .ant-btn')).toHaveCount(2)
  await addModal.locator('.ant-modal-close').click()
  await expect(addModal).toBeHidden()

  if (hasStreams) {
    const encodedName = encodeURIComponent(streamName)
    const editorResponsePromise = page.waitForResponse(response =>
      response.request().method() === 'GET'
      && new URL(response.url()).pathname.startsWith('/api/streams/'),
    )
    await gotoRoute(page, `/streams/${encodedName}`)
    const editorResponse = await editorResponsePromise
    expect(editorResponse.ok()).toBe(true)

    await expect(page.locator('.right-settings-container')).toBeVisible()
    await assertTabsHaveContent(page.locator('.right-settings-container .ant-tabs').first())

    const editorSelects = page.locator('.right-settings-container .ant-select:not(.ant-select-disabled):visible')
    if (await editorSelects.count() > 0)
      await assertSelectsHaveOptions(page, editorSelects)

    const collapseRoots = page.locator('.domain-edit-container .ant-collapse:visible')
    if (await collapseRoots.count() > 0) {
      await assertCollapsePanelsHaveContent(collapseRoots)

      const directiveAddButton = page.locator('.domain-edit-container .list-group + div .ant-btn:visible').first()
      if (await directiveAddButton.count() > 0) {
        await directiveAddButton.click()
        await assertSelectsHaveOptions(page, page.locator('.ant-select:not(.ant-select-disabled):visible'))
      }

      const editorTabs = page.locator('.domain-edit-container .ant-tabs:visible')
      for (let index = 0; index < await editorTabs.count(); index++)
        await assertTabsHaveContent(editorTabs.nth(index))
    }

    await assertEditorOverlays(page)
    await assertRenderedListsHaveItems(page.locator('.right-settings-container'))
  }

  expect(pageErrors, 'Unexpected browser page errors').toEqual([])
  expect(consoleErrors.filter(error => !isIgnorableDemoNetworkError(error)),
    'Unexpected browser console errors').toEqual([])
})
