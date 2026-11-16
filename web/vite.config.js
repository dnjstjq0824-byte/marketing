import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev proxy forwards /api to the Go server on :8080.
// Production: `npm run build` emits web/dist, served by the Go server itself.
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
