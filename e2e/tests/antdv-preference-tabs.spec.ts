import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute, tableRows } from './helpers'

// Navigation order of the Preference sections. Terminal is hidden in demo mode.
const preferenceSections = [
  { key: 'server', label: 'Server' },
  { key: 'app', label: 'App' },
  { key: 'node', label: 'Node' },
  { key: 'http', label: 'HTTP' },
  { key: 'auth', label: 'Auth' },
  { key: 'access_tokens', label: 'Access Tokens' },
  { key: 'cert', label: 'Cert' },
  { key: 'nginx', label: 'Nginx' },
  { key: 'plugin', label: 'Plugins' },
  { key: 'openai', label: 'LLM' },
  { key: 'health_check', label: 'Health Check' },
  { key: 'external_notify', label: 'External Notify' },
  { key: 'logrotate', label: 'Logrotate' },
]

async function openPreferenceSection(page: Page, label: string) {
  const navItem = page.locator('.preference-nav').getByRole('button', { name: label, exact: true })
  await navItem.click()

  await expect(navItem).toHaveAttribute('aria-current', 'page')
  const content = page.locator('.preference-content')
  await expect(content.locator('.preference-section-title')).toHaveText(label)
  await expect(content).not.toBeEmpty()
  return content
}

async function assertSelectOptions(page: Page, root: Locator, context: string) {
  // Collapsed sections keep their controls mounted but hidden (v-show).
  const selects = root.locator('.ant-select:visible')

  for (let index = 0; index < await selects.count(); index++) {
    const select = selects.nth(index)
    const combobox = select.getByRole('combobox')
    const content = select.locator('.ant-select-content').first()
    const closedText = (await content.innerText()).trim()
    const selectedLabel = await content.getAttribute('title')

    expect(closedText, `${context} select ${index} has no closed label`).not.toBe('')

    await combobox.click()
    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown, `${context} select ${index} did not open`).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)
    await expect.poll(
      () => dropdown.locator('.ant-select-item-option').count(),
      { message: `${context} select ${index} rendered no options` },
    ).toBeGreaterThan(0)

    const optionTexts = await dropdown.locator('.ant-select-item-option').allTextContents()
    expect(
      optionTexts.some(optionText => optionText.trim() !== ''),
      `${context} select ${index} rendered only blank options`,
    ).toBe(true)

    // ASelect uses a rendered label for a selected value. If the migration passes
    // through the raw enum instead, it will not match any rendered option label.
    const isAutoComplete = (await select.getAttribute('class') ?? '').includes('ant-select-auto-complete')
    if (!isAutoComplete && selectedLabel)
      expect(optionTexts.map(optionText => optionText.trim())).toContain(selectedLabel.trim())

    await page.keyboard.press('Escape')
    await expect(dropdown).toBeHidden()
  }
}

async function assertTableHasRenderedBody(root: Locator) {
  const table = root.locator('.ant-table').first()
  await expect(table).toBeVisible()
  await expect(table.locator('.ant-table-thead')).toBeVisible()

  const rows = tableRows(table)
  const emptyState = table.locator('.ant-table-placeholder')
  await expect.poll(async () => (await rows.count()) + (await emptyState.count())).toBeGreaterThan(0)

  if (await rows.count() > 0) {
    await expect(rows.first()).toBeVisible()
    await expect(rows.first()).not.toBeEmpty()
  }
  else {
    await expect(emptyState.first()).toBeVisible()
    await expect(emptyState.first()).not.toBeEmpty()
  }
}

async function assertModalContent(modal: Locator) {
  const container = modal.locator('.ant-modal-container')
  await expect(container).toBeVisible()
  await expect.poll(() => container.boundingBox().then(box => box?.width ?? 0)).toBeGreaterThan(200)
  await expect(container).not.toBeEmpty()
}

function isKnownDemoNetworkError(message: string) {
  return message.startsWith('Failed to load resource:')
    || message.startsWith('Failed to fetch short token:')
}

test('preference sections render every migrated panel and their controls', async ({ page }) => {
  const unexpectedConsoleErrors: string[] = []

  page.on('console', message => {
    if (message.type() === 'error' && !isKnownDemoNetworkError(message.text()))
      unexpectedConsoleErrors.push(message.text())
  })
  page.on('pageerror', error => {
    unexpectedConsoleErrors.push(`pageerror: ${error.message}`)
  })

  await gotoRoute(page, '/preference')
  const nav = page.locator('.preference-nav')
  await expect(nav).toBeVisible()
  await expect(nav.locator('.preference-nav-item')).toHaveText(preferenceSections.map(section => section.label))

  for (const { key, label } of preferenceSections) {
    const panel = await openPreferenceSection(page, label)

    if (key === 'openai') {
      // The API type select lives in the collapsed Advanced section.
      await panel.getByRole('button', { name: 'Advanced' }).click()
      await expect(panel.locator('.ant-select:visible')).toHaveCount(6)
    }

    await assertSelectOptions(page, panel, `preference section ${key}`)

    if (key === 'external_notify') {
      await assertTableHasRenderedBody(panel)

      const rows = tableRows(panel)
      if (await rows.count() > 0) {
        await expect(rows.first().locator('td').nth(0)).not.toBeEmpty()
        await expect(rows.first().locator('td').nth(1)).not.toBeEmpty()
        await expect(rows.first().getByRole('switch')).toBeVisible()
      }

      // StdCurd exposes its create action as a link. Open the form, but never save it.
      const addAction = panel.locator('.ant-card-extra a').first()
      await expect(addAction).toBeVisible()
      await addAction.click()

      const modal = page.locator('.ant-modal:visible').last()
      await expect(modal).toBeVisible()
      await assertModalContent(modal)
      await assertSelectOptions(page, modal, 'external notify modal')

      await modal.locator('.ant-modal-close').click()
      await expect(modal).toBeHidden()
    }

    if (key === 'auth') {
      await assertTableHasRenderedBody(panel)

      const rows = tableRows(panel)
      if (await rows.count() > 0) {
        await expect(rows.first().locator('td').nth(0)).not.toBeEmpty()
        await expect(rows.first().locator('td').nth(1)).not.toBeEmpty()
        await expect(rows.first().locator('td').nth(2)).not.toBeEmpty()
        await expect(rows.first().locator('td').nth(3)).not.toBeEmpty()
      }
    }

    if (key === 'access_tokens') {
      await assertTableHasRenderedBody(panel)

      const rows = tableRows(panel)
      if (await rows.count() > 0) {
        await expect(rows.first().locator('td').nth(0)).not.toBeEmpty()
        await expect(rows.first().locator('td').nth(1).locator('.ant-tag')).not.toHaveCount(0)
        await expect(rows.first().locator('td').nth(4).locator('.ant-tag')).not.toHaveCount(0)
      }

      const createAction = panel.getByRole('button', { name: 'Create Token' })
      await expect(createAction).toBeVisible()
      await createAction.click()

      const modal = page.locator('.ant-modal:visible').last()
      await expect(modal).toBeVisible()
      await assertModalContent(modal)
      await expect(modal.locator('.ant-form')).toBeVisible()

      await modal.locator('.ant-modal-close').click()
      await expect(modal).toBeHidden()
    }
  }

  expect(unexpectedConsoleErrors, `Unexpected browser errors: ${unexpectedConsoleErrors.join(' | ')}`).toEqual([])
})
