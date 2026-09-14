<template>
  <section class="card">
    <el-tabs v-model="activeTab" class="detail-tabs">
      <el-tab-pane name="comments">
        <template #label>
          <span class="tab-label"><el-icon><ChatLineRound /></el-icon> {{ t('issue.detail.comments', { n: comments.length }) }}</span>
        </template>

        <!-- 添加评论 -->
        <div class="add-comment">
          <el-input
            v-model="newComment"
            type="textarea"
            :rows="3"
            :placeholder="t('issue.detail.commentPlaceholder')"
          />
          <div class="comment-actions">
            <el-button type="primary" :loading="commentLoading" :disabled="!newComment.trim()" @click="submitComment">
              <el-icon><ChatLineRound /></el-icon>
              {{ t('issue.detail.postComment') }}
            </el-button>
          </div>
        </div>

        <!-- 评论列表 -->
        <div class="comment-list">
          <div v-for="comment in comments" :key="comment.id" :class="['comment-item', { 'system-comment': comment.user_id === 0 }]">
            <div :class="['comment-avatar', { 'system-avatar': comment.user_id === 0 }]">
              {{ comment.user_id === 0 ? t('common.system').charAt(0) : (comment.user?.display_name?.charAt(0) || '?') }}
            </div>
            <div class="comment-body">
              <div class="comment-header">
                <span :class="['comment-author', { 'system-author': comment.user_id === 0 }]">{{ comment.user_id === 0 ? t('common.system') : (comment.user?.display_name || t('common.unknownUser')) }}</span>
                <span class="comment-time">{{ formatTime(comment.created_at) }}</span>
              </div>
              <!-- 系统评论的正文和活动详情一样，存的是结构化 key，渲染时才翻译；
                   人写的评论原样显示 -->
              <div class="comment-text">{{ comment.user_id === 0 ? formatActivityDetails(comment.content) : comment.content }}</div>
            </div>
          </div>
          <div v-if="comments.length === 0" class="empty-placeholder">
            {{ t('issue.detail.noComments') }}
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane name="worklogs">
        <template #label>
          <span class="tab-label"><el-icon><Clock /></el-icon> {{ t('issue.detail.worklogs', { n: worklogs.length }) }}</span>
        </template>

        <!-- 添加工作日志 -->
        <div class="add-worklog">
          <el-form :model="worklogForm" label-position="top" size="default">
            <el-form-item :label="t('issue.detail.worklogDesc')">
              <el-input
                v-model="worklogForm.description"
                type="textarea"
                :rows="3"
                :placeholder="t('issue.detail.worklogDescPlaceholder')"
              />
            </el-form-item>
            <el-row :gutter="16">
              <el-col :span="8">
                <el-form-item :label="t('issue.detail.timeSpent')">
                  <el-input v-model="worklogForm.time_spent" :placeholder="t('issue.detail.timeSpentPlaceholder')" />
                  <div class="form-hint">{{ t('issue.detail.timeFormatHint') }}</div>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item :label="t('issue.detail.workDate')">
                  <el-date-picker
                    v-model="worklogForm.worked_at"
                    type="datetime"
                    :placeholder="t('issue.detail.selectDateTime')"
                    style="width: 100%"
                  />
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item :label="t('issue.detail.workType')">
                  <el-select v-model="worklogForm.work_type" :placeholder="t('issue.detail.selectType')" style="width: 100%" clearable>
                    <el-option v-for="opt in workTypeOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>
            <el-button type="primary" :loading="worklogLoading" :disabled="!canSubmitWorklog" @click="submitWorklog">
              <el-icon><Plus /></el-icon>
              {{ t('issue.detail.addWorklog') }}
            </el-button>
          </el-form>
        </div>

        <!-- 工作日志列表 -->
        <div class="worklog-list">
          <div v-if="totalTimeSpent > 0" class="worklog-summary">
            <el-icon><Clock /></el-icon>
            <span>{{ t('issue.detail.totalTime', { time: formatTimeSpent(totalTimeSpent) }) }}</span>
          </div>
          <div v-for="worklog in worklogs" :key="worklog.id" class="worklog-item">
            <div class="worklog-avatar">
              {{ worklog.user?.display_name?.charAt(0) || '?' }}
            </div>
            <div class="worklog-body">
              <div class="worklog-header">
                <div class="worklog-meta">
                  <span class="worklog-author">{{ worklog.user?.display_name || t('common.unknownUser') }}</span>
                  <el-tag size="small" type="info">{{ worklog.time_spent }}</el-tag>
                  <el-tag v-if="worklog.work_type" size="small" type="success">{{ worklog.work_type }}</el-tag>
                </div>
                <div class="worklog-actions">
                  <span class="worklog-time">{{ formatTime(worklog.worked_at) }}</span>
                  <el-button v-if="canEditWorklog(worklog)" link type="primary" size="small" @click="handleEditWorklog(worklog)">
                    {{ t('common.edit') }}
                  </el-button>
                  <el-button v-if="canEditWorklog(worklog)" link type="danger" size="small" @click="handleDeleteWorklog(worklog.id)">
                    {{ t('common.delete') }}
                  </el-button>
                </div>
              </div>
              <div class="worklog-text">{{ worklog.description }}</div>
            </div>
          </div>
          <div v-if="worklogs.length === 0" class="empty-placeholder">
            {{ t('issue.detail.noWorklogs') }}
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>
  </section>
