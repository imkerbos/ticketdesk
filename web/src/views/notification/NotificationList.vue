<template>
  <!-- 通知是时间流，不是表格：保留列表形态，但把筛选换成分段控件、
       条目换成 _apple.scss 的发丝线行，和其它页面同一套语言。 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('notification.title') }}</h1>
      <div class="grow"></div>
      <button v-if="notificationStore.unreadCount > 0" class="btn secondary" @click="handleMarkAllAsRead">
        {{ t('notification.markAllRead') }}
      </button>
    </div>

    <div class="toolbar">
      <div class="seg" role="group" :aria-label="t('notification.type')">
        <button :aria-pressed="filterStatus === 'all'" @click="selectStatus('all')">{{ t('common.all') }}</button>
        <button :aria-pressed="filterStatus === 'unread'" @click="selectStatus('unread')">
          {{ t('notification.unread') }}
          <template v-if="notificationStore.unreadCount > 0">({{ notificationStore.unreadCount > 99 ? '99+' : notificationStore.unreadCount }})</template>
        </button>
        <button :aria-pressed="filterStatus === 'read'" @click="selectStatus('read')">{{ t('notification.read') }}</button>
      </div>
      <el-select v-model="filterType" :placeholder="t('notification.type')" clearable class="filter-select" @change="handleFilterChange">
        <el-option :label="t('notification.typeAssigned')" value="issue_assigned" />
        <el-option :label="t('notification.typeStatusChanged')" value="issue_status_changed" />
        <el-option :label="t('notification.typeCommented')" value="issue_commented" />
        <el-option :label="t('notification.typeMention')" value="mention" />
        <el-option :label="t('notification.typeUpdated')" value="issue_updated" />
      </el-select>
    </div>

    <section class="card">
      <div v-if="loading" class="notify-loading">
        <el-skeleton :rows="6" animated />
      </div>

      <div v-else-if="notifications.length === 0" class="empty">
        <!-- 说明句跟着当前筛选走：「未读」下为空是好事，和真的一条通知都没有不是一回事 -->
        <TdEmptyState preset="no-data" :title="t('notification.empty')" :description="emptyDescription" />
      </div>

      <ul v-else class="notify-list">
        <li
          v-for="item in notifications"
          :key="item.id"
          class="notify-item"
          :class="{ 'is-unread': !item.is_read }"
          @click="handleClick(item)"
        >
          <!-- 未读点：整行唯一的状态信号。类型在标题里写着，不需要彩色图标块 -->
          <span class="dot" :style="{ background: item.is_read ? 'transparent' : 'var(--td-color-primary)' }"></span>

          <div class="notify-main">
            <div class="notify-title">
              <span class="txt">{{ item.title }}</span>
              <span v-if="item.entity_key" class="key">{{ item.entity_key }}</span>
            </div>
            <div v-if="item.content" class="notify-body">{{ item.content }}</div>
          </div>

          <span v-if="item.actor_name" class="muted notify-actor">{{ item.actor_name }}</span>
          <!-- 相对时间带中文，不走 .time 的等宽：中文跟着等宽排，数字和单位之间会豁开一格 -->
          <span class="muted notify-time">{{ formatTime(item.created_at) }}</span>

          <div class="row-actions">
            <button v-if="!item.is_read" class="link-btn" @click.stop="handleMarkRead(item.id)">{{ t('notification.markRead') }}</button>
            <el-dropdown trigger="click">
              <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="handleDelete(item.id)">{{ t('common.delete') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </li>
      </ul>

      <div v-if="total > pageSize" class="table-foot">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useNotificationStore } from '@/stores/notification'
import { getNotificationList, markAsRead, deleteNotification } from '@/api/notification'
import type { NotificationItem } from '@/types/notification'

const { t } = useI18n()

const router = useRouter()
const notificationStore = useNotificationStore()

const notifications = ref<NotificationItem[]>([])
const loading = ref(false)
const currentPage = ref(1)
const pageSize = 15
const total = ref(0)
const filterStatus = ref('all')
const filterType = ref('')

const emptyDescription = computed(() => {
  if (filterType.value) return t('notification.emptyFilteredDesc')
  if (filterStatus.value === 'unread') return t('notification.emptyUnreadDesc')
  if (filterStatus.value === 'read') return t('notification.emptyReadDesc')
  return t('notification.emptyAllDesc')
})

const fetchData = async () => {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: currentPage.value,
      page_size: pageSize,
    }
    if (filterStatus.value === 'unread') params.is_read = false
    if (filterStatus.value === 'read') params.is_read = true
    if (filterType.value) params.type = filterType.value

    const res = await getNotificationList(params)
    notifications.value = res.data.data.items || []
    total.value = res.data.data.total
  } catch {
    // 静默处理
  } finally {
    loading.value = false
  }
}

