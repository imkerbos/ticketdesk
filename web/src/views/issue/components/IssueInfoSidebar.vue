<template>
  <!--
  右侧原来是三张独立卡片（详细信息 / 时间跟踪 / 关注人）。
  后两张各只装一两个字段，却各带一套边框、圆角和卡片头 ——
  边框在这里表达的是"这是一个独立对象"，而它们其实是同一块侧栏的三段。
  合成一张卡，段与段之间用发丝线分隔。
-->
  <section class="card info-card">
    <div class="sidebar-section">
      <div class="sidebar-section__title">{{ t('issue.detail.info') }}</div>
      <div class="info-list">
        <div v-if="issue.parent_key" class="info-item">
          <span class="info-label">{{ t('issue.detail.parentIssue') }}</span>
          <el-link type="primary" @click="emit('navigate', issue.parent_key!)">
            {{ issue.parent_key }}
          </el-link>
        </div>
        <div v-if="issue.epic_key" class="info-item">
          <span class="info-label">Epic</span>
          <el-link type="primary" @click="emit('navigate', issue.epic_key!)">
            <el-icon><Link /></el-icon>
            {{ issue.epic_key }}{{ issue.epic_title ? ' - ' + issue.epic_title : '' }}
          </el-link>
        </div>
        <div v-if="issue.merged_into_issue_key" class="info-item">
          <span class="info-label">{{ t('issue.detail.mergedInto') }}</span>
          <el-link type="primary" @click="emit('navigate', issue.merged_into_issue_key!)">
            <el-icon><Link /></el-icon>
            {{ issue.merged_into_issue_key }}
          </el-link>
        </div>
        <div v-if="issue.merged_from_issue_keys?.length" class="info-item">
          <span class="info-label">{{ t('issue.detail.mergedFrom') }}</span>
          <div class="merged-from-links">
            <el-link
              v-for="mKey in issue.merged_from_issue_keys"
              :key="mKey"
              type="primary"
              style="margin-right: 8px;"
              @click="emit('navigate', mKey)"
            >
              {{ mKey }}
            </el-link>
          </div>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.status') }}</span>
          <div class="status-badge sm" :class="issue.status">
            <span class="status-dot"></span>
            <span>{{ getStatusText(issue.status) }}</span>
          </div>
        </div>
        <div v-if="slaStatus" class="info-item">
          <span class="info-label">{{ t('issue.detail.slaStatus') }}</span>
          <div class="sla-status-wrap">
            <el-tag v-if="slaStatus.level === 'overdue'" type="danger" size="small" effect="dark">{{ t('issue.detail.overdue') }}</el-tag>
            <el-tag v-else-if="slaStatus.level === 'due_soon'" type="warning" size="small" effect="dark">{{ t('issue.detail.dueSoon') }}</el-tag>
            <el-tag v-else type="success" size="small" effect="dark">{{ t('issue.detail.inProgress') }}</el-tag>
            <span class="sla-hint">{{ slaStatus.hint }}</span>
          </div>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.priority') }}</span>
          <el-tag :type="getPriorityType(issue.priority)" size="small" effect="dark">{{ issue.priority }}</el-tag>
        </div>
        <div v-if="issue.resolution" class="info-item">
          <span class="info-label">{{ t('issue.detail.resolution') }}</span>
          <el-tag size="small" type="success">{{ getResolutionText(issue.resolution) }}</el-tag>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.project') }}</span>
          <el-link type="primary" @click="embedded ? router.push(`/projects/${issue.project_key}`) : router.push(`/issues?project_key=${issue.project_key}`)">
            {{ issue.project_key }}
          </el-link>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.type') }}</span>
          <span>{{ issue.issue_type?.display_name || '-' }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.assignee') }}</span>
          <div class="assignee-with-action">
            <template v-if="!editingAssignee">
              <template v-if="issue.assignee">
                <div class="user-info">
                  <div class="mini-avatar">{{ issue.assignee.display_name?.charAt(0) || '?' }}</div>
                  <span>{{ issue.assignee.display_name }}</span>
                </div>
              </template>
              <span v-else class="text-muted">{{ t('common.unassigned') }}</span>
              <el-button
                v-if="issue.assignee?.id !== userStore.user?.id"
                link
                type="primary"
                size="small"
                @click="emit('assign-to-me')"
              >
                {{ t('issue.detail.assignToMe') }}
              </el-button>
              <el-button
                v-if="userStore.isProjectAdmin"
                link
                size="small"
                @click="emit('start-edit-assignee')"
              >
                <el-icon><Edit /></el-icon>
              </el-button>
            </template>
            <template v-else>
              <el-select
                v-model="editAssigneeId"
                :placeholder="t('issue.detail.selectAssignee')"
                filterable
                clearable
                size="small"
                style="width: 160px;"
                @change="(v: any) => emit('assignee-change', v)"
              >
                <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id" />
              </el-select>
              <el-button link size="small" @click="editingAssignee = false">{{ t('common.cancel') }}</el-button>
            </template>
          </div>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.detail.creator') }}</span>
          <span>{{ issue.reporter?.display_name || t('common.unknown') }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('common.createdAt') }}</span>
          <span class="time">{{ formatTime(issue.created_at) }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('common.updatedAt') }}</span>
          <span class="time">{{ formatTime(issue.updated_at) }}</span>
        </div>
        <div v-if="issue.due_date" class="info-item">
          <span class="info-label">{{ t('issue.detail.dueDate') }}</span>
          <span class="time">{{ formatDate(issue.due_date) }}</span>
          <el-tag v-if="dueDateStatus === 'overdue'" type="danger" size="small" style="margin-left: 6px;">{{ t('issue.detail.overdue') }}</el-tag>
          <el-tag v-else-if="dueDateStatus === 'due_soon'" type="warning" size="small" style="margin-left: 6px;">{{ t('issue.detail.dueSoon') }}</el-tag>
        </div>
        <div v-if="issue.planned_start_date" class="info-item">
          <span class="info-label">{{ t('issue.detail.plannedStart') }}</span>
          <span class="time">{{ formatDate(issue.planned_start_date) }}</span>
        </div>
        <div v-if="issue.planned_end_date" class="info-item">
          <span class="info-label">{{ t('issue.detail.plannedEnd') }}</span>
          <span class="time">{{ formatDate(issue.planned_end_date) }}</span>
        </div>
        <div v-if="issue.actual_start_date" class="info-item">
          <span class="info-label">{{ t('issue.detail.actualStart') }}</span>
          <span class="time">{{ formatTime(issue.actual_start_date) }}</span>
        </div>
        <div v-if="issue.actual_end_date" class="info-item">
          <span class="info-label">{{ t('issue.detail.actualEnd') }}</span>
          <span class="time">{{ formatTime(issue.actual_end_date) }}</span>
        </div>
      </div>
    </div>

    <div v-if="showTimeTracking" class="sidebar-section">
      <div class="sidebar-section__title">{{ t('issue.detail.timeTracking') }}</div>
      <div class="info-list">
        <div v-if="estimatedTimeSec > 0" class="time-progress-wrap">
          <el-progress
            :percentage="timeProgress"
            :color="remainingTimeSec < 0 ? 'var(--td-color-danger)' : 'var(--td-color-primary)'"
            :stroke-width="10"
          />
        </div>
        <div v-if="estimatedTimeSec > 0" class="info-item">
          <span class="info-label">{{ t('issue.detail.estimated') }}</span>
          <span>{{ formatTimeSpent(estimatedTimeSec) }}</span>
        </div>
        <div class="info-item">
          <span class="info-label">{{ t('issue.detail.spent') }}</span>
          <span>{{ formatTimeSpent(totalTimeSpent) }}</span>
        </div>
        <div v-if="estimatedTimeSec > 0" class="info-item">
          <span class="info-label">{{ t('issue.detail.remaining') }}</span>
          <span :style="{ color: remainingTimeSec < 0 ? 'var(--td-color-danger)' : undefined }">
            {{ remainingTimeSec < 0 ? t('issue.detail.over', { time: formatTimeSpent(Math.abs(remainingTimeSec)) }) : formatTimeSpent(remainingTimeSec) }}
          </span>
        </div>
      </div>
    </div>

    <div class="sidebar-section">
      <div class="card-header-with-action">
        <div class="sidebar-section__title">
          {{ t('issue.detail.watchers') }}
          <span class="card-count">{{ watchers.length }}</span>
        </div>
        <div class="watcher-header-actions">
          <el-button v-if="!isWatching" link type="primary" size="small" @click="emit('watch')">
            <el-icon><View /></el-icon>
            {{ t('issue.detail.watch') }}
          </el-button>
          <el-button v-else link type="danger" size="small" @click="emit('unwatch')">
            {{ t('issue.detail.unwatch') }}
          </el-button>
          <el-button link type="primary" size="small" @click="emit('add-watcher')">
            <el-icon><Plus /></el-icon>
          </el-button>
        </div>
      </div>
      <div class="watcher-list">
        <div v-for="watcher in watchers" :key="watcher.id" class="watcher-item">
          <div class="watcher-info">
            <div class="mini-avatar">{{ watcher.user?.display_name?.charAt(0) || '?' }}</div>
            <span class="watcher-name">{{ watcher.user?.display_name || t('common.unknownUser') }}</span>
          </div>
          <el-button
            v-if="canRemoveWatcher(watcher)"
            link
            type="danger"
            size="small"
            @click="emit('remove-watcher', watcher.user_id)"
          >
            {{ t('common.remove') }}
          </el-button>
        </div>
        <div v-if="watchers.length === 0" class="empty-placeholder sm">
          {{ t('issue.detail.noWatchers') }}
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * 工单详情右侧信息栏：详细信息 / 时间跟踪 / 关注人三段。
 *
 * 只负责展示与交互意图，所有写操作（改指派人、关注、移除关注）
 * 仍由父组件执行 —— 它们都要在成功后重新拉取工单或关注人列表，
 * 放在子组件里会让同一份数据有两个写入方。
 *
 * 指派人的编辑态用 defineModel 双向绑回父组件：父组件在打开编辑前
 * 要先懒加载用户列表，状态留在那边流程最短。
 */
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Link, Edit, View, Plus } from '@element-plus/icons-vue'
import { useUserStore } from '@/stores/user'
import { getStatusText, getPriorityType, getResolutionText, formatTime, formatDate, formatTimeSpent } from '../issue-display'
import type { Issue, IssueWatcher } from '@/types/issue'
import type { UserOption } from '@/types/user'

