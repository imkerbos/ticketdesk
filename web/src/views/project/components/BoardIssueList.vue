<template>
  <div class="board-issue-list">
    <!-- 列表头部 -->
    <div class="list-header">
      <div class="header-left">
        <span class="list-title">{{ t('project.boardList.title') }}</span>
        <span class="list-count">{{ total }}</span>
      </div>
      <el-button type="primary" size="small" class="create-btn" @click="$emit('create')">
        <el-icon><Plus /></el-icon>
        {{ t('common.create') }}
      </el-button>
    </div>

    <!-- 筛选区 -->
    <div class="list-filters">
      <div class="filter-row">
        <el-input
          v-model="keyword"
          :placeholder="t('project.boardList.searchPlaceholder')"
          clearable
          size="small"
          class="search-input"
          @clear="handleSearch"
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select
          v-model="statusFilter"
          :placeholder="t('issue.status')"
          size="small"
          multiple
          collapse-tags
          collapse-tags-tooltip
          clearable
          class="filter-select"
          @change="handleSearch"
        >
          <el-option :label="t('issue.statusMap.open')" value="open" />
          <el-option :label="t('issue.statusMap.in_progress')" value="in_progress" />
          <el-option :label="t('issue.statusMap.pending_review')" value="pending_review" />
          <el-option :label="t('issue.statusMap.reopened')" value="reopened" />
          <el-option :label="t('issue.statusMap.resolved')" value="resolved" />
          <el-option :label="t('issue.statusMap.closed')" value="closed" />
          <el-option :label="t('issue.statusMap.merged')" value="merged" />
        </el-select>
        <el-select
          v-model="priorityFilter"
          :placeholder="t('issue.priority')"
          size="small"
          clearable
          class="filter-select"
          @change="handleSearch"
        >
          <el-option :label="t('issue.priorityMap.P0')" value="P0" />
          <el-option :label="t('issue.priorityMap.P1')" value="P1" />
          <el-option :label="t('issue.priorityMap.P2')" value="P2" />
          <el-option :label="t('issue.priorityMap.P3')" value="P3" />
        </el-select>
        <el-popover
          :visible="moreFilterVisible"
          placement="bottom-end"
          :width="280"
          trigger="click"
          popper-class="board-more-filter-popover"
        >
          <template #reference>
            <el-button
              size="small"
              class="more-filter-btn"
              :class="{ active: extraFilterCount > 0 }"
              @click="moreFilterVisible = !moreFilterVisible"
            >
              <el-icon><Filter /></el-icon>
              <span v-if="extraFilterCount > 0" class="filter-badge">{{ extraFilterCount }}</span>
            </el-button>
          </template>
          <div class="more-filter-panel">
            <div class="filter-item">
              <label class="filter-label">{{ t('alert.rules.issueType') }}</label>
              <el-select
                v-model="issueTypeFilter"
                :placeholder="t('common.all')"
                size="small"
                clearable
                style="width: 100%"
                @change="handleSearch"
              >
                <el-option
                  v-for="type in issueTypes"
                  :key="type.id"
                  :label="type.display_name"
                  :value="type.id"
                />
              </el-select>
            </div>
            <div class="filter-item">
              <label class="filter-label">{{ t('project.boardList.handler') }}</label>
              <el-select
                v-model="assigneeFilter"
                :placeholder="t('common.all')"
                size="small"
                clearable
                filterable
                style="width: 100%"
                @change="handleSearch"
              >
                <el-option
                  v-for="m in members"
                  :key="m.user_id"
                  :label="m.user?.display_name || t('project.boardList.userFallback', { id: m.user_id })"
                  :value="m.user_id"
                />
              </el-select>
            </div>
            <div class="filter-item">
              <label class="filter-label">{{ t('project.boardList.category') }}</label>
              <el-select
                v-model="categoryFilter"
                :placeholder="t('common.all')"
                size="small"
                clearable
                style="width: 100%"
                @change="handleSearch"
              >
                <el-option :label="t('project.boardList.categoryNormal')" value="normal" />
                <el-option :label="t('project.boardList.categoryAlert')" value="alert" />
              </el-select>
            </div>
            <div class="filter-actions">
              <el-button size="small" text @click="resetExtraFilters">{{ t('common.reset') }}</el-button>
            </div>
          </div>
        </el-popover>
      </div>
    </div>

    <!-- 工单卡片列表 -->
    <div v-loading="loading" class="issue-cards">
      <div v-if="issueList.length === 0 && !loading" class="empty-state">
        <TdEmptyState preset="no-result" :title="t('project.boardList.noMatch')" />
      </div>
      <div
        v-for="item in issueList"
        :key="item.id"
        class="issue-card"
        :class="{ selected: item.issue_key === selectedKey }"
        @click="$emit('select', item.issue_key)"
      >
        <div class="card-top">
          <div class="key-group">
            <span class="priority-dot" :class="item.priority" :title="item.priority"></span>
            <a class="issue-key-link" @click.prevent.stop="router.push(`/issues/${item.issue_key}`)">{{ item.issue_key }}</a>
          </div>
          <div class="status-badge" :class="item.status">
            <span class="status-dot"></span>
            <span>{{ getStatusText(item.status) }}</span>
          </div>
        </div>
        <span class="card-title">{{ item.title }}</span>
        <div class="card-bottom">
          <div class="assignee-info">
            <div v-if="item.assignee" class="mini-avatar">{{ item.assignee.display_name?.charAt(0) || '?' }}</div>
            <div v-else class="mini-avatar unassigned">?</div>
            <span class="assignee-name">{{ item.assignee?.display_name || t('common.unassigned') }}</span>
          </div>
          <span v-if="item.issue_type" class="issue-type">{{ item.issue_type.display_name }}</span>
        </div>
      </div>
    </div>

    <!-- 简洁分页 -->
    <div v-if="totalPages > 1" class="list-pagination">
      <el-button text size="small" :disabled="page <= 1" @click="changePage(page - 1)">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="page-info">{{ page }} / {{ totalPages }}</span>
      <el-button text size="small" :disabled="page >= totalPages" @click="changePage(page + 1)">
        <el-icon><ArrowRight /></el-icon>
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Search, ArrowLeft, ArrowRight, Filter } from '@element-plus/icons-vue'
import { getIssueList } from '@/api/issue'
import { getProjectIssueTypes, getProjectMembers } from '@/api/project'
import type { Issue } from '@/types/issue'
import type { ProjectIssueType, ProjectMember } from '@/types/project'


