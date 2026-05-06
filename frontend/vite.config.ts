import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/web/',
  server: {
    port: 5442,
    proxy: {
      '/api': 'http://localhost:8585',
      '/ws': {
        target: 'ws://localhost:8585',
        ws: true,
      },
    },
  },
})
