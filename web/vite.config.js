import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { TDesignResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      resolvers: [TDesignResolver({ library: 'vue-next' })],
    }),
    Components({
      resolvers: [TDesignResolver({ library: 'vue-next' })],
    }),
  ],
  server: {
    // 显式写出：harness 等这个端口，且没有 strictPort 时 Vite 被占就静默换端口，
    // harness 探到的会是别人的服务；隐式绑定还只绑 [::1]，127.0.0.1 连不上。
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': 'http://127.0.0.1:8188',
    },
  },
  test: {
    environment: 'node',
  },
})
