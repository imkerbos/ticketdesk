<template>
  <div class="custom-node end-node" :class="{ selected: selected }">
    <Handle type="target" :position="Position.Left" />
    <div class="node-body">
      <div class="node-icon">
        <el-icon><CircleCheck /></el-icon>
      </div>
      <div class="node-label">{{ data.label }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'
import { CircleCheck } from '@element-plus/icons-vue'

defineProps<{
  data: { label: string }
  selected?: boolean
}>()
</script>

<style scoped lang="scss">
.end-node {
  .node-body {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 5px;
    padding: 11px 18px;
    /* 起止节点原本是整块饱和实心 + 彩色投影，是画布上最跳的两个元素。
       它们只是流程的端点标识，淡底 + 同色描边即可。 */
    background: var(--td-tag-info-bg);
    border: 1px solid var(--td-tag-info-border);
    border-radius: 10px;
    color: var(--td-tag-info-text);
    min-width: 80px;
    box-shadow: var(--td-elevation-3);
    transition: border-color 150ms ease-out, box-shadow 150ms ease-out;
  }

  &.selected .node-body {
    border-color: var(--td-color-primary);
    box-shadow: var(--td-focus-ring), var(--td-elevation-3);
  }

  &:hover .node-body {
    border-color: var(--td-border-color-dark);
  }

  .node-icon {
    font-size: 18px;
    line-height: 1;
  }

  .node-label {
    font-size: 12px;
    font-weight: 590;
    letter-spacing: -0.01em;
    white-space: nowrap;
  }
}
</style>
