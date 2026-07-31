const { defineConfig, devices } = require("@playwright/test");

const port = 18081;
const baseURL = `http://127.0.0.1:${port}`;

module.exports = defineConfig({
  testDir: "e2e",
  globalSetup: "./e2e/global-setup.js",
  fullyParallel: true,
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: "node e2e/start-server.js",
    url: `${baseURL}/healthz`,
    timeout: 30_000,
    reuseExistingServer: false,
  },
});
