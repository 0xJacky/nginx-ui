import type { Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute } from './helpers'

// The public demo runs with 2FA disabled for the admin account, so every
// protected nginx control flow stops at the "Two-factor authentication
// required" guard before it can reach /api/host/* or POST /api/settings.
// These tests therefore only assert DOM state and that no mutating or
// host-setup request is ever issued.

function trackApiRequests(page: Page) {
  const requests: string[] = []
  page.on('request', request => {
    const url = new URL(request.url())
    if (url.pathname.startsWith('/api/'))
      requests.push(`${request.method()} ${url.pathname}`)
  })
  return requests
}

function expectReadOnlyTraffic(requests: string[]) {
  expect(requests.filter(entry => entry.includes('/api/host/'))).toEqual([])
  expect(requests.filter(entry => !entry.startsWith('GET ') && entry.endsWith('/api/settings'))).toEqual([])
}

function controlModeItem(page: Page) {
  return page.locator('.ant-form-item').filter({ has: page.getByText('Nginx Control Mode', { exact: true }) }).first()
}

test('Nginx control mode stays read-only until Edit passes the 2FA guard', async ({ page }) => {
  const requests = trackApiRequests(page)

  await gotoRoute(page, '/preference')
  await page.getByRole('tab', { name: 'Nginx', exact: true }).click()

  const item = controlModeItem(page)
  await expect(item).toBeVisible()
  await expect(item.locator('.ant-tag').filter({ hasText: /^Local$/ })).toBeVisible()
  const editButton = item.getByRole('button', { name: /Edit/ })
  await expect(editButton).toBeVisible()

  // The mode radios only mount in edit mode, so the read-only view cannot
  // mutate the settings store without going through the guarded Edit flow.
  await expect(item.locator('.ant-radio-group')).toHaveCount(0)
  await expect(item.getByRole('radio')).toHaveCount(0)

  // Editing the control mode is protected by a secure session. With 2FA
  // disabled on the demo account the UI guides to 2FA settings instead.
  await editButton.click()
  const guard = page.locator('.ant-modal').filter({ hasText: 'Two-factor authentication required' })
  await expect(guard).toBeVisible()
  await guard.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(guard).toBeHidden()

  // The guard cancelled the edit, so the view stays read-only.
  await expect(item.locator('.ant-tag').filter({ hasText: /^Local$/ })).toBeVisible()
  await expect(item.getByRole('radio')).toHaveCount(0)

  expectReadOnlyTraffic(requests)
})

test('Host SSH setup wizard route stops at the 2FA guard without calling api/host', async ({ page }) => {
  const requests = trackApiRequests(page)

  await gotoRoute(page, '/preference/nginx-host-setup')

  const wizardPage = page.locator('.host-setup-page')
  await expect(wizardPage.getByRole('button', { name: /Back to Nginx settings/ })).toBeVisible()

  const result = wizardPage.locator('.ant-result-403')
  await expect(result).toBeVisible()
  await expect(result).toContainText('Two-factor authentication required')
  await expect(result).toContainText('Enable two-factor authentication before changing Nginx control settings.')
  await expect(result.getByRole('button', { name: '2FA Settings', exact: true })).toBeVisible()

  // The stepper only mounts once a secure session is established.
  await expect(wizardPage.locator('.ant-steps')).toHaveCount(0)

  await result.getByRole('button', { name: 'Back', exact: true }).click()
  await expect.poll(() => new URL(page.url()).hash).toBe('#/preference?tab=nginx')
  await expect(page.getByRole('tab', { name: 'Nginx', exact: true })).toHaveAttribute('aria-selected', 'true')

  expectReadOnlyTraffic(requests)
})
