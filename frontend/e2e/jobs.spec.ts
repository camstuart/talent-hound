import { test, expect } from "@playwright/test";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is used.
const stamp = Date.now();

async function openWorkspace(page: import("@playwright/test").Page, name: string) {
  await page.goto("/");
  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();
}

test("runs a demo job to completion, showing its progress", async ({ page }) => {
  await openWorkspace(page, `Jobs ${stamp}`);
  const jobs = page.getByRole("region", { name: "Jobs" });

  await page.getByRole("button", { name: "Start demo job" }).click();

  // The real backend runs it: state and counts follow the actual work.
  await expect(jobs.getByText(/demo — (queued|running)/)).toBeVisible();
  await expect(jobs.getByText(/demo — completed, 4\/4/)).toBeVisible({ timeout: 15_000 });
});

test("cancels a demo job and finds it in the cancelled tab, then retries it", async ({ page }) => {
  await openWorkspace(page, `Cancel jobs ${stamp}`);
  const jobs = page.getByRole("region", { name: "Jobs" });

  await page.getByRole("button", { name: "Start demo job" }).click();
  await expect(jobs.getByText(/demo — (queued|running)/)).toBeVisible();

  const cancel = jobs.getByRole("button", { name: /^Cancel job/ });
  await cancel.click();

  // Cancelling moves it out of the list of work in progress. Asserted on the
  // job rather than on an empty list: unattached jobs from other work show in
  // every workspace, so "no jobs at all" is not this test's business.
  await expect(jobs.getByRole("tab", { name: "Cancelled (1)" })).toBeVisible();
  await expect(jobs.getByText(/demo — cancelled/)).toHaveCount(0);

  await jobs.getByRole("tab", { name: "Cancelled (1)" }).click();
  await expect(jobs.getByText(/demo — cancelled/)).toBeVisible();

  // And it can be run again from there.
  await jobs.getByRole("button", { name: /^Retry job/ }).click();
  await expect(jobs.getByRole("tab", { name: "Cancelled (0)" })).toBeVisible();
  await jobs.getByRole("tab", { name: "Current" }).click();
  await expect(jobs.getByText(/demo — completed, 4\/4/)).toBeVisible({ timeout: 15_000 });
});
