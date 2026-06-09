import { sveltekit } from "@sveltejs/kit/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig, loadEnv, type Plugin } from "vite";

function rejectDevLoginInBuild(): Plugin {
  return {
    name: "harpia-dev-login-build-guard",
    config(_, { command, mode }) {
      if (command !== "build") return;

      const env = loadEnv(mode, process.cwd(), "PUBLIC_");
      if (env.PUBLIC_DEV_LOGIN_ENABLED === "true") {
        throw new Error(
          "PUBLIC_DEV_LOGIN_ENABLED=true enables the dev auth bypass and is only allowed with vite dev.",
        );
      }
    },
  };
}

export default defineConfig({
  plugins: [rejectDevLoginInBuild(), tailwindcss(), sveltekit()],
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/harpia": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://localhost:8080",
        ws: true,
      },
    },
  },
});
