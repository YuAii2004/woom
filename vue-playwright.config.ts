import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e-vue',
  use: {
    baseURL: 'http://127.0.0.1:5174',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure'
  },
  projects: [{
    name: 'chromium-vue',
    use: {
      ...devices['Desktop Chrome'],
      launchOptions: {
        args: ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream']
      }
    }
  }],
  webServer: [
    {
      command: 'go run .',
      url: 'http://127.0.0.1:4000/healthz',
      reuseExistingServer: true,
      timeout: 120_000
    },
    {
      command: 'npm run dev:vue -- --host 127.0.0.1 --port 5174',
      url: 'http://127.0.0.1:5174',
      reuseExistingServer: true,
      timeout: 120_000
    }
  ]
})
