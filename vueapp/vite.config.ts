import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  root: fileURLToPath(new URL('.', import.meta.url)),
  plugins: [vue()],
  server: {
    proxy: {
      '/room': 'http://127.0.0.1:4000',
      '/user': 'http://127.0.0.1:4000',
      '/whip': 'http://127.0.0.1:4000',
      '/whep': 'http://127.0.0.1:4000'
    }
  },
  build: {
    outDir: fileURLToPath(new URL('../static/dist-vue', import.meta.url)),
    emptyOutDir: true
  }
})