const { t } = useI18n()
const router = useRouter()
const userStore = useUserStore()

defineProps<{
  issue: Issue
  watchers: IssueWatcher[]
  users: UserOption[]
  isWatching: boolean
  showTimeTracking: boolean
  estimatedTimeSec: number
  totalTimeSpent: number
  timeProgress: number
  remainingTimeSec: number
  slaStatus: { level: 'overdue' | 'due_soon' | 'normal'; hint: string } | null
  dueDateStatus: 'overdue' | 'due_soon' | 'normal' | null
  embedded?: boolean
}>()

const emit = defineEmits<{
  (e: 'navigate', key: string): void
  (e: 'assign-to-me'): void
  (e: 'start-edit-assignee'): void
  (e: 'assignee-change', userId: number | undefined): void
  (e: 'watch'): void
  (e: 'unwatch'): void
  (e: 'add-watcher'): void
  (e: 'remove-watcher', userId: number): void
}>()

const editingAssignee = defineModel<boolean>('editingAssignee', { required: true })
const editAssigneeId = defineModel<number | undefined>('editAssigneeId', { required: true })

/** 只能移除自己 */
const canRemoveWatcher = (watcher: IssueWatcher) => watcher.user_id === userStore.user?.id
</script>

<style scoped lang="scss">
@use '../issue-detail-shared.scss' as *;

