<template>
  <div class="workflow-designer">
    <!-- 顶部工具栏 -->
    <div class="designer-toolbar">
      <div class="toolbar-left">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          {{ t('common.back') }}
        </el-button>
        <el-divider direction="vertical" />
        <div class="workflow-title">
          <el-icon class="title-icon"><Connection /></el-icon>
          <span v-if="!editingTitle" class="title-text" @dblclick="startEditTitle">
            {{ workflowName }}
          </span>
          <el-input
            v-else
            ref="titleInputRef"
            v-model="workflowName"
            size="small"
            style="width: 200px"
            @blur="finishEditTitle"
            @keyup.enter="finishEditTitle"
          />
          <el-tag v-if="hasUnsavedChanges" type="warning" size="small" class="unsaved-tag">{{ t('workflow.designer.unsaved') }}</el-tag>
        </div>
      </div>
      <div class="toolbar-right">
        <el-button :icon="Rank" @click="autoLayout">{{ t('workflow.designer.autoLayout') }}</el-button>
        <el-button type="primary" :loading="saving" :icon="Check" @click="handleSave">
          {{ t('common.save') }}
        </el-button>
      </div>
    </div>

    <!-- 主体区域 -->
    <div v-loading="loading" class="designer-body" :element-loading-text="t('workflow.designer.loading')">
      <!-- 左侧节点工具栏 -->
      <NodeToolbar />

      <!-- 中间画布 -->
      <div ref="canvasRef" class="canvas-wrapper" @drop="onDrop" @dragover="onDragOver">
        <VueFlow
          v-model:nodes="nodes"
          v-model:edges="edges"
          :node-types="nodeTypes"
          :default-edge-options="defaultEdgeOptions"
          :connection-mode="ConnectionMode.Loose"
          :snap-to-grid="true"
          :snap-grid="[20, 20]"
          fit-view-on-init
          class="vue-flow-canvas"
          @node-click="onNodeClick"
          @edge-click="onEdgeClick"
          @pane-click="onPaneClick"
          @nodes-change="onNodesChange"
          @connect="onConnect"
          @node-drag-stop="onNodeDragStop"
        >
          <Background :gap="20" :size="1" pattern-color="var(--td-border-color)" />
          <Controls position="bottom-left" />
          <!-- 没有节点时缩略图就是一块空白板子，只剩噪音 -->
          <MiniMap v-if="nodes.length > 0" position="bottom-right" :pannable="true" :zoomable="true" />
        </VueFlow>

        <!-- 空画布要说清楚下一步怎么做：草稿工作流打开就是一整片网格，
             没有任何提示时看不出「从左边拖一个节点进来」是入口。 -->
        <div v-if="!loading && nodes.length === 0" class="canvas-empty">
          <div class="canvas-empty-title">{{ t('workflow.designer.emptyTitle') }}</div>
          <div class="canvas-empty-desc">{{ t('workflow.designer.emptyDesc') }}</div>
        </div>
      </div>

      <!-- 右侧配置面板 -->
      <NodeConfigPanel
        v-if="selectedNode"
        :node="selectedNode"
        :users="allUsers"
        @update="onNodeConfigUpdate"
        @delete="onDeleteNode"
        @close="selectedNode = null"
      />

      <!-- 边配置弹窗 -->
      <EdgeConfigPanel
        v-if="selectedEdge"
        :edge="selectedEdge"
        :nodes="(nodes as any)"
        @update="onEdgeConfigUpdate"
        @delete="onDeleteEdge"
        @close="selectedEdge = null"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, markRaw, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { VueFlow, ConnectionMode, MarkerType, useVueFlow } from '@vue-flow/core'
import type { Node, Edge, Connection, NodeChange } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { MiniMap } from '@vue-flow/minimap'
import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'
import '@vue-flow/minimap/dist/style.css'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Connection as ConnectionIcon, Check, Rank } from '@element-plus/icons-vue'
import {
  getWorkflowDetail,
  getWorkflowNodes,
  getWorkflowEdges,
  updateWorkflow,
  createNode,
  updateNode,
  deleteNode as apiDeleteNode,
  createEdge,
  updateEdge,
  deleteEdge as apiDeleteEdge,
} from '@/api/workflow'
import { getAllUsers } from '@/api/user'
import type { WorkflowNode, WorkflowEdge, NodeConfig, NodeType } from '@/types/workflow'
import type { UserOption } from '@/types/user'

