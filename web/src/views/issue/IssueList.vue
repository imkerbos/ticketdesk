<template>
  <!-- 这一页照着设计样张的 DOM 重写：原生 table / button / input，
       不再用 el-table + el-card + el-button-group 那一套。
       组件的默认样式要靠一层层 !important 去压，压不干净也压不稳；
       自己的 DOM 配 .ap 命名空间下的样式，改一处就是一处。 -->
  <div class="issue-list-container">
    <div class="page-head">
      <h1>{{ t('issue.listTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn secondary" @click="showAdvancedFilters = !showAdvancedFilters">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16M7 12h10M10 18h4" /></svg>
        {{ t('common.filter') }}
        <span v-if="activeFilterCount" class="btn-count">{{ activeFilterCount }}</span>
      </button>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('issue.createIssue') }}
      </button>
    </div>

    <div class="tabs" role="tablist">
      <button
        v-for="tab in categoryTabs"
        :key="tab.value"
        class="tab"
        role="tab"
        :aria-selected="queryParams.category === tab.value"
        @click="handleCategorySelect(tab.value)"
      >
        {{ t(tab.labelKey) }}
      </button>
    </div>

    <div class="toolbar">
      <div class="seg" role="group" :aria-label="t('issue.quickFilter')">
        <button
          v-for="f in quickFilters"
          :key="f.value"
          :aria-pressed="activeQuickFilter === f.value"
          @click="applyQuickFilter(f.value)"
        >
          {{ t(f.labelKey) }}
        </button>
      </div>

      <label class="search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input
          v-model="queryParams.keyword"
          type="search"
          :placeholder="t('issue.searchPlaceholder')"
          @keyup.enter="handleQuery"
          @search="handleQuery"
        />
      </label>

      <div class="grow"></div>

      <el-select
        v-model="selectedSavedViewId"
        clearable
        :placeholder="t('issue.savedView')"
        class="saved-view-select"
        @change="handleSavedViewChange"
      >
        <el-option v-for="view in savedViews" :key="view.id" :label="view.name" :value="view.id" />
      </el-select>
      <button class="btn secondary" @click="handleSaveCurrentView">{{ t('issue.saveCurrentView') }}</button>
      <button v-if="selectedSavedViewId" class="btn secondary danger-text" @click="handleDeleteCurrentView">
        {{ t('issue.deleteView') }}
      </button>
    </div>

    <!-- 高级筛选：展开时才渲染，收起时连同分隔线一起消失 -->
    <div v-if="showAdvancedFilters" class="filters">
      <el-select v-model="queryParams.project_key" :placeholder="t('issue.project')" clearable class="filter-select" @change="handleProjectFilterChange">
        <el-option v-for="pj in projects" :key="pj.project_key" :label="pj.name" :value="pj.project_key" />
      </el-select>
      <el-select v-model="queryParams.status" :placeholder="t('issue.status')" clearable class="filter-select" @change="handleQuery">
        <el-option :label="t('issue.statusMap.open')" value="open" />
        <el-option :label="t('issue.statusMap.in_progress')" value="in_progress" />
        <el-option :label="t('issue.statusMap.pending_review')" value="pending_review" />
        <el-option :label="t('issue.metricResolved')" value="resolved" />
        <el-option :label="t('issue.statusMap.closed')" value="closed" />
        <el-option :label="t('issue.statusMap.merged')" value="merged" />
      </el-select>
      <el-select v-model="queryParams.priority" :placeholder="t('issue.priority')" clearable class="filter-select-sm" @change="handleQuery">
        <el-option v-for="pr in ['P0', 'P1', 'P2', 'P3']" :key="pr" :label="t(`issue.priorityMap.${pr}`)" :value="pr" />
      </el-select>
      <el-select v-model="queryParams.assignee_id" :placeholder="t('issue.assignee')" clearable filterable class="filter-select" @change="handleAssigneeFilterChange">
        <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id" />
      </el-select>
      <el-select v-model="queryParams.reporter_id" :placeholder="t('issue.reporter')" clearable filterable class="filter-select" @change="handleReporterFilterChange">
        <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id" />
      </el-select>
      <el-select v-model="queryParams.issue_type_id" :placeholder="t('issue.type')" clearable class="filter-select" @change="handleQuery">
        <el-option v-for="type in filterIssueTypes" :key="type.id" :label="type.display_name" :value="type.id" />
      </el-select>
      <el-select v-model="queryParams.epic_id" placeholder="Epic" clearable filterable class="filter-select" @change="handleQuery">
        <el-option v-for="e in filterEpics" :key="e.id" :label="`${e.issue_key}: ${e.title}`" :value="e.id" />
      </el-select>
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        :range-separator="t('issue.list.dateSeparator')"
        :start-placeholder="t('issue.startDate')"
        :end-placeholder="t('issue.endDate')"
        value-format="YYYY-MM-DD"
        class="date-range-picker"
        @change="handleDateRangeChange"
      />
      <button class="btn secondary" @click="handleReset">
        <el-icon><Refresh /></el-icon>{{ t('common.reset') }}
      </button>
    </div>

    <section class="card">
      <div class="kpis">
        <div class="kpi"><div class="k">{{ t('issue.metricTotal') }}</div><div class="v">{{ issueStats.total }}</div></div>
        <div class="kpi"><div class="k">{{ t('issue.metricResolved') }}</div><div class="v">{{ issueStats.resolved }}</div></div>
        <div class="kpi"><div class="k">{{ t('issue.metricCompletionRate') }}</div><div class="v">{{ issueStats.completion_rate }}%</div></div>
        <div class="kpi"><div class="k">{{ t('issue.metricAvgResolve') }}</div><div class="v">{{ issueStats.avg_resolve_hours }}h</div></div>
        <div class="grow"></div>
        <div class="count">{{ t('common.total', { n: total }) }}</div>
        <div class="seg" role="group" :aria-label="t('issue.tableView')">
          <button :aria-pressed="viewMode === 'table'" @click="switchView('table')">{{ t('issue.tableView') }}</button>
          <button :aria-pressed="viewMode === 'kanban'" @click="switchView('kanban')">{{ t('issue.boardView') }}</button>
        </div>
      </div>

      <div v-if="viewMode === 'table'" v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col style="width: 92px" /><col /><col style="width: 78px" /><col style="width: 84px" />
            <col style="width: 62px" /><col style="width: 86px" /><col style="width: 104px" />
            <col style="width: 104px" /><col style="width: 132px" /><col style="width: 46px" />
          </colgroup>
          <thead>
            <tr>
              <th
                v-for="col in columns"
                :key="col.key"
                :class="{ sortable: col.sortable, 'is-sorted': sortState.prop === col.key }"
                :aria-sort="col.sortable ? (sortState.prop === col.key ? (sortState.order === 'ascending' ? 'ascending' : 'descending') : 'none') : undefined"
                @click="col.sortable && toggleSort(col.key)"
              >
                {{ col.labelKey ? t(col.labelKey) : '' }}
                <span v-if="col.sortable" class="caret">{{ sortState.prop === col.key && sortState.order === 'ascending' ? '▲' : '▼' }}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in issueList" :key="row.id" @click="handleRowClick(row, null, $event)">
              <td class="key">{{ row.issue_key }}</td>
              <td>
                <div class="title-cell">
                  <span class="dot" :style="{ background: priorityColor(row.priority) }"></span>
                  <span class="txt" :title="row.title">{{ row.title }}</span>
                  <template v-if="slaOf(row)">
                    <!-- 轻微超时走中性色：红色只留给确实拖过头的那几单，
                         两档都染红等于一档都没分 -->
                    <span v-if="slaOf(row)!.level === 'overdue'" class="pill sla">
                      {{ t('issue.list.overdueBy', { d: formatOverdueSpan(slaOf(row)!.minutesLeft) }) }}
                    </span>
                    <span v-else-if="slaOf(row)!.level === 'overdue_mild'" class="pill neutral mono">
                      {{ t('issue.list.overdueBy', { d: formatOverdueSpan(slaOf(row)!.minutesLeft) }) }}
                    </span>
                    <span v-else-if="slaOf(row)!.level === 'due_soon'" class="pill orange">{{ t('issue.detail.dueSoon') }}</span>
                  </template>
                </div>
              </td>
              <td><span class="pill neutral">{{ row.project_key }}</span></td>
              <td class="muted">{{ row.issue_type?.display_name || '-' }}</td>
              <td>
                <span class="prio">
                  <span class="dot" :style="{ background: priorityColor(row.priority) }"></span>{{ row.priority }}
                </span>
              </td>
              <td><span class="pill" :class="statusTone(row.status)">{{ getStatusText(row.status) }}</span></td>
              <td>
                <span v-if="row.assignee" class="person">
                  <span class="ava" :style="{ color: personColor(row.assignee.display_name) }">{{ row.assignee.display_name?.charAt(0) }}</span>
                  <span>{{ row.assignee.display_name }}</span>
                </span>
                <span v-else class="muted">{{ t('common.unassigned') }}</span>
              </td>
              <td>
                <span v-if="row.reporter" class="person">
                  <span class="ava" :style="{ color: personColor(row.reporter.display_name) }">{{ row.reporter.display_name?.charAt(0) }}</span>
                  <span>{{ row.reporter.display_name }}</span>
                </span>
                <span v-else class="muted">{{ t('common.unknown') }}</span>
              </td>
              <td class="time">{{ formatTime(row.created_at) }}</td>
              <td>
                <el-dropdown trigger="click" @command="() => handleDeleteIssue(row)">
                  <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="delete">
                        <el-icon><Delete /></el-icon>{{ t('common.delete') }}
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && issueList.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('issue.list.empty')" />
        </div>

        <div v-if="hasMore || total > queryParams.page_size" class="table-foot">
          <el-pagination
            v-model:current-page="queryParams.page"
            v-model:page-size="queryParams.page_size"
            :total="total"
            :page-sizes="[10, 20, 50, 100]"
            layout="sizes, prev, pager, next"
            @size-change="handleQuery"
            @current-change="handlePageChange"
          />
        </div>
      </div>

      <!-- 看板视图 -->
      <div v-if="viewMode === 'kanban'" v-loading="loading" class="kanban-view">
        <div class="kanban-board">
          <div v-for="column in kanbanColumns" :key="column.status" class="kanban-column">
            <div class="column-header" :class="column.status">
              <div class="column-title-group">
                <span class="column-dot" :class="column.status"></span>
                <span class="column-title">{{ column.label }}</span>
              </div>
              <span class="column-count">{{ column.issues.length }}</span>
            </div>
            <div class="column-content">
              <div
                v-for="issue in column.issues"
                :key="issue.id"
                class="kanban-card"
                @click="navigateTo(`/issues/${issue.issue_key}`, $event)"
              >
                <div class="card-header">
                  <span class="issue-key">{{ issue.issue_key }}</span>
                  <el-tag :type="getPriorityType(issue.priority)" size="small" effect="dark">
                    {{ issue.priority }}
                  </el-tag>
                </div>
                <div class="card-title">{{ issue.title }}</div>
                <div class="card-footer">
                  <el-tag size="small" effect="plain" type="info">{{ issue.project_key }}</el-tag>
                  <div v-if="issue.assignee" class="card-assignee">
                    <div class="mini-avatar sm">{{ issue.assignee.display_name?.charAt(0) || '?' }}</div>
                    <span>{{ issue.assignee.display_name }}</span>
                  </div>
                </div>
              </div>
              <div v-if="column.issues.length === 0" class="empty-column">
                {{ t('issue.list.empty') }}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 创建工单对话框 -->
    <CreateIssueDialog
      v-model="createDialogVisible"
      :default-project-key="queryParams.project_key || ''"
      :projects="projects"
      @created="(key: string) => router.push(`/issues/${key}`)"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
// 页面自己的图标改成内联 SVG（和样张同一套线形），只剩这两个还在组件里用
import { Refresh, Delete } from '@element-plus/icons-vue'
import { getIssueList, getIssueListStats, deleteIssue } from '@/api/issue'
import { getAllProjects, getAllIssueTypes, getProjectIssueTypes } from '@/api/project'
import { getAllUsers } from '@/api/user'
import type { Issue, IssueStatus, IssuePriority, KanbanColumn, IssueListStats } from '@/types/issue'
import type { Project, ProjectIssueType } from '@/types/project'
import type { UserOption } from '@/types/user'
import { useUserStore } from '@/stores/user'
import CreateIssueDialog from '@/components/CreateIssueDialog.vue'
import dayjs from 'dayjs'
import { getSlaState, formatOverdueSpan, type SlaState } from '@/utils/sla'
import { personColor } from '@/utils/avatar'


const { t } = useI18n()

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const loading = ref(false)
const issueList = ref<Issue[]>([])
const total = ref(0)
const hasMore = ref(false)
const viewMode = ref<'table' | 'kanban'>('table')
const projects = ref<Project[]>([])
const users = ref<UserOption[]>([])
const filterIssueTypes = ref<ProjectIssueType[]>([])
const filterEpics = ref<Issue[]>([])

// 从 URL query 初始化筛选条件
const initQuery = route.query

// 高级筛选默认收起：原来 7 个等宽下拉 + 日期区间常驻，
// 把主表格挤到首屏之外，而实际使用中大多数查询只用到关键词
const showAdvancedFilters = ref(false)

const dateRange = ref<[string, string] | null>(
  initQuery.start_date && initQuery.end_date
    ? [initQuery.start_date as string, initQuery.end_date as string]
    : null,
)

// 已生效的高级筛选个数，收起状态下用角标提示，避免「明明筛了却看不见」
const activeFilterCount = computed(() => {
  const p = queryParams
  const n = [p.project_key, p.status, p.priority, p.assignee_id, p.reporter_id, p.issue_type_id, p.epic_id]
    .filter((v) => v !== undefined && v !== null && v !== '').length
  return n + (dateRange.value && dateRange.value.length === 2 ? 1 : 0)
})
const issueStats = reactive<IssueListStats>({
  total: 0,
  resolved: 0,
  completion_rate: 0,
  avg_resolve_hours: 0,
})

type QuickFilterKey = '' | 'my-todo' | 'my-created'
interface SavedViewSnapshot {
  quick_filter: QuickFilterKey
  project_key?: string
  status?: IssueStatus
  priority?: IssuePriority
  assignee_id?: number
  reporter_id?: number
  issue_type_id?: number
  epic_id?: number
  keyword?: string
  category: '' | 'normal' | 'alert'
  sort_by?: string
  order?: 'asc' | 'desc'
}
interface SavedFilterView {
  id: string
  name: string
  snapshot: SavedViewSnapshot
  created_at: string
}

const dashboardFilter = ((initQuery.filter as string) || '') as QuickFilterKey
const currentUserId = userStore.user?.id
const initAssigneeID = initQuery.assignee_id
  ? Number(initQuery.assignee_id)
  : (dashboardFilter === 'my-todo' ? currentUserId : undefined)
const initReporterID = initQuery.reporter_id
  ? Number(initQuery.reporter_id)
  : (dashboardFilter === 'my-created' ? currentUserId : undefined)

const queryParams = reactive({
  page: initQuery.page ? Number(initQuery.page) : 1,
  page_size: initQuery.page_size ? Number(initQuery.page_size) : 20,
  project_key: (initQuery.project_key as string) || undefined,
  status: (initQuery.status as IssueStatus | undefined) || undefined,
  priority: (initQuery.priority as IssuePriority | undefined) || undefined,
  assignee_id: initAssigneeID,
  reporter_id: initReporterID,
  issue_type_id: initQuery.issue_type_id ? Number(initQuery.issue_type_id) : undefined,
  epic_id: initQuery.epic_id ? Number(initQuery.epic_id) : undefined,
  keyword: (initQuery.keyword as string) || undefined,
  category: ((initQuery.category as string) || '') as '' | 'normal' | 'alert',
  sort_by: (initQuery.sort_by as string) || undefined,
  order: (initQuery.order as 'asc' | 'desc' | undefined) || undefined,
  start_date: (initQuery.start_date as string) || undefined,
  end_date: (initQuery.end_date as string) || undefined,
})

const activeQuickFilter = ref<QuickFilterKey>(dashboardFilter)
const savedViews = ref<SavedFilterView[]>([])
const selectedSavedViewId = ref((initQuery.view as string) || '')
const savedViewsStorageKey = computed(() => `issue_list_saved_views_${userStore.user?.id || 'anonymous'}`)

const buildSnapshot = (): SavedViewSnapshot => ({
  quick_filter: activeQuickFilter.value,
  project_key: queryParams.project_key,
  status: queryParams.status,
  priority: queryParams.priority,
  assignee_id: queryParams.assignee_id,
  reporter_id: queryParams.reporter_id,
  issue_type_id: queryParams.issue_type_id,
  epic_id: queryParams.epic_id,
  keyword: queryParams.keyword,
  category: queryParams.category,
  sort_by: queryParams.sort_by,
  order: queryParams.order,
})

const applySnapshot = (snapshot: SavedViewSnapshot) => {
  activeQuickFilter.value = snapshot.quick_filter || ''
  queryParams.project_key = snapshot.project_key
  queryParams.status = snapshot.status
  queryParams.priority = snapshot.priority
  queryParams.assignee_id = snapshot.assignee_id
  queryParams.reporter_id = snapshot.reporter_id
  queryParams.issue_type_id = snapshot.issue_type_id
  queryParams.epic_id = snapshot.epic_id
  queryParams.keyword = snapshot.keyword
  queryParams.category = snapshot.category || ''
  queryParams.sort_by = snapshot.sort_by
  queryParams.order = snapshot.order
}

const loadSavedViewsFromStorage = () => {
  try {
    const raw = localStorage.getItem(savedViewsStorageKey.value)
    savedViews.value = raw ? JSON.parse(raw) : []
  } catch {
    savedViews.value = []
  }
}

const persistSavedViews = () => {
  localStorage.setItem(savedViewsStorageKey.value, JSON.stringify(savedViews.value))
}

// 筛选条件同步到 URL query params
const syncQueryToUrl = () => {
  const query: Record<string, string> = {}
  if (activeQuickFilter.value) query.filter = activeQuickFilter.value
  if (queryParams.keyword) query.keyword = queryParams.keyword
  if (queryParams.project_key) query.project_key = queryParams.project_key
  if (queryParams.status) query.status = queryParams.status
  if (queryParams.priority) query.priority = queryParams.priority
  if (queryParams.assignee_id) query.assignee_id = String(queryParams.assignee_id)
  if (queryParams.reporter_id) query.reporter_id = String(queryParams.reporter_id)
  if (queryParams.issue_type_id) query.issue_type_id = String(queryParams.issue_type_id)
  if (queryParams.epic_id) query.epic_id = String(queryParams.epic_id)
  if (queryParams.category) query.category = queryParams.category
  if (queryParams.sort_by) query.sort_by = queryParams.sort_by
  if (queryParams.order) query.order = queryParams.order
  if (queryParams.start_date) query.start_date = queryParams.start_date
  if (queryParams.end_date) query.end_date = queryParams.end_date
  if (selectedSavedViewId.value) query.view = selectedSavedViewId.value
  if (queryParams.page > 1) query.page = String(queryParams.page)
  router.replace({ query })
}

// 看板的列必须覆盖全部状态。原来只写了 open / in_progress / pending_review / resolved
// 四列，状态是 reopened / closed / merged 的工单在 forEach 里找不到列就被默默丢掉 ——
// 列表说「共 35 条」，看板只画得出 28 张卡，少掉的 7 条用户完全看不见。
const KANBAN_STATUSES = [
  'open', 'in_progress', 'pending_review', 'reopened', 'resolved', 'closed', 'merged',
] as const

const kanbanColumns = computed<KanbanColumn[]>(() => {
  const columns: KanbanColumn[] = KANBAN_STATUSES.map((status) => ({
    status,
    label: t(`issue.statusMap.${status}`),
    issues: [],
  }))
  const byStatus = new Map(columns.map((c) => [c.status, c]))
  issueList.value.forEach((issue) => {
    // 真出现了没列出来的状态也不能吞掉：归到最后一列，总比凭空消失好
    const column = byStatus.get(issue.status) ?? columns[columns.length - 1]
    column.issues.push(issue)
  })
  // 终态列没有卡时不占位，避免一排空列把有内容的列挤窄
  return columns.filter((c) => c.issues.length > 0 || !['closed', 'merged', 'reopened'].includes(c.status))
})

const createDialogVisible = ref(false)

const loadData = async () => {
  loading.value = true
  try {
    const params = { ...queryParams }
    if (viewMode.value === 'kanban') params.page_size = 100
    const [listRes, statsRes] = await Promise.all([
      getIssueList(params),
      getIssueListStats(params),
    ])
    issueList.value = listRes.data.data.items
    total.value = listRes.data.data.total
    hasMore.value = listRes.data.data.has_more || false
    Object.assign(issueStats, statsRes.data.data)
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const loadFilterOptions = async () => {
  try {
    const [projectsRes, usersRes] = await Promise.all([getAllProjects(), getAllUsers()])
    projects.value = projectsRes.data.data
    users.value = usersRes.data.data
  } catch {
    // ignored
  }
}

const loadFilterIssueTypes = async () => {
  try {
    if (queryParams.project_key) {
      const { data } = await getProjectIssueTypes(queryParams.project_key)
      filterIssueTypes.value = data.data || []
    } else {
      const { data } = await getAllIssueTypes()
      filterIssueTypes.value = data.data || []
    }
  } catch {
    // ignored
  }
}

const loadFilterEpics = async () => {
  try {
    const epicType = filterIssueTypes.value.find(t => t.name.toLowerCase() === 'epic')
    if (!epicType) {
      filterEpics.value = []
      return
    }
    const params: Record<string, any> = { issue_type_id: epicType.id, page_size: 100 }
    if (queryParams.project_key) {
      params.project_key = queryParams.project_key
    }
    const { data } = await getIssueList(params)
    filterEpics.value = data.data.items || []
  } catch {
    // ignored
  }
}

const clearSelectedSavedView = () => {
  selectedSavedViewId.value = ''
}

const normalizeQuickFilter = () => {
  if (activeQuickFilter.value === 'my-todo' && queryParams.assignee_id !== currentUserId) {
    activeQuickFilter.value = ''
  }
  if (activeQuickFilter.value === 'my-created' && queryParams.reporter_id !== currentUserId) {
    activeQuickFilter.value = ''
  }
}

const applyQuickFilter = (filter: QuickFilterKey) => {
  if ((filter === 'my-todo' || filter === 'my-created') && !currentUserId) {
    ElMessage.warning(t('issue.noCurrentUser'))
    return
  }
  activeQuickFilter.value = filter
  if (filter === 'my-todo') {
    queryParams.assignee_id = currentUserId
    queryParams.reporter_id = undefined
  } else if (filter === 'my-created') {
    queryParams.reporter_id = currentUserId
    queryParams.assignee_id = undefined
  } else {
    queryParams.assignee_id = undefined
    queryParams.reporter_id = undefined
  }
  clearSelectedSavedView()
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}

const handleSaveCurrentView = async () => {
  try {
    const promptResult = await ElMessageBox.prompt(t('issue.list.viewNamePrompt'), t('issue.saveCurrentView'), {
      inputValue: activeQuickFilter.value === 'my-todo' ? t('issue.myTodo') : activeQuickFilter.value === 'my-created' ? t('issue.myCreated') : '',
      inputPlaceholder: t('issue.viewNamePlaceholder'),
      confirmButtonText: t('common.save'),
      cancelButtonText: t('common.cancel'),
      inputValidator: (val: string) => !!val?.trim() || t('issue.list.viewNameRequired'),
    })
    if (typeof promptResult === 'string' || !promptResult || !('value' in promptResult)) return
    const name = promptResult.value.trim()
    const id = `${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
    savedViews.value.unshift({
      id,
      name,
      snapshot: buildSnapshot(),
      created_at: new Date().toISOString(),
    })
    persistSavedViews()
    selectedSavedViewId.value = id
    syncQueryToUrl()
    ElMessage.success(t('issue.list.viewSaved'))
  } catch {
    // 用户取消
  }
}

const handleSavedViewChange = async (viewID?: string) => {
  if (!viewID) {
    selectedSavedViewId.value = ''
    syncQueryToUrl()
    return
  }
  const view = savedViews.value.find(v => v.id === viewID)
  if (!view) {
    ElMessage.warning(t('issue.list.viewNotFound'))
    selectedSavedViewId.value = ''
    syncQueryToUrl()
    return
  }
  selectedSavedViewId.value = view.id
  applySnapshot(view.snapshot)
  queryParams.page = 1
  await loadFilterIssueTypes()
  await loadFilterEpics()
  syncQueryToUrl()
  loadData()
}

const handleDeleteCurrentView = async () => {
  if (!selectedSavedViewId.value) return
  const view = savedViews.value.find(v => v.id === selectedSavedViewId.value)
  if (!view) return
  try {
    await ElMessageBox.confirm(t('issue.list.confirmDeleteView', { name: view.name }), t('issue.deleteView'), { type: 'warning' })
    savedViews.value = savedViews.value.filter(v => v.id !== selectedSavedViewId.value)
    selectedSavedViewId.value = ''
    persistSavedViews()
    syncQueryToUrl()
    ElMessage.success(t('issue.list.viewDeleted'))
  } catch {
    // 用户取消
  }
}

const handleCategoryChange = () => {
  clearSelectedSavedView()
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}
const handleQuery = () => {
  clearSelectedSavedView()
  normalizeQuickFilter()
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}
const handlePageChange = () => { syncQueryToUrl(); loadData() }

const handleAssigneeFilterChange = () => {
  clearSelectedSavedView()
  normalizeQuickFilter()
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}

const handleReporterFilterChange = () => {
  clearSelectedSavedView()
  normalizeQuickFilter()
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}

const handleDateRangeChange = (val: [string, string] | null) => {
  clearSelectedSavedView()
  queryParams.start_date = val ? val[0] : undefined
  queryParams.end_date = val ? val[1] : undefined
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}

// 排序：prop 到后端字段的映射
// ── 模板用到的静态结构 ──────────────────────────────────
// 列定义、tab、快捷筛选都从模板里的重复标签抽成数据，
// 模板只剩一层 v-for，改列宽或加一列不用再翻两百行 JSX 式的标签。

const categoryTabs = [
  { value: '', labelKey: 'issue.tabAll' },
  { value: 'normal', labelKey: 'issue.tabNormal' },
  { value: 'alert', labelKey: 'issue.tabAlert' },
] as const

const quickFilters = [
  { value: '' as QuickFilterKey, labelKey: 'common.all' },
  { value: 'my-todo' as QuickFilterKey, labelKey: 'issue.myTodo' },
  { value: 'my-created' as QuickFilterKey, labelKey: 'issue.myCreated' },
]

const columns = [
  { key: 'issue_key', labelKey: 'issue.key', sortable: true },
  { key: 'title', labelKey: 'issue.title', sortable: false },
  { key: 'project_key', labelKey: 'issue.project', sortable: false },
  { key: 'issue_type_id', labelKey: 'issue.type', sortable: true },
  { key: 'priority', labelKey: 'issue.priority', sortable: true },
  { key: 'status', labelKey: 'issue.status', sortable: true },
  { key: 'assignee', labelKey: 'issue.assignee', sortable: false },
  { key: 'reporter', labelKey: 'issue.reporter', sortable: false },
  { key: 'created_at', labelKey: 'common.createdAt', sortable: true },
  { key: 'actions', labelKey: '', sortable: false },
]

// 状态药丸的色调：和语义色一一对应，不要在模板里写条件表达式
const STATUS_TONE: Record<string, string> = {
  open: 'neutral',
  reopened: 'neutral',
  in_progress: 'orange',
  pending_review: 'blue',
  resolved: 'green',
  closed: 'neutral',
  merged: 'purple',
}

const statusTone = (status: string) => STATUS_TONE[status] || 'neutral'

const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)',
}

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

const handleCategorySelect = (value: (typeof categoryTabs)[number]['value']) => {
  queryParams.category = value
  handleCategoryChange()
}

const switchView = (mode: 'table' | 'kanban') => {
  viewMode.value = mode
  handleViewModeChange()
}

const sortFieldMap: Record<string, string> = {
  issue_key: 'id',
  issue_type_id: 'issue_type_id',
  priority: 'priority',
  status: 'status',
  created_at: 'created_at',
}

const defaultSort = computed(() => {
  if (!queryParams.sort_by) return {}
  // 反向查找 prop
  const prop = Object.entries(sortFieldMap).find(([, v]) => v === queryParams.sort_by)?.[0]
  if (!prop) return {}
  return { prop, order: queryParams.order === 'asc' ? 'ascending' : 'descending' }
})

const sortState = computed<{ prop: string; order: string | null }>(() => {
  const d = defaultSort.value as { prop?: string; order?: string }
  return { prop: d.prop || '', order: d.order || null }
})

// 原生表头的三态排序：升 → 降 → 取消。el-table 是自己管这个状态的，
// 换成原生 table 后要在这里补上。
const toggleSort = (prop: string) => {
  const cur = sortState.value
  const next = cur.prop !== prop ? 'ascending' : cur.order === 'ascending' ? 'descending' : null
  handleSortChange({ prop, order: next })
}

const handleSortChange = ({ prop, order }: { prop: string; order: string | null }) => {
  if (!order) {
    queryParams.sort_by = undefined
    queryParams.order = undefined
  } else {
    queryParams.sort_by = sortFieldMap[prop] || prop
    queryParams.order = order === 'ascending' ? 'asc' : 'desc'
  }
  clearSelectedSavedView()
  queryParams.page = 1
  syncQueryToUrl()
  loadData()
}

const handleProjectFilterChange = async () => {
  clearSelectedSavedView()
  normalizeQuickFilter()
  queryParams.issue_type_id = undefined
  queryParams.epic_id = undefined
  queryParams.page = 1
  syncQueryToUrl()
  await loadFilterIssueTypes()
  await loadFilterEpics()
  loadData()
}

const handleReset = async () => {
  clearSelectedSavedView()
  activeQuickFilter.value = ''
  queryParams.page = 1
  queryParams.page_size = 20
  queryParams.project_key = undefined
  queryParams.status = undefined
  queryParams.priority = undefined
  queryParams.assignee_id = undefined
  queryParams.reporter_id = undefined
  queryParams.issue_type_id = undefined
  queryParams.epic_id = undefined
  queryParams.keyword = undefined
  queryParams.category = ''
  queryParams.sort_by = undefined
  queryParams.order = undefined
  queryParams.start_date = undefined
  queryParams.end_date = undefined
  dateRange.value = null
  syncQueryToUrl()
  await loadFilterIssueTypes()
  await loadFilterEpics()
  loadData()
}

const handleViewModeChange = () => { loadData() }
const handleRowClick = (row: Issue, _col: any, event: MouseEvent) => {
  const path = `/issues/${row.issue_key}`
  if (event.metaKey || event.ctrlKey) {
    window.open(router.resolve(path).href, '_blank')
  } else {
    router.push(path)
  }
}

const navigateTo = (path: string, event: MouseEvent) => {
  if (event.metaKey || event.ctrlKey) {
    window.open(router.resolve(path).href, '_blank')
  } else {
    router.push(path)
  }
}

const handleCreate = () => {
  createDialogVisible.value = true
}

const handleDeleteIssue = async (issue: Issue) => {
  try {
    await ElMessageBox.confirm(
      t('issue.list.confirmDelete', { key: issue.issue_key }),
      t('issue.list.deleteTitle'),
      { type: 'warning' }
    )
    await deleteIssue(issue.issue_key)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
const getPriorityType = (priority: string): TagType => {
  const map: Record<string, TagType> = { P0: 'danger', P1: 'warning', P2: 'info', P3: 'info' }
  return map[priority] || 'info'
}
const getStatusText = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}
const formatTime = (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm')

// 超时判定在 utils/sla.ts，与工单详情共用一份。
// 每行在模板里要读好几次，这里按 issue id 缓存一次渲染内的结果，
// 顺便保证同一行的几个分支读到的是同一个时刻算出来的值。
const slaCache = new Map<number, SlaState | null>()
watch(issueList, () => slaCache.clear())

const slaOf = (row: Issue): SlaState | null => {
  if (!slaCache.has(row.id)) slaCache.set(row.id, getSlaState(row))
  return slaCache.get(row.id) ?? null
}

onMounted(async () => {
  loadSavedViewsFromStorage()
  if (selectedSavedViewId.value) {
    const view = savedViews.value.find(v => v.id === selectedSavedViewId.value)
    if (view) {
      applySnapshot(view.snapshot)
    } else {
      selectedSavedViewId.value = ''
    }
  }
  await loadFilterOptions()
  await loadFilterIssueTypes()
  await loadFilterEpics()
  loadData()
})
</script>

<style scoped lang="scss">
// 表格、工具带、指标条的样式全部在 src/styles/_apple.scss（.ap 命名空间）里，
// 这一页只留两样东西：页面自身的排版容器，和看板视图（样张里没有看板）。

.issue-list-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.grow { flex: 1; }

// 高级筛选行：展开时才渲染，所以不需要分隔线，本身就是一条独立的带子
.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-select { width: 140px; }
.filter-select-sm { width: 120px; }
.date-range-picker { width: 260px; }
.saved-view-select { width: 180px; }

// 删除视图是破坏性操作：默认中性，悬停才变红
.btn.danger-text:hover { color: var(--td-color-danger); border-color: var(--td-color-danger); }

.empty { padding: 48px 0; }


.kanban-view {
  min-height: calc(100vh - 400px);

  .kanban-board {
    display: flex;
    gap: 20px;
    overflow-x: auto;
    padding-bottom: 16px;
    height: calc(100vh - 400px);
  }

  .kanban-column {
    flex: 1;
    min-width: 320px;
    background-color: var(--td-bg-page);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .column-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 14px 16px;
    border-radius: 12px 12px 0 0;

    .column-title-group {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    .column-dot {
      width: 10px;
      height: 10px;
      border-radius: 50%;

      // 和表格里状态药丸用的是同一套语义色
      &.open { background: var(--td-text-placeholder); }
      &.in_progress { background: var(--td-color-warning); }
      &.pending_review { background: var(--td-color-primary); }
      &.reopened { background: var(--td-color-danger); }
      &.resolved { background: var(--td-color-success); }
      &.closed { background: var(--td-text-disabled); }
      &.merged { background: var(--td-tag-purple-text); }
    }

    .column-title {
      font-weight: 600;
      font-size: 14px;
      color: var(--td-text-regular);
    }

    .column-count {
      background-color: var(--td-border-color);
      padding: 2px 10px;
      border-radius: 10px;
      font-size: 12px;
      font-weight: 600;
      color: var(--td-text-secondary);
    }
  }

  .column-content {
    flex: 1;
    overflow-y: auto;
    padding: 12px;
  }

  .kanban-card {
    background-color: var(--td-bg-card);
    border-radius: 10px;
    padding: 14px;
    margin-bottom: 10px;
    box-shadow: var(--td-elevation-1);
    cursor: pointer;
    border: 1px solid var(--td-divider-color);
    transition: var(--td-transition-border), var(--td-transition-bg);

    &:hover {
      border-color: var(--td-color-primary);
      background-color: var(--td-bg-section);
    }

    &:active {
      background-color: var(--td-bg-page);
    }

    .card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;

      .issue-key {
        color: var(--td-color-primary);
        font-size: 12px;
        font-weight: 600;
      }
    }

    .card-title {
      font-size: 14px;
      font-weight: 500;
      color: var(--td-text-primary);
      margin-bottom: 12px;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
      line-height: 1.5;
    }

    .card-footer {
      display: flex;
      justify-content: space-between;
      align-items: center;

      .card-assignee {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 12px;
        color: var(--td-text-secondary);
      }

      .mini-avatar.sm {
        width: 22px;
        height: 22px;
        border-radius: 6px;
        font-size: 10px;
      }
    }
  }

  .empty-column {
    text-align: center;
    padding: 24px;
    color: var(--td-text-placeholder);
    font-size: 14px;
  }
}

// 日期范围选择器
.date-range-picker {
  width: 240px;
}

// KPI 统计卡片
.metrics-strip {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  padding: 14px 18px;
  border-bottom: 1px solid var(--td-border-color-light);

  // 视图切换并到统计条右端：它自己占一条 60px 的横带，
  // 而这条带子上除了两个按钮和一个计数什么都没有
  &__view {
    display: flex;
    align-items: center;
    gap: var(--td-space-3);
    margin-left: auto;
  }

  .total-count {
    font-size: var(--td-font-sm);
    color: var(--td-text-placeholder);
  }
}

// 最后一个指标不画分隔线，也不留右边距
.metrics-strip .td-stat-tile:nth-of-type(4) {
  border-right: 0;
  margin-right: 0;
}

.filter-toggle__count {
  display: inline-block;
  margin-left: 6px;
  min-width: 16px;
  padding: 0 4px;
  border-radius: var(--td-radius-full);
  background: var(--td-color-primary);
  color: #fff;
  font-size: var(--td-font-xs);
  line-height: 16px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.stats-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--td-space-4);
  margin-bottom: var(--td-space-5);
}

// 响应式
@media (max-width: 768px) {
  .saved-view-select,
  .filter-select,
  .filter-select-sm,
  .date-range-picker {
    width: 100% !important;
  }
}
</style>
