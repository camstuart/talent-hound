import { test, expect } from "@playwright/test";

// Unique per run: the E2E database persists across local runs, so every record
// is named for this run. All names are invented.
const stamp = Date.now();

test("creates records through the real backend and persists them across a restart", async ({ page }) => {
  const company = `Northwind Robotics ${stamp}`;
  const contact = `Dana Okafor ${stamp}`;
  const role = `Staff Engineer ${stamp}`;
  await page.goto("/");

  // A workspace is needed to reach the Context area.
  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(`Records ${stamp}`);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  await page.getByRole("button", { name: "Create" }).click();

  const companyForm = page.getByRole("form", { name: "New company" });
  await companyForm.getByLabel("Name *").fill(company);
  await companyForm.getByLabel("Website").fill("https://northwind.test");
  await companyForm.getByRole("button", { name: "Add company" }).click();
  await expect(page.getByRole("region", { name: "Companies" }).getByText(company)).toBeVisible();

  const contactForm = page.getByRole("form", { name: "New contact" });
  await contactForm.getByLabel("Company").selectOption({ label: company });
  await contactForm.getByLabel("Full name *").fill(contact);
  await contactForm.getByLabel("Role or title").fill("Head of Engineering");
  await contactForm.getByRole("button", { name: "Add contact" }).click();

  const roleForm = page.getByRole("form", { name: "New role" });
  await roleForm.getByLabel("Title *").fill(role);
  await roleForm.getByLabel("Company name").fill(company);
  await roleForm.getByRole("button", { name: "Add role" }).click();
  await expect(page.getByRole("region", { name: "Roles" }).getByText(role)).toBeVisible();

  // Contacts-at-company counts only that company's people.
  await page.getByLabel("Contacts at company").selectOption({ label: company });
  await expect(page.getByTestId("contacts-count")).toHaveText("1 known contact");
  await expect(page.getByText(contact)).toBeVisible();

  // Records are shared, not owned by the initiative: they survive a reload.
  await page.reload();
  await page.locator(".sidebar").getByRole("button", { name: `Records ${stamp}` }).click();
  await expect(page.getByRole("region", { name: "Companies" }).getByText(company)).toBeVisible();
  await expect(page.getByRole("region", { name: "Roles" }).getByText(role)).toBeVisible();
});

test("shows the backend's validation message against the field that failed", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "New initiative" }).click();
  await page.getByPlaceholder("Initiative name").fill(`Validation ${stamp}`);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  await page.getByRole("button", { name: "Create" }).click();

  const companyForm = page.getByRole("form", { name: "New company" });
  await companyForm.getByLabel("Name *").fill(`Bad website ${stamp}`);
  await companyForm.getByLabel("Website").fill("northwind.test");
  await companyForm.getByRole("button", { name: "Add company" }).click();

  await expect(companyForm.getByText(/absolute http or https URL/)).toBeVisible();
  await expect(companyForm.getByLabel("Website")).toHaveAttribute("aria-invalid", "true");
  // Nothing was created, and what was typed is still there.
  await expect(page.getByRole("region", { name: "Companies" }).getByText(`Bad website ${stamp}`)).toHaveCount(0);
  await expect(companyForm.getByLabel("Name *")).toHaveValue(`Bad website ${stamp}`);
});