// 自定义节点组件
import StartNode from './components/nodes/StartNode.vue'
import EndNode from './components/nodes/EndNode.vue'
import ApprovalNode from './components/nodes/ApprovalNode.vue'
import WorkNode from './components/nodes/WorkNode.vue'
import SystemNode from './components/nodes/SystemNode.vue'
import NodeToolbar from './components/NodeToolbar.vue'
import NodeConfigPanel from './components/NodeConfigPanel.vue'
import EdgeConfigPanel from './components/EdgeConfigPanel.vue'

const { t } = useI18n()

// 避免 unused 警告
void ConnectionIcon

const route = useRoute()
const router = useRouter()
const workflowId = computed(() => Number(route.params.id))

// Vue Flow 实例
const { project } = useVueFlow()

// 注册自定义节点类型
const nodeTypes: any = {
  start: markRaw(StartNode),
  end: markRaw(EndNode),
  approval: markRaw(ApprovalNode),
  work: markRaw(WorkNode),
  system: markRaw(SystemNode),
}

// 默认边选项
const defaultEdgeOptions = {
  type: 'smoothstep',
  animated: true,
  style: { stroke: 'var(--td-border-color-dark)', strokeWidth: 1.5 },
  markerEnd: {
    type: MarkerType.ArrowClosed,
    color: 'var(--td-border-color-dark)',
  },
}

// 状态
const loading = ref(false)
const saving = ref(false)
const workflowName = ref('')
const editingTitle = ref(false)
const titleInputRef = ref()
const nodes = ref<Node[]>([])
const edges = ref<Edge[]>([])
const allUsers = ref<UserOption[]>([])
const canvasRef = ref<HTMLElement>()

// 选中状态
const selectedNode = ref<any>(null)
const selectedEdge = ref<any>(null)

// 原始数据快照（用于检测变更）
const originalNodes = ref<WorkflowNode[]>([])
const originalEdges = ref<WorkflowEdge[]>([])

// 临时 ID 计数器（用于新建节点/边）
let tempIdCounter = 0
const getTempId = () => `temp_${++tempIdCounter}`

// 是否有未保存的变更
const hasUnsavedChanges = ref(false)

// ============ 数据加载 ============

const loadWorkflow = async () => {
  loading.value = true
  try {
    const [detailRes, nodesRes, edgesRes] = await Promise.all([
      getWorkflowDetail(workflowId.value, { _redirectOn404: true }),
      getWorkflowNodes(workflowId.value),
      getAllUsers(),
    ])

    workflowName.value = (detailRes.data as any).data.name || ''

    const backendNodes: WorkflowNode[] = (nodesRes.data as any).data || []
    originalNodes.value = JSON.parse(JSON.stringify(backendNodes))

    // 加载边
    const edgesRes2 = await getWorkflowEdges(workflowId.value)
    const backendEdges: WorkflowEdge[] = (edgesRes2.data as any).data || []
    originalEdges.value = JSON.parse(JSON.stringify(backendEdges))

    // 转换为 Vue Flow 格式
    nodes.value = backendNodes.map(convertBackendNodeToFlow)
    edges.value = backendEdges.map(convertBackendEdgeToFlow)

    allUsers.value = (edgesRes.data as any).data || []
    hasUnsavedChanges.value = false
  } catch {
    ElMessage.error(t('workflow.designer.loadFailed'))
  } finally {
    loading.value = false
  }
}

// ============ 数据转换 ============

const convertBackendNodeToFlow = (node: WorkflowNode): Node => {
  let config: NodeConfig | undefined
  if (node.config) {
    config = typeof node.config === 'string' ? JSON.parse(node.config as any) : node.config
  }

  return {
    id: String(node.id),
    type: node.node_type,
    position: { x: node.position_x || 0, y: node.position_y || 0 },
    data: {
      label: node.name,
      nodeType: node.node_type,
      config: config || {},
      backendId: node.id,
    },
  }
}

// 预设条件的显示名称映射
const presetConditionLabels = (): Record<string, string> => ({
  approved: t('workflow.edge.approved'),
  rejected: t('issue.detail.reject'),
})

