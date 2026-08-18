import { test, expect } from "@playwright/test";

// Unique per run: the E2E database persists across local runs. All content is
// invented — no real candidate information is uploaded.
const stamp = Date.now();

async function openWorkspace(page: import("@playwright/test").Page, name: string) {
  await page.goto("/");
  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  await page.getByRole("button", { name: "Create" }).click();
  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();
}

test("uploads a file, renames it, detaches it, and finds it in the orphan library", async ({ page }) => {
  const workspace = `Artifacts ${stamp}`;
  const filename = `resume-${stamp}.txt`;
  const renamed = `Priya — resume ${stamp}`;
  await openWorkspace(page, workspace);

  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  await page.getByLabel("Attach a file").setInputFiles({
    name: filename,
    mimeType: "text/plain",
    buffer: Buffer.from("A short synthetic resume.\nTwo lines long.\n"),
  });

  // The display name defaults to the filename, and the backend reports what the
  // bytes actually are.
  await expect(artifacts.getByText(filename)).toBeVisible();
  await expect(artifacts.getByText(/text\/plain/)).toBeVisible();

  await page.getByLabel(`Rename ${filename}`).click();
  await page.getByLabel(`New name for ${filename}`).fill(renamed);
  await page.getByLabel(`New name for ${filename}`).press("Enter");
  await expect(artifacts.getByText(renamed)).toBeVisible();

  // Bytes and provenance survive a reload; only the display name changed.
  await page.reload();
  await page.locator(".sidebar").getByRole("button", { name: workspace }).click();
  await expect(artifacts.getByText(renamed)).toBeVisible();

  // Detaching removes the link and leaves a visible orphan.
  await page.getByLabel(`Detach ${renamed}`).click();
  await expect(artifacts.getByText(renamed)).toHaveCount(0);
  const orphans = page.getByRole("region", { name: "Orphaned artifacts" });
  await expect(orphans.getByText(renamed)).toBeVisible();

  // And it can be attached back.
  await page.getByLabel(`Attach ${renamed}`).click();
  await expect(artifacts.getByText(renamed)).toBeVisible();
});

test("ingests pasted text as an artifact with no filename", async ({ page }) => {
  const workspace = `Pasted ${stamp}`;
  const name = `Notes from a call ${stamp}`;
  await openWorkspace(page, workspace);

  await page.getByLabel("Pasted text name").fill(name);
  await page.getByLabel("Pasted text", { exact: true }).fill("Zoë said she is available from September.");
  await page.getByRole("button", { name: "Add pasted text" }).click();

  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  await expect(artifacts.getByText(name)).toBeVisible();
  await expect(artifacts.getByText(/text\/plain/)).toBeVisible();
});

test("two uploads of identical bytes are two artifacts", async ({ page }) => {
  const workspace = `Duplicates ${stamp}`;
  const bytes = Buffer.from("identical bytes, two ingestions\n");
  await openWorkspace(page, workspace);

  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  for (const suffix of ["a", "b"]) {
    await page.getByLabel("Attach a file").setInputFiles({
      name: `same-${stamp}-${suffix}.txt`,
      mimeType: "text/plain",
      buffer: bytes,
    });
    await expect(artifacts.getByText(`same-${stamp}-${suffix}.txt`)).toBeVisible();
  }

  // Neither replaced the other: both are listed, with their own provenance.
  await expect(artifacts.getByText(`same-${stamp}-a.txt`)).toBeVisible();
  await expect(artifacts.getByText(`same-${stamp}-b.txt`)).toBeVisible();
});