</template>

<script setup lang="ts">
/**
 * 工单的评论与工作日志面板。
 *
 * 从 IssueDetail.vue 拆出：这一块（模板 129 行 + 样式 180 行 + 提交/删除
 * 逻辑）自成一个功能单元，只和详情页共用 .empty-placeholder 一条样式。
 *
 * 数据仍由父组件加载并传入 —— 工时合计要喂给右侧「时间跟踪」，
 * 放在子组件里会让同一份数据有两个来源。这里只负责写入，
 * 写完发 changed 让父组件重新拉取。
 */
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ChatLineRound, Plus, Clock } from '@element-plus/icons-vue'
import { addIssueComment, addWorklog, deleteWorklog } from '@/api/issue'
import { useUserStore } from '@/stores/user'
import { formatTime, formatTimeSpent } from '../issue-display'
import { formatActivityDetails } from '@/utils/activity'
import type { IssueComment, Worklog } from '@/types/issue'

const { t } = useI18n()
const userStore = useUserStore()

const props = defineProps<{
  issueKey: string
  comments: IssueComment[]
  worklogs: Worklog[]
  workTypeOptions: { value: string; label: string }[]
  totalTimeSpent: number
}>()

const emit = defineEmits<{
  (e: 'comments-changed'): void
  (e: 'worklogs-changed'): void
}>()

const activeTab = ref('comments')

// ---- 评论 ----
const newComment = ref('')
const commentLoading = ref(false)

const submitComment = async () => {
  if (!newComment.value.trim()) return
  commentLoading.value = true
  try {
    await addIssueComment(props.issueKey, { content: newComment.value })
    ElMessage.success(t('issue.msg.commentSuccess'))
    newComment.value = ''
    emit('comments-changed')
  } catch { /* ignored */ }
  finally { commentLoading.value = false }
}

// ---- 工作日志 ----
const worklogLoading = ref(false)
const worklogForm = reactive({
  description: '',
  time_spent: '',
  worked_at: new Date().toISOString(),
  work_type: '',
})

const canSubmitWorklog = computed(() =>
  Boolean(worklogForm.description.trim() && worklogForm.time_spent.trim() && worklogForm.worked_at),
)

const canEditWorklog = (worklog: Worklog) => worklog.user_id === userStore.user?.id

const submitWorklog = async () => {
  if (!canSubmitWorklog.value) return
  worklogLoading.value = true
  try {
    const workedAtDate = new Date(worklogForm.worked_at)

    await addWorklog(props.issueKey, {
      description: worklogForm.description,
      time_spent: worklogForm.time_spent,
      worked_at: workedAtDate.toISOString(),
      work_type: worklogForm.work_type || undefined,
    })
    ElMessage.success(t('issue.msg.worklogAdded'))
    Object.assign(worklogForm, {
      description: '',
      time_spent: '',
      worked_at: new Date().toISOString(),
      work_type: '',
    })
    emit('worklogs-changed')
  } catch {
    ElMessage.error(t('issue.msg.worklogAddFailed'))
  } finally {
    worklogLoading.value = false
  }
}

const handleEditWorklog = (_worklog: Worklog) => {
  ElMessage.info(t('issue.msg.worklogEditTodo'))
}

const handleDeleteWorklog = async (worklogId: number) => {
  try {
    await ElMessageBox.confirm(t('issue.msg.confirmDeleteWorklog'), t('issue.msg.tipTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning',
    })
    await deleteWorklog(props.issueKey, worklogId)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    emit('worklogs-changed')
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('issue.msg.deleteFailed2'))
    }
  }
}
</script>