const convertBackendEdgeToFlow = (edge: WorkflowEdge): Edge => {
  // 将条件表达式转为友好标签：预设条件用中文，自定义条件直接显示
  const conditionLabel = presetConditionLabels()[edge.condition_expr] || edge.condition_expr || ''
  return {
    id: String(edge.id),
    source: String(edge.source_node_id),
    target: String(edge.target_node_id),
    type: 'smoothstep',
    animated: true,
    style: { stroke: 'var(--td-border-color-dark)', strokeWidth: 1.5 },
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: 'var(--td-border-color-dark)',
    },
    label: conditionLabel,
    // 边标签是 SVG 文本，而节点是 HTML 层、天然盖在边层之上。
    // 单靠 z-index 压不过去（那会让连线也盖住节点，更糟），
    // 因此给标签配一块不透明底：即使与节点重叠，文字本身依然可读。
    labelShowBg: true,
    labelBgPadding: [6, 3] as [number, number],
    labelBgBorderRadius: 4,
    labelBgStyle: { fill: 'var(--td-bg-card)', stroke: 'var(--td-border-color)', strokeWidth: 1 },
    labelStyle: { fill: 'var(--td-text-secondary)', fontSize: 11 },
    data: {
      conditionExpr: edge.condition_expr || '',
      backendId: edge.id,
    },
  }
}

// ============ 拖拽添加节点 ============

const onDragOver = (event: DragEvent) => {
  event.preventDefault()
  if (event.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }
}

const onDrop = (event: DragEvent) => {
  if (!event.dataTransfer) return
  const nodeType = event.dataTransfer.getData('application/vueflow') as NodeType
  if (!nodeType) return

  // 计算画布上的位置
  const bounds = canvasRef.value?.getBoundingClientRect()
  if (!bounds) return

  const position = project({
    x: event.clientX - bounds.left,
    y: event.clientY - bounds.top,
  })

  const defaultNames: Record<string, string> = {
    start: t('workflow.nodeTypeMap.start'),
    end: t('workflow.nodeTypeMap.end'),
    approval: t('workflow.nodeTypeFullMap.approval'),
    work: t('workflow.nodeTypeFullMap.work'),
    system: t('workflow.nodeTypeFullMap.system'),
  }

  const newNode: Node = {
    id: getTempId(),
    type: nodeType,
    position,
    data: {
      label: defaultNames[nodeType] || t('workflow.designer.newNode'),
      nodeType: nodeType,
      config: {},
      backendId: undefined,
    },
  }

  nodes.value = [...nodes.value, newNode]
  hasUnsavedChanges.value = true

  // 自动选中新节点
  nextTick(() => {
    selectedNode.value = newNode
    selectedEdge.value = null
  })
}

// ============ 节点/边交互 ============

const onNodeClick = (event: any) => {
  selectedNode.value = event.node
  selectedEdge.value = null
}

const onEdgeClick = (event: any) => {
  selectedEdge.value = event.edge
  selectedNode.value = null
}

const onPaneClick = () => {
  selectedNode.value = null
  selectedEdge.value = null
}

const onNodesChange = (changes: NodeChange[]) => {
  // Vue Flow 首次渲染时会发出 dimensions 变更（测量节点尺寸），select 只是
  // 选中态 —— 两者都不是对图的修改。原来不加区分地标脏，页面一打开就常驻
  // 「未保存」，离开时还会弹一次"有未保存变更"的确认框。
  if (changes.some((c) => c.type !== 'dimensions' && c.type !== 'select')) {
    hasUnsavedChanges.value = true
  }
}

const onNodeDragStop = () => {
  hasUnsavedChanges.value = true
}

const onConnect = (connection: Connection) => {
  // 检查是否已存在相同的边
  const exists = edges.value.some(
    e => e.source === connection.source && e.target === connection.target
  )
  if (exists) {
    ElMessage.warning(t('workflow.designer.edgeExists'))
    return
  }

  const newEdge: Edge = {
    id: getTempId(),
    source: connection.source,
    target: connection.target,
    type: 'smoothstep',
    animated: true,
    style: { stroke: 'var(--td-border-color-dark)', strokeWidth: 1.5 },
    markerEnd: {
      type: MarkerType.ArrowClosed,
      color: 'var(--td-border-color-dark)',
    },
    data: {
      conditionExpr: '',
      backendId: undefined,
    },
  }

  edges.value = [...edges.value, newEdge]
  hasUnsavedChanges.value = true
}

// ============ 配置更新 ============

