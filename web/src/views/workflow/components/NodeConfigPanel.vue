<template>
  <div v-if="node" class="config-panel">
    <div class="panel-header">
      <h3 class="panel-title">{{ t('workflow.panel.title') }}</h3>
      <el-button text size="small" @click="$emit('close')">
        <el-icon><Close /></el-icon>
      </el-button>
    </div>

    <el-scrollbar class="panel-body">
      <el-form label-position="top" size="default">
        <!-- 基本信息 -->
        <div class="config-section">
          <div class="section-title">{{ t('workflow.panel.basic') }}</div>
          <el-form-item :label="t('workflow.nodeName')">
            <el-input
              v-model="localName"
              :placeholder="t('workflow.nodeNamePlaceholder')"
              @change="emitUpdate"
            />
          </el-form-item>
          <el-form-item :label="t('workflow.nodeType')">
            <el-tag :type="nodeTypeTagType" size="large">{{ nodeTypeText }}</el-tag>
          </el-form-item>
        </div>

        <!-- 审批节点配置 -->
        <template v-if="node.data.nodeType === 'approval'">
          <div class="config-section">
            <div class="section-title">{{ t('workflow.panel.approvalConfig') }}</div>
            <el-form-item :label="t('workflow.approvalType')">
              <el-select v-model="localConfig.approval_type" :placeholder="t('workflow.approvalTypePlaceholder')" style="width: 100%" @change="emitUpdate">
                <el-option :label="t('workflow.approvalTypeMap.single')" value="single" />
                <el-option :label="t('workflow.approvalTypeOption.countersign')" value="countersign" />
                <el-option :label="t('workflow.approvalTypeOption.or_sign')" value="or_sign" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('workflow.approvers')">
              <el-select
                v-model="localConfig.approvers"
                multiple
                filterable
                :placeholder="t('workflow.approversPlaceholder')"
                style="width: 100%"
                @change="emitUpdate"
              >
                <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('workflow.approverRole')">
              <el-input v-model="localConfig.approver_role" :placeholder="t('workflow.approverRolePlaceholder')" @change="emitUpdate" />
            </el-form-item>
          </div>
        </template>

        <!-- 工作节点配置 -->
        <template v-if="node.data.nodeType === 'work'">
          <div class="config-section">
            <div class="section-title">{{ t('workflow.panel.assignConfig') }}</div>
            <el-form-item :label="t('workflow.assigneeType')">
              <el-select v-model="localConfig.assignee_type" :placeholder="t('workflow.assigneeTypePlaceholder')" style="width: 100%" @change="emitUpdate">
                <el-option :label="t('workflow.assigneeTypeMap.user')" value="user" />
                <el-option :label="t('workflow.assigneeTypeMap.role')" value="role" />
                <el-option :label="t('workflow.assigneeTypeMap.reporter')" value="reporter" />
                <el-option :label="t('workflow.assigneeTypeMap.project_lead')" value="project_lead" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="localConfig.assignee_type === 'user'" :label="t('workflow.assignees')">
              <el-select
                v-model="localConfig.assignees"
                multiple
                filterable
                :placeholder="t('workflow.assigneesPlaceholder')"
                style="width: 100%"
                @change="emitUpdate"
              >
                <el-option v-for="u in users" :key="u.id" :label="u.display_name" :value="u.id" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="localConfig.assignee_type === 'role'" :label="t('workflow.assigneeRole')">
              <el-input v-model="localConfig.assignee_role" :placeholder="t('workflow.assigneeRolePlaceholder')" @change="emitUpdate" />
            </el-form-item>
          </div>
        </template>

        <!-- 系统节点配置 -->
        <template v-if="node.data.nodeType === 'system'">
          <div class="config-section">
            <div class="section-title">{{ t('workflow.systemAction') }}</div>
            <el-form-item :label="t('workflow.actionName')">
              <el-input v-model="localConfig.action" :placeholder="t('workflow.actionPlaceholder')" @change="emitUpdate" />
            </el-form-item>
          </div>
        </template>

        <!-- 通用配置（开始/结束节点不显示） -->
        <template v-if="node.data.nodeType !== 'start' && node.data.nodeType !== 'end'">
          <div class="config-section">
            <div class="section-title">{{ t('workflow.panel.common') }}</div>
            <el-form-item :label="t('workflow.targetStatus')">
              <el-select
                v-model="localConfig.target_status"
                :placeholder="t('workflow.targetStatusPlaceholder')"
                style="width: 100%"
                clearable
                @change="emitUpdate"
              >
                <el-option :label="t('workflow.statusOption.open')" value="open" />
                <el-option :label="t('workflow.statusOption.in_progress')" value="in_progress" />
                <el-option :label="t('workflow.statusOption.pending_review')" value="pending_review" />
                <el-option :label="t('workflow.statusOption.resolved')" value="resolved" />
                <el-option :label="t('workflow.statusOption.closed')" value="closed" />
              </el-select>
            </el-form-item>
            <el-form-item :label="t('workflow.timeout')">
              <el-input-number
                v-model="localConfig.timeout_hours"
                :min="0"
                :max="720"
                :placeholder="t('workflow.timeoutPlaceholder')"
                style="width: 100%"
                @change="emitUpdate"
              />
            </el-form-item>
            <el-form-item :label="t('workflow.nodeDesc')">
              <el-input
                v-model="localConfig.description"
                type="textarea"
                :rows="3"
                :placeholder="t('workflow.nodeDesc')"
                @change="emitUpdate"
              />
            </el-form-item>
          </div>
        </template>

        <!-- 删除按钮（开始/结束节点不可删除） -->
        <div v-if="node.data.nodeType !== 'start' && node.data.nodeType !== 'end'" class="config-section">
          <el-button type="danger" plain style="width: 100%" @click="$emit('delete', node.id)">
            <el-icon><Delete /></el-icon>
            {{ t('workflow.panel.deleteNode') }}
          </el-button>
        </div>
      </el-form>
    </el-scrollbar>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, watch, computed, reactive } from 'vue'
