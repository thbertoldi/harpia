import { sveltekit } from "@sveltejs/kit/vite";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [sveltekit()],
  test: {
    exclude: ["**/node_modules/**", "**/e2e/**"],
    include: ["src/**/*.{test,spec}.{js,ts}"],
  },
});
