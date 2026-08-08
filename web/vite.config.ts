import { fileURLToPath, URL } from 'node:url'

import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
// Vitest's defineConfig, not Vite's: same config object, plus the `test` key.
import { defineConfig } from 'vitest/config'

// Where `npm run dev` forwards /api. The Go server's default listen address
// (BLACKLIGHT_ADDR in .env.example); override for a server on another port.
const apiTarget = process.env.BLACKLIGHT_DEV_PROXY_TARGET ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [react(), tailwindcss()],

  resolve: {
    alias: {
      // Mirrored in tsconfig.app.json's `paths`. shadcn/ui writes imports in
      // this form, so both resolvers have to agree.
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },

  server: {
    // Proxying rather than enabling CORS is deliberate: in production the SPA
    // is served by the Go binary itself (M0B-010), so requests are same-origin
    // there. Making dev same-origin too means cookie flags, CSRF headers and
    // SameSite behave identically in both, and no CORS configuration exists to
    // get wrong. The path is passed through unrewritten — the server mounts the
    // API at /api/v1.
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: false,
      },
    },
  },

  build: {
    // Read by the Go binary's embed.FS in M0B-010.
    outDir: 'dist',
    sourcemap: true,
  },

  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
