import type { Locator } from '@playwright/test'
import { expect, test } from '@playwright/test'
import {
  expectTableRows,
  gotoRoute,
  waitForApiResponse,
} from './helpers'

interface DashboardResponse {
  summary?: Record<string, unknown>
  hourly_stats?: unknown[]
  daily_stats?: unknown[]
  top_urls?: unknown[]
  browsers?: unknown[]
  operating_systems?: unknown[]
  devices?: unknown[]
}

interface GeographicResponse {
  data?: unknown[]
}

const numericCell = /^\s*\d[\d,]*(?:\.\d+)?\s*$/
const percentageCell = /^\s*-?\d+(?:\.\d+)?%\s*$/

function isExpectedNetworkConsoleError(message: string) {
  return /Failed to load resource|net::ERR_|NetworkError|Failed to fetch|WebSocket.*(?:failed|closed)/i.test(message)
}

async function expectRenderedAnalyticsTable(table: Locator) {
  await expect(table).toBeVisible()

  const headers = table.locator('.ant-table-thead > tr > th')
  await expect(headers).toHaveCount(3)
  for (let index = 0; index < await headers.count(); index++)
    await expect(headers.nth(index)).toContainText(/\S/)

  const rows = await expectTableRows(table, 1)
  for (let index = 0; index < await rows.count(); index++) {
    const cells = rows.nth(index).locator('td')
    await expect(cells).toHaveCount(3)
    await expect(cells.nth(0)).toContainText(/\S/)
    await expect(cells.nth(1)).toHaveText(numericCell)
    await expect(cells.nth(2)).toHaveText(percentageCell)
  }
}

