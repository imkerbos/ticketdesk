import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/stores/user'
import { hasProjectPermission } from '@/utils/project-permissions'
import { clearLoadFailure } from '@/utils/load-failure'
import { useBrandStore } from '@/stores/brand'
import { useSetupStore } from '@/stores/setup'
import { i18n } from '@/i18n'

declare module 'vue-router' {
  interface RouteMeta {
    /** 页面标题的语言包 key；为空表示该页不显示标题栏标题 */
    titleKey?: string
    /** 无需登录即可访问 */
    public?: boolean
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/setup',
      name: 'Setup',
      component: () => import('@/views/setup/Setup.vue'),
      meta: { titleKey: 'nav.setup', public: true },
    },
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/auth/Login.vue'),
      meta: { titleKey: 'auth.login', public: true },
    },
    {
      path: '/forgot-password',
      name: 'ForgotPassword',
      component: () => import('@/views/auth/ForgotPassword.vue'),
      meta: { titleKey: 'nav.forgotPassword', public: true },
    },
    {
      path: '/reset-password',
      name: 'ResetPassword',
      component: () => import('@/views/auth/ResetPassword.vue'),
      meta: { titleKey: 'nav.resetPassword', public: true },
    },
    {
      path: '/auth/sso/callback',
      name: 'SSOCallback',
      component: () => import('@/views/auth/SSOCallback.vue'),
      meta: { titleKey: 'nav.ssoLogin', public: true },
    },
    {
      path: '/auth/sso/login',
      name: 'SSOLogin',
      component: () => import('@/views/auth/SSOLogin.vue'),
      meta: { titleKey: 'nav.ssoLogin', public: true },
    },
    {
      path: '/',
      redirect: '/dashboard',
    },
    // 首页/仪表盘
    {
      path: '/dashboard',
      name: 'Dashboard',
      component: () => import('@/views/dashboard/Dashboard.vue'),
      meta: { titleKey: 'nav.dashboard' },
    },
    // API 文档 (Swagger UI 桥接, 需登录, 设 cookie 后 iframe)
    {
      path: '/api-docs',
      name: 'ApiDocs',
      component: () => import('@/views/api/ApiDocs.vue'),
      meta: { titleKey: 'nav.apiDocs', hideHeader: true },
    },
    // 工单管理
    {
      path: '/issues',
      name: 'IssueList',
      component: () => import('@/views/issue/IssueList.vue'),
      meta: { titleKey: 'nav.issues' },
    },
    {
      path: '/issues/:key',
      name: 'IssueDetail',
      component: () => import('@/views/issue/IssueDetail.vue'),
      meta: { titleKey: 'nav.issueDetail' },
    },
    // 项目管理
    {
      path: '/projects',
      name: 'ProjectList',
      component: () => import('@/views/project/ProjectList.vue'),
      meta: { titleKey: 'nav.projects' },
    },
    {
      path: '/projects/:key',
      name: 'ProjectOverview',
      component: () => import('@/views/project/ProjectOverview.vue'),
      meta: { titleKey: 'nav.projectOverview' },
    },
    {
      path: '/projects/:key/board',
      name: 'ProjectBoard',
      component: () => import('@/views/project/ProjectBoard.vue'),
      meta: { titleKey: 'nav.projectBoard', hideHeader: true },
    },
    {
      path: '/projects/:key/board/:issueKey',
      name: 'ProjectBoardIssue',
      component: () => import('@/views/project/ProjectBoard.vue'),
      meta: { titleKey: 'nav.projectBoard', hideHeader: true },
    },
    {
      path: '/projects/:key/settings',
      name: 'ProjectSettings',
      component: () => import('@/views/project/ProjectSettings.vue'),
      meta: { titleKey: 'nav.projectSettings', requiresProjectPerm: 'project:manage' },
    },
    {
      path: '/projects/:key/roles',
      name: 'ProjectRoles',
      component: () => import('@/views/project/ProjectRoles.vue'),
      meta: { titleKey: 'nav.projectRoles', requiresProjectPerm: 'role:manage' },
    },
    // 工作流管理（需要项目管理员权限）
    {
      path: '/workflows',
      name: 'WorkflowList',
      component: () => import('@/views/workflow/WorkflowList.vue'),
      meta: { titleKey: 'nav.workflows', requiresProjectAdmin: true },
    },
    {
      path: '/workflows/:id/designer',
      name: 'WorkflowDesigner',
      component: () => import('@/views/workflow/WorkflowDesigner.vue'),
      meta: { titleKey: 'nav.workflowDesigner', requiresProjectAdmin: true },
    },
    // 字段管理（需要项目管理员权限）
    {
      path: '/fields',
      name: 'FieldManagement',
      component: () => import('@/views/field/FieldManagement.vue'),
      meta: { titleKey: 'nav.fields', requiresProjectAdmin: true },
    },
    // 告警中心
    {
      path: '/alerts',
      name: 'AlertList',
      component: () => import('@/views/alert/AlertList.vue'),
      meta: { titleKey: 'nav.alertList' },
    },
    {
      path: '/alerts/:id',
      name: 'AlertDetail',
      component: () => import('@/views/alert/AlertDetail.vue'),
      meta: { titleKey: 'nav.alertDetail' },
    },
    {
      path: '/alert-rules',
      name: 'AlertRules',
      component: () => import('@/views/alert/AlertRules.vue'),
      meta: { titleKey: 'nav.alertRules', requiresProjectAdmin: true },
    },
    {
      path: '/alert-silences',
      name: 'AlertSilences',
      component: () => import('@/views/alert/AlertSilences.vue'),
      meta: { titleKey: 'nav.alertSilences', requiresProjectAdmin: true },
    },
    {
      path: '/alert-datasources',
      name: 'AlertDatasources',
      component: () => import('@/views/alert/AlertDatasources.vue'),
      meta: { titleKey: 'nav.datasources', requiresAdmin: true },
    },
    // 通知中心
    {
      path: '/notifications',
      name: 'NotificationList',
      component: () => import('@/views/notification/NotificationList.vue'),
      meta: { titleKey: 'nav.notifications' },
    },
    // 需求池管理（需要项目管理员权限）
    {
      path: '/requirement-pools',
      name: 'RequirementPoolList',
      component: () => import('@/views/requirement/RequirementPoolList.vue'),
      meta: { titleKey: 'nav.requirementPools', requiresProjectAdmin: true },
    },
    {
      path: '/requirements',
      name: 'RequirementList',
      component: () => import('@/views/requirement/RequirementList.vue'),
      meta: { titleKey: 'nav.requirementList', requiresProjectAdmin: true },
    },
    {
      path: '/requirements/kanban',
      name: 'RequirementKanban',
      component: () => import('@/views/requirement/RequirementKanban.vue'),
      meta: { titleKey: 'nav.requirementKanban', requiresProjectAdmin: true },
    },
    {
      path: '/requirement-categories',
      name: 'RequirementCategoryList',
      component: () => import('@/views/requirement/RequirementCategoryList.vue'),
      meta: { titleKey: 'nav.requirementCategories', requiresProjectAdmin: true },
    },
    // 报表统计
    {
      path: '/reports',
      name: 'Reports',
      component: () => import('@/views/report/Reports.vue'),
      meta: { titleKey: 'nav.reports' },
    },
    // 系统管理（需要管理员权限）
    {
      path: '/users',
      name: 'UserList',
      component: () => import('@/views/user/UserList.vue'),
      meta: { titleKey: 'nav.users', requiresAdmin: true },
    },
    // 个人设置
    {
      path: '/profile',
      name: 'Profile',
      component: () => import('@/views/user/Profile.vue'),
      meta: { titleKey: 'nav.profile' },
    },
    // 系统设置（需要管理员权限）
    {
      path: '/settings',
      name: 'SystemSettings',
      component: () => import('@/views/system/SystemSettings.vue'),
      meta: { titleKey: 'nav.settings', requiresAdmin: true },
    },
    // 错误页面
    {
      path: '/403',
      name: 'Forbidden',
      component: () => import('@/views/error/Forbidden.vue'),
      meta: { titleKey: 'nav.forbidden', public: true },
    },
    {
      path: '/500',
      name: 'ServerError',
      component: () => import('@/views/error/ServerError.vue'),
      meta: { titleKey: 'nav.serverError', public: true },
    },
    // 404 兜底（必须放在最后）
    {
      path: '/:pathMatch(.*)*',
      name: 'NotFound',
      component: () => import('@/views/error/NotFound.vue'),
      meta: { titleKey: 'nav.notFound', public: true },
    },
  ],
})

