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

interface SiteNavigationFrame {
  type?: string
  data?: Array<{
    name?: string
  }>
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

test('nginx log list and raw view render fabricated traffic', async ({ page }) => {
  test.setTimeout(240_000)

  await gotoRoute(page, '/nginx_log/list')
  await expectTableRows(page, 1)

  const rawResponsePromise = waitForApiResponse(page, '/api/nginx_log/page', 'POST')
  await gotoRoute(page, '/nginx_log/access')
  const rawResponse = await rawResponsePromise
  expect(rawResponse.ok()).toBe(true)
  const rawBody = await rawResponse.json()
  expect(rawBody.content?.split('\n').filter(Boolean).length).toBeGreaterThan(0)
  await expect.poll(() => page.locator('.nginx-log-line').count()).toBeGreaterThan(0)
})

test('sites list is populated and navigation cards include healthy and failing sites', async ({ page }) => {
  test.setTimeout(120_000)

  let navigationSocket: WebSocket | undefined
  let navigationSocketError: string | undefined
  let initialNavigationFrame: SiteNavigationFrame | undefined

  page.on('websocket', socket => {
    if (new URL(socket.url()).pathname !== '/api/site_navigation_ws')
      return

    navigationSocket = socket
    socket.on('socketerror', error => {
      navigationSocketError = error
    })
    socket.on('framereceived', event => {
      try {
        const payload = typeof event.payload === 'string' ? event.payload : event.payload.toString()
        const frame = JSON.parse(payload) as SiteNavigationFrame
        if (frame.type === 'initial')
          initialNavigationFrame = frame
      }
      catch {
        // A malformed initial frame will be surfaced by the assertions below.
      }
    })
  })

  await gotoRoute(page, '/sites/list')
  const siteRows = await expectTableRows(page, 2)
  await expect(siteRows.filter({ hasText: 'ojbk.me' }).first()).toBeVisible()
  await expect(siteRows.filter({ hasText: 'Prime Sponsor' }).first()).toBeVisible()

  const navigationResponsePromise = waitForApiResponse(page, '/api/site_navigation', 'GET')
  await gotoRoute(page, '/dashboard/sites')
  const navigationResponse = await navigationResponsePromise
  expect(navigationResponse.ok()).toBe(true)
  const navigationBody = await navigationResponse.json()
  expect(navigationBody.data?.length).toBeGreaterThanOrEqual(2)

  await expect.poll(() => Boolean(navigationSocket)).toBe(true)
  expect(new URL(navigationSocket?.url() ?? 'http://invalid').searchParams.get('token')).toBeTruthy()
  await expect.poll(() => initialNavigationFrame?.data?.length ?? 0).toBeGreaterThanOrEqual(2)
  expect(initialNavigationFrame?.type).toBe('initial')
  expect(navigationSocketError).toBeUndefined()
  expect(navigationSocket?.isClosed()).toBe(false)

  const healthy = page.locator('.site-card').filter({ hasText: 'ojbk.me' }).first()
  const failing = page.locator('.site-card').filter({ hasText: /Prime Sponsor|langgood\.com/ }).first()
  const failingSiteData = navigationBody.data.find((site: { name?: string, title?: string }) =>
    site.name === 'langgood.com' || site.title === 'Prime Sponsor',
  )
  expect(failingSiteData?.error_type).toBe('status_code')
  await expect(healthy).toBeVisible()
  await expect(failing).toBeVisible()
  await expect(healthy.locator('[data-testid="site-health-check-status"]')).toHaveClass(/status-online/, { timeout: 60_000 })
  await expect(healthy).toContainText('200')
  await expect(failing.locator('[data-testid="site-health-check-status"]')).toHaveClass(/status-error/, { timeout: 60_000 })
  await expect(failing.locator('[data-testid="site-health-check-error"]')).toContainText('Unexpected status code: 503')
  await expect(failing).toContainText('503')
})

test('upstream targets render and include the deliberately offline socket', async ({ page }) => {
  const socketsResponsePromise = waitForApiResponse(page, '/api/upstream/sockets', 'GET')
  await gotoRoute(page, '/upstream/sockets')

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
  await gotoRoute(page, '/certificates/list')
  await expect(page.getByText('Certificates', { exact: true }).first()).toBeVisible()
  await expectEmptyTable(page)

  await gotoRoute(page, '/namespaces')
  await expect(page.getByText('Namespaces', { exact: true }).first()).toBeVisible()
  await expectEmptyTable(page)
})
