import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://localhost:6174',
      '/health': 'http://localhost:6174',
    },
  },
  build: {
    outDir: 'dist',
  },
})