import { Close, Delete } from '@element-plus/icons-vue'
import type { NodeConfig } from '@/types/workflow'

const { t } = useI18n()

interface FlowNode {
  id: string
  data: {
    label: string
    nodeType: string
    config?: NodeConfig
    backendId?: number
  }
}

interface UserOption {
  id: number
  display_name: string
}

const props = defineProps<{
  node: FlowNode | null
  users: UserOption[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update', payload: { id: string; name: string; config: NodeConfig }): void
  (e: 'delete', id: string): void
}>()

const localName = ref('')
const localConfig = reactive<NodeConfig>({
  approval_type: undefined,
  approvers: [],
  approver_role: '',
  assignee_type: undefined,
  assignees: [],
  assignee_role: '',
  action: '',
  parameters: {},
  timeout_hours: 0,
  description: '',
  target_status: undefined,
})

// 当选中节点变化时，同步数据到本地
watch(
  () => props.node,
  (newNode) => {
    if (newNode) {
      localName.value = newNode.data.label || ''
      const cfg = newNode.data.config || {}
      Object.assign(localConfig, {
        approval_type: cfg.approval_type || undefined,
        approvers: cfg.approvers || [],
        approver_role: cfg.approver_role || '',
        assignee_type: cfg.assignee_type || undefined,
        assignees: cfg.assignees || [],
        assignee_role: cfg.assignee_role || '',
        action: cfg.action || '',
        parameters: cfg.parameters || {},
        timeout_hours: cfg.timeout_hours || 0,
        description: cfg.description || '',
        target_status: cfg.target_status || undefined,
      })
    }
  },
  { immediate: true, deep: true }
)

const emitUpdate = () => {
  if (!props.node) return
  emit('update', {
    id: props.node.id,
    name: localName.value,
    config: { ...localConfig },
  })
}

const nodeTypeText = computed(() => {
  const map: Record<string, string> = {
    start: t('workflow.nodeTypeFullMap.start'),
    end: t('workflow.nodeTypeFullMap.end'),
    approval: t('workflow.nodeTypeFullMap.approval'),
    work: t('workflow.nodeTypeFullMap.work'),
    system: t('workflow.nodeTypeFullMap.system'),
  }
  return props.node ? map[props.node.data.nodeType] || props.node.data.nodeType : ''
})

const nodeTypeTagType = computed((): 'primary' | 'success' | 'info' | 'warning' | 'danger' => {
  const map: Record<string, 'primary' | 'success' | 'info' | 'warning' | 'danger'> = {
    start: 'success',
    end: 'info',
    approval: 'warning',
    work: 'primary',
    system: 'danger',
  }
  return props.node ? (map[props.node.data.nodeType] || 'info') : 'info'
})
</script>

<style scoped lang="scss">
.config-panel {
  width: 300px;
  background: var(--td-bg-card);
  border-left: 1px solid var(--td-border-color);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow: hidden;
}

/* 高度与设计器顶部工具栏一致，两条分隔线连成一线 */
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  height: 48px;
  padding: 0 10px 0 14px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--td-border-color);

  .panel-title {
    font-size: 14px;
    font-weight: 590;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
    margin: 0;
  }
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: 14px;
}

.config-section {
  margin-bottom: 18px;

  &:last-child {
    margin-bottom: 0;
  }

  .section-title {
    font-size: 12px;
    font-weight: 590;
    letter-spacing: -0.005em;
    color: var(--td-text-secondary);
    margin-bottom: 12px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--td-divider-color);
  }
}

:deep(.el-form-item) {
  margin-bottom: 14px;
}

:deep(.el-form-item__label) {
  font-size: 12px;
  color: var(--td-text-secondary);
  padding-bottom: 4px !important;
}
</style>
