<template>
  <!-- 字段管理：两个 tab（字段 / 模板），都改成表格。
       原来字段是一排排的卡片行、模板是卡片网格，字段固定的东西用表格更好扫。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('field.title') }}</h1>
      <div class="grow"></div>
      <button v-if="activeTab === 'fields'" class="btn primary" @click="openCreateFieldDialog">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('field.create') }}
      </button>
      <button v-else class="btn primary" @click="openCreateTemplateDialog">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('field.createTemplate') }}
      </button>
    </div>

    <div class="tabs" role="tablist">
      <button class="tab" role="tab" :aria-selected="activeTab === 'fields'" @click="activeTab = 'fields'">
        {{ t('field.globalFields') }}
      </button>
      <button class="tab" role="tab" :aria-selected="activeTab === 'templates'" @click="activeTab = 'templates'">
        {{ t('field.templates') }}
      </button>
    </div>

    <!-- ========== 字段 ========== -->
    <template v-if="activeTab === 'fields'">
      <div class="toolbar">
        <label class="search">
          <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
          <input v-model="fieldSearchText" type="search" :placeholder="t('field.searchPlaceholder')" />
        </label>
      </div>

      <!-- 系统字段：可折叠，默认收起——它们不可编辑，多数时候不需要看 -->
      <section class="card">
        <div class="card-head as-toggle" @click="systemFieldsCollapsed = !systemFieldsCollapsed">
          <svg class="chev" :class="{ 'is-collapsed': systemFieldsCollapsed }" viewBox="0 0 24 24" aria-hidden="true"><path d="m18 15-6-6-6 6" /></svg>
          <h2>{{ t('field.systemFields') }}</h2>
          <span class="n">{{ filteredSystemFields.length }}</span>
          <div class="grow"></div>
          <span class="muted hint">{{ t('field.systemHint') }}</span>
        </div>

        <div v-show="!systemFieldsCollapsed" v-loading="fieldsLoading" class="table-wrap">
          <table class="issues">
            <!-- 弹性列给「字段名称」：类型列只放「单选」「多行文本」这种短词，
                 之前让它吃掉全部富余宽度，结果类型列 810px、里面 90% 是空白 -->
            <colgroup><col /><col style="width: 200px" /><col style="width: 140px" /><col style="width: 96px" /><col style="width: 72px" /></colgroup>
            <thead>
              <tr>
                <th>{{ t('field.fieldName') }}</th>
                <th>{{ t('field.fieldKey') }}</th>
                <th>{{ t('issue.type') }}</th>
                <th>{{ t('field.sort') }}</th>
                <th>{{ t('issue.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="field in filteredSystemFields" :key="field.id">
                <td>{{ field.field_name }}</td>
                <td class="key static">{{ field.field_key }}</td>
                <td class="muted">{{ t(getFieldTypeLabelKey(field.field_type)) }}</td>
                <td>
                  <el-input v-model.number="field.sort_order" type="number" size="small" class="sort-input" @change="handleUpdateFieldSort(field)" />
                </td>
                <td>
                  <el-switch v-model="field.is_active" size="small" @change="handleToggleFieldActive(field)" />
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="!fieldsLoading && filteredSystemFields.length === 0" class="empty">
            <TdEmptyState preset="no-result" :title="t('field.noSystemMatch')" />
          </div>
        </div>
      </section>

      <!-- 自定义字段 -->
      <section class="card">
        <div class="card-head">
          <h2>{{ t('field.customFields') }}</h2>
          <span class="n">{{ filteredCustomFields.length }}</span>
          <div class="grow"></div>
          <span class="muted hint">{{ t('field.customHint') }}</span>
        </div>

        <div v-loading="fieldsLoading" class="table-wrap">
          <table v-if="filteredCustomFields.length > 0" class="issues">
            <colgroup><col style="width: 220px" /><col style="width: 200px" /><col style="width: 110px" /><col /><col style="width: 72px" /><col style="width: 110px" /></colgroup>
            <thead>
              <tr>
                <th>{{ t('field.fieldName') }}</th>
                <th>{{ t('field.fieldKey') }}</th>
                <th>{{ t('issue.type') }}</th>
                <th>{{ t('issue.description') }}</th>
                <th>{{ t('issue.status') }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="field in filteredCustomFields" :key="field.id">
                <td>{{ field.field_name }}</td>
                <td class="key static">{{ field.field_key }}</td>
                <td class="muted">{{ t(getFieldTypeLabelKey(field.field_type)) }}</td>
                <td class="muted desc">{{ field.description || '-' }}</td>
                <td>
                  <el-switch v-model="field.is_active" size="small" @change="handleToggleFieldActive(field)" />
                </td>
                <td>
                  <div class="row-actions">
                    <button class="link-btn" @click="handleEditField(field)">{{ t('common.edit') }}</button>
                    <el-dropdown trigger="click">
                      <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item @click="handleDeleteField(field)">{{ t('common.delete') }}</el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>

          <div v-if="!fieldsLoading && filteredCustomFields.length === 0 && !fieldSearchText" class="empty">
            <TdEmptyState preset="first-time" :title="t('field.noCustom')" :description="t('field.noCustomDesc')">
              <button class="btn primary" @click="openCreateFieldDialog">{{ t('field.createFirst') }}</button>
            </TdEmptyState>
          </div>
          <div v-if="!fieldsLoading && filteredCustomFields.length === 0 && fieldSearchText" class="empty">
            <TdEmptyState preset="no-result" :title="t('field.noCustomMatch')" />
          </div>
        </div>
      </section>
    </template>

    <!-- ========== 模板 ========== -->
    <section v-else class="card">
      <div class="card-head">
        <h2>{{ t('field.templates') }}</h2>
        <div class="grow"></div>
        <span class="muted hint">{{ t('field.templateHint') }}</span>
      </div>

      <div v-loading="templatesLoading" class="table-wrap">
        <table v-if="templates.length > 0" class="issues">
          <colgroup><col style="width: 240px" /><col /><col style="width: 110px" /><col style="width: 86px" /><col style="width: 130px" /></colgroup>
          <thead>
            <tr>
              <th>{{ t('common.name') }}</th>
              <th>{{ t('issue.description') }}</th>
              <th>{{ t('field.fieldCountLabel') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="tpl in templates" :key="tpl.id" @click="openTemplateDetail(tpl)">
              <td>{{ tpl.name }}</td>
              <td class="muted desc">{{ tpl.description || t('project.noDescription') }}</td>
              <td class="time">{{ tpl.item_count || 0 }}</td>
              <td>
                <span class="pill" :class="tpl.is_active ? 'green' : 'neutral'">
                  {{ tpl.is_active ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click.stop="openTemplateDetail(tpl)">{{ t('field.configFields') }}</button>
                  <el-dropdown trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditTemplate(tpl)">{{ t('field.editInfo') }}</el-dropdown-item>
                        <el-dropdown-item divided @click="handleDeleteTemplate(tpl)">{{ t('field.deleteTemplate') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!templatesLoading && templates.length === 0" class="empty">
          <TdEmptyState preset="first-time" :title="t('field.noTemplate')" :description="t('field.noTemplateDesc')">
            <button class="btn primary" @click="openCreateTemplateDialog">{{ t('field.createFirstTemplate') }}</button>
          </TdEmptyState>
        </div>
      </div>
    </section>

    <!-- 创建/编辑字段对话框 -->
    <el-dialog
      v-model="fieldDialogVisible"
      :title="isEditFieldMode ? t('field.edit') : t('field.createGlobal')"
      width="560px"
      destroy-on-close
      class="custom-dialog"
    >
      <el-form ref="fieldFormRef" :model="fieldForm" :rules="fieldFormRules" label-position="top">
        <el-form-item :label="t('field.fieldKey')" prop="field_key">
          <el-input
            v-model="fieldForm.field_key"
            :placeholder="t('field.fieldKeyPlaceholder')"
            :disabled="isEditFieldMode"
          />
        </el-form-item>
        <el-form-item :label="t('field.fieldName')" prop="field_name">
          <el-input v-model="fieldForm.field_name" :placeholder="t('field.fieldNamePlaceholder')" />
        </el-form-item>
        <el-form-item v-if="!isEditFieldMode" :label="t('field.fieldType')" prop="field_type">
          <el-select v-model="fieldForm.field_type" :placeholder="t('field.fieldTypePlaceholder')" style="width: 100%">
            <el-option
              v-for="(labelKey, value) in fieldTypeOptions"
              :key="value"
              :label="t(labelKey)"
              :value="value"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="fieldForm.description" type="textarea" :rows="2" :placeholder="t('field.descPlaceholder')" />
        </el-form-item>
        <el-form-item
          v-if="fieldForm.field_type === 'select' || fieldForm.field_type === 'multiselect'"
          :label="t('field.options')"
        >
          <div class="options-editor">
            <div v-for="(opt, idx) in editingOptions" :key="idx" class="option-row">
              <el-input v-model="opt.label" :placeholder="t('field.optionLabelPlaceholder')" size="small" class="option-input" />
              <el-input v-model="opt.value" :placeholder="t('field.optionValuePlaceholder')" size="small" class="option-input" />
              <el-button type="danger" text size="small" @click="editingOptions.splice(idx, 1)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
            <el-button size="small" @click="editingOptions.push({ value: '', label: '' })">
              <el-icon><Plus /></el-icon> {{ t('field.addOption') }}
            </el-button>
          </div>
        </el-form-item>
        <el-form-item :label="t('field.defaultValue')">
          <el-input v-model="fieldForm.default_value" :placeholder="t('field.defaultValuePlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="fieldDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="fieldSubmitLoading" @click="submitFieldForm">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 创建/编辑模板对话框 -->
    <el-dialog
      v-model="templateDialogVisible"
      :title="isEditTemplateMode ? t('field.editTemplate') : t('field.createTemplateFull')"
      width="500px"
      destroy-on-close
      class="custom-dialog"
    >
      <el-form ref="templateFormRef" :model="templateForm" :rules="templateFormRules" label-position="top">
        <el-form-item :label="t('field.templateName')" prop="name">
          <el-input v-model="templateForm.name" :placeholder="t('field.templateNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="templateForm.description" type="textarea" :rows="3" :placeholder="t('field.templateDescPlaceholder')" />
        </el-form-item>
        <el-form-item v-if="isEditTemplateMode" :label="t('issue.status')">
          <el-switch v-model="templateForm.is_active" :active-text="t('common.enabled')" :inactive-text="t('common.disabled')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="templateDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="templateSubmitLoading" @click="submitTemplateForm">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 模板字段配置对话框 -->
    <el-dialog
      v-model="templateDetailVisible"
      :title="t('field.templateConfigTitle', { name: currentTemplate?.name || '' })"
      width="940px"
      destroy-on-close
      class="custom-dialog"
    >
      <div v-loading="templateDetailLoading" class="template-detail-content">
        <div class="template-detail-header">
          <div class="template-detail-info">
            <span class="detail-field-count">{{ t('field.fieldCount', { n: templateItems.length }) }}</span>
            <el-tag v-if="templateItemsDirty" type="warning" size="small" effect="plain">{{ t('field.unsaved') }}</el-tag>
          </div>
          <el-button type="primary" size="small" @click="openAddTemplateFieldDialog">
            <el-icon><Plus /></el-icon>
            {{ t('field.addField') }}
          </el-button>
        </div>

        <el-table v-if="templateItems.length > 0" :data="templateItems" stripe class="detail-table" row-class-name="detail-row">
          <el-table-column :label="t('field.field')" min-width="200">
            <template #default="{ row }">
              <div class="field-cell">
                <div class="field-card-info">
                  <div class="field-card-name">{{ row.field?.field_name }}</div>
                  <div class="field-card-key">{{ row.field?.field_key }}</div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column :label="t('field.required')" width="70" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.is_required" size="small" @change="markTemplateItemsDirty" />
            </template>
          </el-table-column>
          <el-table-column :label="t('field.onCreate')" width="70" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.is_visible_create" size="small" @change="markTemplateItemsDirty" />
            </template>
          </el-table-column>
          <el-table-column :label="t('field.onEdit')" width="70" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.is_visible_edit" size="small" @change="markTemplateItemsDirty" />
            </template>
          </el-table-column>
          <el-table-column :label="t('field.onDetail')" width="70" align="center">
            <template #default="{ row }">
              <el-switch v-model="row.is_visible_detail" size="small" @change="markTemplateItemsDirty" />
            </template>
          </el-table-column>
          <el-table-column :label="t('field.sort')" width="90" align="center">
            <template #default="{ row }">
              <el-input
                v-model.number="row.sort_order"
                type="number"
                size="small"
                class="sort-input"
                @change="markTemplateItemsDirty"
              />
            </template>
          </el-table-column>
          <el-table-column label="" width="60" align="center">
            <template #default="{ $index }">
              <el-button size="small" type="danger" text circle @click="removeTemplateItem($index)">
                <el-icon><Close /></el-icon>
              </el-button>
            </template>
          </el-table-column>
        </el-table>
        <TdEmptyState v-if="templateItems.length === 0" preset="no-data" :title="t('field.emptyItems')" />
      </div>
      <template #footer>
        <el-button @click="templateDetailVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="templateItemsSaving" :disabled="!templateItemsDirty" @click="saveTemplateItems">
          {{ t('field.saveConfig') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 添加模板字段选择对话框 -->
    <el-dialog
      v-model="addTemplateFieldVisible"
      :title="t('field.pickFieldsTitle')"
      width="620px"
      destroy-on-close
      class="custom-dialog"
    >
      <el-input
        v-model="templateFieldSearch"
        :placeholder="t('field.searchFieldPlaceholder')"
        :prefix-icon="Search"
        clearable
        style="margin-bottom: 16px"
      />
      <el-table
        :data="availableFieldsForTemplate"
        max-height="400"
        class="select-field-table"
        @selection-change="handleTemplateFieldSelectionChange"
      >
        <el-table-column type="selection" width="46" />
        <el-table-column :label="t('field.field')" min-width="200">
          <template #default="{ row }">
            <div class="field-cell">
              <div class="field-card-info">
                <div class="field-card-name">{{ row.field_name }}</div>
                <div class="field-card-key">{{ row.field_key }}</div>
              </div>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('issue.type')" width="110">
          <template #default="{ row }">
            <div class="field-type-tag inline">
              {{ t(getFieldTypeLabelKey(row.field_type)) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('field.source')" width="90" align="center">
          <template #default="{ row }">
            <el-tag size="small" :type="row.is_system ? 'info' : 'success'" effect="plain" round>
              {{ row.is_system ? t('common.system') : t('field.custom') }}
            </el-tag>
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="addTemplateFieldVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :disabled="selectedFieldsForTemplate.length === 0" @click="confirmAddTemplateFields">
          {{ t('field.addSelected', { n: selectedFieldsForTemplate.length }) }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Plus, Search, Close, Delete } from '@element-plus/icons-vue'
import {
  getGlobalFields,
  createGlobalField,
  updateGlobalField,
  deleteGlobalField,
  getFieldUsage,
  getTemplates,
  createTemplate,
  getTemplate,
  updateTemplate,
  deleteTemplate,
  updateTemplateItems } from '@/api/admin-field'
import type {
  FieldDefinition,
  FieldSchemeTemplate,
  FieldSchemeTemplateItem,
  FieldUsage,
  TemplateItemInput } from '@/types/field'
import { getFieldTypeLabelKey, FieldType } from '@/types/field'

const { t } = useI18n()

// ============ 路由 & Tab 持久化 ============

const route = useRoute()
const router = useRouter()

const validTabs = ['fields', 'templates']
const initialTab = validTabs.includes(route.query.tab as string) ? (route.query.tab as string) : 'fields'
const activeTab = ref(initialTab)

watch(activeTab, (tab) => {
  router.replace({ query: { ...route.query, tab } })
})

// ============ 状态 ============

const systemFieldsCollapsed = ref(false)

// 全局字段
const fieldsLoading = ref(false)
const allFields = ref<FieldDefinition[]>([])
const fieldSearchText = ref('')

// 模板
const templatesLoading = ref(false)
const templates = ref<FieldSchemeTemplate[]>([])

// ============ 计算属性 ============

const filteredSystemFields = computed(() => {
  const search = fieldSearchText.value.toLowerCase()
  return allFields.value
    .filter(f => f.is_system)
    .filter(f => !search || f.field_name.toLowerCase().includes(search) || f.field_key.toLowerCase().includes(search))
    .sort((a, b) => a.sort_order - b.sort_order)
})

const filteredCustomFields = computed(() => {
  const search = fieldSearchText.value.toLowerCase()
  return allFields.value
    .filter(f => !f.is_system)
    .filter(f => !search || f.field_name.toLowerCase().includes(search) || f.field_key.toLowerCase().includes(search))
    .sort((a, b) => a.sort_order - b.sort_order)
})

// ============ 加载数据 ============

const loadFields = async () => {
  fieldsLoading.value = true
  try {
    const { data } = await getGlobalFields()
    allFields.value = data.data || []
  } catch {
    ElMessage.error(t('field.loadFieldsFailed'))
  } finally {
    fieldsLoading.value = false
  }
}

const loadTemplates = async () => {
  templatesLoading.value = true
  try {
    const { data } = await getTemplates()
    templates.value = data.data || []
  } catch {
    ElMessage.error(t('field.loadTemplatesFailed'))
  } finally {
    templatesLoading.value = false
  }
}

// ============ 字段 CRUD ============

const fieldDialogVisible = ref(false)
const isEditFieldMode = ref(false)
const editingFieldId = ref<number | null>(null)
const fieldSubmitLoading = ref(false)
const fieldFormRef = ref<FormInstance>()
const fieldForm = ref({
  field_key: '',
  field_name: '',
  field_type: '' as string,
  description: '',
  options: '',
  default_value: '' })
// 可视化选项编辑列表
const editingOptions = ref<{ value: string; label: string }[]>([])

// 将选项列表序列化为 JSON（提交时使用）
function serializeOptions(): string {
  const valid = editingOptions.value
    .filter(o => o.label.trim())
    .map(o => ({
      value: o.value.trim() || o.label.trim(),
      label: o.label.trim() }))
  return valid.length > 0 ? JSON.stringify(valid) : ''
}

// 从 JSON 解析选项列表（编辑时使用）
function deserializeOptions(json: string): { value: string; label: string }[] {
  if (!json) return []
  try {
    const parsed = JSON.parse(json)
    if (!Array.isArray(parsed)) return []
    return parsed.map((item: any) => {
      if (typeof item === 'string') return { value: item, label: item }
      return {
        value: String(item.value ?? item.label ?? ''),
        label: String(item.label ?? item.value ?? '') }
    })
  } catch {
    return []
  }
}

const fieldFormRules: FormRules = {
  field_key: [
    { required: true, message: t('field.fieldKeyRequired'), trigger: ['blur', 'change'] },
    { pattern: /^[a-zA-Z][a-zA-Z0-9_]*$/, message: t('field.fieldKeyPattern'), trigger: 'blur' },
  ],
  field_name: [{ required: true, message: t('field.fieldNameRequired'), trigger: ['blur', 'change'] }],
  field_type: [{ required: true, message: t('field.fieldTypeRequired'), trigger: 'change' }] }

// 字段类型下拉选项 (与 FieldType 常量同源, 避免类型遗漏)
const fieldTypeOptions: Record<string, string> = {
  [FieldType.TEXT]: getFieldTypeLabelKey(FieldType.TEXT),
  [FieldType.TEXTAREA]: getFieldTypeLabelKey(FieldType.TEXTAREA),
  [FieldType.NUMBER]: getFieldTypeLabelKey(FieldType.NUMBER),
  [FieldType.DATE]: getFieldTypeLabelKey(FieldType.DATE),
  [FieldType.DATETIME]: getFieldTypeLabelKey(FieldType.DATETIME),
  [FieldType.SELECT]: getFieldTypeLabelKey(FieldType.SELECT),
  [FieldType.MULTISELECT]: getFieldTypeLabelKey(FieldType.MULTISELECT),
  [FieldType.USER]: getFieldTypeLabelKey(FieldType.USER),
  [FieldType.MULTIUSER]: getFieldTypeLabelKey(FieldType.MULTIUSER),
  [FieldType.LABEL]: getFieldTypeLabelKey(FieldType.LABEL),
  [FieldType.URL]: getFieldTypeLabelKey(FieldType.URL),
  [FieldType.CHECKBOX]: getFieldTypeLabelKey(FieldType.CHECKBOX) }

const openCreateFieldDialog = () => {
  isEditFieldMode.value = false
  editingFieldId.value = null
  fieldForm.value = {
    field_key: '',
    field_name: '',
    field_type: '',
    description: '',
    options: '',
    default_value: '' }
  editingOptions.value = []
  fieldDialogVisible.value = true
}

const handleEditField = (field: FieldDefinition) => {
  isEditFieldMode.value = true
  editingFieldId.value = field.id
  fieldForm.value = {
    field_key: field.field_key,
    field_name: field.field_name,
    field_type: field.field_type,
    description: field.description || '',
    options: field.options || '',
    default_value: field.default_value || '' }
  editingOptions.value = deserializeOptions(field.options)
  fieldDialogVisible.value = true
}

const submitFieldForm = async () => {
  if (!fieldFormRef.value) return
  await fieldFormRef.value.validate(async (valid) => {
    if (!valid) return
    fieldSubmitLoading.value = true
    try {
      // select/multiselect 类型的选项从可视化编辑器序列化
      const optionsJson = (fieldForm.value.field_type === 'select' || fieldForm.value.field_type === 'multiselect')
        ? serializeOptions()
        : fieldForm.value.options

      if (isEditFieldMode.value && editingFieldId.value) {
        await updateGlobalField(editingFieldId.value, {
          field_name: fieldForm.value.field_name,
          description: fieldForm.value.description,
          options: optionsJson,
          default_value: fieldForm.value.default_value })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createGlobalField({
          field_key: fieldForm.value.field_key,
          field_name: fieldForm.value.field_name,
          field_type: fieldForm.value.field_type as any,
          description: fieldForm.value.description,
          options: optionsJson,
          default_value: fieldForm.value.default_value })
        ElMessage.success(t('common.createSuccess'))
      }
      fieldDialogVisible.value = false
      await loadFields()
    } catch {
      ElMessage.error(isEditFieldMode.value ? t('field.updateFieldFailed') : t('field.createFieldFailed'))
    } finally {
      fieldSubmitLoading.value = false
    }
  })
}

const handleToggleFieldActive = async (field: FieldDefinition) => {
  try {
    await updateGlobalField(field.id, { is_active: field.is_active })
  } catch {
    field.is_active = !field.is_active
    ElMessage.error(t('field.updateStatusFailed'))
  }
}

const handleUpdateFieldSort = async (field: FieldDefinition) => {
  try {
    await updateGlobalField(field.id, { sort_order: field.sort_order })
  } catch {
    ElMessage.error(t('field.updateSortFailed'))
    await loadFields()
  }
}

const handleDeleteField = async (field: FieldDefinition) => {
  try {
    const { data } = await getFieldUsage(field.id)
    const usage: FieldUsage = data.data

    let warningMsg = t('field.confirmDeleteField', { name: field.field_name })
    if (usage.scheme_count > 0 || usage.value_count > 0 || usage.template_count > 0) {
      warningMsg = t('field.inUseWarning', {
        name: field.field_name,
        schemes: usage.scheme_count,
        values: usage.value_count,
        templates: usage.template_count })
    }

    await ElMessageBox.confirm(warningMsg, t('issue.list.deleteTitle'), {
      type: 'warning',
      confirmButtonText: t('field.confirmDeleteBtn'),
      cancelButtonText: t('common.cancel') })

    await deleteGlobalField(field.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    await loadFields()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('field.deleteFieldFailed'))
    }
  }
}

// ============ 模板 CRUD ============

const templateDialogVisible = ref(false)
const isEditTemplateMode = ref(false)
const editingTemplateId = ref<number | null>(null)
const templateSubmitLoading = ref(false)
const templateFormRef = ref<FormInstance>()
const templateForm = ref({
  name: '',
  description: '',
  is_active: true })
const templateFormRules: FormRules = {
  name: [{ required: true, message: t('field.templateNameRequired'), trigger: ['blur', 'change'] }] }

const openCreateTemplateDialog = () => {
  isEditTemplateMode.value = false
  editingTemplateId.value = null
  templateForm.value = { name: '', description: '', is_active: true }
  templateDialogVisible.value = true
}

const handleEditTemplate = (tpl: FieldSchemeTemplate) => {
  isEditTemplateMode.value = true
  editingTemplateId.value = tpl.id
  templateForm.value = {
    name: tpl.name,
    description: tpl.description || '',
    is_active: tpl.is_active }
  templateDialogVisible.value = true
}

const submitTemplateForm = async () => {
  if (!templateFormRef.value) return
  await templateFormRef.value.validate(async (valid) => {
    if (!valid) return
    templateSubmitLoading.value = true
    try {
      if (isEditTemplateMode.value && editingTemplateId.value) {
        await updateTemplate(editingTemplateId.value, {
          name: templateForm.value.name,
          description: templateForm.value.description,
          is_active: templateForm.value.is_active })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createTemplate({
          name: templateForm.value.name,
          description: templateForm.value.description })
        ElMessage.success(t('common.createSuccess'))
      }
      templateDialogVisible.value = false
      await loadTemplates()
    } catch {
      ElMessage.error(isEditTemplateMode.value ? t('field.updateTemplateFailed') : t('field.createTemplateFailed'))
    } finally {
      templateSubmitLoading.value = false
    }
  })
}

const handleDeleteTemplate = async (tpl: FieldSchemeTemplate) => {
  try {
    await ElMessageBox.confirm(t('field.confirmDeleteTemplate', { name: tpl.name }), t('issue.list.deleteTitle'), {
      type: 'warning' })
    await deleteTemplate(tpl.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    await loadTemplates()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('field.deleteTemplateFailed'))
    }
  }
}

// ============ 模板字段配置 ============

const templateDetailVisible = ref(false)
const templateDetailLoading = ref(false)
const currentTemplate = ref<FieldSchemeTemplate | null>(null)
const templateItems = ref<FieldSchemeTemplateItem[]>([])
const templateItemsDirty = ref(false)
const templateItemsSaving = ref(false)

const openTemplateDetail = async (tpl: FieldSchemeTemplate) => {
  currentTemplate.value = tpl
  templateDetailVisible.value = true
  templateDetailLoading.value = true
  templateItemsDirty.value = false
  try {
    const { data } = await getTemplate(tpl.id)
    templateItems.value = data.data?.items || []
  } catch {
    ElMessage.error(t('field.loadTemplateDetailFailed'))
  } finally {
    templateDetailLoading.value = false
  }
}

const markTemplateItemsDirty = () => {
  templateItemsDirty.value = true
}

const removeTemplateItem = (index: number) => {
  templateItems.value.splice(index, 1)
  templateItemsDirty.value = true
}

const saveTemplateItems = async () => {
  if (!currentTemplate.value) return
  templateItemsSaving.value = true
  try {
    const items: TemplateItemInput[] = templateItems.value.map((item, idx) => ({
      field_id: item.field_id,
      is_required: item.is_required,
      is_visible_create: item.is_visible_create,
      is_visible_edit: item.is_visible_edit,
      is_visible_detail: item.is_visible_detail,
      sort_order: item.sort_order ?? idx,
      default_value: item.default_value || '' }))
    await updateTemplateItems(currentTemplate.value.id, items)
    ElMessage.success(t('common.saveSuccess'))
    templateItemsDirty.value = false
    await loadTemplates()
  } catch {
    ElMessage.error(t('field.saveConfigFailed'))
  } finally {
    templateItemsSaving.value = false
  }
}

// 添加字段到模板
const addTemplateFieldVisible = ref(false)
const templateFieldSearch = ref('')
const selectedFieldsForTemplate = ref<FieldDefinition[]>([])

const availableFieldsForTemplate = computed(() => {
  const existingFieldIds = new Set(templateItems.value.map(i => i.field_id))
  const search = templateFieldSearch.value.toLowerCase()
  return allFields.value
    .filter(f => f.is_active && !existingFieldIds.has(f.id))
    .filter(f => !search || f.field_name.toLowerCase().includes(search) || f.field_key.toLowerCase().includes(search))
})

const openAddTemplateFieldDialog = () => {
  templateFieldSearch.value = ''
  selectedFieldsForTemplate.value = []
  addTemplateFieldVisible.value = true
  if (allFields.value.length === 0) {
    loadFields()
  }
}

const handleTemplateFieldSelectionChange = (selection: FieldDefinition[]) => {
  selectedFieldsForTemplate.value = selection
}

const confirmAddTemplateFields = () => {
  const maxSort = templateItems.value.reduce((max, item) => Math.max(max, item.sort_order || 0), 0)
  for (let i = 0; i < selectedFieldsForTemplate.value.length; i++) {
    const field = selectedFieldsForTemplate.value[i]
    templateItems.value.push({
      id: 0,
      template_id: currentTemplate.value?.id || 0,
      field_id: field.id,
      field: field,
      is_required: false,
      is_visible_create: true,
      is_visible_edit: true,
      is_visible_detail: true,
      sort_order: maxSort + i + 1,
      default_value: '' })
  }
  templateItemsDirty.value = true
  addTemplateFieldVisible.value = false
}

// ============ 字段图标/样式工具 ============

// ============ 初始化 ============

onMounted(async () => {
  await Promise.all([loadFields(), loadTemplates()])
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里，这一页只留几处单元格和对话框。

// 可折叠的卡片头（系统字段那一段）
.card-head.as-toggle {
  cursor: pointer;
  user-select: none;

  &:hover { background: var(--td-bg-card-hover); }

  .chev {
    width: 12px;
    height: 12px;
    stroke: var(--td-text-placeholder);
    stroke-width: 1.8;
    fill: none;
    transition: transform var(--td-duration-fast) var(--td-ease-out);
  }

  .chev.is-collapsed { transform: rotate(180deg); }
}

.hint { font-size: 12px; }
.desc { max-width: 0; overflow: hidden; text-overflow: ellipsis; }
.sort-input { width: 72px; }

.template-detail-content {
  min-height: 200px;
}

.template-detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.template-detail-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.detail-field-count {
  font-size: 13px;
  color: var(--td-text-secondary);
  font-weight: 500;
}

.detail-table {
  :deep(.el-table__header th) {
    background: var(--td-bg-page);
    font-weight: 600;
    font-size: 13px;
    color: var(--td-text-secondary);
  }
}

.field-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

// ============ 折叠动画 ============
.collapse-enter-active,
.collapse-leave-active {
  transition: all 150ms ease-out;
  overflow: hidden;
}
.collapse-enter-from,
.collapse-leave-to {
  opacity: 0;
  max-height: 0;
}
.collapse-enter-to,
.collapse-leave-from {
  opacity: 1;
  max-height: 2000px;
}

// ============ 对话框美化 ============
.custom-dialog {
  :deep(.el-dialog) {
    border-radius: 12px;
  }

  :deep(.el-dialog__header) {
    padding: 20px 24px 16px;
    border-bottom: 1px solid var(--td-divider-color);
    margin-right: 0;
  }

  :deep(.el-dialog__body) {
    padding: 20px 24px;
  }

  :deep(.el-dialog__footer) {
    padding: 16px 24px 20px;
    border-top: 1px solid var(--td-divider-color);
  }
}

.options-editor {
  width: 100%;
}

.option-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.option-input {
  flex: 1;
}

/* 方案模板明细里的字段单元格 —— 这几个类模板里在用，样式是重写时删掉的 */
.field-card-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.field-card-name {
  font-size: 13px;
  color: var(--td-text-primary);
}

.field-card-key {
  font-size: 11.5px;
  font-family: var(--td-font-mono);
  color: var(--td-text-placeholder);
}

.field-type-tag {
  display: inline-flex;
  align-items: center;
  font-size: 11.5px;
  color: var(--td-text-secondary);
  background: var(--td-bg-section);
  border-radius: 5px;
  padding: 1px 7px;

  &.inline { margin: 0; }
}

.select-field-table :deep(.el-table__cell) {
  padding: 5px 0;
}
</style>
