import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

const connectionError =
  'node HTTP probe failed: tls: failed to verify certificate: x509: certificate has expired or is not yet valid: ' +
  'current time 2026-08-13T00:16:06Z is before 2026-08-13T00:26:10Z'

test('node list exposes the latest connection error', async ({ page }, testInfo) => {
  await page.route('**/api/nodes?**', async route => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }

    const response = await route.fetch()
    const payload = await response.json()
    const node = payload.data?.find((item: { name?: string }) => item.name === 'demo-node-2')
    expect(node, 'The demo node fixture was not returned by /api/nodes').toBeTruthy()
    Object.assign(node, {
      status: false,
      connection_error: connectionError,
      connection_error_code: 'clock_skew',
      connection_error_at: '2026-08-13T00:16:06Z',
    })

    await route.fulfill({ response, json: payload })
  })

  await gotoRoute(page, '/nodes')
  const rows = await expectTableRows(page, 2)
  const nodeRow = rows.filter({ hasText: 'demo-node-2' }).first()
  await expect(nodeRow).toContainText('Offline')

  const errorIndicator = nodeRow.getByRole('button', { name: 'System clocks are out of sync' })
  await expect(errorIndicator).toBeVisible()
  await errorIndicator.hover()
  const popover = page.locator('.ant-popover').filter({ hasText: 'System clocks are out of sync' })
  await expect(popover).toBeVisible()
  await expect(popover).toContainText('Synchronize the host system time on both the controller and the node.')
  await expect(popover).toContainText('sudo timedatectl set-ntp true')
  await expect(popover).toContainText('The node will reconnect automatically after the clocks are synchronized.')

  await page.screenshot({
    path: testInfo.outputPath('node-connection-error.png'),
    fullPage: true,
  })
})
