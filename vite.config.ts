import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: { outDir: "internal/web/dist", emptyOutDir: true },
  server: {
    proxy: Object.fromEntries(
      ["/api", "/hls", "/recordings", "/healthz"].map((path) => [
        path,
        "http://127.0.0.1:40773",
      ]),
    ),
  },
  test: {
    environment: "jsdom",
    setupFiles: ["src/test/setup.ts"],
    include: ["src/**/*.test.{ts,tsx}"],
    restoreMocks: true,
  },
});
