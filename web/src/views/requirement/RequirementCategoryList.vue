<template>
  <!-- 结构同其它列表页 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('requirement.categoryTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('requirement.categoryList.create') }}
      </button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup><col style="width: 72px" /><col style="width: 200px" /><col /><col style="width: 96px" /><col style="width: 96px" /><col style="width: 120px" /></colgroup>
          <thead>
            <tr>
              <th>{{ t('requirement.categoryList.sort') }}</th>
              <th>{{ t('requirement.categoryList.key') }}</th>
              <th>{{ t('requirement.categoryList.label') }}</th>
              <th>{{ t('requirement.categoryList.isDefault') }}</th>
              <th>{{ t('issue.type') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in categories" :key="row.id">
              <td class="time">{{ row.sort_order }}</td>
              <td class="key static">{{ row.name }}</td>
              <td>{{ row.label }}</td>
              <!-- 默认项只标"是"，不是默认的留空——一列里半数绿标等于没标 -->
              <td>
                <span v-if="row.is_default" class="pill green">{{ t('common.yes') }}</span>
                <span v-else class="muted">-</span>
              </td>
              <td><span class="pill neutral">{{ row.is_system ? t('common.system') : t('requirement.categoryList.custom') }}</span></td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click="handleEdit(row)">{{ t('common.edit') }}</button>
                  <el-dropdown v-if="!row.is_system" trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleDelete(row)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && categories.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('requirement.categoryList.empty')" />
        </div>
      </div>
    </section>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="showDialog"
      :title="editing ? t('requirement.categoryList.edit') : t('requirement.categoryList.create')"
      width="500px"
      destroy-on-close
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item :label="t('requirement.categoryList.key')" prop="name">
          <el-input
            v-model="form.name"
            :placeholder="t('requirement.categoryList.keyPlaceholder')"
            :disabled="!!editing"
          />
          <div v-if="!editing" class="form-tip">{{ t('requirement.categoryList.keyTip') }}</div>
        </el-form-item>
        <el-form-item :label="t('requirement.categoryList.label')" prop="label">
          <el-input v-model="form.label" :placeholder="t('requirement.categoryList.labelPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('requirement.categoryList.color')" prop="color">
          <el-select v-model="form.color" :placeholder="t('requirement.categoryList.colorPlaceholder')" style="width: 100%">
            <el-option :label="t('requirement.categoryList.colorPrimary')" value="primary">
              <el-tag type="primary" size="small">{{ t('requirement.categoryList.sample') }}</el-tag>
              <span style="margin-left: 8px;">{{ t('requirement.categoryList.blue') }}</span>
            </el-option>
            <el-option :label="t('requirement.categoryList.colorSuccess')" value="success">
              <el-tag type="success" size="small">{{ t('requirement.categoryList.sample') }}</el-tag>
              <span style="margin-left: 8px;">{{ t('requirement.categoryList.green') }}</span>
            </el-option>
            <el-option :label="t('requirement.categoryList.colorDanger')" value="danger">
              <el-tag type="danger" size="small">{{ t('requirement.categoryList.sample') }}</el-tag>
              <span style="margin-left: 8px;">{{ t('requirement.categoryList.red') }}</span>
            </el-option>
            <el-option :label="t('requirement.categoryList.colorWarning')" value="warning">
              <el-tag type="warning" size="small">{{ t('requirement.categoryList.sample') }}</el-tag>
              <span style="margin-left: 8px;">{{ t('requirement.categoryList.orange') }}</span>
            </el-option>
            <el-option :label="t('requirement.categoryList.colorInfo')" value="info">
              <el-tag type="info" size="small">{{ t('requirement.categoryList.sample') }}</el-tag>
              <span style="margin-left: 8px;">{{ t('requirement.categoryList.grey') }}</span>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item v-if="editing" :label="t('requirement.categoryList.sort')" prop="sort_order">
          <el-input-number v-model="form.sort_order" :min="0" :max="999" />
        </el-form-item>
        <el-form-item v-if="editing" :label="t('requirement.categoryList.setDefault')">
          <el-switch v-model="form.is_default" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  getRequirementCategories,
  createRequirementCategory,
  updateRequirementCategory,
  deleteRequirementCategory,
} from '@/api/requirement'
import type { RequirementCategoryDef } from '@/types/requirement'

const { t } = useI18n()

const categories = ref<RequirementCategoryDef[]>([])
const loading = ref(false)
const submitting = ref(false)
const showDialog = ref(false)
const editing = ref<RequirementCategoryDef | null>(null)
const formRef = ref<FormInstance>()

const form = reactive({
  name: '',
  label: '',
  color: 'info' as string,
  sort_order: 0,
  is_default: false,
})

const rules: FormRules = {
  name: [
    { required: true, message: t('requirement.categoryList.keyRequired'), trigger: ['blur', 'change'] },
    { pattern: /^[a-z][a-z0-9_]*$/, message: t('requirement.categoryList.keyPattern'), trigger: 'blur' },
    { min: 1, max: 30, message: t('requirement.categoryList.keyLength'), trigger: 'blur' },
  ],
  label: [
    { required: true, message: t('requirement.categoryList.labelRequired'), trigger: ['blur', 'change'] },
    { min: 1, max: 50, message: t('requirement.categoryList.labelLength'), trigger: 'blur' },
  ],
  color: [{ required: true, message: t('requirement.categoryList.colorRequired'), trigger: 'change' }],
}

const loadCategories = async () => {
  loading.value = true
  try {
    const { data } = await getRequirementCategories()
    categories.value = data.data
  } catch {
    ElMessage.error(t('requirement.categoryList.loadFailed'))
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  resetForm()
  showDialog.value = true
}

const handleEdit = (cat: RequirementCategoryDef) => {
  editing.value = cat
  form.name = cat.name
  form.label = cat.label
  form.color = cat.color
  form.sort_order = cat.sort_order
  form.is_default = cat.is_default
  showDialog.value = true
}

const handleDelete = async (cat: RequirementCategoryDef) => {
  try {
    await ElMessageBox.confirm(t('requirement.categoryList.confirmDelete', { name: cat.label }), t('issue.msg.tipTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
    await deleteRequirementCategory(cat.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadCategories()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || t('issue.msg.deleteFailed2'))
    }
  }
}

const handleSubmit = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitting.value = true
    try {
      if (editing.value) {
        await updateRequirementCategory(editing.value.id, {
          label: form.label,
          color: form.color,
          sort_order: form.sort_order,
          is_default: form.is_default,
        })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createRequirementCategory({
          name: form.name,
          label: form.label,
          color: form.color,
        })
        ElMessage.success(t('common.createSuccess'))
      }
      showDialog.value = false
      loadCategories()
    } catch (error: any) {
      ElMessage.error(error.response?.data?.message || t('common.operationFailed'))
    } finally {
      submitting.value = false
    }
  })
}

const resetForm = () => {
  editing.value = null
  form.name = ''
  form.label = ''
  form.color = 'info'
  form.sort_order = 0
  form.is_default = false
  formRef.value?.resetFields()
}

onMounted(() => {
  loadCategories()
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里。

.form-tip {
  font-size: 12px;
  color: var(--td-text-placeholder);
  margin-top: 4px;
  line-height: 1.5;
}

// 响应式
@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
}
</style>
