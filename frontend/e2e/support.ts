import { expect, type Page } from "@playwright/test";

// Shared setup for the end-to-end specs.
//
// Every spec needs a workspace and most need a record in it, and six copies of
// that setup is six places for one to drift into testing something slightly
// different from the others.
//
// All content passed through here is invented. No real candidate information
// appears in this repository's tests, fixtures, or output.

/** newWorkspace creates an initiative and waits for its tab. */
export async function newWorkspace(page: Page, name: string, type = "talent_search") {
  await page.goto("/");
  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption(type);
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();
}

/** newCandidate adds a candidate through the records form. */
export async function newCandidate(page: Page, name: string, location = "") {
  const form = page.getByRole("form", { name: "New candidate" });
  await form.getByLabel("Full name").fill(name);
  if (location) await form.getByLabel("Location", { exact: true }).fill(location);
  await form.getByRole("button", { name: "Add candidate" }).click();
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toBeVisible();
}

/** newRole adds a role through the records form. */
export async function newRole(page: Page, title: string) {
  const form = page.getByRole("form", { name: "New role" });
  await form.getByLabel("Title").fill(title);
  await form.getByRole("button", { name: "Add role" }).click();
  await expect(page.getByRole("region", { name: "Roles" }).getByText(title)).toBeVisible();
}

/** addCriterion adds one search criterion and waits for it to appear. */
export async function addCriterion(page: Page, text: string, priority = "must_have") {
  const criteria = page.getByRole("region", { name: "Search criteria" });
  await criteria.getByLabel("New criterion").fill(text);
  await criteria.getByLabel("Priority to add with").selectOption(priority);
  await criteria.getByLabel("Add this criterion").click();
  await expect(criteria.getByRole("list", { name: "Criteria" })).toContainText(text.slice(0, 20), {
    timeout: 15_000,
  });
}

/** indexWorkspace chunks the initiative's extracted artifacts. */
export async function indexWorkspace(page: Page) {
  const search = page.getByRole("region", { name: "Search" });
  await search.getByLabel("Index this initiative's artifacts").click();
  await expect(search.getByText(/[1-9]\d* indexed section/)).toBeVisible({ timeout: 15_000 });
}
