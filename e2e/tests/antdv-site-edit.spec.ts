import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute, tableRows } from './helpers'

function isExpectedNetworkConsoleError(message: string) {
  return /Failed to load resource|net::ERR_(?:ABORTED|CONNECTION|FAILED|RESET|TIMED_OUT)/i.test(message)
}

function normalizeVisibleText(value: string) {
  return value.replace(/\s+/g, ' ').trim()
}

async function expectPopulated(locator: Locator, description: string) {
  await expect.poll(async () => {
    const text = normalizeVisibleText(await locator.innerText().catch(() => ''))
    const controls = await locator.locator(
      'input, textarea, button, [contenteditable="true"], .ant-empty, .ant-tabs, .ant-collapse, .nui-list, .cm-editor',
    ).count()

    return text.length > 0 || controls > 0
  }, {
    message: `${description} should render content`,
  }).toBe(true)
}

async function assertSelectHasOptions(page: Page, select: Locator, description: string) {
  const trigger = select.getByRole('combobox').first()
  await expect(trigger, `${description} should have a Select trigger`).toBeVisible()
  if (await trigger.isDisabled())
    return

  const content = select.locator('.ant-select-content').first()
  const closedText = normalizeVisibleText(await content.innerText())
  const placeholder = select.locator('.ant-select-placeholder').first()
  const hasPlaceholder = await placeholder.count() > 0
    && normalizeVisibleText(await placeholder.innerText().catch(() => '')) !== ''

  await trigger.click()

  await expect.poll(async () => await page.locator('.ant-select-dropdown:visible').count() + await page.locator('.ant-modal:visible').count(), {
    message: `${description} should open a dropdown or selector dialog`,
  }).toBeGreaterThan(0)

  const dropdowns = page.locator('.ant-select-dropdown:visible')
  if (await dropdowns.count() === 0) {
    const selectorModal = page.locator('.ant-modal:visible').last()
    await expect(selectorModal, `${description} should open its selector dialog`).toBeVisible()
    await expectPopulated(selectorModal, `${description} selector dialog`)

    const selectorTable = selectorModal.locator('.ant-table').first()
    await expect(selectorTable, `${description} selector dialog should render its table`).toBeVisible()
    const selectorRows = tableRows(selectorTable)
    await expect.poll(async () => {
      if (await selectorRows.count() > 0)
        return true

      return await selectorTable.locator('.ant-table-placeholder .ant-empty').count() > 0
    }, {
      message: `${description} selector dialog should render rows or an explicit empty state`,
    }).toBe(true)
    await closeModal(selectorModal)
    return
  }

  const dropdown = dropdowns.last()
  await expect(dropdown, `${description} dropdown should open`).toBeVisible()
  await expect(dropdown, `${description} dropdown must not be empty`).not.toHaveClass(/ant-select-dropdown-empty/)
  await expect(dropdown.locator('.ant-select-item-empty'), `${description} must not show an empty state`).toHaveCount(0)

  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count(), {
    message: `${description} should expose at least one option`,
  }).toBeGreaterThan(0)
  const optionTexts = await options.allTextContents()
  expect(optionTexts.some(option => normalizeVisibleText(option).length > 0), `${description} options should have labels`).toBe(true)

  if (closedText && !hasPlaceholder) {
    expect(closedText, `${description} should display a human label, not a raw enum value`)
      .not.toMatch(/^[a-z][a-z0-9]*(?:[_-][a-z0-9]+)*$/)
  }

  await page.keyboard.press('Escape')
  await expect(dropdown).toBeHidden()
}

async function assertVisibleSelects(page: Page, root: Locator, description: string) {
  const selects = root.locator('.ant-select:visible')
  const count = await selects.count()

  for (let index = 0; index < count; index++) {
    await assertSelectHasOptions(page, selects.nth(index), `${description} Select ${index + 1}`)
  }
}

async function selectFirstOption(page: Page, select: Locator, description: string) {
  await select.getByRole('combobox').first().click()
  const dropdown = page.locator('.ant-select-dropdown:visible').last()
  await expect(dropdown, `${description} dropdown should open before selecting`).toBeVisible()
  const option = dropdown.locator('.ant-select-item-option').first()
  await expect(option, `${description} should have a first option`).toBeVisible()
  await option.click()
}

