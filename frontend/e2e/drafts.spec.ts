import { test, expect } from "@playwright/test";
import { newWorkspace, newCandidate } from "./support";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
//
// transport-check-exempt: this file names senders to assert their absence
const stamp = Date.now();

test.describe.configure({ mode: "serial" });

test("asks a question and reports honestly when there is nothing to answer from", async ({ page }) => {
  await newWorkspace(page, `Asking ${stamp}`);
  await page.getByRole("tab", { name: "Drafts" }).click();
  const panel = page.getByRole("region", { name: "Ask and draft" });

  await expect(panel.getByRole("list", { name: "Answers" })).toContainText("No questions asked yet.");

  await panel.getByLabel("Question", { exact: true }).fill("what does this workspace know");
  await panel.getByLabel("Ask this question").click();

  // Nothing is indexed, so the answer says exactly that rather than producing a
  // plausible paragraph.
  await expect(panel.getByLabel("Answer 1")).toContainText(/nothing indexed/, { timeout: 20_000 });
  await expect(panel.getByText(/not supported/)).toBeVisible();
});

test("refuses to draft without approved evidence, in the backend's own words", async ({ page }) => {
  await newWorkspace(page, `Drafting ${stamp}`);
  const name = `Kalinda Reyes ${stamp}`;
  await newCandidate(page, name, "Melbourne, VIC");

  await page.getByRole("tab", { name: "Drafts" }).click();
  const panel = page.getByRole("region", { name: "Ask and draft" });
  await panel.getByLabel("Draft about candidate").selectOption({ label: name });
  await panel.getByLabel("Write a pitch").click();

  // No approved profile, so the refusal names approval rather than producing a
  // pitch about someone nobody has checked.
  await expect(panel.getByText(/approved evidence|approved profile/)).toBeVisible({ timeout: 20_000 });
  await expect(panel.getByRole("list", { name: "Drafts" })).toContainText("No drafts yet.");
});

// The application drafts; the recruiter sends.
test("offers nothing anywhere that sends a message", async ({ page }) => {
  await newWorkspace(page, `No sender ${stamp}`);

  for (const area of ["Context", "Research", "Matches", "Drafts"]) {
    await page.getByRole("tab", { name: area }).click();
    for (const forbidden of [/^send$/i, /send message/i, /send email/i, /send outreach/i]) {
      await expect(page.getByRole("button", { name: forbidden })).toHaveCount(0);
    }
  }

  // And the settings screen collects no credential for a sender.
  await page.getByRole("button", { name: "Settings" }).click();
  const keys = page.getByRole("region", { name: "Provider keys" });
  for (const sender of [/smtp/i, /sendgrid/i, /twilio/i, /mailgun/i]) {
    await expect(keys.getByText(sender)).toHaveCount(0);
  }
});
