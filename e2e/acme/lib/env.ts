import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

// Mirrors scripts/env.sh. Every value can be overridden from the environment,
// and both sides must agree on the defaults.

export const acmeDir = fileURLToPath(new URL('..', import.meta.url))
export const composeFile = join(acmeDir, 'compose.yml')

export const project = process.env.E2E_ACME_PROJECT ?? 'nginxui-acme-e2e-suite'
export const uiPort = process.env.E2E_ACME_UI_PORT ?? '18181'
export const challtestsrvPort = process.env.E2E_ACME_CHALLTESTSRV_PORT ?? '18056'
export const subnetPrefix = process.env.E2E_ACME_SUBNET_PREFIX ?? '10.31.0'
export const baseURL = process.env.E2E_ACME_BASE_URL ?? `http://127.0.0.1:${uiPort}`
export const challtestsrvURL = process.env.E2E_ACME_CHALLTESTSRV_URL ?? `http://127.0.0.1:${challtestsrvPort}`

export const dataDir = process.env.E2E_ACME_DATA_DIR
  ?? join(process.env.RUNNER_TEMP ?? tmpdir(), project)

// Per-run secrets, removed by scripts/down.sh.
export const credentialsFile = join(dataDir, 'e2e-admin.json')
export const authStateFile = join(dataDir, 'e2e-auth-state.json')

export const addresses = {
  pebble: `${subnetPrefix}.2`,
  challtestsrv: `${subnetPrefix}.3`,
  nginxUI: `${subnetPrefix}.10`,
  backend: `${subnetPrefix}.20`,
  // Inside the subnet but assigned to no container.
  unused: `${subnetPrefix}.250`,
}

// Environment for `docker compose` so its interpolation resolves exactly as it
// did in scripts/up.sh.
export function composeEnv(): NodeJS.ProcessEnv {
  return {
    ...process.env,
    E2E_ACME_PROJECT: project,
    E2E_ACME_UI_PORT: uiPort,
    E2E_ACME_CHALLTESTSRV_PORT: challtestsrvPort,
    E2E_ACME_SUBNET_PREFIX: subnetPrefix,
    E2E_ACME_DATA_DIR: dataDir,
  }
}
