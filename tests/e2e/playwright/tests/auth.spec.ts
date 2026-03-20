import { test, expect } from '@playwright/test';

// Auth tests run WITHOUT the pre-loaded storage state — they own the login flow
test.use({ storageState: { cookies: [], origins: [] } });

test.describe('Authentication', () => {
  test('login page renders with username and password fields', async ({ page }) => {
    await page.goto('/login');

    await expect(page.getByPlaceholder('admin')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();
  });

  test('wrong credentials show an error message', async ({ page }) => {
    await page.goto('/login');

    await page.getByPlaceholder('admin').fill('admin');
    await page.locator('input[type="password"]').fill('definitely-wrong-password');
    await page.getByRole('button', { name: 'Sign in' }).click();

    // Some form of error feedback must appear
    const error = page.locator('[role="alert"], .text-red-500, .text-red-400').first();
    await expect(error).toBeVisible({ timeout: 8000 });
  });

  test('valid credentials redirect to the dashboard', async ({ page }) => {
    await page.goto('/login');

    await page.getByPlaceholder('admin').fill(process.env.TEST_ADMIN_USER ?? 'admin');
    await page.locator('input[type="password"]').fill(process.env.TEST_ADMIN_PASS ?? 'TestPassword123!');
    await page.getByRole('button', { name: 'Sign in' }).click();

    await page.waitForURL('**/');
    await expect(page).toHaveURL(/\/$/);
  });

  test('logout returns to the login page', async ({ page }) => {
    // Log in first
    await page.goto('/login');
    await page.getByPlaceholder('admin').fill(process.env.TEST_ADMIN_USER ?? 'admin');
    await page.locator('input[type="password"]').fill(process.env.TEST_ADMIN_PASS ?? 'TestPassword123!');
    await page.getByRole('button', { name: 'Sign in' }).click();
    await page.waitForURL('**/');

    // Logout — button is in the sidebar
    await page.getByRole('button', { name: /log\s*out/i }).click();

    await page.waitForURL('**/login');
    await expect(page.getByRole('button', { name: 'Sign in' })).toBeVisible();
  });
});
