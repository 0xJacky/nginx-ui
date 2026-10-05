import { expect, test } from '@playwright/test'

test('HTTP login keeps the session-binding cookie usable after HTTPS was disabled', async ({ page }) => {
  await page.goto('/')
  const baseURL = new URL(page.url()).origin
  const cookies = await page.context().cookies(baseURL)
  const sessionCookie = cookies.find(cookie => cookie.name === '_nginx_ui_secure_session_http')

  expect(sessionCookie, 'HTTP mode must use its protocol-specific session cookie').toBeDefined()
  expect(sessionCookie?.httpOnly).toBe(true)
  expect(sessionCookie?.secure).toBe(false)

  const token = await page.evaluate(() => {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw).token as string : ''
  })
  const response = await page.request.post('/api/token/short', {
    headers: { Authorization: token },
  })
  expect(response.ok(), await response.text()).toBe(true)
})
