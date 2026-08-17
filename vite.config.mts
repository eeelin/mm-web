import { defineConfig } from "vite";

export default defineConfig({
  server: {
    allowedHosts: ["server.ruyi.homes"],
    proxy: {
      "/api": "http://127.0.0.1:8080",
    },
  },
});
