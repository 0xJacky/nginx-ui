import type { Page, WebSocket } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { routeUrl } from './helpers'

// Logging out deletes the session's token server-side, so this spec signs in
// with its own session instead of the shared storage state the other specs
// depend on.
test.use({ storageState: { cookies: [], origins: [] } })

// Sockets owned by Pinia stores. A store is set up once per tab, so these are
// the connections that have to survive a logout and login without a reload.
const STORE_SOCKETS = [
  '/api/upstream/availability_ws',
  '/api/analytic/nodes',
  '/api/events',
] as const

interface TrackedSocket {
  pathname: string
  socket: WebSocket
  framesReceived: number
}

function trackSockets(page: Page) {
  const sockets: TrackedSocket[] = []

  page.on('websocket', socket => {
    const tracked: TrackedSocket = {
      pathname: new URL(socket.url()).pathname,
      socket,
      framesReceived: 0,
    }
    socket.on('framereceived', () => {
      tracked.framesReceived++
    })
    sockets.push(tracked)
  })

  return sockets
}

async function loginThroughUi(page: Page) {
  await page.getByPlaceholder('Username').fill(process.env.E2E_USERNAME ?? 'admin')
  await page.getByPlaceholder('Password').fill(process.env.E2E_PASSWORD ?? 'admin')

  const loginResponse = page.waitForResponse(response =>
    new URL(response.url()).pathname === '/api/login' && response.request().method() === 'POST',
  )
  await page.getByRole('button', { name: 'Login', exact: true }).click()

  expect((await loginResponse).ok()).toBe(true)
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
}

// Navigate inside the SPA: a page.goto() could reload the document and hide
// exactly the bug this spec guards against.
async function openRouteInApp(page: Page, route: string) {
  await page.evaluate(hash => {
    window.location.hash = hash
  }, routeUrl(route).slice(1))
  await expect.poll(() => new URL(page.url()).hash.split('?')[0]).toBe(`#${route}`)
}

async function expectDeliveringSockets(sockets: TrackedSocket[]) {
  for (const pathname of STORE_SOCKETS) {
    await expect.poll(
      () => sockets.some(tracked => tracked.pathname === pathname && tracked.framesReceived > 0),
      { message: `${pathname} should connect and deliver data` },
    ).toBe(true)
  }
}

test('store-owned sockets follow the session across logout and login', async ({ page }) => {
  const sockets = trackSockets(page)

  await page.goto(routeUrl('/login'), { waitUntil: 'domcontentloaded' })
  await loginThroughUi(page)
  await openRouteInApp(page, '/upstream')
  await expectDeliveringSockets(sockets)

  const firstSession = sockets.filter(tracked =>
    (STORE_SOCKETS as readonly string[]).includes(tracked.pathname),
  )

  // Marks this document so the second half can prove no reload happened.
  await page.evaluate(() => {
    (window as unknown as { __sameDocument?: boolean }).__sameDocument = true
  })

  const logoutResponse = page.waitForResponse(response =>
    new URL(response.url()).pathname === '/api/logout' && response.request().method() === 'DELETE',
  )
  await page.locator('.header a').filter({ has: page.locator('.anticon-logout') }).click()
  expect((await logoutResponse).ok()).toBe(true)
  await expect(page).toHaveURL(/\/login(?:\?|$)/)

  // Nothing may keep streaming once the session is gone. Before the fix the
  // event bus stayed connected on the login page.
  await expect.poll(
    () => firstSession.filter(tracked => !tracked.socket.isClosed()).map(tracked => tracked.pathname),
    { message: 'sockets from the first session should be closed after logout' },
  ).toEqual([])

  const secondSessionStart = sockets.length
  await loginThroughUi(page)
  await openRouteInApp(page, '/upstream')

  expect(
    await page.evaluate(() => (window as unknown as { __sameDocument?: boolean }).__sameDocument),
    'logging back in must not reload the page, or the stores would be recreated',
  ).toBe(true)

  // Before the fix the node socket never reopened after logging back in, and
  // the other store sockets redialled with the previous session's credential.
  await expectDeliveringSockets(sockets.slice(secondSessionStart))
})
