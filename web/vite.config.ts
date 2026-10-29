import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    // The lazily loaded liveness chunk is ~1.1 MB.
    chunkSizeWarningLimit: 1300,
  },
  server: {
    port: 5173,
    // S3 CORS allows exactly this origin; don't silently move to 5174.
    strictPort: true,
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