const { t } = useI18n()

const router = useRouter()

const props = defineProps<{
  projectKey: string
  selectedKey: string
  initialStatus?: string // 逗号分隔的初始状态，如 "open,reopened"
}>()

defineEmits<{
  select: [issueKey: string]
  create: []
}>()

const loading = ref(false)
const issueList = ref<Issue[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 30
const keyword = ref('')
const statusFilter = ref<string[]>([])
const priorityFilter = ref('')

// 更多筛选
const moreFilterVisible = ref(false)
const issueTypeFilter = ref<number | undefined>(undefined)
const assigneeFilter = ref<number | undefined>(undefined)
const categoryFilter = ref('')
const issueTypes = ref<ProjectIssueType[]>([])
const members = ref<ProjectMember[]>([])

// 额外筛选生效数量
const extraFilterCount = computed(() => {
  let count = 0
  if (issueTypeFilter.value) count++
  if (assigneeFilter.value) count++
  if (categoryFilter.value) count++
  return count
})

const resetExtraFilters = () => {
  issueTypeFilter.value = undefined
  assigneeFilter.value = undefined
  categoryFilter.value = ''
  moreFilterVisible.value = false
  handleSearch()
}

// 默认显示的活跃状态（未手动选择时使用）
const defaultActiveStatuses = ['open', 'in_progress', 'pending_review', 'reopened']

const totalPages = computed(() => Math.ceil(total.value / pageSize) || 1)

const loadIssues = async () => {
  loading.value = true
  try {
    const params: Record<string, any> = {
      project_key: props.projectKey,
      page: page.value,
      page_size: pageSize,
      keyword: keyword.value || undefined,
      priority: priorityFilter.value || undefined,
      issue_type_id: issueTypeFilter.value || undefined,
      assignee_id: assigneeFilter.value || undefined,
      category: categoryFilter.value || undefined,
    }
    // 状态筛选：用户选择 > 默认活跃状态（后端过滤，保证 total 和分页准确）
    if (statusFilter.value.length > 0) {
      params.status = statusFilter.value.join(',')
    } else {
      params.status = defaultActiveStatuses.join(',')
    }
    const { data } = await getIssueList(params)
    issueList.value = data.data.items || []
    total.value = data.data.total
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

// 加载筛选选项数据
const loadFilterOptions = async () => {
  try {
    const [typesRes, membersRes] = await Promise.all([
      getProjectIssueTypes(props.projectKey),
      getProjectMembers(props.projectKey),
    ])
    issueTypes.value = typesRes.data.data || []
    members.value = membersRes.data.data || []
  } catch {
    // ignored
  }
}

const handleSearch = () => {
  page.value = 1
  loadIssues()
}

const changePage = (p: number) => {
  page.value = p
  loadIssues()
}

const getStatusText = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}

watch(() => props.projectKey, () => {
  page.value = 1
  loadIssues()
  loadFilterOptions()
})

onMounted(() => {
  // 从 initialStatus prop 初始化状态筛选
  if (props.initialStatus) {
    statusFilter.value = props.initialStatus.split(',').filter(Boolean)
  }
  loadIssues()
  loadFilterOptions()
})

// 暴露 reload 给父组件 (例如创建工单后刷新列表)
defineExpose({
  reload: () => {
    page.value = 1
    return loadIssues()
  },
})
</script>

<style scoped lang="scss">
.board-issue-list {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--td-bg-card);
}

// ---- 头部 ----
.list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  flex-shrink: 0;

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .list-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--td-text-primary);
  }

  .list-count {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 20px;
    height: 18px;
    padding: 0 6px;
    border-radius: 5px;
    background: var(--td-bg-section);
    font-size: 11px;
    font-weight: 600;
    color: var(--td-text-secondary);
  }

  .create-btn {
    border-radius: 6px;
  }
}

