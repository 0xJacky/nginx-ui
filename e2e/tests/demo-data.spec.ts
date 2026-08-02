import type { WebSocket } from '@playwright/test'
import { expect, test } from '@playwright/test'
import {
  expectEmptyTable,
  expectPositiveText,
  expectTableRows,
  gotoRoute,
  tableRows,
  waitForApiResponse,
} from './helpers'

interface AnalyticsFrame {
  cpu?: {
    system?: number
    user?: number
  }
  memory?: {
    pressure?: number
  }
  network?: {
    bytesRecv?: number
    bytesSent?: number
  }
}

test('server dashboard renders non-zero live gauges over the analytics WebSocket', async ({ page }) => {
  test.setTimeout(120_000)

  let analyticsSocket: WebSocket | undefined
  const analyticsFrames: AnalyticsFrame[] = []

  page.on('websocket', socket => {
    if (new URL(socket.url()).pathname !== '/api/analytic')
      return

    analyticsSocket = socket
    socket.on('framereceived', event => {
      try {
        const payload = typeof event.payload === 'string' ? event.payload : event.payload.toString()
        analyticsFrames.push(JSON.parse(payload) as AnalyticsFrame)
      }
      catch {
        // A malformed frame will be surfaced by the missing metric assertions below.
      }
    })
  })

  const initResponsePromise = waitForApiResponse(page, '/api/analytic/init', 'GET')
  await gotoRoute(page, '/dashboard/server')

  const initResponse = await initResponsePromise
  expect(initResponse.ok()).toBe(true)
  const initial = await initResponse.json()
  expect(Number(initial.memory?.pressure)).toBeGreaterThan(0)
  expect(Number(initial.network?.init?.bytesRecv)).toBeGreaterThan(0)
  expect(Number(initial.network?.init?.bytesSent)).toBeGreaterThan(0)

  await expect.poll(() => Boolean(analyticsSocket)).toBe(true)

  await page.evaluate(async () => {
    await Promise.all(Array.from({ length: 40 }, (_, index) =>
      fetch(`/healthz?e2e-network-sample=${index}`, { cache: 'no-store' }),
    ))
  })

  await expect.poll(() => analyticsFrames.some(frame =>
    Number(frame.cpu?.user ?? 0) + Number(frame.cpu?.system ?? 0) > 0
    && Number(frame.memory?.pressure ?? 0) > 0
    && Number(frame.network?.bytesRecv ?? 0) > 0
    && Number(frame.network?.bytesSent ?? 0) > 0,
  ), { timeout: 60_000 }).toBe(true)

  const memoryCard = page.locator('.ant-card').filter({
    has: page.getByText('Memory and Storage', { exact: true }),
  }).first()
  await expect(memoryCard).toBeVisible()
  await expectPositiveText(memoryCard.locator('.apexcharts-datalabel-value').first())

  const cpuCard = page.locator('.ant-card').filter({
    has: page.getByText('CPU Status', { exact: true }),
  }).first()
  await expectPositiveText(cpuCard.locator('.ant-statistic-content-value').first(), 60_000)

  const networkCard = page.locator('.ant-card').filter({
    has: page.getByText('Network', { exact: true }),
  }).first()
  const networkValues = networkCard.locator('.ant-statistic-content-value')
  await expect(networkValues).toHaveCount(2)
  await expectPositiveText(networkValues.nth(0), 60_000)
  await expectPositiveText(networkValues.nth(1), 60_000)
})

test('nginx log list, raw view, structured view, and geographic dashboards render fabricated traffic', async ({ page }) => {
  test.setTimeout(240_000)

  await gotoRoute(page, '/nginx_log/list')
  await expectTableRows(page, 1)

  const rawResponsePromise = waitForApiResponse(page, '/api/nginx_log/page', 'POST')
  await gotoRoute(page, '/nginx_log/access?view=raw')
  const rawResponse = await rawResponsePromise
  expect(rawResponse.ok()).toBe(true)
  const rawBody = await rawResponse.json()
  expect(rawBody.content?.split('\n').filter(Boolean).length).toBeGreaterThan(0)
  await expect.poll(() => page.locator('.nginx-log-line').count()).toBeGreaterThan(0)

  const searchResponsePromise = waitForApiResponse(page, '/api/nginx_log/search', 'POST', 180_000)
  await gotoRoute(page, '/nginx_log/access?view=structured')
  const searchResponse = await searchResponsePromise
  expect(searchResponse.ok()).toBe(true)
  const searchBody = await searchResponse.json()
  expect(searchBody.entries?.length).toBeGreaterThan(0)
  await expectTableRows(page.locator('.log-table-container'), 1)

  const dashboardResponsePromise = waitForApiResponse(page, '/api/nginx_log/dashboard', 'POST', 180_000)
  const worldResponsePromise = waitForApiResponse(page, '/api/nginx_log/geo/world', 'POST', 180_000)
  const chinaResponsePromise = waitForApiResponse(page, '/api/nginx_log/geo/china', 'POST', 180_000)
  await gotoRoute(page, '/nginx_log/access?view=dashboard')

  const [dashboardResponse, worldResponse, chinaResponse] = await Promise.all([
    dashboardResponsePromise,
    worldResponsePromise,
    chinaResponsePromise,
  ])
  expect(dashboardResponse.ok()).toBe(true)
  expect(worldResponse.ok()).toBe(true)
  expect(chinaResponse.ok()).toBe(true)

  const dashboardBody = await dashboardResponse.json()
  const worldBody = await worldResponse.json()
  const chinaBody = await chinaResponse.json()
  expect(Number(dashboardBody.summary?.total_pv)).toBeGreaterThan(0)
  expect(worldBody.data?.length).toBeGreaterThan(0)
  expect(chinaBody.data?.length).toBeGreaterThan(0)

  await expectPositiveText(page.locator('.ant-statistic').filter({
    has: page.getByText('Total PV', { exact: true }),
  }).locator('.ant-statistic-content-value'))

  const worldMap = page.locator('.world-map-container')
  await expect(worldMap).toBeVisible()
  await expect(worldMap.locator('canvas')).toBeVisible()
  await expectTableRows(worldMap, 1)

  const chinaMap = page.locator('.china-map-container')
  await expect(chinaMap).toBeAttached()
  await expect.poll(() => tableRows(chinaMap).count()).toBeGreaterThan(0)
})

