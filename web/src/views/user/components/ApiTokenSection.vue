<template>
  <section class="card">
    <div class="card-head">
      <h2>{{ t('user.token.title') }}</h2>
      <span class="n">{{ tokens.length }} / 20</span>
      <div class="grow"></div>
      <button class="btn secondary" @click="openCreate">{{ t('user.token.create') }}</button>
    </div>

    <div class="api-token-desc">
      {{ t('user.token.desc') }}
    </div>

    <div v-loading="loading">
      <el-table v-if="tokens.length > 0" :data="tokens" class="token-table">
        <el-table-column :label="t('common.name')" min-width="150">
          <template #default="{ row }">
            <span class="token-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('user.token.prefix')" width="180">
          <template #default="{ row }">
            <code class="token-prefix">{{ row.prefix }}...</code>
          </template>
        </el-table-column>
        <el-table-column :label="t('user.token.lastUsed')" width="160">
          <template #default="{ row }">
            <span v-if="row.last_used_at">{{ formatTime(row.last_used_at) }}</span>
            <span v-else class="never-used">{{ t('user.token.neverUsed') }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('user.token.expiry')" width="160">
          <template #default="{ row }">
            <el-tag v-if="row.is_expired" type="danger" size="small">{{ t('user.token.expired') }}</el-tag>
            <span v-else-if="row.expires_at">{{ formatTime(row.expires_at) }}</span>
            <el-tag v-else type="info" size="small">{{ t('user.token.never') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.operation')" width="80" align="center">
          <template #default="{ row }">
            <el-button link type="danger" @click="confirmDelete(row)">{{ t('user.token.revoke') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <TdEmptyState
        v-else-if="!loading"
        preset="first-time"
        :title="t('user.token.emptyTitle')"
      >
        <el-button type="primary" @click="openCreate">
          <el-icon><Plus /></el-icon>{{ t('user.token.createFirst') }}
        </el-button>
      </TdEmptyState>
    </div>

    <!-- 创建对话框 -->
    <el-dialog v-model="createDialogVisible" :title="t('user.token.createTitle')" width="480px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top">
        <el-form-item :label="t('common.name')" prop="name">
          <el-input v-model="form.name" :placeholder="t('user.token.namePlaceholder')" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item :label="t('user.token.expiryLabel')">
          <el-radio-group v-model="form.expires_preset">
            <el-radio value="7d">{{ t('user.token.days7') }}</el-radio>
            <el-radio value="30d">{{ t('user.token.days30') }}</el-radio>
            <el-radio value="90d">{{ t('user.token.days90') }}</el-radio>
            <el-radio value="365d">{{ t('user.token.days365') }}</el-radio>
            <el-radio value="never">{{ t('user.token.never') }}</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>

    <!-- 一次性明文展示对话框 -->
    <el-dialog
      v-model="showTokenDialog"
      :title="t('user.token.saveTitle')"
      width="600px"
      :close-on-click-modal="false"
      :show-close="false"
    >
      <el-alert type="warning" :closable="false" class="token-warning">
        <i18n-t keypath="user.token.saveWarning" tag="span"><template #name><strong>{{ newToken?.name }}</strong></template></i18n-t>
      </el-alert>

      <div class="token-display">
        <code>{{ newToken?.token }}</code>
        <el-button type="primary" size="small" @click="copyToken">
          <el-icon><CopyDocument /></el-icon>{{ t('common.copy') }}
        </el-button>
      </div>

      <div class="usage-section">
        <div class="usage-label">{{ t('user.token.usageExample') }}</div>
        <el-input
          :model-value="usageExample"
          type="textarea"
          :rows="3"
          readonly
          class="usage-snippet"
        />
      </div>

      <template #footer>
        <el-button type="primary" @click="confirmSaved">{{ t('user.token.saved') }}</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, CopyDocument } from '@element-plus/icons-vue'
import { listApiTokens, createApiToken, deleteApiToken } from '@/api/token'
import type { APIToken, CreateTokenResponse } from '@/types/token'
import dayjs from 'dayjs'

const { t } = useI18n()

const tokens = ref<APIToken[]>([])
const loading = ref(false)

const createDialogVisible = ref(false)
const creating = ref(false)
const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  expires_preset: '90d' as '7d' | '30d' | '90d' | '365d' | 'never' })

const formRules: FormRules = {
  name: [
    { required: true, message: t('user.token.nameRequired'), trigger: ['blur', 'change'] },
    { max: 100, message: t('user.token.nameMax'), trigger: 'blur' },
  ] }

const showTokenDialog = ref(false)
const newToken = ref<CreateTokenResponse | null>(null)

const usageExample = computed(() => {
  if (!newToken.value) return ''
  return `curl -X POST ${window.location.origin}/api/v1/issues \\\n  -H "Authorization: Bearer ${newToken.value.token}" \\\n  -H "Content-Type: application/json" \\\n  -d '{"project_key":"YOUR_PROJ","issue_type_id":1,"title":"..."}'`
})

// 将过期时间预设转换为秒数
const presetToSeconds = (preset: string): number => {
  const day = 24 * 60 * 60
  switch (preset) {
    case '7d': return 7 * day
    case '30d': return 30 * day
    case '90d': return 90 * day
    case '365d': return 365 * day
    case 'never':
    default: return 0
  }
}

const formatTime = (s: string | null) => (s ? dayjs(s).format('YYYY-MM-DD HH:mm') : '')

const loadTokens = async () => {
  loading.value = true
  try {
    const { data } = await listApiTokens()
    tokens.value = data.data || []
  } catch {
    // 静默处理，避免影响页面渲染
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  if (tokens.value.length >= 20) {
    ElMessage.warning(t('user.token.limitReached'))
    return
  }
  form.value = { name: '', expires_preset: '90d' }
  createDialogVisible.value = true
}

const submitCreate = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    creating.value = true
    try {
      const { data } = await createApiToken({
        name: form.value.name,
        expires_in: presetToSeconds(form.value.expires_preset) })
      newToken.value = data.data
      createDialogVisible.value = false
      showTokenDialog.value = true
      await loadTokens()
    } catch {
      ElMessage.error(t('user.token.createFailed'))
    } finally {
      creating.value = false
    }
  })
}

