import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// Build into web/dist so the Go binary can embed it (see web/embed.go).
// In dev, proxy /api to a locally running edp-manager.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: "dist", emptyOutDir: true },
  server: {
    proxy: {
      "/api": { target: "http://localhost:9090", changeOrigin: true },
    },
  },
});
