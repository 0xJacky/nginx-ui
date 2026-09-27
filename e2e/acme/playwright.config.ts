import { defineConfig, devices } from '@playwright/test'
import { authStateFile, baseURL } from './lib/env'

// Real ACME issuance against Pebble. Start the stack first with
// scripts/up.sh; see README.md.
export default defineConfig({
  testDir: './tests',
  outputDir: './test-results',
  fullyParallel: false,
  // The specs share one Nginx; concurrent reloads would race each other.
  workers: 1,
  timeout: 180_000,
  expect: {
    timeout: 30_000,
  },
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  reporter: [
    ['line'],
    ['html', { open: 'never', outputFolder: './playwright-report' }],
  ],
  use: {
    baseURL,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'setup',
      testMatch: /.*\.setup\.ts/,
    },
    {
      name: 'chromium',
      testMatch: /.*\.spec\.ts/,
      use: {
        ...devices['Desktop Chrome'],
        storageState: authStateFile,
      },
      dependencies: ['setup'],
    },
  ],
})
