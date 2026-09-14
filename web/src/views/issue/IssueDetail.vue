<template>
  <div v-loading="loading" class="issue-detail-container" :class="{ 'is-embedded': embedded }">
    <template v-if="issue">
      <!-- 头部信息 -->
      <div class="issue-header">
        <div class="header-left">
          <div v-if="!embedded" class="issue-breadcrumb">
            <el-breadcrumb separator="/">
              <el-breadcrumb-item :to="{ path: '/issues' }">{{ t('issue.listTitle') }}</el-breadcrumb-item>
              <el-breadcrumb-item :to="{ path: '/issues?project_key=' + issue.project_key }">{{ issue.project_key }}</el-breadcrumb-item>
              <el-breadcrumb-item>{{ issue.issue_key }}</el-breadcrumb-item>
            </el-breadcrumb>
          </div>
          <div v-if="embedded" class="embedded-key-row">
            <a class="embedded-key-link" @click.prevent="router.push(`/issues/${issue.issue_key}`)">
              {{ issue.issue_key }}
              <el-icon class="open-icon"><TopRight /></el-icon>
            </a>
          </div>
          <h1 v-if="!embedded" class="issue-title">{{ issue.title }}</h1>
          <a v-else class="issue-title issue-title-link" @click.prevent="router.push(`/issues/${issue.issue_key}`)">{{ issue.title }}</a>
          <div class="issue-meta">
            <div v-if="issue.issue_type" class="type-badge">
              <el-icon class="type-icon"><Document /></el-icon>
              <span class="type-text">{{ issue.issue_type.display_name }}</span>
            </div>
            <span class="prio">
              <span class="dot" :style="{ background: priorityColor(issue.priority) }"></span>{{ issue.priority }}
            </span>
            <span class="pill" :class="statusTone(issue.status)">{{ getStatusText(issue.status) }}</span>
            <!--
              创建人与创建时间只在右侧「详细信息」里出现一次。
              页头留给扫读最需要的三项：类型、优先级、状态。
            -->
          </div>
        </div>
        <div class="header-actions">
          <!-- 工作流快捷操作按钮 -->
          <el-dropdown v-if="workflowInstance" trigger="hover" @command="handleWorkflowCommand">
            <!--
              这颗按钮是工作流下拉菜单的入口，和下面工作流卡片里的
              「流转至 XXX」是同一个动作的两个入口。实心填充留给那颗真正
              提交的按钮，这里用默认样式，一屏就只剩一个主操作。
            -->
            <el-button
              :disabled="!canOperateWorkflow && !isWorkflowOperable"
              class="workflow-action-btn"
            >
              <el-icon><Promotion /></el-icon>
              {{ workflowActionBtnText }}
              <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <!-- 审批节点操作 -->
                <template v-if="workflowInstance.current_node?.node_type === 'approval' && isWorkflowOperable">
                  <el-dropdown-item
                    command="approve"
                    :disabled="!isCurrentUserApprover"
                  >
                    <el-icon style="color: var(--td-color-success)"><Check /></el-icon>
                    {{ t('issue.detail.approveAction') }}
                  </el-dropdown-item>
                  <el-dropdown-item
                    command="reject"
                    :disabled="!isCurrentUserApprover"
                  >
                    <el-icon style="color: var(--td-color-danger)"><Delete /></el-icon>
                    {{ t('issue.detail.rejectAction') }}
                  </el-dropdown-item>
                </template>
                <!-- 工作节点操作 -->
                <template v-else-if="isWorkNode && isWorkflowOperable">
                  <!-- 有条件分支的工作节点：动态生成操作按钮 -->
                  <template v-if="workNodeHasBranching">
                    <el-dropdown-item
                      v-for="action in workNodeOutgoingActions"
                      :key="action.conditionExpr"
                      :command="`complete-condition:${action.conditionExpr}`"
                    >
                      <el-icon :style="{ color: action.conditionExpr === 'rejected' ? 'var(--td-color-danger)' : 'var(--td-color-success)' }">
                        <component :is="action.conditionExpr === 'rejected' ? Delete : Check" />
                      </el-icon>
                      {{ action.label }}{{ action.targetNodeName ? ` → ${action.targetNodeName}` : '' }}
                    </el-dropdown-item>
                  </template>
                  <!-- 无分支的工作节点：显示完成 -->
                  <template v-else>
                    <el-dropdown-item command="complete">
                      <el-icon style="color: var(--td-color-primary)"><Check /></el-icon>
                      {{ nextNodeName ? t('issue.detail.transitTo', { name: nextNodeName }) : t('issue.detail.completeNode') }}
                    </el-dropdown-item>
                  </template>
                </template>
                <!-- 工作流可操作但节点信息缺失（如 reviewing 状态且节点被重建） -->
                <template v-else-if="isWorkflowOperable">
                  <el-dropdown-item command="complete">
                    <el-icon style="color: var(--td-color-primary)"><Check /></el-icon>
                    {{ t('issue.detail.confirmComplete') }}
                  </el-dropdown-item>
                </template>
                <!-- 工作流已结束提示 -->
                <template v-else>
                  <el-dropdown-item disabled>
                    {{ t('issue.detail.workflowState', { state: getWorkflowStatusText(workflowInstance.status) }) }}
                  </el-dropdown-item>
                </template>
                <el-dropdown-item divided command="view-workflow">
                  <el-icon style="color: var(--td-color-info)"><View /></el-icon>
                  {{ t('issue.detail.viewWorkflow') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <button class="btn secondary" @click="handleEdit">{{ t('common.edit') }}</button>
          <!-- 删除收进 ··· 菜单：页头一进来就摆一个删除按钮，既是噪音也提高误触 -->
          <el-dropdown trigger="click">
            <button class="more" :aria-label="t('common.operation')">···</button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="handleDelete">{{ t('common.delete') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>

      <el-row :gutter="20">
        <!-- 左侧：描述和评论 -->
        <el-col :xs="24" :lg="16">
          <!-- 描述 -->
          <section class="card">
            <div class="card-head">
              <h2>{{ t('issue.description') }}</h2>
            </div>
            <div v-if="issue.description" class="description-content">
              {{ issue.description }}
            </div>
            <div v-else class="empty-placeholder">{{ t('issue.detail.noDescription') }}</div>
          </section>

          <!-- Epic 下的 Issues（仅当当前工单是 Epic 类型时显示）-->
          <section v-if="issue.issue_type?.name?.toLowerCase() === 'epic'" class="card epic-issues-card">
            <div class="card-head">
              <h2>{{ t('issue.detail.epicIssues', { n: epicIssues.length }) }}</h2>
            </div>
            <div class="epic-issues-list">
              <div v-if="epicIssues.length === 0" class="empty-state">
                <TdEmptyState preset="no-data" :title="t('issue.detail.noEpicIssues')" />
              </div>
              <div v-for="epicIssue in epicIssues" :key="epicIssue.id" class="epic-issue-item">
                <div class="issue-left">
                  <div class="issue-type-icon" :class="epicIssue.issue_type?.name?.toLowerCase() || 'task'">
                    <el-icon><Document /></el-icon>
                  </div>
                  <router-link :to="`/issues/${epicIssue.issue_key}`" class="issue-link">
                    <span class="issue-key">{{ epicIssue.issue_key }}</span>
                  </router-link>
                  <div class="issue-title">{{ epicIssue.title }}</div>
                </div>
                <div class="issue-right">
                  <el-tag :type="getPriorityType(epicIssue.priority)" size="small" effect="dark" class="priority-tag">
                    {{ epicIssue.priority }}
                  </el-tag>
                  <div class="status-badge" :class="epicIssue.status">
                    <span class="status-dot"></span>
                    <span>{{ getStatusText(epicIssue.status) }}</span>
                  </div>
                  <div v-if="epicIssue.assignee" class="assignee-info">
                    <div class="assignee-avatar" :title="epicIssue.assignee.display_name">
                      {{ epicIssue.assignee.display_name?.charAt(0) || '?' }}
                    </div>
                    <span class="assignee-name">{{ epicIssue.assignee.display_name }}</span>
                  </div>
                  <div v-else class="assignee-info">
                    <div class="assignee-avatar unassigned" :title="t('common.unassigned')">?</div>
                    <span class="assignee-name unassigned">{{ t('common.unassigned') }}</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <!-- 工作流卡片 -->
          <section v-if="workflowInstance" class="card workflow-card">
            <div class="card-head">
              <h2>{{ t('issue.detail.workflow') }}</h2>
              <span class="pill" :class="workflowTone(workflowInstance.status)">
                {{ getWorkflowStatusText(workflowInstance.status) }}
              </span>
              <el-button
                v-if="(workflowInstance.approvals && workflowInstance.approvals.length > 0) || workflowHistoryList.length > 0"
                link
                size="small"
                style="margin-left: auto;"
                @click.stop="workflowExpanded = !workflowExpanded"
              >
                {{ workflowExpanded ? t('issue.detail.collapse') : t('issue.detail.expand') }}
                <el-icon><component :is="workflowExpanded ? ArrowUp : ArrowDown" /></el-icon>
              </el-button>
            </div>

            <!-- 当前节点信息 -->
            <div class="workflow-current-node">
              <div class="current-node-label">{{ t('issue.detail.currentNode') }}</div>
              <div class="current-node-info">
                <el-tag size="default" effect="plain">
                  {{ workflowInstance.current_node?.name || t('issue.detail.nodeFallback', { id: workflowInstance.current_node_id }) }}
                </el-tag>
                <el-tag v-if="workflowInstance.current_node?.node_type" size="small" type="info">
                  {{ workflowInstance.current_node.node_type === 'approval' ? t('issue.detail.approvalNode') : workflowInstance.current_node.node_type === 'work' ? t('issue.detail.workNode') : workflowInstance.current_node.node_type }}
                </el-tag>
              </div>
            </div>

            <!-- 审批记录（默认折叠） -->
            <div v-show="workflowExpanded" v-if="workflowInstance.approvals && workflowInstance.approvals.length > 0" class="workflow-approvals">
              <div class="approvals-label">{{ t('issue.detail.approvalRecords') }}</div>
              <div class="approvals-list">
                <div v-for="approval in workflowInstance.approvals" :key="approval.id" class="approval-item">
                  <div class="approval-user">
                    <div class="mini-avatar">{{ approval.approver_name?.charAt(0) || '?' }}</div>
                    <span>{{ approval.approver_name || t('issue.detail.userFallback', { id: approval.approver_id }) }}</span>
                  </div>
                  <el-tag :type="getApprovalStatusType(approval.status)" size="small">
                    {{ getApprovalStatusText(approval.status) }}
                  </el-tag>
                  <span v-if="approval.comment" class="approval-comment">{{ approval.comment }}</span>
                </div>
              </div>
            </div>

            <!-- 审批操作按钮（仅审批节点显示） -->
            <div v-if="isCurrentUserApprover && isWorkflowOperable && workflowInstance.current_node?.node_type === 'approval'" class="workflow-actions">
              <div class="actions-label">{{ t('issue.detail.approvalActions') }}</div>
              <div class="actions-row">
                <el-input v-model="approveComment" :placeholder="t('issue.detail.approvalCommentPlaceholder')" size="default" style="flex: 1; margin-right: 12px;" />
                <!-- 一组里只有「通过」是主操作，实心留给它；「拒绝」是破坏性的那一个，
                     不常驻红（CLAUDE.md 3.1），中性描边，悬停才亮出红色。
                     图标也从垃圾桶换成叉 —— 拒绝不是删除。 -->
                <el-button type="primary" :loading="approveLoading" @click="handleApprove">
                  <el-icon><Check /></el-icon>
                  {{ t('issue.detail.approve') }}
                </el-button>
                <el-button class="reject-btn" @click="showRejectDialog">
                  <el-icon><Close /></el-icon>
                  {{ t('issue.detail.reject') }}
                </el-button>
              </div>
            </div>

            <!-- 工作节点完成按钮 -->
            <div v-if="isWorkNode" class="workflow-actions">
              <div class="actions-label">{{ workNodeHasBranching || !nextNodeName ? t('issue.detail.nodeActions') : t('issue.detail.transitActions') }}</div>
              <div class="actions-row">
                <el-input v-model="completeComment" :placeholder="t('issue.detail.remarkPlaceholder')" size="default" style="flex: 1; margin-right: 12px;" />
                <!-- 有条件分支的工作节点：动态生成操作按钮 -->
                <template v-if="workNodeHasBranching">
                  <el-button
                    v-for="action in workNodeOutgoingActions"
                    :key="action.conditionExpr"
                    :type="action.conditionExpr === 'rejected' ? 'warning' : 'success'"
                    :loading="completeLoading"
                    @click="handleCompleteWithResult(action.conditionExpr)"
                  >
                    <el-icon><component :is="action.conditionExpr === 'rejected' ? Delete : Check" /></el-icon>
                    {{ action.label }}{{ action.targetNodeName ? ` → ${action.targetNodeName}` : '' }}
                  </el-button>
                </template>
                <!-- 无分支的工作节点：显示完成按钮 -->
                <template v-else>
                  <el-button type="primary" :loading="completeLoading" @click="handleComplete">
                    <el-icon><Check /></el-icon>
                    {{ nextNodeName ? t('issue.detail.transitTo', { name: nextNodeName }) : t('issue.detail.completeNode') }}
                  </el-button>
                </template>
              </div>
            </div>

            <!-- 流转历史时间线（默认折叠） -->
            <div v-show="workflowExpanded" v-if="workflowHistoryList.length > 0" class="workflow-history">
              <div class="history-label">{{ t('issue.detail.transitHistory') }}</div>
              <el-timeline class="workflow-timeline">
                <el-timeline-item
                  v-for="item in workflowHistoryList"
                  :key="item.id"
                  :timestamp="formatTime(item.operated_at || item.created_at)"
                  placement="top"
                  :type="item.action === 'reject' ? 'danger' : item.action === 'complete' ? 'success' : 'primary'"
                >
                  <div class="history-content">
                    <span class="history-user">{{ item.operator_name || (item.operator_id === 0 ? t('common.system') : t('issue.detail.userFallback', { id: item.operator_id })) }}</span>
                    <span class="history-action">{{ getHistoryActionText(item.action) }}</span>
                    <template v-if="item.to_node">
                      <span class="history-arrow">→</span>
                      <el-tag size="small">{{ item.to_node.name }}</el-tag>
                    </template>
                    <div v-if="item.comment" class="history-comment">{{ item.comment }}</div>
                  </div>
                </el-timeline-item>
              </el-timeline>
            </div>
          </section>

          <!-- 扩展字段 - 显示在主体区域 -->
          <section v-if="customFields.length > 0" class="card custom-fields-card">
            <div class="card-head">
              <h2>{{ t('issue.detail.customFields') }}</h2>
            </div>
            <div class="custom-fields-grid">
              <div v-for="field in customFields" :key="field.field_id" class="field-item">
                <div class="field-label">{{ field.field_name }}</div>
                <div class="field-value">
                  <template v-if="isFieldValueSet(field)">
                    <!-- Epic Link 特殊显示 -->
                    <template v-if="field.field_type === 'epic_link' && typeof field.value === 'object' && field.value.issue_key">
                      <router-link :to="`/issues/${field.value.issue_key}`" class="epic-link">
                        <el-icon><Link /></el-icon>
                        {{ field.value.issue_key }}: {{ field.value.title }}
                      </router-link>
                    </template>
                    <!-- 多选类型：标签展示 -->
                    <template v-else-if="Array.isArray(field.value)">
                      <el-tag v-for="item in field.value" :key="item" size="small" style="margin-right: 4px;">{{ item }}</el-tag>
                    </template>
                    <!-- 其他字段 -->
                    <template v-else>
                      {{ field.display_value || field.value }}
                    </template>
                  </template>
                  <span v-else class="empty-value">{{ t('common.unset') }}</span>
                </div>
              </div>
            </div>
          </section>

          <!-- 子任务列表 -->
          <section v-if="subtasks.length > 0 || issue.issue_type?.name?.toLowerCase() === 'task'" class="card subtasks-card">
            <div class="card-head">
              <h2>{{ t('issue.detail.subtasks', { n: subtasks.length }) }}</h2>
              <div class="grow"></div>
              <el-button link type="primary" size="small" @click="handleCreateSubtask">
                <el-icon><Plus /></el-icon>
                {{ t('issue.detail.createSubtask') }}
              </el-button>
            </div>
            <div class="subtasks-list">
              <div v-if="subtasks.length === 0" class="empty-state-compact">
                <span class="empty-text">{{ t('issue.detail.noSubtasks') }}</span>
              </div>
              <div v-for="subtask in subtasks" :key="subtask.id" class="subtask-item">
                <div class="issue-left">
                  <div class="issue-type-icon" :class="subtask.issue_type?.name?.toLowerCase() || 'task'">
                    <el-icon><Document /></el-icon>
                  </div>
                  <router-link :to="`/issues/${subtask.issue_key}`" class="issue-link">
                    <span class="issue-key">{{ subtask.issue_key }}</span>
                  </router-link>
                  <div class="issue-title">{{ subtask.title }}</div>
                </div>
                <div class="issue-right">
                  <el-tag :type="getPriorityType(subtask.priority)" size="small" effect="dark" class="priority-tag">
                    {{ subtask.priority }}
                  </el-tag>
                  <div class="status-badge" :class="subtask.status">
                    <span class="status-dot"></span>
                    <span>{{ getStatusText(subtask.status) }}</span>
                  </div>
                  <div v-if="subtask.assignee" class="assignee-info">
                    <div class="assignee-avatar" :title="subtask.assignee.display_name">
                      {{ subtask.assignee.display_name?.charAt(0) || '?' }}
                    </div>
                    <span class="assignee-name">{{ subtask.assignee.display_name }}</span>
                  </div>
                  <div v-else class="assignee-info">
                    <div class="assignee-avatar unassigned" :title="t('common.unassigned')">?</div>
                    <span class="assignee-name unassigned">{{ t('common.unassigned') }}</span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <!-- 关联告警 -->
          <section v-if="issueAlerts.length > 0" class="card alert-card">
            <div class="card-head">
              <h2>{{ t('issue.detail.linkedAlerts', { n: issueAlerts.length }) }}</h2>
              <div class="grow"></div>
              <el-button link type="primary" size="small" @click="$router.push(`/alerts?issue_id=${issue!.id}`)">
                {{ t('issue.detail.viewInAlertList') }}
              </el-button>
            </div>
            <el-table :data="issueAlerts" style="width: 100%" size="small" :row-class-name="() => 'clickable-row'" @row-click="(row: Alert) => $router.push(`/alerts/${row.id}`)">
              <el-table-column prop="alert_name" :label="t('alert.name')" min-width="180">
                <template #default="{ row }">
                  <span class="alert-name-text">{{ row.alert_name }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="severity" :label="t('alert.severity')" width="90" align="center">
                <template #default="{ row }">
                  <el-tag :type="getAlertSeverityType(row.severity)" size="small" effect="dark">
                    {{ getAlertSeverityText(row.severity) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" :label="t('issue.status')" width="90" align="center">
                <template #default="{ row }">
                  <div class="alert-status-badge" :class="row.status">
                    <span class="status-dot"></span>
                    <span>{{ getAlertStatusText(row.status) }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="t('alert.instance')" min-width="140">
                <template #default="{ row }">
                  <span class="text-muted">{{ row.labels?.instance || row.labels?.target_ident || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="starts_at" :label="t('alert.startsAt')" width="150">
                <template #default="{ row }">
                  <span class="text-muted">{{ formatTime(row.starts_at) }}</span>
                </template>
              </el-table-column>
            </el-table>
          </section>

          <!-- 附件 -->
          <section class="card attachment-card">
            <div class="card-head">
              <h2>{{ t('issue.detail.attachments') }}</h2>
              <span class="n">{{ attachments.length }}</span>
              <div class="grow"></div>
              <el-button v-if="!showAttachmentUpload && attachments.length === 0" link type="primary" size="small" @click="showAttachmentUpload = true">
                <el-icon><Plus /></el-icon>
                {{ t('issue.detail.uploadAttachment') }}
              </el-button>
            </div>
            <div class="attachment-section">
              <AttachmentUpload v-if="issue && (showAttachmentUpload || attachments.length > 0)" :issue-key="issue.issue_key" @success="loadAttachments(issue.issue_key)" />
              <AttachmentList v-if="issue && attachments.length > 0" :issue-key="issue.issue_key" :attachments="attachments" @refresh="loadAttachments(issue.issue_key)" />
              <div v-if="attachments.length === 0 && !showAttachmentUpload" class="empty-placeholder sm">{{ t('issue.detail.noAttachments') }}</div>
            </div>
          </section>

          <IssueActivityPanel
            :issue-key="issue.issue_key"
            :comments="comments"
            :worklogs="worklogs"
            :work-type-options="workTypeOptions"
            :total-time-spent="totalTimeSpent"
            @comments-changed="loadComments(issue!.issue_key)"
            @worklogs-changed="loadWorklogs(issue!.issue_key)"
          />

          <!-- 评论和工作日志 -->
          <section class="card">
            <div class="card-head">
              <h2>{{ t('issue.detail.activity') }}</h2>
            </div>
            <el-timeline v-if="activities.length > 0" class="activity-timeline">
              <el-timeline-item
                v-for="activity in activities"
                :key="activity.id"
                :timestamp="formatTime(activity.created_at)"
                placement="top"
              >
                <div class="activity-content">
                  <span class="activity-user">{{ activity.user_name }}</span>
                  <template v-if="activity.details">
                    <span class="activity-details">{{ formatActivityDetails(activity.details) }}</span>
                  </template>
                  <template v-else>
                    <span class="activity-action">{{ formatActivityAction(activity.action) }}</span>
                    <template v-if="activity.field">
                      <span class="activity-field">{{ activity.field }}</span>
                      <span v-if="activity.old_value" class="activity-old-value">{{ activity.old_value }}</span>
                      <el-icon v-if="activity.old_value"><ArrowRight /></el-icon>
                      <span v-if="activity.new_value" class="activity-new-value">{{ activity.new_value }}</span>
                    </template>
                  </template>
                </div>
              </el-timeline-item>
            </el-timeline>
            <div v-else class="empty-placeholder">{{ t('issue.detail.noActivity') }}</div>
          </section>
        </el-col>

        <!-- 右侧：详细信息 -->
        <el-col :xs="24" :lg="8">
          <IssueInfoSidebar
            v-model:editing-assignee="editingAssignee"
            v-model:edit-assignee-id="editAssigneeId"
            :issue="issue"
            :watchers="watchers"
            :users="users"
            :is-watching="isWatching"
            :show-time-tracking="showTimeTracking"
            :estimated-time-sec="estimatedTimeSec"
            :total-time-spent="totalTimeSpent"
            :time-progress="timeProgress"
            :remaining-time-sec="remainingTimeSec"
            :sla-status="slaStatus"
            :due-date-status="dueDateStatus"
            :embedded="embedded"
            @navigate="navigateToIssue"
            @assign-to-me="handleAssignToMe"
            @start-edit-assignee="startEditAssignee"
            @assignee-change="handleAssigneeChange"
            @watch="handleWatchIssue"
            @unwatch="handleUnwatchIssue"
            @add-watcher="showAddWatcherDialog"
            @remove-watcher="handleRemoveWatcher"
          />
        </el-col>
      </el-row>
    </template>

    <!-- 添加关注人对话框 -->
    <el-dialog v-model="addWatcherDialogVisible" :title="t('issue.detail.addWatcher')" width="400px" destroy-on-close>
      <el-select
        v-model="selectedWatcherUserId"
        :placeholder="t('issue.detail.selectUser')"
        style="width: 100%"
        filterable
      >
        <el-option
          v-for="u in availableWatcherUsers"
          :key="u.id"
          :label="u.display_name"
          :value="u.id"
        />
      </el-select>
      <template #footer>
        <el-button @click="addWatcherDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="watcherLoading" :disabled="!selectedWatcherUserId" @click="handleAddWatcher">
          {{ t('common.add') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 拒绝审批对话框 -->
    <el-dialog v-model="rejectDialogVisible" :title="t('issue.detail.rejectApproval')" width="450px" destroy-on-close>
      <el-form label-position="top">
        <el-form-item :label="t('issue.detail.rejectReason')">
          <el-input v-model="rejectComment" type="textarea" :rows="3" :placeholder="t('issue.detail.rejectReasonPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rejectDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="danger" :loading="rejectLoading" :disabled="!rejectComment.trim()" @click="handleReject">
          {{ t('issue.detail.confirmReject') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 编辑对话框 -->
    <el-dialog v-model="editDialogVisible" :title="t('issue.detail.editIssue')" width="640px" destroy-on-close class="edit-dialog">
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-position="top">
        <el-form-item :label="t('issue.title')" prop="title">
          <el-input v-model="editForm.title" maxlength="200" show-word-limit />
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item :label="t('issue.detail.resolution')">
              <el-select v-model="editForm.resolution" :placeholder="t('common.select')" style="width: 100%" clearable>
                <el-option :label="t('issue.resolutionMap.fixed')" value="fixed" />
                <el-option :label="t('issue.resolutionMap.wont_fix')" value="wont_fix" />
                <el-option :label="t('issue.resolutionMap.duplicate')" value="duplicate" />
                <el-option :label="t('issue.resolutionMap.cannot_reproduce')" value="cannot_reproduce" />
                <el-option :label="t('issue.resolutionMap.works_as_designed')" value="works_as_designed" />
                <el-option :label="t('issue.resolutionMap.incomplete')" value="incomplete" />
                <el-option :label="t('issue.resolutionMap.done')" value="done" />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <!-- 所有字段由字段方案驱动 -->
        <div v-if="editFieldScheme.length > 0" class="edit-custom-fields">
          <el-row :gutter="20">
            <el-col
              v-for="item in editFieldScheme"
              :key="item.field_id"
              :span="getEditFieldColSpan(item)"
            >
              <el-form-item :required="item.is_required">
                <template #label>
                  <span>{{ item.field?.field_name }}</span>
                  <el-tooltip v-if="item.field?.description" :content="item.field?.description" placement="top">
                    <el-icon class="field-hint" style="margin-left: 4px; font-size: 14px; color: var(--td-text-disabled); cursor: help;"><QuestionFilled /></el-icon>
                  </el-tooltip>
                </template>
                <!-- assignee 字段：包装"分配给我"按钮 -->
                <div v-if="item.field?.field_key === 'assignee'" style="display: flex; gap: 8px; width: 100%;">
                  <FieldRenderer
                    v-if="item.field && issue"
                    v-model="editFieldValues[item.field_id]"
                    :field="item.field"
                    :scheme="item"
                    :project-key="issue.project_key"
                    style="flex: 1;"
                  />
                  <el-button @click="assignToMeField(item.field_id)">{{ t('issue.detail.assignToMe') }}</el-button>
                </div>
                <FieldRenderer
                  v-else-if="item.field && issue"
                  v-model="editFieldValues[item.field_id]"
                  :field="item.field"
                  :scheme="item"
                  :project-key="issue.project_key"
                />
              </el-form-item>
            </el-col>
          </el-row>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="editLoading" @click="submitEdit">
          <el-icon><Check /></el-icon>
          {{ t('common.save') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- 创建子任务对话框 -->
    <CreateIssueDialog
      v-model="createSubtaskDialogVisible"
      :title="t('issue.detail.createSubtask')"
      :default-project-key="issue?.project_key || ''"
      :parent-id="issue?.id"
      @created="() => issue && loadSubtasks(issue.issue_key)"
    />

    <!-- 流程图连同布局算法与样式已拆到子组件，此处只负责传数据 -->
    <WorkflowDiagramDialog
      v-model="diagramVisible"
      :nodes="diagramNodes"
      :edges="diagramEdges"
      :instance="workflowInstance"
      :history="workflowHistoryList"
      :loading="diagramLoading"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  ArrowDown, ArrowUp, ArrowRight, Plus, Document, View, Check, Close, Link, Delete, Promotion, QuestionFilled, TopRight
} from '@element-plus/icons-vue'
import {
  getIssueDetail, updateIssue, deleteIssue,
  getIssueComments, getIssueActivities, getIssueWatchers,
  getWorklogs,
  addIssueWatcher, removeIssueWatcher, getEpicIssues, getSubtasks } from '@/api/issue'
import { getAlertList } from '@/api/alert'
import type { Alert } from '@/types/alert'
import { listAttachments, uploadAttachment } from '@/api/attachment'
import { getWorkflowInstance, getWorkflowHistory, approveWorkflow, rejectWorkflow, completeWorkflow, getWorkflowNodes, getWorkflowEdges } from '@/api/workflow'
import type { WorkflowInstance, WorkflowHistory, WorkflowNode, WorkflowEdge } from '@/types/workflow'
import type { Attachment } from '@/types/attachment'
import AttachmentUpload from '@/components/attachment/AttachmentUpload.vue'
import AttachmentList from '@/components/attachment/AttachmentList.vue'
import { getAllUsers } from '@/api/user'
import { getPublicConfig } from '@/api/system'
import { getIssueFieldValues, getFieldScheme } from '@/api/field'
import CreateIssueDialog from '@/components/CreateIssueDialog.vue'
import { useUserStore } from '@/stores/user'
import type { Issue, IssueComment, IssueActivity, IssueWatcher, IssueResolution, UpdateIssueRequest, Worklog } from '@/types/issue'
import type { UserOption } from '@/types/user'
import type { FieldValue, FieldSchemeItem, FieldTypeValue } from '@/types/field'
import FieldRenderer from '@/components/field/FieldRenderer.vue'
import { isBuiltinField } from '@/types/field'
import { extractBuiltinFields, backfillBuiltinFields } from '@/utils/builtin-fields'
import dayjs from 'dayjs'
import { getSlaState } from '@/utils/sla'
import { formatActivityAction, formatActivityDetails } from '@/utils/activity'
import WorkflowDiagramDialog from './components/WorkflowDiagramDialog.vue'
import IssueActivityPanel from './components/IssueActivityPanel.vue'
import IssueInfoSidebar from './components/IssueInfoSidebar.vue'

interface Props {
  embedded?: boolean
  issueKey?: string
  onNavigateIssue?: (key: string) => void
  onDeleted?: () => void
}

const { t } = useI18n()

const props = withDefaults(defineProps<Props>(), {
  embedded: false,
  issueKey: '',
  onNavigateIssue: undefined,
  onDeleted: undefined })

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const loading = ref(false)
const issue = ref<Issue | null>(null)
const comments = ref<IssueComment[]>([])
const activities = ref<IssueActivity[]>([])
const watchers = ref<IssueWatcher[]>([])
const worklogs = ref<Worklog[]>([])
const customFields = ref<FieldValue[]>([])
const users = ref<UserOption[]>([])
const epicIssues = ref<Issue[]>([])
const subtasks = ref<Issue[]>([])
const attachments = ref<Attachment[]>([])
const issueAlerts = ref<Alert[]>([])
const showAttachmentUpload = ref(false)

// 工作流相关
const workflowExpanded = ref(false)
const workflowInstance = ref<WorkflowInstance | null>(null)
const workflowHistoryList = ref<WorkflowHistory[]>([])
const approveComment = ref('')
const rejectComment = ref('')
const approveLoading = ref(false)
const rejectLoading = ref(false)
const rejectDialogVisible = ref(false)

// 创建子任务
const createSubtaskDialogVisible = ref(false)

const editDialogVisible = ref(false)
const editLoading = ref(false)
const editFormRef = ref<FormInstance>()
const editForm = reactive({
  title: '',
  resolution: undefined as IssueResolution | undefined })
const editRules: FormRules = {
  title: [{ required: true, message: t('issue.msg.titleRequired'), trigger: ['blur', 'change'] }] }
// 编辑用的字段方案和值
const editFieldScheme = ref<FieldSchemeItem[]>([])
const editFieldValues = ref<Record<number, any>>({})

const loadIssue = async () => {
  const key = props.embedded ? props.issueKey : (route.params.key as string)
  if (!key) return
  loading.value = true
  try {
    const { data } = await getIssueDetail(key, { _redirectOn404: true })
    issue.value = data.data
    // 先加载字段（需要 issue.value 中的 project_key 和 issue_type_id）
    await loadCustomFields(data.data.id)
    // 然后并行加载其他数据
    await Promise.all([
      loadComments(key),
      loadActivities(key),
      loadWatchers(key),
      loadWorklogs(key),
      loadEpicIssues(key),
      loadSubtasks(key),
      loadAttachments(key),
      loadWorkflowData(key),
      loadIssueAlerts(data.data.id),
    ])
  } catch {
    ElMessage.error(t('issue.msg.loadFailed'))
  } finally {
    loading.value = false
  }
}

// embedded 模式下的工单内部导航
const navigateToIssue = (key: string) => {
  if (props.embedded && props.onNavigateIssue) {
    props.onNavigateIssue(key)
  } else {
    router.push(`/issues/${key}`)
  }
}

const loadComments = async (key: string) => {
  try { const { data } = await getIssueComments(key); comments.value = data.data } catch { /* ignored */ }
}
const loadActivities = async (key: string) => {
  try { const { data } = await getIssueActivities(key); activities.value = data.data.items || [] } catch { /* ignored */ }
}
const loadWatchers = async (key: string) => {
  try { const { data } = await getIssueWatchers(key); watchers.value = data.data } catch { /* ignored */ }
}
const loadWorklogs = async (key: string) => {
  try { const { data } = await getWorklogs(key); worklogs.value = data.data } catch { /* ignored */ }
}
const loadEpicIssues = async (key: string) => {
  // 只有当工单类型是 Epic 时才加载
  if (issue.value?.issue_type?.name?.toLowerCase() !== 'epic') {
    epicIssues.value = []
    return
  }
  try {
    const { data } = await getEpicIssues(key)
    epicIssues.value = data.data || []
  } catch {
    epicIssues.value = []
  }
}

const loadSubtasks = async (key: string) => {
  try {
    const { data } = await getSubtasks(key)
    subtasks.value = data.data || []
  } catch {
    subtasks.value = []
  }
}

const loadAttachments = async (key: string) => {
  try {
    const response = await listAttachments(key)
    attachments.value = (response as any).data.data || []
  } catch {
    attachments.value = []
  }
}

// 粘贴图片上传为附件
const handlePaste = async (e: ClipboardEvent) => {
  if (!issue.value || !e.clipboardData) return

  const imageFiles: File[] = []
  for (const item of Array.from(e.clipboardData.items)) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile()
      if (file) imageFiles.push(file)
    }
  }
  if (imageFiles.length === 0) return

  // 阻止默认粘贴行为（避免在输入框中插入图片）
  e.preventDefault()

  const issueKey = issue.value.issue_key
  showAttachmentUpload.value = true

  for (const file of imageFiles) {
    // 给剪贴板图片生成文件名
    const ext = file.type.split('/')[1] || 'png'
    const timestamp = dayjs().format('YYYYMMDDHHmmss')
    const namedFile = new File([file], `paste-${timestamp}.${ext}`, { type: file.type })

    // 检查文件大小
    if (namedFile.size > 10 * 1024 * 1024) {
      ElMessage.error(t('issue.msg.pasteTooLarge'))
      continue
    }

    try {
      await uploadAttachment(issueKey, namedFile)
      ElMessage.success(t('issue.msg.pasteUploaded'))
    } catch {
      ElMessage.error(t('issue.msg.pasteFailed'))
    }
  }
  loadAttachments(issueKey)
}

// 关联告警加载
const loadIssueAlerts = async (issueId: number) => {
  try {
    const { data } = await getAlertList({ issue_id: issueId, page_size: 50 })
    issueAlerts.value = data.data.items || []
  } catch {
    issueAlerts.value = []
  }
}

// 工作流数据加载
const loadWorkflowData = async (key: string) => {
  // 加载工作流实例，404 表示没有工作流，静默处理
  try {
    const { data } = await getWorkflowInstance(key)
    workflowInstance.value = (data as any).data
  } catch {
    // 404 或其他错误表示没有关联工作流，不显示卡片
    workflowInstance.value = null
    workflowHistoryList.value = []
    return
  }

  // 加载工作流节点和边（用于显示下一步节点名称）
  try {
    const workflowId = workflowInstance.value!.workflow_id
    const [nodesRes, edgesRes] = await Promise.all([
      getWorkflowNodes(workflowId),
      getWorkflowEdges(workflowId),
    ])
    diagramNodes.value = (nodesRes.data as any).data || []
    diagramEdges.value = (edgesRes.data as any).data || []
  } catch {
    // 静默处理
  }

  // 只有工作流实例存在时才加载流转历史
  try {
    const { data } = await getWorkflowHistory(key)
    workflowHistoryList.value = (data as any).data || []
  } catch {
    workflowHistoryList.value = []
  }
}

// 判断当前用户是否是审批人（与后端 isAdminOrProjectOwner 对齐）
const isCurrentUserApprover = computed(() => {
  if (!workflowInstance.value || !userStore.user) return false
  const allApprovals = workflowInstance.value.approvals || []
  const currentNodeId = workflowInstance.value.current_node_id
  // 过滤出当前节点的审批记录
  const currentNodeApprovals = allApprovals.filter(a => a.node_id === currentNodeId)
  // 检查用户是否在当前节点的待审批记录中
  const hasPendingRecord = currentNodeApprovals.some(
    a => a.approver_id === userStore.user!.id && a.status === 'pending'
  )
  if (hasPendingRecord) return true
  // 未配置审批人时（当前节点无审批记录），允许系统管理员或项目管理员审批
  if (currentNodeApprovals.length === 0 && (userStore.isAdmin || userStore.isProjectAdmin)) return true
  return false
})

// 判断当前节点是否是工作节点（可完成）
const isWorkNode = computed(() => {
  if (!workflowInstance.value || !isWorkflowOperable.value) return false
  const nodeType = workflowInstance.value.current_node?.node_type
  return nodeType === 'work' || nodeType === 'start'
})

// 获取当前工作节点的所有出边（带条件和目标节点名称）
interface OutgoingAction {
  conditionExpr: string  // 条件表达式（如 approved, rejected, 自定义条件名称）
  label: string          // 显示标签
  targetNodeName: string // 目标节点名称
}

const workNodeOutgoingActions = computed<OutgoingAction[]>(() => {
  if (!workflowInstance.value || !isWorkNode.value) return []
  const currentNodeId = workflowInstance.value.current_node_id
  if (!currentNodeId) return []

  const outEdges = diagramEdges.value.filter(e => e.source_node_id === currentNodeId)

  // 预设条件的显示名称
  const presetLabels: Record<string, string> = {
    approved: t('issue.conditionMap.approved'),
    rejected: t('issue.conditionMap.rejected'),
    confirmed: t('issue.conditionMap.confirmed'),
    continue: t('issue.conditionMap.continue') }

  return outEdges
    .filter(e => e.condition_expr) // 只取有条件的边
    .map(e => {
      const targetNode = diagramNodes.value.find(n => n.id === e.target_node_id)
      return {
        conditionExpr: e.condition_expr,
        label: presetLabels[e.condition_expr] || e.condition_expr,
        targetNodeName: targetNode?.name || '' }
    })
})

// 判断当前工作节点是否有多个条件分支
const workNodeHasBranching = computed(() => {
  return workNodeOutgoingActions.value.length > 0
})

// 获取下一个节点名称（无条件边的目标，用于单一流转）
const nextNodeName = computed(() => {
  if (!workflowInstance.value) return ''
  const currentNodeId = workflowInstance.value.current_node_id
  if (!currentNodeId) return ''

  // 优先找无条件边
  const outEdges = diagramEdges.value.filter(e => e.source_node_id === currentNodeId)
  const unconditionalEdge = outEdges.find(e => !e.condition_expr)
  const targetEdge = unconditionalEdge || outEdges[0]
  if (!targetEdge) return ''

  const targetNode = diagramNodes.value.find(n => n.id === targetEdge.target_node_id)
  return targetNode?.name || ''
})

// 工作流是否处于可操作状态（active 和 reviewing 都可以操作）
const isWorkflowOperable = computed(() => {
  const status = workflowInstance.value?.status
  return status === 'active' || status === 'reviewing'
})

// 当前用户是否可以操作工作流
const canOperateWorkflow = computed(() => {
  return isCurrentUserApprover.value || isWorkNode.value
})

// 工作流快捷按钮文本
const workflowActionBtnText = computed(() => {
  if (!workflowInstance.value) return t('issue.detail.workflow')
  const status = workflowInstance.value.status
  if (status === 'completed' || status === 'cancelled') {
    return t(`issue.workflowStatusMap.${status}`)
  }
  // active 和 reviewing 都显示当前节点名
  const nodeName = workflowInstance.value.current_node?.name || t('issue.detail.currentNode')
  return nodeName
})

// 工作流下拉菜单命令处理
const handleWorkflowCommand = (command: string) => {
  // 处理动态条件命令：complete-condition:xxx
  if (command.startsWith('complete-condition:')) {
    const condition = command.replace('complete-condition:', '')
    handleQuickCompleteWithResult(condition)
    return
  }

  switch (command) {
    case 'approve':
      handleQuickApprove()
      break
    case 'reject':
      showRejectDialog()
      break
    case 'complete':
      handleQuickComplete()
      break
    case 'view-workflow':
      showWorkflowDiagram()
      break
  }
}

// 快捷审批通过（从下拉菜单触发，弹确认框）
const handleQuickApprove = async () => {
  try {
    await ElMessageBox.confirm(t('issue.msg.confirmApprove'), t('issue.msg.approveTitle'), {
      confirmButtonText: t('issue.detail.approve'),
      cancelButtonText: t('common.cancel'),
      type: 'success' })
    handleApprove()
  } catch {
    // 用户取消
  }
}

// 快捷完成节点（从下拉菜单触发，弹确认框）
const handleQuickComplete = async () => {
  try {
    const confirmMsg = nextNodeName.value
      ? t('issue.msg.confirmTransit', { name: nextNodeName.value })
      : t('issue.msg.confirmCompleteNode')
    await ElMessageBox.confirm(confirmMsg, t('issue.msg.transitTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'info' })
    handleComplete()
  } catch {
    // 用户取消
  }
}

// 快捷完成节点（带结果，从下拉菜单触发）
const handleQuickCompleteWithResult = async (result: string) => {
  // 从出边动作中找到对应的信息
  const action = workNodeOutgoingActions.value.find(a => a.conditionExpr === result)
  const actionLabel = action?.label || result
  const targetName = action?.targetNodeName || ''
  const confirmMsg = targetName
    ? t('issue.msg.confirmActionTransit', { action: actionLabel, name: targetName })
    : t('issue.msg.confirmAction', { action: actionLabel })
  try {
    await ElMessageBox.confirm(confirmMsg, t('issue.msg.actionTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: result === 'rejected' ? 'warning' : 'success' })
    handleCompleteWithResult(result)
  } catch {
    // 用户取消
  }
}

const completeLoading = ref(false)
const completeComment = ref('')

// 完成工作节点（默认流转）
const handleComplete = async () => {
  if (!issue.value) return
  completeLoading.value = true
  try {
    await completeWorkflow(issue.value.issue_key, { comment: completeComment.value || undefined })
    ElMessage.success(t('issue.msg.nodeCompleted'))
    completeComment.value = ''
    await loadIssue()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('issue.msg.completeFailed'))
  } finally {
    completeLoading.value = false
  }
}

// 完成工作节点（带结果：任意条件）
const handleCompleteWithResult = async (result: string) => {
  if (!issue.value) return
  completeLoading.value = true
  try {
    await completeWorkflow(issue.value.issue_key, {
      comment: completeComment.value || undefined,
      result: result })
    // 从出边动作中找到对应的标签
    const action = workNodeOutgoingActions.value.find(a => a.conditionExpr === result)
    const actionLabel = action?.label || result
    ElMessage.success(t('issue.msg.actionDone', { action: actionLabel }))
    completeComment.value = ''
    await loadIssue()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('issue.msg.actionFailed'))
  } finally {
    completeLoading.value = false
  }
}

// 审批通过
const handleApprove = async () => {
  if (!issue.value) return
  approveLoading.value = true
  try {
    await approveWorkflow(issue.value.issue_key, { comment: approveComment.value || undefined })
    ElMessage.success(t('issue.approvalStatusMap.approved'))
    approveComment.value = ''
    await loadIssue() // 刷新工单（状态已由工作流联动更新）
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('issue.msg.approveFailed'))
  } finally {
    approveLoading.value = false
  }
}

// 打开拒绝对话框
const showRejectDialog = () => {
  rejectComment.value = ''
  rejectDialogVisible.value = true
}

// 审批拒绝
const handleReject = async () => {
  if (!issue.value || !rejectComment.value.trim()) {
    ElMessage.warning(t('issue.msg.rejectReasonRequired'))
    return
  }
  rejectLoading.value = true
  try {
    await rejectWorkflow(issue.value.issue_key, { comment: rejectComment.value })
    ElMessage.success(t('issue.approvalStatusMap.rejected'))
    rejectComment.value = ''
    rejectDialogVisible.value = false
    await loadIssue() // 刷新工单（状态已由工作流联动更新）
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || t('issue.msg.rejectFailed'))
  } finally {
    rejectLoading.value = false
  }
}

const getWorkflowStatusText = (status: string) => {
  const known = ['active', 'completed', 'cancelled', 'reviewing']
  return known.includes(status) ? t(`issue.workflowStatusMap.${status}`) : status
}

const getApprovalStatusText = (status: string) => {
  const known = ['pending', 'approved', 'rejected']
  return known.includes(status) ? t(`issue.approvalStatusMap.${status}`) : status
}

const getApprovalStatusType = (status: string): 'success' | 'info' | 'warning' | 'danger' => {
  const map: Record<string, 'success' | 'info' | 'warning' | 'danger'> = {
    pending: 'warning',
    approved: 'success',
    rejected: 'danger' }
  return map[status] || 'info'
}

const getHistoryActionText = (action: string) => {
  const known = ['start', 'approve', 'reject', 'forward', 'advance', 'complete', 'cancel']
  return known.includes(action) ? t(`issue.historyActionMap.${action}`) : action
}

// 判断字段值是否已设置（支持数组类型）
const isFieldValueSet = (field: FieldValue): boolean => {
  if (field.value === null || field.value === undefined || field.value === '') return false
  if (Array.isArray(field.value) && field.value.length === 0) return false
  return true
}

const loadCustomFields = async (issueId: number) => {
  if (!issue.value) return
  try {
    // 获取该工单类型配置的字段方案
    const schemeRes = await getFieldScheme(issue.value.project_key, issue.value.issue_type_id)
    const scheme = schemeRes.data.data || []

    // 只获取详情页可见的字段，排除内置字段（已在右侧硬编码展示）
    const visibleScheme = scheme.filter((s: FieldSchemeItem) => s.is_visible_detail && !isBuiltinField(s.field?.field_key || ''))

    // 获取已保存的字段值
    const valuesRes = await getIssueFieldValues(issueId)
    const savedValues = valuesRes.data.data || []

    // 合并：用字段方案作为基础，填充已保存的值
    const valueMap = new Map(savedValues.map((v: FieldValue) => [v.field_id, v]))

    customFields.value = visibleScheme.map((item: FieldSchemeItem) => {
      const savedValue = valueMap.get(item.field_id)
      return {
        field_id: item.field_id,
        field_key: item.field?.field_key || '',
        field_name: item.field?.field_name || '',
        field_type: (item.field?.field_type || 'text') as FieldTypeValue,
        value: savedValue?.value ?? null,
        display_value: savedValue?.display_value || ''
      }
    })
  } catch {
    // ignored
  }
}


// 通过 field_id 设置"分配给我"
const assignToMeField = (fieldId: number) => {
  if (userStore.user) {
    editFieldValues.value[fieldId] = userStore.user.id
  }
}

// 编辑/子任务表单的字段列宽
const getEditFieldColSpan = (item: FieldSchemeItem): number => {
  const fieldType = item.field?.field_type || ''
  if (fieldType === 'textarea' || fieldType === 'epic_link') return 24
  return 12
}

const handleAssignToMe = async () => {
  if (!issue.value || !userStore.user) return
  try {
    await updateIssue(issue.value.issue_key, {
      assignee_id: userStore.user.id
    })
    ElMessage.success(t('issue.msg.assignedToYou'))
    loadIssue()
  } catch {
    ElMessage.error(t('issue.msg.assignFailed'))
  }
}

// 指派人内联编辑（项目管理员）
const editingAssignee = ref(false)
const editAssigneeId = ref<number | undefined>(undefined)

const startEditAssignee = async () => {
  if (users.value.length === 0) {
    try { const { data } = await getAllUsers(); users.value = data.data } catch { /* ignored */ }
  }
  editAssigneeId.value = issue.value?.assignee?.id
  editingAssignee.value = true
}

const handleAssigneeChange = async (userId: number | undefined) => {
  if (!issue.value) return
  try {
    await updateIssue(issue.value.issue_key, { assignee_id: userId || 0 })
    ElMessage.success(t('issue.msg.assigneeUpdated'))
    editingAssignee.value = false
    loadIssue()
  } catch {
    ElMessage.error(t('issue.msg.assigneeUpdateFailed'))
  }
}

const handleDelete = async () => {
  if (!issue.value) return
  try {
    await ElMessageBox.confirm(
      t('issue.msg.confirmDelete', { key: issue.value.issue_key }),
      t('issue.msg.deleteTitle'),
      {
        confirmButtonText: t('issue.msg.confirmDeleteBtn'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
        confirmButtonClass: 'el-button--danger' }
    )
    await deleteIssue(issue.value.issue_key)
    ElMessage.success(t('issue.msg.deleted'))
    if (props.embedded && props.onDeleted) {
      props.onDeleted()
    } else {
      router.push('/issues')
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('issue.msg.deleteFailed'))
    }
  }
}

const handleCreateSubtask = () => {
  if (!issue.value) return
  createSubtaskDialogVisible.value = true
}

const handleEdit = async () => {
  if (!issue.value) return
  try { const { data } = await getAllUsers(); users.value = data.data } catch { /* ignored */ }

  Object.assign(editForm, {
    title: issue.value.title,
    resolution: issue.value.resolution || undefined })

  // 加载编辑用的字段方案
  try {
    const schemeRes = await getFieldScheme(issue.value.project_key, issue.value.issue_type_id)
    const scheme = schemeRes.data.data || []
    editFieldScheme.value = scheme.filter((s: FieldSchemeItem) => s.is_visible_edit)

    // 初始化字段值
    const arrayFieldTypes = ['multiselect', 'label', 'component']
    editFieldValues.value = {}
    // 先为所有数组类型字段初始化空数组
    editFieldScheme.value.forEach(item => {
      if (item.field && arrayFieldTypes.includes(item.field.field_type)) {
        editFieldValues.value[item.field_id] = []
      }
    })

    // 回填内置字段值（从 issue 对象读取）
    backfillBuiltinFields(issue.value, editFieldScheme.value, editFieldValues.value)

    // 加载 EAV 保存的扩展字段值
    const valuesRes = await getIssueFieldValues(issue.value.id)
    const savedValues = valuesRes.data.data || []
    savedValues.forEach((v: FieldValue) => {
      // 只回填非内置字段（内置字段已从 issue 对象回填）
      if (!isBuiltinField(v.field_key)) {
        editFieldValues.value[v.field_id] = v.value
      }
    })
  } catch { /* ignored */ }

  editDialogVisible.value = true
}

const submitEdit = async () => {
  if (!editFormRef.value || !issue.value) return
  await editFormRef.value.validate(async (valid) => {
    if (!valid) return
    editLoading.value = true
    try {
      // 校验必填字段
      for (const item of editFieldScheme.value) {
        if (item.is_required) {
          const val = editFieldValues.value[item.field_id]
          const isEmpty = val === undefined || val === null || val === '' || (Array.isArray(val) && val.length === 0)
          if (isEmpty) {
            ElMessage.error(t('issue.msg.fieldRequired', { field: item.field?.field_name }))
            editLoading.value = false
            return
          }
        }
      }

      // 分离内置字段和扩展字段
      const { builtinValues, customFields } = extractBuiltinFields(editFieldScheme.value, editFieldValues.value)

      const updateData: UpdateIssueRequest = {
        title: editForm.title,
        resolution: editForm.resolution,
        description: builtinValues.description || '',
        priority: builtinValues.priority || undefined,
        assignee_id: builtinValues.assignee_id || undefined,
        planned_start_date: builtinValues.planned_start_date || undefined,
        planned_end_date: builtinValues.planned_end_date || undefined,
        epic_id: builtinValues.epic_id || undefined,
        custom_fields: customFields.length > 0 ? customFields : undefined }
      await updateIssue(issue.value!.issue_key, updateData)
      ElMessage.success(t('issue.msg.updateSuccess'))
      editDialogVisible.value = false
      loadIssue()
    } catch { /* ignored */ }
    finally { editLoading.value = false }
  })
}

// Watcher related
const addWatcherDialogVisible = ref(false)
const selectedWatcherUserId = ref<number | undefined>(undefined)
const watcherLoading = ref(false)

const isWatching = computed(() => {
  if (!userStore.user) return false
  return watchers.value.some(w => w.user_id === userStore.user?.id)
})

const availableWatcherUsers = computed(() => {
  const watcherUserIds = new Set(watchers.value.map(w => w.user_id))
  return users.value.filter(u => !watcherUserIds.has(u.id))
})


const showAddWatcherDialog = async () => {
  if (users.value.length === 0) {
    try {
      const { data } = await getAllUsers()
      users.value = data.data
    } catch {
      ElMessage.error(t('issue.msg.loadUsersFailed'))
      return
    }
  }
  selectedWatcherUserId.value = undefined
  addWatcherDialogVisible.value = true
}

const handleAddWatcher = async () => {
  if (!issue.value || !selectedWatcherUserId.value) return
  watcherLoading.value = true
  try {
    await addIssueWatcher(issue.value.issue_key, selectedWatcherUserId.value)
    ElMessage.success(t('issue.msg.watcherAdded'))
    addWatcherDialogVisible.value = false
    loadWatchers(issue.value.issue_key)
  } catch (error: any) {
    if (error.response?.status === 400) {
      ElMessage.warning(t('issue.msg.watcherExists'))
    } else {
      ElMessage.error(t('issue.msg.watcherAddFailed'))
    }
  } finally {
    watcherLoading.value = false
  }
}

const handleRemoveWatcher = async (userId: number) => {
  if (!issue.value) return
  try {
    await ElMessageBox.confirm(t('issue.msg.confirmRemoveWatcher'), t('issue.msg.tipTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning' })
    await removeIssueWatcher(issue.value.issue_key, userId)
    ElMessage.success(t('issue.msg.removeSuccess'))
    loadWatchers(issue.value.issue_key)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('issue.msg.removeFailed'))
    }
  }
}

const handleWatchIssue = async () => {
  if (!issue.value || !userStore.user) return
  try {
    await addIssueWatcher(issue.value.issue_key, userStore.user.id)
    ElMessage.success(t('issue.msg.watchSuccess'))
    loadWatchers(issue.value.issue_key)
  } catch (error: any) {
    if (error.response?.status === 400) {
      ElMessage.warning(t('issue.msg.alreadyWatching'))
    } else {
      ElMessage.error(t('issue.msg.watchFailed'))
    }
  }
}

const handleUnwatchIssue = async () => {
  if (!issue.value || !userStore.user) return
  try {
    await removeIssueWatcher(issue.value.issue_key, userStore.user.id)
    ElMessage.success(t('issue.msg.unwatchSuccess'))
    loadWatchers(issue.value.issue_key)
  } catch {
    ElMessage.error(t('issue.msg.unwatchFailed'))
  }
}

// Worklog related
// value 是落库的字面量，历史数据全是中文，不能随语言变；只有 label 走语言包
const defaultWorkTypeKeys: { value: string; key: string }[] = [
  { value: '开发', key: 'dev' }, { value: '测试', key: 'test' },
  { value: '调试', key: 'debug' }, { value: '文档', key: 'docs' },
  { value: '故障排查', key: 'troubleshoot' }, { value: '监控运维', key: 'ops' },
  { value: '部署发布', key: 'deploy' }, { value: '配置变更', key: 'config' },
  { value: '巡检', key: 'inspection' }, { value: '安全响应', key: 'security' },
  { value: '其他', key: 'other' },
]
// 后端未配置时用内置列表，配置了则原样使用（管理员自定义的文案不翻译）
const customWorkTypes = ref<{ value: string; label: string }[] | null>(null)
const workTypeOptions = computed<{ value: string; label: string }[]>(() =>
  customWorkTypes.value
    ?? defaultWorkTypeKeys.map(o => ({ value: o.value, label: t(`issue.workTypeMap.${o.key}`) }))
)

const loadWorkTypeOptions = async () => {
  try {
    const res = await getPublicConfig('worklog.work_types')
    const parsed = JSON.parse(res.data.data.config_value || '[]')
    if (Array.isArray(parsed) && parsed.length > 0) {
      customWorkTypes.value = parsed
    }
  } catch {
    // 加载失败时使用默认值，不影响使用
  }
}

const totalTimeSpent = computed(() => {
  return worklogs.value.reduce((sum, w) => sum + w.time_spent_sec, 0)
})

// 时间跟踪
const estimatedTimeSec = computed(() => {
  const field = customFields.value.find(f => f.field_type === 'time_estimate')
  if (!field || !field.value) return 0
  return Number(field.value) || 0
})

const remainingTimeSec = computed(() => {
  if (estimatedTimeSec.value <= 0) return 0
  return estimatedTimeSec.value - totalTimeSpent.value
})

const timeProgress = computed(() => {
  if (estimatedTimeSec.value <= 0) return 0
  return Math.min(Math.round((totalTimeSpent.value / estimatedTimeSec.value) * 100), 100)
})

const showTimeTracking = computed(() => {
  return totalTimeSpent.value > 0
})






type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'
// 优先级圆点与状态药丸：和列表页共用同一套语义
const PRIORITY_COLOR: Record<string, string> = {
  P0: 'var(--td-color-danger)',
  P1: 'var(--td-color-warning)',
  P2: 'var(--td-cat-2)',
  P3: 'var(--td-text-disabled)' }

const priorityColor = (p: string) => PRIORITY_COLOR[p] || 'var(--td-text-disabled)'

const STATUS_TONE: Record<string, string> = {
  open: 'neutral',
  reopened: 'neutral',
  in_progress: 'orange',
  pending_review: 'blue',
  resolved: 'green',
  closed: 'neutral',
  merged: 'purple' }

const statusTone = (s: string) => STATUS_TONE[s] || 'neutral'

// 工作流实例状态：进行中与待审都给橙（需要有人动手），完成给绿，
// 取消是中性——取消不是失败，只是不再推进
const workflowTone = (s: string) =>
  s === 'active' || s === 'reviewing' ? 'orange' : s === 'completed' ? 'green' : 'neutral'

const getPriorityType = (priority: string): TagType => {
  const map: Record<string, TagType> = { P0: 'danger', P1: 'warning', P2: 'info', P3: 'success' }
  return map[priority] || 'info'
}
const getStatusText = (status: string) => {
    // 状态文案统一走语言包：它同时出现在列表、详情、报表、看板，
  // 各处各写一份必然改一处漏三处
  return t(`issue.statusMap.${status}`)
}


const formatTime = (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm')

// SLA 判定走 utils/sla.ts，与工单列表共用一份 ——
// 之前两边各写一份，都漏了 due_date，导致这张卡左边写「已超时 30 天」、
// 右边写「截止时间 2026-09-16」（还没到），自相矛盾。
const slaStatus = computed<{ level: 'overdue' | 'due_soon' | 'normal'; hint: string } | null>(() => {
  if (!issue.value) return null

  const state = getSlaState(issue.value)
  if (!state) return null

  if (state.minutesLeft < 0) {
    return {
      level: 'overdue',
      hint: t('issue.sla.overdueBy', { d: formatSLADuration(Math.abs(Math.round(state.minutesLeft))) }),
    }
  }
  const hint = t('issue.sla.remaining', { d: formatSLADuration(Math.round(state.minutesLeft)) })
  return { level: state.level === 'due_soon' ? 'due_soon' : 'normal', hint }
})

// 格式化 SLA 时长
const formatSLADuration = (minutes: number): string => {
  if (minutes < 60) return t('issue.duration.minutes', { n: minutes })
  const hours = Math.floor(minutes / 60)
  const mins = minutes % 60
  if (hours < 24) {
    return mins > 0 ? t('issue.duration.hoursMinutes', { h: hours, m: mins }) : t('issue.duration.hours', { h: hours })
  }
  const days = Math.floor(hours / 24)
  const remainHours = hours % 24
  return remainHours > 0 ? t('issue.duration.daysHours', { d: days, h: remainHours }) : t('issue.duration.days', { d: days })
}

// 截止时间超时状态（仅用于 due_date 字段旁的标签）
const dueDateStatus = computed<'overdue' | 'due_soon' | 'normal' | null>(() => {
  if (!issue.value?.due_date) return null
  if (['resolved', 'closed'].includes(issue.value.status)) return null
  const now = dayjs()
  const due = dayjs(issue.value.due_date)
  const hoursLeft = due.diff(now, 'hour', true)
  if (hoursLeft < 0) return 'overdue'
  if (hoursLeft < 24) return 'due_soon'
  return 'normal'
})

const getAlertSeverityType = (severity: string) => {
  const map: Record<string, TagType> = { critical: 'danger', warning: 'warning', info: 'info' }
  return map[severity] || 'info'
}
const getAlertSeverityText = (severity: string) => {
  const known = ['critical', 'warning', 'info']
  return known.includes(severity) ? t(`alert.severityMap.${severity}`) : severity
}
const getAlertStatusText = (status: string) => {
  const known = ['firing', 'resolved', 'acked']
  return known.includes(status) ? t(`alert.statusMap.${status}`) : status
}

onMounted(() => {
  loadIssue()
  loadWorkTypeOptions()
  document.addEventListener('paste', handlePaste)
})

onBeforeUnmount(() => {
  document.removeEventListener('paste', handlePaste)
})

// 监听路由参数或 props 变化，当切换到不同的 Issue 时重新加载数据
watch(
  () => props.embedded ? props.issueKey : route.params.key,
  (newKey, oldKey) => {
    if (newKey && newKey !== oldKey) {
      loadIssue()
    }
  }
)

// ============ 工作流流程图 ============

const diagramVisible = ref(false)
const diagramLoading = ref(false)
const diagramNodes = ref<WorkflowNode[]>([])
const diagramEdges = ref<WorkflowEdge[]>([])

// 计算流程图布局（支持分支）
// 显示工作流流程图
const showWorkflowDiagram = async () => {
  diagramVisible.value = true
  // 如果节点数据已加载（loadWorkflowData 中已加载），直接显示
  if (diagramNodes.value.length > 0) {
    return
  }
  // 否则重新加载
  diagramLoading.value = true
  try {
    const workflowId = workflowInstance.value!.workflow_id
    const [nodesRes, edgesRes] = await Promise.all([
      getWorkflowNodes(workflowId),
      getWorkflowEdges(workflowId),
    ])
    diagramNodes.value = (nodesRes.data as any).data || []
    diagramEdges.value = (edgesRes.data as any).data || []
  } catch {
    ElMessage.error(t('issue.msg.loadDiagramFailed'))
  } finally {
    diagramLoading.value = false
  }
}
</script>

<style scoped lang="scss">
@use './issue-detail-shared.scss' as *;

.issue-detail-container {
  width: 100%;

  &.is-embedded {
    padding: 16px 20px;
    background: var(--td-bg-section);

    .issue-header {
      margin-bottom: 16px;
      padding: 20px 24px;
      border-radius: 10px;
      box-shadow: none;
      border: 1px solid var(--td-border-color);
    }

    .embedded-key-row {
      margin-bottom: 6px;
    }

    .embedded-key-link {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      font-size: 13px;
      font-weight: 600;
      color: var(--td-color-primary);
      cursor: pointer;
      text-decoration: none;
      transition: color 150ms ease-out;

      .open-icon {
        font-size: 12px;
      }

      &:hover {
        color: var(--td-color-primary-hover);
        text-decoration: underline;
      }
    }

    .issue-title {
      font-size: 18px;
      font-weight: 600;
      margin-bottom: 10px;
    }

    .issue-title-link {
      display: block;
      color: inherit;
      text-decoration: none;
      cursor: pointer;
      transition: color 150ms ease-out;

      &:hover {
        color: var(--td-color-primary);
      }
    }

    .header-actions {
      flex-shrink: 0;
    }

    .content-card,
    .info-card {
      border-radius: 10px;
      border: 1px solid var(--td-border-color);
      box-shadow: none;
    }

    :deep(.el-row) {
      .el-col {
        .content-card,
        .info-card {
          border-radius: 10px;
        }
      }
    }
  }
}

// 头部
// 页头不再是一张浮起的白卡片：和其它页面一样裸放在页面底色上。
// 面包屑顶栏已经有了，这里留给标题和三项元信息。
.issue-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 14px;
  padding: 0;
  background: transparent;
  border-radius: 0;
  box-shadow: none;

  .header-left { flex: 1; min-width: 0; }

  .issue-breadcrumb { display: none; }

  .issue-title {
    font-size: 26px;
    font-weight: 600;
    letter-spacing: -0.022em;
    margin: 0 0 10px 0;
    color: var(--td-text-primary);
    line-height: 1.15;
  }

  .issue-meta {
    display: flex;
    align-items: center;
    gap: 14px;
    color: var(--td-text-secondary);
    font-size: 14px;
    flex-wrap: wrap;

    // 工单类型不是"状态"，只是分类信息，不该用实心主色抢走整屏注意力。
    // 与 el-tag 的淡底写法保持一致，让同一行里的优先级、状态各自可辨。
    .type-badge {
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 3px 9px;
      background: var(--td-tag-info-bg);
      color: var(--td-tag-info-text);
      border: 1px solid var(--td-tag-info-border);
      border-radius: var(--td-radius-sm);
      font-size: var(--td-font-sm);
      font-weight: var(--td-weight-medium);
      box-shadow: none;

      .type-icon {
        font-size: 14px;
      }

      .type-text {
        line-height: 1;
      }
    }

    .meta-item {
      display: flex;
      align-items: center;
      gap: 5px;
    }
  }

  .header-actions {
    display: flex;
    gap: 10px;
    flex-shrink: 0;
    margin-left: 20px;

    .workflow-action-btn {
      font-weight: 500;
    }
  }
}

// 状态徽章

// 通用卡片





// 侧栏分段：一张卡片内的三段，靠发丝线和小标题区分，不再各起一张卡


// 描述
.description-content {
  white-space: pre-wrap;
  line-height: 1.8;
  color: var(--td-text-regular);
  padding: 20px;
}

// 扩展字段
.custom-fields-card {
  .custom-fields-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 20px;
    padding: 20px;

    .field-item {
      .field-label {
        font-size: 12px;
        font-weight: 500;
        color: var(--td-text-secondary);
        margin-bottom: 6px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
      }

      .field-value {
        font-size: 14px;
        color: var(--td-text-primary);
        line-height: 1.6;
        word-break: break-word;

        .empty-value {
          color: var(--td-text-placeholder);
        }

        .epic-link {
          display: inline-flex;
          align-items: center;
          gap: 6px;
          color: var(--td-color-primary);
          text-decoration: none;
          font-weight: 500;
          transition: all 150ms ease-out;

          &:hover {
            color: var(--td-color-primary-hover);
            text-decoration: underline;
          }

          .el-icon {
            font-size: 14px;
          }
        }
      }
    }
  }
}

// 评论区


// 活动时间线
.activity-timeline {
  padding: 20px;

  .activity-content {
    font-size: 14px;

    .activity-user { font-weight: 600; color: var(--td-text-primary); }
    /* 6px 而不是一个空格的 4px：用户名是 600 字重，英文下「adminUpdated」
       两个词会挤在一起；中文有天然的字面间距，4px 就够，所以一直没暴露。 */
    .activity-details { color: var(--td-text-secondary); margin-left: 6px; }
    .activity-action { color: var(--td-text-secondary); margin: 0 6px; }
    .activity-field { color: var(--td-color-info); margin: 0 4px; }
    .activity-old-value { text-decoration: line-through; color: var(--td-color-danger); margin: 0 4px; }
    .activity-new-value { color: var(--td-color-success); margin: 0 4px; }
  }
}

// 时间跟踪进度条

// 右侧信息


// 关注人列表


// 工作日志



// 工作流卡片
.workflow-card {
  .workflow-current-node {
    padding: 16px 20px;
    border-bottom: 1px solid var(--td-divider-color);

    .current-node-label {
      font-size: 12px;
      font-weight: 500;
      color: var(--td-text-secondary);
      margin-bottom: 8px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }

    .current-node-info {
      display: flex;
      align-items: center;
      gap: 8px;
    }
  }

  .workflow-approvals {
    padding: 16px 20px;
    border-bottom: 1px solid var(--td-divider-color);

    .approvals-label {
      font-size: 12px;
      font-weight: 500;
      color: var(--td-text-secondary);
      margin-bottom: 10px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }

    .approvals-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .approval-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 8px 12px;
      background: var(--td-bg-page);
      border-radius: 8px;

      .approval-user {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 13px;
        color: var(--td-text-primary);
        font-weight: 500;
      }

      .approval-comment {
        font-size: 12px;
        color: var(--td-text-secondary);
        margin-left: auto;
      }
    }
  }

  .workflow-actions {
    padding: 16px 20px;
    border-bottom: 1px solid var(--td-divider-color);

    .actions-label {
      font-size: 12px;
      font-weight: 500;
      color: var(--td-text-secondary);
      margin-bottom: 10px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }

    .actions-row {
      display: flex;
      align-items: center;
    }
  }

  .workflow-history {
    padding: 16px 20px;

    .history-label {
      font-size: 12px;
      font-weight: 500;
      color: var(--td-text-secondary);
      margin-bottom: 12px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
    }

    .workflow-timeline {
      padding-left: 4px;
    }

    .history-content {
      font-size: 13px;

      .history-user {
        font-weight: 600;
        color: var(--td-text-primary);
      }

      .history-action {
        color: var(--td-text-secondary);
        margin: 0 4px;
      }

      .history-arrow {
        color: var(--td-text-placeholder);
        margin: 0 4px;
      }

      .history-comment {
        margin-top: 4px;
        font-size: 12px;
        color: var(--td-text-secondary);
        padding: 6px 10px;
        background: var(--td-bg-page);
        border-radius: 6px;
      }
    }
  }
}

// 响应式
@media (max-width: 768px) {
  .issue-header {
    flex-direction: column;
    gap: 16px;

    .header-actions {
      margin-left: 0;
      width: 100%;
    }
  }
}

// 编辑对话框
.edit-custom-fields {
  margin-top: 8px;

  .section-divider {
    display: flex;
    align-items: center;
    margin-bottom: 16px;
    padding-top: 8px;
    border-top: 1px dashed var(--td-border-color);

    span {
      font-size: 13px;
      font-weight: 500;
      color: var(--td-text-secondary);
    }
  }
}

:deep(.edit-dialog) {
  .el-dialog__body {
    max-height: 70vh;
    overflow-y: auto;
  }
}

// Epic Issues 列表
.epic-issues-card {
  .epic-issues-list {
    padding: 8px;

    .empty-state {
      padding: 40px 0;
    }

    .epic-issue-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 14px 16px;
      margin-bottom: 8px;
      background: var(--td-bg-card);
      border-radius: 8px;
      border: 1px solid var(--td-border-color);
      transition: all 150ms ease-out;
      cursor: pointer;

      &:hover {
        background: var(--td-bg-section);
        border-color: var(--td-border-color-dark);
      }

      &:last-child {
        margin-bottom: 0;
      }

      .issue-left {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 12px;
        min-width: 0;

        .issue-type-icon {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          font-size: 15px;
          flex-shrink: 0;
          color: var(--td-color-primary);

          // 只给图标上色，不套实心方块（§3.1 不放装饰性图标色块）
          &.bug, &.fault { color: var(--td-color-danger); }
          &.epic { color: var(--td-cat-5); }
          &.subtask { color: var(--td-text-secondary); }
        }

        .issue-link {
          text-decoration: none;
          flex-shrink: 0;

          .issue-key {
            font-size: 13px;
            font-weight: 600;
            color: var(--td-color-primary);
            transition: all 150ms ease-out;

            &:hover {
              color: var(--td-color-primary-hover);
              text-decoration: underline;
            }
          }
        }

        .issue-title {
          font-size: 14px;
          color: var(--td-text-primary);
          font-weight: 500;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          flex: 1;
          min-width: 0;
        }
      }

      .issue-right {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-shrink: 0;

        .priority-tag {
          font-size: 11px;
          font-weight: 600;
          padding: 2px 8px;
          border-radius: 4px;
        }

        .status-badge {
          display: flex;
          align-items: center;
          gap: 6px;
          padding: 4px 12px;
          border-radius: 12px;
          font-size: 12px;
          font-weight: 500;
          background: var(--td-bg-section);
          color: var(--td-text-secondary);

          .status-dot {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background: currentColor;
          }

          &.open {
            background: var(--td-tag-primary-border);
            color: var(--td-tag-primary-text);
          }

          &.in_progress {
            background: var(--td-tag-orange-border);
            color: var(--td-tag-orange-text);
          }

          &.resolved {
            background: var(--td-tag-success-border);
            color: var(--td-tag-success-text);
          }

          &.closed {
            background: var(--td-border-color);
            color: var(--td-text-regular);
          }

          &.pending_review {
            background: var(--td-tag-warning-bg);
            color: var(--td-tag-orange-text);
          }

          &.merged {
            background: var(--td-tag-purple-bg);
            color: var(--td-tag-purple-text);
          }
        }

        .assignee-info {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-shrink: 0;

          .assignee-avatar {
            width: 22px;
            height: 22px;
            border-radius: 50%;
            background: var(--td-tag-primary-bg);
            color: var(--td-tag-primary-text);
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 10.5px;
            font-weight: 600;
            flex-shrink: 0;

            &.unassigned {
              background: var(--td-border-color);
              color: var(--td-text-placeholder);
            }
          }

          .assignee-name {
            font-size: 13px;
            color: var(--td-text-regular);
            font-weight: 500;
            white-space: nowrap;

            &.unassigned {
              color: var(--td-text-placeholder);
            }
          }


        }
      }
    }
  }
}

// 子任务列表（复用 Epic Issues 样式）
.subtasks-card {
  .subtasks-list {
    padding: 8px;

    .empty-state-compact {
      padding: 12px 16px;
      text-align: center;

      .empty-text {
        color: var(--td-text-placeholder);
        font-size: 13px;
      }
    }

    .subtask-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 14px 16px;
      margin-bottom: 8px;
      background: var(--td-bg-card);
      border-radius: 8px;
      border: 1px solid var(--td-border-color);
      transition: all 150ms ease-out;
      cursor: pointer;

      &:hover {
        background: var(--td-bg-section);
        border-color: var(--td-border-color-dark);
      }

      &:last-child {
        margin-bottom: 0;
      }

      .issue-left {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 12px;
        min-width: 0;

        .issue-type-icon {
          display: inline-flex;
          align-items: center;
          justify-content: center;
          font-size: 15px;
          flex-shrink: 0;
          color: var(--td-color-primary);

          // 只给图标上色，不套实心方块（§3.1 不放装饰性图标色块）
          &.bug, &.fault { color: var(--td-color-danger); }
          &.epic { color: var(--td-cat-5); }
          &.subtask { color: var(--td-text-secondary); }
        }

        .issue-link {
          text-decoration: none;
          flex-shrink: 0;

          .issue-key {
            font-size: 13px;
            font-weight: 600;
            color: var(--td-color-danger);
            transition: all 150ms ease-out;

            &:hover {
              color: var(--td-color-danger);
              text-decoration: underline;
            }
          }
        }

        .issue-title {
          font-size: 14px;
          color: var(--td-text-primary);
          font-weight: 500;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
          flex: 1;
          min-width: 0;
        }
      }

      .issue-right {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-shrink: 0;

        .priority-tag {
          font-size: 11px;
          font-weight: 600;
          padding: 2px 8px;
          border-radius: 4px;
        }

        .status-badge {
          display: flex;
          align-items: center;
          gap: 6px;
          padding: 4px 12px;
          border-radius: 12px;
          font-size: 12px;
          font-weight: 500;
          background: var(--td-bg-section);
          color: var(--td-text-secondary);

          .status-dot {
            width: 6px;
            height: 6px;
            border-radius: 50%;
            background: currentColor;
          }

          &.open {
            background: var(--td-tag-primary-border);
            color: var(--td-tag-primary-text);
          }

          &.in_progress {
            background: var(--td-tag-orange-border);
            color: var(--td-tag-orange-text);
          }

          &.resolved {
            background: var(--td-tag-success-border);
            color: var(--td-tag-success-text);
          }

          &.closed {
            background: var(--td-border-color);
            color: var(--td-text-regular);
          }

          &.pending_review {
            background: var(--td-tag-warning-bg);
            color: var(--td-tag-orange-text);
          }

          &.merged {
            background: var(--td-tag-purple-bg);
            color: var(--td-tag-purple-text);
          }
        }

        .assignee-info {
          display: flex;
          align-items: center;
          gap: 8px;
          flex-shrink: 0;

          .assignee-avatar {
            width: 22px;
            height: 22px;
            border-radius: 50%;
            background: var(--td-tag-orange-bg);
            color: var(--td-tag-orange-text);
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 10.5px;
            font-weight: 600;
            flex-shrink: 0;

            &.unassigned {
              background: var(--td-border-color);
              color: var(--td-text-placeholder);
            }
          }

          .assignee-name {
            font-size: 13px;
            color: var(--td-text-regular);
            font-weight: 500;
            white-space: nowrap;

            &.unassigned {
              color: var(--td-text-placeholder);
            }
          }


        }
      }
    }
  }
}

// 附件区域
// 卡片 body 本身已经有内边距，这里再套一层就变成每边 36px，
// 一句"暂无附件"被撑成 130px 高的空块
.attachment-section {
  padding: 0;
}

// 工作流流程图

// 关联告警样式
.alert-card {
  :deep(.el-table) {
    .clickable-row { cursor: pointer; &:hover { background-color: var(--td-bg-page); } }
  }

  .alert-name-text { font-weight: 500; color: var(--td-text-primary); font-size: 13px; }

  .alert-status-badge {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 8px;
    border-radius: 12px;
    font-size: 12px;

    .status-dot { width: 6px; height: 6px; border-radius: 50%; }

    &.firing { background: var(--td-tag-danger-bg); color: var(--td-color-danger); .status-dot { background: var(--td-color-danger); } }
    &.resolved { background: var(--td-tag-success-bg); color: var(--td-color-success); .status-dot { background: var(--td-color-success); } }
  }
}
/* 拒绝：破坏性操作不常驻红，中性描边，悬停和按下才亮红 */
.reject-btn {
  &:hover,
  &:focus-visible {
    color: var(--td-color-danger);
    border-color: var(--td-color-danger);
    background: var(--td-tag-danger-bg);
  }
}
</style>
