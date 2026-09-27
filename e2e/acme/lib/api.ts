import type { APIRequestContext, APIResponse } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { request } from '@playwright/test'
import { authStateFile, baseURL } from './env'

interface StorageState {
  origins?: { origin: string, localStorage: { name: string, value: string }[] }[]
}

/** The admin token that the setup project stored with the browser state. */
export function readToken(): string {
  const state = JSON.parse(readFileSync(authStateFile, 'utf8')) as StorageState
  for (const origin of state.origins ?? []) {
    const user = origin.localStorage.find(item => item.name === 'user')
    if (user) {
      const token = (JSON.parse(user.value) as { token?: string }).token
      if (token)
        return token
    }
  }
  throw new Error(`No token in ${authStateFile}; did the setup project run?`)
}

async function expectOk(response: APIResponse, what: string) {
  if (!response.ok())
    throw new Error(`${what} answered ${response.status()}: ${await response.text()}`)
  return response
}

export interface AcmeApi {
  context: APIRequestContext
  saveSite: (name: string, content: string) => Promise<void>
  enableSite: (name: string) => Promise<void>
  disableSite: (name: string) => Promise<void>
  /** Save the config, then enable it (enable runs nginx -t and reloads). */
  createEnabledSite: (name: string, content: string) => Promise<void>
  dispose: () => Promise<void>
}

export async function createApi(): Promise<AcmeApi> {
  const context = await request.newContext({
    baseURL,
    storageState: authStateFile,
    extraHTTPHeaders: { Authorization: readToken() },
  })

  const saveSite = async (name: string, content: string) => {
    // Saving an enabled site only tests the config; post_action reloads it.
    await expectOk(await context.post(`/api/sites/${encodeURIComponent(name)}`, {
      data: { content, overwrite: true, post_action: 'reload_nginx' },
    }), `POST /api/sites/${name}`)
  }

  const enableSite = async (name: string) => {
    await expectOk(await context.post(`/api/sites/${encodeURIComponent(name)}/enable`), `POST /api/sites/${name}/enable`)
  }

  const disableSite = async (name: string) => {
    await expectOk(await context.post(`/api/sites/${encodeURIComponent(name)}/disable`), `POST /api/sites/${name}/disable`)
  }

  return {
    context,
    saveSite,
    enableSite,
    disableSite,
    async createEnabledSite(name, content) {
      await saveSite(name, content)
      await enableSite(name)
    },
    dispose: () => context.dispose(),
  }
}
