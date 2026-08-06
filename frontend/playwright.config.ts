import { defineConfig, devices } from "@playwright/test";
import path from "node:path";

const PORT = 8080;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
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
    },
    url: `http://localhost:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
