import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // loadEnv reads frontend/.env* — this runs in Node at config time, so
  // process.env isn't populated automatically the way client code's
  // import.meta.env is. '' as the third arg loads all vars, not just
  // VITE_-prefixed ones (fine here since nothing here ships to the browser).
  const env = loadEnv(mode, process.cwd(), '')

  return {
    plugins: [react()],
    server: {
      port: Number(env.FRONTEND_PORT) || 3000,
      // Mirrors what Caddy will do in production: anything under /api goes
      // to the Go backend, everything else is the SPA. Keeps dev and prod
      // routing behavior consistent instead of diverging.
      proxy: {
        '/api': env.API_PROXY_TARGET || 'http://localhost:8080',
      },
    },
  }
})
