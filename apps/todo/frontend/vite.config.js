import path from "node:path";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const root = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@cogs/brand': path.resolve(root, '../../../docs/brand/react'),
    },
  },
  root,
  publicDir: path.join(root, "static"),
  build: {
    outDir: process.env.COGS_VITE_OUT_DIR
      ? path.resolve(process.env.COGS_VITE_OUT_DIR)
      : path.resolve(root, "../public"),
    emptyOutDir: !!process.env.COGS_VITE_OUT_DIR,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": { target: "http://127.0.0.1:8080", changeOrigin: true },
    },
  },
});
