<template>
  <el-dialog
    :model-value="modelValue"
    :title="title || t('issue.createIssue')"
    width="640px"
    destroy-on-close
    class="create-issue-dialog"
    @update:model-value="$emit('update:modelValue', $event)"
    @open="handleDialogOpen"
    @closed="handleDialogClose"
  >
    <el-form
      ref="createFormRef"
      :model="createForm"
      :rules="createRules"
      label-position="top"
      class="create-issue-form"
    >
      <el-row :gutter="20">
        <el-col :span="12">
          <el-form-item :label="t('issue.project')" prop="project_key">
            <el-input
              v-if="fixedProjectKey"
              :model-value="fixedProjectKey"
              disabled
            />
            <el-select
              v-else
              v-model="createForm.project_key"
              :placeholder="t('component.createIssue.selectProject')"
              style="width: 100%"
              @change="handleProjectChange"
            >
              <el-option
                v-for="p in projectList"
                :key="p.project_key"
                :label="p.name"
                :value="p.project_key"
              />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item :label="t('issue.type')" prop="issue_type_id">
            <el-select
              v-model="createForm.issue_type_id"
              :placeholder="t('component.createIssue.selectType')"
              style="width: 100%"
              :disabled="!effectiveProjectKey"
              @change="handleIssueTypeChange"
            >
              <el-option
                v-for="type in issueTypes"
                :key="type.id"
                :label="type.display_name"
                :value="type.id"
              />
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>

      <el-form-item :label="t('issue.title')" prop="title">
        <el-input
          v-model="createForm.title"
          :placeholder="parentId ? t('component.createIssue.subtaskTitlePlaceholder') : t('component.createIssue.titlePlaceholder')"
          maxlength="200"
          show-word-limit
        />
      </el-form-item>

      <!-- 字段方案驱动 -->
      <div v-if="fieldScheme.length > 0" v-loading="fieldSchemeLoading" class="custom-fields-section">
        <el-row :gutter="20">
          <el-col
            v-for="item in fieldScheme"
            :key="item.field_id"
            :span="getFieldColSpan(item.field?.field_type)"
          >
            <el-form-item :required="item.is_required">
              <template #label>
                <span>{{ item.field?.field_name }}</span>
                <el-tooltip v-if="item.field?.description" :content="item.field?.description" placement="top">
                  <el-icon class="field-hint"><QuestionFilled /></el-icon>
                </el-tooltip>
              </template>
              <FieldRenderer
                v-if="item.field"
                v-model="customFieldValues[item.field_id]"
                :field="item.field"
                :scheme="item"
                :project-key="effectiveProjectKey"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </div>

      <el-form-item :label="t('component.createIssue.attachments')">
        <PendingAttachmentList
          ref="pendingListRef"
          v-model="pendingFiles"
        />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="createLoading" @click="submitCreate">
        <el-icon><Check /></el-icon>
        {{ title || t('issue.createIssue') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, nextTick } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Check, QuestionFilled } from '@element-plus/icons-vue'
import { createIssue } from '@/api/issue'
import { getAllProjects, getProjectIssueTypes } from '@/api/project'
import { getFieldScheme } from '@/api/field'
import type { CreateIssueRequest } from '@/types/issue'
import type { Project, ProjectIssueType } from '@/types/project'
import type { FieldSchemeItem } from '@/types/field'
import { FieldRenderer } from '@/components/field'
import { extractBuiltinFields } from '@/utils/builtin-fields'
import PendingAttachmentList from '@/components/attachment/PendingAttachmentList.vue'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  fixedProjectKey?: string
  defaultProjectKey?: string
  parentId?: number
  projects?: Project[]
}>(), {
  title: '',
  fixedProjectKey: '',
  defaultProjectKey: '',
  parentId: undefined,
  projects: undefined,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'created', issueKey: string): void
}>()

// 内部状态
const createLoading = ref(false)
const createFormRef = ref<FormInstance>()
const internalProjects = ref<Project[]>([])
const issueTypes = ref<ProjectIssueType[]>([])
const fieldScheme = ref<FieldSchemeItem[]>([])
const customFieldValues = ref<Record<number, any>>({})
const fieldSchemeLoading = ref(false)
const pendingFiles = ref<File[]>([])
const pendingListRef = ref<InstanceType<typeof PendingAttachmentList> | null>(null)

