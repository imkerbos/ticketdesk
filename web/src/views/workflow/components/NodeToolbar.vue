<template>
  <div class="node-toolbar">
    <div class="toolbar-title">{{ t('workflow.toolbar.title') }}</div>
    <div class="toolbar-hint">{{ t('workflow.toolbar.hint') }}</div>
    <div class="toolbar-items">
      <div
        v-for="item in nodeTypes"
        :key="item.type"
        class="toolbar-item"
        :class="item.type"
        draggable="true"
        @dragstart="onDragStart($event, item.type)"
      >
        <div class="item-icon">
          <el-icon><component :is="item.icon" /></el-icon>
        </div>
        <div class="item-info">
          <div class="item-name">{{ t(item.labelKey) }}</div>
          <div class="item-desc">{{ t(item.descKey) }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { VideoPlay, CircleCheck, Checked, Operation, Setting } from '@element-plus/icons-vue'
import type { NodeType } from '@/types/workflow'

const { t } = useI18n()

const nodeTypes: { type: NodeType; labelKey: string; descKey: string; icon: any }[] = [
  { type: 'start', labelKey: 'workflow.nodeTypeMap.start', descKey: 'workflow.toolbar.startDesc', icon: VideoPlay },
  { type: 'end', labelKey: 'workflow.nodeTypeMap.end', descKey: 'workflow.toolbar.endDesc', icon: CircleCheck },
  { type: 'approval', labelKey: 'workflow.nodeTypeMap.approval', descKey: 'workflow.toolbar.approvalDesc', icon: Checked },
  { type: 'work', labelKey: 'workflow.nodeTypeMap.work', descKey: 'workflow.toolbar.workDesc', icon: Operation },
  { type: 'system', labelKey: 'workflow.nodeTypeMap.system', descKey: 'workflow.toolbar.systemDesc', icon: Setting },
]

const onDragStart = (event: DragEvent, nodeType: NodeType) => {
  if (event.dataTransfer) {
    event.dataTransfer.setData('application/vueflow', nodeType)
    event.dataTransfer.effectAllowed = 'move'
  }
}
</script>

<style scoped lang="scss">
.node-toolbar {
  width: 200px;
  background: var(--td-bg-card);
  border-right: 1px solid var(--td-border-color);
  padding: 14px 14px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex-shrink: 0;
  overflow-y: auto;
}

.toolbar-title {
  font-size: 13px;
  font-weight: 590;
  letter-spacing: -0.01em;
  color: var(--td-text-primary);
}

.toolbar-hint {
  font-size: 11px;
  line-height: 1.5;
  color: var(--td-text-placeholder);
  margin-top: -7px;
}

.toolbar-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.toolbar-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: grab;
  border: 1px solid var(--td-border-color);
  background: var(--td-bg-card);
  transition: background-color 150ms ease-out, border-color 150ms ease-out;

  /* 类型色已经由左边的图标颜色表达，悬停不再整条换成对应色，
     五条并排一路变色比不变更难扫读。 */
  &:hover {
    background: var(--td-bg-section);
    border-color: var(--td-border-color-dark);
  }

  &:active {
    cursor: grabbing;
    transform: translateY(0.5px);
  }

  /* 只给图标本身上类型色，不套底色方块 ——
     「浅色圆角方块 + 图标」是后台模板最强的特征，CLAUDE.md 3.1 明确不用。
     颜色照旧和画布上同类型节点的头部对应，图例作用没丢。 */
  .item-icon {
    width: 20px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 16px;
    flex-shrink: 0;
  }

  .item-info {
    flex: 1;
    min-width: 0;
  }

  .item-name {
    font-size: 13px;
    font-weight: 500;
    letter-spacing: -0.01em;
    color: var(--td-text-primary);
  }

  .item-desc {
    font-size: 11px;
    color: var(--td-text-placeholder);
  }

  /* 与画布上同类型节点的头部色调一一对应 */
  &.start .item-icon { color: var(--td-tag-success-text); }
  &.end .item-icon { color: var(--td-tag-info-text); }
  &.approval .item-icon { color: var(--td-tag-orange-text); }
  &.work .item-icon { color: var(--td-tag-primary-text); }
  &.system .item-icon { color: var(--td-tag-purple-text); }
}

@media (prefers-reduced-motion: reduce) {
  .toolbar-item {
    transition: none;
  }
}
</style>
