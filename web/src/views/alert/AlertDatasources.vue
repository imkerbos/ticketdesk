<template>
  <!-- 原来是卡片网格：每张卡里塞了连接状态、Webhook、轮询间隔、最后检查四组
       label/value，纵向拉得很长。字段是固定的，改成表格一屏能看完。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('alert.datasources.title') }}</h1>
      <div class="grow"></div>
      <button v-if="datasources.length > 0" class="btn primary" @click="openCreateDialog">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('alert.datasources.add') }}
      </button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table v-if="datasources.length > 0" class="issues">
          <colgroup>
            <col style="width: 200px" /><col style="width: 120px" /><col style="width: 100px" />
            <col style="width: 110px" /><col /><col style="width: 150px" /><col style="width: 72px" /><col style="width: 120px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('alert.datasources.name') }}</th>
              <th>{{ t('issue.type') }}</th>
              <th>{{ t('alert.datasources.mode') }}</th>
              <th>{{ t('alert.datasources.connStatus') }}</th>
              <th>Webhook / {{ t('alert.datasources.pollInterval') }}</th>
              <th>{{ t('alert.datasources.lastCheck') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="ds in datasources" :key="ds.id">
              <td>
                <div class="title-cell">
                  <span class="type-icon" v-html="getTypeLogo(ds.type)"></span>
                  <span class="txt">{{ ds.name }}</span>
                </div>
              </td>
              <!-- 类型和接入方式是分类维度，不占语义色 -->
              <td><span class="pill neutral">{{ getTypeLabel(ds.type) }}</span></td>
              <td><span class="pill neutral">{{ ds.push_mode ? t('alert.datasources.modeWebhook') : t('alert.datasources.modePoll') }}</span></td>
              <td>
                <span v-if="ds.last_check_ok === true" class="pill green">{{ t('alert.datasources.connOk') }}</span>
                <span v-else-if="ds.last_check_ok === false" class="pill sla" :title="ds.last_check_msg">{{ t('alert.datasources.connFail') }}</span>
                <span v-else class="pill neutral">{{ t('alert.datasources.connUnknown') }}</span>
              </td>
              <td>
                <div v-if="ds.push_mode" class="webhook-cell">
                  <code>{{ getFullWebhookUrl(ds.webhook_url) }}</code>
                  <button class="link-btn" @click="copyWebhookUrl(ds.webhook_url)">{{ t('common.copy') }}</button>
                </div>
                <span v-else class="muted">{{ ds.poll_interval }} {{ t('alert.datasources.seconds') }}</span>
              </td>
              <td class="time">{{ ds.last_check_at ? formatTime(ds.last_check_at) : '-' }}</td>
              <td>
                <el-switch
                  :model-value="ds.status === 1"
                  size="small"
                  @change="(val: string | number | boolean) => handleToggleStatus(ds, val as boolean)"
                />
              </td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click="handleTestConnection(ds)">{{ t('alert.datasources.testConn') }}</button>
                  <button class="link-btn" @click="openEditDialog(ds)">{{ t('common.edit') }}</button>
                  <el-dropdown trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleDelete(ds.id)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && datasources.length === 0" class="empty">
          <TdEmptyState
            preset="first-time"
            :title="t('alert.datasources.empty')"
            :description="t('alert.datasources.emptyDesc')"
          >
            <button class="btn primary" @click="openCreateDialog">{{ t('alert.datasources.addFirst') }}</button>
          </TdEmptyState>
        </div>
      </div>
    </section>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? t('alert.datasources.editTitle') : t('alert.datasources.add')"
      width="600px"
      :close-on-click-modal="false"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-form-item :label="t('common.name')" prop="name">
          <el-input v-model="form.name" :placeholder="t('alert.datasources.namePlaceholder')" :disabled="isEdit" />
          <div class="form-hint">{{ t('alert.datasources.nameHint') }}</div>
        </el-form-item>

        <el-form-item :label="t('issue.type')" prop="type">
          <el-select v-model="form.type" :placeholder="t('alert.datasources.typePlaceholder')" :disabled="isEdit" style="width: 100%">
            <el-option
              v-for="dsType in DatasourceTypes"
              :key="dsType.value"
              :label="t(dsType.labelKey)"
              :value="dsType.value"
            />
          </el-select>
        </el-form-item>

        <el-form-item :label="t('issue.description')">
          <el-input v-model="form.description" type="textarea" :rows="2" :placeholder="t('alert.datasources.descPlaceholder')" />
        </el-form-item>

        <el-form-item :label="t('alert.datasources.accessMode')">
          <div class="mode-selector">
            <el-radio-group v-model="form.push_mode">
              <el-radio :value="true">{{ t('alert.datasources.modeWebhook') }}</el-radio>
              <el-radio :value="false">{{ t('alert.datasources.modePoll') }}</el-radio>
            </el-radio-group>
            <div class="form-hint">
              {{ form.push_mode ? t('alert.datasources.modeHintPush') : t('alert.datasources.modeHintPoll') }}
            </div>
          </div>
        </el-form-item>

        <!-- 夜莺配置 -->
        <template v-if="form.type === 'nightingale'">
          <el-form-item :label="t('alert.datasources.apiUrl')" prop="config.base_url">
            <el-input v-model="form.config.base_url" placeholder="http://nightingale:17000" />
          </el-form-item>
          <el-form-item label="Token">
            <el-input v-model="form.config.token" :placeholder="t('alert.datasources.tokenPlaceholder')" show-password />
          </el-form-item>
        </template>

        <!-- Prometheus 配置 -->
        <template v-if="form.type === 'prometheus'">
          <el-form-item :label="t('alert.datasources.apiUrl')">
            <el-input v-model="form.config.base_url" :placeholder="t('alert.datasources.baseUrlPlaceholder')" />
          </el-form-item>
        </template>

        <el-form-item v-if="!form.push_mode" :label="t('alert.datasources.pollInterval')">
          <el-input-number v-model="form.poll_interval" :min="5" :max="3600" :step="5" />
          <span style="margin-left: 8px; color: var(--td-color-info)">{{ t('alert.datasources.seconds') }}</span>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button :loading="testLoading" @click="handleTestBeforeSave">
          <el-icon><Connection /></el-icon>
          {{ t('alert.datasources.testConn') }}
        </el-button>
        <el-button type="primary" :loading="saveLoading" @click="handleSave">
          {{ isEdit ? t('common.save') : t('common.create') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import dayjs from 'dayjs'
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { Connection } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  getDatasourceList,
  createDatasource,
  updateDatasource,
  deleteDatasource,
  testDatasource,
  testDatasourceById } from '@/api/alert'
import { DatasourceTypes } from '@/types/alert'
import type { AlertDatasource } from '@/types/alert'

const { t } = useI18n()

const loading = ref(false)
const datasources = ref<AlertDatasource[]>([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const editId = ref(0)
const saveLoading = ref(false)
const testLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  name: '',
  type: 'nightingale' as string,
  description: '',
  config: {} as Record<string, any>,
  push_mode: false,
  poll_interval: 30 })

const formRules: FormRules = {
  name: [
    { required: true, message: t('alert.datasources.nameRequired'), trigger: ['blur', 'change'] },
    { min: 1, max: 100, message: t('alert.datasources.nameLength'), trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: t('alert.datasources.namePattern'), trigger: 'blur' },
  ],
  type: [{ required: true, message: t('alert.datasources.typeRequired'), trigger: 'change' }] }

const fetchDatasources = async () => {
  loading.value = true
  try {
    const res = await getDatasourceList({ page: 1, page_size: 100 })
    datasources.value = res.data?.data?.items || []
  } catch {
    ElMessage.error(t('alert.datasources.loadFailed'))
  } finally {
    loading.value = false
  }
}

const getTypeLogo = (type: string) => {
  if (type === 'nightingale') {
    // 夜莺 logo — 紫色蜂鸟风格
    return `<svg viewBox="0 0 200 200" width="36" height="36" xmlns="http://www.w3.org/2000/svg">
      <defs>
        <linearGradient id="n9e-bg" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" style="stop-color:var(--td-cat-2)"/>
          <stop offset="100%" style="stop-color:var(--td-cat-2)"/>
        </linearGradient>
      </defs>
      <circle cx="100" cy="100" r="96" fill="url(#n9e-bg)"/>
      <g transform="translate(100,100) scale(0.7) translate(-100,-100)">
        <path d="M140 50 C165 55 175 80 170 105 C165 130 145 155 115 170 L100 175 L95 165 C90 150 88 130 92 110 C95 95 102 80 115 68 C125 58 135 52 140 50Z" fill="#fff" opacity="0.95"/>
        <path d="M100 175 L85 170 C65 158 50 140 48 115 C46 95 55 78 70 65 L92 110 C88 130 90 150 95 165 Z" fill="#fff" opacity="0.75"/>
        <path d="M40 85 C45 80 55 78 70 65" fill="none" stroke="#fff" stroke-width="5" stroke-linecap="round" opacity="0.6"/>
        <circle cx="148" cy="72" r="5" fill="url(#n9e-bg)"/>
      </g>
    </svg>`
  }
  if (type === 'prometheus') {
    // Prometheus 官方 logo
    return `<svg viewBox="0 0 430 430" width="36" height="36" xmlns="http://www.w3.org/2000/svg">
      <path fill="#E75225" d="M215.926 7.068c115.684.024 210.638 93.784 210.493 207.844-.148 115.793-94.713 208.252-212.912 208.169C97.95 423 4.52 329.143 4.601 213.221 4.68 99.867 99.833 7.044 215.926 7.068zm-63.947 73.001c2.652 12.978.076 25.082-3.846 36.988-2.716 8.244-6.47 16.183-8.711 24.539-3.694 13.769-7.885 27.619-9.422 41.701-2.21 20.25 5.795 38.086 19.493 55.822L86.527 225.94c.11 1.978-.007 2.727.21 3.361 5.968 17.43 16.471 32.115 28.243 45.957 1.246 1.465 4.082 2.217 6.182 2.221 62.782.115 125.565.109 188.347.028 1.948-.003 4.546-.369 5.741-1.618 13.456-14.063 23.746-30.079 30.179-50.257l-66.658 12.976c4.397-8.567 9.417-16.1 12.302-24.377 9.869-28.315 5.779-55.69-8.387-81.509-11.368-20.72-21.854-41.349-16.183-66.32-12.005 11.786-16.615 26.79-19.541 42.253-2.882 15.23-4.58 30.684-6.811 46.136-.317-.467-.728-.811-.792-1.212-.258-1.621-.499-3.255-.587-4.893-1.355-25.31-6.328-49.696-16.823-72.987-6.178-13.71-12.99-27.727-6.622-44.081-4.31 2.259-8.205 4.505-10.997 7.711-8.333 9.569-11.779 21.062-12.666 33.645-.757 10.75-1.796 21.552-3.801 32.123-2.107 11.109-5.448 21.998-12.956 32.209-3.033-21.81-3.37-43.38-22.928-57.237zm161.877 216.523H116.942v34.007h196.914v-34.007zm-157.871 51.575c-.163 28.317 28.851 49.414 64.709 47.883 29.716-1.269 56.016-24.51 53.755-47.883H155.985z"/>
    </svg>`
  }
  return ''
}

const getTypeLabel = (type: string) => {
  const found = DatasourceTypes.find((t) => t.value === type)
  return found ? t(found.labelKey) : type
}

const getFullWebhookUrl = (path: string) => {
  return `${window.location.origin}${path}`
}

const copyWebhookUrl = async (path: string) => {
  try {
    await navigator.clipboard.writeText(getFullWebhookUrl(path))
    ElMessage.success(t('alert.datasources.copied'))
  } catch {
    ElMessage.error(t('alert.datasources.copyFailed'))
  }
}

const formatTime = (time: string) => {
  if (!time) return '-'
  // toLocaleString('zh-CN') 出来是「2026/9/10 18:00:00」—— 斜杠、月份不补零、还带秒，
  // 和全站其它表格的 YYYY-MM-DD HH:mm 对不上，等宽列也排不齐
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

const resetForm = () => {
  form.name = ''
  form.type = 'nightingale'
  form.description = ''
  form.config = {}
  form.push_mode = false
  form.poll_interval = 30
}

const openCreateDialog = () => {
  resetForm()
  isEdit.value = false
  editId.value = 0
  dialogVisible.value = true
}

const openEditDialog = (ds: AlertDatasource) => {
  isEdit.value = true
  editId.value = ds.id
  form.name = ds.name
  form.type = ds.type
  form.description = ds.description
  form.config = { ...ds.config }
  form.push_mode = ds.push_mode
  form.poll_interval = ds.poll_interval
  dialogVisible.value = true
}

const handleSave = async () => {
  if (!formRef.value) return
  await formRef.value.validate()

  saveLoading.value = true
  try {
    if (isEdit.value) {
      await updateDatasource(editId.value, {
        description: form.description,
        config: form.config,
        push_mode: form.push_mode,
        poll_interval: form.poll_interval })
      ElMessage.success(t('alert.datasources.updated'))
    } else {
      await createDatasource({
        name: form.name,
        type: form.type,
        description: form.description,
        config: form.config,
        push_mode: form.push_mode,
        poll_interval: form.poll_interval })
      ElMessage.success(t('alert.datasources.created'))
    }
    dialogVisible.value = false
    fetchDatasources()
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.message || t('alert.datasources.saveFailed'))
  } finally {
    saveLoading.value = false
  }
}

const handleToggleStatus = async (ds: AlertDatasource, enabled: boolean) => {
  try {
    await updateDatasource(ds.id, { status: enabled ? 1 : 0 })
    ElMessage.success(enabled ? t('alert.datasources.hasEnabled') : t('alert.datasources.hasDisabled'))
    fetchDatasources()
  } catch {
    ElMessage.error(t('common.operationFailed'))
  }
}

const handleDelete = async (id: number) => {
  try {
    await deleteDatasource(id)
    ElMessage.success(t('alert.datasources.deleted'))
    fetchDatasources()
  } catch (error: any) {
    const msg = error?.response?.data?.message || t('alert.datasources.deleteFailed')
    ElMessage.error(msg)
  }
}

const handleTestConnection = async (ds: AlertDatasource) => {
  try {
    const res = await testDatasourceById(ds.id)
    const result = res.data?.data
    if (result?.success) {
      ElMessage.success(t('alert.datasources.connSuccess', { ms: result.latency_ms }))
    } else {
      ElMessage.warning(result?.message || t('alert.datasources.connFailed'))
    }
    fetchDatasources()
  } catch {
    ElMessage.error(t('alert.datasources.testFailed'))
  }
}

const handleTestBeforeSave = async () => {
  testLoading.value = true
  try {
    const res = await testDatasource({
      type: form.type,
      config: form.config })
    const result = res.data?.data
    if (result?.success) {
      ElMessage.success(t('alert.datasources.connSuccess', { ms: result.latency_ms }))
    } else {
      ElMessage.warning(result?.message || t('alert.datasources.connFailed'))
    }
  } catch {
    ElMessage.error(t('alert.datasources.testFailed'))
  } finally {
    testLoading.value = false
  }
}

onMounted(() => {
  fetchDatasources()
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里，这一页只留对话框相关和两处单元格。

// 数据源类型 logo：官方 SVG，尺寸对齐圆点那一档
.type-icon {
  display: inline-flex;
  width: 14px;
  height: 14px;
  flex: 0 0 14px;

  :deep(svg) { width: 100%; height: 100%; }
}

// Webhook 地址：等宽 + 截断，后面跟一个复制
.webhook-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;

  code {
    font-family: var(--td-font-mono);
    font-size: 11.5px;
    color: var(--td-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.form-hint {
  font-size: 12px;
  color: var(--td-text-placeholder);
  margin-top: 4px;
  line-height: 1.4;
}

/* ===== 响应式 ===== */
@media (max-width: 768px) {
  .card-details {
    grid-template-columns: 1fr 1fr;
  }
}

/* 接入方式：单选一行、说明另起一行 */
.mode-selector {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 100%;
}
</style>
