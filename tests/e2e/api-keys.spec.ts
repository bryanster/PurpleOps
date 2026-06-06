import { test, expect, Page } from '@playwright/test';

const adminEmail = process.env.ADMIN_EMAIL || 'admin@admin.com';
const adminPassword = process.env.ADMIN_PASSWORD || 'admin';

async function loginAsAdmin(page: Page) {
  await page.goto('/login');
  await page.fill('input[name="email"]', adminEmail);
  await page.fill('input[name="password"]', adminPassword);
  await page.click('button[type="submit"]');
  if (page.url().includes('/password/change')) {
    test.skip();
  }
}

test.describe('API Keys', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
  });

  test('API keys page renders', async ({ page }) => {
    await page.goto('/api-keys');
    await expect(page).toHaveURL('/api-keys');
    await expect(page.locator('body')).toBeVisible();
  });

  test('create API key via request', async ({ page, request }) => {
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'purpleops');

    const res = await request.post('/api-keys', {
      headers: {
        'Cookie': `purpleops=${sessionCookie!.value}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      data: 'name=E2E+Test+Key',
    });
    expect(res.status()).toBe(200);

    const body = await res.json();
    expect(body).toHaveProperty('id');
    expect(body).toHaveProperty('key');
    expect(body.key).toMatch(/^pops_/);
    expect(body.name).toBe('E2E Test Key');

    // Cleanup: revoke the key.
    await request.delete(`/api-keys/${body.id}`, {
      headers: { 'Cookie': `purpleops=${sessionCookie!.value}` },
    });
  });

  test('create API key with no name returns 400', async ({ page, request }) => {
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'purpleops');

    const res = await request.post('/api-keys', {
      headers: {
        'Cookie': `purpleops=${sessionCookie!.value}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      data: 'name=',
    });
    expect(res.status()).toBe(400);
  });

  test('delete nonexistent API key returns 404', async ({ page, request }) => {
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'purpleops');

    const res = await request.delete('/api-keys/507f1f77bcf86cd799439011', {
      headers: { 'Cookie': `purpleops=${sessionCookie!.value}` },
    });
    expect(res.status()).toBe(404);
  });
});
