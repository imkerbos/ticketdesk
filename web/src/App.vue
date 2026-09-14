<template>
  <!-- Element Plus 的内置文案（表格空态、分页、日期选择器等）跟随应用语言切换。
       之前完全没配，中文界面里混着 "No Data" / "Go to"；
       只配死中文又会在切到英文时出现"界面英文、组件中文"的割裂。 -->
  <el-config-provider :locale="elementLocale">
    <div id="app">
      <!-- 版本更新提示：固定在浏览器顶部 -->
      <div v-if="hasNewVersion" class="update-banner">
        <el-icon><RefreshRight /></el-icon>
        <span v-if="dismissed">{{ t('common.updated') }}</span>
        <span v-else>{{ t('common.newVersion', { n: countdown }) }}</span>
        <a class="update-banner-action" @click.stop="reloadPage">{{ t('common.reloadNow') }}</a>
        <template v-if="!dismissed">
          <span>{{ t('common.or') }}</span>
          <a class="update-banner-action update-banner-dismiss" @click.stop="dismiss">{{ t('common.dismissReload') }}</a>
          <span>）</span>
        </template>
      </div>

      <!-- 跳到主内容：键盘用户每换一页都要先按 20 次 Tab 才穿过侧边栏和顶栏。
           平时不占位，聚焦到才现身 —— 这是无障碍的标准做法，鼠标用户看不到。 -->
      <a v-if="!isAuthPage" class="skip-to-content" href="#main-content">
        {{ t('common.skipToContent') }}
      </a>

      <!-- 登录页面：独立全屏显示 -->
      <router-view v-if="isAuthPage" />

      <!-- 主布局：侧边栏 + 顶栏。
           这一层照着设计样张的 DOM 重写，不再用 el-container / el-menu ——
           那套组件的内部结构和默认样式要靠一层层 !important 去压，
           压不干净，组件一升级还会失效。原生标签 + .ap 命名空间下的样式最稳。 -->
      <div v-else class="ap" :class="{ 'has-update-banner': hasNewVersion }">
        <div v-if="mobileNavOpen" class="nav-scrim" @click="mobileNavOpen = false"></div>

        <aside class="sidebar" :class="{ 'is-open': mobileNavOpen }">
          <div class="brand">
            <span class="mark">
              <img v-if="brandStore.logoUrl" :src="brandStore.logoUrl" :alt="brandStore.systemName" />
              <svg v-else viewBox="0 0 24 24" aria-hidden="true"><path d="M4 5h16v4H4zM4 11h16v8H4z" /></svg>
            </span>
            {{ brandStore.systemName }}
          </div>

          <nav v-for="group in navGroups" :key="group.key" class="nav-group">
            <div v-if="group.label" class="nav-label">{{ t(`navGroup.${group.key}`) }}</div>

            <template v-for="item in group.items" :key="item.path || item.key">
              <button
                v-if="item.children"
                class="nav-item"
                :aria-expanded="!collapsed.has(item.key)"
                @click="toggleGroup(item.key)"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true" v-html="ICONS[item.icon]" />
                {{ t(item.labelKey) }}
                <svg class="chev" :class="{ 'is-collapsed': collapsed.has(item.key) }" viewBox="0 0 24 24" aria-hidden="true"><path d="m18 15-6-6-6 6" /></svg>
              </button>

              <button
                v-for="child in (collapsed.has(item.key) ? [] : item.children || [])"
                :key="child.path"
                class="nav-sub"
                :aria-current="activeMenu === child.path ? 'page' : undefined"
                @click="handleMenuSelect(child.path)"
              >
                {{ t(child.labelKey) }}
              </button>

              <button
                v-if="!item.children"
                class="nav-item"
                :aria-current="activeMenu === item.path ? 'page' : undefined"
                @click="go(item.path)"
              >
                <svg viewBox="0 0 24 24" aria-hidden="true" v-html="ICONS[item.icon]" />
                {{ t(item.labelKey) }}
                <span v-if="item.badge && notificationStore.unreadCount > 0" class="nav-badge">
                  {{ notificationStore.unreadCount > 99 ? '99+' : notificationStore.unreadCount }}
                </span>
              </button>
            </template>
          </nav>

          <div class="sidebar-foot">{{ brandStore.copyrightText }}</div>
        </aside>

        <div class="main">
          <header class="topbar">
            <button class="icon-btn nav-toggle" :aria-label="t('common.menu')" @click="mobileNavOpen = !mobileNavOpen">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16M4 12h16M4 17h16" /></svg>
            </button>

            <nav v-if="$route.meta.titleKey" class="crumb" :aria-label="t('common.breadcrumb')">
              {{ t(`navGroup.${currentNavGroup}`) }} / <b>{{ pageTitle }}</b>
            </nav>

            <div class="grow"></div>

            <button class="icon-btn" :aria-label="t('common.toDark')" @click="themeStore.toggleMode()">
              <svg v-if="themeStore.mode === 'light'" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="4" /><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" /></svg>
              <svg v-else-if="themeStore.mode === 'dark'" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5z" /></svg>
              <svg v-else viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="5" width="18" height="12" rx="2" /><path d="M8 21h8" /></svg>
            </button>

            <el-dropdown trigger="click" @command="handleLocaleChange">
              <button class="icon-btn" :aria-label="t('lang.label')"><span class="locale-code">{{ localeShort }}</span></button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item
                    v-for="l in SUPPORTED_LOCALES"
                    :key="l"
                    :command="l"
                    :class="{ 'is-active-locale': l === currentLocale }"
                  >
                    {{ t(`lang.${l}`) }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>

            <NotificationBell />

            <el-dropdown trigger="click">
              <div class="who">
                <span class="avatar">{{ userStore.displayName?.charAt(0) || 'U' }}</span>
                {{ userStore.displayName || t('common.userFallback') }}
              </div>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="router.push('/profile')">
                    <el-icon><User /></el-icon>
                    {{ t('common.personalSettings') }}
                  </el-dropdown-item>
                  <el-dropdown-item divided @click="handleLogout">
                    <el-icon><SwitchButton /></el-icon>
                    {{ t('nav.logout') }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </header>

          <main id="main-content" class="content" tabindex="-1">
            <TdLoadFailureBar />
            <router-view />
          </main>
        </div>
      </div>
    </div>
  </el-config-provider>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { elementLocaleMap, setLocale, SUPPORTED_LOCALES, type AppLocale } from '@/i18n'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import TdLoadFailureBar from '@/components/td/TdLoadFailureBar.vue'
import { useRoute, useRouter } from 'vue-router'
// 侧边栏和顶栏的图标改成内联 SVG（和设计样张同一套线形），
// 这里只留下拉菜单和更新横幅还在用的几个
import { User, SwitchButton, RefreshRight } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { updateCurrentUser } from '@/api/user'
import { useNotificationStore } from '@/stores/notification'
import { useThemeStore } from '@/stores/theme'
import { useBrandStore } from '@/stores/brand'
import { useVersionCheck } from '@/composables/useVersionCheck'
import NotificationBell from '@/components/NotificationBell.vue'

const route = useRoute()
const router = useRouter()

// 窄屏侧边栏开合。跳转后自动收起，否则点完菜单遮罩还压在新页面上。
const mobileNavOpen = ref(false)
watch(() => route.fullPath, () => { mobileNavOpen.value = false })
const { t, locale } = useI18n()

// 依赖 locale ref 才能在切换时重新求值——直接读 getLocale() 不是响应式的
const currentLocale = computed(() => locale.value as AppLocale)
const elementLocale = computed(() => elementLocaleMap[currentLocale.value])
const localeShort = computed(() => t('common.localeShort'))

// 页面标题统一走语言包 key
// 回落是刻意的——路由表里还有一批未抽取的标题，不该因此显示空白。
const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : ''
})

