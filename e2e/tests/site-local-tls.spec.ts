import { expect, test } from '@playwright/test'
import { execFile } from 'node:child_process'
import { mkdtemp, readFile, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { promisify } from 'node:util'
import { authHeaders, gotoRoute, waitForApiResponse } from './helpers'

const siteName = 'e2e-local-tls'
const fixtureDir = 'e2e-fixtures'
const certificateName = 'issue-1822.crt.conf'
const privateKeyName = 'issue-1822.key.conf'
const tlsIncludeName = 'issue-1822-tls.conf'
const execFileAsync = promisify(execFile)

async function addConfig(
  page: import('@playwright/test').Page,
  headers: Record<string, string>,
  name: string,
  content: string,
) {
  const response = await page.request.post('/api/configs', {
    headers,
    data: {
      name,
      base_dir: fixtureDir,
      content,
      overwrite: true,
    },
  })
  expect(response.ok(), `Failed to create ${name}: ${await response.text()}`).toBe(true)
}

async function cleanUp(page: import('@playwright/test').Page, headers: Record<string, string>) {
  await page.request.post(`/api/sites/${siteName}/disable`, { headers })
  await page.request.delete(`/api/sites/${siteName}`, { headers })
  await page.request.post('/api/config_delete', {
    headers,
    data: {
      base_path: '',
      name: fixtureDir,
    },
  })
}

test('ordinary mode saves a TLS site whose local certificate paths come from an include', async ({ page }) => {
  await gotoRoute(page, '/')
  const headers = await authHeaders(page)
  const localFixtureDir = await mkdtemp(join(tmpdir(), 'nginxui-e2e-tls-'))
  const certificatePath = join(localFixtureDir, 'certificate.pem')
  const privateKeyPath = join(localFixtureDir, 'private-key.pem')

  await cleanUp(page, headers)

  try {
    await execFileAsync('openssl', [
      'req',
      '-x509',
      '-newkey', 'rsa:2048',
      '-nodes',
      '-keyout', privateKeyPath,
      '-out', certificatePath,
      '-days', '1',
      '-subj', '/CN=e2e-local-tls.example.com',
    ])
    const certificate = await readFile(certificatePath, 'utf8')
    const privateKey = await readFile(privateKeyPath, 'utf8')

    await addConfig(page, headers, certificateName, certificate)
    await addConfig(page, headers, privateKeyName, privateKey)
    await addConfig(page, headers, tlsIncludeName, [
      `ssl_certificate /etc/nginx/${fixtureDir}/${certificateName};`,
      `ssl_certificate_key /etc/nginx/${fixtureDir}/${privateKeyName};`,
      '',
    ].join('\n'))

    const siteContent = [
      'server {',
      '    listen 9443 ssl;',
      '    server_name e2e-local-tls.example.com;',
      `    include /etc/nginx/${fixtureDir}/${tlsIncludeName};`,
      '    return 204;',
      '}',
      '',
    ].join('\n')

    const createResponse = await page.request.post(`/api/sites/${siteName}`, {
      headers,
      data: {
        name: siteName,
        content: siteContent,
        overwrite: true,
      },
    })
    expect(createResponse.ok()).toBe(true)

    const enableResponse = await page.request.post(`/api/sites/${siteName}/enable`, { headers })
    expect(enableResponse.ok(), `Failed to enable the TLS fixture: ${await enableResponse.text()}`).toBe(true)

    await gotoRoute(page, `/sites/${siteName}`)
    await expect(page.getByRole('button', { name: 'Save', exact: true })).toBeVisible()

    const savePromise = waitForApiResponse(page, `/api/sites/${siteName}`, 'POST')
    await page.getByRole('button', { name: 'Save', exact: true }).click()
    const saveResponse = await savePromise
    expect(saveResponse.ok(), `Ordinary-mode save failed: ${await saveResponse.text()}`).toBe(true)

    const getResponse = await page.request.get(`/api/sites/${siteName}`, { headers })
    expect(getResponse.ok()).toBe(true)
    const site = await getResponse.json() as { config: string, status: string }
    expect(site.config).toContain(`include /etc/nginx/${fixtureDir}/${tlsIncludeName};`)
    expect(site.config).not.toContain('ssl_certificate ')
    expect(site.status).toBe('enabled')
  }
  finally {
    await cleanUp(page, headers)
    await rm(localFixtureDir, { recursive: true, force: true })
  }
})
