import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

// En desarrollo, /api se reenvía al backend (sin CORS). En producción lo hace nginx (ver nginx.conf).
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '')
  return {
    plugins: [vue()],
    resolve: { alias: { "@": new URL("./src", import.meta.url).pathname } },
    server: {
      port: 5173,
      proxy: {
        '/api': {
          target: env.VITE_API_PROXY || 'http://localhost:8081',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api/, ''),
        },
      },
    },
  }
})