router.beforeEach(async (to, _from, next) => {
  // 上一页的加载失败横幅不该跟到下一页来
  clearLoadFailure()

  const brandStore = useBrandStore()
  const appName = brandStore.systemName || 'TicketDesk'
  const titleKey = to.meta.titleKey as string | undefined
  document.title = `${titleKey ? i18n.global.t(titleKey) : appName} - ${appName}`

  // 初始化状态优先于登录态：一个还没有管理员的实例，
  // 让人停在登录页反复试密码没有意义。
  //
  // 在守卫里自己保证状态就绪，而不是依赖 main.ts 里先拉一次 ——
  // 路由的首次导航在 app.use(router) 时就开始了，早于任何 then 回调，
  // 靠 bootstrap 顺序保证会稳定地跑输。
  const setupStore = useSetupStore()
  if (!setupStore.loaded) {
    await setupStore.fetchStatus()
  }
  if (!setupStore.initialized) {
    if (to.name !== 'Setup') {
      next('/setup')
      return
    }
    next()
    return
  }
  // 已初始化就没必要再看向导，误入直接送去登录
  if (to.name === 'Setup') {
    next('/login')
    return
  }

  const userStore = useUserStore()

  // 公开页面（如登录页）直接放行
  if (to.meta.public) {
    next()
    return
  }

  // 检查是否已登录
  if (!userStore.isLoggedIn) {
    next('/login')
    return
  }

  // 检查是否需要管理员权限
  if (to.meta.requiresAdmin && !userStore.isAdmin) {
    next('/403')
    return
  }

  // 检查是否需要项目管理员权限
  if (to.meta.requiresProjectAdmin && !userStore.isProjectAdmin) {
    next('/403')
    return
  }

  // 检查在这个项目里有没有指定权限。
  //
  // 后端本来就用同一套 requirePerm 挡住了写操作，但页面照样渲染得出来 ——
  // 项目成员打开「项目设置」能看到完整表单和「危险操作 → 删除此项目」，
  // 按下去才吃 403。控件摆在那里却一按就报错，比不显示更糟。
  const perm = to.meta.requiresProjectPerm as string | undefined
  if (perm && typeof to.params.key === 'string') {
    if (!(await hasProjectPermission(to.params.key, perm))) {
      next('/403')
      return
    }
  }

  next()
})

export default router
