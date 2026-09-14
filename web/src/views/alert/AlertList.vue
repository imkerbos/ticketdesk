<template>
  <!-- 结构同工单列表：.page / .toolbar / .card / table.issues -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('alert.listTitle') }}</h1>
      <div class="grow"></div>
      <el-popover placement="bottom-end" :width="480" trigger="click">
        <template #reference>
          <button class="btn secondary">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 6h16M7 12h10M10 18h4" /></svg>
            {{ t('alert.labelFilter') }}
            <span v-if="labelFilters.length" class="btn-count">{{ labelFilters.length }}</span>
          </button>
        </template>
        <div class="label-filter-panel">
          <div v-for="(filter, index) in labelFilters" :key="index" class="label-filter-row">
            <el-select v-model="filter.key" :placeholder="t('alert.labelKeyPlaceholder')" size="small" style="width: 140px" filterable allow-create default-first-option>
              <el-option v-for="k in labelKeyOptions" :key="k" :label="k" :value="k" />
            </el-select>
            <el-select v-model="filter.op" size="small" style="width: 110px">
              <el-option :label="t('alert.opEq')" value="==" />
              <el-option :label="t('alert.opNe')" value="!=" />
              <el-option :label="t('alert.opMatch')" value="=~" />
              <el-option :label="t('alert.opNotMatch')" value="!~" />
            </el-select>
            <el-input v-model="filter.value" :placeholder="t('alert.labelValuePlaceholder')" size="small" style="width: 160px" />
            <el-button :icon="Close" size="small" text type="danger" @click="removeLabelFilter(index)" />
          </div>
          <div class="label-filter-actions">
            <el-button size="small" text type="primary" :icon="Plus" @click="addLabelFilter">{{ t('alert.addCondition') }}</el-button>
            <el-button size="small" type="primary" @click="applyLabelFilters">{{ t('alert.apply') }}</el-button>
          </div>
        </div>
      </el-popover>
    </div>

    <div class="toolbar">
      <label class="search">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
        <input v-model="queryParams.alert_name" type="search" :placeholder="t('alert.searchPlaceholder')" @keyup.enter="handleQuery" @search="handleQuery" />
      </label>
      <el-select v-model="queryParams.status" :placeholder="t('issue.status')" clearable class="filter-select" @change="handleQuery">
        <el-option :label="t('alert.statusMap.firing')" value="firing" />
        <el-option :label="t('alert.statusMap.resolved')" value="resolved" />
      </el-select>
      <el-select v-model="queryParams.severity" :placeholder="t('alert.severity')" clearable class="filter-select" @change="handleQuery">
        <el-option :label="t('alert.severityMap.critical')" value="critical" />
        <el-option :label="t('alert.severityMap.warning')" value="warning" />
        <el-option :label="t('alert.severityMap.info')" value="info" />
      </el-select>
      <button class="btn secondary" @click="handleReset">
        <el-icon><Refresh /></el-icon>{{ t('common.reset') }}
      </button>
      <div class="grow"></div>
      <el-select v-if="viewMode === 'group'" v-model="groupBy" class="filter-select" @change="loadGroupData">
        <el-option :label="t('alert.groupCluster')" value="cluster" />
        <el-option :label="t('alert.groupNamespace')" value="namespace" />
        <el-option :label="t('alert.groupService')" value="service" />
        <el-option :label="t('alert.instance')" value="instance" />
      </el-select>
    </div>

    <section class="card">
      <div class="kpis">
        <!-- 指标可点：点一下就是一次筛选，比再去下拉里选一遍快 -->
        <button class="kpi as-filter" :class="{ 'is-active': !queryParams.status && !queryParams.severity }" @click="handleStatClick()">
          <div class="k">{{ t('alert.metricTotal') }}</div><div class="v">{{ stats.total }}</div>
        </button>
        <button class="kpi as-filter" :class="{ 'is-active': queryParams.status === 'firing' && !queryParams.severity }" @click="handleStatClick('firing')">
          <div class="k">{{ t('alert.metricActive') }}</div><div class="v">{{ stats.firing }}</div>
        </button>
        <button class="kpi as-filter" :class="{ 'is-active': queryParams.status === 'resolved' && !queryParams.severity }" @click="handleStatClick('resolved')">
          <div class="k">{{ t('alert.metricResolved') }}</div><div class="v">{{ stats.resolved }}</div>
        </button>
        <button class="kpi as-filter" :class="{ 'is-active': queryParams.severity === 'critical' && !queryParams.status }" @click="handleStatClick(undefined, 'critical')">
          <div class="k">{{ t('alert.severityMap.critical') }}</div><div class="v">{{ stats.critical }}</div>
        </button>
        <div class="grow"></div>
        <div class="count">{{ t('common.total', { n: total }) }}</div>
        <div class="seg" role="group" :aria-label="t('alert.listView')">
          <button :aria-pressed="viewMode === 'list'" @click="switchView('list')">{{ t('alert.listView') }}</button>
          <button :aria-pressed="viewMode === 'group'" @click="switchView('group')">{{ t('alert.groupView') }}</button>
        </div>
      </div>

      <div v-if="viewMode === 'list'" v-loading="loading" class="table-wrap">
        <table class="issues">
          <colgroup>
            <col /><col style="width: 84px" /><col style="width: 86px" /><col style="width: 300px" />
            <col style="width: 110px" /><col style="width: 150px" /><col style="width: 140px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('alert.name') }}</th>
              <th>{{ t('alert.severity') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th>{{ t('alert.labels') }}</th>
              <th>{{ t('alert.linkedIssue') }}</th>
              <th>{{ t('alert.startsAt') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in alertList" :key="row.id" @click="handleRowClick(row, $event)">
              <td>
                <div class="title-cell">
                  <span class="dot" :style="{ background: severityColor(row.severity) }"></span>
                  <span class="txt">{{ row.alert_name }}</span>
                  <span class="fingerprint">{{ row.fingerprint.substring(0, 8) }}</span>
                </div>
              </td>
              <td><span class="pill" :class="severityTone(row.severity)">{{ getSeverityText(row.severity) }}</span></td>
              <td><span class="pill" :class="row.status === 'firing' ? 'orange' : 'green'">{{ getStatusText(row.status) }}</span></td>
              <td>
                <div class="labels-cell">
                  <span v-for="[key, value] in visibleLabels(row.labels)" :key="key" class="pill neutral" :title="`${key}=${value}`">
                    {{ key }}={{ value }}
                  </span>
                  <el-tooltip v-if="hiddenLabelCount(row.labels) > 0" placement="top" :show-after="120">
                    <template #content>
                      <div v-for="[key, value] in allLabels(row.labels)" :key="key">{{ key }}={{ value }}</div>
                    </template>
                    <span class="pill neutral">+{{ hiddenLabelCount(row.labels) }}</span>
                  </el-tooltip>
                  <span v-if="allLabels(row.labels).length === 0" class="muted">-</span>
                </div>
              </td>
              <td>
                <a v-if="row.issue_key" class="key link" @click.stop="$router.push(`/issues/${row.issue_key}`)">{{ row.issue_key }}</a>
                <span v-else class="muted">-</span>
              </td>
              <td class="time">{{ formatTime(row.starts_at) }}</td>
              <td>
                <div class="row-actions">
                  <button v-if="row.status === 'firing' && !row.ack_at" class="link-btn" @click.stop="handleAck(row)">{{ t('alert.ack') }}</button>
                  <button v-if="row.status === 'firing'" class="link-btn" @click.stop="handleResolve(row)">{{ t('alert.resolve') }}</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && alertList.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('alert.empty')" />
        </div>

        <div v-if="total > queryParams.page_size" class="table-foot">
          <el-pagination
            v-model:current-page="queryParams.page"
            v-model:page-size="queryParams.page_size"
            :total="total"
            :page-sizes="[10, 20, 50, 100]"
            layout="sizes, prev, pager, next"
            @size-change="handlePageChange"
            @current-change="handlePageChange"
          />
        </div>
      </div>

      <!-- 分组视图 -->
      <div v-if="viewMode === 'group'" v-loading="loading">
        <el-row :gutter="20">
          <el-col
            v-for="group in groupData"
            :key="group.group_value"
            :xs="24"
            :sm="12"
            :md="8"
            :lg="6"
            class="group-col"
          >
            <div class="group-card">
              <div class="group-name">{{ group.group_value }}</div>
              <div class="group-total">{{ group.count }}</div>
              <div class="group-divider"></div>
              <div class="group-severity-row">
                <div class="severity-item critical">
                  <span class="severity-dot"></span>
                  <span>{{ t('alert.severityMap.critical') }} {{ group.severity.critical || 0 }}</span>
                </div>
                <div class="severity-item warning">
                  <span class="severity-dot"></span>
                  <span>{{ t('alert.severityMap.warning') }} {{ group.severity.warning || 0 }}</span>
                </div>
                <div class="severity-item info">
                  <span class="severity-dot"></span>
                  <span>{{ t('alert.severityMap.info') }} {{ group.severity.info || 0 }}</span>
                </div>
              </div>
            </div>
          </el-col>
        </el-row>
      </div>
    </section>

    <!-- 确认对话框 -->
    <el-dialog v-model="ackDialogVisible" :title="t('alert.ackTitle')" width="460px" class="alert-dialog">
      <div class="dialog-icon-header">
        <div class="dialog-icon ack">
          <el-icon><Check /></el-icon>
        </div>
        <p class="dialog-tip">{{ t('alert.ackTip') }}</p>
      </div>
      <el-form :model="ackForm" label-position="top">
        <el-form-item :label="t('alert.remark')">
          <el-input
            v-model="ackForm.comment"
            type="textarea"
            :rows="3"
            :placeholder="t('alert.ackPlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ackDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" @click="confirmAck">
          <el-icon><Check /></el-icon>
          {{ t('alert.ack') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 解决对话框 -->
    <el-dialog v-model="resolveDialogVisible" :title="t('alert.resolveTitle')" width="460px" class="alert-dialog">
      <div class="dialog-icon-header">
        <div class="dialog-icon resolve">
          <el-icon><CircleCheck /></el-icon>
        </div>
        <p class="dialog-tip">{{ t('alert.resolveTip') }}</p>
      </div>
      <el-form :model="resolveForm" label-position="top">
        <el-form-item :label="t('alert.remark')">
          <el-input
            v-model="resolveForm.comment"
            type="textarea"
            :rows="3"
            :placeholder="t('alert.resolvePlaceholder')"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resolveDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="success" @click="confirmResolve">
          <el-icon><CircleCheck /></el-icon>
          {{ t('alert.resolve') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Refresh, Check, CircleCheck,
  Close, Plus
} from '@element-plus/icons-vue'
import { getAlertList, getAlertStats, ackAlert, resolveAlert, getAlertGroup, getAlertLabelKeys } from '@/api/alert'
import type { Alert, AlertGroupItem, AlertStatsResponse } from '@/types/alert'
import dayjs from 'dayjs'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const alertList = ref<Alert[]>([])
const total = ref(0)
const viewMode = ref<'list' | 'group'>('list')
const groupBy = ref('cluster')
const groupData = ref<AlertGroupItem[]>([])

const stats = reactive<AlertStatsResponse>({ total: 0, firing: 0, resolved: 0, critical: 0, warning: 0, info: 0 })

const labelFilters = reactive<Array<{ key: string; op: string; value: string }>>([])
const labelKeyOptions = ref<string[]>([])
const queryParams = reactive({
  page: 1, page_size: 20,
  status: undefined as 'firing' | 'resolved' | undefined,
  severity: undefined as 'critical' | 'warning' | 'info' | undefined,
  alert_name: undefined as string | undefined,
  issue_id: undefined as number | undefined,
  label_filters: undefined as string | undefined })

const ackDialogVisible = ref(false)
const ackForm = reactive({ id: 0, comment: '' })
const resolveDialogVisible = ref(false)
const resolveForm = reactive({ id: 0, comment: '' })

const loadData = async () => {
  loading.value = true
  try {
    const { data } = await getAlertList(queryParams)
    alertList.value = data.data.items
    total.value = data.data.total
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const { data } = await getAlertStats()
    Object.assign(stats, data.data)
  } catch {
    // ignored
  }
}

const loadGroupData = async () => {
  loading.value = true
  try {
    const { data } = await getAlertGroup(groupBy.value, {
      status: queryParams.status, severity: queryParams.severity })
    groupData.value = data.data.items
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const syncQueryToURL = () => {
  const query: Record<string, string> = {}
  if (queryParams.status) query.status = queryParams.status
  if (queryParams.severity) query.severity = queryParams.severity
  if (queryParams.alert_name) query.alert_name = queryParams.alert_name
  if (queryParams.issue_id) query.issue_id = String(queryParams.issue_id)
  if (queryParams.label_filters) query.label_filters = queryParams.label_filters
  if (queryParams.page > 1) query.page = String(queryParams.page)
  if (queryParams.page_size !== 20) query.page_size = String(queryParams.page_size)
  router.replace({ query })
}

const addLabelFilter = () => {
  labelFilters.push({ key: '', op: '==', value: '' })
}

const removeLabelFilter = (index: number) => {
  labelFilters.splice(index, 1)
  applyLabelFilters()
}

const applyLabelFilters = () => {
  const parts = labelFilters
    .filter(f => f.key && f.value)
    .map(f => `${f.key}${f.op}${f.value}`)
  queryParams.label_filters = parts.length > 0 ? parts.join(',') : undefined
  queryParams.page = 1
  syncQueryToURL()
  if (viewMode.value === 'list') { loadData() } else { loadGroupData() }
}

const handleQuery = () => {
  queryParams.page = 1
  syncQueryToURL()
  if (viewMode.value === 'list') { loadData() } else { loadGroupData() }
}

const handleStatClick = (status?: 'firing' | 'resolved', severity?: 'critical' | 'warning' | 'info') => {
  queryParams.status = status
  queryParams.severity = severity
  handleQuery()
}

const handleReset = () => {
  queryParams.page = 1; queryParams.page_size = 20
  queryParams.status = undefined; queryParams.severity = undefined; queryParams.alert_name = undefined
  queryParams.issue_id = undefined; queryParams.label_filters = undefined
  labelFilters.splice(0, labelFilters.length)
  router.replace({ query: {} })
  if (viewMode.value === 'list') { loadData() } else { loadGroupData() }
}

const handlePageChange = () => {
  syncQueryToURL()
  loadData()
}

const switchView = (mode: 'list' | 'group') => {
  viewMode.value = mode
  handleViewModeChange()
}

const handleViewModeChange = () => { if (viewMode.value === 'list') { loadData() } else { loadGroupData() } }
const handleRowClick = (row: Alert, event: MouseEvent) => {
  const path = `/alerts/${row.id}`
  if (event.metaKey || event.ctrlKey) {
    window.open(router.resolve(path).href, '_blank')
  } else {
    router.push(path)
  }
}
const handleAck = (row: Alert) => { ackForm.id = row.id; ackForm.comment = ''; ackDialogVisible.value = true }
const confirmAck = async () => {
  try {
    await ackAlert(ackForm.id, ackForm.comment)
    ElMessage.success(t('alert.ackSuccess')); ackDialogVisible.value = false; loadData()
  } catch { /* ignored */ }
}

const handleResolve = (row: Alert) => { resolveForm.id = row.id; resolveForm.comment = ''; resolveDialogVisible.value = true }
const confirmResolve = async () => {
  try {
    await resolveAlert(resolveForm.id, resolveForm.comment)
    ElMessage.success(t('alert.resolveSuccess')); resolveDialogVisible.value = false; loadData()
  } catch { /* ignored */ }
}

// 告警级别：圆点色与药丸色调。原来的 getSeverityType 只是把级别映射成
// el-tag 的 type，换成原生药丸后不需要了。
const severityColor = (s: string) =>
  s === 'critical' ? 'var(--td-color-danger)' : s === 'warning' ? 'var(--td-color-warning)' : 'var(--td-text-placeholder)'

const severityTone = (s: string) => (s === 'critical' ? 'sla' : s === 'warning' ? 'orange' : 'neutral')
const getSeverityText = (severity: string) => {
  const map: Record<string, string> = { critical: t('alert.severityMap.critical'), warning: t('alert.severityMap.warning'), info: t('alert.severityMap.info') }
  return map[severity] || severity
}
const getStatusText = (status: string) => {
  const map: Record<string, string> = {
    firing: t('alert.statusMap.firing'),
    acked: t('alert.statusMap.acked'),
    resolved: t('alert.statusMap.resolved') }
  return map[status] || status
}
// 行内最多显示 2 个标签。告警的标签动辄十几个，全铺开会把行高撑到 100px 以上，
// 且各行不等高，扫读成本极高。
const INLINE_LABEL_LIMIT = 2
// service / instance 逐行不同，是真正能区分告警的；cluster / namespace 往往整列一个值，
// 排在前面等于把两个格子浪费在噪音上，所以按区分度排序。
const LABEL_PRIORITY = ['service', 'instance', 'cluster', 'namespace']

const allLabels = (labels: Record<string, string>): [string, string][] => {
  if (!labels) return []
  const entries = Object.entries(labels)
  return entries.sort(([a], [b]) => {
    const ia = LABEL_PRIORITY.indexOf(a)
    const ib = LABEL_PRIORITY.indexOf(b)
    return (ia < 0 ? LABEL_PRIORITY.length : ia) - (ib < 0 ? LABEL_PRIORITY.length : ib)
  })
}
const visibleLabels = (labels: Record<string, string>) => allLabels(labels).slice(0, INLINE_LABEL_LIMIT)
const hiddenLabelCount = (labels: Record<string, string>) =>
  Math.max(0, allLabels(labels).length - INLINE_LABEL_LIMIT)
const formatTime = (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm')

onMounted(() => {
  // 从 URL 读取筛选条件
  const q = route.query
  if (q.status) queryParams.status = q.status as typeof queryParams.status
  if (q.severity) queryParams.severity = q.severity as typeof queryParams.severity
  if (q.alert_name) queryParams.alert_name = q.alert_name as string
  if (q.issue_id) queryParams.issue_id = Number(q.issue_id)
  if (q.label_filters) {
    queryParams.label_filters = q.label_filters as string
    // 从 URL 还原标签筛选条件到结构化数组
    const parts = (q.label_filters as string).split(',')
    for (const part of parts) {
      const trimmed = part.trim()
      if (!trimmed) continue
      let op = '=='
      let idx = -1
      for (const candidate of ['!~', '=~', '!=', '==']) {
        idx = trimmed.indexOf(candidate)
        if (idx > 0) { op = candidate; break }
      }
      if (idx > 0) {
        labelFilters.push({
          key: trimmed.substring(0, idx).trim(),
          op,
          value: trimmed.substring(idx + op.length).trim() })
      }
    }
  }
  if (q.page) queryParams.page = Number(q.page)
  if (q.page_size) queryParams.page_size = Number(q.page_size)

  loadData(); loadStats(); loadLabelKeys()
})

const loadLabelKeys = async () => {
  try {
    const { data } = await getAlertLabelKeys()
    labelKeyOptions.value = data.data || []
  } catch {
    // ignored
  }
}
</script>

<style scoped lang="scss">
// 列表部分的样式都在 _apple.scss 里，这一页只留分组视图、对话框和标签筛选面板。

// 告警指纹：跟在名字后面的弱色短哈希，用来区分同名告警的不同实例
.fingerprint {
  font-family: var(--td-font-mono);
  font-size: 11px;
  color: var(--td-text-placeholder);
}

// 标签列：一行放不下就截断，多出来的收进 +N
.labels-cell {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow: hidden;
}

.key.link { cursor: pointer; }
.key.link:hover { text-decoration: underline; text-underline-offset: 2px; }

.group-col { margin-bottom: 20px; }

.group-card {
  background: var(--td-bg-card);
  border-radius: 12px;
  padding: 20px;
  border: 1px solid var(--td-divider-color);
  transition: border-color 150ms ease-out;
  cursor: pointer;

  &:hover {
    border-color: var(--td-border-color-dark);
  }

  .group-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--td-text-regular);
    margin-bottom: 8px;
  }

  .group-total {
    font-size: 36px;
    font-weight: 700;
    color: var(--td-color-primary);
    margin-bottom: 12px;
  }

  .group-divider {
    height: 1px;
    background: var(--td-bg-section);
    margin-bottom: 12px;
  }

  .group-severity-row {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
  }

  .severity-item {
    display: flex;
    align-items: center;
    gap: 4px;

    .severity-dot {
      width: 8px;
      height: 8px;
      border-radius: 50%;
    }

    &.critical { color: var(--td-color-danger); .severity-dot { background: var(--td-color-danger); } }
    &.warning { color: var(--td-color-warning); .severity-dot { background: var(--td-color-warning); } }
    &.info { color: var(--td-text-secondary); .severity-dot { background: var(--td-text-placeholder); } }
  }
}

// 对话框
.alert-dialog {
  .dialog-icon-header {
    text-align: center;
    margin-bottom: 20px;

    .dialog-icon {
      width: 56px; height: 56px;
      margin: 0 auto 12px;
      border-radius: 12px;
      display: flex; align-items: center; justify-content: center;
      font-size: 22px;

      &.ack { background: var(--td-tag-primary-bg); color: var(--td-tag-primary-text); }
      &.resolve { background: var(--td-tag-success-bg); color: var(--td-tag-success-text); }
    }

    .dialog-tip { font-size: 14px; color: var(--td-text-secondary); margin: 0; }
  }
}

// 标签筛选面板
.label-filter-panel {
  .label-filter-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 8px;
  }

  .label-filter-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 12px;
    padding-top: 12px;
    border-top: 1px solid var(--td-divider-color);
  }
}

// 响应式
@media (max-width: 768px) {
  .filter-select { width: 100% !important; }
}
</style>
