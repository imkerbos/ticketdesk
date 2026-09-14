<template>
  <!-- 看板：页头 + 一条筛选带 + 列。卡片用发丝线，不再有阴影和实心标签。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('requirement.kanbanTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn secondary" @click="router.push('/requirements')">{{ t('requirement.listView') }}</button>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('requirement.create') }}
      </button>
    </div>

    <div class="toolbar">
      <el-select v-model="filters.pool_id" :placeholder="t('requirement.pool')" clearable class="filter-select" @change="loadKanban">
        <el-option v-for="pool in pools" :key="pool.id" :label="pool.name" :value="pool.id" />
      </el-select>
      <div class="seg" role="group" :aria-label="t('requirement.groupBy')">
        <button
          v-for="g in (['status', 'priority', 'assignee', 'timeline'] as const)"
          :key="g"
          :aria-pressed="filters.group_by === g"
          @click="selectGroupBy(g)"
        >
          {{ t(`requirement.group${g.charAt(0).toUpperCase()}${g.slice(1)}`) }}
        </button>
      </div>
    </div>

    <div v-loading="loading" class="kanban-container">
      <div class="kanban-board">
        <div v-for="column in kanbanData.columns" :key="column.key" class="kanban-column">
          <!--
            列头原来是整条饱和实心色带（三列蓝、一列绿），比页面上任何按钮
            都响；而且待评估/规划中/进行中三个状态同为蓝色，等于没区分。
            改成状态色圆点 + 深色标题，颜色只用来区分状态本身。
          -->
          <div class="column-header">
            <span class="column-dot" :class="`is-${column.key}`"></span>
            <span class="column-title">{{ column.title }}</span>
            <span class="column-count">{{ column.count }}</span>
          </div>
          <div class="column-content">
            <div
              v-for="requirement in column.requirements"
              :key="requirement.id"
              class="kanban-card"
              @click="handleCardClick(requirement)"
            >
              <div class="card-title">{{ requirement.title }}</div>

              <div class="card-tags">
                <span class="prio">
                  <span class="dot" :style="{ background: priorityColor(requirement.priority) }"></span>{{ requirement.priority }}
                </span>
                <span class="pill neutral">{{ getCategoryLabel(requirement.category) }}</span>
                <span v-if="requirement.converted_issue_key" class="key">{{ requirement.converted_issue_key }}</span>
              </div>

              <div class="card-foot">
                <span class="muted">{{ requirement.pool_name }}</span>
                <div class="grow"></div>
                <span v-if="requirement.assignee_name" class="person">
                  <span class="ava" :style="{ color: personColor(requirement.assignee_name) }">{{ requirement.assignee_name.charAt(0) }}</span>
                </span>
                <span v-if="requirement.end_date" class="time">{{ formatDate(requirement.end_date) }}</span>
              </div>
            </div>
            <div v-if="column.requirements.length === 0" class="empty-column">
              {{ t('requirement.empty') }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 需求详情抽屉 -->
    <el-drawer
      v-model="showDetailDrawer"
      :title="selectedRequirement?.title"
      size="600px"
    >
      <template v-if="selectedRequirement">
        <el-descriptions :column="2" border>
          <el-descriptions-item :label="t('requirement.pool')">{{ selectedRequirement.pool_name }}</el-descriptions-item>
          <el-descriptions-item :label="t('requirement.category')">
            <el-tag :type="getCategoryType(selectedRequirement.category)" size="small">
              {{ getCategoryLabel(selectedRequirement.category) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('issue.priority')">
            <el-tag :type="getPriorityType(selectedRequirement.priority)" size="small">
              {{ selectedRequirement.priority }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('issue.status')">
            <el-tag :type="getStatusType(selectedRequirement.status)" size="small">
              {{ getStatusLabel(selectedRequirement.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('requirement.source')">{{ selectedRequirement.reporter_name || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('requirement.assignee')">{{ selectedRequirement.assignee_name || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('requirement.creator')">{{ selectedRequirement.creator_name }}</el-descriptions-item>
          <el-descriptions-item :label="t('requirement.linkedIssue')">
            <div v-if="selectedRequirement.converted_issue_key">
              <router-link
                :to="`/issues/${selectedRequirement.converted_issue_key}`"
                class="link"
              >
                {{ selectedRequirement.converted_issue_key }}
              </router-link>
              <el-tag
                v-if="selectedRequirement.converted_issue_status"
                :type="getIssueStatusType(selectedRequirement.converted_issue_status)"
                size="small"
                style="margin-left: 8px;"
              >
                {{ getIssueStatusLabel(selectedRequirement.converted_issue_status) }}
              </el-tag>
            </div>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('requirement.startDate')">{{ formatDateTime(selectedRequirement.start_date) }}</el-descriptions-item>
          <el-descriptions-item :label="t('requirement.endDate')">{{ formatDateTime(selectedRequirement.end_date) }}</el-descriptions-item>
          <el-descriptions-item :label="t('common.createdAt')">{{ selectedRequirement.created_at }}</el-descriptions-item>
        </el-descriptions>

        <div class="description-section">
          <h4>{{ t('issue.description') }}</h4>
          <div class="description-content">{{ selectedRequirement.description || t('requirement.noDescription') }}</div>
        </div>

        <div v-if="selectedRequirement.progress" class="description-section">
          <h4>{{ t('requirement.progress') }}</h4>
          <div class="description-content">{{ selectedRequirement.progress }}</div>
        </div>

        <div v-if="selectedRequirement.result" class="description-section">
          <h4>{{ t('requirement.result') }}</h4>
          <div class="description-content">{{ selectedRequirement.result }}</div>
        </div>

        <div v-if="selectedRequirement.tags && selectedRequirement.tags.length" class="tags-section">
          <h4>{{ t('requirement.tags') }}</h4>
          <div class="tags">
            <el-tag v-for="tag in selectedRequirement.tags" :key="tag" size="small">{{ tag }}</el-tag>
          </div>
        </div>

        <div class="actions-section">
          <el-button-group>
            <el-button v-if="selectedRequirement.status === 'pending_review'" @click="handleStatusChange('planning')">
              {{ t('requirement.startPlanning') }}
            </el-button>
            <el-button v-if="selectedRequirement.status === 'pending_review' || selectedRequirement.status === 'planning' || selectedRequirement.status === 'on_hold'" type="primary" @click="handleStatusChange('in_progress')">
              {{ t('requirement.startWork') }}
            </el-button>
            <el-button v-if="selectedRequirement.status === 'in_progress'" type="success" @click="handleStatusChange('completed')">
              {{ t('requirement.complete') }}
            </el-button>
            <el-button v-if="selectedRequirement.status === 'pending_review' || selectedRequirement.status === 'planning' || selectedRequirement.status === 'in_progress'" type="warning" @click="handleStatusChange('on_hold')">
              {{ t('requirement.hold') }}
            </el-button>
            <el-button v-if="selectedRequirement.status === 'pending_review' || selectedRequirement.status === 'planning' || selectedRequirement.status === 'in_progress'" type="danger" @click="handleStatusChange('rejected')">
              {{ t('requirement.reject') }}
            </el-button>
            <el-button v-if="selectedRequirement.status === 'rejected' || selectedRequirement.status === 'on_hold'" @click="handleStatusChange('pending_review')">
              {{ t('requirement.reevaluate') }}
            </el-button>
            <el-button v-if="selectedRequirement.status !== 'completed' && selectedRequirement.status !== 'rejected' && !selectedRequirement.converted_issue_id" type="primary" @click="handleConvert">
              {{ t('requirement.convert') }}
            </el-button>
          </el-button-group>
        </div>
      </template>
    </el-drawer>

    <!-- 创建需求对话框 -->
    <el-dialog v-model="showCreateDialog" :title="t('requirement.create')" width="700px" @closed="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item :label="t('requirement.pool')" prop="pool_id">
          <el-select v-model="form.pool_id" :placeholder="t('requirement.poolPlaceholder')" style="width: 100%">
            <el-option
              v-for="pool in pools"
              :key="pool.id"
              :label="pool.name"
              :value="pool.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('issue.title')" prop="title">
          <el-input v-model="form.title" :placeholder="t('requirement.titlePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            :placeholder="t('requirement.descPlaceholder')"
          />
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('requirement.category')" prop="category">
              <el-select v-model="form.category" :placeholder="t('requirement.categoryPlaceholder')" style="width: 100%">
                <el-option
                  v-for="cat in categories"
                  :key="cat.name"
                  :label="cat.label"
                  :value="cat.name"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('issue.priority')" prop="priority">
              <el-select v-model="form.priority" :placeholder="t('requirement.priorityPlaceholder')" style="width: 100%">
                <el-option :label="t('issue.priorityMap.P0')" value="P0" />
                <el-option :label="t('issue.priorityMap.P1')" value="P1" />
                <el-option :label="t('issue.priorityMap.P2')" value="P2" />
                <el-option :label="t('issue.priorityMap.P3')" value="P3" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('requirement.assignee')" prop="assignee_id">
              <el-select v-model="form.assignee_id" :placeholder="t('requirement.assigneePlaceholder')" filterable clearable style="width: 100%">
                <el-option
                  v-for="user in users"
                  :key="user.id"
                  :label="user.display_name"
                  :value="user.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="handleCancel">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreateSubmit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  getRequirementKanban,
  getRequirementPoolList,
  createRequirement,
  updateRequirement,
  getRequirementCategories,
} from '@/api/requirement'
import { getAllUsers } from '@/api/user'
import { personColor } from '@/utils/avatar'
import type {
  Requirement,
  RequirementPool,
  CreateRequirementRequest,
  KanbanResponse,
  RequirementStatus,
  RequirementPriority,
  RequirementCategory,
  RequirementCategoryDef,
} from '@/types/requirement'


const { t } = useI18n()

const router = useRouter()

// 数据
const pools = ref<RequirementPool[]>([])
const categories = ref<RequirementCategoryDef[]>([])
const users = ref<any[]>([])
const kanbanData = ref<KanbanResponse>({ group_by: 'status', columns: [], total: 0 })
const loading = ref(false)
const submitting = ref(false)
const showDetailDrawer = ref(false)
const showCreateDialog = ref(false)
const selectedRequirement = ref<Requirement | null>(null)
const formRef = ref<FormInstance>()

// 筛选条件
const filters = reactive({
  pool_id: undefined as number | undefined,
  group_by: 'status' as 'status' | 'priority' | 'assignee' | 'timeline',
})

// 表单
const form = reactive<CreateRequirementRequest>({
  pool_id: undefined,
  title: '',
  description: '',
  priority: 'P2',
  category: 'feature',
  assignee_id: undefined,
})

// 表单验证规则
const rules: FormRules = {
  pool_id: [{
    required: true,
    trigger: 'change',
    validator: (_rule, value, callback) => {
      if (!value || value === 0) {
        callback(new Error(t('requirement.poolRequired')))
      } else {
        callback()
      }
    }
  }],
  title: [{ required: true, message: t('requirement.titleRequired'), trigger: ['blur', 'change'] }],
  priority: [{ required: true, message: t('requirement.priorityRequired'), trigger: 'change' }],
  category: [{ required: true, message: t('requirement.categoryRequired'), trigger: 'change' }],
}

// 状态映射
type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

const getStatusLabel = (status: RequirementStatus) => {
  const map: Record<RequirementStatus, string> = {
    pending_review: t('requirement.statusMap.pending_review'),
    planning: t('requirement.statusMap.planning'),
    in_progress: t('requirement.statusMap.in_progress'),
    completed: t('requirement.statusMap.completed'),
    on_hold: t('requirement.statusMap.on_hold'),
    rejected: t('requirement.statusMap.rejected'),
  }
  return map[status] || status
}

const getStatusType = (status: RequirementStatus): TagType => {
  const map: Record<RequirementStatus, TagType> = {
    pending_review: 'info',
    planning: 'warning',
    in_progress: 'primary',
    completed: 'success',
    on_hold: 'info',
    rejected: 'danger',
  }
  return map[status] || 'info'
}

const getCategoryLabel = (category: RequirementCategory) => {
  const cat = categories.value.find(c => c.name === category)
  return cat?.label || category
}

const getCategoryType = (category: RequirementCategory): TagType => {
  const cat = categories.value.find(c => c.name === category)
  return (cat?.color as TagType) || 'info'
}

// 分组切换（原来是下拉，改成分段控件）
const selectGroupBy = (g: 'status' | 'priority' | 'assignee' | 'timeline') => {
  filters.group_by = g
  loadKanban()
}

const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)',
}

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

const getPriorityType = (priority: RequirementPriority): TagType => {
  const map: Record<RequirementPriority, TagType> = {
    P0: 'danger',
    P1: 'warning',
    P2: 'primary',
    P3: 'info',
  }
  return map[priority] || 'info'
}

// 工单状态映射
const getIssueStatusLabel = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}

const getIssueStatusType = (status: string): TagType => {
  const map: Record<string, TagType> = {
    open: 'info',
    'in-progress': 'warning',
    resolved: 'success',
    closed: 'info',
    reopened: 'danger',
  }
  return map[status] || 'info'
}

// 格式化日期时间
const formatDateTime = (dateStr: string | undefined) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}`
}

// 需求只有日期粒度，卡片上带 00:00 是纯噪音
const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

// 加载看板数据
const loadKanban = async () => {
  loading.value = true
  try {
    const { data } = await getRequirementKanban({
      pool_id: filters.pool_id,
      group_by: filters.group_by,
    })
    kanbanData.value = data.data
  } catch {
    ElMessage.error(t('requirement.loadKanbanFailed'))
  } finally {
    loading.value = false
  }
}

// 加载需求分类
const loadCategories = async () => {
  try {
    const { data } = await getRequirementCategories()
    categories.value = data.data
  } catch {
    // ignored
  }
}

// 加载需求池列表
const loadPools = async () => {
  try {
    const { data } = await getRequirementPoolList({ status: 'active', page_size: 100 })
    pools.value = data.data.items
  } catch {
    // ignored
  }
}

// 加载用户列表
const loadUsers = async () => {
  try {
    const { data } = await getAllUsers()
    users.value = data.data
  } catch {
    // ignored
  }
}

// 点击卡片
const handleCardClick = (requirement: Requirement) => {
  selectedRequirement.value = requirement
  showDetailDrawer.value = true
}

// 状态变更
const handleStatusChange = async (status: RequirementStatus) => {
  if (!selectedRequirement.value) return

  try {
    await updateRequirement(selectedRequirement.value.id, { status })
    ElMessage.success(t('requirement.statusUpdated'))
    showDetailDrawer.value = false
    loadKanban()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('requirement.statusUpdateFailed'))
  }
}

// 转化为工单
const handleConvert = () => {
  if (!selectedRequirement.value) return
  router.push(`/requirements?convert=${selectedRequirement.value.id}`)
}

// 创建需求
const handleCreateSubmit = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    // 再次检查必填字段
    if (!form.pool_id) {
      ElMessage.error(t('requirement.poolRequired'))
      return
    }

    submitting.value = true
    try {
      // 构建请求数据，只包含有值的字段
      const createData: any = {
        pool_id: form.pool_id,
        title: form.title,
        description: form.description || '',
        priority: form.priority,
        category: form.category,
        tags: [],
      }

      // 只添加有值的可选字段
      if (form.assignee_id !== undefined) {
        createData.assignee_id = form.assignee_id
      }

      await createRequirement(createData)
      ElMessage.success(t('common.createSuccess'))
      showCreateDialog.value = false
      resetForm()
      loadKanban()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || t('project.settings.createFailed'))
    } finally {
      submitting.value = false
    }
  })
}

// 创建需求
const handleCreate = () => {
  resetForm()
  showCreateDialog.value = true
}

// 取消对话框
const handleCancel = () => {
  showCreateDialog.value = false
}

// 获取默认分类
const getDefaultCategory = (): RequirementCategory => {
  const defaultCat = categories.value.find(c => c.is_default)
  return defaultCat?.name || categories.value[0]?.name || 'feature'
}

// 重置表单
const resetForm = () => {
  form.pool_id = undefined
  form.title = ''
  form.description = ''
  form.priority = 'P2'
  form.category = getDefaultCategory()
  form.assignee_id = undefined
  formRef.value?.resetFields()
}

onMounted(() => {
  loadKanban()
  loadPools()
  loadUsers()
  loadCategories()
})
</script>

<style scoped lang="scss">
// 看板列与卡片。列表页那套（.page/.card/.pill/.prio）来自 _apple.scss，
// 这里只写看板独有的布局。

.kanban-container { overflow-x: auto; }

.kanban-board {
  display: flex;
  gap: 14px;
  align-items: flex-start;
  min-height: 60vh;
}

.kanban-column {
  flex: 0 0 288px;
  background: var(--td-bg-section);
  border: 1px solid var(--td-border-color);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 220px);
}

.column-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--td-divider-color);
}

.column-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex: 0 0 6px;
  background: var(--td-text-placeholder);

  &.is-in_progress { background: var(--td-color-warning); }
  &.is-completed { background: var(--td-color-success); }
  &.is-rejected { background: var(--td-color-danger); }
}

.column-title { font-size: 13px; font-weight: var(--td-weight-semibold); }
.column-count { font-size: 12px; color: var(--td-text-placeholder); font-variant-numeric: tabular-nums; }

.column-content {
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  overflow-y: auto;
}

// 卡片：发丝线，不用阴影。hover 只换边框与底色。
.kanban-card {
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 8px;
  padding: 10px 12px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 7px;
  transition: var(--td-transition-border), var(--td-transition-bg);

  &:hover {
    border-color: var(--td-color-primary);
    background: var(--td-bg-card-hover);
  }
}

.card-title {
  font-size: 13px;
  line-height: 1.45;
  color: var(--td-text-primary);
}

.card-tags,
.card-foot {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.card-foot { font-size: 11.5px; }
.card-foot .grow { flex: 1; }

.empty-column {
  padding: 28px 12px;
  text-align: center;
  font-size: 12px;
  color: var(--td-text-placeholder);
}


/* 详情抽屉：描述 / 标签 / 状态流转按钮三块 */
.description-section,
.tags-section {
  margin-top: 18px;

  h4 {
    margin: 0 0 6px;
    font-size: 12.5px;
    font-weight: 590;
    color: var(--td-text-secondary);
  }
}

.description-content {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: var(--td-text-primary);
  white-space: pre-wrap;
  word-break: break-word;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.actions-section {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--td-divider-color);
}
</style>
