import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { examplePreviewPlugin } from './example/preview.mjs'

const webRoot = fileURLToPath(new URL('.', import.meta.url))

export default defineConfig({
  root: webRoot,
  appType: 'mpa',
  publicDir: false,
  plugins: [vue(), examplePreviewPlugin({ webRoot, workspaceRoot: process.env.WT_MEDIA_WORKSPACE_ROOT })],
  server: { host: '127.0.0.1', port: 8768, strictPort: true },
  preview: { host: '127.0.0.1', port: 8768, strictPort: true },
  build: { outDir: 'dist-example', emptyOutDir: true, rollupOptions: { input: fileURLToPath(new URL('./example/index.html', import.meta.url)) } },
})