const createForm = reactive({
  project_key: '',
  issue_type_id: undefined as number | undefined,
  title: '',
})

const createRules = computed<FormRules>(() => {
  const rules: FormRules = {
    issue_type_id: [{ required: true, message: t('component.createIssue.typeRequired'), trigger: 'change' }],
    title: [{ required: true, message: t('component.createIssue.titleRequired'), trigger: ['blur', 'change'] }],
  }
  if (!props.fixedProjectKey) {
    rules.project_key = [{ required: true, message: t('component.createIssue.projectRequired'), trigger: 'change' }]
  }
  return rules
})

const projectList = computed(() => props.projects ?? internalProjects.value)
const effectiveProjectKey = computed(() => props.fixedProjectKey || createForm.project_key)

// 粘贴监听：在对话框内监听粘贴事件，将文件传递给附件组件
const handlePaste = (e: ClipboardEvent) => {
  const files = e.clipboardData?.files
  if (!files || files.length === 0) return

  // 命中输入框/textarea 时仅处理纯图片，避免抢占文本粘贴
  const target = e.target as HTMLElement | null
  const inEditable =
    target &&
    (target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.isContentEditable)
  if (inEditable) {
    const allImages = Array.from(files).every(f => f.type.startsWith('image/'))
    if (!allImages) return
    e.preventDefault()
  }

  pendingListRef.value?.addFiles(Array.from(files))
}

const bindPasteListener = () => {
  document.querySelector('.create-issue-dialog')?.addEventListener('paste', handlePaste as unknown as (ev: Event) => void)
}

const unbindPasteListener = () => {
  document.querySelector('.create-issue-dialog')?.removeEventListener('paste', handlePaste as unknown as (ev: Event) => void)
}

// 对话框打开时重置状态
const handleDialogOpen = async () => {
  createForm.project_key = props.defaultProjectKey || ''
  createForm.issue_type_id = undefined
  createForm.title = ''
  fieldScheme.value = []
  customFieldValues.value = {}
  issueTypes.value = []
  pendingFiles.value = []

  // 加载项目列表（仅当外部未传入且非固定项目时）
  if (!props.fixedProjectKey && !props.projects) {
    try {
      const { data } = await getAllProjects()
      internalProjects.value = data.data
    } catch {
      // ignored
    }
  }

  // 预选项目时自动加载工单类型
  const initialProject = props.fixedProjectKey || props.defaultProjectKey
  if (initialProject) {
    await handleProjectChange(initialProject)
  }

  await nextTick()
  bindPasteListener()
}

// 对话框关闭时解绑监听并清理附件
const handleDialogClose = () => {
  unbindPasteListener()
  pendingFiles.value = []
}

// 项目变更
const handleProjectChange = async (projectKey: string) => {
  createForm.issue_type_id = undefined
  fieldScheme.value = []
  customFieldValues.value = {}
  if (!projectKey) {
    issueTypes.value = []
    return
  }
  try {
    const { data } = await getProjectIssueTypes(projectKey)
    issueTypes.value = data.data
  } catch {
    // ignored
  }
}

// 工单类型变更
const handleIssueTypeChange = async (issueTypeId: number) => {
  fieldScheme.value = []
  customFieldValues.value = {}
  if (!issueTypeId || !effectiveProjectKey.value) return
  fieldSchemeLoading.value = true
  try {
    const { data } = await getFieldScheme(effectiveProjectKey.value, issueTypeId)
    const schemeItems = data.data || []
    fieldScheme.value = schemeItems.filter(item => item.is_visible_create)
    // 初始化字段默认值
    const arrayFieldTypes = ['multiselect', 'label', 'component']
    fieldScheme.value.forEach(item => {
      const fieldType = item.field?.field_type || ''
      if (arrayFieldTypes.includes(fieldType)) {
        if (item.default_value) {
          try {
            const parsed = JSON.parse(item.default_value)
            customFieldValues.value[item.field_id] = Array.isArray(parsed) ? parsed : []
          } catch {
            customFieldValues.value[item.field_id] = []
          }
        } else {
          customFieldValues.value[item.field_id] = []
        }
      } else if (item.default_value) {
        customFieldValues.value[item.field_id] = item.default_value
      }
    })
  } catch {
    // ignored
  } finally {
    fieldSchemeLoading.value = false
  }
}

