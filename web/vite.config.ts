import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

/** Block SPA fallback for /8charToken paths so browsers get 404 instead of empty shell. */
function noSpaForAccessTokens(): Plugin {
  const isToken = (seg: string) => /^[A-Za-z0-9]{8}$/.test(seg)
  return {
    name: 'no-spa-for-access-tokens',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const url = req.url?.split('?')[0] || ''
        const seg = url.replace(/^\//, '').split('/')[0] || ''
        if (seg && isToken(seg)) {
          // proxy docker paths to backend
          if (url.includes('/v2') || url.includes('/token')) {
            next()
            return
          }
          res.statusCode = 404
          res.setHeader('Content-Type', 'application/json; charset=utf-8')
          res.setHeader('X-Robots-Tag', 'noindex, nofollow, noarchive')
          res.end(
            JSON.stringify({
              error: '页面不存在',
              code: 'NOT_FOUND',
              hint: '此路径仅用于 Docker 镜像拉取，请勿在浏览器中打开',
            }),
          )
          return
        }
        next()
      })
    },
  }
}

export default defineConfig({
  plugins: [vue(), tailwindcss(), noSpaForAccessTokens()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: '../src/dist',
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:5000',
      '/ready': 'http://127.0.0.1:5000',
      '/v2': 'http://127.0.0.1:5000',
      '/token': 'http://127.0.0.1:5000',
    },
  },
})
