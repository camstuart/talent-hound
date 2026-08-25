import { test, expect } from "@playwright/test";
import { newRole, newWorkspace } from "./support";

// Unique per run: specs run in parallel against one shared backend. All
// content is invented — no real person is named, and no request reaches a
// real provider: the backend has no Exa or GitHub key in this environment, so
// it refuses rather than sending.
const stamp = Date.now();
// Base-36 so the token has no long run of digits: seven or more digits in a
// row is phone-shaped, and the scrubber removes it on purpose.
const tag = `wombatstack${stamp.toString(36)}`;

test.describe.configure({ mode: "serial" });

test("a people query is built from the role's profile, edited, cancelled, and refused without a key", async ({ page }) => {
  await newWorkspace(page, `Sourcing ${stamp}`);
  const title = `Platform engineer ${stamp}`;
  await newRole(page, title);

  // A people query is built from a ready role profile. This one is completed
  // by hand, which is what a recruiter does when there is no listing to read.
  await page.getByRole("tab", { name: "Research" }).click();
  const profiles = page.getByRole("region", { name: "Role profiles" });
  await profiles.getByLabel(`Requirement to add to ${title}`).fill(`Must have production Go and ${tag} experience`);
  await profiles.getByLabel(`Add a requirement to ${title}`).click();
  await expect(profiles.getByLabel(`Requirement 1 of ${title}`, { exact: true })).toContainText(tag, { timeout: 15_000 });

  const panel = page.getByRole("region", { name: "Find people" });
  await panel.getByLabel("Search for role").selectOption({ label: title });
  await panel.getByLabel("Build a people query").click();
  const field = panel.getByLabel("People query to send");
  await expect(field).toHaveValue(new RegExp(tag), { timeout: 15_000 });

  // Editing is allowed and what is in the box is what would go.
  await field.fill(`Go engineers in Melbourne ${tag}`);
  await expect(field).toHaveValue(`Go engineers in Melbourne ${tag}`);

  // Cancelling is the absence of the operation: nothing was recorded.
  await panel.getByLabel("Cancel this people search").click();
  await expect(panel.getByLabel("People query to send")).toBeHidden();
  await expect(panel.getByRole("list", { name: "Past people searches" })).toContainText("No people searches yet.");

  // Sending with no key stored is refused in the backend's own words, and the
  // attempt is recorded with its query — the recruiter pressed send.
  await panel.getByLabel("Build a people query").click();
  await expect(panel.getByLabel("People query to send")).toBeVisible({ timeout: 15_000 });
  await panel.getByLabel("Send this people search").click();
  await expect(panel.getByText(/no search credential is stored|could not be reached|not answer in time/)).toBeVisible({
    timeout: 20_000,
  });
  await expect(panel.getByLabel("People query sent for search 1")).toContainText(tag, { timeout: 15_000 });
  await expect(panel.getByRole("list", { name: "Leads" })).toContainText("No leads yet.");
});

test("a candidate's identities are managed in the CRM and enrichment waits for a token", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("CRM", { exact: true }).click();

  const name = `Sam Quokka ${stamp}`;
  await page.getByRole("button", { name: "New candidate", exact: true }).click();
  const createForm = page.getByRole("form", { name: "New candidate" });
  await createForm.getByLabel("Full name *").fill(name);
  await createForm.getByRole("button", { name: "Add candidate" }).click();
  await page.getByRole("list", { name: "Records" }).getByText(name, { exact: true }).click();

  const identities = page.getByRole("region", { name: "Identities" });
  await expect(identities.getByLabel("Identity list")).toContainText("No identities recorded.");
  // No GitHub identity yet, so enrichment says why it will not run.
  await expect(identities.getByLabel("Enrich from GitHub")).toBeDisabled();
  await expect(identities.getByLabel("Enrich unavailable reason")).toContainText("no GitHub identity");

  // The handle is parsed from the URL, lowercased, and shown.
  await identities.getByLabel("Identity URL").fill(`https://github.com/Quokka${tag}`);
  await identities.getByLabel("Add this identity").click();
  await expect(identities.getByLabel("Identity list")).toContainText(`github: quokka${tag}`, { timeout: 15_000 });

  // With a handle but no token, the reason changes and it still will not run.
  await expect(identities.getByLabel("Enrich from GitHub")).toBeDisabled();
  await expect(identities.getByLabel("Enrich unavailable reason")).toContainText("no GitHub token");

  // A duplicate handle belongs to someone already, and the backend says so.
  await identities.getByLabel("Identity URL").fill(`https://github.com/quokka${tag}/`);
  await identities.getByLabel("Add this identity").click();
  await expect(identities.getByRole("alert")).toContainText("already belongs", { timeout: 15_000 });

  await identities.getByLabel(`Remove github quokka${tag}`).click();
  await expect(identities.getByLabel("Identity list")).toContainText("No identities recorded.", { timeout: 15_000 });
});
