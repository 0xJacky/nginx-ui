import { expect, test as setup } from '@playwright/test'
import { mkdir } from 'node:fs/promises'
import { dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { routeUrl } from './helpers'

const authState = fileURLToPath(new URL('../playwright/.auth/admin.json', import.meta.url))

setup('authenticate through the demo login UI', async ({ page }) => {
  await page.goto(routeUrl('/login'), { waitUntil: 'domcontentloaded' })

  await page.getByPlaceholder('Username').fill(process.env.E2E_USERNAME ?? 'admin')
  await page.getByPlaceholder('Password').fill(process.env.E2E_PASSWORD ?? 'admin')

  const loginResponsePromise = page.waitForResponse(response =>
    new URL(response.url()).pathname === '/api/login' && response.request().method() === 'POST',
  )

  await page.getByRole('button', { name: 'Login', exact: true }).click()

  const loginResponse = await loginResponsePromise
  expect(loginResponse.ok()).toBe(true)
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
  await expect.poll(() => page.evaluate(() => {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw).token : ''
  })).not.toBe('')

  const cookies = await page.context().cookies()
  expect(cookies.some(cookie => cookie.name === '_nginx_ui_secure_session' && cookie.httpOnly)).toBe(true)

  await mkdir(dirname(authState), { recursive: true })
  await page.context().storageState({ path: authState })
})
