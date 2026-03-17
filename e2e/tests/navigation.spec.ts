import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Navigation", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("should navigate between pages via navbar", async ({ page }) => {
    // Home / assessments page
    await page.goto("/");
    await expect(page.locator("#assessmentsTable")).toBeVisible();

    // Access management page
    await page.goto("/manage/access");
    await expect(page.locator("#userTable")).toBeVisible();

    // Back to home
    await page.goto("/");
    await expect(page.locator("#assessmentsTable")).toBeVisible();
  });

  test("should serve static assets", async ({ page }) => {
    // Check that CSS loads
    const cssResponse = await page.request.get("/static/style/bootstrap.min.css");
    expect(cssResponse.status()).toBe(200);

    // Check that JS loads
    const jsResponse = await page.request.get(
      "/static/scripts/assessments.js",
    );
    expect(jsResponse.status()).toBe(200);
  });

  test("should return 404 for non-existent assessment", async ({ page }) => {
    await page.goto("/assessment/000000000000000000000000/");
    // Should show an error since the assessment doesn't exist
    await expect(page.locator("body")).toContainText("not found");
  });
});
