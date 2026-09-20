import type { Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { expectTableRows, gotoRoute } from './helpers'

// The demo ships exactly two sites, and each one carries a distinct server_name
// plus distinct log paths. That is what makes a leaked editor state visible:
// "Prime Sponsor" must never render anything belonging to "ojbk.me".
const SITE_A = { row: 'ojbk.me', marker: 'ojbk.me' }
const SITE_B = { row: 'Prime Sponsor', marker: 'langgood.com' }

function isSiteDetailRequest(url: URL, name: string) {
  return decodeURIComponent(url.pathname) === `/api/sites/${name}`
}

/** Holds the detail response so the loading window becomes observable. */
async function delaySiteDetail(page: Page, name: string, delayMs: number) {
  await page.route(
    url => isSiteDetailRequest(url, name),
    async route => {
      if (route.request().method() !== 'GET') {
        await route.fallback()
        return
      }
      await new Promise(resolve => { setTimeout(resolve, delayMs) })
      await route.continue()
    },
  )
}

function waitForSiteDetail(page: Page, name: string) {
  return page.waitForResponse(response =>
    isSiteDetailRequest(new URL(response.url()), name)
    && response.request().method() === 'GET',
  )
}

/**
 * Everything the editor currently shows, including input values. The site name
 * lives in directive inputs in basic mode and in the code editor's text in
 * advanced mode, so this stays valid for either.
 */
function editorSnapshot(page: Page) {
  return page.locator('.site-edit-container').evaluate(element => {
    const values = Array.from(element.querySelectorAll('input, textarea'))
      .map(field => (field as HTMLInputElement | HTMLTextAreaElement).value)

    return [(element as HTMLElement).innerText, ...values].join('\n')
  })
}

async function openSiteFromList(page: Page, rowText: string) {
  const rows = await expectTableRows(page, 2)
  const row = rows.filter({ hasText: rowText }).first()
  await expect(row).toBeVisible()
  await row.getByRole('button').first().click()
  await expect(page.locator('.site-edit-container')).toBeVisible()
}

async function backToList(page: Page) {
  await page.locator('.ant-pro-footer-toolbar')
    .getByRole('button', { name: 'Back', exact: true })
    .click()
  await expect(page.locator('.site-edit-container')).toBeHidden()
}

test('editing another site never keeps the previous site on screen', async ({ page }) => {
  await gotoRoute(page, '/sites/list')
  await openSiteFromList(page, SITE_A.row)
  await expect.poll(() => editorSnapshot(page)).toContain(SITE_A.marker)

  // Hold the second site's response so the window between navigation and data
  // is long enough to inspect.
  await delaySiteDetail(page, SITE_B.row, 3000)

  await backToList(page)
  await openSiteFromList(page, SITE_B.row)

  // The store is shared between both visits, so this is where the previous
  // site used to stay on screen until a manual refresh.
  expect(await editorSnapshot(page)).not.toContain(SITE_A.marker)

  await expect.poll(() => editorSnapshot(page)).toContain(SITE_B.marker)
  expect(await editorSnapshot(page)).not.toContain(SITE_A.marker)
})

test('a late response for the site the operator left cannot replace the current one', async ({ page }) => {
  await gotoRoute(page, '/sites/list')

  // The first site answers only after the operator has already moved on.
  await delaySiteDetail(page, SITE_A.row, 6000)
  const lateResponse = waitForSiteDetail(page, SITE_A.row)

  await openSiteFromList(page, SITE_A.row)
  await backToList(page)
  await openSiteFromList(page, SITE_B.row)
  await expect.poll(() => editorSnapshot(page)).toContain(SITE_B.marker)

  expect((await lateResponse).ok()).toBe(true)
  await page.waitForTimeout(1000)

  const snapshot = await editorSnapshot(page)
  expect(snapshot).toContain(SITE_B.marker)
  expect(snapshot).not.toContain(SITE_A.marker)
})
