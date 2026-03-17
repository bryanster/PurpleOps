import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("Access Management", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("should show access management page", async ({ page }) => {
    await page.goto("/manage/access");
    await expect(page.locator("#userTable")).toBeVisible();
    await expect(page.locator("#add")).toBeVisible();
  });

  test("should show admin user in the table", async ({ page }) => {
    await page.goto("/manage/access");
    await expect(
      page.locator("#userTable td").getByText("admin", { exact: true }),
    ).toBeVisible();
    await expect(
      page.locator("#userTable td").getByText("admin@purpleops.com", {
        exact: true,
      }),
    ).toBeVisible();
  });

  test("should open new user modal", async ({ page }) => {
    await page.goto("/manage/access");
    await page.click("#add");
    await expect(page.locator("#userDetailModal.show")).toBeVisible();
    await expect(page.locator("#userDetailLabel")).toHaveText("New User");
  });

  test("should create a new user", async ({ page }) => {
    await page.goto("/manage/access");
    await page.click("#add");
    await expect(page.locator("#userDetailModal.show")).toBeVisible();

    const username = `testuser_${Date.now()}`;
    await page.fill("#username", username);
    await page.fill("#email", `${username}@test.com`);
    await page.fill("#password", "securepassword123");

    await page.click("#userDetailButton");
    // Wait for AJAX response and table update
    await expect(
      page.locator(`#userTable td`).getByText(username, { exact: true }),
    ).toBeVisible({ timeout: 5000 });
  });

  test("should edit a user", async ({ page }) => {
    await page.goto("/manage/access");

    // Create a user first
    await page.click("#add");
    await expect(page.locator("#userDetailModal.show")).toBeVisible();

    const username = `edituser_${Date.now()}`;
    await page.fill("#username", username);
    await page.fill("#email", `${username}@test.com`);
    await page.fill("#password", "securepassword123");
    await page.click("#userDetailButton");
    await expect(
      page.locator(`#userTable td`).getByText(username, { exact: true }),
    ).toBeVisible({ timeout: 5000 });

    // Click edit on the new user's row
    const row = page.locator(`#userTable tr:has-text("${username}")`);
    await row.locator('button[onclick*="editUser"]').click();
    await expect(page.locator("#userDetailModal.show")).toBeVisible();

    // Update the email
    await page.fill("#email", `updated_${username}@test.com`);
    await page.click("#userDetailButton");
    await page.waitForTimeout(1000);

    // Verify the update
    await expect(
      page.locator(
        `#userTable td:has-text("updated_${username}@test.com")`,
      ),
    ).toBeVisible();
  });

  test("should delete a non-admin user", async ({ page }) => {
    await page.goto("/manage/access");

    // Create a user first
    await page.click("#add");
    await expect(page.locator("#userDetailModal.show")).toBeVisible();

    const username = `deleteuser_${Date.now()}`;
    await page.fill("#username", username);
    await page.fill("#email", `${username}@test.com`);
    await page.fill("#password", "securepassword123");
    await page.click("#userDetailButton");
    await expect(
      page.locator(`#userTable td`).getByText(username, { exact: true }),
    ).toBeVisible({ timeout: 5000 });

    // Click delete on the new user's row
    const row = page.locator(`#userTable tr`).filter({ hasText: username });
    await row.locator('button[onclick*="deleteUser"]').click();

    // Confirm deletion
    await expect(page.locator("#deleteUserModal.show")).toBeVisible();
    await page.click("#deleteUserButton");
    await page.waitForTimeout(1000);

    // Verify user is gone
    await expect(
      page.locator(`#userTable td`).getByText(username, { exact: true }),
    ).not.toBeVisible();
  });

  test("should not allow deleting the admin user", async ({ page }) => {
    await page.goto("/manage/access");

    // The admin row should not have a delete button
    const adminRow = page
      .locator("#userTable tr")
      .filter({ has: page.locator("td").getByText("admin", { exact: true }) })
      .first();
    const deleteBtn = adminRow.locator('button[onclick*="deleteUser"]');
    await expect(deleteBtn).toHaveCount(0);
  });
});
