<template>
  <!-- 原来是卡片网格，改成和其它列表页同一套表格：
       项目的字段是固定的那几项，表格扫读比卡片快，也不用为了排版塞空描述。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('project.listTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('project.create') }}
      </button>
    </div>

    <div class="toolbar">
      <label class="search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input v-model="keyword" type="search" :placeholder="t('project.searchPlaceholder')" @keyup.enter="handleSearch" @search="handleSearch" />
      </label>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col style="width: 92px" /><col style="width: 200px" /><col /><col style="width: 140px" />
            <col style="width: 80px" /><col style="width: 86px" /><col style="width: 96px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('project.key') }}</th>
              <th>{{ t('project.name') }}</th>
              <th>{{ t('project.description') }}</th>
              <th>{{ t('project.lead') }}</th>
              <th>{{ t('project.members') }}</th>
              <!-- 只有确实存在停用项目时才出这一列：全启用时它是一整列破折号，
                   一列全占位符等于这列不该存在 -->
              <th v-if="hasDisabledProject">{{ t('issue.status') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="project in projectList" :key="project.id" @click="handleViewProject(project)">
              <td class="key">{{ project.project_key }}</td>
              <td>{{ project.name }}</td>
              <td class="muted desc">{{ project.description || t('project.noDescription') }}</td>
              <td>
                <span v-if="project.lead_user" class="person">
                  <span class="ava" :style="{ background: 'var(--td-text-placeholder)' }">{{ project.lead_user.display_name?.charAt(0) }}</span>
                  <span>{{ project.lead_user.display_name }}</span>
                </span>
                <span v-else class="muted">{{ t('project.noLead') }}</span>
              </td>
              <td class="time">{{ project.member_count || 0 }}</td>
              <td v-if="hasDisabledProject">
                <!-- 只标停用：启用是常态，每行挂一个绿色「启用」等于什么都没说 -->
                <span v-if="project.status !== 1" class="pill neutral">{{ t('common.disabled') }}</span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click.stop="router.push(`/projects/${project.project_key}/settings`)">
                    {{ t('project.settingsLabel') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="projectList.length === 0 && !loading" class="empty">
          <TdEmptyState preset="first-time" :title="t('project.empty')" :description="t('project.emptyDesc')">
            <button class="btn primary" @click="handleCreate">{{ t('project.createFirst') }}</button>
          </TdEmptyState>
        </div>

        <div v-if="total > pageSize" class="table-foot">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[12, 24, 48]"
            layout="sizes, prev, pager, next"
            @size-change="loadProjects"
            @current-change="loadProjects"
          />
        </div>
      </div>
    </section>

    <!-- 创建/编辑项目对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? t('project.edit') : t('project.create')"
      width="520px"
      destroy-on-close
      class="project-dialog"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('project.key')" prop="project_key">
              <el-input
                v-model="form.project_key"
                :placeholder="t('project.keyPlaceholder')"
                :disabled="isEdit"
                maxlength="10"
              >
                <template #prefix>
                  <el-icon><Key /></el-icon>
                </template>
              </el-input>
              <div class="form-tip">{{ t('project.keyTip') }}</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('project.name')" prop="name">
              <el-input v-model="form.name" :placeholder="t('project.namePlaceholder')" maxlength="50">
                <template #prefix>
                  <el-icon><Folder /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item v-if="!isEdit" :label="t('project.template')">
          <div class="template-cards">
            <div
              class="template-card"
              :class="{ active: form.template === 'standard' }"
              @click="form.template = 'standard'"
            >
              <div class="template-card-icon">
                <el-icon :size="22"><Folder /></el-icon>
              </div>
              <div class="template-card-body">
                <div class="template-card-title">{{ t('project.templateStandard') }}</div>
                <div class="template-card-desc">{{ t('project.templateStandardDesc') }}</div>
              </div>
            </div>
            <div
              class="template-card"
              :class="{ active: form.template === 'blank' }"
              @click="form.template = 'blank'"
            >
              <div class="template-card-icon">
                <el-icon :size="22"><Plus /></el-icon>
              </div>
              <div class="template-card-body">
                <div class="template-card-title">{{ t('project.templateEmpty') }}</div>
                <div class="template-card-desc">{{ t('project.templateEmptyDesc') }}</div>
              </div>
            </div>
          </div>
        </el-form-item>
        <el-form-item :label="t('project.lead')">
          <el-select v-model="form.lead_user_id" :placeholder="t('project.leadPlaceholder')" style="width: 100%" clearable filterable>
            <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="form.description" type="textarea" :rows="3" :placeholder="t('project.descPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submitForm">
          <el-icon><Check /></el-icon>
          {{ isEdit ? t('common.save') : t('common.create') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Folder, Key, Check } from '@element-plus/icons-vue'
import { getProjectList, createProject, updateProject } from '@/api/project'
import { getAllUsers } from '@/api/user'
import type { Project, CreateProjectRequest } from '@/types/project'
import type { UserOption } from '@/types/user'

const { t } = useI18n()

const router = useRouter()

const loading = ref(false)
const projectList = ref<Project[]>([])

// 全部项目都启用时，「状态」列会是一整列破折号 —— 那列就不该出现
const hasDisabledProject = computed(() => projectList.value.some((p) => p.status !== 1))
const total = ref(0)
const page = ref(1)
const pageSize = ref(12)
const keyword = ref('')
const users = ref<UserOption[]>([])

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingProject = ref<Project | null>(null)

const form = reactive<CreateProjectRequest>({
  project_key: '',
  name: '',
  description: '',
  lead_user_id: undefined,
  template: 'standard' })

const rules: FormRules = {
  project_key: [
    { required: true, message: t('project.keyRequired'), trigger: ['blur', 'change'] },
    { pattern: /^[A-Z][A-Z0-9]*$/, message: t('project.keyPattern'), trigger: 'blur' },
    { min: 2, max: 10, message: t('project.keyLength'), trigger: 'blur' },
  ],
  name: [
    { required: true, message: t('project.nameRequired'), trigger: ['blur', 'change'] },
    { max: 50, message: t('project.nameLength'), trigger: 'blur' },
  ] }

const loadProjects = async () => {
  loading.value = true
  try {
    const { data } = await getProjectList({
      page: page.value, page_size: pageSize.value, keyword: keyword.value || undefined })
    projectList.value = data.data.items
    total.value = data.data.total
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const loadUsers = async () => {
  try { const { data } = await getAllUsers(); users.value = data.data } catch { /* ignored */ }
}

const handleSearch = () => { page.value = 1; loadProjects() }

const handleViewProject = (project: Project) => {
  router.push(`/projects/${project.project_key}`)
}

const handleCreate = () => {
  isEdit.value = false
  editingProject.value = null
  Object.assign(form, { project_key: '', name: '', description: '', lead_user_id: undefined, template: 'standard' })
  dialogVisible.value = true
}

const submitForm = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      if (isEdit.value && editingProject.value) {
        await updateProject(editingProject.value.project_key, {
          name: form.name, description: form.description, lead_user_id: form.lead_user_id })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createProject(form)
        ElMessage.success(t('common.createSuccess'))
      }
      dialogVisible.value = false
      loadProjects()
    } catch {
      // ignored
    } finally {
      submitLoading.value = false
    }
  })
}

onMounted(() => { loadProjects(); loadUsers() })
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里，这一页只留对话框相关。

// 描述列允许被挤压：项目描述长短不一，让它吃掉剩余宽度并单行截断
.desc {
  max-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

.form-tip {
  font-size: 12px;
  color: var(--td-text-placeholder);
  margin-top: 4px;
}

.template-cards {
  display: flex;
  gap: 12px;
  width: 100%;
}

.template-card {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1.5px solid var(--td-border-color);
  border-radius: 8px;
  cursor: pointer;
  transition: all 150ms ease-out;

  &:hover {
    border-color: var(--td-color-primary);
    background: var(--td-tag-primary-bg);
  }

  &.active {
    border-color: var(--td-color-primary);
    background: var(--td-tag-primary-bg);
  }
}

/* 只留图标，不套方块（§3.1 不放装饰性图标色块） */
.template-card-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  color: var(--td-text-placeholder);
  flex-shrink: 0;

  .template-card.active & {
    background: var(--td-tag-primary-border);
    color: var(--td-color-primary);
  }
}

.template-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--td-text-primary);
  line-height: 1.4;
}

.template-card-desc {
  font-size: 12px;
  color: var(--td-text-placeholder);
  line-height: 1.4;
  margin-top: 2px;
}

.settings-link {
  text-decoration: none;
  color: inherit;
}
</style>
