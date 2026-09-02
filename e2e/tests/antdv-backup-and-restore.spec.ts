import type { Locator, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute, tableRows } from './helpers'

const rawSelectValues = new Set([
  'nginx_and_nginx_ui',
  'custom_dir',
  'local',
  's3',
  'daily',
  'weekly',
  'monthly',
  'pending',
  'success',
  'failed',
])

interface BrowserErrors {
  consoleErrors: string[]
  pageErrors: string[]
}

function collectBrowserErrors(page: Page): BrowserErrors {
  const browserErrors: BrowserErrors = {
    consoleErrors: [],
    pageErrors: [],
  }

  page.on('console', message => {
    if (message.type() === 'error')
      browserErrors.consoleErrors.push(message.text())
  })
  page.on('pageerror', error => browserErrors.pageErrors.push(error.message))

  return browserErrors
}

function isExpectedDemoNetworkFailure(message: string): boolean {
  return message.includes('net::ERR_')
    || /^Failed to load resource: the server responded with a status of 500\b/.test(message)
}

async function expectNoUnexpectedBrowserErrors(browserErrors: BrowserErrors) {
  const unexpectedConsoleErrors = browserErrors.consoleErrors.filter(message => !isExpectedDemoNetworkFailure(message))

  expect(unexpectedConsoleErrors, `Unexpected browser console errors: ${JSON.stringify(unexpectedConsoleErrors)}`).toEqual([])
  expect(browserErrors.pageErrors, `Browser page errors: ${JSON.stringify(browserErrors.pageErrors)}`).toEqual([])
}

async function expectHumanSelectLabel(select: Locator) {
  const content = select.locator('.ant-select-content')
  await expect(content).toBeVisible()

  const label = (await content.innerText()).trim()
  expect(label, 'The closed Select must show a visible label or placeholder').not.toBe('')
  expect(rawSelectValues.has(label), `The closed Select exposed raw enum value: ${label}`).toBe(false)
}

async function expectSelectOptions(page: Page, select: Locator) {
  await expectHumanSelectLabel(select)

  const trigger = select.getByRole('combobox')
  await expect(trigger).toBeVisible()
  await trigger.click()

  const dropdown = page.locator('.ant-select-dropdown:visible').last()
  await expect(dropdown).toBeVisible()
  await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)

  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count()).toBeGreaterThan(0)
  await expect(options.first()).toBeVisible()
  await expect(options.first()).toContainText(/\S/)

  await trigger.press('Escape')
  await expect(dropdown).toBeHidden()
}

async function chooseSelectOption(page: Page, select: Locator, optionIndex: number) {
  const trigger = select.getByRole('combobox')
  await trigger.click()

  const dropdown = page.locator('.ant-select-dropdown:visible').last()
  const options = dropdown.locator('.ant-select-item-option')
  await expect.poll(() => options.count()).toBeGreaterThan(optionIndex)
  await options.nth(optionIndex).click()
  await expect(dropdown).toBeHidden()
  await expectHumanSelectLabel(select)
}

test('backup and restore render their content and read-only form flow', async ({ page }) => {
  const browserErrors = collectBrowserErrors(page)

  await gotoRoute(page, '/backup/backup-and-restore')

  const cards = page.locator('.ant-card')
  await expect.poll(() => cards.count()).toBeGreaterThanOrEqual(2)
  for (const cardIndex of [0, 1]) {
    const card = cards.nth(cardIndex)
    await expect(card.locator('.ant-card-head-title')).toContainText(/\S/)
    await expect(card.locator('.ant-card-body')).toContainText(/\S/)
  }

  const backupCard = page.locator('.ant-card').filter({ has: page.locator('button.ant-btn-primary') }).first()
  const createBackupButton = backupCard.locator('button.ant-btn-primary').first()
  await expect(createBackupButton).toBeVisible()

  // The demo gates the real backup endpoint. A local response stub reaches the
  // frontend token modal without changing any server state.
  await page.route('**/api/backup', route => route.fulfill({
    status: 200,
    headers: {
      'content-disposition': 'attachment; filename="nginx-ui-backup.zip"',
      'content-type': 'application/zip',
      'x-backup-security': 'e2e-security-token',
    },
    body: Buffer.from([80, 75, 3, 4]),
  }))

  await createBackupButton.click()

  const tokenModal = page.locator('.ant-modal.backup-token-modal:visible').last()
  await expect(tokenModal).toBeVisible()
  const tokenModalContainer = tokenModal.locator('.ant-modal-container')
  await expect(tokenModalContainer).toBeVisible()
  await expect.poll(async () => (await tokenModalContainer.boundingBox())?.width ?? 0).toBeGreaterThan(400)
  await expect(tokenModal.locator('.ant-modal-body')).toContainText(/\S/)
  await expect(tokenModal.locator('.token-text')).toContainText('e2e-security-token')
  await expect(tokenModal.locator('.warning-box')).toContainText(/\S/)

  await tokenModal.locator('.ant-modal-close').click()
  await expect(tokenModal).toBeHidden()

  const restoreCard = page.locator('.ant-card').filter({ has: page.locator('.ant-upload-drag') }).first()
  const uploadDropzone = restoreCard.locator('.ant-upload-drag')
  await expect(uploadDropzone).toBeVisible()
  await expect(uploadDropzone).toContainText(/\S/)

  const fileInput = uploadDropzone.locator('input[type="file"]')
  await expect(fileInput).toHaveCount(1)
  await fileInput.setInputFiles({
    name: 'e2e-read-only-backup.zip',
    mimeType: 'application/zip',
    buffer: Buffer.from([80, 75, 3, 4]),
  })
  await expect(restoreCard.locator('.ant-upload-list-item-name')).toContainText('e2e-read-only-backup.zip')

  const restoreForm = restoreCard.locator('form')
  await expect(restoreForm).toBeVisible()
  await expect(restoreForm).toContainText(/\S/)

  const restoreCheckboxes = restoreForm.locator('input[type="checkbox"]')
  await expect(restoreCheckboxes).toHaveCount(3)
  await expect(restoreCheckboxes.first()).toBeDisabled()
  for (let checkboxIndex = 0; checkboxIndex < await restoreCheckboxes.count(); checkboxIndex++)
    await expect(restoreCheckboxes.nth(checkboxIndex)).toBeChecked()

  await expect(restoreForm.locator('input[type="text"]')).toHaveCount(1)
  await expect(restoreForm.locator('button.ant-btn-primary')).toBeVisible()

  await expectNoUnexpectedBrowserErrors(browserErrors)
})

