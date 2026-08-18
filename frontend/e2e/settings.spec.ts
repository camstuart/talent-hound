import { test, expect } from "@playwright/test";

// Unique per run: the E2E database persists across local runs. Everything here
// is local configuration — no candidate content and no real provider key.
const stamp = Date.now();

test("configures a model role through the real backend and reports its availability", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings" }).click();

  const roles = page.getByRole("region", { name: "Model roles" });
  await expect(roles.getByText(/embed/).first()).toBeVisible();

  const model = `synthetic-model-${stamp}`;
  await page.getByLabel("Model for embed").fill(model);
  await page.getByLabel("Assign a model to embed").click();

  // Recorded with its revision, unvalidated, and checked against the endpoint —
  // which is not running in the test environment, so the state says exactly
  // that rather than blaming the model.
  await expect(roles.getByText(new RegExp(`${model}.*revision \\d+, unvalidated`))).toBeVisible();
  await expect(roles.getByText(/Ollama is not running|not installed|no answer in time/).first()).toBeVisible();
});

test("shows a provider key as unstored and masks the entry field", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Settings" }).click();

  const keys = page.getByRole("region", { name: "Provider keys" });
  await expect(keys.getByText(/no key stored/).first()).toBeVisible();

  const field = page.getByLabel("exa key");
  await expect(field).toHaveAttribute("type", "password");

  // Storing needs the Windows credential store; everywhere else the backend
  // refuses in its own words rather than falling back to a file.
  await field.fill("not-a-real-key-E2E");
  await page.getByLabel("Save exa").click();
  // On Windows the key lands in Credential Manager; everywhere else the panel
  // shows the refusal. Either way it is the backend's own words.
  await expect(
    page.getByText(/no supported credential store on this platform/)
      .or(keys.getByText(/— key stored/)),
  ).toBeVisible();
});
