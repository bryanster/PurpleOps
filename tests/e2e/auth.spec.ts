import { test, expect } from '@playwright/test';

test.describe('Authentication', () => {
  test('login page renders', async ({ page }) => {
    await page.goto('/login');
    await expect(page).toHaveTitle(/PurpleOps/i);
    await expect(page.locator('input[name="email"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
  });

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login');
    await page.fill('input[name="email"]', 'nobody@example.com');
    await page.fill('input[name="password"]', 'wrongpassword');
    await page.click('#submit');
    await expect(page).toHaveURL('/login');
    // Flash message with error class should appear.
    const flash = page.locator('.alert-danger, [class*="danger"]');
    await expect(flash).toBeVisible();
  });

  test('login with empty fields shows error', async ({ page }) => {
    await page.goto('/login');
    await page.click('#submit');
    await expect(page).toHaveURL('/login');
  });

  test('unauthenticated access to home redirects to login', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/login/);
  });

  test('unauthenticated access to api-keys redirects to login', async ({ page }) => {
    await page.goto('/api-keys');
    await expect(page).toHaveURL(/\/login/);
  });

  test('logout redirects to login', async ({ page }) => {
    // Visit login page first (even unauthenticated logout should redirect to /login).
    await page.goto('/logout');
    await expect(page).toHaveURL(/\/login/);
  });
});

test.describe('Authentication - admin login', () => {
  const adminEmail = process.env.ADMIN_EMAIL || 'admin@admin.com';
  const adminPassword = process.env.ADMIN_PASSWORD || 'admin';

  test.beforeEach(async ({ page }) => {
    await page.goto('/login');
    await page.fill('input[name="email"]', adminEmail);
    await page.fill('input[name="password"]', adminPassword);
    await page.click('#submit');
  });

  test('admin can log in and reach home', async ({ page }) => {
    // After login, should be at / or /password/change (if initpwd=true).
    await expect(page).toHaveURL(/\/(index|password\/change)?$/);
  });

  test('admin can log out', async ({ page }) => {
    await page.goto('/logout');
    await expect(page).toHaveURL(/\/login/);
  });
});
