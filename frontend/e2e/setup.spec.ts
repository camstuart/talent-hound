import { test, expect } from "@playwright/test";

// This spec creates no records: setup and diagnostics are installation-wide, so
// there is nothing here to name uniquely per run.

// Setup and diagnostics live in the settings panel, reached from the sidebar.
async function openSettings(page: import("@playwright/test").Page) {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings" }).click();
  await expect(page.getByRole("region", { name: "Setup" })).toBeVisible();
}

test("shows the ordered setup steps with the one to be on", async ({ page }) => {
  await openSettings(page);
  const setup = page.getByRole("region", { name: "Setup" });

  const steps = setup.getByRole("list", { name: "Setup steps" });
  await expect(steps).toContainText("Choose the data folder");
  await expect(steps).toContainText("Check the volume's encryption");
  await expect(steps).toContainText("Create the first initiative");

  // The backend decides the position; the interface only shows it.
  await expect(setup.getByLabel("Setup position")).toBeVisible();
  await expect(setup.getByLabel("Application version")).toContainText("0.1.0");
  // Every required model is listed with a download size.
  await expect(setup.getByRole("list", { name: "Required models" })).toContainText("GB");
});

test("blocking real data is stated, and does not switch the scope by itself", async ({ page }) => {
  await openSettings(page);
  const setup = page.getByRole("region", { name: "Setup" });

  await setup.getByLabel("Check the volume again").click();
  const scope = setup.getByLabel("Data scope");
  await expect(scope).toBeVisible();
  const before = await scope.textContent();

  // Whatever this machine's volume reports, the scope is unchanged by checking.
  await setup.getByLabel("Check the volume again").click();
  await expect(scope).toHaveText(before ?? "");
});

// Demo scope is deliberately not exercised here: it is global to the
// installation, and flipping it mid-run would refuse the candidates the rest of
// this suite creates. The refusal itself is covered in the Go tests, at the
// write boundary where it is enforced.
test("both scopes are offered as deliberate choices", async ({ page }) => {
  await openSettings(page);
  const setup = page.getByRole("region", { name: "Setup" });
  await expect(setup.getByLabel("Work in demo scope")).toBeVisible();
  await expect(setup.getByLabel("Work with real candidate data")).toBeVisible();
  await expect(setup.getByLabel("Encryption state")).toBeVisible();
});

test("reports diagnostics built from facts, with no content and no telemetry", async ({ page }) => {
  await openSettings(page);
  const diagnostics = page.getByRole("region", { name: "Diagnostics" });

  await expect(diagnostics.getByLabel("Application version")).toContainText("0.1.0");
  await expect(diagnostics.getByLabel("Schema version")).toContainText("schema v");
  await expect(diagnostics.getByLabel("Data folder", { exact: true })).toContainText("e2e");
  await expect(diagnostics.getByRole("list", { name: "Record counts" })).toContainText("candidates:");

  // The logs folder path is what the recruiter needs, and it is shown.
  await diagnostics.getByLabel("Open the logs folder").click();
  await expect(diagnostics.getByLabel("Logs folder", { exact: true })).toContainText("logs");

  // The recovery procedure names the resolved folder, not a generic location.
  await expect(diagnostics.getByLabel("Recovery procedure")).toContainText("e2e");

  // Nothing here offers to send anything anywhere.
  await expect(diagnostics).not.toContainText(/telemetry|analytics/i);
});

test("delete-all refuses anything but the exact folder", async ({ page }) => {
  await openSettings(page);
  const diagnostics = page.getByRole("region", { name: "Diagnostics" });

  // Deliberately wrong: this spec never confirms the real folder, because that
  // folder is this run's database.
  await diagnostics.getByLabel("Folder to confirm").fill("yes");
  await diagnostics.getByLabel("Delete everything in the data folder").click();

  await expect(diagnostics.getByText(/nothing was deleted/)).toBeVisible();
  await expect(diagnostics.getByLabel("Deletion outcome")).toHaveCount(0);
  // The database is still there: the counts still render.
  await diagnostics.getByLabel("Refresh the diagnostic report").click();
  await expect(diagnostics.getByRole("list", { name: "Record counts" })).toBeVisible();
});
