import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/admin': {
        target: 'http://8.136.58.109:8088',
        changeOrigin: true,
      },
    },
  },
})
