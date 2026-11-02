import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/admin/',
  plugins: [vue()],
  test: {
    environment: 'jsdom',
    // Keep local and release-gate runs one-shot even when launched from a TTY.
    watch: false,
  },
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080' },
  },
})
