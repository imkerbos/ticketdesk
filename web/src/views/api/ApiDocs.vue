<template>
  <div class="api-docs-container">
    <iframe
      v-if="ready"
      ref="frameRef"
      :src="swaggerUrl"
      class="api-docs-frame"
      :title="t('apiDocs.title')"
      @load="applyTheme"
    ></iframe>
    <div v-else class="api-docs-loading">
      <el-icon class="loading-spin"><Loading /></el-icon>
      <p>{{ t('apiDocs.loading') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import request from '@/utils/request'
import { useThemeStore } from '@/stores/theme'
import swaggerTheme from '@/styles/swagger-theme.css?raw'

const { t } = useI18n()

const router = useRouter()
const themeStore = useThemeStore()
const ready = ref(false)
const swaggerUrl = ref('')
const frameRef = ref<HTMLIFrameElement>()

// 需要传给 iframe 的 token。iframe 是同源的，但它有自己的 document，
// 拿不到父页面 :root 上的变量，所以这里读取计算值再写进去 ——
// 配色仍然只有一个来源，改 token 两边一起变。
const THEME_TOKENS = [
  'bg-page', 'bg-card', 'bg-section', 'code-bg',
  'border-color', 'border-color-dark', 'divider-color',
  'text-primary', 'text-regular', 'text-secondary', 'text-placeholder', 'text-white',
  'color-primary', 'color-success', 'color-warning', 'color-danger',
  'tag-primary-bg', 'tag-primary-text', 'tag-primary-border',
  'tag-success-bg', 'tag-success-text', 'tag-success-border',
  'tag-orange-bg', 'tag-orange-text', 'tag-orange-border',
  'tag-purple-bg', 'tag-purple-text', 'tag-purple-border',
  'tag-danger-bg', 'tag-danger-text', 'tag-danger-border',
  'input-bg', 'input-border', 'input-placeholder',
  'radius-xs', 'radius-sm', 'radius-md', 'radius-lg', 'radius-full',
  'space-1', 'space-2', 'space-3', 'space-4', 'space-5', 'space-6',
  'font-xs', 'font-sm', 'font-base', 'font-lg',
  'duration-fast', 'ease-out', 'elevation-4', 'ring-primary',
  'scrollbar-track', 'scrollbar-thumb',
]

const STYLE_ID = 'td-swagger-theme'

/**
 * 把主题注入 iframe。
 *
 * iframe 每次 load 都要重新注入（文档被换掉了），主题切换时也要重新注入
 * （token 值变了）。用固定 id 覆盖写入，不会越积越多。
 */
const applyTheme = () => {
  const doc = frameRef.value?.contentDocument
  // 跨域或还没 load 完时 contentDocument 为 null，静默跳过
  if (!doc?.head) return

  const rootStyle = getComputedStyle(document.documentElement)
  const vars = THEME_TOKENS
    .map((name) => `  --td-${name}: ${rootStyle.getPropertyValue(`--td-${name}`).trim()};`)
    .join('\n')

  let style = doc.getElementById(STYLE_ID)
  if (!style) {
    style = doc.createElement('style')
    style.id = STYLE_ID
    doc.head.appendChild(style)
  }
  style.textContent = `:root {\n${vars}\n}\n\n${swaggerTheme}`

  // 暗色下 swagger 有一批写死的深色，靠这个类名挂钩
  doc.documentElement.classList.toggle('td-dark', themeStore.isDark)
}

watch(() => themeStore.isDark, applyTheme)

onMounted(async () => {
  const token = localStorage.getItem('token')
  if (!token) {
    ElMessage.warning(t('apiDocs.loginFirst'))
    router.replace('/login')
    return
  }

  try {
    // 调后端用 JWT 鉴权设 HttpOnly cookie (path=/api/v1/swagger), 后续 swagger UI 静态资源请求自动带 cookie
    await request.post('/auth/swagger-session')
    swaggerUrl.value = '/api/v1/swagger/index.html'
    ready.value = true
  } catch {
    ElMessage.error(t('apiDocs.initFailed'))
  }
})
</script>

<style scoped lang="scss">
/* swagger 自带整页排版，外层不再给它留内边距：
   用负边距吃掉 .ap .content 的 20/24/24，让 iframe 贴满内容区。 */
.api-docs-container {
  width: 100%;
  height: calc(100vh - 52px); /* 52px = .ap .topbar 高度 */
  /* 抵消 .content 的内边距做满幅。跟着变量走 —— 写死 24px 的话，
     窄屏下 .content 变成 16px，两边各多撑 8px，页面主体就横滚了。 */
  margin: calc(var(--td-content-pad-top) * -1) calc(var(--td-content-pad-x) * -1)
    calc(var(--td-content-pad-bottom) * -1);
  background: var(--td-bg-page);
  display: flex;
  flex-direction: column;
}

.api-docs-frame {
  width: 100%;
  height: 100%;
  border: none;
  /* 不写死 #fff：暗色下 iframe 底色要跟着应用走，否则加载瞬间会闪一下白 */
  background: var(--td-bg-page);
}

.api-docs-loading {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 13px;
  color: var(--td-text-secondary);

  p {
    margin: 0;
  }

  .loading-spin {
    font-size: 20px;
    color: var(--td-text-placeholder);
    animation: spin 1s linear infinite;
  }
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@media (prefers-reduced-motion: reduce) {
  .api-docs-loading .loading-spin {
    animation: none;
  }
}
</style>
