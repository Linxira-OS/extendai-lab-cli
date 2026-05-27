import { defineConfig } from "vitest/config";
import path from "path";

export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    include: ["packages/**/*.test.ts"],
    testTimeout: 10_000,
  },
  resolve: {
    alias: {
      "@extendai/kernel": path.resolve(__dirname, "packages/kernel/src"),
      "@extendai/plugin": path.resolve(__dirname, "packages/plugin/src"),
      "@extendai/agent": path.resolve(__dirname, "packages/agent/src"),
      "@extendai/tui": path.resolve(__dirname, "packages/tui/src"),
      "@extendai/cli": path.resolve(__dirname, "packages/cli/src"),
    },
  },
});