async function assertTabSetHasContent(tabSet: Locator, description: string) {
  const tabItems = tabSet.locator(':scope > .ant-tabs-nav .ant-tabs-tab')
  const tabCount = await tabItems.count()
  expect(tabCount, `${description} should render tabs`).toBeGreaterThan(0)

  for (let index = 0; index < tabCount; index++) {
    const tab = tabItems.nth(index)
    await expect(tab.getByRole('tab'), `${description} tab ${index + 1} should be visible`).toBeVisible()
    await tab.click()

    const panel = tabSet.locator(':scope > .ant-tabs-body-holder .ant-tabs-content-active')
    await expect(panel, `${description} tab ${index + 1} panel should be visible`).toBeVisible()
    await expectPopulated(panel, `${description} tab ${index + 1} panel`)
  }
}

async function closeModal(modal: Locator) {
  await expect(modal.locator('.ant-modal-close')).toBeVisible()
  await modal.locator('.ant-modal-close').click()
  await expect(modal).toBeHidden()
}

async function assertHistoryTable(modal: Locator) {
  const table = modal.locator('.ant-table')
  await expect(table, 'configuration history should render a table').toBeVisible()

  const rows = tableRows(table)
  await expect.poll(async () => {
    if (await rows.count() > 0)
      return true

    return await table.locator('.ant-table-placeholder .ant-empty').count() > 0
  }, {
    message: 'configuration history should render rows or an explicit empty state',
  }).toBe(true)

  if (await rows.count() > 0) {
    await expect(rows.first()).toContainText(/\S+/)
    await expect(rows.first().locator('td').last()).toContainText(/\S+/)
  }
}

async function inspectTemplateList(page: Page, panel: Locator) {
  const list = panel.locator('.nui-list')
  await expect(list, 'config template list should render').toBeVisible()

  const items = list.locator('.nui-list-item')
  await expect.poll(() => items.count(), {
    message: 'config template list should contain items',
  }).toBeGreaterThan(0)

  const itemCount = await items.count()
  for (let index = 0; index < itemCount; index++) {
    const item = items.nth(index)
    await expect(item.locator('.nui-list-item-meta-title')).toContainText(/\S+/)
    await expect(item.locator('.nui-list-item-meta-description')).toContainText(/\S+/)
  }

  let foundTemplateSelect = false
  for (let index = 0; index < itemCount; index++) {
    const item = items.nth(index)
    const viewButton = item.locator('.nui-list-item-actions').getByRole('button').first()
    const detailResponsePromise = page.waitForResponse(response => {
      const url = new URL(response.url())
      return url.pathname.startsWith('/api/templates/block/')
        && response.request().method() === 'GET'
    })

    await viewButton.click()
    const detailResponse = await detailResponsePromise
    expect(detailResponse.ok()).toBe(true)

    const modal = page.locator('.ant-modal:visible').last()
    await expect(modal.locator('.ant-modal-container')).toBeVisible()
    await expectPopulated(modal, `config template ${index + 1} modal`)

    if (await modal.locator('.ant-select:visible').count() > 0) {
      await assertVisibleSelects(page, modal, `config template ${index + 1}`)
      foundTemplateSelect = true
    }

    await closeModal(modal)

    if (foundTemplateSelect)
      break
  }

  expect(foundTemplateSelect, 'at least one reachable config template should exercise a Select').toBe(true)
}

