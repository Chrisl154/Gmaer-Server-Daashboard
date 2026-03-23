import { chromium, FullConfig } from '@playwright/test';
import * as fs from 'fs';

/**
 * Global setup: logs in as admin once and saves the browser storage state
 * so every test spec starts already authenticated.
 */
async function globalSetup(config: FullConfig) {
  const baseURL = config.projects[0].use.baseURL ?? 'http://localhost:3000';
  const adminUser = process.env.TEST_ADMIN_USER ?? 'admin';
  const adminPass = process.env.TEST_ADMIN_PASS ?? 'TestPassword123!';

  const browser = await chromium.launch();
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();

  // Wait for the app to respond (Vite dev or nginx may take a moment)
  let attempts = 30;
  while (attempts-- > 0) {
    try {
      const res = await page.goto(`${baseURL}/login`, { timeout: 5000 });
      if (res && res.ok()) break;
    } catch {
      await new Promise(r => setTimeout(r, 1000));
    }
  }

  // Fill login form
  await page.getByPlaceholder('admin').fill(adminUser);
  await page.locator('input[type="password"]').fill(adminPass);
  await page.getByRole('button', { name: 'Sign in' }).click();

  // Wait for redirect to dashboard
  await page.waitForURL('**/');

  // Persist auth cookies/localStorage for all test specs
  fs.mkdirSync('.auth', { recursive: true });
  await context.storageState({ path: '.auth/admin.json' });

  await browser.close();
}

export default globalSetup;
