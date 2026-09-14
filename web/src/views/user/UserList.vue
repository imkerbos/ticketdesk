<template>
  <!-- 与工单列表同一套结构：.page-head / .toolbar / .card / table.issues
       样式都在 src/styles/_apple.scss 里，这一页只写自己的单元格。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('user.listTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('user.create') }}
      </button>
    </div>

    <div class="toolbar">
      <label class="search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input v-model="queryParams.keyword" type="search" :placeholder="t('user.searchPlaceholder')" @keyup.enter="handleSearch" @search="handleSearch" />
      </label>
      <el-select v-model="queryParams.role" :placeholder="t('user.role')" clearable class="filter-select" @change="handleSearch">
        <el-option :label="t('user.roleAdmin')" value="admin" />
        <el-option :label="t('user.roleUser')" value="user" />
      </el-select>
      <el-select v-model="queryParams.status" :placeholder="t('issue.status')" clearable class="filter-select" @change="handleSearch">
        <el-option :label="t('common.enabled')" :value="1" />
        <el-option :label="t('common.disabled')" :value="0" />
      </el-select>
      <button class="btn secondary" @click="handleReset">
        <el-icon><Refresh /></el-icon>{{ t('common.reset') }}
      </button>
    </div>

    <section class="card">
      <div class="kpis">
        <div class="kpi"><div class="k">{{ t('user.statTotal') }}</div><div class="v">{{ stats.total }}</div></div>
        <div class="kpi"><div class="k">{{ t('user.statActive') }}</div><div class="v">{{ stats.active }}</div></div>
        <div class="kpi"><div class="k">{{ t('user.statAdmin') }}</div><div class="v">{{ stats.admin }}</div></div>
        <div class="kpi"><div class="k">{{ t('user.statDisabled') }}</div><div class="v">{{ stats.disabled }}</div></div>
        <div class="grow"></div>
        <div class="count">{{ t('common.total', { n: total }) }}</div>
      </div>

      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col /><col style="width: 96px" /><col style="width: 128px" /><col style="width: 64px" />
            <col style="width: 150px" /><col style="width: 150px" /><col style="width: 128px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('user.userInfo') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th>{{ t('user.authSource') }}</th>
              <th>MFA</th>
              <th>{{ t('user.lastLogin') }}</th>
              <th>{{ t('common.createdAt') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in userList" :key="row.id">
              <td>
                <div class="person">
                  <!-- 头像不再用底色区分管理员：名字后面那颗「管理员」标签已经说了，
                       一屏八行里再多一个饱和实心蓝纯属重复 -->
                  <span class="ava tinted">
                    {{ (row.display_name || row.username).charAt(0) }}
                  </span>
                  <span class="user-cell">
                    <span class="user-name">
                      {{ row.display_name || row.username }}
                      <span v-if="isAdmin(row)" class="pill neutral">{{ t('user.roleAdmin') }}</span>
                    </span>
                    <span class="user-meta">@{{ row.username }} · {{ row.email }}</span>
                  </span>
                </div>
              </td>
              <td>
                <span class="pill" :class="row.status === 1 ? 'green' : 'neutral'">
                  {{ row.status === 1 ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
              <td>
                <span class="pill neutral">
                  {{ row.auth_source === 'sso' ? `SSO (${row.sso_provider || 'SSO'})` : t('user.authLocal') }}
                </span>
              </td>
              <!-- 同一列里「启用」原来是裸的黑字，比隔壁状态列的淡底徽章还抢眼；
                   开了 MFA 的是少数，用绿色徽章标出来，没开的留一个弱色破折号 -->
              <td>
                <span v-if="row.mfa_enabled" class="pill green">{{ t('common.enabled') }}</span>
                <span v-else class="muted">—</span>
              </td>
              <td class="time">{{ row.last_login_at ? formatTime(row.last_login_at) : t('user.neverLogin') }}</td>
              <td class="time">{{ formatTime(row.created_at) }}</td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click="handleEdit(row)">{{ t('common.edit') }}</button>
                  <el-dropdown trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item :disabled="row.auth_source === 'sso'" @click="handleResetPassword(row)">
                          {{ t('user.resetPassword') }}
                        </el-dropdown-item>
                        <el-dropdown-item @click="handleToggleStatus(row)">
                          {{ row.status === 1 ? t('user.disable') : t('user.enable') }}
                        </el-dropdown-item>
                        <el-dropdown-item divided @click="handleDeleteUser(row)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && userList.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('user.empty')" />
        </div>

        <div v-if="total > queryParams.page_size" class="table-foot">
          <el-pagination
            v-model:current-page="queryParams.page"
            v-model:page-size="queryParams.page_size"
            :total="total"
            :page-sizes="[10, 20, 50]"
            layout="sizes, prev, pager, next"
            @size-change="loadUsers"
            @current-change="loadUsers"
          />
        </div>
      </div>
    </section>

    <!-- 创建/编辑用户对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? t('user.edit') : t('user.create')"
      width="520px"
      destroy-on-close
      class="user-dialog"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('auth.username')" prop="username">
              <el-input
                v-model="form.username"
                :placeholder="t('user.usernamePlaceholder')"
                :disabled="isEdit"
              >
                <template #prefix>
                  <el-icon><User /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('user.displayName')" prop="display_name">
              <el-input v-model="form.display_name" :placeholder="t('user.displayNamePlaceholder')">
                <template #prefix>
                  <el-icon><Postcard /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="t('user.profile.email')" prop="email">
          <el-input v-model="form.email" :placeholder="t('user.emailPlaceholder')">
            <template #prefix>
              <el-icon><Message /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item v-if="!isEdit" :label="t('auth.password')" prop="password">
          <el-input v-model="form.password" type="password" :placeholder="t('user.passwordPlaceholder')" show-password>
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item :label="t('user.role')" prop="roles">
          <el-radio-group v-model="selectedRole" class="role-radio-group">
            <el-radio value="user" border>
              <div class="role-option">
                <el-icon class="role-icon user"><User /></el-icon>
                <div class="role-text">
                  <span class="role-name">{{ t('user.roleUser') }}</span>
                  <span class="role-desc">{{ t('user.roleUserDesc') }}</span>
                </div>
              </div>
            </el-radio>
            <el-radio value="admin" border>
              <div class="role-option">
                <el-icon class="role-icon admin"><Avatar /></el-icon>
                <div class="role-text">
                  <span class="role-name">{{ t('user.roleAdmin') }}</span>
                  <span class="role-desc">{{ t('user.roleAdminDesc') }}</span>
                </div>
              </div>
            </el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submitForm">
          <el-icon><Check /></el-icon>
          {{ isEdit ? t('user.saveChanges') : t('user.create') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 重置密码对话框 -->
    <el-dialog
      v-model="resetPasswordVisible"
      :title="t('user.resetPassword')"
      width="400px"
      destroy-on-close
      class="password-dialog"
    >
      <div class="password-dialog-header">
        <div class="password-icon">
          <el-icon><Key /></el-icon>
        </div>
        <p class="password-tip">{{ t('user.resetPasswordTip') }}</p>
      </div>
      <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-position="top">
        <el-form-item :label="t('user.newPassword')" prop="password">
          <el-input
            v-model="resetForm.password"
            type="password"
            :placeholder="t('user.newPasswordPlaceholder')"
            show-password
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item :label="t('user.confirmPassword')" prop="confirmPassword">
          <el-input
            v-model="resetForm.confirmPassword"
            type="password"
            :placeholder="t('user.confirmPasswordPlaceholder')"
            show-password
          >
            <template #prefix>
              <el-icon><Lock /></el-icon>
            </template>
          </el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPasswordVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="resetLoading" @click="submitResetPassword">
          <el-icon><Check /></el-icon>
          {{ t('user.confirmReset') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Avatar, Check, Key, Lock, Message, Postcard, Refresh, User } from '@element-plus/icons-vue'
import { getUserList, createUser, updateUser, enableUser, disableUser, resetUserPassword, deleteUser } from '@/api/user'
import type { User as UserType, CreateUserRequest } from '@/types/user'
import dayjs from 'dayjs'

const { t } = useI18n()

// 数据
const loading = ref(false)
const userList = ref<UserType[]>([])
const total = ref(0)

// 统计数据
const stats = computed(() => {
  const users = userList.value
  return {
    total: total.value,
    active: users.filter(u => u.status === 1).length,
    admin: users.filter(u => u.roles?.includes('admin')).length,
    disabled: users.filter(u => u.status === 0).length }
})

// 查询参数
const queryParams = reactive({
  page: 1,
  page_size: 20,
  keyword: '',
  role: undefined as string | undefined,
  status: undefined as number | undefined })

// 创建/编辑对话框
const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingUser = ref<UserType | null>(null)

const form = reactive<CreateUserRequest>({
  username: '',
  email: '',
  password: '',
  display_name: '',
  roles: ['user'] })
const selectedRole = ref('user')

const rules: FormRules = {
  username: [
    { required: true, message: t('user.usernameRequired'), trigger: ['blur', 'change'] },
    { min: 3, max: 20, message: t('user.usernameLength'), trigger: 'blur' },
  ],
  email: [
    { required: true, message: t('user.emailRequired'), trigger: ['blur', 'change'] },
    { type: 'email', message: t('user.emailInvalid'), trigger: 'blur' },
  ],
  password: [
    { required: true, message: t('user.passwordRequired'), trigger: ['blur', 'change'] },
    { min: 6, message: t('user.passwordMin'), trigger: 'blur' },
  ],
  display_name: [
    { required: true, message: t('user.displayNameRequired'), trigger: ['blur', 'change'] },
  ],
  role: [
    { required: true, message: t('user.roleRequired'), trigger: 'change' },
  ] }

// 重置密码对话框
const resetPasswordVisible = ref(false)
const resetLoading = ref(false)
const resetFormRef = ref<FormInstance>()
const resetUserId = ref<number>(0)

const resetForm = reactive({
  password: '',
  confirmPassword: '' })

const resetRules: FormRules = {
  password: [
    { required: true, message: t('user.newPasswordRequired'), trigger: ['blur', 'change'] },
    { min: 6, message: t('user.passwordMin'), trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: t('user.confirmRequired'), trigger: ['blur', 'change'] },
    {
      validator: (_rule, value, callback) => {
        if (value !== resetForm.password) {
          callback(new Error(t('user.passwordMismatch')))
        } else {
          callback()
        }
      },
      trigger: 'blur' },
  ] }

// 表格行样式
// 加载用户列表
// 角色存在 roles 数组里，不是单个 role 字段
const isAdmin = (user: UserType) => user.roles?.includes('admin') ?? false

const loadUsers = async () => {
  loading.value = true
  try {
    const { data } = await getUserList(queryParams)
    userList.value = data.data.items
    total.value = data.data.total
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

// 搜索
const handleSearch = () => {
  queryParams.page = 1
  loadUsers()
}

// 重置
const handleReset = () => {
  queryParams.page = 1
  queryParams.keyword = ''
  queryParams.role = undefined
  queryParams.status = undefined
  loadUsers()
}

// 创建用户
const handleCreate = () => {
  isEdit.value = false
  editingUser.value = null
  Object.assign(form, {
    username: '',
    email: '',
    password: '',
    display_name: '',
    role: 'user' })
  dialogVisible.value = true
}

// 编辑用户
const handleEdit = (user: UserType) => {
  isEdit.value = true
  editingUser.value = user
  Object.assign(form, {
    username: user.username,
    email: user.email,
    password: '',
    display_name: user.display_name,
    roles: user.roles || ['user'] })
  selectedRole.value = user.roles?.includes('admin') ? 'admin' : 'user'
  dialogVisible.value = true
}

// 提交表单
const submitForm = async () => {
  if (!formRef.value) return

  await formRef.value.validate(async (valid) => {
    if (!valid) return

    submitLoading.value = true
    try {
      if (isEdit.value && editingUser.value) {
        await updateUser(editingUser.value.id, {
          email: form.email,
          display_name: form.display_name,
          roles: [selectedRole.value] })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        form.roles = [selectedRole.value]
        await createUser(form)
        ElMessage.success(t('common.createSuccess'))
      }
      dialogVisible.value = false
      loadUsers()
    } catch {
      // ignored
    } finally {
      submitLoading.value = false
    }
  })
}

// 重置密码
const handleResetPassword = (user: UserType) => {
  resetUserId.value = user.id
  resetForm.password = ''
  resetForm.confirmPassword = ''
  resetPasswordVisible.value = true
}

// 提交重置密码
const submitResetPassword = async () => {
  if (!resetFormRef.value) return

  await resetFormRef.value.validate(async (valid) => {
    if (!valid) return

    resetLoading.value = true
    try {
      await resetUserPassword(resetUserId.value, resetForm.password)
      ElMessage.success(t('user.resetSuccess'))
      resetPasswordVisible.value = false
    } catch {
      // ignored
    } finally {
      resetLoading.value = false
    }
  })
}

// 切换状态
const handleToggleStatus = async (user: UserType) => {
  const action = user.status === 1 ? t('user.disable') : t('user.enable')
  try {
    await ElMessageBox.confirm(
      t('user.confirmToggle', { action, name: user.display_name }),
      t('issue.msg.tipTitle'),
      { type: 'warning' }
    )

    if (user.status === 1) {
      await disableUser(user.id)
    } else {
      await enableUser(user.id)
    }
    ElMessage.success(t('user.toggleSuccess', { action }))
    loadUsers()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const handleDeleteUser = async (user: UserType) => {
  try {
    await ElMessageBox.confirm(
      t('user.confirmDelete', { name: user.display_name || user.username }),
      t('issue.list.deleteTitle'),
      { type: 'warning' }
    )
    await deleteUser(user.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadUsers()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

// 工具函数
const formatTime = (time: string) => {
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

// 初始化
onMounted(() => {
  loadUsers()
})
</script>

<style scoped lang="scss">
// 列表本身的样式都在 _apple.scss 里，这一页只留对话框。

.user-dialog {
  :deep(.el-dialog__body) {
    padding: 20px 24px;
  }
}

.role-radio-group {
  display: flex;
  gap: 16px;
  width: 100%;

  :deep(.el-radio) {
    flex: 1;
    height: auto;
    padding: 16px;
    margin-right: 0;

    &.is-bordered {
      border-radius: 10px;
    }

    &.is-checked {
      border-color: var(--td-color-primary);
      background: var(--td-bg-section);
    }

    .el-radio__input {
      display: none;
    }

    .el-radio__label {
      padding-left: 0;
      width: 100%;
    }
  }
}

.role-option {
  display: flex;
  align-items: center;
  gap: 12px;

  .role-icon {
    width: 40px;
    height: 40px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 20px;

    // 淡底 + 同色图标，不用实心色块 + 白图标（§3.1）
    &.user {
      background: var(--td-tag-primary-bg);
      color: var(--td-tag-primary-text);
    }

    &.admin {
      background: var(--td-tag-orange-bg);
      color: var(--td-tag-orange-text);
    }
  }

  .role-text {
    display: flex;
    flex-direction: column;
    gap: 2px;

    .role-name {
      font-size: 14px;
      font-weight: 600;
      color: var(--td-text-primary);
    }

    .role-desc {
      font-size: 12px;
      color: var(--td-text-placeholder);
    }
  }
}

// 重置密码对话框
.password-dialog {
  .password-dialog-header {
    text-align: center;
    margin-bottom: 24px;

    .password-icon {
      width: 52px;
      height: 52px;
      margin: 0 auto 14px;
      background: var(--td-tag-orange-bg);
      color: var(--td-tag-orange-text);
      border-radius: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 22px;
    }

    .password-tip {
      font-size: 14px;
      color: var(--td-text-secondary);
      margin: 0;
    }
  }
}

// 响应式
@media (max-width: 768px) {
  .role-radio-group {
    flex-direction: column;
  }
}
</style>