const onNodeConfigUpdate = (payload: { id: string; name: string; config: NodeConfig }) => {
  const idx = nodes.value.findIndex(n => n.id === payload.id)
  if (idx === -1) return

  const updated = { ...nodes.value[idx] }
  updated.data = {
    ...updated.data,
    label: payload.name,
    config: payload.config,
  }
  nodes.value[idx] = updated
  // 触发响应式更新
  nodes.value = [...nodes.value]
  hasUnsavedChanges.value = true

  // 同步选中节点
  if (selectedNode.value?.id === payload.id) {
    selectedNode.value = updated
  }
}

const onEdgeConfigUpdate = (payload: { id: string; conditionExpr: string; label?: string }) => {
  const idx = edges.value.findIndex(e => e.id === payload.id)
  if (idx === -1) return

  const updated = { ...edges.value[idx] }
  // 使用显式标签，如果没有则用条件表达式的友好名称
  const conditionLabel = presetConditionLabels()[payload.conditionExpr] || payload.conditionExpr
  updated.label = payload.label || conditionLabel || ''
  updated.data = {
    ...updated.data,
    conditionExpr: payload.conditionExpr,
  }
  edges.value[idx] = updated
  edges.value = [...edges.value]
  hasUnsavedChanges.value = true
  selectedEdge.value = null
}

// ============ 删除操作 ============

const onDeleteNode = async (nodeId: string) => {
  try {
    await ElMessageBox.confirm(t('workflow.designer.confirmDeleteNode'), t('issue.list.deleteTitle'), {
      type: 'warning',
    })
  } catch {
    return
  }

  // 删除关联的边
  edges.value = edges.value.filter(e => e.source !== nodeId && e.target !== nodeId)
  // 删除节点
  nodes.value = nodes.value.filter(n => n.id !== nodeId)
  selectedNode.value = null
  hasUnsavedChanges.value = true
}

const onDeleteEdge = async (edgeId: string) => {
  try {
    await ElMessageBox.confirm(t('workflow.designer.confirmDeleteEdge'), t('issue.list.deleteTitle'), {
      type: 'warning',
    })
  } catch {
    return
  }

  edges.value = edges.value.filter(e => e.id !== edgeId)
  selectedEdge.value = null
  hasUnsavedChanges.value = true
}

// ============ 自动布局 ============

const autoLayout = () => {
  if (nodes.value.length === 0) return

  // 简单的从左到右自动布局
  // 找到开始节点
  const startNode = nodes.value.find(n => n.data.nodeType === 'start')
  if (!startNode) {
    ElMessage.warning(t('workflow.designer.noStartNode'))
    return
  }

  // BFS 遍历确定层级
  const levels: Map<string, number> = new Map()
  const visited = new Set<string>()
  const queue: { id: string; level: number }[] = [{ id: startNode.id, level: 0 }]
  visited.add(startNode.id)

  while (queue.length > 0) {
    const { id, level } = queue.shift()!
    levels.set(id, level)

    // 找到所有出边
    const outEdges = edges.value.filter(e => e.source === id)
    for (const edge of outEdges) {
      if (!visited.has(edge.target)) {
        visited.add(edge.target)
        queue.push({ id: edge.target, level: level + 1 })
      }
    }
  }

  // 未连接的节点放到最后一层
  const maxLevel = Math.max(...Array.from(levels.values()), 0)
  for (const node of nodes.value) {
    if (!levels.has(node.id)) {
      levels.set(node.id, maxLevel + 1)
    }
  }

  // 按层级分组
  const levelGroups: Map<number, string[]> = new Map()
  for (const [nodeId, level] of levels) {
    if (!levelGroups.has(level)) {
      levelGroups.set(level, [])
    }
    levelGroups.get(level)!.push(nodeId)
  }

  // 从左往右排：层级决定 x，同层内的分支往下错开。
  // 节点的 handle 在左右两边（见 nodes/*.vue），纵向排会让每条边都绕 U 形。
  const xGap = 300
  const yGap = 170

  const updatedNodes = nodes.value.map(node => {
    const level = levels.get(node.id) || 0
    const group = levelGroups.get(level) || [node.id]
    const indexInGroup = group.indexOf(node.id)
    const groupHeight = (group.length - 1) * yGap
    const startY = 240 - groupHeight / 2

    return {
      ...node,
      position: {
        x: 60 + level * xGap,
        y: startY + indexInGroup * yGap,
      },
    }
  })

  nodes.value = updatedNodes
  hasUnsavedChanges.value = true
}

// ============ 保存逻辑 ============

