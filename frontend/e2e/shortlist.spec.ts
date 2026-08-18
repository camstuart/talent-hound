import { test, expect } from "@playwright/test";
import { newWorkspace } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();
const tag = `quokkastack${stamp.toString(36)}`;

test.describe.configure({ mode: "serial" });

// A role in this workspace with its listing attached, extracted, and profiled.
//
// The listing goes on through the role panel's paste control, which is what
// attaches an artifact to the role itself — an artifact linked only to the
// workspace is evidence no role owns, and retrieval maps results to roles.
async function addRole(page: import("@playwright/test").Page, title: string, body: string) {
  const form = page.getByRole("form", { name: "New role" });
  await form.getByLabel("Title").fill(title);
  await form.getByRole("button", { name: "Add role" }).click();
  await expect(page.getByRole("region", { name: "Roles" }).getByText(title)).toBeVisible();

  await page.getByRole("tab", { name: "Research" }).click();
  const roles = page.getByRole("region", { name: "Role profiles" });
  await roles.getByLabel(`Listing text for ${title}`).fill(`${title} requirements. ${body}`);
  await roles.getByLabel(`Attach a listing to ${title}`).click();

  await page.getByRole("tab", { name: "Context" }).click();
  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  await expect(artifacts.getByText(new RegExp(title))).toBeVisible({ timeout: 15_000 });
  await page.getByLabel(`Extract ${title} (pasted)`).click();
  await expect(artifacts.getByText(/, read/).first()).toBeVisible({ timeout: 15_000 });
}

test("builds a shortlist that explains why each role is on it", async ({ page }) => {
  await newWorkspace(page, `Shortlist ${stamp}`);

  const matching = `Platform engineer ${stamp}`;
  await addRole(page, matching, `We need strong ${tag} experience in production, at scale.`);
  const other = `Financial analyst ${stamp}`;
  await addRole(page, other, "Quarterly reporting and reconciliation for a mid-market lender.");

  // Index the workspace so lexical retrieval has something to search.
  const search = page.getByRole("region", { name: "Search" });
  await search.getByLabel("Index this initiative's artifacts").click();
  await expect(search.getByText(/[1-9]\d* indexed section/)).toBeVisible({ timeout: 15_000 });

  const criteria = page.getByRole("region", { name: "Search criteria" });
  await criteria.getByLabel("New criterion").fill(`strong ${tag} experience`);
  await criteria.getByLabel("Add this criterion").click();
  await expect(criteria.getByRole("list", { name: "Criteria" })).toContainText(tag, { timeout: 15_000 });

  // Both listings are attached to roles, but only a Ready-profiled role is
  // eligible — so profile them by hand, which is the path Phase 12 provides.
  await page.getByRole("tab", { name: "Research" }).click();
  const roles = page.getByRole("region", { name: "Role profiles" });
  for (const title of [matching, other]) {
    await roles.getByLabel(`Requirement to add to ${title}`).fill(`Requirement for ${title}`);
    await roles.getByLabel(`Add a requirement to ${title}`).click();
    await expect(
      roles.getByRole("listitem").filter({ hasText: title }).first(),
    ).toContainText("ready — used in assessment", { timeout: 15_000 });
  }

  await page.getByRole("tab", { name: "Matches" }).click();
  const panel = page.getByRole("region", { name: "Shortlist" });
  await panel.getByLabel("Build the shortlist").click();

  // The matching role is on the list, and the list says why.
  await expect(panel.getByText(new RegExp(`1\\. ${matching}`))).toBeVisible({ timeout: 20_000 });
  const why = panel.getByLabel(`Why ${matching} is here`);
  await expect(why).toContainText(/lexical match at rank \d+/);
  await expect(why).toContainText(tag);

  // And the scope is stated, so "nothing matched" and "nothing in scope" can
  // never look the same.
  await expect(panel.getByLabel("Shortlist scope")).toContainText(/of 2 roles in scope/);
});

test("says nothing matched without pretending there were no roles", async ({ page }) => {
  await newWorkspace(page, `Empty shortlist ${stamp}`);

  // Distinct from the first spec's roles: the role list is global.
  const title = `Ledger analyst ${stamp}`;
  await addRole(page, title, "Quarterly reporting and reconciliation for a mid-market lender.");

  const search = page.getByRole("region", { name: "Search" });
  await search.getByLabel("Index this initiative's artifacts").click();
  await expect(search.getByText(/[1-9]\d* indexed section/)).toBeVisible({ timeout: 15_000 });

  const criteria = page.getByRole("region", { name: "Search criteria" });
  await criteria.getByLabel("New criterion").fill(`strong ${tag} experience`);
  await criteria.getByLabel("Add this criterion").click();
  await expect(criteria.getByRole("list", { name: "Criteria" })).toContainText(tag, { timeout: 15_000 });

  await page.getByRole("tab", { name: "Research" }).click();
  const roles = page.getByRole("region", { name: "Role profiles" });
  await roles.getByLabel(`Requirement to add to ${title}`).fill("Reporting and reconciliation");
  await roles.getByLabel(`Add a requirement to ${title}`).click();
  await expect(
    roles.getByRole("listitem").filter({ hasText: title }).first(),
  ).toContainText("ready — used in assessment", { timeout: 15_000 });

  await page.getByRole("tab", { name: "Matches" }).click();
  const panel = page.getByRole("region", { name: "Shortlist" });
  await panel.getByLabel("Build the shortlist").click();

  await expect(panel.getByText(/Nothing matched/)).toBeVisible({ timeout: 20_000 });
  // The role is still there — the market is not empty, the criteria are narrow.
  await expect(panel.getByLabel("Shortlist scope")).toContainText(/0 of 1 roles in scope/);
});