const copyToken = async () => {
  if (!newToken.value) return
  try {
    await navigator.clipboard.writeText(newToken.value.token)
    ElMessage.success(t('user.token.copied'))
  } catch {
    ElMessage.error(t('user.token.copyFailed'))
  }
}

const confirmSaved = () => {
  showTokenDialog.value = false
  newToken.value = null
}

const confirmDelete = async (token: APIToken) => {
  try {
    await ElMessageBox.confirm(
      t('user.token.confirmRevoke', { name: token.name }),
      t('user.token.revokeTitle'),
      { type: 'warning', confirmButtonText: t('user.token.revoke'), cancelButtonText: t('common.cancel'), confirmButtonClass: 'el-button--danger' },
    )
  } catch {
    return
  }
  try {
    await deleteApiToken(token.id)
    ElMessage.success(t('user.token.revoked'))
    await loadTokens()
  } catch {
    ElMessage.error(t('user.token.revokeFailed'))
  }
}

onMounted(() => {
  loadTokens()
})
</script>

<style scoped lang="scss">
.settings-card {
  border-radius: var(--td-radius-lg);
  border: none;
  box-shadow: var(--td-elevation-1);
  margin-bottom: var(--td-space-5);

  :deep(.el-card__header) {
    padding: var(--td-space-4) var(--td-space-5);
    border-bottom: 1px solid var(--td-border-color);
  }
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title-group {
  display: flex;
  align-items: center;
  gap: var(--td-space-2);
  font-size: var(--td-font-md);
  font-weight: var(--td-weight-semibold);
  color: var(--td-text-primary);

  .card-icon {
    color: var(--td-color-primary);
    font-size: 18px;
  }
}

/* 卡内的说明行要和卡头对齐：原来 padding 是 0，文字直接贴在卡片左边框上，
   比上面的标题还靠左 18px，一眼就能看出没对齐。
   同时限宽 —— 984px 一行的正文太长，读起来要来回扫。 */
.api-token-desc {
  padding: 12px 18px 0;
  max-width: 62ch;
  font-size: var(--td-font-sm);
  color: var(--td-text-secondary);
  line-height: var(--td-leading-normal);
}

.token-table {
  .token-name {
    font-weight: var(--td-weight-medium);
    color: var(--td-text-primary);
  }

  .token-prefix {
    font-family: var(--el-font-family-mono, monospace);
    font-size: var(--td-font-sm);
    color: var(--td-text-secondary);
    padding: 2px var(--td-space-2);
    background: var(--td-bg-section);
    border-radius: var(--td-radius-xs);
  }

  .never-used {
    color: var(--td-text-placeholder);
    font-style: italic;
  }
}

// 一次性明文展示
.token-warning {
  margin-bottom: var(--td-space-4);
}

.token-display {
  display: flex;
  align-items: center;
  gap: var(--td-space-2);
  padding: var(--td-space-3) var(--td-space-4);
  background: var(--td-bg-section);
  border: 1px solid var(--td-border-color);
  border-radius: var(--td-radius-md);
  margin-bottom: var(--td-space-4);

  code {
    flex: 1;
    font-family: var(--el-font-family-mono, monospace);
    font-size: var(--td-font-md);
    color: var(--td-text-primary);
    word-break: break-all;
    user-select: all;
  }
}

.usage-section {
  .usage-label {
    font-size: var(--td-font-sm);
    color: var(--td-text-secondary);
    margin-bottom: var(--td-space-2);
    font-weight: var(--td-weight-medium);
  }
}

.usage-snippet {
  :deep(.el-textarea__inner) {
    font-family: var(--el-font-family-mono, monospace);
    font-size: var(--td-font-sm);
  }
}
</style>
