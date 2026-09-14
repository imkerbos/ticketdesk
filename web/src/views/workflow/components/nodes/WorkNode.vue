<template>
  <div class="custom-node work-node" :class="{ selected: selected }">
    <Handle type="target" :position="Position.Left" />
    <div class="node-body">
      <div class="node-header">
        <div class="node-icon">
          <el-icon><Operation /></el-icon>
        </div>
        <div class="node-title">{{ data.label }}</div>
      </div>
      <div class="node-meta">
        <span class="node-type-badge">{{ t('workflow.nodeTypeMap.work') }}</span>
        <span v-if="assigneeTypeText" class="node-detail">{{ assigneeTypeText }}</span>
      </div>
      <div v-if="data.config?.target_status" class="node-status">
        → {{ statusText }}
      </div>
    </div>
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { Operation } from '@element-plus/icons-vue'
import type { NodeConfig } from '@/types/workflow'

const { t } = useI18n()

const props = defineProps<{
  data: { label: string; config?: NodeConfig }
  selected?: boolean
}>()

const assigneeTypeText = computed(() => {
  const map: Record<string, string> = {
    user: t('workflow.assigneeTypeMap.user'),
    role: t('workflow.assigneeTypeMap.role'),
    reporter: t('workflow.assigneeTypeMap.reporter'),
    project_lead: t('workflow.assigneeTypeMap.project_lead'),
  }
  return props.data.config?.assignee_type ? map[props.data.config.assignee_type] : ''
})

const statusText = computed(() => {
  const map: Record<string, string> = {
    open: t('workflow.statusMap.open'),
    in_progress: t('workflow.statusMap.in_progress'),
    pending_review: t('workflow.statusMap.pending_review'),
    resolved: t('workflow.statusMap.resolved'),
    closed: t('workflow.statusMap.closed'),
  }
  return props.data.config?.target_status ? (map[props.data.config.target_status] || props.data.config.target_status) : ''
})
</script>

<style scoped lang="scss">
.work-node {
  .node-body {
    background: var(--td-bg-card);
    /* 节点是浮在画布上的对象，给一层发丝边 + 轻阴影就够了。
       原来 2px 饱和描边 + 同色光晕，几十个节点铺开满屏都在喊。 */
    border: 1px solid var(--td-border-color);
    border-radius: 10px;
    min-width: 160px;
    overflow: hidden;
    box-shadow: var(--td-elevation-3);
    transition: border-color 150ms ease-out, box-shadow 150ms ease-out;
  }

  /* 选中态所有节点一致：主色描边 + 聚焦环，不做缩放位移 */
  &.selected .node-body {
    border-color: var(--td-color-primary);
    box-shadow: var(--td-focus-ring), var(--td-elevation-3);
  }

  &:hover .node-body {
    border-color: var(--td-border-color-dark);
  }

  .node-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 9px 12px;
    background: var(--td-tag-primary-bg);
    color: var(--td-tag-primary-text);
    border-bottom: 1px solid var(--td-tag-primary-border);
  }

  .node-icon {
    font-size: 15px;
    line-height: 1;
    flex-shrink: 0;
  }

  .node-title {
    font-size: 13px;
    font-weight: 590;
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .node-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 12px;
    flex-wrap: wrap;
  }

  .node-type-badge {
    font-size: 11px;
    line-height: 16px;
    padding: 0 7px;
    background: var(--td-tag-primary-bg);
    color: var(--td-tag-primary-text);
    border: 1px solid var(--td-tag-primary-border);
    border-radius: 5px;
    font-weight: 500;
  }

  .node-detail {
    font-size: 11px;
    color: var(--td-text-secondary);
  }

  .node-status {
    padding: 0 12px 8px;
    font-size: 11px;
    color: var(--td-text-secondary);
  }
}
</style>
