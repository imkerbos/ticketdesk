<template>
  <!-- 结构同其它列表页 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('alert.silences.title') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('alert.silences.create') }}
      </button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col style="width: 180px" /><col style="width: 96px" /><col /><col style="width: 260px" />
            <col style="width: 160px" /><col style="width: 160px" /><col style="width: 86px" /><col style="width: 96px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('alert.silences.name') }}</th>
              <th>{{ t('issue.type') }}</th>
              <th>{{ t('issue.description') }}</th>
              <th>{{ t('alert.silences.matchers') }}</th>
              <th>{{ t('alert.silences.startsAt') }}</th>
              <th>{{ t('alert.silences.endsAt') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in silenceList" :key="row.id">
              <td>{{ row.name }}</td>
              <td>
                <span class="pill" :class="row.silence_type === 1 ? 'orange' : 'neutral'">
                  {{ row.silence_type === 1 ? t('alert.silences.typeInstant') : t('alert.silences.typeScheduled') }}
                </span>
              </td>
              <td class="muted desc">{{ row.description || '-' }}</td>
              <td>
                <div class="labels-cell">
                  <span
                    v-for="(matcher, index) in row.label_matchers.slice(0, 2)"
                    :key="index"
                    class="pill neutral"
                    :title="`${matcher.key} ${matcher.operator} ${matcher.value}`"
                  >
                    {{ matcher.key }} {{ matcher.operator }} {{ matcher.value }}
                  </span>
                  <span v-if="row.label_matchers.length > 2" class="pill neutral">+{{ row.label_matchers.length - 2 }}</span>
                </div>
              </td>
              <!-- 等宽只包时间戳本身，「起」这个中文留在外面 ——
                   整格套 .time 会让中文也跟着等宽排，和数字之间豁开一格；
                   整格退成普通字体又让这一列的时间对不齐。 -->
              <td class="muted">
                <span class="time">{{ formatTime(row.starts_at) }}</span>
                <template v-if="row.silence_type === 1">{{ t('alert.silences.fromSuffix') }}</template>
              </td>
              <td class="muted">
                <template v-if="row.silence_type === 1">{{ t('alert.silences.manualClose') }}</template>
                <span v-else-if="row.ends_at" class="time">{{ formatTime(row.ends_at) }}</span>
                <template v-else>-</template>
              </td>
              <td><span class="pill" :class="statusTone(row.status)">{{ getStatusText(row.status) }}</span></td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click="handleEdit(row)">{{ t('common.edit') }}</button>
                  <el-dropdown trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item v-if="row.status === 0" @click="handleEnable(row)">{{ t('common.enabled') }}</el-dropdown-item>
                        <el-dropdown-item v-if="row.status === 1" @click="handleDisable(row)">{{ t('common.close') }}</el-dropdown-item>
                        <el-dropdown-item divided @click="handleDelete(row)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && silenceList.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('alert.silences.empty')" />
        </div>
      </div>
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="600px"
      @close="handleDialogClose"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item :label="t('alert.silences.name')" prop="name">
          <el-input v-model="form.name" :placeholder="t('alert.silences.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')" prop="description">
          <el-input v-model="form.description" type="textarea" :rows="2" :placeholder="t('alert.silences.descPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('alert.silences.type')" prop="silence_type">
          <el-radio-group v-model="form.silence_type">
            <el-radio :value="1">{{ t('alert.silences.typeInstant') }} <span class="type-hint">{{ t('alert.silences.typeInstantHint') }}</span></el-radio>
            <el-radio :value="2">{{ t('alert.silences.typeScheduled') }} <span class="type-hint">{{ t('alert.silences.typeScheduledHint') }}</span></el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="t('alert.silences.matchers')" prop="label_matchers">
          <div style="width: 100%">
            <div
              v-for="(matcher, index) in form.label_matchers"
              :key="index"
              class="matcher-row"
            >
              <el-input v-model="matcher.key" :placeholder="t('alert.silences.matcherKey')" style="flex: 1" />
              <el-select v-model="matcher.operator" style="width: 100px">
                <el-option label="==" value="==" />
                <el-option label="!=" value="!=" />
                <el-option label="=~" value="=~" />
                <el-option label="!~" value="!~" />
              </el-select>
              <el-input v-model="matcher.value" :placeholder="t('alert.silences.matcherValue')" style="flex: 1" />
              <!-- 不常驻红色：这只是删掉一行匹配器，中性图标钮就够（§3.1） -->
              <el-button
                text
                :icon="Delete"
                class="matcher-remove"
                @click="removeMatcher(index)"
              />
            </div>
            <el-button type="primary" text @click="addMatcher">
              <el-icon><Plus /></el-icon>
              {{ t('alert.silences.addMatcher') }}
            </el-button>
          </div>
        </el-form-item>
        <el-row v-if="form.silence_type === 2" :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('alert.silences.startsAt')" prop="starts_at">
              <el-date-picker
                v-model="form.starts_at"
                type="datetime"
                :placeholder="t('alert.silences.startsAtPlaceholder')"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('alert.silences.endsAt')" prop="ends_at">
              <el-date-picker
                v-model="form.ends_at"
                type="datetime"
                :placeholder="t('alert.silences.endsAtPlaceholder')"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item :label="t('alert.silences.comment')" prop="comment">
          <el-input v-model="form.comment" type="textarea" :rows="2" :placeholder="t('alert.silences.commentPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="handleSubmit">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
import {
  getAlertSilenceList,
  createAlertSilence,
  updateAlertSilence,
  deleteAlertSilence,
  cancelAlertSilence } from '@/api/alert'
import type { AlertSilence, LabelMatcher } from '@/types/alert'
import dayjs from 'dayjs'

const { t } = useI18n()

const loading = ref(false)
const silenceList = ref<AlertSilence[]>([])
const total = ref(0)
const queryParams = reactive({
  page: 1,
  page_size: 20 })

const dialogVisible = ref(false)
const dialogTitle = ref('')
const formRef = ref<FormInstance>()
const form = reactive({
  id: 0,
  name: '',
  description: '',
  silence_type: 1 as number,
  label_matchers: [] as LabelMatcher[],
  starts_at: '',
  ends_at: '',
  comment: '' })

const rules: FormRules = {
  name: [{ required: true, message: t('alert.silences.namePlaceholder'), trigger: ['blur', 'change'] }],
  silence_type: [{ required: true, message: t('alert.silences.typeRequired'), trigger: 'change' }],
  label_matchers: [{ required: true, message: t('alert.silences.matcherRequired'), trigger: 'change' }],
  starts_at: [{
    validator: (_rule, _value, callback) => {
      if (form.silence_type === 2 && !form.starts_at) {
        callback(new Error(t('alert.silences.startRequired')))
      } else {
        callback()
      }
    },
    trigger: 'change' }],
  ends_at: [{
    validator: (_rule, _value, callback) => {
      if (form.silence_type === 2 && !form.ends_at) {
        callback(new Error(t('alert.silences.endRequired')))
      } else {
        callback()
      }
    },
    trigger: 'change' }] }

const loadData = async () => {
  loading.value = true
  try {
    const { data } = await getAlertSilenceList(queryParams)
    silenceList.value = data.data.items
    total.value = data.data.total
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  dialogTitle.value = t('alert.silences.create')
  form.id = 0
  form.name = ''
  form.description = ''
  form.silence_type = 1
  form.label_matchers = [{ key: '', operator: '==', value: '' }]
  form.starts_at = ''
  form.ends_at = ''
  form.comment = ''
  dialogVisible.value = true
}

const handleEdit = (row: AlertSilence) => {
  dialogTitle.value = t('alert.silences.edit')
  form.id = row.id
  form.name = row.name
  form.description = row.description
  form.silence_type = row.silence_type
  form.label_matchers = JSON.parse(JSON.stringify(row.label_matchers))
  form.starts_at = row.starts_at
  form.ends_at = row.ends_at || ''
  form.comment = row.comment
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    try {
      const payload: Record<string, unknown> = {
        name: form.name,
        description: form.description,
        silence_type: form.silence_type,
        label_matchers: form.label_matchers,
        comment: form.comment }

      // 预约静默才传时间
      if (form.silence_type === 2) {
        payload.starts_at = form.starts_at
        payload.ends_at = form.ends_at
      }

      if (form.id) {
        await updateAlertSilence(form.id, payload)
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createAlertSilence(payload)
        ElMessage.success(t('common.createSuccess'))
      }
      dialogVisible.value = false
      loadData()
    } catch {
      // ignored
    }
  })
}

// 启用静默规则
const handleEnable = async (row: AlertSilence) => {
  try {
    // 预约静默：如果时间已过期，弹出编辑框让用户更新时间
    if (row.silence_type === 2 && row.ends_at && dayjs(row.ends_at).isBefore(dayjs())) {
      await ElMessageBox.confirm(
        t('alert.silences.expiredMsg'),
        t('alert.silences.expiredTitle'),
        { type: 'warning', confirmButtonText: t('alert.silences.goEdit'), cancelButtonText: t('common.cancel') }
      )
      handleEdit(row)
      return
    }

    await ElMessageBox.confirm(
      t('alert.silences.enableMsg'),
      t('alert.silences.enableTitle'),
      { type: 'info', confirmButtonText: t('common.enabled'), cancelButtonText: t('common.cancel') }
    )
    await updateAlertSilence(row.id, { status: 1 })
    ElMessage.success(t('alert.silences.enabled'))
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

// 关闭静默规则
const handleDisable = async (row: AlertSilence) => {
  try {
    await ElMessageBox.confirm(t('alert.silences.disableMsg'), t('alert.silences.disableTitle'), {
      type: 'warning',
      confirmButtonText: t('alert.silences.disable'),
      cancelButtonText: t('common.cancel') })
    await cancelAlertSilence(row.id)
    ElMessage.success(t('alert.silences.disabled'))
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const handleDelete = async (row: AlertSilence) => {
  try {
    await ElMessageBox.confirm(t('alert.silences.confirmDelete'), t('alert.silences.deleteTitle'), {
      type: 'warning' })
    await deleteAlertSilence(row.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadData()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const addMatcher = () => {
  form.label_matchers.push({ key: '', operator: '==', value: '' })
}

const removeMatcher = (index: number) => {
  form.label_matchers.splice(index, 1)
}

const handleDialogClose = () => {
  formRef.value?.resetFields()
}

// 药丸色调：生效中给绿，停用与过期都是中性——过期不是错误，只是失效了
const statusTone = (status: number) => (status === 1 ? 'green' : 'neutral')

const getStatusText = (status: number) => {
  const map: Record<number, string> = {
    0: t('alert.silences.disabled'),
    1: t('alert.silences.enabled'),
    2: t('alert.silences.expired') }
  return map[status] || t('common.unknown')
}

const formatTime = (time: string) => {
  if (!time) return '-'
  return dayjs(time).format('YYYY-MM-DD HH:mm')
}

onMounted(() => {
  loadData()
})
</script>

<style scoped lang="scss">

/* 删除匹配器：默认弱色，悬停才变红 —— 破坏性操作不常驻红色 */
.matcher-remove {
  color: var(--td-text-placeholder);
  flex-shrink: 0;

  &:hover {
    color: var(--td-color-danger);
  }
}
// 列表样式在 _apple.scss 里，这一页只留对话框相关。

.labels-cell { display: flex; align-items: center; gap: 4px; overflow: hidden; }
.desc { max-width: 0; overflow: hidden; text-overflow: ellipsis; }

// 对话框里的匹配器编辑行
.matcher-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.type-hint {
  color: var(--td-text-placeholder);
  font-size: 12px;
  margin-left: 4px;
}

</style>
