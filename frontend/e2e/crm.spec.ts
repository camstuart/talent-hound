import { test, expect } from "@playwright/test";
import { newWorkspace, newCandidate, newRole } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

test("a logged call becomes findable evidence with a visible history", async ({ page }) => {
  const name = `Casey Quokka ${stamp}`;
  // A workspace is needed to reach the records form that creates the candidate;
  // the CRM tab itself only searches, edits, and logs against existing records.
  await newWorkspace(page, `Call log ${stamp}`);
  await newCandidate(page, name);

  await page.getByLabel("CRM", { exact: true }).click();
  await page.getByRole("list", { name: "Records" }).getByText(name, { exact: true }).click();

  // Log a call whose wording is unique to this run.
  const phrase = `prefers wombatscale-${stamp} platforms`;
  await page.getByLabel("Interaction note").fill(`Casey ${phrase}.`);
  await page.getByLabel("Log interaction form").getByRole("button", { name: "Log interaction" }).click();
  await expect(page.getByRole("list", { name: "Interaction history" }).getByText(phrase)).toBeVisible();

  // The note is talent-search evidence once the chunk job has run.
  await expect(async () => {
    await page.getByLabel("Talent search", { exact: true }).fill(`wombatscale-${stamp}`);
    await page.getByLabel("Talent search form").press("Enter");
    await expect(
      page.getByLabel("Talent search results").getByText(name, { exact: true }),
    ).toBeVisible({ timeout: 2000 });
  }).toPass({ timeout: 20000 });
});

test("an outcome names its role in the timeline", async ({ page }) => {
  const name = `Robin Wombat ${stamp}`;
  const roleTitle = `Staff Recruiter ${stamp}`;
  await newWorkspace(page, `Placement ${stamp}`);
  await newCandidate(page, name);
  await newRole(page, roleTitle);

  await page.getByLabel("CRM", { exact: true }).click();
  await page.getByRole("list", { name: "Records" }).getByText(name, { exact: true }).click();

  await page.getByLabel("Interaction kind").selectOption("placement");
  // The role select only appears for outcome kinds, and only once a role exists.
  await expect(page.getByLabel("Interaction role")).toBeVisible();
  await page.getByLabel("Interaction role").selectOption({ label: roleTitle });
  await page.getByLabel("Interaction note").fill(`Placed into ${roleTitle}.`);
  await page.getByLabel("Log interaction form").getByRole("button", { name: "Log interaction" }).click();

  await expect(
    page.getByRole("list", { name: "Interaction history" }).getByText(roleTitle, { exact: true }),
  ).toBeVisible();
});
