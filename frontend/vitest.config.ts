import { defineConfig } from "vitest/config";
import path from "node:path";

export default defineConfig({
  resolve: {
    alias: {
      $lib: path.resolve("./src/lib"),
    },
  },
  test: {
    exclude: ["**/node_modules/**", "**/e2e/**"],
  },
});
