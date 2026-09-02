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

test('Nginx control mode exposes Host via SSH and switches the summary tag without saving', async ({ page }) => {
  const requests = trackApiRequests(page)

  await gotoRoute(page, '/preference')
  await page.getByRole('tab', { name: 'Nginx', exact: true }).click()

  const item = controlModeItem(page)
  await expect(item).toBeVisible()
  await expect(item.locator('.ant-tag').filter({ hasText: /^Local$/ })).toBeVisible()

  const localRadio = item.getByRole('radio', { name: 'Local / Bundled' })
  const containerRadio = item.getByRole('radio', { name: 'External Container' })
  const sshRadio = item.getByRole('radio', { name: 'Host via SSH' })
  await expect(localRadio).toBeVisible()
  await expect(containerRadio).toBeVisible()
  await expect(sshRadio).toBeVisible()
  await expect(localRadio).toBeChecked()

  await sshRadio.check()
  await expect(sshRadio).toBeChecked()
  await expect(item.locator('.ant-tag').filter({ hasText: 'Host via SSH' })).toBeVisible()
  await expect(item.locator('.ant-tag').filter({ hasText: /^Local$/ })).toHaveCount(0)

  await localRadio.check()
  await expect(localRadio).toBeChecked()
  await expect(item.locator('.ant-tag').filter({ hasText: /^Local$/ })).toBeVisible()

  // Editing the control mode is protected by a secure session. With 2FA
  // disabled on the demo account the UI guides to 2FA settings instead.
  await item.getByRole('button', { name: /Edit/ }).click()
  const guard = page.locator('.ant-modal').filter({ hasText: 'Two-factor authentication required' })
  await expect(guard).toBeVisible()
  await guard.getByRole('button', { name: 'Cancel', exact: true }).click()
  await expect(guard).toBeHidden()

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
