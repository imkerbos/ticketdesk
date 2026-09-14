<template>
  <!-- 项目概览：页头 + 项目内导航 + 指标条 + 两栏。
       原来 hero 区是一整块带背景的头部，里面又套 back 圆钮和三个带图标的
       导航块；页头统一成和其它页一样，项目内导航收成一排 tab。 -->
  <div class="page">
    <div class="page-head">
      <div class="detail-title">
        <h1>{{ project?.name || projectKey }}</h1>
        <div class="detail-meta">
          <span class="key static">{{ projectKey }}</span>
          <span class="muted">{{ project?.description || t('project.noDescription') }}</span>
        </div>
      </div>
      <div class="grow"></div>
      <!-- 「设置」下面那排 tab 里已经有一个，页头不再放第二个入口 -->
      <button class="btn secondary" @click="$router.push('/projects')">{{ t('project.roles.back') }}</button>
    </div>

    <div class="tabs" role="tablist">
      <button class="tab" role="tab" aria-selected="true">{{ t('project.overview') }}</button>
      <button class="tab" role="tab" aria-selected="false" @click="$router.push(`/projects/${projectKey}/board`)">{{ t('project.board') }}</button>
      <!-- 没有 project:manage 就不摆这个入口：点进去只会被路由守卫弹回 403 -->
      <button v-if="canManageProject" class="tab" role="tab" aria-selected="false" @click="$router.push(`/projects/${projectKey}/settings`)">{{ t('project.settingsLabel') }}</button>
    </div>

    <div v-loading="loading" class="page">
      <section class="card">
        <div class="kpis">
          <button class="kpi as-filter" @click="goBoard('open,reopened')">
            <div class="k">{{ t('issue.statusMap.open') }}</div><div class="v">{{ stats.open }}</div>
          </button>
          <button class="kpi as-filter" @click="goBoard('in_progress')">
            <div class="k">{{ t('issue.statusMap.in_progress') }}</div><div class="v">{{ stats.inProgress }}</div>
          </button>
          <button class="kpi as-filter" @click="goBoard('pending_review')">
            <div class="k">{{ t('issue.statusMap.pending_review') }}</div><div class="v">{{ stats.pendingReview }}</div>
          </button>
          <button class="kpi as-filter" @click="goBoard('resolved,closed')">
            <div class="k">{{ t('issue.statusMap.resolved') }}</div><div class="v">{{ stats.resolved }}</div>
          </button>
        </div>
      </section>

      <div class="grid-2">
        <div class="col">
          <section class="card">
            <div class="card-head">
              <h2>{{ t('project.recentIssues') }}</h2>
              <div class="grow"></div>
              <button class="link-btn" @click="$router.push(`/projects/${projectKey}/board`)">{{ t('project.gotoBoard') }}</button>
            </div>
            <div v-if="recentIssues.length === 0" class="empty">{{ t('project.noIssues') }}</div>
            <ul v-else class="rows">
              <li v-for="item in recentIssues" :key="item.id" class="row-item" @click="goIssue(item.issue_key)">
                <span class="dot" :style="{ background: priorityColor(item.priority) }"></span>
                <span class="key">{{ item.issue_key }}</span>
                <span class="txt">{{ item.title }}</span>
                <span class="muted status-text">{{ getStatusText(item.status) }}</span>
                <!-- 未指派也要占位：不然这一行的状态文字会顶到最右，
                     和有指派人的行差 28px，整列看着在左右跳 -->
                <span
                  class="ava" :class="{ unassigned: !item.assignee }"
                  :title="item.assignee?.display_name || t('common.unassigned')"
                >
                  {{ item.assignee ? item.assignee.display_name?.charAt(0) : '' }}
                </span>
              </li>
            </ul>
          </section>
        </div>

        <div class="col">
          <section class="card">
            <div class="card-head">
              <h2>{{ t('project.members') }}</h2>
              <span class="n">{{ members.length }}</span>
            </div>
            <div v-if="members.length === 0" class="empty">{{ t('project.noMembers') }}</div>
            <ul v-else class="rows">
              <li v-for="member in members" :key="member.id" class="row-item">
                <span class="ava" :style="{ color: personColor(member.user?.display_name) }">
                  {{ member.user?.display_name?.charAt(0) || '?' }}
                </span>
                <span class="txt">{{ member.user?.display_name || t('common.unknownUser') }}</span>
                <span class="muted">{{ member.role_name || member.role }}</span>
              </li>
            </ul>
          </section>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getProjectDetail, getProjectMembers } from '@/api/project'
import { getIssueList, getProjectOverviewStats } from '@/api/issue'
import type { Project, ProjectMember } from '@/types/project'
import type { Issue } from '@/types/issue'
import { personColor } from '@/utils/avatar'
import { hasProjectPermission } from '@/utils/project-permissions'


const { t } = useI18n()

const route = useRoute()
const router = useRouter()

const projectKey = computed(() => route.params.key as string)
const loading = ref(false)
const project = ref<Project | null>(null)
const recentIssues = ref<Issue[]>([])
const members = ref<ProjectMember[]>([])

const stats = reactive({
  open: 0,
  inProgress: 0,
  pendingReview: 0,
  resolved: 0,
})

const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)',
}

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

const getStatusText = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}

const loadData = async () => {
  loading.value = true
  try {
    const [projectRes, membersRes, issuesRes] = await Promise.all([
      getProjectDetail(projectKey.value, { _redirectOn404: true, _redirectOn403: true }),
      getProjectMembers(projectKey.value),
      getIssueList({ project_key: projectKey.value, page: 1, page_size: 10 }),
    ])
    project.value = projectRes.data.data
    members.value = membersRes.data.data || []
    recentIssues.value = issuesRes.data.data.items || []

    // 统计各状态数量（分别请求）
    await loadStats()
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const { data } = await getProjectOverviewStats(projectKey.value)
    stats.open = data.data.pending
    stats.inProgress = data.data.in_progress
    stats.pendingReview = data.data.pending_review
    stats.resolved = data.data.completed
  } catch {
    // ignored
  }
}

const goBoard = (status?: string) => {
  const query = status ? { status } : {}
  router.push({ path: `/projects/${projectKey.value}/board`, query })
}

const goIssue = (issueKey: string) => {
  router.push(`/projects/${projectKey.value}/board/${issueKey}`)
}

// 设置入口只对有 project:manage 的人显示。默认 false —— 没问出来之前
// 先不摆出来，晚 200ms 出现比点进去弹 403 好。
const canManageProject = ref(false)

onMounted(async () => {
  loadData()
  canManageProject.value = await hasProjectPermission(projectKey.value, 'project:manage')
})
</script>

<style scoped lang="scss">
// 骨架在 _apple.scss 里，这一页只留标题区和状态列。

.detail-title { min-width: 0; }
.detail-title h1 { font-size: 26px; font-weight: 600; letter-spacing: -0.022em; line-height: 1.15; margin: 0; }

.detail-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  font-size: 12.5px;
  min-width: 0;

  .muted { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
}

.status-text { font-size: 12px; white-space: nowrap; }
.empty { padding: 36px 0; text-align: center; color: var(--td-text-placeholder); font-size: 13px; }

/* 未指派时的占位圆：只占位不显字，保证状态文字这一列左右不跳 */
.ava.unassigned {
  background: var(--td-bg-section);
  box-shadow: inset 0 0 0 1px var(--td-border-color);
}
</style>
