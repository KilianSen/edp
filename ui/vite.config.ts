import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// During `npm run dev`, point the SPA at a local edp and proxy /api + /hooks so
// the browser stays same-origin (no CORS needed in dev). In production the static
// build talks cross-origin to whatever host public/config.js names.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      "/api": { target: process.env.VITE_EDP_API || "http://localhost:8080", changeOrigin: true },
      "/hooks": { target: process.env.VITE_EDP_API || "http://localhost:8080", changeOrigin: true },
    },
  },
});
