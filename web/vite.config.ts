import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import { writeFileSync } from 'fs'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

// 构建时生成 version.json，前端定期轮询比对，版本不一致时提示用户刷新
function versionPlugin(): Plugin {
  const version = Date.now().toString(36)
  let outDir = ''
  return {
    name: 'version-plugin',
    config() {
      return {
        define: {
          __APP_VERSION__: JSON.stringify(version),
        },
      }
    },
    configResolved(config) {
      outDir = resolve(config.root, config.build.outDir)
    },
    closeBundle() {
      writeFileSync(
        resolve(outDir, 'version.json'),
        JSON.stringify({ version }),
      )
    },
  }
}

export default defineConfig({
  plugins: [
    vue(),
    versionPlugin(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
      imports: ['vue', 'vue-router', 'pinia'],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: 'src/components.d.ts',
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    rollupOptions: {
      output: {
        // 把体积大且更新频率低的依赖拆成独立 chunk，
        // 让业务代码发版时这些 chunk 的浏览器缓存仍然有效
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia'],
          vueflow: [
            '@vue-flow/core',
            '@vue-flow/background',
            '@vue-flow/controls',
            '@vue-flow/minimap',
          ],
          // 图表库只在报表页用到，单独成块，不进首屏
          echarts: ['echarts/core', 'echarts/charts', 'echarts/components', 'echarts/renderers', 'vue-echarts'],
        },
      },
    },
  },
  server: {
    port: 3100,
    host: '0.0.0.0', // 允许外部访问（Docker 需要）
    strictPort: false, // 端口被占用时自动尝试下一个
    proxy: {
      // 键要带结尾斜杠：写成 '/api' 会把前端路由 /api-docs 也代理给后端，
      // 直接访问或刷新那个页面就是后端的 404（从侧边栏点进去不刷新，看不出来）。
      // 前端所有请求都是 /api/v1/... ，带斜杠不影响。
      '/api/': {
        // 优先用显式指定的后端地址（k8s 里是 td-backend:10010），
        // 其次 Docker Compose 的服务名，最后本地
        target:
          process.env.VITE_PROXY_TARGET ||
          (process.env.DOCKER_ENV ? 'http://backend:10010' : 'http://localhost:10010'),
        changeOrigin: true,
        ws: true, // 启用 WebSocket 代理
      },
    },
    // 经 Ingress 访问 dev server 时，浏览器连的是 80 端口，
    // 而 vite 默认让 HMR 客户端去连 server.port（3100），连不上就没有热更新。
    hmr: process.env.VITE_HMR_CLIENT_PORT
      ? { clientPort: Number(process.env.VITE_HMR_CLIENT_PORT) }
      : true,
    // vite 5.4.12+ 会拦掉 Host 头不在白名单里的请求，
    // 用域名（td.kerbos.test）访问时必须显式放行
    allowedHosts: process.env.VITE_ALLOWED_HOSTS
      ? process.env.VITE_ALLOWED_HOSTS.split(',').filter(Boolean)
      : undefined,
    watch: {
      // Docker / hostPath 挂载下 inotify 事件传不出来，只能轮询。
      // 但必须显式排除 node_modules —— 那是几万个文件，全量轮询会把
      // 内存吃爆（实测 2Gi 限额下 20 秒被 OOMKilled）。
      usePolling: true,
      interval: 500,
      binaryInterval: 1500,
      ignored: ['**/node_modules/**', '**/dist/**', '**/.git/**'],
    },
  },
})
