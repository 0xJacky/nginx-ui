import { expect, test } from '@playwright/test'
import { authHeaders, expectDemoRefusal, gotoRoute } from './helpers'

test('GET /api/pty returns the typed demo refusal before any upgrade', async ({ page }) => {
  await gotoRoute(page, '/dashboard/server')

  const response = await page.request.get('/api/pty', {
    headers: await authHeaders(page),
  })

  await expectDemoRefusal(response)
  expect(response.headers().upgrade).toBeUndefined()
})

test('terminal commands run in the browser-local simulated shell without a PTY WebSocket', async ({ page }) => {
  const websocketUrls: string[] = []
  page.on('websocket', socket => websocketUrls.push(socket.url()))

  await gotoRoute(page, '/terminal')
  await expect(page.getByText('This is a simulated terminal running entirely in your browser.', { exact: false })).toBeVisible()

  const terminalInput = page.locator('.xterm-helper-textarea').first()
  const terminalRows = page.locator('.xterm-rows').first()
  await expect(terminalInput).toBeAttached()
  await terminalInput.focus()

  await page.keyboard.type('help')
  await page.keyboard.press('Enter')
  await expect(terminalRows).toContainText('Available commands:')
  await expect(terminalRows).toContainText('nginx -t')

  await page.keyboard.type('nginx -t')
  await page.keyboard.press('Enter')
  await expect(terminalRows).toContainText('syntax is ok')
  await expect(terminalRows).toContainText('test is successful')

  await page.keyboard.type('not-a-demo-command')
  await page.keyboard.press('Enter')
  await expect(terminalRows).toContainText('not-a-demo-command: command not found in the demo shell')

  expect(websocketUrls.filter(url => new URL(url).pathname === '/api/pty')).toEqual([])
})

test('Terminal remains reachable while Preference hides its Terminal tab in demo mode', async ({ page }) => {
  await gotoRoute(page, '/terminal')
  await expect(page.locator('.xterm-screen').first()).toBeVisible()

  await gotoRoute(page, '/preference')
  await expect(page.getByRole('tab', { name: 'Server', exact: true })).toBeVisible()
  await expect(page.getByRole('tab', { name: 'Terminal', exact: true })).toHaveCount(0)
})

test('destructive settings, password, backup, and notifier routes return code 40300', async ({ page }) => {
  await gotoRoute(page, '/dashboard/server')
  const headers = {
    ...await authHeaders(page),
    'Content-Type': 'application/json',
  }

  const [settingsResponse, passwordResponse, backupResponse, notifierResponse] = await Promise.all([
    page.request.post('/api/settings', { headers, data: '{' }),
    page.request.post('/api/user/password', { headers, data: '{' }),
    page.request.get('/api/backup', { headers }),
    page.request.post('/api/external_notifies/test', {
      headers,
      data: {
        type: 'wecom',
        language: 'en',
        config: { webhook_url: 'http://127.0.0.1/internal' },
      },
    }),
  ])

  await expectDemoRefusal(settingsResponse)
  await expectDemoRefusal(passwordResponse)
  await expectDemoRefusal(backupResponse)
  await expectDemoRefusal(notifierResponse)
})
