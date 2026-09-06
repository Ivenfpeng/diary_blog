import { defineConfig } from '@playwright/test'

const browserChannel = process.env.PLAYWRIGHT_BUNDLED_CHROMIUM === '1' ? undefined : (process.env.PLAYWRIGHT_CHANNEL ?? 'chrome')

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  workers: 1,
  globalSetup: './tests/e2e/global-setup.ts',
  timeout: 45_000,
  expect: { timeout: 10_000 },
  outputDir: '/tmp/diary-blog-e2e-results',
  use: {
    baseURL: 'http://127.0.0.1:18080',
    browserName: 'chromium',
    ...(browserChannel ? { channel: browserChannel } : {}),
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  reporter: [['list']],
})
