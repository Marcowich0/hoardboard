import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The Go server listens on 8098 (HOARDBOARD_PORT). In `npm run dev`, Vite
// serves the app and forwards /api there.
const backend = 'http://localhost:8098'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: 'dist',
    // Built JS/CSS go under /static/. Their names carry a content hash, so the
    // server lets browsers keep them for a year. Files in public/ get no
    // hash, so none of them may go in public/static/.
    assetsDir: 'static',
  },
  server: {
    proxy: { '/api': backend },
  },
})
