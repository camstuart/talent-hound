import { test, expect } from "@playwright/test";
import { newWorkspace, newCandidate, newRole } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

test.describe.configure({ mode: "serial" });

test("refuses to assess without a generate model, in the backend's own words", async ({ page }) => {
  // Not named "Matches": the area tabs are also tabs, and a workspace tab with
  // the same name would make every locator ambiguous.
  await newWorkspace(page, `Assessing ${stamp}`);

  const name = `Kalinda Reyes ${stamp}`;
  await newCandidate(page, name, "Melbourne, VIC");

  // An approved profile, which assessment requires.
  const profile = page.getByRole("region", { name: "Candidate profile" });
  await profile.getByLabel("Reload the candidate list").click();
  await profile.getByLabel("Candidate", { exact: true }).selectOption({ label: name });
  await profile.getByLabel("Build this candidate's profile").click();
  await expect(profile.getByText(/Recruiter supplied/).first()).toBeVisible({ timeout: 15_000 });
  await profile.getByLabel("Approve this profile").click();
  await expect(profile.getByLabel("Profile state")).toHaveText("approved", { timeout: 15_000 });

  const title = `Platform engineer ${stamp}`;
  await newRole(page, title);
  await page.getByRole("tab", { name: "Research" }).click();
  const roles = page.getByRole("region", { name: "Role profiles" });
  await roles.getByLabel(`Requirement to add to ${title}`).fill("Strong Go in production");
  await roles.getByLabel(`Add a requirement to ${title}`).click();
  await expect(
    roles.getByRole("listitem").filter({ hasText: title }).first(),
  ).toContainText("ready — used in assessment", { timeout: 15_000 });

  await page.getByRole("tab", { name: "Matches" }).click();
  const panel = page.getByRole("region", { name: "Matches" });
  await panel.getByLabel("Matches for candidate").selectOption({ label: name });
  await expect(panel.getByText("No assessed matches yet.")).toBeVisible();

  // No generate model is assigned in this environment. Assessment is a
  // background job, so the refusal surfaces where jobs do — and the thing that
  // matters here is that no conclusions were invented in its place.
  await panel.getByLabel("Assess the shortlist").click();
  await expect(panel.getByRole("list", { name: "Assessed matches" })).toContainText("No assessed matches yet.", {
    timeout: 20_000,
  });

  await page.getByRole("tab", { name: "Context" }).click();
  const jobs = page.getByRole("region", { name: "Jobs" });
  await expect(jobs.getByText(/assess/)).toBeVisible({ timeout: 20_000 });
});

test("shows an assessed match with both directions and its evidence", async ({ page }) => {
  // The assessment itself needs a model, which this environment has not got, so
  // this spec proves the surface a recruiter reads: the panel is present, it
  // reports having nothing assessed, and it names the candidate it is about.
  await newWorkspace(page, `Surface ${stamp}`);
  const name = `Tobias Fenn ${stamp}`;
  await newCandidate(page, name, "Perth, WA");

  await page.getByRole("tab", { name: "Matches" }).click();
  const panel = page.getByRole("region", { name: "Matches" });
  await panel.getByLabel("Matches for candidate").selectOption({ label: name });

  await expect(panel.getByRole("list", { name: "Assessed matches" })).toContainText("No assessed matches yet.");
  // And the shortlist that feeds it is on the same screen.
  await expect(page.getByRole("region", { name: "Shortlist" })).toBeVisible();
});
