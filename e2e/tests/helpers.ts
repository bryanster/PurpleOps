import { Page, expect } from "@playwright/test";

export const ADMIN_EMAIL = "admin@purpleops.com";
export const ADMIN_PASSWORD = "testpassword123";

export async function login(
  page: Page,
  email = ADMIN_EMAIL,
  password = ADMIN_PASSWORD,
) {
  await page.goto("/login");
  await page.fill("#email", email);
  await page.fill("#password", password);
  await page.click("#submit");
  await page.waitForURL(/\/(index)?$/);
}

export async function logout(page: Page) {
  await page.goto("/logout");
  await page.waitForURL("/login");
}

export async function createAssessment(
  page: Page,
  name: string,
  description: string,
): Promise<string> {
  await page.goto("/");
  await page.click("#newAssessment");
  await page.waitForSelector("#newAssessmentModal.show", { state: "visible" });
  await page.fill('#newAssessmentModal input[name="name"]', name);
  await page.fill('#newAssessmentModal textarea[name="description"]', description);
  await page.click("#newAssessmentButton");
  // Wait for the modal to close and the table to update
  await page.waitForTimeout(500);
  // Get the assessment ID from the table
  const row = page.locator(`#assessmentsTable td:has-text("${name}")`);
  await expect(row.first()).toBeVisible();
  const link = page.locator(`#assessmentsTable a:has-text("${name}")`);
  const href = await link.getAttribute("href");
  // href is /assessment/{id}
  return href?.replace("/assessment/", "") ?? "";
}
