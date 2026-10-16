import type { Page, Request } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { gotoRoute } from './helpers'

// Issue #2023: once a save replaced the editor's config, a location added to a
// server tab that was already open stayed on screen but never reached
// build_config or the saved file. "Prime Sponsor" is the demo's basic-mode
// site; its only server has no locations yet.
const SITE = 'Prime Sponsor'
const ADDED_PATH = '= /internal/authelia/authz'

function isSiteDetail(url: URL) {
  return decodeURIComponent(url.pathname) === `/api/sites/${SITE}`
}

function locationPaths(request: Request) {
  const config = request.postDataJSON() as { servers?: { locations?: { path: string }[] | null }[] }
  return (config.servers ?? []).flatMap(server => (server.locations ?? []).map(location => location.path))
}

/**
 * Answers every save with the site as it was loaded, so the editor takes the
 * response like after a real save (a fresh config object) while the demo file
 * stays untouched.
 */
async function answerSavesWithSite(page: Page, site: unknown) {
  await page.route(url => isSiteDetail(url), async route => {
    if (route.request().method() !== 'POST') {
      await route.fallback()
      return
    }
    await route.fulfill({ json: site })
  })
}

async function saveSite(page: Page) {
  const saveResponse = page.waitForResponse(response =>
    isSiteDetail(new URL(response.url())) && response.request().method() === 'POST',
  )
  const buildRequest = page.waitForRequest(request =>
    new URL(request.url()).pathname === '/api/ngx/build_config' && request.method() === 'POST',
  )

  await page.locator('.ant-pro-footer-toolbar')
    .getByRole('button', { name: 'Save', exact: true })
    .click()

  const request = await buildRequest
  expect((await saveResponse).ok()).toBe(true)
  return request
}

test('a location added after saving reaches the built config', async ({ page }) => {
  const detailResponse = page.waitForResponse(response =>
    isSiteDetail(new URL(response.url())) && response.request().method() === 'GET',
  )
  await gotoRoute(page, `/sites/${encodeURIComponent(SITE)}`)
  const site = await (await detailResponse).json()
  await answerSavesWithSite(page, site)

  // The server tab is rendered before the save, which is what kept it bound
  // to the config object loaded back then.
  const serverPanel = page.locator('.site-edit-container .ant-tabs-content-active').first()
  await expect(serverPanel.getByRole('button', { name: 'Add Location' })).toBeVisible()

  await saveSite(page)

  await serverPanel.getByRole('button', { name: 'Add Location' }).click()
  const modal = page.locator('.ant-modal:visible').last()
  await expect(modal).toBeVisible()
  await modal.locator('input.ant-input').first().fill(ADDED_PATH)
  await modal.getByRole('button', { name: 'OK' }).click()
  await expect(modal).toBeHidden()
  await expect(serverPanel).toContainText(`Location ${ADDED_PATH}`)

  const buildRequest = await saveSite(page)
  expect(locationPaths(buildRequest)).toContain(ADDED_PATH)
})
