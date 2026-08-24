import { test, expect } from "@playwright/test";

// Everything here is local configuration — no candidate content and no real
// provider key.

test("shows role pickers and a model library against the real backend", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings" }).click();

  // One select per role. With no Ollama running nothing is installed, so the
  // pickers stay empty rather than offering models that are not there.
  const roles = page.getByRole("region", { name: "Model roles" });
  await expect(roles.getByText(/embed/).first()).toBeVisible();
  for (const role of ["embed", "classify", "generate"]) {
    await expect(roles.getByLabel(`Model for ${role}`)).toBeVisible();
  }
  // The endpoint is not running in the test environment, and the state says
  // exactly that rather than blaming the model.
  await expect(roles.getByText(/Ollama is not running|not installed|no answer in time|no model chosen/).first()).toBeVisible();

  // The library offers a custom download behind Add model. What the curated
  // list shows depends on which models this machine already holds, so the
  // stable assertion is the custom path.
  const library = page.getByRole("region", { name: "Model library" });
  await library.getByLabel("Add model").click();
  await expect(library.getByLabel("Custom model name")).toBeVisible();
  await expect(library.getByLabel("Download the custom model")).toBeDisabled();
});

test("reflects the operating system's provider-key support", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings" }).click();

  const keys = page.getByRole("region", { name: "Provider keys" });
  const os = await page.locator("html").getAttribute("data-os");
  const store = os === "windows" ? "Windows Credential Manager" : os === "darwin" ? "macOS Keychain" : "";
  if (!store) {
    const name = os === "darwin" ? "macOS" : os === "linux" ? "Linux" : os;
    await expect(keys).toContainText(`Provider key storage is unavailable on ${name}.`);
    await expect(page.getByLabel("Save exa")).toBeDisabled();
    return;
  }

  await expect(keys).toContainText(`Keys are held by ${store}.`);

  const field = page.getByLabel("exa key");
  await expect(field).toHaveAttribute("type", "password");
  await expect(field).toBeEnabled();
  await expect(page.getByLabel("Save exa")).toBeEnabled();
});
