<template>
  <!-- 结构同其它列表页 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('requirement.poolTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('requirement.poolList.create') }}
      </button>
    </div>

    <div class="toolbar">
      <el-select v-model="filters.type" :placeholder="t('issue.type')" clearable class="filter-select" @change="loadData">
        <el-option :label="t('requirement.poolList.typeGlobal')" value="global" />
        <el-option :label="t('requirement.poolList.typeProject')" value="project" />
      </el-select>
      <el-select v-model="filters.status" :placeholder="t('issue.status')" clearable class="filter-select" @change="loadData">
        <el-option :label="t('requirement.poolList.statusActive')" value="active" />
        <el-option :label="t('requirement.poolList.statusArchived')" value="archived" />
      </el-select>
      <button class="btn secondary" @click="resetFilters">{{ t('common.reset') }}</button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col /><col style="width: 96px" /><col style="width: 150px" /><col style="width: 120px" />
            <col style="width: 96px" /><col style="width: 96px" /><col style="width: 150px" /><col style="width: 150px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('common.name') }}</th>
              <th>{{ t('issue.type') }}</th>
              <th>{{ t('requirement.poolList.linkedProject') }}</th>
              <th>{{ t('requirement.assignee') }}</th>
              <th>{{ t('requirement.poolList.count') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th>{{ t('common.createdAt') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in pools" :key="row.id" @click="handleViewRequirements(row)">
              <td>{{ row.name }}</td>
              <!-- 全局/项目是分类，不是状态：都走中性药丸 -->
              <td><span class="pill neutral">{{ row.type === 'global' ? t('requirement.poolList.typeGlobal') : t('requirement.poolList.typeProject') }}</span></td>
              <td class="muted">{{ row.project_name || '-' }}</td>
              <td class="muted">{{ row.owner_name || '-' }}</td>
              <td class="time">{{ row.requirement_count ?? 0 }}</td>
              <td>
                <span class="pill" :class="row.status === 'active' ? 'green' : 'neutral'">
                  {{ row.status === 'active' ? t('requirement.poolList.statusActive') : t('requirement.poolList.statusArchived') }}
                </span>
              </td>
              <td class="time">{{ formatDateTime(row.created_at) }}</td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click.stop="handleEdit(row)">{{ t('common.edit') }}</button>
                  <el-dropdown trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleViewRequirements(row)">{{ t('requirement.poolList.viewRequirements') }}</el-dropdown-item>
                        <el-dropdown-item divided @click="handleDelete(row)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && pools.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('requirement.poolList.empty')" />
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
      :title="editingPool ? t('requirement.poolList.edit') : t('requirement.poolList.create')"
      width="600px"
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item :label="t('common.name')" prop="name">
          <el-input v-model="form.name" :placeholder="t('requirement.poolList.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            :placeholder="t('requirement.poolList.descPlaceholder')"
          />
        </el-form-item>
        <el-form-item :label="t('issue.type')" prop="type">
          <el-radio-group v-model="form.type" :disabled="!!editingPool">
            <el-radio label="global">{{ t('requirement.poolList.typeGlobalFull') }}</el-radio>
            <el-radio label="project">{{ t('requirement.poolList.typeProjectFull') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.type === 'project'" :label="t('requirement.poolList.linkedProject')" prop="project_id">
          <el-select v-model="form.project_id" :placeholder="t('requirement.projectPlaceholder')" :disabled="!!editingPool" style="width: 100%">
            <el-option
              v-for="project in projects"
              :key="project.id"
              :label="project.name"
              :value="project.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('requirement.assignee')" prop="owner_id">
          <el-select v-model="form.owner_id" :placeholder="t('requirement.assigneePlaceholder')" filterable style="width: 100%">
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
        <el-button @click="handleCancel">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  getRequirementPoolList,
  createRequirementPool,
  updateRequirementPool,
  deleteRequirementPool,
} from '@/api/requirement'
import { getAllProjects } from '@/api/project'
import { getAllUsers } from '@/api/user'
import type { RequirementPool, CreateRequirementPoolRequest, UpdateRequirementPoolRequest, RequirementPoolType, RequirementPoolStatus } from '@/types/requirement'

const { t } = useI18n()

const router = useRouter()

// 数据
const pools = ref<RequirementPool[]>([])
const projects = ref<any[]>([])
const users = ref<any[]>([])
const loading = ref(false)
const submitting = ref(false)
const showCreateDialog = ref(false)
const editingPool = ref<RequirementPool | null>(null)
const formRef = ref<FormInstance>()

// 筛选条件
const filters = reactive({
  type: undefined as RequirementPoolType | undefined,
  status: undefined as RequirementPoolStatus | undefined,
})

// 分页
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})

// 表单
const form = reactive<CreateRequirementPoolRequest>({
  name: '',
  description: '',
  type: 'global',
  owner_id: undefined,
})

// 表单验证规则
const rules: FormRules = {
  name: [{ required: true, message: t('requirement.poolList.nameRequired'), trigger: ['blur', 'change'] }],
  type: [{ required: true, message: t('requirement.poolList.typeRequired'), trigger: 'change' }],
  owner_id: [{
    required: true,
    trigger: 'change',
    validator: (_rule, value, callback) => {
      if (!value || value === 0) {
        callback(new Error(t('requirement.assigneeRequired')))
      } else {
        callback()
      }
    }
  }],
  project_id: [
    {
      required: true,
      message: t('requirement.poolList.projectRequired'),
      trigger: 'change',
      validator: (_rule, value, callback) => {
        if (form.type === 'project' && !value) {
          callback(new Error(t('requirement.poolList.projectMustLink')))
        } else {
          callback()
        }
      },
    },
  ],
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

// 加载数据
const loadData = async () => {
  loading.value = true
  try {
    const { data } = await getRequirementPoolList({
      ...filters,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    pools.value = data.data.items
    pagination.total = data.data.total
  } catch {
    ElMessage.error(t('requirement.poolList.loadFailed'))
  } finally {
    loading.value = false
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

// 重置筛选条件
const resetFilters = () => {
  filters.type = undefined
  filters.status = undefined
  pagination.page = 1
  loadData()
}

// 编辑
const handleEdit = (pool: RequirementPool) => {
  editingPool.value = pool
  form.name = pool.name
  form.description = pool.description
  form.type = pool.type
  form.owner_id = pool.owner_id
  form.project_id = pool.project_id
  showCreateDialog.value = true
}

// 查看需求
const handleViewRequirements = (pool: RequirementPool) => {
  router.push(`/requirements?pool_id=${pool.id}`)
}

// 删除
const handleDelete = async (pool: RequirementPool) => {
  try {
    await ElMessageBox.confirm(t('requirement.poolList.confirmDelete', { name: pool.name }), t('issue.msg.tipTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })

    await deleteRequirementPool(pool.id)
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

    submitting.value = true
    try {
      if (editingPool.value) {
        // 更新
        const updateData: UpdateRequirementPoolRequest = {
          name: form.name,
          description: form.description,
          owner_id: form.owner_id,
        }
        await updateRequirementPool(editingPool.value.id, updateData)
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        // 创建
        await createRequirementPool(form)
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

// 创建需求池
const handleCreate = () => {
  resetForm()
  showCreateDialog.value = true
}

// 取消对话框
const handleCancel = () => {
  showCreateDialog.value = false
}

// 重置表单
const resetForm = () => {
  editingPool.value = null
  form.name = ''
  form.description = ''
  form.type = 'global'
  form.owner_id = undefined
  form.project_id = undefined
  formRef.value?.resetFields()
}

onMounted(() => {
  loadData()
  loadProjects()
  loadUsers()
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里。

</style>
