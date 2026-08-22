import { defineConfig, devices } from "@playwright/test";
import path from "node:path";
import { resetDatabase } from "./e2e/reset-db";

const PORT = 8080;

// Before anything else, and in particular before the web server starts.
//
// Only in the runner process. Playwright loads this config again in every
// worker, so an unguarded call here empties the database four more times while
// the server is serving from it — which leaves an empty file and fails most of
// the suite. Workers get TEST_WORKER_INDEX; the runner does not.
if (process.env.TEST_WORKER_INDEX === undefined) resetDatabase();

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  // Every assertion in this suite waits on a real Go backend over a real
  // transport, so the meaningful failure is "it never happened", not "it took
  // longer than five seconds". Most waits here already say 15_000 one at a
  // time; the two creation helpers did not, so a machine busy enough to slow a
  // round-trip failed there and nowhere else, which reads as a flaky suite
  // rather than a loaded one. Said once, here, instead.
  expect: { timeout: 15_000 },
  // Playwright's per-test default is thirty seconds, and a test here is several
  // real backend round-trips in sequence: drop a résumé, build a profile,
  // approve it, drop another, watch it go stale. The slowest takes fifteen
  // seconds on an idle machine, which leaves no headroom at all — so on a busy
  // one it fails for having been given half the time it needs rather than for
  // anything being wrong.
  timeout: 90_000,
  use: {
    baseURL: `http://localhost:${PORT}`,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "wails3 task run:server DEV=true",
    cwd: "..",
    env: {
      // Keep E2E data out of the user's real database.
      TALENT_HOUND_DB_PATH: path.resolve(".e2e-db/e2e.db"),
      // First-run settings live outside the data folder, so an E2E run gets its
      // own rather than reading or writing the developer's.
      TALENT_HOUND_CONFIG_DIR: path.resolve(".e2e-db/config"),
    },
    url: `http://localhost:${PORT}`,
    // Not reused. The run starts by emptying the database, and a server left
    // over from a previous run is holding the file that gets emptied — which
    // fails thirty-five specs rather than one, and does it for a reason that
    // has nothing to do with the product.
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
