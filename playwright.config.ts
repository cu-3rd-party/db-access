import {devices, type PlaywrightTestConfig} from '@playwright/test';

// the Go harness in tests/e2e/e2e_test.go starts the server and sets DBACCESS_URL
const BASE_URL = process.env.DBACCESS_URL?.replace(/\/$/g, '') || 'http://localhost:3003';

// use a preinstalled browser instead of `playwright install` (e.g. in sandboxes)
const chromiumLaunchOptions = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE ?
  {executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE} :
  {};

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default {
  testDir: './tests/e2e/',
  testMatch: /.*\.test\.e2e\.ts/,

  // tests share one database that is reset per file by the Go harness
  workers: 1,
  timeout: 30 * 1000,
  expect: {timeout: 3000},

  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  maxFailures: process.env.CI ? 1 : 0,

  reporter: process.env.CI ? 'list' : [['list'], ['html', {outputFolder: 'tests/e2e/reports/', open: 'never'}]],

  use: {
    headless: true,
    locale: 'en-US',
    actionTimeout: 3000,
    navigationTimeout: 10 * 1000,
    baseURL: BASE_URL,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {name: 'chromium', use: {...devices['Desktop Chrome'], launchOptions: chromiumLaunchOptions}},
    {name: 'firefox', use: {...devices['Desktop Firefox']}},
    {name: 'Mobile Chrome', use: {...devices['Pixel 5'], launchOptions: chromiumLaunchOptions}},
  ],

  outputDir: 'tests/e2e/test-artifacts/',
  snapshotDir: 'tests/e2e/test-snapshots/',
} satisfies PlaywrightTestConfig;
