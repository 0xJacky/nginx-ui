import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

test('node list explains authentication upgrade progress and failure', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1600, height: 1000 })

  await page.route('**/api/self_check', async route => {
    await route.fulfill({ status: 200, contentType: 'application/json', json: [] })
  })

  await page.route('**/api/nodes?**', async route => {
    if (route.request().method() !== 'GET') {
      await route.continue()
      return
    }

    const fixture = {
      created_at: '2026-08-17T08:00:00Z',
      updated_at: '2026-08-17T09:00:00Z',
      url: 'https://node.example.test:9000',
      version: 'v2.5.9',
      status: true,
      enabled: true,
      auth_method: 'legacy_secret',
      credential_status: 'active',
      has_credential: true,
      auth_upgrade_attempt_count: 0,
    }

    const data = [
      {
        ...fixture,
        id: 9101,
        name: 'edge-pending',
        status: true,
        enabled: true,
        auth_method: 'legacy_secret',
        credential_status: 'active',
        auth_upgrade_status: 'pending',
        auth_upgrade_step: 'queued',
        auth_upgrade_attempt_count: 0,
      },
      {
        ...fixture,
        id: 9102,
        name: 'edge-upgrading',
        status: true,
        enabled: true,
        auth_method: 'legacy_secret',
        credential_status: 'active',
        auth_upgrade_status: 'in_progress',
        auth_upgrade_step: 'verify',
        auth_upgrade_attempt_count: 1,
        auth_upgrade_attempted_at: '2026-08-17T09:22:10Z',
      },
      {
        ...fixture,
        id: 9103,
        name: 'edge-waiting-target',
        status: true,
        enabled: true,
        auth_method: 'legacy_secret',
        credential_status: 'active',
        auth_upgrade_status: 'waiting_target',
        auth_upgrade_step: 'request',
        auth_upgrade_attempt_count: 1,
        auth_upgrade_attempted_at: '2026-08-17T09:18:32Z',
        auth_upgrade_next_retry_at: '2026-08-17T10:18:32Z',
        auth_upgrade_error_code: 'target_unsupported',
      },
      {
        ...fixture,
        id: 9104,
        name: 'edge-upgrade-failed',
        status: true,
        enabled: true,
        auth_method: 'legacy_secret',
        credential_status: 'active',
        auth_upgrade_status: 'failed',
        auth_upgrade_step: 'verify',
        auth_upgrade_attempt_count: 2,
        auth_upgrade_attempted_at: '2026-08-17T09:15:06Z',
        auth_upgrade_next_retry_at: '2026-08-17T10:15:06Z',
        auth_upgrade_error_code: 'invalid_confirmation',
        auth_upgrade_error: 'The target node returned an invalid upgrade confirmation.',
      },
    ]
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      json: {
        data,
        pagination: {
          total: data.length,
          per_page: 20,
          current_page: 1,
          total_pages: 1,
        },
      },
    })
  })

  await gotoRoute(page, '/nodes')
  const rows = await expectTableRows(page, 4)
  await expect(rows.filter({ hasText: 'edge-pending' })).toContainText('Upgrade pending')
  await expect(rows.filter({ hasText: 'edge-upgrading' })).toContainText('Upgrading')
  await expect(rows.filter({ hasText: 'edge-waiting-target' })).toContainText('Waiting for target')
  await expect(rows.filter({ hasText: 'edge-upgrade-failed' })).toContainText('Upgrade failed')

  await page.screenshot({
    path: testInfo.outputPath('node-auth-upgrade-overview.png'),
    fullPage: true,
  })

  const failedRow = rows.filter({ hasText: 'edge-upgrade-failed' })
  await failedRow.getByRole('button', { name: 'Authentication upgrade failed' }).click()

  const popover = page.locator('.ant-popover').filter({ hasText: 'Authentication upgrade failed' })
  await expect(popover).toBeVisible()
  await expect(popover).toContainText('The target returned an invalid upgrade confirmation.')
  await expect(popover).toContainText('Verify target confirmation')
  await expect(popover).toContainText('Retry authentication upgrade')
  await expect(popover).toContainText('invalid_confirmation')
  await popover.getByText('Technical details').click()
  await expect(popover.getByText('invalid_confirmation')).toBeVisible()

  await page.screenshot({
    path: testInfo.outputPath('node-auth-upgrade-failed.png'),
    fullPage: true,
  })
})
