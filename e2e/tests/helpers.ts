import type { APIResponse, Locator, Page, Response } from '@playwright/test'
import { expect } from '@playwright/test'

type LocatorRoot = Locator | Page

interface PersistedUserState {
  secureSessionId?: string
  token?: string
}

export function routeUrl(route: string) {
  return `/#${route.startsWith('/') ? route : `/${route}`}`
}

export async function gotoRoute(page: Page, route: string) {
  const url = routeUrl(route)
  const response = await page.goto(url, { waitUntil: 'domcontentloaded' })

  if (response) {
    expect(response.ok(), `Navigation to ${url} returned ${response.status()}`).toBe(true)
  }
  await expect.poll(() => new URL(page.url()).pathname).toBe('/')
  await expect.poll(() => new URL(page.url()).hash.split('?')[0]).toBe(url.slice(1))
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/)
}

export function tableRows(root: LocatorRoot) {
  return root.locator('.ant-table-tbody > tr.ant-table-row')
}

export async function expectTableRows(root: LocatorRoot, minimum: number) {
  const rows = tableRows(root)
  await expect.poll(() => rows.count()).toBeGreaterThanOrEqual(minimum)
  return rows
}

export async function expectEmptyTable(root: LocatorRoot) {
  await expect.poll(() => tableRows(root).count()).toBe(0)
  await expect(root.locator('.ant-table-placeholder .ant-empty').first()).toBeVisible()
}

export function waitForApiResponse(
  page: Page,
  pathname: string,
  method: string,
  timeout = 90_000,
): Promise<Response> {
  return page.waitForResponse(response => {
    const url = new URL(response.url())
    return url.pathname === pathname && response.request().method() === method
  }, { timeout })
}

export async function authHeaders(page: Page): Promise<Record<string, string>> {
  const state = await page.evaluate(() => {
    const raw = localStorage.getItem('user')
    return raw ? JSON.parse(raw) as PersistedUserState : {}
  })

  expect(state.token, 'The persisted user store has no access token').toBeTruthy()

  return {
    Authorization: state.token ?? '',
    ...(state.secureSessionId ? { 'X-Secure-Session-ID': state.secureSessionId } : {}),
  }
}

function containsCode(value: unknown, expectedCode: number): boolean {
  if (Array.isArray(value)) {
    return value.some(item => containsCode(item, expectedCode))
  }
  if (value && typeof value === 'object') {
    return Object.entries(value).some(([key, item]) =>
      (key === 'code' && Number(item) === expectedCode) || containsCode(item, expectedCode),
    )
  }
  return false
}

export async function expectDemoRefusal(response: APIResponse) {
  expect(response.status()).toBe(500)
  expect(response.headers()['content-type']).toContain('application/json')

  const body = await response.json() as unknown
  expect(containsCode(body, 40300), `Expected a nested code 40300 in ${JSON.stringify(body)}`).toBe(true)
}

export function numberFromText(text: string): number {
  const match = text.replaceAll(',', '').match(/-?\d+(?:\.\d+)?/)
  return match ? Number(match[0]) : Number.NaN
}

export async function expectPositiveText(locator: Locator, timeout = 45_000) {
  await expect.poll(async () => numberFromText(await locator.textContent() ?? ''), { timeout }).toBeGreaterThan(0)
}