test('nginx log dashboard keeps antdv-next controls, charts, and rendered tables populated', async ({ page }) => {
  test.setTimeout(240_000)

  const consoleErrors: string[] = []
  const pageErrors: string[] = []
  page.on('console', message => {
    if (message.type() === 'error')
      consoleErrors.push(message.text())
  })
  page.on('pageerror', error => pageErrors.push(error.message))

  await gotoRoute(page, '/nginx_log/access')

  const viewControl = page.locator('.ant-segmented').first()
  const viewItems = viewControl.locator('.ant-segmented-item')
  await expect(viewItems).toHaveCount(3)
  await expect(viewItems.nth(1)).toBeVisible()

  const dashboardResponsePromise = waitForApiResponse(page, '/api/nginx_log/dashboard', 'POST', 180_000)
  const worldResponsePromise = waitForApiResponse(page, '/api/nginx_log/geo/world', 'POST', 180_000)
  const chinaResponsePromise = waitForApiResponse(page, '/api/nginx_log/geo/china', 'POST', 180_000)
  await viewItems.nth(1).click()
  await expect(viewItems.nth(1).locator('input')).toBeChecked()

  const [dashboardResponse, worldResponse, chinaResponse] = await Promise.all([
    dashboardResponsePromise,
    worldResponsePromise,
    chinaResponsePromise,
  ])
  expect(dashboardResponse.ok()).toBe(true)
  expect(worldResponse.ok()).toBe(true)
  expect(chinaResponse.ok()).toBe(true)

  const dashboardBody = await dashboardResponse.json() as DashboardResponse
  expect(dashboardBody.summary).toBeTruthy()
  for (const key of ['hourly_stats', 'daily_stats', 'top_urls', 'browsers', 'operating_systems', 'devices'])
    expect(Array.isArray(dashboardBody[key as keyof DashboardResponse]), `${key} must be an array`).toBe(true)

  const worldBody = await worldResponse.json() as GeographicResponse
  const chinaBody = await chinaResponse.json() as GeographicResponse
  expect(Array.isArray(worldBody.data)).toBe(true)
  expect(Array.isArray(chinaBody.data)).toBe(true)

  const dashboard = page.locator('.dashboard-viewer')
  const quickSelect = dashboard.locator('.ant-dropdown-trigger').first()
  const rangePicker = dashboard.locator('.ant-picker').first()
  await expect(quickSelect).toBeVisible()
  await expect(rangePicker).toBeVisible()

  const rangeInputs = rangePicker.locator('.ant-picker-input input')
  await expect(rangeInputs).toHaveCount(2)
  await expect(rangeInputs.nth(0)).toHaveValue(/\S+/)
  await expect(rangeInputs.nth(1)).toHaveValue(/\S+/)

  await rangePicker.click()
  const calendar = page.locator('.ant-picker-dropdown:visible').last()
  await expect(calendar).toBeVisible()
  await expect.poll(() => calendar.locator('.ant-picker-cell').count()).toBeGreaterThan(0)
  await page.keyboard.press('Escape')
  await expect(calendar).toBeHidden()

  await quickSelect.click()
  const quickMenu = page.locator('.ant-dropdown:visible').last()
  await expect(quickMenu).toBeVisible()
  const quickMenuItems = quickMenu.locator('.ant-dropdown-menu-item')
  await expect.poll(() => quickMenuItems.count()).toBeGreaterThan(0)
  for (let index = 0; index < await quickMenuItems.count(); index++)
    await expect(quickMenuItems.nth(index)).toContainText(/\S/)
  const quickMenuBox = await quickMenu.boundingBox()
  expect(quickMenuBox?.width ?? 0).toBeGreaterThan(0)
  await page.keyboard.press('Escape')
  await expect(quickMenu).toBeHidden()

  const selects = page.locator('.ant-select:visible')
  await expect.poll(() => selects.count()).toBeGreaterThan(0)
  for (let index = 0; index < await selects.count(); index++) {
    const select = selects.nth(index)
    const content = select.locator('.ant-select-content')
    const selectedTitle = await content.getAttribute('title')
    const selectedLabel = (selectedTitle ?? await content.innerText()).trim()
    expect(selectedLabel, `Select ${index} has no human-readable selected label`).not.toBe('')

    await select.getByRole('combobox').click()
    const dropdown = page.locator('.ant-select-dropdown:visible').last()
    await expect(dropdown).toBeVisible()
    await expect(dropdown).not.toHaveClass(/ant-select-dropdown-empty/)
    await expect(dropdown.locator('.ant-select-item-empty')).toHaveCount(0)

    const options = dropdown.locator('.ant-select-item-option')
    await expect.poll(() => options.count()).toBeGreaterThan(0)
    const optionLabels = (await options.allTextContents()).map(label => label.trim())
    expect(optionLabels.every(label => label.length > 0)).toBe(true)
    expect(optionLabels).toContain(selectedLabel)

    const dropdownBox = await dropdown.boundingBox()
    expect(dropdownBox?.width ?? 0).toBeGreaterThan(0)
    await page.keyboard.press('Escape')
    await expect(dropdown).toBeHidden()
  }

  const statistics = dashboard.locator('.ant-statistic')
  await expect(statistics).toHaveCount(8)
  for (let index = 0; index < await statistics.count(); index++)
    await expect(statistics.nth(index).locator('.ant-statistic-content-value')).toContainText(/\S/)

  const charts = dashboard.locator('.apexcharts-canvas')
  await expect(charts).toHaveCount(2)
  for (let index = 0; index < await charts.count(); index++) {
    const chart = charts.nth(index)
    await expect(chart).toBeVisible()
    await expect(chart.locator('svg')).toBeVisible()
    await expect.poll(() => chart.locator('.apexcharts-series').count()).toBeGreaterThan(0)
  }

  const tables = dashboard.locator('.ant-table')
  await expect.poll(() => tables.count()).toBeGreaterThanOrEqual(4)
  const tableCount = await tables.count()
  // Geo tables, when available, precede these final four dashboard tables.
  for (const table of [
    tables.nth(tableCount - 4),
    tables.nth(tableCount - 3),
    tables.nth(tableCount - 2),
    tables.nth(tableCount - 1),
  ]) {
    await expectRenderedAnalyticsTable(table)
  }

  const geoCard = dashboard.locator('.geo-map-card')
  await expect(geoCard).toBeVisible()
  const worldMap = geoCard.locator('.world-map-content .world-map-container')
  const worldEmpty = geoCard.locator('.world-map-content .no-data')
  await expect.poll(async () => (await worldMap.count()) + (await worldEmpty.count())).toBeGreaterThan(0)
  if (await worldMap.count() > 0) {
    await expect(worldMap).toBeVisible()
    await expect(worldMap.locator('canvas')).toBeVisible()
    await expectTableRows(worldMap, 1)
  }
  else {
    await expect(worldEmpty).toBeVisible()
    await expect(worldEmpty).toContainText(/\S/)
  }

  const mapItems = geoCard.locator('.ant-segmented-item')
  if (await mapItems.count() > 1) {
    await expect(mapItems).toHaveCount(2)
    await mapItems.nth(1).click()
    await expect(mapItems.nth(1).locator('input')).toBeChecked()

    const chinaMap = geoCard.locator('.china-map-content .china-map-container')
    const chinaEmpty = geoCard.locator('.china-map-content .no-data')
    await expect.poll(async () => (await chinaMap.count()) + (await chinaEmpty.count())).toBeGreaterThan(0)
    if (await chinaMap.count() > 0) {
      await expect(chinaMap).toBeVisible()
      await expect(chinaMap.locator('canvas')).toBeVisible()
      await expectTableRows(chinaMap, 1)
    }
    else {
      await expect(chinaEmpty).toBeVisible()
      await expect(chinaEmpty).toContainText(/\S/)
    }

    await mapItems.nth(0).click()
    await expect(mapItems.nth(0).locator('input')).toBeChecked()
  }

  const unexpectedConsoleErrors = consoleErrors.filter(message => !isExpectedNetworkConsoleError(message))
  expect(unexpectedConsoleErrors, `Unexpected browser console errors: ${unexpectedConsoleErrors.join(' | ')}`).toEqual([])
  expect(pageErrors, `Browser page errors: ${pageErrors.join(' | ')}`).toEqual([])
})
