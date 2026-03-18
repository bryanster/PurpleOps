import { test, expect } from "@playwright/test";
import { login } from "./helpers";

test.describe("API Keys", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("should show API keys page", async ({ page }) => {
    await page.goto("/api-keys");
    await expect(page.locator("#keyTable")).toBeVisible();
    await expect(page.locator('button:has-text("New API Key")')).toBeVisible();
  });

  test("should redirect unauthenticated users to login", async ({ page }) => {
    await page.goto("/logout");
    await page.waitForURL("/login");
    await page.goto("/api-keys");
    await expect(page).toHaveURL("/login");
  });

  test("should open new API key modal", async ({ page }) => {
    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();
    await expect(page.locator("#keyName")).toBeVisible();
  });

  test("should require name field (HTML5 validation)", async ({ page }) => {
    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();
    const nameInput = page.locator("#keyName");
    await expect(nameInput).toHaveAttribute("required", "");
  });

  test("should create a new API key and reveal it once", async ({ page }) => {
    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();

    const keyName = `test-key-${Date.now()}`;
    await page.fill("#keyName", keyName);
    await page.click("#apiKeyDetailButton");

    // The reveal modal should appear with the generated key
    await expect(page.locator("#keyRevealModal.show")).toBeVisible({
      timeout: 5000,
    });
    const revealedKey = page.locator("#revealedKey");
    await expect(revealedKey).toBeVisible();

    const keyValue = await revealedKey.inputValue();
    expect(keyValue).toMatch(/^pops_/);
    expect(keyValue.length).toBeGreaterThan(10);
  });

  test("should list created key in table", async ({ page }) => {
    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();

    const keyName = `list-key-${Date.now()}`;
    await page.fill("#keyName", keyName);
    await page.click("#apiKeyDetailButton");

    // Wait for reveal modal then close it and reload
    await expect(page.locator("#keyRevealModal.show")).toBeVisible({
      timeout: 5000,
    });
    await page.click("#keyRevealModal .btn-secondary");
    await page.waitForURL("/api-keys");

    // Key should appear in the table
    await expect(
      page.locator("#keyTable td").getByText(keyName, { exact: true }),
    ).toBeVisible({ timeout: 5000 });
  });

  test("should show key prefix (not the full key) in table", async ({
    page,
  }) => {
    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();

    const keyName = `prefix-key-${Date.now()}`;
    await page.fill("#keyName", keyName);
    await page.click("#apiKeyDetailButton");

    await expect(page.locator("#keyRevealModal.show")).toBeVisible({
      timeout: 5000,
    });
    const fullKey = await page.locator("#revealedKey").inputValue();
    const prefix = fullKey.slice(0, 13); // "pops_" + 8 hex chars

    await page.click("#keyRevealModal .btn-secondary");
    await page.waitForURL("/api-keys");

    // The prefix (with "...") should appear in the table; full key must NOT
    await expect(
      page.locator("#keyTable").getByText(prefix, { exact: false }),
    ).toBeVisible({ timeout: 5000 });
    await expect(page.locator("#keyTable")).not.toContainText(fullKey);
  });

  test("should revoke an API key", async ({ page }) => {
    // Create a key first
    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();

    const keyName = `delete-key-${Date.now()}`;
    await page.fill("#keyName", keyName);
    await page.click("#apiKeyDetailButton");

    await expect(page.locator("#keyRevealModal.show")).toBeVisible({
      timeout: 5000,
    });
    await page.click("#keyRevealModal .btn-secondary");
    await page.waitForURL("/api-keys");

    // Find the key row and click the delete (revoke) button
    const row = page.locator("#keyTable tr").filter({ hasText: keyName });
    await expect(row).toBeVisible({ timeout: 5000 });
    await row.locator("button.btn-danger").click();

    // Confirm delete modal
    await expect(page.locator("#deleteKeyModal.show")).toBeVisible();
    await expect(page.locator("#deleteKeyModal")).toContainText(keyName);
    await page.click("#deleteKeyButton");
    await page.waitForTimeout(1000);

    // Key should be gone
    await expect(
      page.locator("#keyTable td").getByText(keyName, { exact: true }),
    ).not.toBeVisible();
  });

  test("should copy key to clipboard via copy button", async ({
    page,
    context,
  }) => {
    await context.grantPermissions(["clipboard-read", "clipboard-write"]);

    await page.goto("/api-keys");
    await page.click('button:has-text("New API Key")');
    await expect(page.locator("#apiKeyDetailModal.show")).toBeVisible();

    await page.fill("#keyName", `copy-key-${Date.now()}`);
    await page.click("#apiKeyDetailButton");

    await expect(page.locator("#keyRevealModal.show")).toBeVisible({
      timeout: 5000,
    });
    const fullKey = await page.locator("#revealedKey").inputValue();
    await page.click('#keyRevealModal button:has-text("Copy")');

    const clipboard = await page.evaluate(() =>
      navigator.clipboard.readText(),
    );
    expect(clipboard).toBe(fullKey);
  });
});