const handleClick = (notification: NotificationItem) => {
  if (!notification.is_read) {
    handleMarkRead(notification.id)
  }
  if (notification.entity_type === 'issue' && notification.entity_key) {
    router.push(`/issues/${notification.entity_key}`)
  }
}

const handleMarkRead = async (id: number) => {
  try {
    await markAsRead(id)
    const notif = notifications.value.find((n) => n.id === id)
    if (notif && !notif.is_read) {
      notif.is_read = true
      notificationStore.unreadCount = Math.max(0, notificationStore.unreadCount - 1)
    }
  } catch {
    ElMessage.error(t('common.operationFailed'))
  }
}

const handleMarkAllAsRead = async () => {
  await notificationStore.markAllAsRead()
  notifications.value.forEach((n) => {
    n.is_read = true
  })
  ElMessage.success(t('notification.allMarkedRead'))
}

const handleDelete = async (id: number) => {
  try {
    await deleteNotification(id)
    const idx = notifications.value.findIndex((n) => n.id === id)
    if (idx >= 0) {
      const notif = notifications.value[idx]
      if (!notif.is_read) {
        notificationStore.unreadCount = Math.max(0, notificationStore.unreadCount - 1)
      }
      notifications.value.splice(idx, 1)
      total.value--
    }
    ElMessage.success(t('notification.deleted'))
  } catch {
    ElMessage.error(t('issue.msg.deleteFailed2'))
  }
}

// 分段控件选中即筛选（原来是 el-radio-group 的 change）
const selectStatus = (v: 'all' | 'unread' | 'read') => {
  filterStatus.value = v
  handleFilterChange()
}

const handleFilterChange = () => {
  currentPage.value = 1
  fetchData()
}

const handlePageChange = () => {
  fetchData()
}

const formatTime = (dateStr: string): string => {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffMin = Math.floor(diffMs / (1000 * 60))
  const diffHour = Math.floor(diffMs / (1000 * 60 * 60))
  const diffDay = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffMin < 1) return t('notification.justNow')
  if (diffMin < 60) return t('notification.minutesAgo', { n: diffMin })
  if (diffHour < 24) return t('notification.hoursAgo', { n: diffHour })
  if (diffDay < 7) return t('notification.daysAgo', { n: diffDay })

  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

onMounted(() => {
  fetchData()
  notificationStore.fetchUnreadCount()
})
</script>

<style scoped lang="scss">
// 通知条目：一行发丝线，左边未读点、中间标题与正文、右边人和时间。
// 原来每行左侧有一个 42px 的实心彩色图标块按类型上色——类型在标题里
// 已经写清楚了，色块只是把每行推高 20px。

.notify-list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.notify-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  height: 52px;
  border-bottom: 1px solid var(--td-divider-color);
  cursor: pointer;
  transition: background-color var(--td-duration-fast) var(--td-ease-out);

  &:last-child { border-bottom: 0; }
  &:hover { background: var(--td-bg-card-hover); }
}

.notify-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.notify-title {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;

  .txt {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.is-unread .notify-title .txt { font-weight: var(--td-weight-semibold); }

.notify-body {
  font-size: 12px;
  color: var(--td-text-placeholder);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notify-actor { font-size: 12px; white-space: nowrap; }
.notify-time { font-size: 12px; white-space: nowrap; font-variant-numeric: tabular-nums; }

.notify-loading { padding: 20px; }
</style>
