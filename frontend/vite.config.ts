import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // Mirrors what Caddy will do in production: anything under /api goes
    // to the Go backend, everything else is the SPA. Keeps dev and prod
    // routing behavior consistent instead of diverging.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