.time-progress-wrap {
  padding: 12px 20px 4px;
}

.info-list {
  // 右栏是"属性清单"，不是"表格"。原来每行 12px 上下内边距 + 一条实线分隔，
  // 十来行下来近 600px，且每行都被线切开，读起来像一张表。
  // 改为紧凑行距 + 去掉行间分隔线，靠标签/值的对齐关系成组。
  .info-item {
    display: grid;
    grid-template-columns: 76px 1fr;
    align-items: baseline;
    gap: var(--td-space-3);
    padding: var(--td-space-2) var(--td-space-4);
    border-bottom: none;

    &:last-child { border-bottom: none; }

    // 值为空时整行压得更紧，避免"未设置"占据与真实内容同样的重量
    &:hover { background: var(--td-bg-card-hover); }

    .info-label {
      color: var(--td-text-placeholder);
      font-size: var(--td-font-sm);
      font-weight: var(--td-weight-regular);
      text-align: left;
      line-height: var(--td-leading-normal);
    }

    .user-info {
      display: flex;
      align-items: center;
      gap: 8px;
      color: var(--td-text-primary);
      font-size: 13px;
    }

    .assignee-with-action {
      display: flex;
      align-items: center;
      gap: 8px;
      justify-content: space-between;
      width: 100%;
    }

    > span:not(.info-label),
    > .el-tag,
    > .el-link,
    > .status-badge {
      color: var(--td-text-primary);
      font-size: 13px;
      justify-self: start;
    }

    .sla-status-wrap {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: nowrap;
      min-width: 0;

      .sla-hint {
        font-size: 12px;
        color: var(--td-text-placeholder);
        white-space: nowrap;
      }
    }
  }
}

.watcher-list {
  .watcher-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    padding: 10px 20px;
    border-bottom: 1px solid var(--td-divider-color);

    &:last-child { border-bottom: none; }

    .watcher-info {
      display: flex;
      align-items: center;
      gap: 10px;
      flex: 1;
    }

    .watcher-name { font-size: 14px; color: var(--td-text-regular); }
  }
}

.watcher-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 合并来源可能有多个，换行排开而不是撑破一行 */
.merged-from-links {
  display: flex;
  flex-wrap: wrap;
  gap: 2px 0;
  min-width: 0;
}
</style>
