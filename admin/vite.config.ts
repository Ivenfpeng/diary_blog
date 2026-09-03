import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  base: '/admin/',
  plugins: [vue()],
  test: { environment: 'jsdom' },
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080' },
  },
})