test('sites list is populated and navigation cards include healthy and failing sites', async ({ page }) => {
  test.setTimeout(120_000)

  await gotoRoute(page, '/sites')
  const siteRows = await expectTableRows(page, 2)
  await expect(siteRows.filter({ hasText: 'ojbk.me' }).first()).toBeVisible()
  await expect(siteRows.filter({ hasText: 'Prime Sponsor' }).first()).toBeVisible()

  const navigationResponsePromise = waitForApiResponse(page, '/api/site_navigation', 'GET')
  await gotoRoute(page, '/dashboard/sites')
  const navigationResponse = await navigationResponsePromise
  expect(navigationResponse.ok()).toBe(true)
  const navigationBody = await navigationResponse.json()
  expect(navigationBody.data?.length).toBeGreaterThanOrEqual(2)

  const healthy = page.locator('.site-card').filter({ hasText: 'ojbk.me' }).first()
  const failing = page.locator('.site-card').filter({ hasText: /Prime Sponsor|langgood\.com/ }).first()
  await expect(healthy).toBeVisible()
  await expect(failing).toBeVisible()
  await expect(healthy.locator('[data-testid="site-health-check-status"]')).toHaveClass(/status-online/, { timeout: 60_000 })
  await expect(healthy).toContainText('200')
  await expect(failing.locator('[data-testid="site-health-check-status"]')).toHaveClass(/status-error/, { timeout: 60_000 })
  await expect(failing).toContainText('503')
})

test('upstream targets render and include the deliberately offline socket', async ({ page }) => {
  const socketsResponsePromise = waitForApiResponse(page, '/api/upstream/sockets', 'GET')
  await gotoRoute(page, '/upstream')

  const socketsResponse = await socketsResponsePromise
  expect(socketsResponse.ok()).toBe(true)
  const socketsBody = await socketsResponse.json()
  expect(socketsBody.data?.length).toBeGreaterThan(0)

  const rows = await expectTableRows(page, 1)
  const offline = rows.filter({ hasText: '127.0.0.1:9005' }).first()
  await expect(offline).toBeVisible()
  await expect(offline).toContainText('Offline', { timeout: 60_000 })
})

test('DNS credentials, domains, and fabricated records render their seeded counts', async ({ page }) => {
  await gotoRoute(page, '/dns/credentials')
  await expect.poll(() => tableRows(page).count()).toBe(3)
  await expect(page.getByText('Cloudflare (demo)', { exact: true })).toBeVisible()
  await expect(page.getByText('Aliyun DNS (demo)', { exact: true })).toBeVisible()
  await expect(page.getByText('Tencent Cloud DNS (demo)', { exact: true })).toBeVisible()

  await gotoRoute(page, '/dns/domains')
  const domainRows = tableRows(page)
  await expect.poll(() => domainRows.count()).toBe(4)

  const recordsResponsePromise = page.waitForResponse(response =>
    /\/api\/dns\/domains\/\d+\/records$/.test(new URL(response.url()).pathname)
    && response.request().method() === 'GET',
  )
  await domainRows.first().getByRole('button', { name: 'Manage Records', exact: true }).click()

  const recordsResponse = await recordsResponsePromise
  expect(recordsResponse.ok()).toBe(true)
  const recordsBody = await recordsResponse.json()
  expect(recordsBody.data).toHaveLength(12)
  await expect(page).toHaveURL(/\/dns\/domains\/\d+\/records$/)
  await expect.poll(() => tableRows(page.locator('.dns-record-table')).count()).toBeGreaterThan(0)
})

test('config and node screens render their real bundled fixtures', async ({ page }) => {
  await gotoRoute(page, '/config')
  const configRows = await expectTableRows(page, 1)
  await expect(configRows.filter({ hasText: 'conf.d' }).first()).toBeVisible()

  await gotoRoute(page, '/nodes')
  const nodeRows = await expectTableRows(page, 2)
  await expect(nodeRows.filter({ hasText: 'demo-node-2' }).first()).toBeVisible()
  await expect(nodeRows.filter({ hasText: 'demo-node-3' }).first()).toBeVisible()
})

test('certificate and namespace screens render clean empty states', async ({ page }) => {
  await gotoRoute(page, '/certificates')
  await expect(page.getByText('Certificates', { exact: true }).first()).toBeVisible()
  await expectEmptyTable(page)

  await gotoRoute(page, '/namespaces')
  await expect(page.getByText('Namespaces', { exact: true }).first()).toBeVisible()
  await expectEmptyTable(page)
})
