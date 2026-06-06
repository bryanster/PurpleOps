import { test, expect, Page } from '@playwright/test';

const adminEmail = process.env.ADMIN_EMAIL || 'admin@admin.com';
const adminPassword = process.env.ADMIN_PASSWORD || 'admin';

async function loginAsAdmin(page: Page) {
  await page.goto('/login');
  await page.fill('input[name="email"]', adminEmail);
  await page.fill('input[name="password"]', adminPassword);
  await page.click('button[type="submit"]');
  // Handle initpwd redirect.
  if (page.url().includes('/password/change')) {
    test.skip();
  }
}

test.describe('Assessments', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsAdmin(page);
  });

  test('home page lists assessments', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/$/);
    // Page should render without errors.
    await expect(page.locator('body')).toBeVisible();
  });

  test('create a new assessment via API', async ({ page, request }) => {
    // Get session cookie from the logged-in page.
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'purpleops');
    expect(sessionCookie).toBeTruthy();

    const res = await request.post('/assessment', {
      headers: {
        'Cookie': `purpleops=${sessionCookie!.value}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      data: 'name=E2E+Test+Assessment&description=Created+by+Playwright',
    });
    expect(res.status()).toBe(200);

    const body = await res.json();
    expect(body).toHaveProperty('id');
    expect(body.name).toBe('E2E Test Assessment');

    // Clean up: delete the assessment.
    await request.delete(`/assessment/${body.id}/`, {
      headers: { 'Cookie': `purpleops=${sessionCookie!.value}` },
    });
  });

  test('assessment page renders for valid ID', async ({ page, request }) => {
    // Create a temporary assessment.
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'purpleops');

    const res = await request.post('/assessment', {
      headers: {
        'Cookie': `purpleops=${sessionCookie!.value}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      data: 'name=E2E+Page+Test&description=',
    });
    const body = await res.json();
    const id = body.id;

    await page.goto(`/assessment/${id}/`);
    await expect(page).toHaveURL(`/assessment/${id}/`);
    await expect(page.locator('body')).toBeVisible();

    // Cleanup.
    await request.delete(`/assessment/${id}/`, {
      headers: { 'Cookie': `purpleops=${sessionCookie!.value}` },
    });
  });

  test('delete assessment returns 200', async ({ page, request }) => {
    const cookies = await page.context().cookies();
    const sessionCookie = cookies.find(c => c.name === 'purpleops');

    const res = await request.post('/assessment', {
      headers: {
        'Cookie': `purpleops=${sessionCookie!.value}`,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      data: 'name=E2E+Delete+Test&description=',
    });
    const body = await res.json();

    const del = await request.delete(`/assessment/${body.id}/`, {
      headers: { 'Cookie': `purpleops=${sessionCookie!.value}` },
    });
    expect(del.status()).toBe(200);
  });
});
