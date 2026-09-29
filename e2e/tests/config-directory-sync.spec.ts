import type { APIRequestContext, Page } from '@playwright/test'
import { expect, test } from '@playwright/test'
import { execFileSync } from 'node:child_process'
import { authHeaders, gotoRoute } from './helpers'

// Runs against the demo image, where local config CRUD stays open by design. The
// peer nodes of the demo image share the primary's nginx directory, so "already
// exists on the node" is the natural state of every deployed file.
//
// E2E_CONTAINER names the container so the spec can drop files nginx-ui itself
// refuses to create (notes.txt, a binary) next to the certificates.
const container = process.env.E2E_CONTAINER

const sslPem = '-----BEGIN CERTIFICATE-----\nMIIBe2etest\n-----END CERTIFICATE-----\n'
const sslKey = '-----BEGIN PRIVATE KEY-----\nMIIBe2etest\n-----END PRIVATE KEY-----\n'

interface SyncResult {
  node: string
  success: boolean
  error?: string
  skipped_existing?: number
  skipped_paths?: string[]
}

interface SyncSummary {
  total: number
  succeeded: number
  failed: number
  results: SyncResult[]
  skipped: { path: string, reason: string }[]
}

async function peerNodeIds(request: APIRequestContext, headers: Record<string, string>) {
  const response = await request.get('/api/nodes?page=1&page_size=50', { headers })
  expect(response.ok()).toBe(true)
  const body = await response.json() as { data: { id: number, name: string }[] }
  const ids = body.data.map(node => node.id)
  expect(ids.length, 'the demo image seeds peer nodes').toBeGreaterThan(0)

  return ids
}

async function createConfig(
  request: APIRequestContext,
  headers: Record<string, string>,
  baseDir: string,
  name: string,
  content: string,
) {
  const response = await request.post('/api/configs', {
    headers,
    data: { name, base_dir: baseDir, content, overwrite: true },
  })
  expect(response.ok(), `create ${baseDir}/${name}: ${await response.text()}`).toBe(true)
}

async function syncDirectory(
  request: APIRequestContext,
  headers: Record<string, string>,
  dir: string,
  nodes: number[],
  overwrite: boolean,
) {
  const response = await request.post('/api/config_sync_directory', {
    headers,
    data: { dir, sync_node_ids: nodes, sync_overwrite: overwrite },
  })
  expect(response.ok(), await response.text()).toBe(true)

  return await response.json() as SyncSummary
}

async function openAuthenticated(page: Page) {
  await gotoRoute(page, '/config')

  return await authHeaders(page)
}

test.describe.serial('config directory deployment', () => {
  test('certificates are accepted and deployed to the nodes', async ({ page }) => {
    const headers = await openAuthenticated(page)
    const nodes = await peerNodeIds(page.request, headers)

    await createConfig(page.request, headers, 'e2e-ssl', 'site.pem', sslPem)
    await createConfig(page.request, headers, 'e2e-ssl', 'site.key', sslKey)

    const summary = await syncDirectory(page.request, headers, 'e2e-ssl', nodes, true)

    expect(summary.failed, JSON.stringify(summary.results)).toBe(0)
    expect(summary.succeeded).toBe(nodes.length)
    expect(summary.skipped).toEqual([])
  })

  test('files the node kept because they already exist are reported', async ({ page }) => {
    const headers = await openAuthenticated(page)
    const nodes = await peerNodeIds(page.request, headers)

    const summary = await syncDirectory(page.request, headers, 'e2e-ssl', nodes, false)

    expect(summary.failed, JSON.stringify(summary.results)).toBe(0)
    for (const result of summary.results) {
      expect(result.skipped_existing).toBe(2)
      expect([...(result.skipped_paths ?? [])].sort()).toEqual(['e2e-ssl/site.key', 'e2e-ssl/site.pem'])
    }
  })

  test('the UI warns about existing files that were not overwritten', async ({ page }) => {
    await gotoRoute(page, '/config')

    const row = page.locator('.ant-table-tbody > tr.ant-table-row', { hasText: 'e2e-ssl' }).first()
    await expect(row).toBeVisible()
    await row.getByText('Deploy', { exact: true }).click()

    const modal = page.locator('.ant-modal', { hasText: 'Deploy Directory' })
    await expect(modal).toBeVisible()

    await modal.locator('.ant-checkbox-group .ant-checkbox-input').first().check()
    await modal.getByRole('checkbox', { name: 'Overwrite' }).uncheck()
    await modal.getByRole('button', { name: 'OK' }).click()

    const notice = page.locator('.ant-notification', { hasText: 'not overwritten' })
    await expect(notice).toBeVisible()
    await expect(notice).toContainText('e2e-ssl/site.pem')
    await page.screenshot({ path: 'test-results/config-directory-sync-existing.png' })
  })

  test('files that cannot be deployed are listed with the reason', async ({ page }) => {
    test.skip(!container, 'set E2E_CONTAINER to drop unsupported files into the config directory')

    execFileSync('docker', [
      'exec', container!, 'sh', '-c',
      'mkdir -p /etc/nginx/e2e-junk && printf "not a config\\n" > /etc/nginx/e2e-junk/notes.txt '
      + '&& printf "\\377\\376\\000\\001" > /etc/nginx/e2e-junk/blob.pem',
    ])

    const headers = await openAuthenticated(page)
    const nodes = await peerNodeIds(page.request, headers)

    const summary = await syncDirectory(page.request, headers, 'e2e-junk', nodes, true)

    // Nothing deployable: not a success, and every file is accounted for.
    expect(summary.total).toBe(0)
    const reasons = Object.fromEntries(summary.skipped.map(file => [file.path, file.reason]))
    expect(reasons['e2e-junk/notes.txt']).toBe('unsupported_type')
    expect(reasons['e2e-junk/blob.pem']).toBe('not_text')

    await gotoRoute(page, '/config')
    const row = page.locator('.ant-table-tbody > tr.ant-table-row', { hasText: 'e2e-junk' }).first()
    await row.getByText('Deploy', { exact: true }).click()
    const modal = page.locator('.ant-modal', { hasText: 'Deploy Directory' })
    await modal.locator('.ant-checkbox-group .ant-checkbox-input').first().check()
    await modal.getByRole('button', { name: 'OK' }).click()

    const notice = page.locator('.ant-notification', { hasText: 'Nothing was synchronized' })
    await expect(notice).toBeVisible()
    await expect(notice).toContainText('e2e-junk/notes.txt (unsupported file type)')
    await expect(notice).toContainText('e2e-junk/blob.pem (not a text file)')
    await page.screenshot({ path: 'test-results/config-directory-sync-skipped.png' })
  })
})
