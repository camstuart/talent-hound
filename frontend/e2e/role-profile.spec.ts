import { test, expect } from "@playwright/test";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

// The role listing is global, so these specs share it and must not race.
test.describe.configure({ mode: "serial" });

async function openWorkspace(page: import("@playwright/test").Page, name: string) {
  await page.goto("/");
  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();
}

async function addRole(page: import("@playwright/test").Page, title: string) {
  const form = page.getByRole("form", { name: "New role" });
  await form.getByLabel("Title").fill(title);
  await form.getByRole("button", { name: "Add role" }).click();
  await expect(page.getByRole("region", { name: "Roles" }).getByText(title)).toBeVisible();
}

test("shows every role's profile state and lets a failed listing be completed by hand", async ({ page }) => {
  await openWorkspace(page, `Roles ${stamp}`);
  const title = `Senior platform engineer ${stamp}`;
  await addRole(page, title);

  await page.getByRole("tab", { name: "Research" }).click();
  const panel = page.getByRole("region", { name: "Role profiles" });

  // A role nobody has profiled is shown as exactly that — an absence in this
  // list would be indistinguishable from a role that was never discovered.
  const row = panel.getByRole("listitem").filter({ hasText: title }).first();
  await expect(row).toContainText("not profiled yet");

  // No classify model is assigned in the test environment, so profiling refuses
  // in the backend's own words rather than inventing a decomposition.
  await panel.getByLabel(`Profile ${title}`).click();
  await expect(panel.locator(".modal-error")).toContainText(/classify|model/, { timeout: 15_000 });

  // The way forward is manual entry, and a hand-built profile is a first-class
  // one: recruiter supplied, and assessable.
  await panel.getByLabel(`Requirement to add to ${title}`).fill(`Must have Go ${stamp}`);
  await panel.getByLabel(`Add a requirement to ${title}`).click();

  const updated = panel.getByRole("listitem").filter({ hasText: title }).first();
  await expect(updated).toContainText("ready — used in assessment", { timeout: 15_000 });
  await expect(updated).toContainText("Recruiter supplied");
});

test("a hand-entered requirement can be edited and shows its evidence", async ({ page }) => {
  await openWorkspace(page, `Role edits ${stamp}`);
  const title = `Data engineer ${stamp}`;
  await addRole(page, title);

  await page.getByRole("tab", { name: "Research" }).click();
  const panel = page.getByRole("region", { name: "Role profiles" });

  await panel.getByLabel(`Requirement to add to ${title}`).fill(`Must have dbt ${stamp}`);
  await panel.getByLabel(`Add a requirement to ${title}`).click();
  await expect(panel.getByLabel(`Requirement 1 of ${title}`, { exact: true })).toContainText(String(stamp), { timeout: 15_000 });

  // A recruiter-authored requirement cites the record a person typed it into,
  // which is the same citation rule in a different currency.
  await panel.getByLabel(`Show the evidence for requirement 1 of ${title}`).click();
  await expect(panel.getByText(/Recruiter supplied — role \d+/)).toBeVisible();
  await panel.getByLabel("Close the evidence for requirement 1").click();

  await panel.getByLabel(`Edit requirement 1 of ${title}`).click();
  await panel.getByLabel(`Wording for requirement 1 of ${title}`).fill(`dbt — negotiable ${stamp}`);
  await panel.getByLabel(`Save requirement 1 of ${title}`).click();
  await expect(panel.getByLabel(`Requirement 1 of ${title}`, { exact: true })).toContainText("negotiable", { timeout: 15_000 });

  // Editing does not make it stale: the evidence did not move, the recruiter's
  // account of it did.
  const row = panel.getByRole("listitem").filter({ hasText: title }).first();
  await expect(row).toContainText("ready — used in assessment");
});
