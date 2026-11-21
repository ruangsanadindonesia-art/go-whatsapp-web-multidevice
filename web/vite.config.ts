import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

// The SPA is embedded into the GOWA binary (src/ui/webapp) and served at
// {APP_BASE_PATH}/panel/. Assets use relative URLs so the same build works
// under any base path; the router reads the real base from <base> at runtime.
export default defineConfig({
  base: './',
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../src/ui/webapp/dist',
    emptyOutDir: true,
    assetsDir: 'assets',
  },
  server: {
    proxy: {
      // Dev: run `go run . rest` on :3000, then `npm run dev`.
      '^/(?!panel/|@|src/|node_modules/).*': { target: 'http://localhost:3000', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
  },
})
