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

async function expectPopulatedSelect(page: Page, select: Locator) {
  const trigger = select.getByRole('combobox')
  await expect(trigger).toBeVisible()
  await trigger.click()

  const dropdown = page.locator('.ant-select-dropdown:visible').last()
  await expect(dropdown).toBeVisible()
  await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count()).toBeGreaterThan(0)

  const optionLabel = (await options.first().innerText()).trim()
  expect(optionLabel).not.toBe('')
  expect(optionLabel).not.toMatch(/^(http01|dns01|wildcard|custom|self_signed|p256|p384)$/i)

  await options.first().click()

  const closedLabel = select.locator('.ant-select-content')
  await expect(closedLabel).toBeVisible()
  await expect(closedLabel).toContainText(optionLabel)
  expect((await closedLabel.innerText()).trim()).not.toMatch(/^(http01|dns01|wildcard|custom|self_signed|p256|p384)$/i)
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
  const issueSelects = issueModal.locator('.ant-select')
  await expect(issueSelects).toHaveCount(4)
  for (let index = 0; index < await issueSelects.count(); index++)
    await expectPopulatedSelect(page, issueSelects.nth(index))

  const credentialInfo = issueModal.locator('.anticon-info-circle')
  await expect(credentialInfo).toBeVisible()
  await credentialInfo.hover()
  const credentialTooltip = page.locator('.ant-tooltip:visible').last()
  await expect(credentialTooltip).toBeVisible()
  await expect.poll(async () => (await credentialTooltip.innerText()).trim().length).toBeGreaterThan(0)
  expect((await credentialTooltip.boundingBox())?.width ?? 0).toBeGreaterThan(0)

  await page.keyboard.press('Escape')
  await expect(credentialTooltip).toBeHidden()
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

  const syncNodeOptions = page.locator('.ant-checkbox-group').first().locator('.ant-checkbox-wrapper')
  await expect.poll(() => syncNodeOptions.count()).toBeGreaterThan(0)
  for (let index = 0; index < await syncNodeOptions.count(); index++)
    expect((await syncNodeOptions.nth(index).innerText()).trim()).not.toBe('')

  expect(unexpectedConsoleErrors).toEqual([])
})
