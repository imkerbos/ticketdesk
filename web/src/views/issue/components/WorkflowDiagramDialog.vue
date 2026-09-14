<template>
  <el-dialog v-model="visible" :title="t('issue.detail.flowDiagram')" width="860px" destroy-on-close>
    <div v-loading="!!loading" class="workflow-diagram">
      <div v-if="diagramLayout.nodes.length > 0" class="diagram-graph" :style="{ minHeight: diagramLayout.height + 'px', minWidth: diagramLayout.width + 'px' }">
        <!-- SVG 连线层 -->
        <svg class="diagram-edges" :width="diagramLayout.width" :height="diagramLayout.height">
          <defs>
            <marker id="arrowhead" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--td-border-color-dark)" />
            </marker>
            <marker id="arrowhead-visited" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
              <polygon points="0 0, 8 3, 0 6" fill="var(--td-color-success)" />
            </marker>
          </defs>
          <template v-for="edge in diagramLayout.edges" :key="`${edge.from}-${edge.to}`">
            <line
              :x1="edge.x1" :y1="edge.y1" :x2="edge.x2" :y2="edge.y2"
              :stroke="edge.visited ? 'var(--td-color-success)' : 'var(--td-border-color-dark)'"
              stroke-width="2"
              :marker-end="edge.visited ? 'url(#arrowhead-visited)' : 'url(#arrowhead)'"
            />
            <text
              v-if="edge.label"
              :x="(edge.x1 + edge.x2) / 2"
              :y="(edge.y1 + edge.y2) / 2 - 6"
              text-anchor="middle"
              :fill="edge.visited ? 'var(--td-color-success)' : 'var(--td-color-info)'"
              font-size="11"
            >{{ edge.label }}</text>
          </template>
        </svg>
        <!-- 节点层 -->
        <div
          v-for="ln in diagramLayout.nodes"
          :key="ln.node.id"
          class="diagram-node"
          :class="getNodeDiagramClass(ln.node)"
          :style="{ left: ln.x + 'px', top: ln.y + 'px' }"
        >
          <div class="diagram-node-icon">
            <span v-if="ln.node.node_type === 'start'">▶</span>
            <span v-else-if="ln.node.node_type === 'end'">◉</span>
            <span v-else-if="ln.node.node_type === 'approval'">✓</span>
            <span v-else-if="ln.node.node_type === 'work'">⚙</span>
            <span v-else>●</span>
          </div>
          <div class="diagram-node-name">{{ ln.node.name }}</div>
          <div class="diagram-node-type">{{ getNodeTypeText(ln.node.node_type) }}</div>
          <div v-if="getNodeDiagramClass(ln.node).visited" class="diagram-node-check">✓</div>
        </div>
      </div>
      <div v-else-if="!loading" class="diagram-empty">
        <TdEmptyState preset="no-data" :title="t('issue.detail.noFlowNodes')" />
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
/**
 * 工作流流程图对话框。
 *
 * 从 IssueDetail.vue 里拆出来：这块（模板 + 布局算法 + 样式）约 330 行，
 * 且和详情页其余部分没有任何共用样式，是最干净的一条切分线。
 * 组件只负责画图，节点与流转历史由父组件加载后传进来。
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WorkflowNode, WorkflowEdge, WorkflowInstance, WorkflowHistory } from '@/types/workflow'

const { t } = useI18n()

const props = defineProps<{
  nodes: WorkflowNode[]
  edges: WorkflowEdge[]
  instance: WorkflowInstance | null
  history: WorkflowHistory[]
  loading?: boolean
}>()

const visible = defineModel<boolean>({ required: true })

interface LayoutNode { node: WorkflowNode; x: number; y: number }
interface LayoutEdge { from: number; to: number; x1: number; y1: number; x2: number; y2: number; visited: boolean; label: string }

const diagramLayout = computed(() => {
  const nodes = props.nodes
  const edges = props.edges
  const result = { nodes: [] as LayoutNode[], edges: [] as LayoutEdge[], width: 0, height: 0 }

  if (nodes.length === 0) return result

  const nodeW = 110
  const nodeH = 80
  const gapX = 160
  const gapY = 120
  const padX = 40
  const padY = 30

  // BFS 分层
  const nodeMap = new Map<number, WorkflowNode>()
  nodes.forEach(n => nodeMap.set(n.id, n))

  const adjacency = new Map<number, number[]>()
  nodes.forEach(n => adjacency.set(n.id, []))
  edges.forEach(e => {
    if (adjacency.has(e.source_node_id)) {
      adjacency.get(e.source_node_id)!.push(e.target_node_id)
    }
  })

  // 找开始节点
  const startNode = nodes.find(n => n.node_type === 'start')
  if (!startNode) return result

  const levels = new Map<number, number>()
  const visited = new Set<number>()
  const queue: { id: number; level: number }[] = [{ id: startNode.id, level: 0 }]
  visited.add(startNode.id)

  while (queue.length > 0) {
    const { id, level } = queue.shift()!
    levels.set(id, Math.max(levels.get(id) || 0, level))
    for (const next of (adjacency.get(id) || [])) {
      if (!visited.has(next)) {
        visited.add(next)
        queue.push({ id: next, level: level + 1 })
      }
    }
  }

  // 未连接的节点
  const maxLevel = Math.max(...Array.from(levels.values()), 0)
  nodes.forEach(n => {
    if (!levels.has(n.id)) levels.set(n.id, maxLevel + 1)
  })

  // 按层分组
  const levelGroups = new Map<number, number[]>()
  for (const [nodeId, level] of levels) {
    if (!levelGroups.has(level)) levelGroups.set(level, [])
    levelGroups.get(level)!.push(nodeId)
  }

  // 计算节点位置（水平布局）
  const nodePositions = new Map<number, { x: number; y: number }>()
  const totalLevels = Math.max(...Array.from(levelGroups.keys())) + 1

  for (let lvl = 0; lvl < totalLevels; lvl++) {
    const group = levelGroups.get(lvl) || []
    const startY = padY + (group.length > 1 ? 0 : (gapY - nodeH) / 2)

    group.forEach((nodeId, idx) => {
      const x = padX + lvl * gapX
      const y = startY + idx * gapY
      nodePositions.set(nodeId, { x, y })
    })
  }

  // 生成布局节点
  for (const [nodeId, pos] of nodePositions) {
    const node = nodeMap.get(nodeId)
    if (node) {
      result.nodes.push({ node, x: pos.x, y: pos.y })
    }
  }

  // 生成布局边
  const presetConditionLabels: Record<string, string> = { approved: t('issue.detail.approve'), rejected: t('issue.detail.reject'), confirmed: t('issue.conditionMap.confirmed'), continue: t('issue.conditionMap.continue') }
  for (const edge of edges) {
    const fromPos = nodePositions.get(edge.source_node_id)
    const toPos = nodePositions.get(edge.target_node_id)
    if (!fromPos || !toPos) continue

    result.edges.push({
      from: edge.source_node_id,
      to: edge.target_node_id,
      x1: fromPos.x + nodeW,
      y1: fromPos.y + nodeH / 2,
      x2: toPos.x,
      y2: toPos.y + nodeH / 2,
      visited: visitedEdgePairs.value.has(`${edge.source_node_id}-${edge.target_node_id}`),
      label: presetConditionLabels[edge.condition_expr] || edge.condition_expr || '',
    })
  }

  // 计算画布尺寸
  let maxX = 0, maxY = 0
  for (const pos of nodePositions.values()) {
    maxX = Math.max(maxX, pos.x + nodeW)
    maxY = Math.max(maxY, pos.y + nodeH)
  }
  result.width = maxX + padX
  result.height = maxY + padY

  return result
})

// 获取已访问的节点 ID 集合（从流转历史中提取）
const visitedNodeIds = computed(() => {
  const ids = new Set<number>()
  props.history.forEach(h => {
    if (h.from_node_id) ids.add(h.from_node_id)
    if (h.to_node_id) ids.add(h.to_node_id)
  })
  return ids
})

// 获取已访问的边（from→to 对）
const visitedEdgePairs = computed(() => {
  const pairs = new Set<string>()
  props.history.forEach(h => {
    if (h.from_node_id && h.to_node_id) {
      pairs.add(`${h.from_node_id}-${h.to_node_id}`)
    }
  })
  return pairs
})

// 判断节点的流程图样式类
const getNodeDiagramClass = (node: WorkflowNode) => {
  const currentNodeId = props.instance?.current_node_id
  const isCurrent = node.id === currentNodeId
  const isVisited = visitedNodeIds.value.has(node.id) && !isCurrent
  const instanceStatus = props.instance?.status

  const isOperable = instanceStatus === 'active' || instanceStatus === 'reviewing'

  return {
    current: isCurrent && isOperable,
    visited: isVisited || (isCurrent && instanceStatus === 'completed'),
    cancelled: instanceStatus === 'cancelled' && isCurrent,
    pending: !isCurrent && !isVisited,
    'node-start': node.node_type === 'start',
    'node-end': node.node_type === 'end',
    'node-approval': node.node_type === 'approval',
    'node-work': node.node_type === 'work',
  }
}

// 获取节点类型文本
const getNodeTypeText = (nodeType: string) => {
  const known = ['start', 'end', 'approval', 'work', 'system']
  return known.includes(nodeType) ? t(`issue.nodeTypeMap.${nodeType}`) : nodeType
}

</script>

<style scoped lang="scss">
.workflow-diagram {
  min-height: 120px;
  overflow: auto;
  padding: 20px 0;

  .diagram-graph {
    position: relative;
    margin: 0 auto;
  }

  .diagram-edges {
    position: absolute;
    top: 0;
    left: 0;
    pointer-events: none;
  }

  .diagram-node {
    position: absolute;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    width: 110px;
    height: 80px;
    border-radius: 10px;
    border: 2px solid var(--td-border-color);
    background: var(--td-bg-page);
    transition: all 150ms ease-out;
    cursor: default;

    .diagram-node-icon {
      font-size: 20px;
      margin-bottom: 4px;
      color: var(--td-color-info);
    }

    .diagram-node-name {
      font-size: 13px;
      font-weight: 600;
      color: var(--td-text-primary);
      white-space: nowrap;
      max-width: 100px;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .diagram-node-type {
      font-size: 11px;
      color: var(--td-color-info);
      margin-top: 2px;
    }

    .diagram-node-check {
      position: absolute;
      top: -8px;
      right: -8px;
      width: 20px;
      height: 20px;
      border-radius: 50%;
      background: var(--td-color-success);
      color: var(--td-text-white);
      font-size: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      font-weight: bold;
    }

    // 节点类型图标颜色
    &.node-start .diagram-node-icon { color: var(--td-color-success); }
    &.node-end .diagram-node-icon { color: var(--td-color-info); }
    &.node-approval .diagram-node-icon { color: var(--td-color-warning); }
    &.node-work .diagram-node-icon { color: var(--td-color-primary); }

    // 已完成节点
    &.visited {
      border-color: var(--td-color-success);
      background: var(--td-tag-success-bg);

      .diagram-node-name { color: var(--td-color-success); }
      .diagram-node-icon { color: var(--td-color-success); }
    }

    // 当前节点
    &.current {
      border-color: var(--td-color-primary);
      background: var(--td-tag-primary-bg);
      box-shadow: var(--td-focus-ring);
      animation: pulse-border 2s ease-in-out infinite;

      .diagram-node-name { color: var(--td-color-primary); }
      .diagram-node-icon { color: var(--td-color-primary); }
    }

    // 被取消节点
    &.cancelled {
      border-color: var(--td-color-danger);
      background: var(--td-tag-danger-bg);

      .diagram-node-name { color: var(--td-color-danger); }
      .diagram-node-icon { color: var(--td-color-danger); }
    }

    // 未到达节点
    &.pending {
      border-color: var(--td-border-color);
      background: var(--td-bg-page);
      opacity: 0.7;
    }
  }

  .diagram-empty {
    display: flex;
    justify-content: center;
    padding: 20px;
  }
}
</style>