<style scoped lang="scss">
// 与详情页共用的卡片外观：子组件的根节点用了这个类，
// 父组件的 scoped 样式覆盖不到，必须在这里也有一份
.content-card {
  margin-bottom: 20px;
  border-radius: 12px;

  :deep(.el-card__header) {
    padding: 16px 20px;
    border-bottom: 1px solid var(--td-divider-color);
  }
}

.add-comment {
  padding: 20px;
  border-bottom: 1px solid var(--td-divider-color);

  .comment-actions {
    margin-top: 12px;
    text-align: right;
  }
}

.comment-list {
  .comment-item {
    display: flex;
    gap: 14px;
    padding: 16px 20px;
    border-bottom: 1px solid var(--td-divider-color);

    &:last-child { border-bottom: none; }

    .comment-avatar {
      width: 26px;
      height: 26px;
      border-radius: 50%;
      background: var(--td-tag-primary-bg);
      color: var(--td-tag-primary-text);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 11px;
      font-weight: 600;
      flex-shrink: 0;

      &.system-avatar {
        background: var(--td-bg-section);
        color: var(--td-text-secondary);
      }
    }

    .comment-body {
      flex: 1;

      .comment-header {
        display: flex;
        align-items: center;
        gap: 12px;
        margin-bottom: 8px;

        .comment-author { font-weight: 600; color: var(--td-text-primary); font-size: 14px; }
        .system-author { color: var(--td-tag-purple-text); }
        /* 时间走等宽 + tabular-nums：一列评论的时间戳要能纵向对齐（CLAUDE.md 3.2） */
        .comment-time {
          font-family: var(--td-font-mono);
          font-variant-numeric: tabular-nums;
          font-size: 12px;
          color: var(--td-text-placeholder);
        }
      }

      .comment-text {
        color: var(--td-text-regular);
        line-height: 1.6;
        white-space: pre-wrap;
        font-size: 14px;
      }
    }

    &.system-comment {
      background: var(--td-tag-purple-bg);
      border-radius: 8px;
      margin: 4px 0;
    }
  }
}

.detail-tabs {
  :deep(.el-tabs__header) {
    margin-bottom: 20px;
  }

  .tab-label {
    display: flex;
    align-items: center;
    gap: 6px;

    .el-icon { font-size: 16px; }
  }

  .tab-badge {
    margin-left: 4px;
  }
}

.add-worklog {
  padding: 20px;
  background: var(--td-bg-page);
  border-radius: 8px;
  margin-bottom: 20px;

  .form-hint {
    font-size: 12px;
    color: var(--td-text-placeholder);
    margin-top: 4px;
  }
}

.worklog-list {
  .worklog-summary {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px 16px;
    background: var(--td-tag-success-bg);
    border-radius: 8px;
    margin-bottom: 16px;
    font-size: 14px;
    font-weight: 500;
    color: var(--td-color-success);

    .el-icon { font-size: 16px; }
  }

  .worklog-item {
    display: flex;
    gap: 12px;
    padding: 16px 0;
    border-bottom: 1px solid var(--td-divider-color);

    &:last-child { border-bottom: none; }

    .worklog-avatar {
      width: 26px;
      height: 26px;
      border-radius: 50%;
      background: var(--td-tag-primary-bg);
      color: var(--td-tag-primary-text);
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 14px;
      font-weight: 600;
      flex-shrink: 0;
    }

    .worklog-body {
      flex: 1;
      min-width: 0;

      .worklog-header {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        margin-bottom: 8px;
        gap: 12px;

        .worklog-meta {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-wrap: wrap;

          .worklog-author {
            font-weight: 600;
            color: var(--td-text-primary);
            font-size: 14px;
          }
        }

        .worklog-actions {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-shrink: 0;

          .worklog-time {
            font-size: 13px;
            color: var(--td-text-placeholder);
          }
        }
      }

      .worklog-text {
        color: var(--td-text-regular);
        font-size: 14px;
        line-height: 1.6;
        white-space: pre-wrap;
        word-break: break-word;
      }
    }
  }
}

// 与详情页共用的空状态样式：这里只用到 sm 变体
.empty-placeholder {
  color: var(--td-text-placeholder);
  text-align: center;
  padding: var(--td-space-8) 0;
  font-size: 14px;

  &.sm { padding: var(--td-space-3) 0; }
}
</style>
