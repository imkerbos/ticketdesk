<template>
  <!-- 结构同其它列表页：.page / .card / table.issues -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('alert.rules.title') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('alert.rules.create') }}
      </button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col style="width: 170px" /><col style="width: 96px" /><col style="width: 120px" /><col style="width: 96px" />
            <col style="width: 84px" /><col /><col style="width: 62px" /><col style="width: 96px" />
            <col style="width: 72px" /><col style="width: 86px" /><col style="width: 96px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('alert.rules.name') }}</th>
              <th>{{ t('issue.project') }}</th>
              <th>{{ t('alert.rules.datasource') }}</th>
              <th>{{ t('alert.rules.issueType') }}</th>
              <th>{{ t('alert.rules.severity') }}</th>
              <th>{{ t('alert.rules.extraMatch') }}</th>
              <th>{{ t('issue.priority') }}</th>
              <th>{{ t('alert.rules.mergeWindow') }}</th>
              <th>{{ t('alert.rules.autoResolve') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in ruleList" :key="row.id">
              <td>{{ row.name }}</td>
              <td><span class="pill neutral">{{ row.project_name }}</span></td>
              <td class="muted">{{ row.datasource_name || t('common.all') }}</td>
              <td class="muted">{{ row.issue_type_name }}</td>
              <td>
                <span v-if="getSeverityFromMatchers(row.label_matchers)" class="pill" :class="severityTone(getSeverityFromMatchers(row.label_matchers))">
                  {{ getSeverityLabel(getSeverityFromMatchers(row.label_matchers)) }}
                </span>
                <span v-else class="muted">{{ t('common.all') }}</span>
              </td>
              <td>
                <div class="labels-cell">
                  <template v-for="(matcher, index) in getExtraMatchers(row.label_matchers)" :key="index">
                    <span v-if="index < 2" class="pill neutral">{{ matcher.key }} {{ matcher.operator }} {{ matcher.value }}</span>
                  </template>
                  <span v-if="getExtraMatchers(row.label_matchers).length > 2" class="pill neutral">
                    +{{ getExtraMatchers(row.label_matchers).length - 2 }}
                  </span>
                  <span v-if="getExtraMatchers(row.label_matchers).length === 0" class="muted">-</span>
                </div>
              </td>
              <td>
                <span class="prio">
                  <span class="dot" :style="{ background: priorityColor(row.priority) }"></span>{{ row.priority }}
                </span>
              </td>
              <!-- 不套 .time：那是给纯数字/时间戳用的等宽样式，
                   「2 分钟」里的中文跟着等宽排，数字和单位之间会豁开一个空格宽 -->
              <td class="muted">{{ formatMergeWindow(row.merge_window) }}</td>
              <!-- 是/否是布尔配置，不是健康状态：不套徽章，弱色文字即可 -->
              <td class="muted">{{ row.auto_resolve ? t('common.yes') : t('common.no') }}</td>
              <td>
                <span class="pill" :class="row.status === 1 ? 'green' : 'neutral'">
                  {{ row.status === 1 ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click="handleEdit(row)">{{ t('common.edit') }}</button>
                  <el-dropdown trigger="click">
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

        <div v-if="!loading && ruleList.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('alert.rules.empty')" />
        </div>

        <div v-if="total > queryParams.page_size" class="table-foot">
          <el-pagination
            v-model:current-page="queryParams.page"
            v-model:page-size="queryParams.page_size"
            :total="total"
            layout="prev, pager, next"
            @current-change="loadData"
          />
        </div>
      </div>
    </section>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="700px"
      @close="handleDialogClose"
    >
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-row :gutter="16">
          <el-col :span="24">
            <el-form-item :label="t('alert.rules.name')" prop="name">
              <el-input v-model="form.name" :placeholder="t('alert.rules.namePlaceholder')" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="24">
            <el-form-item :label="t('issue.description')" prop="description">
              <el-input v-model="form.description" type="textarea" :rows="2" :placeholder="t('alert.rules.descPlaceholder')" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('alert.rules.datasource')" prop="datasource_id">
              <el-select v-model="form.datasource_id" :placeholder="t('alert.rules.datasourcePlaceholder')" style="width: 100%" filterable>
                <el-option
                  v-for="ds in datasourceList"
                  :key="ds.id"
                  :label="ds.name"
                  :value="ds.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('issue.project')" prop="project_id">
              <el-select v-model="form.project_id" :placeholder="t('alert.rules.projectPlaceholder')" style="width: 100%" filterable @change="handleProjectChange">
                <el-option
                  v-for="p in projectList"
                  :key="p.id"
                  :label="`${p.project_key} - ${p.name}`"
                  :value="p.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('alert.rules.issueType')" prop="issue_type_id">
              <el-select v-model="form.issue_type_id" :placeholder="t('alert.rules.issueTypePlaceholder')" style="width: 100%" :disabled="!form.project_id" filterable>
                <el-option
                  v-for="type in issueTypeList"
                  :key="type.id"
                  :label="type.display_name || type.name"
                  :value="type.id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('alert.rules.severity')" prop="severity">
              <el-select v-model="form.severity" :placeholder="t('alert.rules.severityAny')" style="width: 100%" clearable>
                <el-option :label="t('alert.rules.severityCritical')" value="critical" />
                <el-option :label="t('alert.rules.severityWarning')" value="warning" />
                <el-option :label="t('alert.rules.severityInfo')" value="info" />
              </el-select>
              <div class="form-tip">{{ t('alert.rules.severityTip') }}</div>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('alert.rules.issuePriority')" prop="priority">
              <el-select v-model="form.priority" :placeholder="t('alert.rules.priorityPlaceholder')" style="width: 100%">
                <el-option :label="t('issue.priorityMap.P0')" value="P0" />
                <el-option :label="t('issue.priorityMap.P1')" value="P1" />
                <el-option :label="t('issue.priorityMap.P2')" value="P2" />
                <el-option :label="t('issue.priorityMap.P3')" value="P3" />
              </el-select>
              <div class="form-tip">{{ t('alert.rules.priorityTip') }}</div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="24">
            <el-form-item :label="t('alert.rules.matchers')" prop="label_matchers">
              <div class="form-tip" style="margin-bottom: 8px;">
                {{ t('alert.rules.matchersTip1') }}
                {{ t('alert.rules.matchersTip2') }}
              </div>
              <div class="matchers-container">
                <div
                  v-for="(matcher, index) in form.label_matchers"
                  :key="index"
                  class="matcher-item"
                >
                  <el-input v-model="matcher.key" :placeholder="t('alert.rules.matcherKey')" class="matcher-input" />
                  <el-select v-model="matcher.operator" class="matcher-operator">
                    <el-option label="==" value="==" />
                    <el-option label="!=" value="!=" />
                    <el-option label="=~" value="=~" />
                    <el-option label="!~" value="!~" />
                  </el-select>
                  <el-input v-model="matcher.value" :placeholder="t('alert.rules.matcherValue')" class="matcher-input" />
                  <el-button
                    type="danger"
                    :icon="Delete"
                    circle
                    @click="removeMatcher(index)"
                  />
                </div>
                <el-button type="primary" text @click="addMatcher">
                  <el-icon><Plus /></el-icon>
                  {{ t('alert.rules.addMatcher') }}
                </el-button>
              </div>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item :label="t('alert.rules.mergeWindowSec')" prop="merge_window">
              <el-input-number
                v-model="form.merge_window"
                :min="0"
                :max="86400"
                :step="300"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item :label="t('alert.rules.autoResolve')" prop="auto_resolve">
              <el-switch v-model="form.auto_resolve" />
              <div class="form-tip">
                {{ t('alert.rules.autoResolveTip') }}
              </div>
            </el-form-item>
          </el-col>
        </el-row>
        <div class="form-tip" style="margin-top: -8px; margin-bottom: 16px;">
          {{ t('alert.rules.mergeWindowTip') }}
        </div>
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
  getAlertRuleList,
  createAlertRule,
  updateAlertRule,
  deleteAlertRule,
  getDatasourceList } from '@/api/alert'
import { getProjectList } from '@/api/project'
import type { AlertRule, LabelMatcher, AlertDatasource } from '@/types/alert'

const { t } = useI18n()

const loading = ref(false)
const ruleList = ref<AlertRule[]>([])
const total = ref(0)
const queryParams = reactive({
  page: 1,
  page_size: 20 })

// 项目、工单类型和数据源选项
const projectList = ref<{ id: number; name: string; project_key: string }[]>([])
const issueTypeList = ref<{ id: number; name: string; display_name: string }[]>([])
const datasourceList = ref<AlertDatasource[]>([])

const loadProjects = async () => {
  try {
    const { data } = await getProjectList({ page: 1, page_size: 100 })
    projectList.value = data.data.items || []
  } catch {
    // ignored
  }
}

const loadDatasources = async () => {
  try {
    const { data } = await getDatasourceList({ page: 1, page_size: 100 })
    datasourceList.value = data.data.items || []
  } catch {
    // ignored
  }
}

const loadIssueTypes = async (projectKey: string) => {
  try {
    const { getProjectIssueTypes } = await import('@/api/project')
    const { data } = await getProjectIssueTypes(projectKey)
    issueTypeList.value = data.data || []
  } catch {
    // ignored
  }
}

const handleProjectChange = (projectId: number) => {
  form.issue_type_id = undefined
  issueTypeList.value = []
  const project = projectList.value.find(p => p.id === projectId)
  if (project) {
    loadIssueTypes(project.project_key)
  }
}

const dialogVisible = ref(false)
const dialogTitle = ref('')
const formRef = ref<FormInstance>()
const form = reactive({
  id: 0,
  name: '',
  description: '',
  datasource_id: undefined as number | undefined,
  project_id: undefined as number | undefined,
  issue_type_id: undefined as number | undefined,
  severity: '' as string,
  label_matchers: [] as LabelMatcher[],
  priority: 'P2' as 'P0' | 'P1' | 'P2' | 'P3',
  assignee_id: undefined as number | undefined,
  auto_resolve: false,
  merge_window: 3600 })

const rules: FormRules = {
  name: [{ required: true, message: t('alert.rules.namePlaceholder'), trigger: ['blur', 'change'] }],
  datasource_id: [{ required: true, message: t('alert.rules.datasourcePlaceholder'), trigger: 'change' }],
  project_id: [{ required: true, message: t('alert.rules.projectPlaceholder'), trigger: 'change' }],
  issue_type_id: [{ required: true, message: t('alert.rules.issueTypePlaceholder'), trigger: 'change' }],
  priority: [{ required: true, message: t('alert.rules.priorityPlaceholder'), trigger: 'change' }] }

const loadData = async () => {
  loading.value = true
  try {
    const { data } = await getAlertRuleList(queryParams)
    ruleList.value = data.data.items
    total.value = data.data.total
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  dialogTitle.value = t('alert.rules.create')
  form.id = 0
  form.name = ''
  form.description = ''
  form.datasource_id = undefined
  form.project_id = undefined
  form.issue_type_id = undefined
  form.severity = ''
  form.label_matchers = []
  form.priority = 'P2'
  form.assignee_id = undefined
  form.auto_resolve = false
  form.merge_window = 3600
  dialogVisible.value = true
}

const handleEdit = async (row: AlertRule) => {
  dialogTitle.value = t('alert.rules.edit')
  form.id = row.id
  form.name = row.name
  form.description = row.description
  form.datasource_id = row.datasource_id
  form.project_id = row.project_id
  form.issue_type_id = row.issue_type_id
  const matchers: LabelMatcher[] = JSON.parse(JSON.stringify(row.label_matchers))
  // 从 label_matchers 中提取 severity 到独立字段
  const severityIdx = matchers.findIndex(m => m.key === 'severity' && m.operator === '==')
  if (severityIdx >= 0) {
    form.severity = matchers[severityIdx].value
    matchers.splice(severityIdx, 1)
  } else {
    form.severity = ''
  }
  form.label_matchers = matchers
  form.priority = row.priority
  form.assignee_id = row.assignee_id
  form.auto_resolve = row.auto_resolve
  form.merge_window = row.merge_window
  // 加载该项目的工单类型
  const project = projectList.value.find(p => p.id === row.project_id)
  if (project) {
    await loadIssueTypes(project.project_key)
  }
  dialogVisible.value = true
}

const handleSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return

    // 合并 severity 到 label_matchers
    const matchers: LabelMatcher[] = [...form.label_matchers]
    if (form.severity) {
      matchers.unshift({ key: 'severity', operator: '==', value: form.severity })
    }

    const submitData = {
      ...form,
      label_matchers: matchers }

    try {
      if (form.id) {
        await updateAlertRule(form.id, submitData)
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        if (!form.project_id || !form.issue_type_id || !form.datasource_id) {
          ElMessage.error(t('alert.rules.selectRequired'))
          return
        }
        await createAlertRule({
          ...submitData,
          datasource_id: form.datasource_id!,
          project_id: form.project_id!,
          issue_type_id: form.issue_type_id! })
        ElMessage.success(t('common.createSuccess'))
      }
      dialogVisible.value = false
      loadData()
    } catch {
      // ignored
    }
  })
}

const handleDelete = async (row: AlertRule) => {
  try {
    await ElMessageBox.confirm(t('alert.rules.confirmDelete'), t('issue.msg.tipTitle'), {
      type: 'warning' })
    await deleteAlertRule(row.id)
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

const formatMergeWindow = (seconds: number) => {
  if (seconds === 0) return t('alert.rules.noMerge')
  if (seconds < 60) return t('alert.rules.secs', { n: seconds })
  if (seconds < 3600) return t('alert.rules.mins', { n: Math.floor(seconds / 60) })
  return t('alert.rules.hours', { n: Math.floor(seconds / 3600) })
}

// 从 label_matchers 中提取 severity 值
const getSeverityFromMatchers = (matchers: LabelMatcher[]) => {
  const m = matchers?.find(m => m.key === 'severity' && m.operator === '==')
  return m?.value || ''
}

// 获取除 severity 外的额外匹配器
const getExtraMatchers = (matchers: LabelMatcher[]) => {
  return (matchers || []).filter(m => !(m.key === 'severity' && m.operator === '=='))
}

// 告警等级标签
const getSeverityLabel = (severity: string) => {
  const map: Record<string, string> = { critical: t('alert.severityMap.critical'), warning: t('alert.severityMap.warning'), info: t('alert.severityMap.info') }
  return map[severity] || severity
}

const severityTone = (s?: string) => (s === 'critical' ? 'sla' : s === 'warning' ? 'orange' : 'neutral')

const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)' }

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

onMounted(() => {
  loadData()
  loadProjects()
  loadDatasources()
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里，这一页只留对话框里的匹配器编辑区。

.labels-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
}

.matchers-container {
  width: 100%;

  .matcher-item {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
    padding: 12px;
    background: var(--td-bg-page);
    border-radius: 8px;
    border: 1px solid var(--td-border-color);

    .matcher-input {
      flex: 1;
    }

    .matcher-operator {
      width: 100px;
    }
  }
}

.form-tip {
  font-size: 12px;
  color: var(--td-color-info);
  margin-top: 4px;
  line-height: 1.5;
}

// 响应式
@media (max-width: 768px) {
}
</style>
