import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const backend = 'http://localhost:8080'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/geofences': { target: backend, changeOrigin: true },
      '/vehicles': { target: backend, changeOrigin: true },
      '/alerts': { target: backend, changeOrigin: true },
      '/violations': { target: backend, changeOrigin: true },
      '/health': { target: backend, changeOrigin: true },
      '/ws': { target: 'ws://localhost:8080', ws: true },
    },
  },
})
