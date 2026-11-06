import { expect, test } from '@playwright/test'
import { authHeaders, gotoRoute, isDemoTarget } from './helpers'

// The js_* and perl* directives, and the dynamic modules providing them, load
// code into the Nginx worker. Saving them needs a verified two-factor session
// from an interactive administrator.
//
// The public demo never offers that hatch at all: it leaves local config CRUD
// open to visitors precisely because a config write cannot execute code there,
// and its password is published. So the demo gets the plain denylist
// rejection, 40017, whatever a request asks for. A writable container behaves
// like a real deployment, where the seeded administrator has no second factor
// and is told to enrol one (40023).
//
// Codes come from internal/config/errors.go.
const RESTRICTED_CONTENT = 'server {\n    listen 8443;\n    js_import main from conf.d/main.js;\n}\n'
const CONFIG_NAME = 'e2e-restricted-directives.conf'

interface CosyErrorBody {
  scope?: string
  code?: number
  params?: string[]
}

test('a configuration loading njs is refused, and the opt-in flag alone changes nothing', async ({ page, request }) => {
  await gotoRoute(page, '/config')
  const headers = await authHeaders(page)
  const demo = await isDemoTarget(request)
  const expectedCode = demo ? 40017 : 40023

  for (const payload of [
    { name: CONFIG_NAME, content: RESTRICTED_CONTENT },
    // Setting the opt-in by hand must change nothing: the server honours it
    // only for a verified two-factor session, and never on the demo.
    { name: CONFIG_NAME, content: RESTRICTED_CONTENT, allow_restricted_directives: true },
  ]) {
    const response = await request.post('/api/configs', { headers, data: payload })

    expect(response.status(), await response.text()).toBe(500)
    const body = await response.json() as CosyErrorBody
    expect(body.scope).toBe('config')
    expect(body.code).toBe(expectedCode)
    expect(body.params?.[0]).toBe('js_import')
  }

  // Nothing may reach the disk, whichever attempt it was.
  const saved = await request.get('/api/config', { headers, params: { path: CONFIG_NAME } })
  expect(saved.ok(), `${CONFIG_NAME} must not exist: ${await saved.text()}`).toBe(false)
})

test('an ordinary configuration still saves, so the gate only catches the restricted directives', async ({ page, request }) => {
  await gotoRoute(page, '/config')
  const headers = await authHeaders(page)
  const name = 'e2e-restricted-directives-ordinary.conf'

  const response = await request.post('/api/configs', {
    headers,
    data: { name, content: 'server {\n    listen 8444;\n}\n', overwrite: true },
  })
  expect(response.ok(), await response.text()).toBe(true)

  await request.post('/api/config_delete', { headers, data: { base_path: '', name } })
})

// The dialog is built from antdv-next's confirm modal and declares no colours
// of its own, so both themes have to render it readably.
for (const theme of ['light', 'dark'] as const) {
  test(`the config editor explains the refusal in a dialog and keeps the content (${theme})`, async ({ page, request }, testInfo) => {
    test.skip(await isDemoTarget(request), 'the demo never offers the step-up, so it shows the plain rejection instead of this dialog')

    await page.addInitScript(({ theme }) => {
      const raw = localStorage.getItem('settings')
      const settings = raw ? JSON.parse(raw) as Record<string, unknown> : {}
      localStorage.setItem('settings', JSON.stringify({ ...settings, theme, preference_theme: theme }))
    }, { theme })

    await gotoRoute(page, '/config/add')

    const nameInput = page.getByRole('main').locator('input').first()
    await expect(nameInput).toBeVisible()
    await nameInput.fill(CONFIG_NAME)

    const editor = page.locator('.ace_editor').first()
    await expect(editor).toBeVisible()
    await editor.click()
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+a' : 'Control+a')
    await page.keyboard.type(RESTRICTED_CONTENT)
    await expect(editor).toContainText('js_import')

    await page.getByRole('button', { name: 'Save', exact: true }).first().click()

    // The dialog has to say what is missing instead of only repeating the
    // directive, so the administrator knows the save is recoverable.
    const dialog = page.locator('.ant-modal-container:visible').last()
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText('Two-factor authentication required')
    await expect(dialog).toContainText('js_import')
    await testInfo.attach(`restricted-directives-dialog-${theme}`, {
      body: await dialog.screenshot(),
      contentType: 'image/png',
    })

    await page.keyboard.press('Escape')
    await expect(page.locator('.ant-modal-container:visible')).toHaveCount(0)

    // A refused save must not discard what the administrator typed.
    await expect(editor).toContainText('js_import')
  })
}
