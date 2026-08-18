import { test, expect } from "@playwright/test";
import { newWorkspace } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

// The candidate dropdown lists every candidate in the database, so these two
// specs would otherwise be reaching into one shared list concurrently.
test.describe.configure({ mode: "serial" });

async function addCandidate(page: import("@playwright/test").Page, name: string, location: string) {
  const form = page.getByRole("form", { name: "New candidate" });
  await form.getByLabel("Full name").fill(name);
  await form.getByLabel("Location", { exact: true }).fill(location);
  await form.getByRole("button", { name: "Add candidate" }).click();
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toBeVisible();
}

test("blocks matching until a candidate profile is approved, and shows its evidence", async ({ page }) => {
  await newWorkspace(page, `Profile ${stamp}`);
  const name = `Kalinda Reyes ${stamp}`;
  await addCandidate(page, name, "Melbourne, VIC");

  const panel = page.getByRole("region", { name: "Candidate profile" });
  await panel.getByLabel("Reload the candidate list").click();
  await panel.getByLabel("Candidate", { exact: true }).selectOption({ label: name });

  // Nothing has been checked, so search and matching are blocked — and the
  // reason is stated rather than the result quietly being empty.
  await expect(panel.getByLabel("Why this candidate is blocked")).toContainText(/no profile|not been approved/);

  // No model runs in the test environment, so the profile is built from the
  // structured record the recruiter typed: recruiter supplied, citing the record.
  await panel.getByLabel("Build this candidate's profile").click();
  // Surface the backend's own words if it refused, rather than timing out on a
  // missing element and saying nothing about why.
  await expect(panel.locator(".modal-error")).toHaveCount(0);
  await expect(panel.getByText(/Recruiter supplied/).first()).toBeVisible({ timeout: 15_000 });
  await expect(panel.getByLabel("Profile state")).toContainText("proposed — not yet approved");

  // Reviewing an aspect shows what it came from, without leaving the screen.
  await panel.getByLabel("Show the evidence for aspect 1").click();
  await expect(panel.getByLabel("Cited text for aspect 1")).toBeVisible();
  await panel.getByLabel("Close the evidence for aspect 1").click();

  // Editing produces a version, and the edited aspect becomes a person's
  // assertion rather than a document's.
  await panel.getByLabel("Edit aspect 1").click();
  await panel.getByLabel("Wording for aspect 1").fill(`Melbourne, VIC — confirmed ${stamp}`);
  await panel.getByLabel("Save aspect 1").click();
  await expect(panel.getByLabel("Aspect 1 wording")).toContainText(String(stamp), { timeout: 15_000 });

  await panel.getByLabel("Approve this profile").click();
  await expect(panel.getByLabel("Profile state")).toHaveText("approved", { timeout: 15_000 });
  await expect(panel.getByLabel("Why this candidate is blocked")).toBeHidden();
});

test("a dropped resume creates the candidate and makes an approved profile stale", async ({ page }) => {
  await newWorkspace(page, `Stale ${stamp}`);
  const name = `Tobias Fenn ${stamp}`;
  await addCandidate(page, name, "Perth, WA");

  const panel = page.getByRole("region", { name: "Candidate profile" });
  await panel.getByLabel("Reload the candidate list").click();
  await panel.getByLabel("Candidate", { exact: true }).selectOption({ label: name });
  await panel.getByLabel("Build this candidate's profile").click();
  // Surface the backend's own words if it refused, rather than timing out on a
  // missing element and saying nothing about why.
  await expect(panel.locator(".modal-error")).toHaveCount(0);
  await expect(panel.getByText(/Recruiter supplied/).first()).toBeVisible({ timeout: 15_000 });
  await panel.getByLabel("Approve this profile").click();
  await expect(panel.getByLabel("Profile state")).toHaveText("approved", { timeout: 15_000 });

  // A resume dropped onto this candidate: one artifact, linked to the person
  // and to the workspace, in one transaction.
  const filename = `fenn-${stamp}.md`;
  await panel.getByLabel("Attach a resume").setInputFiles({
    name: filename,
    mimeType: "text/markdown",
    buffer: Buffer.from(`# ${name}\n\nFinancial analyst at Harbourline, Perth.\n`),
  });
  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  await expect(artifacts.getByText(new RegExp(filename))).toBeVisible({ timeout: 15_000 });

  // Extracting and indexing is what turns the drop into evidence — which is
  // what the approval was not about.
  await page.getByLabel(`Extract ${filename}`).click();
  await expect(artifacts.getByText(/, read/)).toBeVisible({ timeout: 15_000 });
  const search = page.getByRole("region", { name: "Search" });
  await search.getByLabel("Index this initiative's artifacts").click();
  await expect(search.getByText(/[1-9]\d* indexed section/)).toBeVisible({ timeout: 15_000 });

  // Still usable — that is the point of Stale rather than blocked — but the
  // recruiter is told the approval is about older evidence.
  await panel.getByLabel("Candidate", { exact: true }).selectOption({ label: name });
  await expect(panel.getByLabel("Profile warning")).toContainText(/evidence has changed/, { timeout: 15_000 });
  await expect(panel.getByLabel("Why this candidate is blocked")).toBeHidden();
});
