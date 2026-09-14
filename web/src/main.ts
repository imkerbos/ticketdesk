import { createApp } from 'vue'
import { createPinia } from 'pinia'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
// 组件按需引入：由 vite.config.ts 里的 unplugin-vue-components + ElementPlusResolver
// 在编译期按实际使用注入组件与对应样式。
// 这里刻意不再 `import ElementPlus` 与全量 index.css ——
// 全量引入会把整个组件库塞进首屏主包（约 1.2MB JS + 388KB CSS），
// 同时让上面那套按需配置完全失效。
// ElMessage / ElMessageBox / ElNotification 是在脚本里显式 import 使用的，
// 其样式无法被模板扫描发现，因此在此单独引入。
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-message-box.css'
import 'element-plus/theme-chalk/el-notification.css'
import 'element-plus/theme-chalk/el-loading.css'
import 'element-plus/theme-chalk/dark/css-vars.css'

import { i18n } from './i18n'
import App from './App.vue'
import router from './router'
import './styles/theme.scss'
import './styles/tokens/_index.scss'
import './styles/index.scss'
import './styles/_table.scss'
import './styles/_form.scss'
import './styles/_components.scss'
// Apple 组件层：自带 DOM + 自带样式，收在 .ap 命名空间下，放最后
import './styles/_apple.scss'
import { useUserStore } from './stores/user'
import { useThemeStore } from './stores/theme'
import { useBrandStore } from './stores/brand'

// 空态组件，全站 40 多处在用，全局注册省掉每页一行 import
import TdEmptyState from '@/components/td/TdEmptyState.vue'

const app = createApp(App)
app.use(i18n)

// 注册所有 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

const pinia = createPinia()
app.use(pinia)

// 初始化用户状态（必须在 pinia 注册后）
const userStore = useUserStore()
userStore.initFromStorage()

// 初始化主题
const themeStore = useThemeStore()
themeStore.init()

// 加载品牌配置
const brandStore = useBrandStore()


app.use(router)

app.component('TdEmptyState', TdEmptyState)

// 等待品牌配置和路由初始导航完成后再挂载，避免闪屏。
// 初始化状态由路由守卫自己保证，不放在这里 —— 首次导航早于任何 then 回调。
Promise.all([brandStore.loadBrandConfig(), router.isReady()]).then(() => {
  app.mount('#app')
})
