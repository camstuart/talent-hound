import { defineConfig, devices } from "@playwright/test";
import path from "node:path";

const PORT = 8080;

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
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