const handleSave = async () => {
  saving.value = true
  try {
    // 1. 更新工作流名称
    await updateWorkflow(workflowId.value, { name: workflowName.value })

    // 2. 计算节点差异并同步
    await syncNodes()

    // 3. 计算边差异并同步
    await syncEdges()

    // 4. 重新加载数据以获取最新的后端 ID
    await loadWorkflow()

    ElMessage.success(t('common.saveSuccess'))
    hasUnsavedChanges.value = false
  } catch {
    ElMessage.error(t('project.fieldConfig.saveFailed'))
  } finally {
    saving.value = false
  }
}

const syncNodes = async () => {
  const currentNodeIds = new Set(nodes.value.map(n => n.data.backendId).filter(Boolean))
  const originalNodeIds = new Set(originalNodes.value.map(n => n.id))

  // 删除的节点（原来有，现在没有）
  for (const origNode of originalNodes.value) {
    if (!currentNodeIds.has(origNode.id)) {
      await apiDeleteNode(workflowId.value, origNode.id)
    }
  }

  // 新增和更新的节点
  // 需要先创建新节点，获取后端 ID，然后更新边的引用
  const idMapping: Map<string, number> = new Map() // tempId -> backendId

  for (const node of nodes.value) {
    if (node.data.backendId) {
      // 已有后端 ID，更新
      if (originalNodeIds.has(node.data.backendId)) {
        await updateNode(workflowId.value, node.data.backendId, {
          name: node.data.label,
          config: node.data.config || undefined,
          position_x: Math.round(node.position.x),
          position_y: Math.round(node.position.y),
        })
      }
    } else {
      // 新节点，创建
      const res = await createNode(workflowId.value, {
        name: node.data.label,
        node_type: node.data.nodeType as NodeType,
        config: node.data.config || undefined,
        position_x: Math.round(node.position.x),
        position_y: Math.round(node.position.y),
      })
      const newBackendId = (res.data as any).data.id
      idMapping.set(node.id, newBackendId)
      // 更新本地节点的 backendId
      node.data.backendId = newBackendId
    }
  }

  return idMapping
}

const syncEdges = async () => {
  const currentEdgeBackendIds = new Set(
    edges.value.map(e => e.data?.backendId).filter(Boolean)
  )
  const originalEdgeIds = new Set(originalEdges.value.map(e => e.id))

  // 删除的边
  for (const origEdge of originalEdges.value) {
    if (!currentEdgeBackendIds.has(origEdge.id)) {
      await apiDeleteEdge(workflowId.value, origEdge.id)
    }
  }

  // 新增和更新的边
  for (const edge of edges.value) {
    // 解析源和目标的后端 ID
    const sourceNode = nodes.value.find(n => n.id === edge.source)
    const targetNode = nodes.value.find(n => n.id === edge.target)
    if (!sourceNode?.data.backendId || !targetNode?.data.backendId) continue

    if (edge.data?.backendId && originalEdgeIds.has(edge.data.backendId)) {
      // 更新已有边
      await updateEdge(workflowId.value, edge.data.backendId, {
        condition_expr: edge.data?.conditionExpr || '',
      })
    } else if (!edge.data?.backendId) {
      // 新建边
      await createEdge(workflowId.value, {
        source_node_id: sourceNode.data.backendId,
        target_node_id: targetNode.data.backendId,
        condition_expr: edge.data?.conditionExpr || undefined,
      })
    }
  }
}

// ============ 标题编辑 ============

const startEditTitle = () => {
  editingTitle.value = true
  nextTick(() => {
    titleInputRef.value?.focus()
  })
}

const finishEditTitle = () => {
  editingTitle.value = false
  hasUnsavedChanges.value = true
}

// ============ 导航 ============

const goBack = async () => {
  if (hasUnsavedChanges.value) {
    try {
      await ElMessageBox.confirm(t('workflow.designer.leaveConfirm'), t('issue.msg.tipTitle'), {
        confirmButtonText: t('workflow.designer.leave'),
        cancelButtonText: t('common.cancel'),
        type: 'warning',
      })
    } catch {
      return
    }
  }
  router.push('/workflows')
}

// ============ 初始化 ============

onMounted(() => {
  loadWorkflow()
})
</script>

<style scoped lang="scss">
/* 设计器要占满内容区：吃掉 .ap .content 的 20/24/24 内边距，
   高度按 52px 顶栏算。 */
