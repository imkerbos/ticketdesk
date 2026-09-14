<template>
  <div class="field-config-container">
    <!-- 顶部操作栏 -->
    <div class="config-header">
      <div class="header-left">
        <el-select v-model="selectedIssueTypeId" :placeholder="t('project.fieldConfig.selectIssueType')" style="width: 240px" @change="loadFieldScheme">
          <el-option
            v-for="type in issueTypes"
            :key="type.id"
            :label="type.display_name"
            :value="type.id"
          >
            <div class="type-option">
              <div class="type-color" :style="{ background: type.color || 'var(--td-color-primary)' }"></div>
              <span>{{ type.display_name }}</span>
            </div>
          </el-option>
        </el-select>
      </div>
      <div class="header-right">
        <el-button @click="openApplyTemplateDialog">
          <el-icon><DocumentCopy /></el-icon>
          {{ t('project.fieldConfig.applyTemplate') }}
        </el-button>
        <el-button type="primary" @click="openSchemeFieldDialog">
          <el-icon><Plus /></el-icon>
          {{ t('project.fieldConfig.addField') }}
        </el-button>
      </div>
    </div>

    <!-- 字段方案视图 -->
    <div v-loading="schemesLoading" class="schemes-view">
      <div v-if="selectedIssueTypeId" class="scheme-config">
        <div class="scheme-header">
          <div class="scheme-title">
            <span>{{ t('project.fieldConfig.schemeTitle', { name: getSelectedIssueTypeName() }) }}</span>
            <el-tag size="small" type="info">{{ t('project.fieldConfig.fieldCount', { n: currentScheme.length }) }}</el-tag>
          </div>
        </div>

        <!-- 空的时候整张表不渲染：el-table 自带的「暂无数据」会占掉 260px，
             下面还有一个完整的 TdEmptyState，同一张卡里两个空态叠着 -->
        <el-table v-if="currentScheme.length > 0" ref="schemeTableRef" :data="currentScheme" stripe row-key="field_id" class="scheme-table">
          <el-table-column width="44" align="center" class-name="drag-handle-col">
            <template #default>
              <el-icon class="drag-handle" :title="t('project.fieldConfig.dragSort')"><Rank /></el-icon>
            </template>
          </el-table-column>
          <el-table-column :label="t('project.fieldConfig.field')" min-width="200">
            <template #default="{ row }">
              <div class="field-cell">
                <div class="field-icon small" :class="getFieldTypeClass(row.field?.field_type || '')">
                  <el-icon><component :is="getFieldTypeIcon(row.field?.field_type || '')" /></el-icon>
                </div>
                <div class="field-info">
                  <div class="field-name">{{ row.field?.field_name }}</div>
                  <div class="field-key">{{ row.field?.field_key }}</div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('issue.type')" width="120">
            <template #default="{ row }">
              <el-tag size="small" :type="row.field?.is_system ? 'info' : 'success'">
                {{ row.field?.is_system ? t('common.system') : t('project.fieldConfig.custom') }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('project.fieldConfig.required')" width="80" align="center">
            <template #default="{ row }">
              <el-switch
                v-model="row.is_required"
                size="small"
                @change="handleSchemeChange(row)"
              />
            </template>
          </el-table-column>
          <el-table-column :label="t('project.fieldConfig.showOnCreate')" width="100" align="center">
            <template #default="{ row }">
              <el-switch
                v-model="row.is_visible_create"
                size="small"
                @change="handleSchemeChange(row)"
              />
            </template>
          </el-table-column>
          <el-table-column :label="t('project.fieldConfig.showOnEdit')" width="100" align="center">
            <template #default="{ row }">
              <el-switch
                v-model="row.is_visible_edit"
                size="small"
                @change="handleSchemeChange(row)"
              />
            </template>
          </el-table-column>
          <el-table-column :label="t('project.fieldConfig.showOnDetail')" width="100" align="center">
            <template #default="{ row }">
              <el-switch
                v-model="row.is_visible_detail"
                size="small"
                @change="handleSchemeChange(row)"
              />
            </template>
          </el-table-column>
          <el-table-column :label="t('project.fieldConfig.sort')" width="100" align="center">
            <template #default="{ row }">
              <el-input-number
                v-model="row.sort_order"
                :min="0"
                :max="999"
                size="small"
                controls-position="right"
                @change="handleSchemeChange(row)"
              />
            </template>
          </el-table-column>
          <el-table-column :label="t('common.operation')" width="80" align="center">
            <template #default="{ row }">
              <el-button
                size="small"
                type="danger"
                text
                @click="handleRemoveSchemeField(row)"
              >
                {{ t('common.remove') }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <TdEmptyState v-if="currentScheme.length === 0" preset="first-time" :title="t('project.fieldConfig.emptyTitle')" :description="t('project.fieldConfig.emptyDesc')">
          <el-button type="primary" @click="openSchemeFieldDialog">
            <el-icon><Plus /></el-icon>
            {{ t('project.fieldConfig.addField') }}
          </el-button>
        </TdEmptyState>
      </div>

      <TdEmptyState v-else preset="no-data" :title="t('project.fieldConfig.pickIssueType')" />
    </div>

    <!-- 添加字段到方案对话框 -->
    <el-dialog
      v-model="schemeFieldDialogVisible"
      :title="t('project.fieldConfig.addFieldTitle')"
      width="500px"
      destroy-on-close
      class="custom-dialog"
    >
      <div class="scheme-field-list">
        <div
          v-for="field in availableFieldsForScheme"
          :key="field.id"
          class="scheme-field-item"
          :class="{ selected: selectedFieldIds.includes(field.id) }"
          @click="toggleFieldSelection(field.id)"
        >
          <el-checkbox :model-value="selectedFieldIds.includes(field.id)" />
          <div class="field-icon small" :class="getFieldTypeClass(field.field_type)">
            <el-icon><component :is="getFieldTypeIcon(field.field_type)" /></el-icon>
          </div>
          <div class="field-info">
            <div class="field-name">{{ field.field_name }}</div>
            <div class="field-meta">
              <span class="field-key">{{ field.field_key }}</span>
              <el-tag size="small" :type="field.is_system ? 'info' : 'success'">
                {{ field.is_system ? t('common.system') : t('project.fieldConfig.custom') }}
              </el-tag>
            </div>
          </div>
        </div>
        <TdEmptyState v-if="availableFieldsForScheme.length === 0" preset="no-data" :title="t('project.fieldConfig.allAdded')" />
      </div>
      <template #footer>
        <el-button @click="schemeFieldDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button
          type="primary"
          :loading="addToSchemeLoading"
          :disabled="selectedFieldIds.length === 0"
          @click="addFieldsToScheme"
        >
          <el-icon><Check /></el-icon>
          {{ t('project.fieldConfig.addN', { n: selectedFieldIds.length }) }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 套用模板对话框 -->
    <el-dialog
      v-model="applyTemplateDialogVisible"
      :title="t('project.fieldConfig.applyTemplateTitle')"
      width="520px"
      destroy-on-close
    >
      <div class="apply-form">
        <div class="apply-field">
          <label class="apply-label">{{ t('project.fieldConfig.selectTemplate') }}</label>
          <el-select v-model="selectedTemplateId" :placeholder="t('project.fieldConfig.templatePlaceholder')" style="width: 100%">
            <el-option
              v-for="tpl in templates"
              :key="tpl.id"
              :label="tpl.name"
              :value="tpl.id"
            >
              <div class="template-option">
                <span>{{ tpl.name }}</span>
                <el-tag size="small" type="info">{{ t('project.fieldConfig.fieldCount', { n: tpl.item_count }) }}</el-tag>
              </div>
            </el-option>
          </el-select>
        </div>
        <div class="apply-field">
          <label class="apply-label">{{ t('project.fieldConfig.applyMode') }}</label>
          <div class="mode-cards">
            <div
              class="mode-card"
              :class="{ active: applyMode === 'merge' }"
              @click="applyMode = 'merge'"
            >
              <div class="mode-card-radio">
                <div class="radio-dot" :class="{ checked: applyMode === 'merge' }"></div>
              </div>
              <div class="mode-card-content">
                <div class="mode-card-title">{{ t('project.fieldConfig.modeMerge') }}</div>
                <div class="mode-card-desc">{{ t('project.fieldConfig.modeMergeDesc') }}</div>
              </div>
            </div>
            <div
              class="mode-card"
              :class="{ active: applyMode === 'replace' }"
              @click="applyMode = 'replace'"
            >
              <div class="mode-card-radio">
                <div class="radio-dot" :class="{ checked: applyMode === 'replace' }"></div>
              </div>
              <div class="mode-card-content">
                <div class="mode-card-title">{{ t('project.fieldConfig.modeReplace') }}</div>
                <div class="mode-card-desc">{{ t('project.fieldConfig.modeReplaceDesc') }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="applyTemplateDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button
          type="primary"
          :loading="applyTemplateLoading"
          :disabled="!selectedTemplateId"
          @click="handleApplyTemplate"
        >
          {{ t('project.fieldConfig.apply') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import Sortable from 'sortablejs'
import {
  Plus,
  Check,
  Document,
  List,
  Calendar,
  Select,
  User,
  PriceTag,
  Timer,
  Link,
  Grid,
  DocumentCopy,
  Rank,
} from '@element-plus/icons-vue'
import {
  getFields,
  getFieldScheme,
  updateFieldScheme,
  applyTemplate,
} from '@/api/field'
import { getTemplates } from '@/api/admin-field'
import { getProjectIssueTypes } from '@/api/project'
import type {
  FieldDefinition,
  FieldSchemeItem,
  FieldSchemeTemplate,
} from '@/types/field'
import type { ProjectIssueType } from '@/types/project'

const { t } = useI18n()

const props = defineProps<{
  projectKey: string
}>()

const route = useRoute()
const router = useRouter()

// 字段列表
const allFields = ref<FieldDefinition[]>([])

// 字段方案
const schemesLoading = ref(false)
const issueTypes = ref<ProjectIssueType[]>([])
const selectedIssueTypeId = ref<number | undefined>(
  route.query.issueTypeId ? Number(route.query.issueTypeId) : undefined
)
const currentScheme = ref<FieldSchemeItem[]>([])

// 字段方案表格 + 拖拽排序
const schemeTableRef = ref<any>(null)
let sortableInstance: Sortable | null = null

// 同步状态到 URL
const syncStateToUrl = () => {
  const query: Record<string, string> = { ...route.query as Record<string, string> }
  if (selectedIssueTypeId.value) {
    query.issueTypeId = String(selectedIssueTypeId.value)
  } else {
    delete query.issueTypeId
  }
  router.replace({ query })
}

// 字段方案对话框
const schemeFieldDialogVisible = ref(false)
const selectedFieldIds = ref<number[]>([])
const addToSchemeLoading = ref(false)

// 套用模板
const applyTemplateDialogVisible = ref(false)
const templates = ref<FieldSchemeTemplate[]>([])
const selectedTemplateId = ref<number | undefined>()
const applyMode = ref<'replace' | 'merge'>('merge')
const applyTemplateLoading = ref(false)

const availableFieldsForScheme = computed(() => {
  const usedFieldIds = currentScheme.value.map(s => s.field_id)
  return allFields.value.filter(f => !usedFieldIds.includes(f.id))
})

// 加载字段列表
const loadFields = async () => {
  try {
    const { data } = await getFields(props.projectKey)
    allFields.value = data.data || []
  } catch {
    // ignored
  }
}

// 加载工单类型
const loadIssueTypes = async () => {
  try {
    const { data } = await getProjectIssueTypes(props.projectKey)
    issueTypes.value = data.data
    if (issueTypes.value.length > 0 && !selectedIssueTypeId.value) {
      selectedIssueTypeId.value = issueTypes.value[0].id
    }
  } catch {
    // ignored
  }
}

// 加载字段方案
const loadFieldScheme = async () => {
  if (!selectedIssueTypeId.value) return
  schemesLoading.value = true
  try {
    const { data } = await getFieldScheme(props.projectKey, selectedIssueTypeId.value)
    currentScheme.value = data.data || []
    await nextTick()
    setupSortable()
  } catch {
    // ignored
  } finally {
    schemesLoading.value = false
  }
}

// 初始化/重置 SortableJS, 绑到 el-table 的 tbody
const setupSortable = () => {
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
  const tableEl = schemeTableRef.value?.$el as HTMLElement | undefined
  const tbody = tableEl?.querySelector<HTMLElement>('.el-table__body-wrapper tbody')
  if (!tbody) return
  sortableInstance = Sortable.create(tbody, {
    handle: '.drag-handle',
    animation: 150,
    ghostClass: 'drag-ghost-row',
    chosenClass: 'drag-chosen-row',
    onEnd: handleDragEnd,
  })
}

// 拖拽结束: 按新顺序重排 currentScheme + 重赋 sort_order + 批量保存
const handleDragEnd = async (evt: Sortable.SortableEvent) => {
  const { oldIndex, newIndex } = evt
  if (oldIndex === undefined || newIndex === undefined || oldIndex === newIndex) return
  if (!selectedIssueTypeId.value) return

  // 重排数组 (响应式)
  const moved = currentScheme.value.splice(oldIndex, 1)[0]
  currentScheme.value.splice(newIndex, 0, moved)

  // 重新分配 sort_order, 间隔 10 留出手动微调空间
  currentScheme.value.forEach((item, idx) => {
    item.sort_order = idx * 10
  })

  // 批量保存
  try {
    const allItems = currentScheme.value.map(s => ({
      field_id: s.field_id,
      is_required: s.is_required,
      is_visible_create: s.is_visible_create,
      is_visible_edit: s.is_visible_edit,
      is_visible_detail: s.is_visible_detail,
      sort_order: s.sort_order,
      default_value: s.default_value,
    }))
    await updateFieldScheme(props.projectKey, selectedIssueTypeId.value, { items: allItems })
    ElMessage.success(t('project.fieldConfig.sortSaved'))
  } catch {
    ElMessage.error(t('project.fieldConfig.sortSaveFailed'))
    await loadFieldScheme()
  }
}

// 获取选中的工单类型名称
const getSelectedIssueTypeName = () => {
  const type = issueTypes.value.find(t => t.id === selectedIssueTypeId.value)
  return type?.display_name || ''
}

// 字段类型图标映射
const getFieldTypeIcon = (fieldType: string) => {
  const iconMap: Record<string, any> = {
    text: Document,
    textarea: Document,
    number: Grid,
    date: Calendar,
    select: Select,
    multiselect: List,
    user: User,
    version: PriceTag,
    component: Grid,
    label: PriceTag,
    epic_link: Link,
    time_estimate: Timer,
  }
  return iconMap[fieldType] || Document
}

// 字段类型样式类
const getFieldTypeClass = (fieldType: string) => {
  const classMap: Record<string, string> = {
    text: 'text',
    textarea: 'textarea',
    number: 'number',
    date: 'date',
    select: 'select',
    multiselect: 'multiselect',
    user: 'user',
    version: 'version',
    component: 'component',
    label: 'label',
    epic_link: 'epic',
    time_estimate: 'time',
  }
  return classMap[fieldType] || 'default'
}

// 打开添加字段到方案对话框
const openSchemeFieldDialog = () => {
  selectedFieldIds.value = []
  schemeFieldDialogVisible.value = true
}

// 切换字段选择
const toggleFieldSelection = (fieldId: number) => {
  const index = selectedFieldIds.value.indexOf(fieldId)
  if (index > -1) {
    selectedFieldIds.value.splice(index, 1)
  } else {
    selectedFieldIds.value.push(fieldId)
  }
}

// 添加字段到方案
const addFieldsToScheme = async () => {
  if (!selectedIssueTypeId.value || selectedFieldIds.value.length === 0) return
  addToSchemeLoading.value = true
  try {
    const newSchemeItems = selectedFieldIds.value.map((fieldId, index) => ({
      field_id: fieldId,
      is_required: false,
      is_visible_create: true,
      is_visible_edit: true,
      is_visible_detail: true,
      sort_order: currentScheme.value.length + index,
    }))

    const allItems = [
      ...currentScheme.value.map(s => ({
        field_id: s.field_id,
        is_required: s.is_required,
        is_visible_create: s.is_visible_create,
        is_visible_edit: s.is_visible_edit,
        is_visible_detail: s.is_visible_detail,
        sort_order: s.sort_order,
        default_value: s.default_value,
      })),
      ...newSchemeItems,
    ]

    await updateFieldScheme(props.projectKey, selectedIssueTypeId.value, { items: allItems })
    ElMessage.success(t('project.fieldConfig.addSuccess'))
    schemeFieldDialogVisible.value = false
    await loadFieldScheme()
  } catch {
    ElMessage.error(t('project.fieldConfig.addFailed'))
  } finally {
    addToSchemeLoading.value = false
  }
}

// 处理方案变更（失败时回滚）
const handleSchemeChange = async (_item: FieldSchemeItem) => {
  if (!selectedIssueTypeId.value) return
  try {
    const allItems = currentScheme.value.map(s => ({
      field_id: s.field_id,
      is_required: s.is_required,
      is_visible_create: s.is_visible_create,
      is_visible_edit: s.is_visible_edit,
      is_visible_detail: s.is_visible_detail,
      sort_order: s.sort_order,
      default_value: s.default_value,
    }))
    await updateFieldScheme(props.projectKey, selectedIssueTypeId.value, { items: allItems })
    ElMessage.success(t('project.fieldConfig.saved'))
  } catch {
    ElMessage.error(t('project.fieldConfig.saveFailed'))
    // 回滚：重新加载后端数据
    await loadFieldScheme()
  }
}

// 从方案中移除字段
const handleRemoveSchemeField = async (item: FieldSchemeItem) => {
  if (!selectedIssueTypeId.value) return
  try {
    await ElMessageBox.confirm(t('project.fieldConfig.confirmRemove', { name: item.field?.field_name }), t('project.fieldConfig.removeTitle'), {
      type: 'warning',
    })
    const remainingItems = currentScheme.value
      .filter(s => s.field_id !== item.field_id)
      .map(s => ({
        field_id: s.field_id,
        is_required: s.is_required,
        is_visible_create: s.is_visible_create,
        is_visible_edit: s.is_visible_edit,
        is_visible_detail: s.is_visible_detail,
        sort_order: s.sort_order,
        default_value: s.default_value,
      }))
    await updateFieldScheme(props.projectKey, selectedIssueTypeId.value, { items: remainingItems })
    ElMessage.success(t('issue.msg.removeSuccess'))
    await loadFieldScheme()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('project.fieldConfig.removeFailed'))
    }
  }
}

// 打开套用模板对话框
const openApplyTemplateDialog = async () => {
  if (!selectedIssueTypeId.value) {
    ElMessage.warning(t('project.fieldConfig.pickTypeFirst'))
    return
  }
  try {
    const { data } = await getTemplates()
    templates.value = (data.data || []).filter((t: FieldSchemeTemplate) => t.is_active)
  } catch {
    // ignored
  }
  selectedTemplateId.value = undefined
  applyMode.value = 'merge'
  applyTemplateDialogVisible.value = true
}

// 套用模板
const handleApplyTemplate = async () => {
  if (!selectedIssueTypeId.value || !selectedTemplateId.value) return
  const modeName = applyMode.value === 'replace' ? t('project.fieldConfig.modeReplace') : t('project.fieldConfig.modeMerge')
  try {
    await ElMessageBox.confirm(
      t('project.fieldConfig.confirmApply', { mode: modeName, extra: applyMode.value === 'replace' ? t('project.fieldConfig.confirmApplyExtra') : '' }),
      t('project.fieldConfig.applyTitle'),
      { type: 'warning' }
    )
  } catch {
    return
  }

  applyTemplateLoading.value = true
  try {
    await applyTemplate(props.projectKey, selectedIssueTypeId.value, {
      template_id: selectedTemplateId.value,
      mode: applyMode.value,
    })
    ElMessage.success(t('project.fieldConfig.applySuccess'))
    applyTemplateDialogVisible.value = false
    await loadFieldScheme()
  } catch {
    ElMessage.error(t('project.fieldConfig.applyFailed'))
  } finally {
    applyTemplateLoading.value = false
  }
}

// 监听工单类型选择变化
watch(selectedIssueTypeId, () => {
  syncStateToUrl()
})

// 监听 projectKey 变化
watch(() => props.projectKey, async () => {
  await loadFields()
  await loadIssueTypes()
  if (selectedIssueTypeId.value) {
    await loadFieldScheme()
  }
})

onMounted(async () => {
  await loadFields()
  await loadIssueTypes()
  if (selectedIssueTypeId.value) {
    await loadFieldScheme()
  }
})

onBeforeUnmount(() => {
  if (sortableInstance) {
    sortableInstance.destroy()
    sortableInstance = null
  }
})
</script>

<style scoped lang="scss">
.field-config-container {
  min-height: 360px;
}

/* 卡片靠一条发丝线立住，不用阴影（§3.5：elevation 1/2 恒为 none） */
.config-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  padding: 10px 14px;
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
}

.header-right {
  display: flex;
  gap: 8px;
}

.type-option {
  display: flex;
  align-items: center;
  gap: 7px;

  .type-color {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }
}

/* ── 方案视图 ─────────────────────────────────── */
.schemes-view {
  background: var(--td-bg-card);
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  padding: 14px 16px;
}

.scheme-config {
  .scheme-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
  }

  .scheme-title {
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 14px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }
}

.scheme-table {
  border-radius: 8px;
  overflow: hidden;

  :deep(.el-table__header th) {
    background: var(--td-bg-card);
    font-size: 11.5px;
    font-weight: 500;
    color: var(--td-text-secondary);
  }

  :deep(.drag-handle-col) {
    .drag-handle {
      cursor: grab;
      color: var(--td-text-placeholder);
      transition: color 150ms ease-out;

      &:hover {
        color: var(--td-text-regular);
      }

      &:active {
        cursor: grabbing;
      }
    }
  }

  /* 拖拽中的行（SortableJS 加的类） */
  :deep(.drag-ghost-row) {
    opacity: 0.4;
    background: var(--td-bg-section);
  }

  :deep(.drag-chosen-row) {
    background: var(--td-bg-section);
  }
}

@media (prefers-reduced-motion: reduce) {
  .scheme-table :deep(.drag-handle) {
    transition: none;
  }
}

/* ── 字段单元格 ───────────────────────────────── */
.field-cell {
  display: flex;
  align-items: center;
  gap: 9px;
}

/* 字段类型原先是 44px 的实心色块（十种类型十个颜色），
   一张表里每行挂一个，是这一屏最重的东西。类型本身是数据，保留 ——
   但只给图标上色，不再套实心方块。 */
.field-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  flex-shrink: 0;
  color: var(--td-color-primary);

  &.small {
    font-size: 14px;
  }

  &.text,
  &.textarea { color: var(--td-color-primary); }
  &.number { color: var(--td-color-success); }
  &.date { color: var(--td-color-warning); }
  &.select,
  &.multiselect { color: var(--td-cat-2); }
  &.user { color: var(--td-cat-5); }
  &.version,
  &.label { color: var(--td-cat-6); }
  &.component { color: var(--td-text-secondary); }
  &.epic { color: var(--td-cat-4); }
  &.time { color: var(--td-cat-3); }
}

.field-info {
  flex: 1;
  min-width: 0;

  .field-name {
    font-size: 13px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .field-meta {
    display: flex;
    align-items: center;
    gap: 7px;
    margin-top: 2px;
  }

  .field-key {
    font-size: 11.5px;
    color: var(--td-text-placeholder);
    font-family: var(--td-font-mono);
  }

  .field-desc {
    font-size: 11.5px;
    color: var(--td-text-secondary);
    line-height: 1.45;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

/* ── 弹窗 ─────────────────────────────────────── */
.custom-dialog {
  :deep(.el-dialog__header) {
    padding: 14px 18px;
    border-bottom: 1px solid var(--td-divider-color);
  }

  :deep(.el-dialog__body) {
    padding: 18px;
  }
}

.scheme-field-list {
  max-height: 380px;
  overflow-y: auto;
}

.scheme-field-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 10px;
  border: 1px solid var(--td-border-color);
  border-radius: 8px;
  margin-bottom: 6px;
  cursor: pointer;
  transition: border-color 150ms ease-out, background-color 150ms ease-out;

  &:hover {
    background: var(--td-bg-section);
    border-color: var(--td-border-color-dark);
  }

  &.selected {
    border-color: var(--td-color-primary);
    box-shadow: var(--td-focus-ring);
  }
}

/* ── 套用模板弹窗 ─────────────────────────────── */
.apply-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.apply-field {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.apply-label {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--td-text-secondary);
}

.template-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.mode-cards {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.mode-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 11px 13px;
  border: 1px solid var(--td-border-color);
  border-radius: 10px;
  cursor: pointer;
  transition: border-color 150ms ease-out, background-color 150ms ease-out;

  &:hover {
    border-color: var(--td-border-color-dark);
    background: var(--td-bg-section);
  }

  &.active {
    border-color: var(--td-color-primary);
    box-shadow: var(--td-focus-ring);
  }
}

.mode-card-radio {
  padding-top: 1px;
  flex-shrink: 0;
}

.radio-dot {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 1.5px solid var(--td-border-color-dark);
  transition: border-color 150ms ease-out;
  position: relative;

  &.checked {
    border-color: var(--td-color-primary);

    &::after {
      content: '';
      position: absolute;
      top: 2.5px;
      left: 2.5px;
      width: 8px;
      height: 8px;
      border-radius: 50%;
      background: var(--td-color-primary);
    }
  }
}

.mode-card-content {
  flex: 1;
  min-width: 0;
}

.mode-card-title {
  font-size: 13px;
  font-weight: 590;
  letter-spacing: -0.01em;
  color: var(--td-text-primary);
  line-height: 1.3;
}

.mode-card-desc {
  font-size: 11.5px;
  color: var(--td-text-secondary);
  margin-top: 2px;
  line-height: 1.45;
}
</style>
