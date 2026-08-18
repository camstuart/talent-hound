import { test, expect } from "@playwright/test";
import { newWorkspace } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded, and no request reaches
// a real provider: the backend has no Exa key in this environment, so it
// refuses rather than sending.
const stamp = Date.now();
// Base-36 so the token has no long run of digits: seven or more digits in a
// row is phone-shaped, and the scrubber removes it on purpose — wherever it
// appears, including inside a word.
const tag = `quokkastack${stamp.toString(36)}`;

test.describe.configure({ mode: "serial" });

test("previews a query built from criteria, edits it, and cancels without sending", async ({ page }) => {
  await newWorkspace(page, `Discovery ${stamp}`);

  const criteria = page.getByRole("region", { name: "Search criteria" });
  await criteria.getByLabel("New criterion").fill(`five years of production ${tag} engineering`);
  await criteria.getByLabel("Add this criterion").click();
  await expect(criteria.getByRole("list", { name: "Criteria" })).toContainText(tag, { timeout: 15_000 });

  await page.getByRole("tab", { name: "Research" }).click();
  const panel = page.getByRole("region", { name: "Role discovery" });

  await panel.getByLabel("Build a query").click();
  const field = panel.getByLabel("Query to send");
  await expect(field).toHaveValue(new RegExp(tag), { timeout: 15_000 });

  // Editing is allowed and what is in the box is what would go.
  await field.fill(`platform engineer roles in Melbourne ${tag}`);
  await expect(field).toHaveValue(`platform engineer roles in Melbourne ${tag}`);

  // Cancelling is the absence of the operation: the editor closes and nothing
  // was recorded.
  await panel.getByLabel("Cancel this search").click();
  await expect(panel.getByLabel("Query to send")).toBeHidden();
  await expect(panel.getByRole("list", { name: "Past searches" })).toContainText("No searches yet.");
});

test("a re-added organization warns and a re-added identifier warns more strongly", async ({ page }) => {
  await newWorkspace(page, `Warnings ${stamp}`);

  const name = `Kalinda Reyes ${stamp}`;
  const form = page.getByRole("form", { name: "New candidate" });
  await form.getByLabel("Full name").fill(name);
  await form.getByLabel("Location", { exact: true }).fill("Melbourne, VIC");
  await form.getByRole("button", { name: "Add candidate" }).click();
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toBeVisible();

  // A query is built from approved evidence, so the profile has to be approved
  // before there is anything to build one from.
  const profile = page.getByRole("region", { name: "Candidate profile" });
  await profile.getByLabel("Reload the candidate list").click();
  await profile.getByLabel("Candidate", { exact: true }).selectOption({ label: name });
  await profile.getByLabel("Build this candidate's profile").click();
  await expect(profile.getByText(/Recruiter supplied/).first()).toBeVisible({ timeout: 15_000 });
  await profile.getByLabel("Approve this profile").click();
  await expect(profile.getByLabel("Profile state")).toHaveText("approved", { timeout: 15_000 });

  const criteria = page.getByRole("region", { name: "Search criteria" });
  await criteria.getByLabel("New criterion").fill(`five years of production ${tag} engineering`);
  await criteria.getByLabel("Add this criterion").click();
  await expect(criteria.getByRole("list", { name: "Criteria" })).toContainText(tag, { timeout: 15_000 });

  await page.getByRole("tab", { name: "Research" }).click();
  const panel = page.getByRole("region", { name: "Role discovery" });
  await panel.getByLabel("Search for candidate").selectOption({ label: name });
  await panel.getByLabel("Build a query").click();
  const field = panel.getByLabel("Query to send");
  await expect(field).toBeVisible({ timeout: 15_000 });

  // Naming a company: allowed, and worth knowing.
  await field.fill("platform engineer roles at Northwind Pty Ltd");
  await expect(panel.getByLabel("Organization warning")).toBeVisible({ timeout: 15_000 });
  await expect(panel.getByLabel("Identifier warning")).toBeHidden();

  // Naming the person: the serious one, shown differently.
  await field.fill(`platform engineer roles for ${name}`);
  await expect(panel.getByLabel("Identifier warning")).toBeVisible({ timeout: 15_000 });
  await expect(panel.getByLabel("Identifier warning")).toContainText(/disclose/);

  // Warned or not, it is still the recruiter's choice — the button is there.
  await expect(panel.getByLabel("Send this query")).toBeVisible();
});

test("a search with no provider key fails in the backend's own words", async ({ page }) => {
  await newWorkspace(page, `Send ${stamp}`);

  const criteria = page.getByRole("region", { name: "Search criteria" });
  await criteria.getByLabel("New criterion").fill(`five years of production ${tag} engineering`);
  await criteria.getByLabel("Add this criterion").click();
  await expect(criteria.getByRole("list", { name: "Criteria" })).toContainText(tag, { timeout: 15_000 });

  await page.getByRole("tab", { name: "Research" }).click();
  const panel = page.getByRole("region", { name: "Role discovery" });
  await panel.getByLabel("Build a query").click();
  await expect(panel.getByLabel("Query to send")).toBeVisible({ timeout: 15_000 });

  await panel.getByLabel("Send this query").click();
  // No key is configured here, so the provider rejects it — reported as itself
  // rather than as an empty result.
  await expect(panel.getByText(/rejected the key|could not be reached|not answer in time/)).toBeVisible({
    timeout: 20_000,
  });

  // The attempt is still recorded, with the exact query, because the request
  // was transmitted.
  await expect(panel.getByLabel("Query sent for search 1")).toContainText(tag, { timeout: 15_000 });
});
