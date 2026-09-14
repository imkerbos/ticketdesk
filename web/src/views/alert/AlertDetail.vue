<template>
  <!-- 告警详情：这页原来全是内联 style 和 el-descriptions 的边框表格。
       改成和工单详情同一套：页头 + 属性行 + 标签 + 注解。 -->
  <div v-loading="loading" class="page">
    <div class="page-head">
      <div class="detail-title">
        <h1>{{ alert?.alert_name }}</h1>
        <div class="detail-meta">
          <span class="pill" :class="severityTone(alert?.severity)">{{ getSeverityText(alert?.severity) }}</span>
          <span class="pill" :class="alert?.status === 'firing' ? 'orange' : 'green'">{{ getStatusText(alert?.status) }}</span>
          <span class="key static">{{ alert?.fingerprint?.slice(0, 12) }}</span>
        </div>
      </div>
      <div class="grow"></div>
      <button v-if="alert?.status === 'firing' && !alert?.ack_at" class="btn secondary" @click="handleAck">
        {{ t('alert.ackTitle') }}
      </button>
      <button v-if="alert?.status === 'firing'" class="btn primary" @click="handleResolve">
        {{ t('alert.resolveTitle') }}
      </button>
    </div>

    <section class="card">
      <div class="card-head"><h2>{{ t('common.detail') }}</h2></div>
      <dl class="props">
        <div class="prop"><dt>{{ t('alert.source') }}</dt><dd>{{ alert?.source || '-' }}</dd></div>
        <div class="prop"><dt>{{ t('alert.startsAt') }}</dt><dd class="time">{{ formatTime(alert?.starts_at) }}</dd></div>
        <div class="prop"><dt>{{ t('alert.endsAt') }}</dt><dd class="time">{{ alert?.ends_at ? formatTime(alert.ends_at) : '-' }}</dd></div>
        <div class="prop">
          <dt>{{ t('alert.linkedIssue') }}</dt>
          <dd>
            <a v-if="alert?.issue_key" class="key" @click="$router.push(`/issues/${alert.issue_key}`)">{{ alert.issue_key }}</a>
            <span v-else class="muted">-</span>
          </dd>
        </div>
        <div class="prop">
          <dt>{{ t('alert.ackInfo') }}</dt>
          <dd>
            <span v-if="alert?.ack_at">{{ t('alert.ackedBy', { name: alert.ack_by_name, time: formatTime(alert.ack_at) }) }}</span>
            <span v-else class="muted">-</span>
          </dd>
        </div>
        <div class="prop"><dt>{{ t('alert.fingerprint') }}</dt><dd class="key static">{{ alert?.fingerprint }}</dd></div>
      </dl>
    </section>

    <section class="card">
      <div class="card-head"><h2>{{ t('alert.labels') }}</h2></div>
      <div class="chips">
        <span v-for="(value, key) in alert?.labels" :key="key" class="pill neutral">{{ key }} = {{ value }}</span>
        <span v-if="!alert?.labels || Object.keys(alert.labels).length === 0" class="muted">-</span>
      </div>
    </section>

    <section class="card">
      <div class="card-head"><h2>{{ t('alert.annotations') }}</h2></div>
      <dl class="props">
        <div v-for="(value, key) in alert?.annotations" :key="key" class="prop">
          <dt>{{ key }}</dt><dd>{{ value }}</dd>
        </div>
        <div v-if="!alert?.annotations || Object.keys(alert.annotations).length === 0" class="prop">
          <dd class="muted">-</dd>
        </div>
      </dl>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { getAlertDetail, ackAlert, resolveAlert } from '@/api/alert'
import type { Alert } from '@/types/alert'
import dayjs from 'dayjs'

const { t } = useI18n()

const route = useRoute()
// useRouter is available globally in template as $router, but we call it to ensure Vue Router is setup
useRouter()

const loading = ref(false)
const alert = ref<Alert>()

const loadData = async () => {
  loading.value = true
  try {
    const { data } = await getAlertDetail(Number(route.params.id), { _redirectOn404: true })
    alert.value = data.data
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const handleAck = async () => {
  try {
    await ackAlert(alert.value!.id)
    ElMessage.success(t('alert.ackSuccess'))
    loadData()
  } catch {
    // ignored
  }
}

const handleResolve = async () => {
  try {
    await resolveAlert(alert.value!.id)
    ElMessage.success(t('alert.resolveSuccess'))
    loadData()
  } catch {
    // ignored
  }
}

// 药丸色调（和告警列表共用同一套语义）
const severityTone = (v?: string) => (v === 'critical' ? 'sla' : v === 'warning' ? 'orange' : 'neutral')

const getSeverityText = (severity?: string) => {
  const known = ['critical', 'warning', 'info']
  return known.includes(severity || '') ? t(`alert.severityMap.${severity}`) : severity
}

const getStatusText = (status?: string) => {
  const known = ['firing', 'resolved', 'acked']
  return known.includes(status || '') ? t(`alert.statusMap.${status}`) : status
}

const formatTime = (time?: string) => {
  return time ? dayjs(time).format('YYYY-MM-DD HH:mm:ss') : '-'
}

onMounted(() => {
  loadData()
})
</script>

<style scoped lang="scss">
// 属性行：label 定宽、value 自适应，一行一条发丝线。
// 原来用的是 el-descriptions 的边框表格——那是 Excel 的语言。
.props {
  margin: 0;
  display: flex;
  flex-direction: column;
}

.prop {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 9px 18px;
  border-bottom: 1px solid var(--td-divider-color);

  &:last-child { border-bottom: 0; }

  dt {
    flex: 0 0 140px;
    font-size: 12.5px;
    color: var(--td-text-placeholder);
  }

  dd {
    margin: 0;
    flex: 1;
    min-width: 0;
    font-size: 13px;
    word-break: break-all;
  }
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 14px 18px;
}
</style>
