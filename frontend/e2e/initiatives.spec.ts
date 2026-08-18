import { test, expect } from "@playwright/test";

// Unique per run: the E2E database (see playwright.config.ts webServer env)
// persists across local runs, so assertions must target this run's rows.
// Every name below is invented — no real candidate information is used.
const stamp = Date.now();

async function newInitiative(page: import("@playwright/test").Page, name: string, type: string, candidate?: string) {
  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption(type);
  if (candidate) await page.getByPlaceholder("Candidate full name").fill(candidate);
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();
}

test("creates an initiative, shows it in the sidebar and opens a closable tab", async ({ page }) => {
  const name = `Find staff engineers ${stamp}`;
  await page.goto("/");

  await newInitiative(page, name, "talent_search");

  const sidebarItem = page.locator(".sidebar").getByText(name);
  await expect(sidebarItem).toBeVisible();
  const tab = page.getByRole("tab", { name: new RegExp(name) });
  await expect(tab.locator('[data-icon="talent_search"]')).toBeVisible();

  // Closing the tab keeps the sidebar entry.
  await page.getByRole("button", { name: `Close ${name}` }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toHaveCount(0);
  await expect(sidebarItem).toBeVisible();
});

test("persists initiatives to SQLite across reloads", async ({ page }) => {
  const name = `BD partnerships ${stamp}`;
  await page.goto("/");

  await newInitiative(page, name, "business_development");
  await page.reload();

  // Loaded back from the real SQLite database via InitiativeService.List.
  const sidebarItem = page.locator(".sidebar").getByRole("button", { name });
  await expect(sidebarItem).toBeVisible();
  await expect(sidebarItem.locator('[data-icon="business_development"]')).toBeVisible();
});

test("creates each initiative type, including a job search with its one candidate", async ({ page }) => {
  await page.goto("/");

  const jobSearch = `Find a Go role ${stamp}`;
  await newInitiative(page, jobSearch, "job_search", `Priya Raman ${stamp}`);
  // Scoped to the Candidates list: the candidate profile panel's picker lists
  // the same names, and an <option> in a closed select is not visible.
  await expect(
    page.getByRole("region", { name: "Candidates" }).getByText(`Priya Raman ${stamp}`),
  ).toBeVisible();

  await newInitiative(page, `Hire designers ${stamp}`, "talent_search");
  await expect(page.getByText(/Talent Search is a workspace shell/)).toBeVisible();

  await newInitiative(page, `Partnerships ${stamp}`, "business_development");
  await expect(page.getByText(/Business Development is a workspace shell/)).toBeVisible();

  // Every workspace has the same four areas.
  const areas = page.getByRole("tablist", { name: "Initiative areas" });
  for (const label of ["Context", "Research", "Matches", "Drafts"]) {
    await expect(areas.getByRole("tab", { name: label })).toBeVisible();
  }
  await areas.getByRole("tab", { name: "Matches" }).click();
  await expect(page.getByRole("tabpanel", { name: "Matches" })).toContainText("assessments");
});

test("archives and reopens a workspace through the real backend", async ({ page }) => {
  const name = `Archivable ${stamp}`;
  await page.goto("/");
  await newInitiative(page, name, "talent_search");

  const panel = page.locator(".initiative-panel-header");
  await expect(panel.getByText("Active")).toBeVisible();

  await panel.getByRole("button", { name: "Archive" }).click();
  await expect(panel.getByText("Archived").first()).toBeVisible();

  // Archived initiatives are out of the default listing until asked for.
  await page.reload();
  await expect(page.locator(".sidebar").getByRole("button", { name })).toHaveCount(0);
  await page.getByLabel("Show archived").check();
  const sidebarItem = page.locator(".sidebar").getByRole("button", { name });
  await expect(sidebarItem).toBeVisible();
  await expect(sidebarItem.getByText("Archived")).toBeVisible();

  await sidebarItem.click();
  await panel.getByRole("button", { name: "Reopen" }).click();
  await expect(panel.getByText("Active")).toBeVisible();
});

test("renames an initiative and keeps the new name after a reload", async ({ page }) => {
  const before = `Original ${stamp}`;
  const after = `Renamed ${stamp}`;
  await page.goto("/");
  await newInitiative(page, before, "talent_search");

  await page.locator(".initiative-panel-header").getByRole("button", { name: "Rename" }).click();
  await page.getByLabel("New name").fill(after);
  await page.getByLabel("New name").press("Enter");

  await expect(page.getByRole("tab", { name: new RegExp(after) })).toBeVisible();
  await page.reload();
  await expect(page.locator(".sidebar").getByRole("button", { name: after })).toBeVisible();
});
