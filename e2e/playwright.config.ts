import { defineConfig, devices } from '@playwright/test'
import { fileURLToPath } from 'node:url'

const authState = fileURLToPath(new URL('./playwright/.auth/admin.json', import.meta.url))

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  timeout: 90_000,
  expect: {
    timeout: 20_000,
  },
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: [
    ['line'],
    ['html', { open: 'never' }],
  ],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://127.0.0.1:18110',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
    viewport: { width: 1440, height: 1000 },
  },
  projects: [
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: authState,
      },
      dependencies: ['setup'],
    },
  ],
})

