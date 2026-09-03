import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

interface BrowserIssues {
  consoleErrors: string[]
  pageErrors: string[]
}

function collectBrowserIssues(page: Page): BrowserIssues {
  const issues: BrowserIssues = {
    consoleErrors: [],
    pageErrors: [],
  }

  page.on('console', message => {
    if (message.type() !== 'error')
      return

    // The demo can intentionally fail network requests; browser/runtime errors are not allowed.
    if (!/Failed to load resource|net::ERR_|NetworkError|WebSocket connection .* failed/i.test(message.text()))
      issues.consoleErrors.push(message.text())
  })
  page.on('pageerror', error => issues.pageErrors.push(error.message))

  return issues
}

function expectNoBrowserIssues(issues: BrowserIssues) {
  expect(issues.consoleErrors, `Unexpected browser console errors: ${issues.consoleErrors.join('\n')}`).toEqual([])
  expect(issues.pageErrors, `Unexpected page errors: ${issues.pageErrors.join('\n')}`).toEqual([])
}

async function visibleSelectRoots(page: Page) {
  const comboboxes = page.getByRole('main').getByRole('combobox')
  const roots: Locator[] = []

  for (let index = 0; index < await comboboxes.count(); index++) {
    const combobox = comboboxes.nth(index)
    if (!await combobox.isVisible())
      continue

    const root = combobox.locator('xpath=ancestor::div[contains(concat(" ", normalize-space(@class), " "), " ant-select ")][1]')
    if ((await root.getAttribute('class'))?.includes('ant-select-auto-complete'))
      continue

    roots.push(root)
  }

  return roots
}

async function assertVisibleSelectsHaveOptions(page: Page) {
  const selects = await visibleSelectRoots(page)
  const selectCount = selects.length

  for (let index = 0; index < selectCount; index++) {
    const select = selects[index]
    const content = select.locator('.ant-select-content').first()
    await expect(content).toBeVisible()

    const displayedLabel = (await content.innerText()).trim()
    const hasSelectedValue = (await content.getAttribute('class'))?.includes('ant-select-content-has-value') ?? false
    if (hasSelectedValue)
      expect(displayedLabel, 'Selected Select values must render a human label').not.toBe('')

    await select.getByRole('combobox').click()

    const openedSurface = page.locator('.ant-select-dropdown:visible, .ant-modal-container:visible')
    await expect.poll(() => openedSurface.count()).toBeGreaterThan(0)

    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    if (await dropdown.count() > 0) {
      await expect(dropdown).toBeVisible()
      await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

      const options = dropdown.locator('.ant-select-item-option')
      await expect.poll(() => options.count()).toBeGreaterThan(0)

      const optionLabels = (await options.allInnerTexts()).map(option => option.trim()).filter(Boolean)
      expect(optionLabels, 'Select options must render visible labels').not.toEqual([])

      // A selected value must resolve to an option label, not expose its enum value.
      if (hasSelectedValue)
        expect(optionLabels).toContain(displayedLabel)
    }
    else {
      // StdSelector intentionally hides its dropdown and opens a selector modal.
      const selectorModal = page.locator('.ant-modal-container:visible').last()
      await assertOpenOverlayHasWidth(selectorModal)
      await expect(selectorModal.locator('.ant-modal-body')).toContainText(/\S/)
      await expect.poll(() => selectorModal.locator('.ant-table, .ant-empty, input, button').count()).toBeGreaterThan(0)
    }

    await page.keyboard.press('Escape')
    await expect(page.locator('.ant-select-dropdown:visible')).toHaveCount(0)
    await expect(page.locator('.ant-modal-container:visible')).toHaveCount(0)
  }
}

function tabRoot(tab: Locator) {
  return tab.locator('xpath=ancestor::div[contains(concat(" ", normalize-space(@class), " "), " ant-tabs ")][1]')
}

async function activateTab(tab: Locator, page: Page) {
  const root = tabRoot(tab)
  await tab.click()

  try {
    await expect(tab).toHaveAttribute('aria-selected', 'true', { timeout: 1_000 })
    return
  }
  catch {
    // Tabs hidden by the responsive overflow menu remain in the DOM but cannot be clicked there.
  }

  const more = root.locator('.ant-tabs-nav-more:visible').first()
  await expect(more, 'Overflowed tab must have a reachable overflow control').toBeVisible()
  await more.click()

  const dropdown = page.locator('.ant-tabs-dropdown:visible').last()
  await expect(dropdown).toBeVisible()
  const tabLabel = (await tab.innerText()).trim()
  const menuItem = page.locator('.ant-tabs-dropdown-menu-item:visible').filter({ hasText: tabLabel }).first()
  await expect(menuItem, `Overflow menu is missing tab ${tabLabel || 'without a label'}`).toBeVisible()
  await menuItem.click()

  await expect(tab).toHaveAttribute('aria-selected', 'true')
}

