<template>
  <!-- 工作台：指标条 + 左右两栏。
       原来四个统计卡片各带一个彩色图标块，四个内容卡片头部又各带一个，
       一屏八个彩色方块，没有一个在表达状态。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('dashboard.title') }}</h1>
      <div class="grow"></div>
      <button class="btn secondary" @click="$router.push('/alerts')">{{ t('dashboard.viewAlerts') }}</button>
      <button class="btn secondary" @click="$router.push('/issues')">{{ t('dashboard.allIssues') }}</button>
      <button class="btn primary" @click="handleCreateIssue">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('issue.createIssue') }}
      </button>
    </div>

    <section class="card">
      <div class="kpis">
        <button class="kpi as-filter" @click="$router.push('/issues?filter=my-todo')">
          <div class="k">{{ t('dashboard.myTodo') }}</div><div class="v">{{ animatedStats.myTodo }}</div>
        </button>
        <button class="kpi as-filter" @click="$router.push('/issues?filter=my-created')">
          <div class="k">{{ t('dashboard.myCreated') }}</div><div class="v">{{ animatedStats.myCreated }}</div>
        </button>
        <button class="kpi as-filter" @click="$router.push('/alerts?status=firing')">
          <div class="k">{{ t('dashboard.pendingAlerts') }}</div><div class="v">{{ animatedStats.pendingAlerts }}</div>
        </button>
        <div class="kpi">
          <div class="k">{{ t('dashboard.weekResolved') }}</div><div class="v">{{ animatedStats.weekDone }}</div>
        </div>
      </div>
    </section>

    <div class="grid-2">
      <div class="col">
        <!-- 我的待办 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('dashboard.myTodoIssues') }}</h2>
            <span class="n">{{ animatedStats.myTodo }}</span>
            <div class="grow"></div>
            <button class="link-btn" @click="$router.push('/issues?filter=my-todo')">{{ t('dashboard.viewAll') }}</button>
          </div>
          <div v-loading="loadingTodo">
            <div v-if="todoIssues.length === 0" class="empty">
              <TdEmptyState preset="no-data" :title="t('dashboard.noTodo')" :description="t('dashboard.noTodoDesc')" />
            </div>
            <ul v-else class="rows">
              <li v-for="issue in todoIssues" :key="issue.id" class="row-item" @click="$router.push(`/issues/${issue.issue_key}`)">
                <span class="dot" :style="{ background: priorityColor(issue.priority) }"></span>
                <span class="key">{{ issue.issue_key }}</span>
                <span class="txt">{{ issue.title }}</span>
                <span class="pill neutral">{{ issue.project_key }}</span>
                <span class="muted status-text">{{ getStatusText(issue.status) }}</span>
                <span class="prio">{{ issue.priority }}</span>
              </li>
            </ul>
          </div>
        </section>

        <!-- 我创建的 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('dashboard.myCreatedIssues') }}</h2>
            <span class="n">{{ animatedStats.myCreated }}</span>
            <div class="grow"></div>
            <button class="link-btn" @click="$router.push('/issues?filter=my-created')">{{ t('dashboard.viewAll') }}</button>
          </div>
          <div v-loading="loadingCreated">
            <div v-if="createdIssues.length === 0" class="empty">
              <TdEmptyState preset="no-data" :title="t('dashboard.noCreated')" :description="t('dashboard.noCreatedDesc')" />
            </div>
            <ul v-else class="rows">
              <li v-for="issue in createdIssues" :key="issue.id" class="row-item" @click="$router.push(`/issues/${issue.issue_key}`)">
                <span class="dot" :style="{ background: priorityColor(issue.priority) }"></span>
                <span class="key">{{ issue.issue_key }}</span>
                <span class="txt">{{ issue.title }}</span>
                <span class="muted status-text">{{ getStatusText(issue.status) }}</span>
                <span v-if="issue.assignee" class="person">
                  <span class="ava" :style="{ color: personColor(issue.assignee.display_name) }">{{ issue.assignee.display_name?.charAt(0) }}</span>
                </span>
                <span v-else class="muted">{{ t('common.unassigned') }}</span>
              </li>
            </ul>
          </div>
        </section>
      </div>

      <div class="col">
        <!-- 待确认告警 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('dashboard.pendingAlerts') }}</h2>
            <span class="n">{{ animatedStats.pendingAlerts }}</span>
            <div class="grow"></div>
            <button class="link-btn" @click="$router.push('/alerts?status=firing')">{{ t('dashboard.viewAll') }}</button>
          </div>
          <div v-loading="loadingAlerts">
            <div v-if="pendingAlerts.length === 0" class="empty">
              <TdEmptyState preset="no-data" tone="primary" :title="t('dashboard.noAlerts')" :description="t('dashboard.noAlertsDesc')" />
            </div>
            <ul v-else class="rows">
              <li v-for="alert in pendingAlerts" :key="alert.id" class="row-item" @click="$router.push(`/alerts/${alert.id}`)">
                <span class="dot" :style="{ background: severityColor(alert.severity) }"></span>
                <span class="txt">{{ alert.alert_name }}</span>
                <span class="time">{{ formatTime(alert.starts_at) }}</span>
              </li>
            </ul>
          </div>
        </section>

        <!-- 最近活动 -->
        <section class="card">
          <div class="card-head">
            <h2>{{ t('dashboard.recentActivity') }}</h2>
          </div>
          <div v-loading="loadingActivities">
            <div v-if="recentActivities.length === 0" class="empty">
              <TdEmptyState preset="no-data" :title="t('dashboard.noActivity')" :description="t('dashboard.noActivityDesc')" />
            </div>
            <ul v-else class="rows">
              <li v-for="activity in recentActivities" :key="activity.id" class="row-item activity-row">
                <span class="txt">
                  <b>{{ activity.user_name }}</b>
                  {{ formatActivityAction(activity.action) }}
                  <a v-if="activity.entity_key" class="key" @click.stop="$router.push(`/issues/${activity.entity_key}`)">{{ activity.entity_key }}</a>
                </span>
                <span class="time">{{ formatTime(activity.created_at) }}</span>
              </li>
            </ul>
          </div>
        </section>
      </div>
    </div>

    <!-- 创建工单对话框 -->
    <CreateIssueDialog
      v-model="createDialogVisible"
      @created="(key: string) => router.push(`/issues/${key}`)"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { getMyTodoIssues, getMyCreatedIssues } from '@/api/issue'