// ---- 筛选区 ----
.list-filters {
  padding: 0 12px 10px;
  flex-shrink: 0;

  /* 搜索 + 两个筛选 + 更多，合并成一条（§3.6） */
  .filter-row {
    display: flex;
    gap: 6px;
    align-items: center;

    /* 搜索框吃掉更多宽度：两个筛选下拉只需要放得下「状态 / 优先级」两个词 */
    .search-input {
      flex: 2;
      min-width: 0;

      :deep(.el-input__wrapper) {
        border-radius: 8px;
        box-shadow: 0 0 0 1px var(--td-border-color);

        &:focus-within {
          box-shadow: var(--td-focus-ring);
        }
      }
    }

    .filter-select {
      flex: 1;
      min-width: 0;
    }

    .more-filter-btn {
      flex-shrink: 0;
      padding: 4px 8px;
      position: relative;

      &.active {
        color: var(--td-color-primary);
        border-color: var(--td-color-primary);
        background: var(--td-tag-primary-bg);
      }

      .filter-badge {
        position: absolute;
        top: -4px;
        right: -4px;
        min-width: 15px;
        height: 15px;
        padding: 0 4px;
        border-radius: 5px;
        background: var(--td-color-primary);
        color: var(--td-text-white);
        font-size: 10px;
        font-weight: 600;
        display: flex;
        align-items: center;
        justify-content: center;
        line-height: 1;
      }
    }
  }
}

// ---- 工单卡片列表 ----
.issue-cards {
  flex: 1;
  overflow-y: auto;
  padding: 4px 8px;

  // 自定义滚动条
  &::-webkit-scrollbar {
    width: 4px;
  }

  &::-webkit-scrollbar-track {
    background: transparent;
  }

  &::-webkit-scrollbar-thumb {
    background: var(--td-scrollbar-thumb);
    border-radius: 4px;
  }
}