async function assertTabsHaveContent(tabs: Locator, page: Page, shouldCheckSelects = true) {
  const tabCount = await tabs.count()
  expect(tabCount, 'Expected reachable Tabs').toBeGreaterThan(0)

  for (let index = 0; index < tabCount; index++) {
    const tab = tabs.nth(index)
    const tabLabel = (await tab.innerText()).trim()
    await activateTab(tab, page)

    const panel = tabRoot(tab).locator('[role="tabpanel"][aria-hidden="false"]:visible').first()
    await expect(panel, `Tab ${tabLabel || index} did not render a visible panel`).toBeVisible()
    await expect.poll(async () => {
      const text = (await panel.innerText()).trim()
      const renderedChildren = await panel.locator('input, textarea, button, .ant-empty, .ant-collapse, .ant-tabs, .nui-list, .ace_editor, [role="img"]').count()
      return text.length > 0 || renderedChildren > 0
    }, { message: `Tab ${tabLabel || index} rendered an empty panel` }).toBe(true)

    if (shouldCheckSelects)
      await assertVisibleSelectsHaveOptions(page)
  }
}

async function assertOpenOverlayHasWidth(overlay: Locator) {
  await expect(overlay).toBeVisible()
  const box = await overlay.boundingBox()
  expect(box?.width ?? 0, 'Open overlay must have a non-zero width').toBeGreaterThan(0)
}

test('config file browser and editor preserve rendered rows, tabs, selects, and history', async ({ page }) => {
  const issues = collectBrowserIssues(page)

  await gotoRoute(page, '/config')

  const rows = await expectTableRows(page, 1)
  const rowCount = await rows.count()
  for (let index = 0; index < rowCount; index++) {
    const nameCell = rows.nth(index).locator('td').first()
    await expect(nameCell.locator('.flex')).toBeVisible()
    await expect(nameCell.locator('.i-tabler-folder-filled, .i-tabler-file')).toHaveCount(1)
    await expect(nameCell).toContainText(/\S/)
    await expect(rows.nth(index).locator('td').last()).toContainText(/\S/)
  }

  await assertVisibleSelectsHaveOptions(page)

  const fileRows = rows.filter({ has: page.locator('.i-tabler-file') })
  await expect.poll(() => fileRows.count()).toBeGreaterThan(0)
  const fileRow = fileRows.first()
  await fileRow.getByRole('button').first().click()

  const codeEditor = page.locator('.ace_editor').first()
  await expect(codeEditor).toBeVisible()
  await expect(codeEditor).toContainText(/\S/)

  await assertTabsHaveContent(page.locator('.right-settings').getByRole('tab'), page, false)

  const historyButton = page.locator('button').filter({ has: page.locator('.anticon-history') }).first()
  await expect(historyButton).toBeVisible()
  await historyButton.click()

  const historyModal = page.locator('.ant-modal-container:visible').last()
  await assertOpenOverlayHasWidth(historyModal)
  await expect(historyModal.locator('.ant-table')).toBeVisible()
  await expect(historyModal.locator('.history-footer')).toBeVisible()
  await expect(historyModal.locator('.ant-modal-body')).toContainText(/\S/)

  await page.keyboard.press('Escape')
  await expect(page.locator('.ant-modal-container:visible')).toHaveCount(0)

  expectNoBrowserIssues(issues)
})

