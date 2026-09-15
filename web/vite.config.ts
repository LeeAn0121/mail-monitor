import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Dashboard is embedded into the Go binary and served from "/", with the Go
// server also owning /api/* — so in dev mode we proxy those calls to the Go
// process (run mail-monitor separately, listening on :8080) instead of
// duplicating backend logic in a mock server.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
