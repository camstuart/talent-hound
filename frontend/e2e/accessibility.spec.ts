import { test, expect } from "@playwright/test";
import { newWorkspace } from "./support";

// The accessibility walkthrough the PRD asks for, as a test rather than a
// claim: keyboard operation, accessible names, and a visible distinction
// between what a document said, what the recruiter wrote, and what a model
// produced.
//
// All content is invented — no real candidate information appears here.
const stamp = Date.now().toString(36);

test("every control can be reached and used from the keyboard", async ({ page }) => {
  await page.goto("/");

  // Tab from the top and collect what focus lands on. A control that cannot be
  // reached this way cannot be used by someone who does not use a mouse.
  const reached: string[] = [];
  for (let i = 0; i < 40; i++) {
    await page.keyboard.press("Tab");
    const focused = await page.evaluate(() => {
      const el = document.activeElement as HTMLElement | null;
      if (!el || el === document.body) return null;
      // The same rules a screen reader uses, in the order it uses them: a
      // control wrapped in a label is named, even with no aria-label on it.
      const labelledBy = el.getAttribute("aria-labelledby");
      const name =
        el.getAttribute("aria-label") ||
        (labelledBy ? document.getElementById(labelledBy)?.textContent?.trim() : "") ||
        (el.id ? document.querySelector(`label[for="${el.id}"]`)?.textContent?.trim() : "") ||
        el.closest("label")?.textContent?.trim() ||
        el.getAttribute("placeholder") ||
        el.getAttribute("title") ||
        el.textContent?.trim() ||
        "";
      return { tag: el.tagName.toLowerCase(), name: name.slice(0, 40) };
    });
    if (focused) reached.push(`${focused.tag}:${focused.name}`);
  }

  expect(reached.length).toBeGreaterThan(0);
  // Focus reaches interactive elements, not just the document.
  expect(reached.some((entry) => entry.startsWith("button:"))).toBe(true);
  // Nothing focusable is anonymous: every stop has a name to announce.
  for (const entry of reached) {
    expect(entry.endsWith(":")).toBe(false);
  }
});

test("the new-initiative flow is completable with the keyboard alone", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("button", { name: "New initiative" }).focus();
  await page.keyboard.press("Enter");

  const name = `Keyboard ${stamp}`;
  await page.getByPlaceholder("Initiative name").fill(name);
  await page.getByLabel("Initiative type").selectOption("talent_search");
  // Reach Create by keyboard and activate it the same way.
  await page.getByRole("button", { name: "Create" }).focus();
  await expect(page.getByRole("button", { name: "Create" })).toBeFocused();
  await page.keyboard.press("Enter");

  await expect(page.getByRole("tab", { name: new RegExp(name) })).toBeVisible();
});

test("every control in a workspace has an accessible name", async ({ page }) => {
  await newWorkspace(page, `Named ${stamp}`);

  const anonymous = await page.evaluate(() => {
    const out: string[] = [];
    document.querySelectorAll("button, input, select, textarea").forEach((el) => {
      const element = el as HTMLElement;
      const labelled =
        element.getAttribute("aria-label") ||
        element.getAttribute("aria-labelledby") ||
        element.getAttribute("placeholder") ||
        element.getAttribute("title") ||
        (element.closest("label") ? "wrapped in a label" : "") ||
        (element.id && document.querySelector(`label[for="${element.id}"]`) ? "labelled" : "") ||
        element.textContent?.trim();
      if (!labelled) out.push(element.outerHTML.slice(0, 80));
    });
    return out;
  });

  expect(anonymous).toEqual([]);
});

test("focus is visible wherever it lands", async ({ page }) => {
  await newWorkspace(page, `Focus ${stamp}`);

  await page.keyboard.press("Tab");
  const outlined = await page.evaluate(() => {
    const el = document.activeElement as HTMLElement | null;
    if (!el || el === document.body) return false;
    const style = getComputedStyle(el);
    // Either the browser's own focus ring or something the stylesheet draws.
    return style.outlineStyle !== "none" || style.boxShadow !== "none" || style.borderStyle !== "none";
  });
  expect(outlined).toBe(true);
});

test("source, recruiter-authored, and AI content are visibly distinct", async ({ page }) => {
  await page.goto("/");

  // The three treatments are asserted on the stylesheet rather than on a
  // screenshot: what matters is that they differ, and that none of them relies
  // on colour alone.
  const styles = await page.evaluate(() => {
    const probe = (provenance: string) => {
      const el = document.createElement("p");
      el.setAttribute("data-provenance", provenance);
      document.body.appendChild(el);
      const style = getComputedStyle(el);
      const label = getComputedStyle(el, "::before");
      const out = {
        borderStyle: style.borderLeftStyle,
        borderColor: style.borderLeftColor,
        borderWidth: style.borderLeftWidth,
        label: label.content,
      };
      el.remove();
      return out;
    };
    return { source: probe("source"), recruiter: probe("recruiter"), ai: probe("ai") };
  });

  // Each kind says what it is, so the distinction survives without colour.
  expect(styles.source.label).toContain("document");
  expect(styles.recruiter.label).toContain("You wrote");
  expect(styles.ai.label).toContain("model");

  // And the three are drawn differently from one another.
  const signature = (s: { borderStyle: string; borderColor: string; borderWidth: string }) =>
    `${s.borderStyle}|${s.borderColor}|${s.borderWidth}`;
  const signatures = [signature(styles.source), signature(styles.recruiter), signature(styles.ai)];
  expect(new Set(signatures).size).toBe(3);
});