test('auto backup controls expose populated Selects and overlays', async ({ page }) => {
  const browserErrors = collectBrowserErrors(page)

  await gotoRoute(page, '/backup/auto-backup')

  const table = page.locator('.ant-table').first()
  await expect(table).toBeVisible()
  const rows = tableRows(table)
  const emptyState = table.locator('.ant-table-placeholder .ant-empty').first()
  await expect.poll(async () => (await rows.count()) > 0 || await emptyState.isVisible()).toBe(true)

  const rowCount = await rows.count()
  if (rowCount === 0) {
    await expect(emptyState).toBeVisible()
    await expect(table.locator('.ant-table-placeholder')).toContainText(/\S/)
  }
  else {
    for (let rowIndex = 0; rowIndex < rowCount; rowIndex++)
      await expect(rows.nth(rowIndex)).toContainText(/\S/)
  }

  const headerCells = table.locator('.ant-table-thead th')
  await expect.poll(() => headerCells.count()).toBeGreaterThan(0)
  await expect(headerCells.first()).toBeVisible()

  const autoBackupCard = page.locator('.ant-card').filter({ has: table }).first()
  const visibleSelects = page.locator('.ant-select:visible')
  const visibleSelectCount = await visibleSelects.count()
  expect(visibleSelectCount).toBeGreaterThan(0)
  for (let selectIndex = 0; selectIndex < visibleSelectCount; selectIndex++)
    await expectSelectOptions(page, visibleSelects.nth(selectIndex))

  const settingsTrigger = autoBackupCard.locator('button.ant-dropdown-trigger').first()
  await expect(settingsTrigger).toBeVisible()
  await settingsTrigger.click()
  const settingsDropdown = page.locator('.ant-dropdown:visible').last()
  await expect(settingsDropdown).toBeVisible()
  await expect(settingsDropdown.locator('.table-column-settings')).toBeVisible()
  await expect(settingsDropdown.locator('.column-item').first()).toBeVisible()
  await expect(settingsDropdown).toContainText(/\S/)
  await page.keyboard.press('Escape')
  await expect(settingsDropdown).toBeHidden()

  const addLink = autoBackupCard.locator('.ant-card-extra a').first()
  await expect(addLink).toBeVisible()
  await addLink.click()

  const addModal = page.locator('.ant-modal:visible').last()
  await expect(addModal).toBeVisible()
  const addModalContainer = addModal.locator('.ant-modal-container')
  await expect(addModalContainer).toBeVisible()
  await expect.poll(async () => (await addModalContainer.boundingBox())?.width ?? 0).toBeGreaterThan(400)
  await expect(addModal.locator('.ant-modal-body')).toContainText(/\S/)

  const modalSelects = addModal.locator('.ant-select:visible')
  await expect(modalSelects).toHaveCount(3)
  for (let selectIndex = 0; selectIndex < await modalSelects.count(); selectIndex++)
    await expectSelectOptions(page, modalSelects.nth(selectIndex))

  const initialInputCount = await addModal.locator('input.ant-input:visible').count()
  const backupTypeSelect = modalSelects.nth(0)
  const storageTypeSelect = modalSelects.nth(1)
  const scheduleTypeSelect = modalSelects.nth(2)

  await chooseSelectOption(page, backupTypeSelect, 0)
  await chooseSelectOption(page, storageTypeSelect, 1)
  await expect.poll(() => addModal.locator('input.ant-input:visible').count()).toBeGreaterThan(initialInputCount)

  await chooseSelectOption(page, scheduleTypeSelect, 1)
  await expect.poll(() => addModal.locator('.ant-select:visible').count()).toBe(4)
  const dayOfWeekSelect = addModal.locator('.ant-select:visible').last()
  await expectSelectOptions(page, dayOfWeekSelect)
  await chooseSelectOption(page, dayOfWeekSelect, 0)

  await addModal.locator('.ant-modal-close').click()
  await expect(addModal).toBeHidden()

  await expectNoUnexpectedBrowserErrors(browserErrors)
})
