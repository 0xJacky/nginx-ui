import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute } from './helpers'

function isExpectedDemoNetworkFailure(message: string) {
  return message.startsWith('Failed to load resource:') || /net::ERR_[A-Z_]+/.test(message)
}

async function expectNonEmptyTexts(locator: Locator, description: string) {
  const texts = await locator.allTextContents()
  expect(texts, `${description} should contain at least one item`).not.toHaveLength(0)
  expect(texts.every(text => text.trim().length > 0), `${description} should not contain blank items`).toBe(true)
}

async function dismissOverlay(page: Page) {
  await page.locator('.terminal-layout').click({ position: { x: 10, y: 10 } })
}

async function expectDropdownMenu(page: Page, trigger: Locator) {
  await trigger.click()

  const dropdown = page.locator('.ant-dropdown:visible').last()
  await expect(dropdown).toBeVisible()

  const menuItems = dropdown.locator('.ant-dropdown-menu-item')
  await expect.poll(() => menuItems.count()).toBeGreaterThan(0)
  await expectNonEmptyTexts(menuItems, 'dropdown menu')

  return dropdown
}

test('terminal and assistant controls keep migrated components populated', async ({ page }) => {
  test.setTimeout(120_000)

  const consoleErrors: string[] = []
  const pageErrors: string[] = []

  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedDemoNetworkFailure(message.text())) {
      consoleErrors.push(message.text())
    }
  })
  page.on('pageerror', error => pageErrors.push(error.message))

  await gotoRoute(page, '/terminal')

  const terminalLayout = page.locator('.terminal-layout')
  await expect(terminalLayout).toBeVisible()

  const terminalTabs = page.locator('.terminal-header .terminal-tab')
  await expect.poll(() => terminalTabs.count()).toBeGreaterThan(0)
  await expect(page.locator('.terminal-session.active .console')).toBeVisible()
  await expect.poll(async () => {
    return (await page.locator('.terminal-session.active .console').innerText()).trim().length
  }).toBeGreaterThan(0)

  const statusValues = page.locator('.terminal-status-bar .status-item .value')
  await expect.poll(() => statusValues.count()).toBeGreaterThan(0)
  await expectNonEmptyTexts(statusValues, 'terminal status values')

  // Add a browser-local terminal so both terminal tabs and their panels are exercised.
  const initialTerminalTabCount = await terminalTabs.count()
  await page.locator('.terminal-header .add-tab-btn').click()
  await expect.poll(() => terminalTabs.count()).toBe(initialTerminalTabCount + 1)

  for (let index = 0; index < await terminalTabs.count(); index++) {
    const terminalTab = terminalTabs.nth(index)
    await expect(terminalTab.locator('.tab-name')).not.toHaveText('')
    await terminalTab.click()
    await expect(terminalTab).toHaveClass(/active/)

    const activeTerminal = page.locator('.terminal-session.active')
    await expect(activeTerminal).toHaveCount(1)
    await expect(activeTerminal.locator('.console')).toBeVisible()
    await expect.poll(async () => {
      return (await activeTerminal.locator('.console').innerText()).trim().length
    }).toBeGreaterThan(0)
  }

  const assistantToggle = page.locator('.terminal-header .header-actions .ant-btn')
  await expect(assistantToggle).toBeVisible()
  await expect(assistantToggle).not.toHaveText('')
  await assistantToggle.click()

  const assistantPanel = page.locator('.terminal-right-panel.dark')
  await expect(assistantPanel).toBeVisible()
  await expect(assistantPanel.locator('.llm-wrapper')).toBeVisible()

  // A Select whose label is replaced by an enum is just as broken when its popup has data.
  const selects = page.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)
  for (let index = 0; index < await selects.count(); index++) {
    const select = selects.nth(index)
    const content = select.locator('.ant-select-content')
    await expect(content).toBeVisible()
    const closedLabel = (await content.innerText()).trim()
    expect(closedLabel, 'closed Select should show a human-readable label').not.toBe('')

    await select.getByRole('combobox').click()
    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)
    await expectNonEmptyTexts(options, 'Select options')
    const optionLabels = (await options.allTextContents()).map(text => text.trim())
    expect(optionLabels).toContain(closedLabel)

    await dismissOverlay(page)
    await expect(dropdown).toBeHidden()
  }

  const assistant = assistantPanel.locator('.llm-wrapper')
  const sessionTabs = assistant.locator('.llm-session-tabs .tab-item')
  await expect.poll(() => sessionTabs.count()).toBeGreaterThan(0)
  await expectNonEmptyTexts(assistant.locator('.llm-session-tabs .tab-title'), 'assistant session tabs')

  for (let index = 0; index < await sessionTabs.count(); index++) {
    const sessionTab = sessionTabs.nth(index)
    await sessionTab.click()
    await expect(sessionTab).toHaveClass(/active/)

    await expect(assistant.locator('.message-container')).toBeVisible()
    await expect(assistant.locator('.nui-list.llm-log')).toBeVisible()
    await expect(assistant.locator('.nui-list-items')).toBeAttached()
    await expect(assistant.locator('.input-msg')).toBeVisible()
    await expect(assistant.locator('textarea')).toBeVisible()
  }

  // The history Popover contains the session titles and dates without changing state.
  const sessionsButton = assistant.locator('.sessions-btn')
  await sessionsButton.click()
  const historyPopover = page.locator('.ant-popover:visible').filter({ has: page.locator('.sessions-dropdown') }).last()
  await expect(historyPopover).toBeVisible()
  const historyContainer = historyPopover.locator('.ant-popover-container')
  await expect(historyContainer).toBeVisible()
  const historyWidth = await historyContainer.evaluate(element => Number.parseFloat(getComputedStyle(element).width))
  expect(historyWidth, 'session history Popover should have a non-zero width').toBeGreaterThan(0)

  const sessionItems = historyPopover.locator('.session-item')
  await expect.poll(() => sessionItems.count()).toBeGreaterThan(0)
  await expectNonEmptyTexts(historyPopover.locator('.session-title'), 'session history titles')
  await expectNonEmptyTexts(historyPopover.locator('.session-meta'), 'session history metadata')

  // The history entry has a second Dropdown trigger; verify its items too.
  const historyItem = sessionItems.first()
  await historyItem.hover()
  const historyMenu = await expectDropdownMenu(page, historyItem.locator('.session-actions .ant-dropdown-trigger'))
  await page.keyboard.press('Escape')
  await expect(historyMenu).toBeHidden()
  await page.keyboard.press('Escape')
  await expect(historyPopover).toBeHidden()

  // The visible session tab exposes another Dropdown with its popupRender menu.
  const visibleSessionTab = sessionTabs.first()
  await visibleSessionTab.hover()
  const sessionMenu = await expectDropdownMenu(page, visibleSessionTab.locator('.tab-action-btn'))
  await dismissOverlay(page)
  await expect(sessionMenu).toBeHidden()

  // Open the confirmation Popover but cancel it; no mutating action is submitted.
  const controlButtons = assistant.locator('.control-btn .ant-btn')
  await expect.poll(() => controlButtons.count()).toBeGreaterThanOrEqual(2)
  await controlButtons.first().click()
  const clearConfirmation = page.locator('.ant-popconfirm:visible').last()
  await expect(clearConfirmation).toBeVisible()
  await expectNonEmptyTexts(clearConfirmation.locator('.ant-popconfirm-message-text'), 'clear confirmation message')
  await expect(clearConfirmation.locator('.ant-popconfirm-buttons button')).toHaveCount(2)
  await clearConfirmation.locator('.ant-popconfirm-buttons button').first().click()
  await expect(clearConfirmation).toBeHidden()

  expect(consoleErrors, `Unexpected browser console errors: ${consoleErrors.join(' | ')}`).toEqual([])
  expect(pageErrors, `Unexpected page errors: ${pageErrors.join(' | ')}`).toEqual([])
})
