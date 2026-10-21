import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  use: { baseURL: "http://127.0.0.1:40774" },
  projects: [
    {
      name: "desktop",
      use: { browserName: "chromium", viewport: { width: 1440, height: 1000 } },
    },
    {
      name: "mobile",
      use: { browserName: "chromium", viewport: { width: 390, height: 844 } },
    },
  ],
  webServer: {
    command: "pnpm run build && bash scripts/test-server.sh",
    url: "http://127.0.0.1:40774",
    reuseExistingServer: false,
    timeout: 120000,
  },
});
