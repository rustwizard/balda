import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [tailwindcss(), svelte()],
  server: {
    host: true,
    port: 5173,
    proxy: {
      '/balda/api/v1': {
        target: process.env.BALDA_API_PROXY_URL || 'http://127.0.0.1:9666',
        changeOrigin: true,
      },
      '/api': {
        target: process.env.BALDA_CENTRIFUGO_PROXY_URL || 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '/connection/websocket': {
        target: process.env.BALDA_CENTRIFUGO_PROXY_URL || 'http://127.0.0.1:8000',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
