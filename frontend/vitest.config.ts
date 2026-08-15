// frontend/vitest.config.ts
import { defineConfig } from 'vitest/config'
import path from 'node:path'

// Separate from vite.config.ts (which loads server-only env vars via
// loadEnv per the root CLAUDE.md's Env config note) — tests don't need
// that, and keeping it out avoids test runs depending on a .env file
// existing. Re-declares the @/* alias so test imports match app code.
export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'node',
  },
})
