import { test, expect } from "@playwright/test";
import { login, createAssessment } from "./helpers";

test.describe("Testcase Management", () => {
  let assessmentId: string;
  let assessmentName: string;

  test.beforeAll(async ({ browser }) => {
    const page = await browser.newPage();
    await login(page);
    assessmentName = `TC Assessment ${Date.now()}`;
    assessmentId = await createAssessment(
      page,
      assessmentName,
      "For testcase E2E tests",
    );
    await page.close();
  });

  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("should create a new testcase", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();

    await page.fill('#newTestcaseModal input[name="name"]', "E2E Test Case");
    // Select a tactic
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Verify testcase appears in the table
    await expect(
      page.locator('#assessmentTable td:has-text("E2E Test Case")'),
    ).toBeVisible();
  });

  test("should navigate to testcase detail page", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);

    // Create a testcase first
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();
    await page.fill(
      '#newTestcaseModal input[name="name"]',
      "Detail Test Case",
    );
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Click on the testcase name
    await page.click('#assessmentTable a:has-text("Detail Test Case")');
    await page.waitForURL(/\/testcase\//);

    // Should see the testcase form
    await expect(page.locator("#ttpform")).toBeVisible();
    await expect(page.locator("#name")).toBeVisible();
    await expect(page.locator("#save")).toBeVisible();
  });

  test("should edit testcase fields", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);

    // Create a testcase
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();
    await page.fill('#newTestcaseModal input[name="name"]', "Edit Test Case");
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Navigate to testcase detail
    await page.click('#assessmentTable a:has-text("Edit Test Case")');
    await page.waitForURL(/\/testcase\//);

    // Edit fields
    await page.fill("#name", "Updated Test Case");
    await page.fill("#objective", "Test the editing functionality");
    await page.fill("#actions", "Step 1: Do something\nStep 2: Check result");
    await page.fill("#rednotes", "Red team notes here");

    // Save the form
    await page.click("#save");
    await page.waitForTimeout(1000);

    // Verify toast appears (indicates success)
    await expect(page.locator("#toast")).toBeVisible();

    // Reload and verify data persisted
    await page.reload();
    await expect(page.locator("#name")).toHaveValue("Updated Test Case");
    await expect(page.locator("#objective")).toHaveValue(
      "Test the editing functionality",
    );
  });

  test("should toggle testcase visibility", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);

    // Create a testcase
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();
    await page.fill(
      '#newTestcaseModal input[name="name"]',
      "Visibility Test Case",
    );
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Find the row and click the visibility toggle button
    const row = page.locator(
      '#assessmentTable tr:has-text("Visibility Test Case")',
    );
    const visToggle = row.locator('button[onclick*="visibleTest"]');
    await visToggle.click();
    await page.waitForTimeout(1000);

    // The visible column should show the toggled state (initially true, now false)
    const visCell = row.locator("td").nth(5); // visible column (0=checkbox, 1=mitre, 2=name, 3=tactic, 4=state, 5=visible)
    await expect(visCell).toContainText("❌");
  });

  test("should clone a testcase", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);

    // Create a testcase
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();
    await page.fill(
      '#newTestcaseModal input[name="name"]',
      "Clone Original",
    );
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Click clone button on the row
    const row = page.locator(
      '#assessmentTable tr:has-text("Clone Original")',
    );
    await row.locator('button[onclick*="cloneTest"]').click();
    await page.waitForTimeout(1000);

    // Should see cloned testcase with "(Copy)" suffix
    await expect(
      page.locator(
        '#assessmentTable td:has-text("Clone Original (Copy)")',
      ),
    ).toBeVisible();
  });

  test("should delete a testcase", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);

    // Create a testcase
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();
    await page.fill(
      '#newTestcaseModal input[name="name"]',
      "Delete Me TC",
    );
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Click delete button on the row
    const row = page.locator(
      '#assessmentTable tr:has-text("Delete Me TC")',
    );
    await row.locator('button[onclick*="deleteTest"]').click();
    await page.waitForTimeout(1000);

    // Should be removed from the table
    await expect(
      page.locator('#assessmentTable td:has-text("Delete Me TC")'),
    ).not.toBeVisible();
  });

  test("should set blue team fields", async ({ page }) => {
    await page.goto(`/assessment/${assessmentId}/`);

    // Create a testcase
    await page.click("#newTestcase");
    await expect(page.locator("#newTestcaseModal.show")).toBeVisible();
    await page.fill('#newTestcaseModal input[name="name"]', "Blue Fields TC");
    await page.selectOption(
      '#newTestcaseModal select[name="tactic"]',
      "Execution",
    );
    await page.click("#newTestcaseButton");
    await page.waitForTimeout(1000);

    // Navigate to testcase
    await page.click('#assessmentTable a:has-text("Blue Fields TC")');
    await page.waitForURL(/\/testcase\//);

    // Set prevented
    await page.click("#prevented-yes");
    // Set alerted (this auto-sets logged=Yes and hides the logged container)
    await page.click("#alert-yes");
    // Add blue notes
    await page.fill("#bluenotes", "Detection confirmed in SIEM");

    // Save
    await page.click("#save");
    await page.waitForTimeout(1000);

    // Reload and verify
    await page.reload();
    await expect(page.locator("#prevented-yes")).toBeChecked();
    await expect(page.locator("#alert-yes")).toBeChecked();
    // logged=Yes is auto-set when alerted=Yes
    await expect(page.locator("#log-yes")).toBeChecked();
    await expect(page.locator("#bluenotes")).toHaveValue(
      "Detection confirmed in SIEM",
    );
  });
});
