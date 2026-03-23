import { test, expect } from '@playwright/test';

test.describe('Settings page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
  });

  test('settings page loads', async ({ page }) => {
    // Heading should be present
    await expect(page.getByRole('heading', { name: /settings/i }).first()).toBeVisible();
  });

  test('multiple settings sections are present', async ({ page }) => {
    // Settings page should have several cards/sections
    const cards = page.locator('section, [class*="card"], [class*="Card"]');
    await expect(cards).toHaveCount({ min: 3 });
  });

  test('notifications section is present', async ({ page }) => {
    await expect(page.getByText(/notifications/i).first()).toBeVisible();
  });

  test('security / 2FA section is present', async ({ page }) => {
    await expect(page.getByText(/two.factor|2fa|authenticator/i).first()).toBeVisible();
  });

  test('API keys section is present', async ({ page }) => {
    await expect(page.getByText(/api key/i).first()).toBeVisible();
  });
});

test.describe('Security page', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/security');
    await page.waitForLoadState('networkidle');
  });

  test('security page loads', async ({ page }) => {
    await expect(page.getByRole('heading').first()).toBeVisible();
  });

  test('audit log or user list is visible', async ({ page }) => {
    // Security page should show users or audit entries
    const content = page.locator('table, [class*="list"], [class*="audit"]').first();
    await expect(content).toBeVisible({ timeout: 8000 });
  });
});
