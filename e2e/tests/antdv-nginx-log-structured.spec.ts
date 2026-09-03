import { expect, test } from '@playwright/test'
import { expectPositiveText, expectTableRows, gotoRoute } from './helpers'

function isExpectedDemoNetworkError(message: string) {
  return /Failed to load resource: the server responded with a status of 403 \(Forbidden\)/.test(message)
}

test('structured nginx logs keep migrated controls and custom table renderers', async ({ page }) => {
  test.setTimeout(180_000)

  const consoleErrors: string[] = []
  const pageErrors: string[] = []

  page.on('console', message => {
    if (message.type() === 'error' && !isExpectedDemoNetworkError(message.text()))
      consoleErrors.push(message.text())
  })
  page.on('pageerror', error => pageErrors.push(error.message))

  await gotoRoute(page, '/nginx_log/access')

  const main = page.getByRole('main')
  await expect(main).toBeVisible()

  const table = main.locator('.ant-table').first()
  const rows = await expectTableRows(table, 1)
  await expect(rows.first()).toBeVisible()

  const statistics = main.locator('.ant-statistic')
  await expect.poll(() => statistics.count()).toBeGreaterThanOrEqual(6)
  await expectPositiveText(main.locator('.ant-statistic-content-value').first())

  // The fixed column layout and custom cell renderers must survive the table migration.
  await expect(table.locator('.ant-table-thead th')).toHaveCount(9)
  const firstRow = rows.first()
  const firstRowCells = firstRow.locator('td')
  await expect(firstRowCells).toHaveCount(9)
  await expect(firstRowCells.nth(0)).toHaveText(/\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}/)
  await expect(firstRowCells.nth(2).locator('.ant-tag')).toHaveCount(1)
  await expect(firstRowCells.nth(3).locator('.ant-tag')).toHaveCount(1)
  await expect(firstRowCells.nth(4)).toHaveText(/[A-Za-z]/)
  await expect.poll(() => rows.locator('td:nth-child(6) > div').count()).toBeGreaterThan(0)
  await expect.poll(() => rows.locator('td:nth-child(7) > div').count()).toBeGreaterThan(0)
  await expect.poll(() => rows.locator('td:nth-child(8) .ant-tag').count()).toBeGreaterThan(0)

  const rangePicker = main.locator('.ant-picker-range').first()
  await expect(rangePicker).toBeVisible()
  await expect(rangePicker.locator('input').nth(0)).toHaveValue(/\d{4}-\d{2}-\d{2}/)
  await expect(rangePicker.locator('input').nth(1)).toHaveValue(/\d{4}-\d{2}-\d{2}/)
  await rangePicker.locator('input').first().click()
  const calendar = page.locator('.ant-picker-dropdown:visible').last()
  await expect(calendar).toBeVisible()
  await expect.poll(() => calendar.locator('.ant-picker-cell').count()).toBeGreaterThan(0)
  const calendarBox = await calendar.boundingBox()
  expect(calendarBox?.width ?? 0).toBeGreaterThan(0)
  expect(calendarBox?.height ?? 0).toBeGreaterThan(0)
  await page.keyboard.press('Escape')
  await expect(calendar).not.toBeVisible()

  const quickSelect = main.locator('.ant-dropdown-trigger').first()
  await expect(quickSelect).toBeVisible()
  await quickSelect.click()
  const quickSelectPopup = page.locator('.ant-dropdown:visible').last()
  await expect(quickSelectPopup).toBeVisible()
  await expect.poll(() => quickSelectPopup.locator('.ant-dropdown-menu-item').count()).toBeGreaterThan(0)
  await expect(quickSelectPopup.locator('.ant-dropdown-menu-item').first()).toBeVisible()
  const quickSelectBox = await quickSelectPopup.boundingBox()
  expect(quickSelectBox?.width ?? 0).toBeGreaterThan(0)
  await page.keyboard.press('Escape')
  await expect(quickSelectPopup).not.toBeVisible()

  // Search Filters is a local collapse; opening it exposes every feature-level Select.
  const filterToggle = main.locator('.cursor-pointer:has(.anticon-caret-right)').first()
  await expect(filterToggle).toBeVisible()
  await filterToggle.click()

  const visibleSelects = main.locator('.ant-select:visible')
  await expect.poll(() => visibleSelects.count()).toBeGreaterThanOrEqual(6)
  const selectCount = await visibleSelects.count()

  for (let index = 0; index < selectCount; index++) {
    const select = visibleSelects.nth(index)
    const content = select.locator('.ant-select-content')
    const closedLabel = (await content.getAttribute('title') ?? await content.innerText()).trim()
    expect(closedLabel, `Select ${index} has no human-readable closed label`).not.toBe('')

    await select.getByRole('combobox').click()
    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)
    await expect.poll(() => dropdown.locator('.ant-select-item-option').count()).toBeGreaterThan(0)
    const optionLabels = await dropdown.locator('.ant-select-item-option').allTextContents()
    const selectClass = await select.getAttribute('class') ?? ''
    if (selectClass.includes('ant-pagination-options-size-changer')) {
      expect(closedLabel, `Pagination Select ${index} shows a raw page-size value`).not.toMatch(/^\d+$/)
    }
    else {
      expect(optionLabels, `Select ${index} closed on an option value instead of a label`).not.toContain(closedLabel)
    }

    await page.keyboard.press('Escape')
    await expect(dropdown).not.toBeVisible()
  }

  expect(pageErrors, `Unexpected browser page errors: ${pageErrors.join('; ')}`).toEqual([])
  expect(consoleErrors, `Unexpected browser console errors: ${consoleErrors.join('; ')}`).toEqual([])
})