// 面包屑的第一级 = 侧边栏里该页所属的分组。
// 路由是平铺的（/issues、/alerts 都是顶层），matched 里拿不到父级，
// 所以按路径前缀映射回分组，和侧边栏的分法保持一致。
const NAV_GROUP_BY_PREFIX: Array<[string, string]> = [
  ['/projects', 'project'],
  ['/workflows', 'project'],
  ['/fields', 'project'],
  ['/notifications', 'project'],
  ['/requirement', 'project'],
  ['/alert', 'alert'],
  ['/reports', 'system'],
  ['/api-docs', 'system'],
  ['/users', 'system'],
  ['/settings', 'system'],
  ['/profile', 'system'],
]

// ── 侧边栏导航 ──────────────────────────────────────────
// 菜单从模板里的一长串 el-menu-item 改成数据驱动：
// 权限判断集中在一处，分组结构一眼能看全，也不用再和组件的插槽结构较劲。

const ICONS: Record<string, string> = {
  home: '<path d="M3 10.5 12 4l9 6.5V20a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z"/>',
  ticket: '<rect x="4" y="4" width="16" height="16" rx="3"/><path d="M8 9h8M8 13h5"/>',
  folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
  mail: '<path d="M4 6h16v12H4z"/><path d="m4 7 8 6 8-6"/>',
  doc: '<path d="M6 3h9l4 4v14H6z"/><path d="M14 3v5h5"/>',
  bell: '<path d="M18 15v-5a6 6 0 1 0-12 0v5l-2 3h16z"/><path d="M10 21h4"/>',
  chart: '<path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/>',
  api: '<path d="M6 4h9l4 4v12H6z"/><path d="M9 12h7M9 16h5"/>',
  gear: '<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M2 12h3M19 12h3M4.9 19.1 7 17M17 7l2.1-2.1"/>',
}

