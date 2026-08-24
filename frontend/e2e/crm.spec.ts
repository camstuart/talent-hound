import { test, expect } from "@playwright/test";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

test("a logged call becomes findable evidence with a visible history", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("CRM", { exact: true }).click();

  // Create the candidate through the CRM's own form.
  const name = `Casey Quokka ${stamp}`;
  await page.getByRole("button", { name: "New candidate", exact: true }).click();
  const createForm = page.getByRole("form", { name: "New candidate" });
  await createForm.getByLabel("Full name *").fill(name);
  await createForm.getByRole("button", { name: "Add candidate" }).click();
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
  await page.goto("/");
  await page.getByLabel("CRM", { exact: true }).click();

  // Create the candidate through the CRM's own form.
  const name = `Robin Wombat ${stamp}`;
  await page.getByRole("button", { name: "New candidate", exact: true }).click();
  const candidateForm = page.getByRole("form", { name: "New candidate" });
  await candidateForm.getByLabel("Full name *").fill(name);
  await candidateForm.getByRole("button", { name: "Add candidate" }).click();

  // Create the role through the CRM's own Roles tab and form.
  const roleTitle = `Staff Recruiter ${stamp}`;
  await page.getByRole("tablist", { name: "Record types" }).getByRole("tab", { name: "Roles" }).click();
  await page.getByRole("button", { name: "New role", exact: true }).click();
  const roleForm = page.getByRole("form", { name: "New role" });
  await roleForm.getByLabel("Title *").fill(roleTitle);
  await roleForm.getByRole("button", { name: "Add role" }).click();
  await expect(page.getByRole("list", { name: "Records" }).getByText(roleTitle, { exact: true })).toBeVisible();

  // Back to Candidates to select the candidate and log the placement against it.
  await page.getByRole("tablist", { name: "Record types" }).getByRole("tab", { name: "Candidates" }).click();
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
