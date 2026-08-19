import { defineConfig } from "vite";

export default defineConfig({
  server: {
    allowedHosts: ["server.ruyi.homes"],
    proxy: {
      "/api": process.env.MM_WEB_API_PROXY || "http://127.0.0.1:8080",
    },
  },
});