interface NavItem {
  key: string
  path?: string
  labelKey: string
  icon: string
  badge?: boolean
  children?: Array<{ path: string; labelKey: string }>
}

const navGroups = computed<Array<{ key: string; label: boolean; items: NavItem[] }>>(() => {
  const isProjectAdmin = userStore.isProjectAdmin
  const isAdmin = userStore.isAdmin

  const projectChildren = [
    { path: '/projects', labelKey: 'nav.projectList' },
    ...(isProjectAdmin ? [{ path: '/workflows', labelKey: 'nav.workflows' }] : []),
    ...(isProjectAdmin ? [{ path: '/fields', labelKey: 'nav.fields' }] : []),
  ]

  const alertChildren = [
    { path: '/alerts', labelKey: 'nav.alertList' },
    ...(isProjectAdmin ? [{ path: '/alert-rules', labelKey: 'nav.alertRules' }] : []),
    ...(isProjectAdmin ? [{ path: '/alert-silences', labelKey: 'nav.alertSilences' }] : []),
    ...(isAdmin ? [{ path: '/alert-datasources', labelKey: 'nav.datasources' }] : []),
  ]

  return [
    // 第一组不挂标签：首页和工单管理紧跟 logo，多一行标签只会把整列往下压
    {
      key: 'workspace',
      label: false,
      items: [
        { key: '/dashboard', path: '/dashboard', labelKey: 'nav.dashboard', icon: 'home' },
        { key: '/issues', path: '/issues', labelKey: 'nav.issues', icon: 'ticket' },
      ],
    },
    {
      key: 'project',
      label: true,
      items: [
        { key: 'project-center', labelKey: 'nav.projects', icon: 'folder', children: projectChildren },
        { key: '/notifications', path: '/notifications', labelKey: 'nav.notifications', icon: 'mail', badge: true },
        ...(isProjectAdmin
          ? [{
              key: 'requirement-center',
              labelKey: 'nav.requirements',
              icon: 'doc',
              children: [
                { path: '/requirement-pools', labelKey: 'requirement.poolTitle' },
                { path: '/requirements', labelKey: 'requirement.listTitle' },
                { path: '/requirements/kanban', labelKey: 'requirement.kanbanTitle' },
                { path: '/requirement-categories', labelKey: 'requirement.categoryTitle' },
              ],
            } as NavItem]
          : []),
      ],
    },
    {
      key: 'alert',
      label: true,
      items: [{ key: 'alert-center', labelKey: 'nav.alerts', icon: 'bell', children: alertChildren }],
    },
    {
      key: 'system',
      label: true,
      items: [
        { key: '/reports', path: '/reports', labelKey: 'nav.reports', icon: 'chart' },
        { key: '/api-docs', path: '/api-docs', labelKey: 'nav.apiDocs', icon: 'api' },
        ...(isAdmin
          ? [{
              key: 'system',
              labelKey: 'nav.system',
              icon: 'gear',
              children: [
                { path: '/users', labelKey: 'nav.users' },
                { path: '/settings', labelKey: 'nav.settings' },
              ],
            } as NavItem]
          : []),
      ],
    },
  ]
})

// 默认展开项目和告警两组，需求池与系统管理收起 —— 和设计样张一致。
// 前两组是日常入口，后两组是低频配置，一上来全摊开会把侧栏拉得很长。
const collapsed = ref(new Set<string>(['requirement-center', 'system']))

// 叶子项一定有 path，这里做个空值兜底只是为了让类型收敛
function go(path?: string) {
  if (path) handleMenuSelect(path)
}

