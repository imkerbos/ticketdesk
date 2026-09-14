<template>
  <div
    class="notification-item"
    :class="{ unread: !notification.is_read }"
    @click="$emit('click', notification)"
  >
    <div class="notif-icon">
      <el-icon :size="16" :color="iconColor">
        <component :is="iconName" />
      </el-icon>
    </div>
    <div class="notif-body">
      <div class="notif-title">{{ notification.title }}</div>
      <div v-if="notification.content" class="notif-content">{{ notification.content }}</div>
      <div class="notif-meta">
        <span v-if="notification.actor_name" class="notif-actor">{{ notification.actor_name }}</span>
        <span class="notif-time">{{ timeAgo(notification.created_at) }}</span>
      </div>
    </div>
    <div v-if="!notification.is_read" class="notif-dot"></div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import type { NotificationItem } from '@/types/notification'

const { t } = useI18n()

const props = defineProps<{
  notification: NotificationItem
}>()

defineEmits<{
  click: [notification: NotificationItem]
}>()

const iconName = computed(() => {
  switch (props.notification.type) {
    case 'issue_assigned':
      return 'UserFilled'
    case 'issue_status_changed':
      return 'Switch'
    case 'issue_commented':
      return 'ChatDotRound'
    case 'mention':
      return 'ChatLineSquare'
    case 'issue_updated':
      return 'Edit'
    default:
      return 'Bell'
  }
})

const iconColor = computed(() => {
  switch (props.notification.type) {
    case 'issue_assigned':
      return 'var(--td-color-primary)'
    case 'issue_status_changed':
      return 'var(--td-color-warning)'
    case 'issue_commented':
      return 'var(--td-color-success)'
    case 'mention':
      return 'var(--td-cat-2)'
    case 'issue_updated':
      return 'var(--td-cat-2)'
    default:
      return 'var(--td-text-placeholder)'
  }
})

const timeAgo = (dateStr: string): string => {
  const now = new Date()
  const date = new Date(dateStr)
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  const diffMin = Math.floor(diffSec / 60)
  const diffHour = Math.floor(diffMin / 60)
  const diffDay = Math.floor(diffHour / 24)

  if (diffSec < 60) return t('common.justNow')
  if (diffMin < 60) return t('common.minutesAgo', { n: diffMin })
  if (diffHour < 24) return t('common.hoursAgo', { n: diffHour })
  if (diffDay < 30) return t('common.daysAgo', { n: diffDay })
  return date.toLocaleDateString()
}
</script>

<style scoped>
.notification-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 16px;
  cursor: pointer;
  transition: background-color 150ms ease-out;
  border-bottom: 1px solid var(--td-divider-color);
  position: relative;
}

.notification-item:hover {
  background-color: var(--td-bg-page);
}

.notification-item.unread {
  background-color: var(--td-tag-primary-bg);
}

.notification-item.unread:hover {
  background-color: var(--td-tag-primary-bg);
}

.notif-icon {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--td-bg-page);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 2px;
}

.notif-body {
  flex: 1;
  min-width: 0;
}

.notif-title {
  font-size: 13px;
  color: var(--td-text-primary);
  line-height: 1.4;
  font-weight: 500;
}

.notif-content {
  font-size: 12px;
  color: var(--td-text-secondary);
  margin-top: 2px;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notif-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: 11px;
  color: var(--td-text-placeholder);
}

.notif-actor {
  color: var(--td-text-secondary);
}

.notif-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--td-color-primary);
  margin-top: 6px;
}
</style>
