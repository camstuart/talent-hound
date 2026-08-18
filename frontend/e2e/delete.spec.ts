import { test, expect } from "@playwright/test";
import { newWorkspace, newCandidate, newRole } from "./support";

// Unique per run: the E2E database persists across local runs. All names are
// invented — no real candidate information appears in this repository.
const stamp = Date.now().toString(36);

test("previews and then permanently deletes a candidate", async ({ page }) => {
  const name = `Nadia Frost ${stamp}`;
  await newWorkspace(page, `Delete candidate ${stamp}`);
  await newCandidate(page, name, "Wellington");

  await page.locator("section[aria-label='Delete']").getByLabel(`Preview deleting ${name}`).click();

  // The preview names the derived records rather than saying "and related data".
  const preview = page.getByRole("region", { name: "Deletion preview" });
  await expect(preview).toBeVisible();
  await expect(preview.getByLabel("What would be removed")).toContainText("profile versions");
  await expect(preview.getByLabel("What would be removed")).toContainText("candidate-only artifacts");
  // Previewing changes nothing.
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toBeVisible();

  await preview.getByLabel("Confirm this deletion").click();
  await expect(page.getByLabel("Deletion outcome")).toContainText(name);

  // Gone from the records area, and gone after a reload: the delete committed.
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toHaveCount(0);
  await page.reload();
  await page.locator(".sidebar").getByRole("button", { name: `Delete candidate ${stamp}` }).click();
  await expect(page.getByRole("region", { name: "Candidates" }).getByText(name)).toHaveCount(0);
});

test("purges a stale role with everything derived from it", async ({ page }) => {
  const title = `Field Reliability Lead ${stamp}`;
  await newWorkspace(page, `Purge role ${stamp}`);
  await newRole(page, title);

  await page.locator("section[aria-label='Delete']").getByLabel(`Preview purging ${title}`).click();

  const preview = page.getByRole("region", { name: "Deletion preview" });
  const removes = preview.getByLabel("What would be removed");
  await expect(removes).toContainText("source listings, current and historical");
  await expect(removes).toContainText("matches");
  await expect(removes).toContainText("active drafts");
  // What survives is stated as plainly as what goes.
  await expect(removes).toContainText("recruiter notes survive with the role reference cleared");

  await preview.getByLabel("Confirm this deletion").click();
  await expect(page.getByLabel("Deletion outcome")).toContainText(title);
  await expect(page.getByRole("region", { name: "Roles" }).getByText(title)).toHaveCount(0);

  await page.reload();
  await page.locator(".sidebar").getByRole("button", { name: `Purge role ${stamp}` }).click();
  await expect(page.getByRole("region", { name: "Roles" }).getByText(title)).toHaveCount(0);
});