test('site editor preserves migrated antdv-next controls and content', async ({ page }) => {
  test.setTimeout(180_000)

  const consoleErrors: string[] = []
  const pageErrors: string[] = []

  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedNetworkConsoleError(message.text()))
      consoleErrors.push(message.text())
  })
  page.on('pageerror', error => pageErrors.push(error.message))

  await gotoRoute(page, '/sites/list')
  const siteContainer = page.locator('.site-container')
  const siteEditor = siteContainer.locator('.site-edit-container')
  const rightPanel = siteContainer.locator('.right-settings-container')

  let siteRows = await expectTableRows(page, 1)
  let foundBasicModeSite = false
  const candidateCount = await siteRows.count()
  for (let candidateIndex = 0; candidateIndex < candidateCount; candidateIndex++) {
    const editButton = siteRows.nth(candidateIndex).getByRole('button').first()
    await expect(editButton).toBeVisible()
    await editButton.click()
    await expect(siteEditor).toBeVisible()
    await expect(rightPanel).toBeVisible()

    const candidateTabs = rightPanel.locator('.ant-tabs').first()
    const candidateTabItems = candidateTabs.locator(':scope > .ant-tabs-nav .ant-tabs-tab')
    await expect.poll(() => candidateTabItems.count(), {
      message: 'site editor should render right-panel tabs before choosing a fixture',
    }).toBeGreaterThan(0)

    if (await candidateTabs.locator(':scope > .ant-tabs-nav .ant-tabs-tab[data-node-key="config-template"]').count() > 0) {
      foundBasicModeSite = true
      break
    }

    if (candidateIndex < candidateCount - 1) {
      await gotoRoute(page, '/sites/list')
      siteRows = await expectTableRows(page, 1)
    }
  }

  expect(foundBasicModeSite, 'the demo should expose one basic-mode site-edit fixture').toBe(true)

  const rightTabs = rightPanel.locator('.ant-tabs').first()
  const rightTabItems = rightTabs.locator(':scope > .ant-tabs-nav .ant-tabs-tab')
  await expect.poll(() => rightTabItems.count(), {
    message: 'site editor right panel should render its tabs',
  }).toBeGreaterThan(0)

  for (let index = 0; index < await rightTabItems.count(); index++) {
    const tabItem = rightTabItems.nth(index)
    const tabKey = await tabItem.getAttribute('data-node-key')
    await tabItem.click()

    const panel = rightTabs.locator(':scope > .ant-tabs-body-holder .ant-tabs-content-active')
    await expect(panel).toBeVisible()
    await expectPopulated(panel, `right panel tab ${tabKey ?? index + 1}`)
    await assertVisibleSelects(page, panel, `right panel tab ${tabKey ?? index + 1}`)

    if (tabKey === 'basic') {
      const syncStrategyTrigger = panel.locator('.anticon-info-circle').first()
      await expect(syncStrategyTrigger).toBeVisible()
      await syncStrategyTrigger.click()

      const popover = page.locator('.ant-popover:visible').last()
      await expect(popover).toBeVisible()
      await expectPopulated(popover, 'sync strategy popover')
      const popoverBox = await popover.boundingBox()
      expect(popoverBox?.width ?? 0, 'sync strategy popover should have non-zero width').toBeGreaterThan(0)
      await page.keyboard.press('Escape')
      await expect(popover).toBeHidden()
    }

    if (tabKey === 'dns') {
      const dnsSelects = panel.locator('.ant-select:visible')
      await expect(dnsSelects.first(), 'DNS domain Select should be reachable').toBeVisible()
      await selectFirstOption(page, dnsSelects.first(), 'DNS domain')

      await expect.poll(() => panel.locator('.ant-select:visible').count(), {
        message: 'selecting a DNS domain should reveal the DNS record Select',
      }).toBeGreaterThan(1)
      await expect(panel.locator('.ant-select:visible').nth(1).getByRole('combobox').first(), 'DNS record Select should finish loading').toBeEnabled()
      await assertVisibleSelects(page, panel, 'DNS panel after selecting a domain')

      const checkboxes = panel.getByRole('checkbox')
      await expect(checkboxes.last(), 'DNS panel should expose the local create-record control').toBeVisible()
      await checkboxes.last().check()
      await expect.poll(() => panel.locator('.ant-select:visible').count(), {
        message: 'creating a DNS record should reveal the record type Select',
      }).toBeGreaterThan(2)
      await assertVisibleSelects(page, panel, 'DNS create-record form')
    }

    if (tabKey === 'config-template')
      await inspectTemplateList(page, panel)
  }

  const historyButtons = siteEditor.locator('.ant-card-extra .ant-btn:visible')
  await expect(historyButtons).toHaveCount(2)
  const historyResponsePromise = page.waitForResponse(response => {
    const url = new URL(response.url())
    return url.pathname === '/api/config_histories' && response.request().method() === 'GET'
  })
  await historyButtons.first().click()
  const historyResponse = await historyResponsePromise
  expect(historyResponse.ok()).toBe(true)
  const historyModal = page.locator('.ant-modal:visible').last()
  await expect(historyModal.locator('.ant-modal-container')).toBeVisible()
  await expectPopulated(historyModal, 'configuration history modal')
  await assertHistoryTable(historyModal)
  await closeModal(historyModal)

  const quickSetupButton = siteEditor.locator('.ant-card-extra .ant-btn-primary').first()
  await expect(quickSetupButton).toBeVisible()
  await quickSetupButton.click()
  const quickSetupModal = page.locator('.ant-modal:visible').last()
  await expect(quickSetupModal.locator('.ant-modal-container')).toBeVisible()
  await expect(quickSetupModal.locator('form')).toBeVisible()
  await expectPopulated(quickSetupModal, 'quick setup modal')
  await assertVisibleSelects(page, quickSetupModal, 'quick setup')
  await closeModal(quickSetupModal)

  const mainCollapse = siteEditor.locator('.ant-collapse').first()
  await expect(mainCollapse).toBeVisible()
  const collapseItems = mainCollapse.locator(':scope > .ant-collapse-item')
  await expect(collapseItems).toHaveCount(3)

  for (let index = 0; index < await collapseItems.count(); index++) {
    const item = collapseItems.nth(index)
    const header = item.locator(':scope > .ant-collapse-header')
    const panel = item.locator('.ant-collapse-panel').first()
    await expect(header).toContainText(/\S+/)

    if (!(await panel.isVisible()))
      await header.click()
    await expect(panel).toBeVisible()
    await expectPopulated(panel, `main collapse item ${index + 1}`)

    const nestedTabs = panel.locator('.ant-tabs:visible').first()
    if (await nestedTabs.count() > 0) {
      await assertTabSetHasContent(nestedTabs, `main collapse item ${index + 1}`)

      const activeServerPanel = nestedTabs.locator(':scope > .ant-tabs-body-holder .ant-tabs-content-active')
      await assertVisibleSelects(page, activeServerPanel, `main editor tab ${index + 1}`)

      const dropdownTrigger = nestedTabs.locator('.ant-dropdown-trigger:visible').first()
      if (await dropdownTrigger.count() > 0) {
        await dropdownTrigger.click()
        const dropdown = page.locator('.ant-dropdown:visible').last()
        await expect(dropdown).toBeVisible()
        await expectPopulated(dropdown, `main editor tab ${index + 1} dropdown`)
        await expect(dropdown.getByRole('menuitem')).toHaveCount(1)
        await page.keyboard.press('Escape')
        await expect(dropdown).toBeHidden()
      }

      const addDirectiveButton = activeServerPanel.locator('.ant-btn-block:visible').first()
      if (await addDirectiveButton.count() > 0) {
        await addDirectiveButton.click()
        await expect.poll(() => activeServerPanel.locator('.ant-select:visible').count(), {
          message: 'adding a directive should reveal the directive mode Select',
        }).toBeGreaterThan(0)
        await assertVisibleSelects(page, activeServerPanel, 'directive editor')
        const temporaryDirective = activeServerPanel.locator('.add-directive-temp')
        await expect(temporaryDirective).toBeVisible()
        await temporaryDirective.locator('button').first().click()
      }

      const locationCollapse = activeServerPanel.locator('.ant-collapse:visible').first()
      if (await locationCollapse.count() > 0) {
        const locationItems = locationCollapse.locator(':scope > .ant-collapse-item')
        for (let locationIndex = 0; locationIndex < await locationItems.count(); locationIndex++) {
          const locationItem = locationItems.nth(locationIndex)
          const locationHeader = locationItem.locator(':scope > .ant-collapse-header')
          const locationPanel = locationItem.locator('.ant-collapse-panel').first()
          await expect(locationHeader).toContainText(/\S+/)
          if (!(await locationPanel.isVisible()))
            await locationHeader.click()
          await expect(locationPanel).toBeVisible()
          await expectPopulated(locationPanel, `location ${locationIndex + 1}`)
        }
      }
    }
  }

  await expect(consoleErrors, `Unexpected browser console errors: ${consoleErrors.join(' | ')}`).toEqual([])
  await expect(pageErrors, `Unexpected page errors: ${pageErrors.join(' | ')}`).toEqual([])
})
