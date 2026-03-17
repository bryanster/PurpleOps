import { test, expect } from "@playwright/test";
import { ADMIN_EMAIL, ADMIN_PASSWORD, login, logout } from "./helpers";

test.describe("Authentication", () => {
  test("should show login page", async ({ page }) => {
    await page.goto("/login");
    await expect(page.locator("#email")).toBeVisible();
    await expect(page.locator("#password")).toBeVisible();
    await expect(page.locator("#submit")).toBeVisible();
  });

  test("should reject invalid credentials", async ({ page }) => {
    await page.goto("/login");
    await page.fill("#email", "wrong@example.com");
    await page.fill("#password", "wrongpassword");
    await page.click("#submit");
    await page.waitForURL("/login");
    // Should show a flash message
    await expect(page.locator(".alert")).toBeVisible();
  });

  test("should reject empty fields via HTML5 validation", async ({ page }) => {
    await page.goto("/login");
    // The email and password fields have "required" attribute,
    // so the browser prevents submission with empty fields.
    const emailInput = page.locator("#email");
    await expect(emailInput).toHaveAttribute("required", "");
    const passwordInput = page.locator("#password");
    await expect(passwordInput).toHaveAttribute("required", "");
  });

  test("should login with valid credentials", async ({ page }) => {
    await login(page);
    // Should be on the assessments page
    await expect(page).toHaveURL(/\/(index)?$/);
    // Should see the assessments page content
    await expect(page.locator("#newAssessment")).toBeVisible();
  });

  test("should logout successfully", async ({ page }) => {
    await login(page);
    await logout(page);
    await expect(page).toHaveURL("/login");
  });

  test("should redirect unauthenticated users to login", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL("/login");
  });

  test("should redirect unauthenticated users from protected routes", async ({
    page,
  }) => {
    await page.goto("/manage/access");
    await expect(page).toHaveURL("/login");
  });
});

test.describe("Password Change", () => {
  test("should show password change form", async ({ page }) => {
    await login(page);
    await page.goto("/password/change");
    await expect(page.locator("#password")).toBeVisible();
    await expect(page.locator("#new_password")).toBeVisible();
    await expect(page.locator("#new_password_confirm")).toBeVisible();
  });

  test("should reject wrong current password", async ({ page }) => {
    await login(page);
    await page.goto("/password/change");
    await page.fill("#password", "wrongcurrent");
    await page.fill("#new_password", "newpassword12345");
    await page.fill("#new_password_confirm", "newpassword12345");
    await page.click("#submit");
    await page.waitForURL("/password/change");
    await expect(page.locator(".alert")).toBeVisible();
  });

  test("should reject short new password", async ({ page }) => {
    await login(page);
    await page.goto("/password/change");
    await page.fill("#password", ADMIN_PASSWORD);
    await page.fill("#new_password", "short");
    await page.fill("#new_password_confirm", "short");
    await page.click("#submit");
    await page.waitForURL("/password/change");
    await expect(page.locator(".alert")).toBeVisible();
  });

  test("should reject mismatched passwords", async ({ page }) => {
    await login(page);
    await page.goto("/password/change");
    await page.fill("#password", ADMIN_PASSWORD);
    await page.fill("#new_password", "newpassword12345");
    await page.fill("#new_password_confirm", "differentpassword");
    await page.click("#submit");
    await page.waitForURL("/password/change");
    await expect(page.locator(".alert")).toBeVisible();
  });
});
