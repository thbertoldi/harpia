import path from "node:path";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      $lib: path.resolve("src/lib"),
    },
  },
  test: {
    exclude: ["**/node_modules/**", "**/e2e/**"],
  },
});
