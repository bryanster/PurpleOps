import { test, expect } from "@playwright/test";
import { login, createAssessment } from "./helpers";

test.describe("Export", () => {
  let assessmentId: string;

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await login(page);
    assessmentId = await createAssessment(
      page,
      `Export Assessment ${Date.now()}`,
      "For export E2E tests",
    );

    // Create a testcase so we have data to export
    await page.goto(`/assessment/${assessmentId}/`);
    await page.click("#newTestcase");
    await page.waitForSelector("#newTestcaseModal.show", { state: "visible" });
    await page.fill('#newTestcaseModal input[name="name"]', "Export TC");
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(500);
    await page.close();
  });

  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("should export assessment as JSON", async ({ page }) => {
    const downloadPromise = page.waitForEvent("download");
    // page.goto throws "Download is starting" for attachment responses; ignore the error
    await page.goto(`/assessment/${assessmentId}/export/json`).catch(() => {});
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe("export.json");
  });

  test("should export assessment as CSV", async ({ page }) => {
    const downloadPromise = page.waitForEvent("download");
    await page.goto(`/assessment/${assessmentId}/export/csv`).catch(() => {});
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toBe("export.csv");
  });

  test("should show statistics page", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/stats`);
    // The stats page should load without errors
    await expect(page.locator("body")).not.toContainText("500");
    await expect(page.locator("body")).not.toContainText("error");
  });

  test("should show navigator page", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/navigator`);
    // The navigator page should load without errors
    await expect(page.locator("body")).not.toContainText("500");
  });

  test("should return 501 for report export", async ({ page }) => {
    const response = await page.request.post(
      `/assessment/${assessmentId}/export/report`,
    );
    expect(response.status()).toBe(501);
  });
});