test('NgxConfigEditor preserves Collapse panels, directive handles, tabs, lists, dropdowns, and popovers', async ({ page }) => {
  const issues = collectBrowserIssues(page)
  await page.setViewportSize({ width: 1920, height: 1000 })

  await gotoRoute(page, '/sites')
  const siteRows = await expectTableRows(page, 1)
  const firstSiteRow = siteRows.first()
  let siteName = (await firstSiteRow.getAttribute('data-row-key'))?.trim() ?? ''
  if (!siteName)
    siteName = (await firstSiteRow.innerText()).split('\n').map(value => value.trim()).find(Boolean) ?? ''
  expect(siteName, 'The populated site table must expose a site name').not.toBe('')

  await gotoRoute(page, `/sites/${encodeURIComponent(siteName)}`)

  const editor = page.locator('.site-edit-container').first()
  const collapse = editor.locator('.ant-collapse').first()
  await expect(collapse).toBeVisible()
  const collapseItems = collapse.locator('.ant-collapse-item')
  await expect.poll(() => collapseItems.count()).toBeGreaterThan(0)

  for (let index = 0; index < await collapseItems.count(); index++) {
    const item = collapseItems.nth(index)
    const header = item.locator('.ant-collapse-header')
    if (await header.getAttribute('aria-expanded') !== 'true')
      await header.click()

    const panel = item.locator('.ant-collapse-panel')
    await expect(panel).toBeVisible()
    await expect.poll(async () => {
      const text = (await panel.innerText()).trim()
      const renderedChildren = await panel.locator('.ace_editor, .ant-tabs, .ant-empty, .dir-editor-item, button, input, textarea').count()
      return text.length > 0 || renderedChildren > 0
    }, { message: `Collapse panel ${index} rendered empty content` }).toBe(true)
  }

  const serverPanel = collapseItems.last().locator('.ant-collapse-panel')
  const directives = serverPanel.locator('.dir-editor-item')
  await expect.poll(() => directives.count()).toBeGreaterThan(0)
  for (let index = 0; index < await directives.count(); index++)
    await expect(directives.nth(index).locator('.anticon-holder')).toBeVisible()

  const addDirectiveButton = serverPanel.locator('button.ant-btn-block').first()
  await expect(addDirectiveButton).toBeVisible()
  await addDirectiveButton.click()
  await expect(serverPanel.locator('.add-directive-temp')).toBeVisible()
  await assertVisibleSelectsHaveOptions(page)

  await assertTabsHaveContent(page.getByRole('tab'), page)

  const rightSettings = page.locator('.right-settings')
  const templateTab = rightSettings.locator('[data-node-key="config-template"] [role="tab"]')
  await expect(templateTab).toBeVisible()
  await activateTab(templateTab, page)

  const templateList = rightSettings.locator('.config-list-wrapper')
  await expect(templateList).toBeVisible()
  const templateItems = templateList.locator('.nui-list-item')
  await expect.poll(() => templateItems.count()).toBeGreaterThan(0)
  for (let index = 0; index < await templateItems.count(); index++) {
    const item = templateItems.nth(index)
    await expect(item.locator('.nui-list-item-meta-title')).toContainText(/\S/)
    await expect(item.locator('.nui-list-item-meta-description')).toContainText(/\S/)
  }

  await templateItems.first().getByRole('button').first().click()
  const templateModal = page.locator('.ant-modal-container:visible').last()
  await assertOpenOverlayHasWidth(templateModal)
  await expect.poll(async () => (await templateModal.locator('.ant-modal-title').innerText()).trim().length).toBeGreaterThan(0)
  await expect(templateModal.locator('.ant-modal-body')).toContainText(/\S/)
  await page.keyboard.press('Escape')
  await expect(page.locator('.ant-modal-container:visible')).toHaveCount(0)

  const serverDropdownTrigger = serverPanel.locator('.ant-dropdown-trigger').first()
  await expect(serverDropdownTrigger).toBeVisible()
  await serverDropdownTrigger.click()
  const dropdown = page.locator('.ant-dropdown:visible').last()
  await assertOpenOverlayHasWidth(dropdown)
  await expect(dropdown.locator('.ant-dropdown-menu-item, .ant-menu-item, [role="menuitem"]')).toHaveCount(1)
  await expect(dropdown).toContainText(/\S/)
  await page.keyboard.press('Escape')
  await expect(page.locator('.ant-dropdown:visible')).toHaveCount(0)

  const basicTab = rightSettings.locator('[data-node-key="basic"] [role="tab"]')
  await activateTab(basicTab, page)
  await assertVisibleSelectsHaveOptions(page)

  const infoIcon = rightSettings.locator('.anticon-info-circle').first()
  await expect(infoIcon).toBeVisible()
  await infoIcon.click()
  const popover = page.locator('.ant-popover:visible').last()
  await assertOpenOverlayHasWidth(popover)
  await expect(popover).toContainText(/\S/)
  await page.keyboard.press('Escape')
  await expect(page.locator('.ant-popover:visible')).toHaveCount(0)

  expectNoBrowserIssues(issues)
})