.workflow-designer {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 52px);
  width: calc(100% + var(--td-content-pad-x) * 2);
  /* 抵消 .content 的内边距做满幅。跟着变量走 —— 写死 24px 的话，
     窄屏下 .content 变成 16px，两边各多撑 8px，页面主体就横滚了。 */
  margin: calc(var(--td-content-pad-top) * -1) calc(var(--td-content-pad-x) * -1)
    calc(var(--td-content-pad-bottom) * -1);
  background: var(--td-bg-page);
}

.designer-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  height: 48px;
  flex-shrink: 0;
  background: var(--td-bg-card);
  border-bottom: 1px solid var(--td-border-color);
  z-index: 10;

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .workflow-title {
    display: flex;
    align-items: center;
    gap: 7px;
    min-width: 0;

    .title-icon {
      font-size: 15px;
      color: var(--td-text-placeholder);
    }

    .title-text {
      font-size: 15px;
      font-weight: 590;
      letter-spacing: -0.015em;
      color: var(--td-text-primary);
      cursor: pointer;
      padding: 2px 6px;
      border-radius: 6px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;

      &:hover {
        background: var(--td-bg-section);
      }
    }

    .unsaved-tag {
      margin-left: 2px;
    }
  }
}

.designer-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.canvas-wrapper {
  flex: 1;
  position: relative;
  overflow: hidden;
}

/* 空画布提示：只说话不挡操作 —— pointer-events: none，拖拽和点击照样落到画布上 */
.canvas-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  pointer-events: none;
  text-align: center;
  padding: 0 24px;
}

.canvas-empty-title {
  font-size: 15px;
  font-weight: 590;
  letter-spacing: -0.01em;
  color: var(--td-text-secondary);
}

.canvas-empty-desc {
  font-size: 12.5px;
  color: var(--td-text-placeholder);
  max-width: 32ch;
  line-height: var(--td-leading-normal);
}

.vue-flow-canvas {
  width: 100%;
  height: 100%;
  background: var(--td-bg-page);
}

/* ── Vue Flow 覆盖 ───────────────────────────────── */
/* 连线粗细与 convertBackendEdgeToFlow 里写的 1.5 对齐，
   否则没走那条路径生成的边会粗一档 */
:deep(.vue-flow__edge-path) {
  stroke: var(--td-border-color-dark);
  stroke-width: 1.5;
}

:deep(.vue-flow__edge.selected .vue-flow__edge-path) {
  stroke: var(--td-color-primary);
  stroke-width: 2;
}

:deep(.vue-flow__connection-line) {
  stroke: var(--td-color-primary);
  stroke-width: 1.5;
  stroke-dasharray: 5;
}

:deep(.vue-flow__handle) {
  width: 9px;
  height: 9px;
  background: var(--td-text-placeholder);
  border: 2px solid var(--td-bg-card);
  border-radius: 50%;
  transition: background-color 150ms ease-out;

  &:hover {
    background: var(--td-color-primary);
  }
}

/* 边标签的背景块由 labelBgStyle 提供（见 convertBackendEdgeToFlow），
   这里只统一字号，避免不同浏览器默认值不一致。 */
:deep(.vue-flow__edge-text) {
  font-size: 11px;
}

:deep(.vue-flow__minimap) {
  border: 1px solid var(--td-border-color);
  border-radius: 8px;
  background: var(--td-bg-card);
  box-shadow: var(--td-elevation-3);
  overflow: hidden;
}

/* 缩略图里的节点块和遮罩是 vue-flow 写死的浅灰，暗色下会变成一块亮斑 */
:deep(.vue-flow__minimap-node) {
  fill: var(--td-border-color-dark);
  stroke: none;
}

:deep(.vue-flow__minimap-mask) {
  /* 遮罩盖的是视口之外的区域，用页面底色半透明压暗，两种主题都成立 */
  fill: var(--td-bg-page);
  fill-opacity: 0.6;
}

:deep(.vue-flow__controls) {
  border: 1px solid var(--td-border-color);
  border-radius: 8px;
  box-shadow: var(--td-elevation-3);
  overflow: hidden;

  button {
    background: var(--td-bg-card);
    border-bottom: 1px solid var(--td-border-color-light);
    color: var(--td-text-secondary);

    /* 控件图标同样是写死的深色 fill，暗色下等于看不见 */
    svg { fill: currentColor; }

    &:hover { background: var(--td-bg-card-hover); }
    &:last-child { border-bottom: none; }
  }
}

@media (prefers-reduced-motion: reduce) {
  :deep(.vue-flow__handle) {
    transition: none;
  }
}
</style>
