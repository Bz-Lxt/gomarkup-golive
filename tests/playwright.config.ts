import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  testMatch: 'e2e_flow.spec.ts',
  timeout: 30_000,
  use: {
    baseURL: process.env.GOLIVE_URL || 'http://127.0.0.1:19443',
    headless: true,
  },
})
