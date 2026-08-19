import { test, expect } from "@playwright/test";

// Help is read when the rest of the application is the problem, so these run
// against the real backend with no initiative open, no model assigned, and
// nothing indexed.

test("help is reachable without an initiative and lists every topic", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Help" }).click();

  const help = page.getByRole("region", { name: "Help" });
  await expect(help).toBeVisible();

  const index = help.getByLabel("Help topics");
  await expect(index).toContainText("First steps");
  await expect(index).toContainText("Tutorial");
  await expect(index).toContainText("Deleting things");
});

test("the tutorial walks the flagship loop in order", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Help" }).click();
  await page.getByLabel("Open Tutorial — a candidate from document to draft").click();

  const article = page.getByRole("article", { name: "Help article" });
  await expect(article).toContainText("Create an initiative");
  await expect(article).toContainText("Approve a profile");
  await expect(article).toContainText("Build a shortlist");
  await expect(article).toContainText("copy out a draft");

  // Each step says what the recruiter decides and what the application does.
  await expect(article).toContainText("You decide");
  await expect(article).toContainText("The application does");
});

test("searching answers from the shipped manual with no model assigned", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Help" }).click();

  await page.getByLabel("What do you need help with").fill("why can't I delete a candidate");
  await page.getByLabel("Search the help").click();

  const results = page.getByLabel("Search results");
  await expect(results).toContainText("Deleting");
  // No model is assigned in this environment, and the panel says so rather
  // than showing an empty answer.
  await expect(page.getByLabel("Why there is no written answer")).toBeVisible();
  await expect(page.getByLabel("Written answer", { exact: true })).toHaveCount(0);
});

test("a result opens the page it came from", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Help" }).click();
  await page.getByLabel("What do you need help with").fill("encrypted volume");
  await page.getByLabel("Search the help").click();

  const first = page.getByLabel("Search results").getByRole("button").first();
  await first.click();
  await expect(page.getByRole("article", { name: "Help article" })).toContainText("encrypt");
});

test("help says plainly when the manual has no answer", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("button", { name: "Help" }).click();
  await page.getByLabel("What do you need help with").fill("zzzqqxx");
  await page.getByLabel("Search the help").click();

  await expect(page.getByLabel("Search results")).toContainText("Nothing in the manual matches");
});
