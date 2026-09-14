<template>
  <!-- 角色列表改成表格：角色的字段固定（标识 / 名称 / 描述 / 类型），
       原来每条是一个带彩色图标的大卡片，一屏放不下几个。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('project.roles.title') }}</h1>
      <div class="grow"></div>
      <button class="btn secondary" @click="$router.push('/projects')">{{ t('project.roles.back') }}</button>
      <button class="btn primary" @click="handleCreateRole">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('project.roles.create') }}
      </button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table v-if="roles.length > 0" class="issues">
          <colgroup><col style="width: 200px" /><col style="width: 180px" /><col /><col style="width: 96px" /><col style="width: 180px" /></colgroup>
          <thead>
            <tr>
              <th>{{ t('project.roles.name') }}</th>
              <th>{{ t('project.roles.key') }}</th>
              <th>{{ t('issue.description') }}</th>
              <th>{{ t('issue.type') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="role in roles" :key="role.id">
              <td>{{ role.role_name }}</td>
              <td class="key static">{{ role.role_key }}</td>
              <td class="muted desc">{{ role.description || '-' }}</td>
              <td>
                <span class="pill neutral">{{ role.is_system ? t('project.roles.systemRole') : t('requirement.categoryList.custom') }}</span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click="handleViewMembers(role)">{{ t('project.roles.manageMembers') }}</button>
                  <el-dropdown v-if="!role.is_system" trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditRole(role)">{{ t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item divided @click="handleDeleteRole(role)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && roles.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('project.roles.empty')" />
        </div>
      </div>
    </section>

    <!-- 创建/编辑角色对话框 -->
    <el-dialog v-model="roleDialogVisible" :title="isEditMode ? t('project.roles.edit') : t('project.roles.create')" width="500px" destroy-on-close>
      <el-form ref="roleFormRef" :model="roleForm" :rules="roleRules" label-position="top">
        <el-form-item :label="t('project.roles.key')" prop="role_key">
          <el-input v-model="roleForm.role_key" :placeholder="t('project.roles.keyPlaceholder')" :disabled="isEditMode" />
        </el-form-item>
        <el-form-item :label="t('project.roles.name')" prop="role_name">
          <el-input v-model="roleForm.role_name" :placeholder="t('project.roles.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="roleForm.description" type="textarea" :rows="3" :placeholder="t('project.roles.descPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('project.roles.sort')">
          <el-input-number v-model="roleForm.sort_order" :min="0" :max="999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submitRole">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 成员管理对话框 -->
    <el-dialog v-model="memberDialogVisible" :title="t('project.roles.memberTitle', { name: currentRole?.role_name })" width="600px" destroy-on-close>
      <div class="member-dialog-content">
        <div class="member-add-section">
          <el-select v-model="selectedUserId" :placeholder="t('project.roles.selectUser')" filterable style="width: 300px">
            <el-option v-for="u in availableUsers" :key="u.id" :label="u.display_name" :value="u.id">
              <span>{{ u.display_name }}</span>
              <span style="color: var(--td-text-placeholder); margin-left: 8px">{{ u.username }}</span>
            </el-option>
          </el-select>
          <el-button type="primary" :loading="addMemberLoading" :disabled="!selectedUserId" @click="handleAddMember">
            <el-icon><Plus /></el-icon>
            {{ t('project.roles.addMember') }}
          </el-button>
        </div>
        <el-divider />
        <div v-loading="membersLoading" class="member-list">
          <div v-for="member in roleMembers" :key="member.id" class="member-item">
            <div class="member-info">
              <div class="member-avatar">{{ member.user?.display_name?.charAt(0) || '?' }}</div>
              <div class="member-details">
                <span class="member-name">{{ member.user?.display_name }}</span>
                <span class="member-username">@{{ member.user?.username }}</span>
              </div>
            </div>
            <el-button size="small" type="danger" text @click="handleRemoveMember(member)">
              <el-icon><Delete /></el-icon>
              {{ t('common.remove') }}
            </el-button>
          </div>
          <TdEmptyState v-if="!membersLoading && roleMembers.length === 0" preset="no-data" :title="t('project.noMembers')" />
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
import {
  getProjectRoles,
  createProjectRole,
  updateProjectRole,
  deleteProjectRole,
  getRoleMembers,
  addRoleMember,
  removeRoleMember } from '@/api/project'
import { getAllUsers } from '@/api/user'
import type { ProjectRole, ProjectRoleMember, CreateProjectRoleRequest } from '@/types/project'
import type { UserOption } from '@/types/user'

const { t } = useI18n()

const route = useRoute()
const projectKey = computed(() => route.params.key as string)

const loading = ref(false)
const roles = ref<ProjectRole[]>([])
const users = ref<UserOption[]>([])

// 角色表单
const roleDialogVisible = ref(false)
const isEditMode = ref(false)
const editingRoleId = ref<number | null>(null)
const submitLoading = ref(false)
const roleFormRef = ref<FormInstance>()
const roleForm = reactive<CreateProjectRoleRequest>({
  role_key: '',
  role_name: '',
  description: '',
  sort_order: 0 })
const roleRules: FormRules = {
  role_key: [
    { required: true, message: t('project.roles.keyRequired'), trigger: ['blur', 'change'] },
    { pattern: /^[a-z_]+$/, message: t('project.roles.keyPattern'), trigger: 'blur' },
  ],
  role_name: [{ required: true, message: t('project.roles.nameRequired'), trigger: ['blur', 'change'] }] }

// 成员管理
const memberDialogVisible = ref(false)
const currentRole = ref<ProjectRole | null>(null)
const roleMembers = ref<ProjectRoleMember[]>([])
const membersLoading = ref(false)
const selectedUserId = ref<number | null>(null)
const addMemberLoading = ref(false)

const availableUsers = computed(() => {
  const memberIds = roleMembers.value.map((m) => m.user_id)
  return users.value.filter((u) => !memberIds.includes(u.id))
})

const loadRoles = async () => {
  loading.value = true
  try {
    // 不是项目管理员就别把整个角色管理页渲染出来再让每个操作各弹一次「权限不足」
    const { data } = await getProjectRoles(projectKey.value, { _redirectOn403: true })
    roles.value = data.data
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const loadUsers = async () => {
  try {
    const { data } = await getAllUsers()
    users.value = data.data
  } catch {
    // ignored
  }
}

const handleCreateRole = () => {
  isEditMode.value = false
  editingRoleId.value = null
  Object.assign(roleForm, { role_key: '', role_name: '', description: '', sort_order: 0 })
  roleDialogVisible.value = true
}

const handleEditRole = (role: ProjectRole) => {
  isEditMode.value = true
  editingRoleId.value = role.id
  Object.assign(roleForm, {
    role_key: role.role_key,
    role_name: role.role_name,
    description: role.description,
    sort_order: role.sort_order })
  roleDialogVisible.value = true
}

const submitRole = async () => {
  if (!roleFormRef.value) return
  await roleFormRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      if (isEditMode.value && editingRoleId.value) {
        await updateProjectRole(projectKey.value, editingRoleId.value, {
          role_name: roleForm.role_name,
          description: roleForm.description,
          sort_order: roleForm.sort_order })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createProjectRole(projectKey.value, roleForm)
        ElMessage.success(t('common.createSuccess'))
      }
      roleDialogVisible.value = false
      loadRoles()
    } catch {
      // ignored
    } finally {
      submitLoading.value = false
    }
  })
}

const handleDeleteRole = async (role: ProjectRole) => {
  try {
    await ElMessageBox.confirm(t('project.roles.confirmDeleteRole', { name: role.role_name }), t('issue.list.deleteTitle'), {
      type: 'warning' })
    await deleteProjectRole(projectKey.value, role.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadRoles()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const handleViewMembers = async (role: ProjectRole) => {
  currentRole.value = role
  selectedUserId.value = null
  memberDialogVisible.value = true
  await loadRoleMembers(role.id)
}

const loadRoleMembers = async (roleId: number) => {
  membersLoading.value = true
  try {
    const { data } = await getRoleMembers(projectKey.value, roleId)
    roleMembers.value = data.data
  } catch {
    // ignored
  } finally {
    membersLoading.value = false
  }
}

const handleAddMember = async () => {
  if (!selectedUserId.value || !currentRole.value) return
  addMemberLoading.value = true
  try {
    await addRoleMember(projectKey.value, currentRole.value.id, { user_id: selectedUserId.value })
    ElMessage.success(t('project.roles.addSuccess'))
    selectedUserId.value = null
    await loadRoleMembers(currentRole.value.id)
  } catch {
    // ignored
  } finally {
    addMemberLoading.value = false
  }
}

const handleRemoveMember = async (member: ProjectRoleMember) => {
  if (!currentRole.value) return
  try {
    await ElMessageBox.confirm(t('project.roles.confirmRemoveMember', { name: member.user?.display_name }), t('project.roles.removeTitle'), {
      type: 'warning' })
    await removeRoleMember(projectKey.value, currentRole.value.id, member.user_id)
    ElMessage.success(t('issue.msg.removeSuccess'))
    await loadRoleMembers(currentRole.value.id)
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

onMounted(() => {
  loadRoles()
  loadUsers()
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里，这一页只留成员管理对话框。

.desc { max-width: 0; overflow: hidden; text-overflow: ellipsis; }

.member-dialog-content {
  .member-add-section {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .member-list {
    min-height: 200px;
    max-height: 400px;
    overflow-y: auto;
  }

  .member-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px;
    border-radius: 8px;
    margin-bottom: 8px;
    background: var(--td-bg-page);

    &:hover {
      background: var(--td-bg-section);
    }
  }

  .member-info {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .member-avatar {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: var(--td-tag-primary-bg);
    color: var(--td-tag-primary-text);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 14px;
    font-weight: 600;
  }

  .member-details {
    display: flex;
    flex-direction: column;
  }

  .member-name {
    font-size: 14px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .member-username {
    font-size: 12px;
    color: var(--td-text-placeholder);
  }
}
</style>
