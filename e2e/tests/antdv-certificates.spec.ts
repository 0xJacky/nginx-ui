import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import {
  expectEmptyTable,
  expectTableRows,
  gotoRoute,
} from './helpers'

function isExpectedDemoNetworkFailure(message: string) {
  return message.startsWith('Failed to load resource:')
    || message.startsWith('Failed to fetch short token:')
}

// Raw enum values are lowercase. Case-sensitive on purpose: "HTTP01" and "DNS01"
// are the translated labels of the challenge methods.
const rawEnumValue = /^(?:http01|dns01|wildcard|custom|self_signed|p256|p384)$/

async function expectPopulatedSelect(page: Page, select: Locator) {
  const trigger = select.getByRole('combobox')
  await expect(trigger).toBeVisible()
  await trigger.click()

  const dropdown = page.locator('.ant-select-dropdown:visible').last()
  await expect(dropdown).toBeVisible()
  await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count()).toBeGreaterThan(0)

  // Re-pick the option that is already selected so the form keeps its state:
  // switching the challenge method to HTTP01 would unmount the DNS credential
  // select that is checked next.
  const selectedOption = dropdown.locator('.ant-select-item-option-selected')
  const option = await selectedOption.count() > 0 ? selectedOption.first() : options.first()
  const optionLabel = (await option.innerText()).trim()
  expect(optionLabel).not.toBe('')
  expect(optionLabel).not.toMatch(rawEnumValue)

  await option.click()

  const closedLabel = select.locator('.ant-select-content')
  await expect(closedLabel).toBeVisible()
  await expect(closedLabel).toContainText(optionLabel)
  expect((await closedLabel.innerText()).trim()).not.toMatch(rawEnumValue)
}

test('certificate views keep antdv-next options and rendered content visible', async ({ page }) => {
  test.setTimeout(120_000)

  const unexpectedConsoleErrors: string[] = []
  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedDemoNetworkFailure(message.text()))
      unexpectedConsoleErrors.push(`console: ${message.text()}`)
  })
  page.on('pageerror', error => {
    unexpectedConsoleErrors.push(`pageerror: ${error.message}`)
  })

  await gotoRoute(page, '/certificates/list')
  await expect(page.locator('.ant-table')).toBeVisible()
  await expectEmptyTable(page)

  const listCard = page.locator('.ant-card').first()
  const listActions = listCard.locator('.ant-card-extra').getByRole('button')
  await expect(listActions).toHaveCount(3)
  await listActions.nth(2).click()

  const issueModal = page.locator('.ant-modal:visible').last()
  await expect(issueModal).toBeVisible()
  // DNS01, the default challenge method, shows the DNS Credential select only
  // while the DNS-01 plugin can run it; otherwise the plugin notice takes its
  // place (AutoCertForm.vue). demo.Dockerfile bundles no plugins, so the demo
  // shows the notice. Exactly one of the two must appear once the plugin state
  // has loaded.
  const dns01Notice = issueModal.locator('.ant-alert').filter({ hasText: 'DNS-01 plugin' })
  const credentialItem = issueModal.locator('.ant-form-item').filter({
    has: page.locator('.ant-form-item-label', { hasText: 'DNS Credential' }),
  })
  await expect.poll(async () => await dns01Notice.count() + await credentialItem.count()).toBe(1)
  const hasDns01Plugin = await credentialItem.count() === 1

  // Certificate Type, Challenge Method, Key Type and ACME User, plus DNS
  // Credential when the plugin is available.
  const issueSelects = issueModal.locator('.ant-select')
  await expect(issueSelects).toHaveCount(hasDns01Plugin ? 5 : 4)
  for (let index = 0; index < await issueSelects.count(); index++)
    await expectPopulatedSelect(page, issueSelects.nth(index))

  if (!hasDns01Plugin) {
    // Read-only check: installing a plugin is refused in demo mode.
    await expect(dns01Notice).toBeVisible()
    await expect(dns01Notice.getByRole('button', { name: 'Install DNS-01 plugin' })).toBeVisible()
    await expect(dns01Notice.getByRole('button', { name: 'Go to System > Plugins' })).toBeVisible()
  }

  // Let the select dropdowns finish closing, then move the focus from the last
  // select back to the dialog: a focused select takes the first Escape itself.
  await expect(page.locator('.ant-select-dropdown:visible')).toHaveCount(0)
  await issueModal.locator('.ant-modal-title').click()
  await page.keyboard.press('Escape')
  await expect(issueModal).toBeHidden()

  await gotoRoute(page, '/certificates/acme_users')
  const acmeRow = (await expectTableRows(page, 1)).first()
  await expect(acmeRow).toBeVisible()

  const statusTag = acmeRow.locator('.ant-tag').first()
  await expect(statusTag).toBeVisible()
  const statusLabel = (await statusTag.innerText()).trim()
  expect(statusLabel).not.toBe('')
  expect(statusLabel).not.toMatch(/^(valid|invalid)$/)

  const pageSizeSelect = page.locator('.ant-pagination-options .ant-select')
  await expect(pageSizeSelect).toBeVisible()
  await expectPopulatedSelect(page, pageSizeSelect)
  await expectTableRows(page, 1)

  const actionButtons = acmeRow.getByRole('button')
  await expect(actionButtons).toHaveCount(3)

  await actionButtons.nth(0).click()
  const viewModal = page.locator('.ant-modal:visible').last()
  await expect(viewModal).toBeVisible()
  const viewContainer = viewModal.locator('.ant-modal-container')
  await expect(viewContainer).toBeVisible()
  await expect.poll(async () => (await viewContainer.innerText()).trim().length).toBeGreaterThan(0)
  expect((await viewContainer.boundingBox())?.width ?? 0).toBeGreaterThan(0)

  await page.keyboard.press('Escape')
  await expect(viewModal).toBeHidden()

  await actionButtons.nth(1).click()
  const editModal = page.locator('.ant-modal:visible').last()
  await expect(editModal).toBeVisible()
  await expect.poll(async () => (await editModal.innerText()).trim().length).toBeGreaterThan(0)
  const caDirectorySelect = editModal.locator('.ant-select')
  await expect(caDirectorySelect).toHaveCount(1)
  await expectPopulatedSelect(page, caDirectorySelect)

  await page.keyboard.press('Escape')
  await expect(editModal).toBeHidden()

  await gotoRoute(page, '/certificates/import')
  const certificateEditor = page.locator('.certificate-content-editor')
  await expect(certificateEditor).toBeVisible()
  await expect(certificateEditor.locator('.certificate-file-upload')).toHaveCount(2)
  await expect(certificateEditor.locator('.code-editor-container')).toHaveCount(2)

  // The node picker stays collapsed until "Select nodes" is clicked
  // (CertificateSyncCard.vue).
  const syncCard = page.locator('.ant-card').filter({
    has: page.locator('.ant-card-head-title', { hasText: 'Sync to other nodes' }),
  })
  await syncCard.getByRole('button', { name: 'Select nodes' }).click()
  const syncNodeOptions = syncCard.locator('.ant-checkbox-group').first().locator('.ant-checkbox-wrapper')
  await expect.poll(() => syncNodeOptions.count()).toBeGreaterThan(0)
  for (let index = 0; index < await syncNodeOptions.count(); index++)
    expect((await syncNodeOptions.nth(index).innerText()).trim()).not.toBe('')

  expect(unexpectedConsoleErrors).toEqual([])
})
