import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: parseInt(process.env.VITE_PORT || '5173'),
    proxy: {
      '/api': {
        target: `http://localhost:${process.env.VITE_GO_PORT || '8080'}`,
        changeOrigin: true,
      },
    },
  },
})