import { getAlertList } from '@/api/alert'
import { getRecentActivities } from '@/api/activity'
import type { Issue } from '@/types/issue'
import type { Alert } from '@/types/alert'
import type { Activity } from '@/api/activity'
import CreateIssueDialog from '@/components/CreateIssueDialog.vue'
import dayjs from 'dayjs'
import { formatActivityAction } from '@/utils/activity'
import { personColor } from '@/utils/avatar'


const { t } = useI18n()

const router = useRouter()

// 统计数据
const stats = reactive({
  myTodo: 0,
  myCreated: 0,
  pendingAlerts: 0,
  weekDone: 0 })

// 计数动画
const animatedStats = reactive({
  myTodo: 0,
  myCreated: 0,
  pendingAlerts: 0,
  weekDone: 0 })

const animateTo = (key: keyof typeof animatedStats, to: number) => {
  const from = animatedStats[key]
  if (from === to) {
    animatedStats[key] = to
    return
  }
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    animatedStats[key] = to
    return
  }
  const duration = 250
  const startTime = performance.now()
  const step = (now: number) => {
    const progress = Math.min((now - startTime) / duration, 1)
    const eased = 1 - (1 - progress) ** 3
    animatedStats[key] = Math.round(from + (to - from) * eased)
    if (progress < 1) requestAnimationFrame(step)
  }
  requestAnimationFrame(step)
}

watch(() => stats.myTodo, (val) => animateTo('myTodo', val))
watch(() => stats.myCreated, (val) => animateTo('myCreated', val))
watch(() => stats.pendingAlerts, (val) => animateTo('pendingAlerts', val))
watch(() => stats.weekDone, (val) => animateTo('weekDone', val))

// 待办工单
const loadingTodo = ref(false)
const todoIssues = ref<Issue[]>([])

// 我创建的工单
const loadingCreated = ref(false)
const createdIssues = ref<Issue[]>([])

// 待确认告警
const loadingAlerts = ref(false)
const pendingAlerts = ref<Alert[]>([])

// 最近活动
const loadingActivities = ref(false)
const recentActivities = ref<Activity[]>([])

// 创建工单
const createDialogVisible = ref(false)

// 加载待办工单
const loadTodoIssues = async () => {
  loadingTodo.value = true
  try {
    const { data } = await getMyTodoIssues({ page: 1, page_size: 5 })
    todoIssues.value = data.data.items
    stats.myTodo = data.data.total
  } catch {
    // ignored
  } finally {
    loadingTodo.value = false
  }
}

// 加载我创建的工单
const loadCreatedIssues = async () => {
  loadingCreated.value = true
  try {
    const { data } = await getMyCreatedIssues({ page: 1, page_size: 5 })
    createdIssues.value = data.data.items
    stats.myCreated = data.data.total
  } catch {
    // ignored
  } finally {
    loadingCreated.value = false
  }
}

// 加载待确认告警
const loadPendingAlerts = async () => {
  loadingAlerts.value = true
  try {
    const { data } = await getAlertList({ status: 'firing', page: 1, page_size: 5 })
    pendingAlerts.value = data.data.items
    stats.pendingAlerts = data.data.total
  } catch {
    // ignored
  } finally {
    loadingAlerts.value = false
  }
}

// 加载最近活动
const loadRecentActivities = async () => {
  loadingActivities.value = true
  try {
    const { data } = await getRecentActivities(10)
    recentActivities.value = data.data
  } catch {
    // ignored
  } finally {
    loadingActivities.value = false
  }
}

// 打开创建工单对话框
const handleCreateIssue = () => {
  createDialogVisible.value = true
}

// 工具函数

const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)' }

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

const severityColor = (s: string) =>
  s === 'critical' ? 'var(--td-color-danger)' : s === 'warning' ? 'var(--td-color-warning)' : 'var(--td-text-placeholder)'

const getStatusText = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}

const formatTime = (time: string) => {
  return dayjs(time).format('MM-DD HH:mm')
}

// 初始化
onMounted(() => {
  loadTodoIssues()
  loadCreatedIssues()
  loadPendingAlerts()
  loadRecentActivities()
})
</script>

<style scoped lang="scss">
// 骨架都在 _apple.scss 里，这里只留两处工作台特有的单元格。

// 状态文字：固定宽度，避免每行的优先级和头像位置左右跳
.status-text {
  font-size: 12px;
  white-space: nowrap;
}

.activity-row {
  cursor: default;

  .key { cursor: pointer; }
  .key:hover { text-decoration: underline; text-underline-offset: 2px; }
}
</style>
