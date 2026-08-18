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

test("extracts, indexes, searches, and cites through the real backend", async ({ page }) => {
  const filename = `brief-${stamp}.md`;
  // A unique term, so the assertion is about this run's document only.
  const term = `quokkastack${stamp}`;
  const body = `# Platform engineer\n\n## Requirements\n\nFive years of ${term} in Go. Melbourne based.\n`;

  await openWorkspace(page, `Search ${stamp}`);
  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  const search = page.getByRole("region", { name: "Search" });

  await page.getByLabel("Attach a file").setInputFiles({
    name: filename,
    mimeType: "text/markdown",
    buffer: Buffer.from(body),
  });
  await expect(artifacts.getByText(new RegExp(filename))).toBeVisible();

  await page.getByLabel(`Extract ${filename}`).click();
  await expect(artifacts.getByText(/, read/)).toBeVisible({ timeout: 15_000 });

  await search.getByLabel("Index this initiative's artifacts").click();
  await expect(search.getByText(/[1-9]\d* indexed section/)).toBeVisible({ timeout: 15_000 });

  await search.getByLabel("Search evidence").fill(term);
  await search.getByRole("button", { name: "Search" }).click();

  const results = search.getByRole("list", { name: "Search results" });
  await expect(results.getByText(filename)).toBeVisible({ timeout: 15_000 });
  await expect(results.getByText(/Platform engineer › Requirements/)).toBeVisible();

  // The citation resolves against the stored Markdown before it is shown.
  await results.getByRole("button", { name: /^Cite / }).first().click();
  const cited = search.getByLabel("Cited text");
  await expect(cited).toContainText(term);
  await expect(search.getByText(/Characters \d+–\d+ of/)).toBeVisible();
});

test("a search with no matches says so rather than showing nothing", async ({ page }) => {
  await openWorkspace(page, `Empty search ${stamp}`);
  const search = page.getByRole("region", { name: "Search" });

  await search.getByLabel("Search evidence").fill(`nothinghere${stamp}`);
  await search.getByRole("button", { name: "Search" }).click();
  await expect(search.getByText("Nothing matched every word.")).toBeVisible();
});