function toggleGroup(key: string) {
  const next = new Set(collapsed.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  collapsed.value = next
}

const currentNavGroup = computed(() => {
  const path = route.path
  const hit = NAV_GROUP_BY_PREFIX.find(([prefix]) => path.startsWith(prefix))
  // 首页和工单管理落在"工作区"，也是没命中时的兜底
  return hit ? hit[1] : 'workspace'
})

function handleLocaleChange(l: AppLocale) {
  setLocale(l)
  // 同步写回账号，换台设备登录也能跟上；失败不回滚 —— 本机已经切好了，
  // 为一次同步失败把界面语言弹回去比不同步更难受。
  if (!userStore.isLoggedIn) return
  updateCurrentUser({ locale: l })
    .then(() => userStore.updateUser({ locale: l }))
    .catch(() => {
      // 已在请求拦截器里提示过，这里不再重复打扰
    })
}

const userStore = useUserStore()
const notificationStore = useNotificationStore()
const themeStore = useThemeStore()
const brandStore = useBrandStore()
const { hasNewVersion, countdown, dismissed, reload: reloadPage, dismiss } = useVersionCheck()

// 登录后连接 WebSocket
onMounted(() => {
  if (userStore.isLoggedIn) {
    notificationStore.connectWebSocket()
  }
})

// 监听登录状态变化
watch(() => userStore.isLoggedIn, (loggedIn) => {
  if (loggedIn) {
    notificationStore.connectWebSocket()
  } else {
    notificationStore.disconnectWebSocket()
  }
})

onUnmounted(() => {
  notificationStore.disconnectWebSocket()
})

// 判断是否为认证页面（登录/注册等）
// 这些页面不套主布局：它们要么还没登录、要么系统还没初始化，
// 渲染侧边栏和顶栏不仅没意义，还会去拉一堆当前拿不到的接口
const standalonePages = ['/login', '/register', '/forgot-password', '/reset-password', '/setup']

const isAuthPage = computed(() => standalonePages.includes(route.path))

// 计算当前激活的菜单
const activeMenu = computed(() => {
  const path = route.path
  // 工单详情页面高亮工单列表
  if (path.startsWith('/issues/')) {
    return '/issues'
  }
  // 告警详情页面高亮告警列表
  if (path.startsWith('/alerts/') && path !== '/alerts') {
    return '/alerts'
  }
  // 项目子页面（概览、看板、设置、角色）高亮项目列表
  if (path.startsWith('/projects/')) {
    return '/projects'
  }
  // 工作流设计器页面高亮工作流管理
  if (path.startsWith('/workflows/') && path.includes('/designer')) {
    return '/workflows'
  }
  return path
})

const handleLogout = () => {
  notificationStore.disconnectWebSocket()
  userStore.logout()
  router.push('/login')
}

// 处理菜单点击，支持 Ctrl/Cmd+Click 打开新标签页
const handleMenuSelect = (index: string) => {
  // 检测是否按住 Ctrl (Windows) 或 Cmd (Mac)
  const lastClickEvent = window.event as MouseEvent | undefined
  if (lastClickEvent && (lastClickEvent.ctrlKey || lastClickEvent.metaKey)) {
    // 在新标签页打开
    window.open(index, '_blank')
  } else {
    // 在当前标签页跳转
    router.push(index)
  }
}
</script>

<style scoped lang="scss">
// 外壳的视觉全部在 src/styles/_apple.scss（.ap 命名空间）里，
// 这里只留三件这一层独有的东西：版本更新横幅、窄屏抽屉、下拉里的选中态。

.update-banner {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 40px;
  z-index: var(--td-z-toast);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--td-space-1);
  background-color: var(--td-color-primary);
  color: #fff;
  font-size: var(--td-font-md);
  font-weight: var(--td-weight-medium);
  transition: var(--td-transition-bg);
  box-shadow: var(--td-elevation-2);
}

.update-banner-action {
  color: #fff;
  text-decoration: underline;
  cursor: pointer;
  font-weight: var(--td-weight-bold);
  margin: 0 4px;
}

.update-banner-action:hover {
  opacity: 0.85;
}

.update-banner-dismiss {
  opacity: 0.85;
  font-weight: var(--td-weight-medium);
}

// 窄屏：侧边栏收成抽屉
.nav-scrim {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: var(--td-bg-mask);
}

.nav-toggle { display: none; }

.ap.has-update-banner {
  padding-top: 40px;
}

@media (max-width: 900px) {
  .nav-toggle { display: grid; }

  :deep(.sidebar) {
    display: block;
    transform: translateX(-100%);
    transition: transform 200ms var(--td-ease-out);
    z-index: 1001;
  }

  :deep(.sidebar.is-open) { transform: translateX(0); }
}

.is-active-locale {
  color: var(--td-color-primary);
  font-weight: var(--td-weight-medium);
}
</style>
