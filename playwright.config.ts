import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests/e2e',
  timeout: 30_000,
  retries: 0,
  workers: 1,
  reporter: 'list',

  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:8888',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },

  globalSetup: './tests/e2e/global-setup.ts',

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], headless: true },
    },
  ],
});
