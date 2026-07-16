import vue from "@vitejs/plugin-vue";
import { fileURLToPath } from "url";
import { defineConfig } from "vite";
import AutoImport from "unplugin-auto-import/vite";
import Components from "unplugin-vue-components/vite";
import { TDesignResolver } from "unplugin-vue-components/resolvers";

const __dirname = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  root: __dirname,
  plugins: [
    {
      name: "desktop-html",
      transformIndexHtml(html) {
        // Replace Cloud entry with Desktop entry
        return html.replace(
          '/src/apps/cloud/main.ts',
          '/src/apps/desktop/main.ts'
        )
      },
    },
    vue(),
    AutoImport({
      resolvers: [TDesignResolver({ library: "vue-next" })],
    }),
    Components({
      resolvers: [TDesignResolver({ library: "vue-next" })],
    }),
  ],
  server: {
    host: "127.0.0.1",
    port: 5174,
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist-desktop",
    emptyOutDir: true,
    rollupOptions: {
      input: fileURLToPath(new URL("index.desktop.html", import.meta.url)),
      external: ["@tauri-apps/api/core"],
    },
  },
});
