import { test, expect } from "@playwright/test";

test("greets using the real Go backend", async ({ page }) => {
  await page.goto("/");

  await page.getByPlaceholder("Your name").fill("Playwright");
  await page.getByRole("button", { name: "Greet" }).click();

  await expect(page.getByText("Hello Playwright, welcome to Talent Hound")).toBeVisible();
});
