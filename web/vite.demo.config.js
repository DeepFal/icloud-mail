import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import { fileURLToPath } from "node:url";

// A separate, browser-only entry: this server has no API proxy or real account access.
export default defineConfig({
  base: "./",
  plugins: [vue()],
  server: {
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
    open: false,
    headers: { "Cache-Control": "no-store" },
  },
  preview: { host: "127.0.0.1", port: 5174, strictPort: true },
  build: {
    outDir: "dist/demo",
    rollupOptions: { input: fileURLToPath(new URL("./demo.html", import.meta.url)) },
  },
});
