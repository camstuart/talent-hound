import { test, expect } from "@playwright/test";
import { newWorkspace } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded, and no request reaches
// a real provider.
const stamp = Date.now();

test.describe.configure({ mode: "serial" });

test("approves a cloud task, previews its payload, revokes it, and resets on an endpoint change", async ({ page }) => {
  await newWorkspace(page, `Cloud ${stamp}`);
  const panel = page.getByRole("region", { name: "Cloud" });

  // The endpoint is global and the E2E database persists, so the revision
  // number is whatever this database has reached — what matters is that it
  // moves when the configuration does.
  await panel.getByLabel("Cloud endpoint URL").fill(`https://api.example-cloud.invalid/v1?run=${stamp}`);
  await panel.getByLabel("Cloud model").fill("cloud-model");
  await panel.getByLabel("Save the cloud endpoint").click();
  await expect(panel.getByLabel("Cloud endpoint state")).toContainText(String(stamp), { timeout: 15_000 });

  // Nothing is approved until someone approves it.
  const drafting = panel.getByRole("listitem").filter({ hasText: "Writing drafts" }).first();
  await expect(drafting).toContainText("not approved");

  await panel.getByLabel("Approve drafting").click();
  await expect(
    panel.getByRole("listitem").filter({ hasText: "Writing drafts" }).first(),
  ).toContainText("approved", { timeout: 15_000 });

  // The payload is shown, not described — and identifiers are already replaced.
  await panel.getByLabel("Preview the payload for drafting").click();
  await expect(panel.getByLabel("Payload text")).toBeVisible({ timeout: 15_000 });
  await panel.getByLabel("Close the payload preview").click();

  await panel.getByLabel("Revoke drafting").click();
  await expect(
    panel.getByRole("listitem").filter({ hasText: "Writing drafts" }).first(),
  ).toContainText("not approved", { timeout: 15_000 });

  // Approve again, then change the endpoint: every approval clears.
  await panel.getByLabel("Approve drafting").click();
  await expect(
    panel.getByRole("listitem").filter({ hasText: "Writing drafts" }).first(),
  ).toContainText("approved", { timeout: 15_000 });

  await panel.getByLabel("Cloud endpoint URL").fill(`https://api.another-cloud.invalid/v1?run=${stamp}`);
  await panel.getByLabel("Save the cloud endpoint").click();
  await expect(panel.getByLabel("Cloud endpoint state")).toContainText("another-cloud", {
    timeout: 15_000,
  });
  await expect(
    panel.getByRole("listitem").filter({ hasText: "Writing drafts" }).first(),
  ).toContainText("not approved");
});

// A screen that shows only what is off invites someone to look for the switch
// that turns on what is forbidden.
test("shows what can never use the cloud, and offers no way to enable it", async ({ page }) => {
  await newWorkspace(page, `Cloud denials ${stamp}`);
  const panel = page.getByRole("region", { name: "Cloud" });

  for (const label of ["Building Candidate Profiles", "Embedding evidence", "Sending raw candidate documents"]) {
    const row = panel.getByRole("listitem").filter({ hasText: label }).first();
    await expect(row).toContainText("never");
    await expect(row).toContainText("local-only");
    await expect(row).toContainText("any configuration");
  }

  // No approval, and no preview, for any of them.
  for (const task of ["candidate_extraction", "embedding", "raw_artifact"]) {
    await expect(panel.getByLabel(`Approve ${task}`)).toHaveCount(0);
    await expect(panel.getByLabel(`Preview the payload for ${task}`)).toHaveCount(0);
  }
});
