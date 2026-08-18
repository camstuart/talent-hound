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

test("extracts a text artifact through the real backend and shows it as text", async ({ page }) => {
  const filename = `notes-${stamp}.txt`;
  const body = "# Notes\n\n<script>alert(1)</script>\n\nSpoke to the hiring manager.\n";
  await openWorkspace(page, `Extraction ${stamp}`);

  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  await page.getByLabel("Attach a file").setInputFiles({
    name: filename,
    mimeType: "text/plain",
    buffer: Buffer.from(body),
  });
  await expect(artifacts.getByText(new RegExp(`${filename}`))).toBeVisible();
  await expect(artifacts.getByText(/not read/)).toBeVisible();

  // Text needs no sidecar, so this works on any machine the tests run on.
  await page.getByLabel(`Extract ${filename}`).click();
  await expect(artifacts.getByText(/text\/plain.*, read/)).toBeVisible({ timeout: 15_000 });

  await page.getByLabel(`View extraction of ${filename}`).click();
  const view = page.getByLabel("Extracted text");
  // The script tag is characters in a <pre>, not an element in the page.
  await expect(view).toContainText("<script>alert(1)</script>");
  await expect(view).toContainText("Spoke to the hiring manager.");
  await expect(view.locator("script")).toHaveCount(0);
  expect(await view.evaluate((el) => el.textContent)).toBe(body);
});

test("reports an unreadable artifact with a reason code and offers another attempt", async ({ page }) => {
  const filename = `photo-${stamp}.png`;
  await openWorkspace(page, `Extraction failure ${stamp}`);

  const artifacts = page.getByRole("region", { name: "Artifacts", exact: true });
  // Real PNG magic bytes; nothing in this phase reads an image.
  await page.getByLabel("Attach a file").setInputFiles({
    name: filename,
    mimeType: "image/png",
    buffer: Buffer.from("89504e470d0a1a0a0000000d49484452", "hex"),
  });
  await expect(artifacts.getByText(new RegExp(filename))).toBeVisible();

  await page.getByLabel(`Extract ${filename}`).click();
  await expect(artifacts.getByText(/not read \(unsupported_type\)/)).toBeVisible({ timeout: 15_000 });
  await expect(page.getByLabel(`Extract ${filename}`)).toHaveText("Extract again");
});
