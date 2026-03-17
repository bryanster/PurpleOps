import { test, expect } from "@playwright/test";
import { login, createAssessment } from "./helpers";

test.describe("Assessment CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("should show assessments page after login", async ({ page }) => {
    await expect(page.locator("#newAssessment")).toBeVisible();
    await expect(page.locator("#assessmentsTable")).toBeVisible();
  });

  test("should open new assessment modal", async ({ page }) => {
    await page.click("#newAssessment");
    await expect(
      page.locator("#newAssessmentModal.show"),
    ).toBeVisible();
    await expect(
      page.locator('#newAssessmentModal input[name="name"]'),
    ).toBeVisible();
    await expect(
      page.locator('#newAssessmentModal textarea[name="description"]'),
    ).toBeVisible();
  });

  test("should create a new assessment", async ({ page }) => {
    const name = `Test Assessment ${Date.now()}`;
    const assessmentId = await createAssessment(
      page,
      name,
      "E2E test assessment",
    );
    expect(assessmentId).toBeTruthy();

    // Verify it appears in the table
    await expect(
      page.locator(`#assessmentsTable a:has-text("${name}")`),
    ).toBeVisible();
  });

  test("should navigate to assessment detail page", async ({ page }) => {
    const name = `Detail Assessment ${Date.now()}`;
    const assessmentId = await createAssessment(page, name, "Detail test");

    // Click on the assessment name link
    await page.click(`#assessmentsTable a:has-text("${name}")`);
    // Chi may serve with or without trailing slash
    await page.waitForURL(new RegExp(`/assessment/${assessmentId}/?$`));

    // Should see the assessment page with test case table
    await expect(page.locator("#assessmentTable")).toBeVisible();
    await expect(page.locator("#newTestcase")).toBeVisible();
  });

  test("should delete an assessment", async ({ page }) => {
    const name = `Delete Assessment ${Date.now()}`;
    await createAssessment(page, name, "To be deleted");

    // Find the row and click delete (button element)
    const row = page.locator(`#assessmentsTable tr:has-text("${name}")`);
    await row.locator('button[onclick*="deleteAssessment"]').click();

    // Confirm deletion in modal
    await expect(page.locator("#deleteAssessmentModal.show")).toBeVisible();
    await page.click("#deleteAssessmentButton");
    await page.waitForTimeout(1000);

    // Verify it's gone
    await expect(page.locator(`text="${name}"`)).not.toBeVisible();
  });
});
