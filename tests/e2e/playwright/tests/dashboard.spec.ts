import { test, expect } from '@playwright/test';

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
  });

  test('dashboard page loads without errors', async ({ page }) => {
    // No uncaught JS errors (Playwright captures console errors)
    const errors: string[] = [];
    page.on('pageerror', err => errors.push(err.message));

    await page.waitForLoadState('networkidle');
    expect(errors).toHaveLength(0);
  });

  test('sidebar navigation shows all main links', async ({ page }) => {
    const nav = page.locator('nav, aside').first();

    const links = ['Dashboard', 'Servers', 'Backups', 'Mods', 'Ports', 'Security', 'Logs', 'Settings'];
    for (const label of links) {
      await expect(nav.getByText(label, { exact: true })).toBeVisible();
    }
  });

  test('navigating to /servers via sidebar works', async ({ page }) => {
    await page.getByRole('link', { name: 'Servers' }).first().click();
    await page.waitForURL('**/servers');
    await expect(page).toHaveURL(/\/servers/);
  });

  test('"Live" status indicator is present in the top bar', async ({ page }) => {
    await expect(page.getByText('Live')).toBeVisible();
  });
});
