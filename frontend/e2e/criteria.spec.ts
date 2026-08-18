import { test, expect } from "@playwright/test";
import { newWorkspace } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

// These specs share the global candidate list and the same panel, so they run
// in order rather than racing each other through it.
test.describe.configure({ mode: "serial" });

test("adds lawful criteria, reorders them, and refuses a protected one outright", async ({ page }) => {
  await newWorkspace(page, `Criteria ${stamp}`);
  const panel = page.getByRole("region", { name: "Search criteria" });

  await panel.getByLabel("New criterion").fill(`five years of production Go ${stamp}`);
  await panel.getByLabel("Priority to add with").selectOption("must_have");
  await panel.getByLabel("Add this criterion").click();
  await expect(panel.getByLabel("Criterion 1", { exact: true })).toContainText(String(stamp), { timeout: 15_000 });

  await panel.getByLabel("New criterion").fill(`has led a platform team ${stamp}`);
  await panel.getByLabel("Priority to add with").selectOption("nice_to_have");
  await panel.getByLabel("Add this criterion").click();
  await expect(panel.getByLabel("Criterion 2", { exact: true })).toContainText("platform team", { timeout: 15_000 });

  // A work-rights requirement is lawful and must keep working.
  await panel.getByLabel("New criterion").fill("must have Australian work rights");
  await panel.getByLabel("Add this criterion").click();
  await expect(panel.getByLabel("Criterion 3", { exact: true })).toContainText("work rights", { timeout: 15_000 });

  // Ordering is presentation: the version does not move.
  const versionBefore = await panel.getByLabel("Criteria version").textContent();
  await panel.getByLabel("Move criterion 1 down").click();
  await expect(panel.getByLabel("Criterion 1", { exact: true })).toContainText("platform team", { timeout: 15_000 });
  await expect(panel.getByLabel("Criteria version")).toHaveText(versionBefore ?? "");

  // And a protected criterion is refused, finally, with nothing added.
  await panel.getByLabel("New criterion").fill("must be under 35");
  await panel.getByLabel("Add this criterion").click();
  const refusal = panel.getByLabel("Refused criterion");
  await expect(refusal).toContainText("names age", { timeout: 15_000 });
  await expect(refusal).toContainText("no way to add this one");
  await expect(panel.getByLabel("Criterion 4", { exact: true })).toBeHidden();

  // Retrying the same wording is refused the same way.
  await panel.getByLabel("Add this criterion").click();
  await expect(refusal).toContainText("names age");
  await expect(panel.getByLabel("Criterion 4", { exact: true })).toBeHidden();
});

test("a nationality criterion is refused where a work-rights one is not", async ({ page }) => {
  await newWorkspace(page, `Nationality ${stamp}`);
  const panel = page.getByRole("region", { name: "Search criteria" });

  await panel.getByLabel("New criterion").fill("must be an Australian citizen");
  await panel.getByLabel("Add this criterion").click();
  await expect(panel.getByLabel("Refused criterion")).toContainText("national origin", { timeout: 15_000 });

  await panel.getByLabel("New criterion").fill(`right to work in Australia ${stamp}`);
  await panel.getByLabel("Add this criterion").click();
  await expect(panel.getByLabel("Criterion 1", { exact: true })).toContainText("right to work", { timeout: 15_000 });
});

test("proposals need an approved profile and never apply themselves", async ({ page }) => {
  await newWorkspace(page, `Proposals ${stamp}`);
  const name = `Priya Raman ${stamp}`;
  const form = page.getByRole("form", { name: "New candidate" });
  await form.getByLabel("Full name").fill(name);
  await form.getByLabel("Location", { exact: true }).fill("Sydney, NSW");
  await form.getByRole("button", { name: "Add candidate" }).click();
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toBeVisible();

  const panel = page.getByRole("region", { name: "Search criteria" });
  await panel.getByLabel("Propose from candidate").selectOption({ label: name });
  await panel.getByLabel("Propose criteria from this candidate's approved profile").click();
  // Nothing is approved yet, so the backend says so in its own words rather
  // than proposing from unapproved evidence.
  await expect(panel.getByText(/approved profile/)).toBeVisible({ timeout: 15_000 });
});