// 字段列宽
const getFieldColSpan = (fieldType?: string): number => {
  if (fieldType === 'textarea' || fieldType === 'epic_link') {
    return 24
  }
  return 12
}

// 根据是否有附件构造请求体（JSON 或 multipart/form-data）
function buildPayload(req: CreateIssueRequest, files: File[]): CreateIssueRequest | FormData {
  if (files.length === 0) return req
  const fd = new FormData()
  fd.append('data', JSON.stringify(req))
  for (const f of files) {
    fd.append('files', f, f.name)
  }
  return fd
}

// 提交创建
const submitCreate = async () => {
  if (!createFormRef.value) return
  await createFormRef.value.validate(async (valid) => {
    if (!valid) return
    createLoading.value = true
    try {
      if (!createForm.issue_type_id) {
        ElMessage.error(t('component.createIssue.typeNotSelected'))
        createLoading.value = false
        return
      }

      // 校验必填字段
      for (const item of fieldScheme.value) {
        if (item.is_required) {
          const val = customFieldValues.value[item.field_id]
          const isEmpty = val === undefined || val === null || val === '' || (Array.isArray(val) && val.length === 0)
          if (isEmpty) {
            ElMessage.error(t('component.createIssue.fieldRequired', { field: item.field?.field_name }))
            createLoading.value = false
            return
          }
        }
      }

      // 分离内置字段和扩展字段
      const { builtinValues, customFields } = extractBuiltinFields(fieldScheme.value, customFieldValues.value)

      const requestData: CreateIssueRequest = {
        project_key: props.fixedProjectKey || createForm.project_key,
        issue_type_id: createForm.issue_type_id,
        title: createForm.title,
        description: builtinValues.description || '',
        priority: builtinValues.priority || 'P2',
        assignee_id: builtinValues.assignee_id || undefined,
        planned_start_date: builtinValues.planned_start_date || undefined,
        planned_end_date: builtinValues.planned_end_date || undefined,
        epic_id: builtinValues.epic_id || undefined,
        parent_id: props.parentId || undefined,
        custom_fields: customFields.length > 0 ? customFields : undefined,
      }
      const { data } = await createIssue(buildPayload(requestData, pendingFiles.value))
      ElMessage.success(props.parentId ? t('component.createIssue.subtaskCreated') : t('common.createSuccess'))
      emit('update:modelValue', false)
      emit('created', data.data.issue_key)
    } catch {
      // ignored
    } finally {
      createLoading.value = false
    }
  })
}
</script>

<style scoped lang="scss">
.create-issue-form {
  .el-form-item {
    margin-bottom: 20px;
  }

  .custom-fields-section {
    margin-top: 8px;

    .field-hint {
      margin-left: 4px;
      font-size: 14px;
      color: var(--td-text-disabled);
      cursor: help;
      vertical-align: middle;

      &:hover {
        color: var(--td-color-primary);
      }
    }
  }
}
</style>

<style lang="scss">
.create-issue-dialog {
  .el-dialog__header {
    padding: 16px 20px;
    margin-right: 0;
  }

  .el-dialog__body {
    padding: 20px;
    max-height: 70vh;
    overflow-y: auto;

    &::-webkit-scrollbar {
      width: 6px;
    }

    &::-webkit-scrollbar-thumb {
      background: var(--td-border-color);
      border-radius: 3px;
    }

    &::-webkit-scrollbar-track {
      background: transparent;
    }
  }

  .el-dialog__footer {
    padding: 12px 20px 16px;
  }

  .el-form-item__label {
    font-weight: 500;
    color: var(--td-text-secondary);
    font-size: 13px;
    padding-bottom: 4px;
  }

  .el-input, .el-select, .el-textarea {
    --el-input-bg-color: var(--td-input-bg);
  }

  .el-input__inner, .el-textarea__inner {
    &:focus {
      border-color: var(--td-color-primary);
      box-shadow: var(--td-focus-ring);
    }
  }
}
</style>