.issue-card {
  padding: 9px 10px;
  margin-bottom: 4px;
  border-radius: var(--td-radius-md);
  cursor: pointer;
  border: 1px solid transparent;
  background: transparent;
  transition: var(--td-transition-bg), var(--td-transition-border), var(--td-transition-shadow);
  position: relative;

  &:hover {
    background: var(--td-bg-section);
    border-color: var(--td-border-color);
  }

  &.selected {
    background: var(--td-tag-primary-bg);
    border-color: var(--td-color-primary);

    &:hover {
      background: var(--td-tag-primary-bg);
    }
  }

  .card-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 3px;
  }

  .key-group {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  /* 圆点而不是左侧竖色条：竖色条属于「卡片左边挂一条彩色」那一类装饰，
     这轮已经在角色卡、流程节点、看板选中态上都拆掉了。
     只有 P0 用红 —— 其余用弱色，红色才留得住分量（§3.1）。 */
  .priority-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;

    &.P0 { background: var(--td-color-danger); }
    &.P1 { background: var(--td-color-warning); }
    &.P2 { background: var(--td-text-secondary); }
    &.P3 { background: var(--td-text-disabled); }
  }

  .issue-key-link {
    font-size: 12px;
    font-weight: 600;
    color: var(--td-text-secondary);
    letter-spacing: 0.02em;
    text-decoration: none;
    transition: color 150ms ease-out;

    &:hover {
      color: var(--td-color-primary);
      text-decoration: underline;
    }
  }

  .card-title {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
    word-break: break-all;
    font-size: 12.5px;
    color: var(--td-text-primary);
    line-height: 1.45;
    margin-bottom: 6px;
  }

  .card-bottom {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .assignee-info {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .mini-avatar {
    width: 19px;
    height: 19px;
    border-radius: 50%;
    background: var(--td-tag-primary-bg);
    color: var(--td-tag-primary-text);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 10px;
    font-weight: 600;
    flex-shrink: 0;

    &.unassigned {
      background: var(--td-bg-section);
      color: var(--td-text-placeholder);
    }
  }

  .assignee-name {
    font-size: 12px;
    color: var(--td-text-placeholder);
    max-width: 80px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .issue-type {
    font-size: 11px;
    color: var(--td-text-secondary);
    background: var(--td-bg-section);
    padding: 1px 6px;
    border-radius: 5px;
  }
}

// ---- 状态徽章 ----
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  padding: 1px 7px;
  border-radius: 5px;
  font-weight: 500;
  white-space: nowrap;

  .status-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
  }

  &.open { background: var(--td-tag-orange-bg); color: var(--td-tag-orange-text); .status-dot { background: var(--td-color-warning); } }
  &.in_progress { background: var(--td-tag-primary-bg); color: var(--td-tag-primary-text); .status-dot { background: var(--td-color-primary); } }
  &.pending_review { background: var(--td-tag-indigo-bg); color: var(--td-tag-indigo-text); .status-dot { background: var(--td-tag-indigo-text); } }
  &.resolved { background: var(--td-tag-success-bg); color: var(--td-tag-success-text); .status-dot { background: var(--td-color-success); } }
  &.closed { background: var(--td-bg-section); color: var(--td-text-regular); .status-dot { background: var(--td-text-secondary); } }
  &.reopened { background: var(--td-tag-danger-bg); color: var(--td-tag-danger-text); .status-dot { background: var(--td-color-danger); } }
  &.merged { background: var(--td-tag-purple-bg); color: var(--td-tag-purple-text); .status-dot { background: var(--td-tag-purple-text); } }
}

// ---- 空状态 & 分页 ----
.empty-state {
  padding: 48px 0;
}

.list-pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-top: 1px solid var(--td-divider-color);
  flex-shrink: 0;

  .page-info {
    font-size: 12px;
    color: var(--td-text-placeholder);
    min-width: 48px;
    text-align: center;
  }
}

// ---- 更多筛选面板（scoped 内部分） ----
.more-filter-panel {
  .filter-item {
    margin-bottom: 12px;

    &:last-of-type {
      margin-bottom: 8px;
    }
  }

  .filter-label {
    display: block;
    font-size: 12px;
    color: var(--td-text-secondary);
    margin-bottom: 4px;
  }

  .filter-actions {
    display: flex;
    justify-content: flex-end;
    padding-top: 4px;
    border-top: 1px solid var(--td-divider-color);
  }
}

@media (prefers-reduced-motion: reduce) {
  .issue-card {
    transition: none;
  }
}
</style>
