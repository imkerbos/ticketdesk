<template>
  <div class="custom-node approval-node" :class="{ selected: selected }">
    <Handle type="target" :position="Position.Left" />
    <div class="node-body">
      <div class="node-header">
        <div class="node-icon">
          <el-icon><Checked /></el-icon>
        </div>
        <div class="node-title">{{ data.label }}</div>
      </div>
      <div class="node-meta">
        <span class="node-type-badge">{{ t('workflow.nodeTypeMap.approval') }}</span>
        <span v-if="approvalTypeText" class="node-detail">{{ approvalTypeText }}</span>
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
import { Checked } from '@element-plus/icons-vue'
import type { NodeConfig } from '@/types/workflow'

const { t } = useI18n()

const props = defineProps<{
  data: { label: string; config?: NodeConfig }
  selected?: boolean
}>()

const approvalTypeText = computed(() => {
  const map: Record<string, string> = {
    single: t('workflow.approvalTypeMap.single'),
    countersign: t('workflow.approvalTypeMap.countersign'),
    or_sign: t('workflow.approvalTypeMap.or_sign'),
  }
  return props.data.config?.approval_type ? map[props.data.config.approval_type] : ''
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
.approval-node {
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
    background: var(--td-tag-orange-bg);
    color: var(--td-tag-orange-text);
    border-bottom: 1px solid var(--td-tag-orange-border);
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
    background: var(--td-tag-orange-bg);
    color: var(--td-tag-orange-text);
    border: 1px solid var(--td-tag-orange-border);
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
