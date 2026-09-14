<template>
  <el-popover
    placement="bottom-end"
    :width="380"
    trigger="click"
    @show="onPopoverShow"
  >
    <template #reference>
      <div class="bell-wrapper">
        <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99">
          <el-icon :size="20" class="bell-icon"><Bell /></el-icon>
        </el-badge>
      </div>
    </template>

    <div class="notification-dropdown">
      <div class="dropdown-header">
        <span class="dropdown-title">{{ t('common.notification') }}</span>
        <el-link
          v-if="unreadCount > 0"
          type="primary"
          underline="never"
          @click="handleMarkAllAsRead"
        >
          {{ t('common.markAllRead') }}
        </el-link>
      </div>

      <el-scrollbar max-height="400px">
        <div v-if="recentNotifications.length === 0" class="empty-state">
          <el-icon :size="28" class="empty-icon"><BellFilled /></el-icon>
          <p>{{ t('common.noNotification') }}</p>
        </div>
        <NotificationItem
          v-for="item in recentNotifications"
          :key="item.id"
          :notification="item"
          @click="handleNotificationClick"
        />
      </el-scrollbar>

      <div class="dropdown-footer">
        <el-link type="primary" underline="never" @click="goToNotificationPage">
          {{ t('common.viewAllNotifications') }}
        </el-link>
      </div>
    </div>
  </el-popover>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { Bell, BellFilled } from '@element-plus/icons-vue'
import { useNotificationStore } from '@/stores/notification'
import NotificationItem from '@/components/NotificationItem.vue'
import type { NotificationItem as NotificationItemType } from '@/types/notification'

const { t } = useI18n()

const router = useRouter()
const notificationStore = useNotificationStore()

const unreadCount = computed(() => notificationStore.unreadCount)
const recentNotifications = computed(() => notificationStore.recentNotifications)

const onPopoverShow = () => {
  notificationStore.fetchNotifications()
}

const handleNotificationClick = (notification: NotificationItemType) => {
  // 标记为已读
  if (!notification.is_read) {
    notificationStore.markAsRead(notification.id)
  }
  // 跳转到对应实体
  if (notification.entity_type === 'issue' && notification.entity_key) {
    router.push(`/issues/${notification.entity_key}`)
  }
}

const handleMarkAllAsRead = () => {
  notificationStore.markAllAsRead()
}

const goToNotificationPage = () => {
  router.push('/notifications')
}
</script>

<style scoped>
.bell-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 6px;
  border-radius: 8px;
  transition: background-color 150ms ease-out;
}

.bell-wrapper:hover {
  background-color: var(--td-bg-page);
}

.bell-icon {
  color: var(--td-text-secondary);
}

.notification-dropdown {
  margin: -12px;
}

.dropdown-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--td-divider-color);
}

.dropdown-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--td-text-primary);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 36px 0;
  color: var(--td-text-placeholder);
}

/* 图标颜色从模板里的 color="#d1d5db" 挪到这里：写在属性上暗色模式跟不了 */
.empty-icon {
  color: var(--td-text-placeholder);
}

.empty-state p {
  margin-top: 8px;
  font-size: 13px;
}

.dropdown-footer {
  display: flex;
  justify-content: center;
  padding: 10px 16px;
  border-top: 1px solid var(--td-divider-color);
}
</style>
