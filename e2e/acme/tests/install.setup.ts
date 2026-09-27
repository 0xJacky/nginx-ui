import { expect, test as setup } from '@playwright/test'
import { randomBytes } from 'node:crypto'
import { existsSync, readFileSync, writeFileSync } from 'node:fs'
import { readInstallSecret } from '../lib/docker'
import { authStateFile, credentialsFile } from '../lib/env'

interface Credentials {
  email: string
  username: string
  password: string
}

function readCredentials(): Credentials | undefined {
  if (!existsSync(credentialsFile))
    return undefined
  return JSON.parse(readFileSync(credentialsFile, 'utf8')) as Credentials
}

setup('install nginx-ui on first run and log in', async ({ page, request }) => {
  const lock = await (await request.get('/api/install')).json() as { lock: boolean, timeout: boolean }
  expect(lock.timeout, 'the install window expired; restart the stack with scripts/up.sh').toBe(false)

  let credentials = readCredentials()

  if (!lock.lock) {
    // A fresh password per stack, kept only in the stack's data directory.
    // The install form caps passwords at 20 characters.
    credentials = {
      // Must equal NGINX_UI_CERT_EMAIL in compose.yml. Install stores this
      // address as the cert email, but the default ACME account is only
      // registered at boot (cert.InitRegister), so a different address here
      // leaves issuance failing with "get default user error: record not
      // found" until nginx-ui restarts.
      email: 'acme@e2e.test',
      username: 'admin',
      password: randomBytes(12).toString('base64url'),
    }
    writeFileSync(credentialsFile, JSON.stringify(credentials), { mode: 0o600 })

    await page.goto('/#/install')
    await page.getByPlaceholder('Install Secret (*)').fill(readInstallSecret())
    await page.getByRole('button', { name: 'Verify Secret' }).click()

    // The self check runs after the secret is accepted; Next only appears
    // when it reports no errors.
    await page.getByRole('button', { name: 'Next', exact: true }).click({ timeout: 60_000 })

    await page.getByPlaceholder('Email (*)').fill(credentials.email)
    await page.getByPlaceholder('Username (*)').fill(credentials.username)
    await page.getByPlaceholder('Password (*)').fill(credentials.password)

    const installResponse = page.waitForResponse(response =>
      /\/api\/(?:setup\/)?install$/.test(new URL(response.url()).pathname) && response.request().method() === 'POST')
    await page.getByRole('button', { name: 'Install', exact: true }).click()
    expect((await installResponse).ok()).toBe(true)
    await expect(page).toHaveURL(/\/login(?:\?|$)/)
  }

  expect(credentials, `nginx-ui is already installed but ${credentialsFile} is missing; restart the stack with scripts/up.sh`).toBeDefined()

  // POST /api/login needs the frontend's encryption, so log in through the UI.
  await page.goto('/#/login')
  await page.getByPlaceholder('Username').fill(credentials!.username)
  await page.getByPlaceholder('Password').fill(credentials!.password)
  const loginResponse = page.waitForResponse(response =>
    new URL(response.url()).pathname === '/api/login' && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Login', exact: true }).click()
  expect((await loginResponse).ok()).toBe(true)
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
  await expect.poll(() => page.evaluate(() => {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw).token : ''
  })).not.toBe('')

  await page.context().storageState({ path: authStateFile })
})
