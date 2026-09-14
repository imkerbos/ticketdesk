<template>
  <!-- 结构同其它列表页 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('requirement.listTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn secondary" @click="router.push('/requirements/kanban')">{{ t('requirement.kanbanView') }}</button>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('requirement.create') }}
      </button>
    </div>

    <div class="toolbar">
      <label class="search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input v-model="filters.keyword" type="search" :placeholder="t('requirement.keywordPlaceholder')" @keyup.enter="loadData" @search="loadData" />
      </label>
      <el-select v-model="filters.pool_id" :placeholder="t('requirement.pool')" clearable class="filter-select" @change="loadData">
        <el-option v-for="pool in pools" :key="pool.id" :label="pool.name" :value="pool.id" />
      </el-select>
      <el-select v-model="filters.status" :placeholder="t('issue.status')" clearable class="filter-select" @change="loadData">
        <el-option v-for="st in ['pending_review', 'planning', 'in_progress', 'completed', 'on_hold', 'rejected']" :key="st" :label="t(`requirement.statusMap.${st}`)" :value="st" />
      </el-select>
      <el-select v-model="filters.category" :placeholder="t('requirement.category')" clearable class="filter-select" @change="loadData">
        <el-option v-for="cat in categories" :key="cat.name" :label="cat.label" :value="cat.name" />
      </el-select>
      <el-select v-model="filters.priority" :placeholder="t('issue.priority')" clearable class="filter-select-sm" @change="loadData">
        <el-option v-for="pr in ['P0', 'P1', 'P2', 'P3']" :key="pr" :label="pr" :value="pr" />
      </el-select>
      <button class="btn secondary" @click="resetFilters">{{ t('common.reset') }}</button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col style="width: 260px" /><col /><col style="width: 96px" /><col style="width: 96px" />
            <col style="width: 62px" /><col style="width: 96px" /><col style="width: 200px" />
            <col style="width: 96px" /><col style="width: 130px" /><col style="width: 96px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('requirement.requirementName') }}</th>
              <th>{{ t('requirement.requirementDesc') }}</th>
              <th>{{ t('requirement.source') }}</th>
              <th>{{ t('requirement.category') }}</th>
              <th>{{ t('issue.priority') }}</th>
              <th>{{ t('requirement.assignee') }}</th>
              <th>{{ t('requirement.startDate') }} / {{ t('requirement.endDate') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th>{{ t('requirement.linkedIssue') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in requirements" :key="row.id" @click="handleViewDetail(row)">
              <td>
                <div class="title-cell">
                  <span class="txt">{{ row.title }}</span>
                  <span v-for="tag in (row.tags || []).slice(0, 2)" :key="tag" class="pill neutral">{{ tag }}</span>
                </div>
              </td>
              <td class="muted desc">{{ row.description || '-' }}</td>
              <td class="muted">{{ row.reporter_name || '-' }}</td>
              <!-- 分类是维度不是状态，走中性药丸 -->
              <td><span class="pill neutral">{{ getCategoryLabel(row.category) }}</span></td>
              <td>
                <span class="prio">
                  <span class="dot" :style="{ background: priorityColor(row.priority) }"></span>{{ row.priority }}
                </span>
              </td>
              <td class="muted">{{ row.assignee_name || '-' }}</td>
              <td class="time">
                <template v-if="row.start_date || row.end_date">
                  {{ formatDate(row.start_date) }} → {{ formatDate(row.end_date) }}
                </template>
                <template v-else>-</template>
              </td>
              <td><span class="pill" :class="reqStatusTone(row.status)">{{ getStatusLabel(row.status) }}</span></td>
              <td>
                <a v-if="row.converted_issue_key" class="key" @click.stop="router.push(`/issues/${row.converted_issue_key}`)">
                  {{ row.converted_issue_key }}
                </a>
                <span v-else class="muted">-</span>
              </td>
              <td>
                <div class="row-actions" @click.stop>
                  <button class="link-btn" @click="handleEdit(row)">{{ t('common.edit') }}</button>
                  <el-dropdown trigger="click" @command="(cmd: string) => handleRowCommand(row, cmd)">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item v-if="row.status === 'pending_review'" command="planning">{{ t('requirement.transitTo', { name: t('requirement.statusMap.planning') }) }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 'pending_review' || row.status === 'planning' || row.status === 'on_hold'" command="in_progress">{{ t('requirement.transitTo', { name: t('requirement.statusMap.in_progress') }) }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 'in_progress'" command="completed">{{ t('requirement.transitTo', { name: t('requirement.statusMap.completed') }) }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 'pending_review' || row.status === 'planning' || row.status === 'in_progress'" command="on_hold">{{ t('requirement.transitTo', { name: t('requirement.hold') }) }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 'pending_review' || row.status === 'planning' || row.status === 'in_progress'" command="rejected">{{ t('requirement.transitTo', { name: t('requirement.reject') }) }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 'rejected' || row.status === 'on_hold'" command="pending_review">{{ t('requirement.restoreTo', { name: t('requirement.statusMap.pending_review') }) }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 'completed'" command="in_progress">{{ t('requirement.restoreTo', { name: t('requirement.statusMap.in_progress') }) }}</el-dropdown-item>
                        <el-dropdown-item
                          v-if="row.status !== 'completed' && row.status !== 'rejected' && !row.converted_issue_id"
                          command="convert"
                          divided
                        >
                          {{ t('requirement.toIssue') }}
                        </el-dropdown-item>
                        <el-dropdown-item command="delete" divided style="color: var(--td-color-danger);">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && requirements.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('requirement.empty')" />
        </div>

        <div v-if="pagination.total > pagination.page_size" class="table-foot">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.page_size"
            :total="pagination.total"
            :page-sizes="[10, 20, 50, 100]"
            layout="sizes, prev, pager, next"
            @size-change="loadData"
            @current-change="loadData"
          />
        </div>
      </div>
    </section>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="showCreateDialog"
      :title="editingRequirement ? t('requirement.edit') : t('requirement.create')"
      width="700px"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item :label="t('requirement.pool')" prop="pool_id">
          <el-select v-model="form.pool_id" :placeholder="t('requirement.poolPlaceholder')" :disabled="!!editingRequirement" style="width: 100%">
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
            <el-form-item :label="t('requirement.source')" prop="reporter_id">
              <el-select v-model="form.reporter_id" :placeholder="t('requirement.reporterPlaceholder')" filterable clearable style="width: 100%">
                <el-option
                  v-for="user in users"
                  :key="user.id"
                  :label="user.display_name"
                  :value="user.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
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
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('requirement.startDate')" prop="start_date">
              <el-date-picker
                v-model="form.start_date"
                type="datetime"
                :placeholder="t('requirement.datePlaceholder')"
                value-format="YYYY-MM-DD HH:mm:ss"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('requirement.endDate')" prop="end_date">
              <el-date-picker
                v-model="form.end_date"
                type="datetime"
                :placeholder="t('requirement.datePlaceholder')"
                value-format="YYYY-MM-DD HH:mm:ss"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item v-if="editingRequirement" :label="t('requirement.progress')" prop="progress">
          <el-input
            v-model="form.progress"
            type="textarea"
            :rows="3"
            :placeholder="t('requirement.progressPlaceholder')"
          />
        </el-form-item>
        <el-form-item v-if="editingRequirement" :label="t('requirement.result')" prop="result">
          <el-input
            v-model="form.result"
            type="textarea"
            :rows="3"
            :placeholder="t('requirement.resultPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('requirement.targetProject')" prop="target_project_id">
          <el-select v-model="form.target_project_id" :placeholder="t('requirement.targetProjectPlaceholder')" clearable style="width: 100%">
            <el-option
              v-for="project in projects"
              :key="project.id"
              :label="project.name"
              :value="project.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('requirement.tags')" prop="tags">
          <el-select
            v-model="form.tags"
            multiple
            filterable
            allow-create
            default-first-option
            :placeholder="t('requirement.tagsPlaceholder')"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="handleCancel">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 转化为工单对话框 -->
    <el-dialog v-model="showConvertDialog" :title="t('requirement.convert')" width="500px">
      <el-form ref="convertFormRef" :model="convertForm" :rules="convertRules" label-width="100px">
        <el-form-item :label="t('requirement.convertProject')" prop="project_key">
          <el-select v-model="convertForm.project_key" :placeholder="t('requirement.projectPlaceholder')" style="width: 100%" @change="loadIssueTypes">
            <el-option
              v-for="project in projects"
              :key="project.project_key"
              :label="project.name"
              :value="project.project_key"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('alert.rules.issueType')" prop="issue_type_id">
          <el-select v-model="convertForm.issue_type_id" :placeholder="t('requirement.issueTypePlaceholder')" style="width: 100%">
            <el-option
              v-for="type in issueTypes"
              :key="type.id"
              :label="type.display_name"
              :value="type.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('requirement.convertAssignee')" prop="assignee_id">
          <el-select v-model="convertForm.assignee_id" :placeholder="t('requirement.assigneePlaceholder')" filterable clearable style="width: 100%">
            <el-option
              v-for="user in users"
              :key="user.id"
              :label="user.display_name"
              :value="user.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showConvertDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="converting" @click="handleConvertSubmit">{{ t('requirement.convertAction') }}</el-button>
      </template>
    </el-dialog>

    <!-- 详情抽屉 -->
    <el-drawer v-if="selectedRequirement" v-model="showDetailDrawer" :title="t('requirement.detailTitle')" size="50%">
      <el-descriptions :column="2" border>
        <el-descriptions-item :label="t('issue.title')" :span="2">{{ selectedRequirement.title }}</el-descriptions-item>
        <el-descriptions-item :label="t('requirement.pool')" :span="2">{{ selectedRequirement.pool_name }}</el-descriptions-item>
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
        <el-descriptions-item :label="t('common.createdAt')" :span="2">{{ formatDateTime(selectedRequirement.created_at) }}</el-descriptions-item>
      </el-descriptions>

      <div class="description-section" style="margin-top: 20px;">
        <h4>{{ t('requirement.requirementDesc') }}</h4>
        <p>{{ selectedRequirement.description || t('requirement.noDescription') }}</p>
      </div>

      <div v-if="selectedRequirement.progress" class="description-section" style="margin-top: 20px;">
        <h4>{{ t('requirement.progress') }}</h4>
        <p>{{ selectedRequirement.progress }}</p>
      </div>

      <div v-if="selectedRequirement.result" class="description-section" style="margin-top: 20px;">
        <h4>{{ t('requirement.result') }}</h4>
        <p>{{ selectedRequirement.result }}</p>
      </div>

      <div v-if="selectedRequirement.tags && selectedRequirement.tags.length" class="tags-section" style="margin-top: 20px;">
        <h4>{{ t('requirement.tags') }}</h4>
        <el-tag v-for="tag in selectedRequirement.tags" :key="tag" style="margin-right: 8px;">{{ tag }}</el-tag>
      </div>

      <template #footer>
        <el-button @click="showDetailDrawer = false">{{ t('common.close') }}</el-button>
        <el-button type="primary" @click="handleEdit(selectedRequirement)">{{ t('common.edit') }}</el-button>
        <el-button type="danger" @click="handleDelete(selectedRequirement)">{{ t('common.delete') }}</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  getRequirementList,
  getRequirementPoolList,
  createRequirement,
  updateRequirement,
  deleteRequirement,
  convertToIssue,
  getRequirementCategories,
} from '@/api/requirement'
import { getAllProjects, getProjectIssueTypes } from '@/api/project'
import { getAllUsers } from '@/api/user'
import type {
  Requirement,
  RequirementPool,
  CreateRequirementRequest,
  UpdateRequirementRequest,
  RequirementStatus,
  RequirementPriority,
  RequirementCategory,
  RequirementCategoryDef,
} from '@/types/requirement'


const { t } = useI18n()

const router = useRouter()
const route = useRoute()

// 数据
const requirements = ref<Requirement[]>([])
const pools = ref<RequirementPool[]>([])
const categories = ref<RequirementCategoryDef[]>([])
const projects = ref<any[]>([])
const users = ref<any[]>([])
const issueTypes = ref<any[]>([])
const loading = ref(false)
const submitting = ref(false)
const converting = ref(false)
const showCreateDialog = ref(false)
const showConvertDialog = ref(false)
const showDetailDrawer = ref(false)
const editingRequirement = ref<Requirement | null>(null)
const convertingRequirement = ref<Requirement | null>(null)
const selectedRequirement = ref<Requirement | null>(null)
const formRef = ref<FormInstance>()
const convertFormRef = ref<FormInstance>()

// 筛选条件
const filters = reactive({
  pool_id: undefined as number | undefined,
  status: undefined as RequirementStatus | undefined,
  priority: undefined as RequirementPriority | undefined,
  category: undefined as RequirementCategory | undefined,
  keyword: '',
})

// 分页
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})

// 表单
const form = reactive<CreateRequirementRequest & { progress?: string; result?: string }>({
  pool_id: undefined,
  title: '',
  description: '',
  priority: 'P2',
  category: 'feature',
  reporter_id: undefined,
  assignee_id: undefined,
  start_date: undefined,
  end_date: undefined,
  target_project_id: undefined,
  tags: [],
  progress: undefined,
  result: undefined,
})

// 转化表单
const convertForm = reactive({
  project_key: '' as string,
  issue_type_id: undefined as number | undefined,
  assignee_id: undefined as number | undefined,
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

const convertRules: FormRules = {
  project_key: [{
    required: true,
    trigger: 'change',
    validator: (_rule, value, callback) => {
      if (!value) {
        callback(new Error(t('requirement.projectRequired')))
      } else {
        callback()
      }
    }
  }],
  issue_type_id: [{
    required: true,
    trigger: 'change',
    validator: (_rule, value, callback) => {
      if (!value || value === 0) {
        callback(new Error(t('requirement.issueTypeRequired')))
      } else {
        callback()
      }
    }
  }],
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

// 需求状态的药丸色调：进行中给橙，完成给绿，驳回给红底，其余中性
const reqStatusTone = (s: string) =>
  s === 'in_progress' ? 'orange' : s === 'completed' ? 'green' : s === 'rejected' ? 'sla' : 'neutral'

const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)',
}

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

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

// 格式化日期
const formatDate = (dateStr: string | undefined) => {
  if (!dateStr) return '-'
  const date = new Date(dateStr)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

// 加载数据
const loadData = async () => {
  loading.value = true
  try {
    const { data } = await getRequirementList({
      ...filters,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    requirements.value = data.data.items
    pagination.total = data.data.total
  } catch {
    ElMessage.error(t('requirement.loadListFailed'))
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

// 加载项目列表
const loadProjects = async () => {
  try {
    const { data } = await getAllProjects()
    projects.value = data.data
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

// 加载工单类型
const loadIssueTypes = async () => {
  if (!convertForm.project_key) {
    issueTypes.value = []
    return
  }
  try {
    const { data } = await getProjectIssueTypes(convertForm.project_key)
    issueTypes.value = data.data
  } catch {
    // ignored
  }
}

// 重置筛选条件
const resetFilters = () => {
  filters.pool_id = undefined
  filters.status = undefined
  filters.priority = undefined
  filters.category = undefined
  filters.keyword = ''
  pagination.page = 1
  loadData()
}

// 查看详情
const handleViewDetail = (requirement: Requirement) => {
  selectedRequirement.value = requirement
  showDetailDrawer.value = true
}

// 处理行操作命令（更多下拉菜单）
const handleRowCommand = (requirement: Requirement, command: string) => {
  if (command === 'convert') {
    handleConvert(requirement)
  } else if (command === 'delete') {
    handleDelete(requirement)
  } else {
    handleStatusChange(requirement, command as RequirementStatus)
  }
}

// 变更需求状态
const handleStatusChange = async (requirement: Requirement, status: RequirementStatus) => {
  try {
    await updateRequirement(requirement.id, { status })
    ElMessage.success(t('requirement.statusUpdatedTo', { name: getStatusLabel(status) }))
    loadData()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('requirement.statusUpdateFailed'))
  }
}

// 编辑
const handleEdit = (requirement: Requirement) => {
  editingRequirement.value = requirement
  form.pool_id = requirement.pool_id
  form.title = requirement.title
  form.description = requirement.description
  form.priority = requirement.priority
  form.category = requirement.category
  form.reporter_id = requirement.reporter_id
  form.assignee_id = requirement.assignee_id
  form.start_date = requirement.start_date
  form.end_date = requirement.end_date
  form.progress = requirement.progress
  form.result = requirement.result
  form.target_project_id = requirement.target_project_id
  form.tags = requirement.tags || []
  showCreateDialog.value = true
}

// 转化为工单
const handleConvert = (requirement: Requirement) => {
  convertingRequirement.value = requirement
  // 从目标项目 ID 查找对应的 project_key
  const targetProject = requirement.target_project_id
    ? projects.value.find(p => p.id === requirement.target_project_id)
    : undefined
  convertForm.project_key = targetProject?.project_key || ''
  convertForm.issue_type_id = undefined
  convertForm.assignee_id = requirement.assignee_id
  if (convertForm.project_key) {
    loadIssueTypes()
  }
  showConvertDialog.value = true
}

// 删除
const handleDelete = async (requirement: Requirement) => {
  try {
    await ElMessageBox.confirm(t('requirement.confirmDelete', { name: requirement.title }), t('issue.msg.tipTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })

    await deleteRequirement(requirement.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadData()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || t('issue.msg.deleteFailed2'))
    }
  }
}

// 提交表单
const handleSubmit = async () => {
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
      if (editingRequirement.value) {
        // 更新 — 可空字段用 null 替代 undefined，确保 JSON 序列化时字段不被省略
        const updateData: Record<string, any> = {
          title: form.title,
          description: form.description,
          priority: form.priority,
          category: form.category,
          reporter_id: form.reporter_id ?? null,
          assignee_id: form.assignee_id ?? null,
          start_date: form.start_date || null,
          end_date: form.end_date || null,
          progress: form.progress ?? null,
          result: form.result ?? null,
          target_project_id: form.target_project_id ?? null,
          tags: form.tags ?? [],
        }
        await updateRequirement(editingRequirement.value.id, updateData as UpdateRequirementRequest)
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        // 创建 - 构建请求数据，只包含有值的字段
        const createData: any = {
          pool_id: form.pool_id,
          title: form.title,
          description: form.description || '',
          priority: form.priority,
          category: form.category,
          tags: form.tags || [],
        }

        // 只添加有值的可选字段
        if (form.reporter_id !== undefined) {
          createData.reporter_id = form.reporter_id
        }
        if (form.assignee_id !== undefined) {
          createData.assignee_id = form.assignee_id
        }
        if (form.start_date) {
          createData.start_date = form.start_date
        }
        if (form.end_date) {
          createData.end_date = form.end_date
        }
        if (form.target_project_id !== undefined) {
          createData.target_project_id = form.target_project_id
        }

        await createRequirement(createData)
        ElMessage.success(t('common.createSuccess'))
      }

      showCreateDialog.value = false
      resetForm()
      loadData()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || t('common.operationFailed'))
    } finally {
      submitting.value = false
    }
  })
}

// 提交转化
const handleConvertSubmit = async () => {
  if (!convertFormRef.value || !convertingRequirement.value) return

  await convertFormRef.value.validate(async (valid) => {
    if (!valid) return

    // 再次检查必填字段
    if (!convertForm.project_key) {
      ElMessage.error(t('requirement.projectRequired'))
      return
    }
    if (!convertForm.issue_type_id) {
      ElMessage.error(t('requirement.issueTypeRequired'))
      return
    }

    converting.value = true
    try {
      // 构建请求数据，只包含有值的字段
      const convertData: any = {
        project_key: convertForm.project_key,
        issue_type_id: convertForm.issue_type_id,
      }

      if (convertForm.assignee_id !== undefined) {
        convertData.assignee_id = convertForm.assignee_id
      }

      const { data } = await convertToIssue(convertingRequirement.value!.id, convertData)

      // 显示成功消息，包含工单号
      ElMessage.success(t('requirement.convertSuccess', { key: data.data.issue_key }))

      showConvertDialog.value = false
      loadData()

      // 询问是否跳转到工单详情
      ElMessageBox.confirm(t('requirement.convertAsk', { key: data.data.issue_key }), t('requirement.convertSuccessTitle'), {
        confirmButtonText: t('requirement.viewIssue'),
        cancelButtonText: t('requirement.stayHere'),
        type: 'success',
      }).then(() => {
        router.push(`/issues/${data.data.issue_key}`)
      }).catch(() => {
        // 用户选择留在当前页，不做任何操作
      })
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || t('requirement.convertFailed'))
    } finally {
      converting.value = false
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
  editingRequirement.value = null
  form.pool_id = undefined
  form.title = ''
  form.description = ''
  form.priority = 'P2'
  form.category = getDefaultCategory()
  form.reporter_id = undefined
  form.assignee_id = undefined
  form.start_date = undefined
  form.end_date = undefined
  form.progress = undefined
  form.result = undefined
  form.target_project_id = undefined
  form.tags = []
  formRef.value?.resetFields()
}

onMounted(async () => {
  // 从 URL 参数获取 pool_id
  if (route.query.pool_id) {
    filters.pool_id = Number(route.query.pool_id)
  }

  await Promise.all([loadData(), loadPools(), loadProjects(), loadUsers(), loadCategories()])

  // 处理看板跳转过来的转化请求
  if (route.query.convert) {
    const convertId = Number(route.query.convert)
    const target = requirements.value.find(r => r.id === convertId)
    if (target && !target.converted_issue_id && target.status !== 'completed' && target.status !== 'rejected') {
      handleConvert(target)
    }
  }
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里。

.desc { max-width: 0; overflow: hidden; text-overflow: ellipsis; }


/* 详情抽屉里的描述 / 进度 / 结果 / 标签分块 */
.description-section,
.tags-section {
  h4 {
    margin: 0 0 6px;
    font-size: 12.5px;
    font-weight: 590;
    color: var(--td-text-secondary);
  }

  p {
    margin: 0;
    font-size: 13px;
    line-height: 1.7;
    color: var(--td-text-primary);
    white-space: pre-wrap;
    word-break: break-word;
  }
}
</style>
