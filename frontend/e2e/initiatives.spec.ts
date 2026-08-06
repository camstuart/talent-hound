import { test, expect } from "@playwright/test";

// Unique per run: the E2E database (see playwright.config.ts webServer env)
// persists across local runs, so assertions must target this run's rows.
const stamp = Date.now();

test("creates an initiative, shows it in the sidebar and opens a closable tab", async ({ page }) => {
  const name = `Find staff engineers ${stamp}`;
  await page.goto("/");

  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  await page.getByRole("button", { name: "Create" }).click();

  // Sidebar entry and tab (with matching icon) appear.
  const sidebarItem = page.locator(".sidebar").getByText(name);
  await expect(sidebarItem).toBeVisible();
  const tab = page.getByRole("tab", { name: new RegExp(name) });
  await expect(tab).toBeVisible();
  await expect(tab.locator('[data-icon="talent_search"]')).toBeVisible();

  // Closing the tab keeps the sidebar entry.
  await page.getByRole("button", { name: `Close ${name}` }).click();
  await expect(page.getByRole("tab")).toHaveCount(0);
  await expect(sidebarItem).toBeVisible();
});

test("persists initiatives to SQLite across reloads", async ({ page }) => {
  const name = `BD partnerships ${stamp}`;
  await page.goto("/");

  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption("business_development");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();

  await page.reload();

  // Loaded back from the real SQLite database via InitiativeService.List.
  // Scope to this run's row: the E2E database accumulates rows across runs.
  const sidebarItem = page.locator(".sidebar").getByRole("button", { name });
  await expect(sidebarItem).toBeVisible();
  await expect(sidebarItem.locator('[data-icon="business_development"]')).toBeVisible();
});
