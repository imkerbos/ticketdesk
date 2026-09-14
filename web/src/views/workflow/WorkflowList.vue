<template>
  <!-- 结构同其它列表页 -->
  <div class="page">
    <div class="page-head">
      <h1>{{ t('workflow.listTitle') }}</h1>
      <div class="grow"></div>
      <button class="btn primary" @click="handleCreate">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
        {{ t('workflow.create') }}
      </button>
    </div>

    <section class="card">
      <div v-loading="loading" class="table-wrap">
        <table v-if="workflows.length > 0" class="issues">
          <colgroup>
            <col style="width: 240px" /><col /><col style="width: 140px" /><col style="width: 86px" /><col style="width: 150px" />
          </colgroup>
          <thead>
            <tr>
              <th>{{ t('workflow.name') }}</th>
              <th>{{ t('issue.description') }}</th>
              <th>{{ t('workflow.project') }}</th>
              <th>{{ t('issue.status') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="workflow in workflows" :key="workflow.id" @click="handleDesign(workflow)">
              <td>{{ workflow.name }}</td>
              <td class="muted desc">{{ workflow.description || '-' }}</td>
              <!-- 原来直接渲染 project_id，表里出现的是裸的「1001」，看不出是哪个项目 -->
              <td class="muted">{{ projectLabel(workflow.project_id) }}</td>
              <td>
                <span class="pill" :class="workflow.status === 1 ? 'green' : 'neutral'">
                  {{ workflow.status === 1 ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
              <td>
                <div class="row-actions">
                  <button class="link-btn" @click.stop="handleDesign(workflow)">{{ t('workflow.design') }}</button>
                  <el-dropdown trigger="click">
                    <button class="more" :aria-label="t('common.operation')" @click.stop>···</button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleViewNodes(workflow)">{{ t('workflow.viewNodes') }}</el-dropdown-item>
                        <el-dropdown-item @click="handleEdit(workflow)">{{ t('common.edit') }}</el-dropdown-item>
                        <el-dropdown-item divided @click="handleDelete(workflow)">{{ t('common.delete') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="!loading && workflows.length === 0" class="empty">
          <TdEmptyState preset="no-data" :title="t('workflow.empty')" />
        </div>
      </div>
    </section>

    <!-- 创建/编辑工作流对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEditMode ? t('workflow.edit') : t('workflow.create')" width="600px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top">
        <el-form-item :label="t('workflow.name')" prop="name">
          <el-input v-model="form.name" :placeholder="t('workflow.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('issue.description')">
          <el-input v-model="form.description" type="textarea" :rows="3" :placeholder="t('workflow.descPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('workflow.project')">
          <el-select v-model="form.project_id" :placeholder="t('workflow.projectPlaceholder')" clearable style="width: 100%">
            <el-option v-for="p in projects" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('issue.status')">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" :active-text="t('common.enabled')" :inactive-text="t('common.disabled')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submitForm">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 节点查看对话框 -->
    <el-dialog v-model="nodesDialogVisible" :title="t('workflow.nodesTitle', { name: currentWorkflow?.name })" width="900px" destroy-on-close>
      <div v-loading="nodesLoading" class="nodes-content">
        <!-- 节点管理 -->
        <div class="section-header">
          <h4>{{ t('workflow.nodeList') }}</h4>
          <el-button type="primary" size="small" @click="handleAddNode">
            <el-icon><Plus /></el-icon>
            {{ t('workflow.addNode') }}
          </el-button>
        </div>
        <div class="nodes-list">
          <div v-for="node in workflowNodes" :key="node.id" class="node-item">
            <div class="node-info">
              <div class="node-icon" :class="node.node_type">
                <el-icon v-if="node.node_type === 'start'"><VideoPlay /></el-icon>
                <el-icon v-else-if="node.node_type === 'end'"><CircleCheck /></el-icon>
                <el-icon v-else-if="node.node_type === 'approval'"><Checked /></el-icon>
                <el-icon v-else-if="node.node_type === 'system'"><Setting /></el-icon>
                <el-icon v-else><Operation /></el-icon>
              </div>
              <div class="node-details">
                <div class="node-title-row">
                  <span class="node-name">{{ node.name }}</span>
                  <el-tag size="small">{{ getNodeTypeText(node.node_type) }}</el-tag>
                  <el-tag v-if="node.config?.approval_type" size="small" type="warning">{{ getApprovalTypeText(node.config.approval_type) }}</el-tag>
                  <el-tag v-if="node.config?.assignee_type" size="small" type="success">{{ getAssigneeTypeText(node.config.assignee_type) }}</el-tag>
                </div>
                <p v-if="node.config?.description" class="node-desc">{{ node.config.description }}</p>
              </div>
            </div>
            <div class="node-actions">
              <el-button size="small" @click="handleEditNode(node)">{{ t('common.edit') }}</el-button>
              <el-button size="small" type="danger" :disabled="node.node_type === 'start' || node.node_type === 'end'" @click="handleDeleteNode(node)">{{ t('common.delete') }}</el-button>
            </div>
          </div>
          <TdEmptyState v-if="!nodesLoading && workflowNodes.length === 0" preset="no-data" :title="t('workflow.noNodes')" />
        </div>

        <!-- 边管理 -->
        <div class="section-header" style="margin-top: 24px;">
          <h4>{{ t('workflow.edgeList') }}</h4>
          <el-button type="primary" size="small" :disabled="workflowNodes.length < 2" @click="handleAddEdge">
            <el-icon><Plus /></el-icon>
            {{ t('workflow.addEdge') }}
          </el-button>
        </div>
        <div v-loading="edgesLoading" class="edges-list">
          <div v-for="edge in workflowEdges" :key="edge.id" class="edge-item">
            <div class="edge-info">
              <span class="edge-node">{{ getNodeName(edge.source_node_id) }}</span>
              <el-icon class="edge-arrow"><Right /></el-icon>
              <span class="edge-node">{{ getNodeName(edge.target_node_id) }}</span>
              <el-tag v-if="edge.condition_expr" size="small" type="info" class="edge-condition">{{ edge.condition_expr }}</el-tag>
            </div>
            <div class="edge-actions">
              <el-button size="small" type="danger" @click="handleDeleteEdge(edge)">{{ t('common.delete') }}</el-button>
            </div>
          </div>
          <TdEmptyState v-if="!edgesLoading && workflowEdges.length === 0" preset="no-data" :title="t('workflow.noEdges')" />
        </div>
      </div>
    </el-dialog>

    <!-- 添加/编辑节点对话框 -->
    <el-dialog v-model="nodeDialogVisible" :title="isEditNodeMode ? t('workflow.editNode') : t('workflow.addNode')" width="600px" destroy-on-close>
      <el-form ref="nodeFormRef" :model="nodeForm" :rules="nodeFormRules" label-position="top">
        <el-form-item :label="t('workflow.nodeName')" prop="name">
          <el-input v-model="nodeForm.name" :placeholder="t('workflow.nodeNamePlaceholder')" />
        </el-form-item>
        <el-form-item v-if="!isEditNodeMode" :label="t('workflow.nodeType')" prop="node_type">
          <el-select v-model="nodeForm.node_type" :placeholder="t('workflow.nodeTypePlaceholder')" style="width: 100%" @change="handleNodeTypeChange">
            <el-option :label="t('workflow.nodeTypeFullMap.approval')" value="approval" />
            <el-option :label="t('workflow.nodeTypeFullMap.work')" value="work" />
            <el-option :label="t('workflow.nodeTypeFullMap.system')" value="system" />
          </el-select>
        </el-form-item>
        <el-form-item v-else :label="t('workflow.nodeType')">
          <el-tag>{{ getNodeTypeText(nodeForm.node_type) }}</el-tag>
        </el-form-item>

        <!-- 审批节点配置 -->
        <template v-if="nodeForm.node_type === 'approval'">
          <el-form-item :label="t('workflow.approvalType')">
            <el-select v-model="nodeForm.config.approval_type" :placeholder="t('workflow.approvalTypePlaceholder')" style="width: 100%">
              <el-option :label="t('workflow.approvalTypeMap.single')" value="single" />
              <el-option :label="t('workflow.approvalTypeOption.countersign')" value="countersign" />
              <el-option :label="t('workflow.approvalTypeOption.or_sign')" value="or_sign" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('workflow.approvers')">
            <el-select v-model="nodeForm.config.approvers" multiple :placeholder="t('workflow.approversPlaceholder')" style="width: 100%" filterable>
              <el-option v-for="u in allUsers" :key="u.id" :label="u.display_name" :value="u.id" />
            </el-select>
          </el-form-item>
        </template>

        <!-- 工作节点配置 -->
        <template v-if="nodeForm.node_type === 'work'">
          <el-form-item :label="t('workflow.assigneeType')">
            <el-select v-model="nodeForm.config.assignee_type" :placeholder="t('workflow.assigneeTypePlaceholder')" style="width: 100%">
              <el-option :label="t('workflow.assigneeTypeMap.user')" value="user" />
              <el-option :label="t('workflow.assigneeTypeMap.role')" value="role" />
              <el-option :label="t('workflow.assigneeTypeMap.reporter')" value="reporter" />
              <el-option :label="t('workflow.assigneeTypeMap.project_lead')" value="project_lead" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="nodeForm.config.assignee_type === 'user'" :label="t('workflow.assignees')">
            <el-select v-model="nodeForm.config.assignees" multiple :placeholder="t('workflow.assigneesPlaceholder')" style="width: 100%" filterable>
              <el-option v-for="u in allUsers" :key="u.id" :label="u.display_name" :value="u.id" />
            </el-select>
          </el-form-item>
          <el-form-item v-if="nodeForm.config.assignee_type === 'role'" :label="t('workflow.assigneeRole')">
            <el-input v-model="nodeForm.config.assignee_role" :placeholder="t('workflow.assigneeRolePlaceholder')" />
          </el-form-item>
        </template>

        <!-- 系统节点配置 -->
        <template v-if="nodeForm.node_type === 'system'">
          <el-form-item :label="t('workflow.systemAction')">
            <el-input v-model="nodeForm.config.action" :placeholder="t('workflow.actionPlaceholder')" />
          </el-form-item>
        </template>

        <!-- 通用配置 -->
        <el-form-item :label="t('workflow.timeout')">
          <el-input-number v-model="nodeForm.config.timeout_hours" :min="0" :max="720" :placeholder="t('workflow.timeoutPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('workflow.nodeDesc')">
          <el-input v-model="nodeForm.config.description" type="textarea" :rows="2" :placeholder="t('workflow.nodeDesc')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="nodeDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="nodeSubmitLoading" @click="submitNodeForm">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 添加边对话框 -->
    <el-dialog v-model="edgeDialogVisible" :title="t('workflow.addEdge')" width="500px" destroy-on-close>
      <el-form ref="edgeFormRef" :model="edgeForm" :rules="edgeFormRules" label-position="top">
        <el-form-item :label="t('workflow.sourceNode')" prop="source_node_id">
          <el-select v-model="edgeForm.source_node_id" :placeholder="t('workflow.sourcePlaceholder')" style="width: 100%">
            <el-option v-for="node in workflowNodes" :key="node.id" :label="`${node.name} (${getNodeTypeText(node.node_type)})`" :value="node.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('workflow.targetNode')" prop="target_node_id">
          <el-select v-model="edgeForm.target_node_id" :placeholder="t('workflow.targetPlaceholder')" style="width: 100%">
            <el-option v-for="node in workflowNodes" :key="node.id" :label="`${node.name} (${getNodeTypeText(node.node_type)})`" :value="node.id" :disabled="node.id === edgeForm.source_node_id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('workflow.conditionExpr')">
          <el-input v-model="edgeForm.condition_expr" :placeholder="t('workflow.conditionExprPlaceholder')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="edgeDialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="edgeSubmitLoading" @click="submitEdgeForm">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  Plus,
  VideoPlay,
  CircleCheck,
  Checked,
  Operation,
  Setting,
  Right } from '@element-plus/icons-vue'
import { getAllProjects } from '@/api/project'
import { getAllUsers } from '@/api/user'
import type { Project } from '@/types/project'
import type { UserOption } from '@/types/user'
import {
  getWorkflowList,
  createWorkflow,
  updateWorkflow,
  deleteWorkflow,
  getWorkflowNodes,
  createNode,
  updateNode,
  deleteNode,
  getWorkflowEdges,
  createEdge,
  deleteEdge } from '@/api/workflow'
import type { Workflow, WorkflowNode, WorkflowEdge, NodeConfig } from '@/types/workflow'

const { t } = useI18n()

const loading = ref(false)
const workflows = ref<Workflow[]>([])
const projects = ref<Project[]>([])
const router = useRouter()
// 工作流表单
const dialogVisible = ref(false)
const isEditMode = ref(false)
const editingId = ref<number | null>(null)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
// 列表里展示项目名：projects 本来就为弹窗的下拉加载过，直接复用。
// 项目还没加载完或已被删除时退回编号，至少不会显示成空白。
const projectLabel = (projectId?: number) => {
  if (!projectId) return t('workflow.global')
  return projects.value.find((p) => p.id === projectId)?.name || `#${projectId}`
}

const form = reactive({
  name: '',
  description: '',
  project_id: undefined as number | undefined,
  status: 1 })
const formRules: FormRules = {
  name: [{ required: true, message: t('workflow.nameRequired'), trigger: ['blur', 'change'] }] }

// 节点管理
const nodesDialogVisible = ref(false)
const currentWorkflow = ref<Workflow | null>(null)
const workflowNodes = ref<WorkflowNode[]>([])
const nodesLoading = ref(false)
const allUsers = ref<UserOption[]>([])

// 边管理
const workflowEdges = ref<WorkflowEdge[]>([])
const edgesLoading = ref(false)

// 节点表单
const nodeDialogVisible = ref(false)
const isEditNodeMode = ref(false)
const editingNodeId = ref<number | null>(null)
const nodeSubmitLoading = ref(false)
const nodeFormRef = ref<FormInstance>()

const defaultNodeConfig = (): NodeConfig => ({
  approval_type: undefined,
  approvers: [],
  approver_role: '',
  assignee_type: undefined,
  assignees: [],
  assignee_role: '',
  action: '',
  parameters: {},
  timeout_hours: 0,
  description: '' })

const nodeForm = reactive({
  name: '',
  node_type: '' as string,
  config: defaultNodeConfig() })
const nodeFormRules: FormRules = {
  name: [{ required: true, message: t('workflow.nodeNameRequired'), trigger: ['blur', 'change'] }],
  node_type: [{ required: true, message: t('workflow.nodeTypeRequired'), trigger: 'change' }] }

// 边表单
const edgeDialogVisible = ref(false)
const edgeSubmitLoading = ref(false)
const edgeFormRef = ref<FormInstance>()
const edgeForm = reactive({
  source_node_id: undefined as number | undefined,
  target_node_id: undefined as number | undefined,
  condition_expr: '' })
const edgeFormRules: FormRules = {
  source_node_id: [{ required: true, message: t('workflow.sourceRequired'), trigger: 'change' }],
  target_node_id: [{ required: true, message: t('workflow.targetRequired'), trigger: 'change' }] }

const loadWorkflows = async () => {
  loading.value = true
  try {
    const { data } = await getWorkflowList()
    workflows.value = (data as any).data.items || []
  } catch {
    ElMessage.error(t('workflow.loadListFailed'))
  } finally {
    loading.value = false
  }
}

const loadProjects = async () => {
  try {
    const { data } = await getAllProjects()
    projects.value = data.data
  } catch {
    // ignored
  }
}

const handleDesign = (workflow: Workflow) => {
  router.push(`/workflows/${workflow.id}/designer`)
}

const handleCreate = () => {
  isEditMode.value = false
  editingId.value = null
  Object.assign(form, { name: '', description: '', project_id: undefined, status: 1 })
  dialogVisible.value = true
}

const handleEdit = (workflow: Workflow) => {
  isEditMode.value = true
  editingId.value = workflow.id
  Object.assign(form, {
    name: workflow.name,
    description: workflow.description,
    project_id: workflow.project_id,
    status: workflow.status })
  dialogVisible.value = true
}

const submitForm = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      if (isEditMode.value && editingId.value) {
        await updateWorkflow(editingId.value, {
          name: form.name,
          description: form.description,
          status: form.status })
        ElMessage.success(t('issue.msg.updateSuccess'))
      } else {
        await createWorkflow({
          name: form.name,
          description: form.description,
          project_id: form.project_id })
        ElMessage.success(t('common.createSuccess'))
      }
      dialogVisible.value = false
      loadWorkflows()
    } catch {
      ElMessage.error(isEditMode.value ? t('project.settings.updateFailed') : t('project.settings.createFailed'))
    } finally {
      submitLoading.value = false
    }
  })
}

const handleDelete = async (workflow: Workflow) => {
  try {
    await ElMessageBox.confirm(t('workflow.confirmDelete', { name: workflow.name }), t('issue.list.deleteTitle'), {
      type: 'warning' })
    // TODO: 调用实际的删除 API
    await deleteWorkflow(workflow.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    loadWorkflows()
  } catch (error) {
    if (error !== 'cancel') {
      // ignored
    }
  }
}

const handleViewNodes = async (workflow: Workflow) => {
  currentWorkflow.value = workflow
  nodesDialogVisible.value = true
  await Promise.all([
    loadNodes(workflow.id),
    loadEdges(workflow.id),
    loadAllUsers(),
  ])
}

const loadNodes = async (workflowId: number) => {
  nodesLoading.value = true
  try {
    const { data } = await getWorkflowNodes(workflowId)
    workflowNodes.value = (data as any).data || []
  } catch {
    ElMessage.error(t('workflow.loadNodesFailed'))
  } finally {
    nodesLoading.value = false
  }
}

const loadEdges = async (workflowId: number) => {
  edgesLoading.value = true
  try {
    const { data } = await getWorkflowEdges(workflowId)
    workflowEdges.value = (data as any).data || []
  } catch {
    ElMessage.error(t('workflow.loadEdgesFailed'))
  } finally {
    edgesLoading.value = false
  }
}

const loadAllUsers = async () => {
  if (allUsers.value.length > 0) return
  try {
    const { data } = await getAllUsers()
    allUsers.value = data.data || []
  } catch {
    // ignored
  }
}

const handleNodeTypeChange = () => {
  // 切换节点类型时重置配置
  Object.assign(nodeForm.config, defaultNodeConfig())
}

const handleAddNode = () => {
  isEditNodeMode.value = false
  editingNodeId.value = null
  nodeForm.name = ''
  nodeForm.node_type = ''
  Object.assign(nodeForm.config, defaultNodeConfig())
  nodeDialogVisible.value = true
}

const handleEditNode = (node: WorkflowNode) => {
  isEditNodeMode.value = true
  editingNodeId.value = node.id
  nodeForm.name = node.name
  nodeForm.node_type = node.node_type
  Object.assign(nodeForm.config, defaultNodeConfig(), node.config || {})
  nodeDialogVisible.value = true
}

const submitNodeForm = async () => {
  if (!nodeFormRef.value || !currentWorkflow.value) return
  await nodeFormRef.value.validate(async (valid) => {
    if (!valid) return
    nodeSubmitLoading.value = true
    try {
      if (isEditNodeMode.value && editingNodeId.value) {
        await updateNode(currentWorkflow.value!.id, editingNodeId.value, {
          name: nodeForm.name,
          config: nodeForm.config })
        ElMessage.success(t('workflow.nodeUpdated'))
      } else {
        await createNode(currentWorkflow.value!.id, {
          name: nodeForm.name,
          node_type: nodeForm.node_type as any,
          config: nodeForm.config })
        ElMessage.success(t('workflow.nodeCreated'))
      }
      nodeDialogVisible.value = false
      await loadNodes(currentWorkflow.value!.id)
    } catch {
      ElMessage.error(isEditNodeMode.value ? t('workflow.nodeUpdateFailed') : t('workflow.nodeCreateFailed'))
    } finally {
      nodeSubmitLoading.value = false
    }
  })
}

const handleDeleteNode = async (node: WorkflowNode) => {
  if (!currentWorkflow.value) return
  try {
    await ElMessageBox.confirm(t('workflow.confirmDeleteNode', { name: node.name }), t('issue.list.deleteTitle'), {
      type: 'warning' })
    await deleteNode(currentWorkflow.value.id, node.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    await Promise.all([
      loadNodes(currentWorkflow.value.id),
      loadEdges(currentWorkflow.value.id),
    ])
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('workflow.deleteNodeFailed'))
    }
  }
}

// 边管理
const handleAddEdge = () => {
  edgeForm.source_node_id = undefined
  edgeForm.target_node_id = undefined
  edgeForm.condition_expr = ''
  edgeDialogVisible.value = true
}

const submitEdgeForm = async () => {
  if (!edgeFormRef.value || !currentWorkflow.value) return
  await edgeFormRef.value.validate(async (valid) => {
    if (!valid) return
    edgeSubmitLoading.value = true
    try {
      await createEdge(currentWorkflow.value!.id, {
        source_node_id: edgeForm.source_node_id!,
        target_node_id: edgeForm.target_node_id!,
        condition_expr: edgeForm.condition_expr || undefined })
      ElMessage.success(t('workflow.edgeCreated'))
      edgeDialogVisible.value = false
      await loadEdges(currentWorkflow.value!.id)
    } catch {
      ElMessage.error(t('workflow.edgeCreateFailed'))
    } finally {
      edgeSubmitLoading.value = false
    }
  })
}

const handleDeleteEdge = async (edge: WorkflowEdge) => {
  if (!currentWorkflow.value) return
  try {
    await ElMessageBox.confirm(t('workflow.confirmDeleteEdge'), t('issue.list.deleteTitle'), {
      type: 'warning' })
    await deleteEdge(currentWorkflow.value.id, edge.id)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    await loadEdges(currentWorkflow.value.id)
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error(t('workflow.deleteEdgeFailed'))
    }
  }
}

const getNodeName = (nodeId: number) => {
  const node = workflowNodes.value.find(n => n.id === nodeId)
  return node ? node.name : t('workflow.nodeFallback', { id: nodeId })
}

const getNodeTypeText = (type: string) => {
  const known = ['start', 'end', 'approval', 'work', 'system']
  return known.includes(type) ? t(`workflow.nodeTypeMap.${type}`) : type
}

const getApprovalTypeText = (type: string) => {
  const known = ['single', 'countersign', 'or_sign']
  return known.includes(type) ? t(`workflow.approvalTypeMap.${type}`) : type
}

const getAssigneeTypeText = (type: string) => {
  const known = ['user', 'role', 'reporter', 'project_lead']
  return known.includes(type) ? t(`workflow.assigneeTypeMap.${type}`) : type
}

onMounted(() => {
  loadWorkflows()
  loadProjects()
})
</script>

<style scoped lang="scss">
// 列表样式在 _apple.scss 里。

.desc { max-width: 0; overflow: hidden; text-overflow: ellipsis; }

.nodes-content {
  min-height: 300px;
}

.nodes-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.node-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: var(--td-bg-page);
  border-radius: 8px;

  &:hover {
    background: var(--td-bg-section);
  }
}

.node-info {
  display: flex;
  gap: 12px;
  flex: 1;
}

/* 只给图标上色，不套实心方块 —— 和工作流设计器里的节点保持一套 */
.node-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;

  &.start { color: var(--td-color-success); }
  &.end { color: var(--td-text-secondary); }
  &.approval { color: var(--td-tag-orange-text); }
  &.work { color: var(--td-color-primary); }
  &.system { color: var(--td-tag-purple-text); }
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;

  h4 {
    font-size: 15px;
    font-weight: 600;
    color: var(--td-text-primary);
    margin: 0;
  }
}

// 边列表
.edges-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.edge-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: var(--td-bg-page);
  border-radius: 8px;

  &:hover {
    background: var(--td-bg-section);
  }
}

.edge-info {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 1;

  .edge-node {
    font-size: 14px;
    font-weight: 500;
    color: var(--td-text-primary);
    padding: 4px 10px;
    background: var(--td-border-color);
    border-radius: 6px;
  }

  .edge-arrow {
    color: var(--td-text-placeholder);
    font-size: 16px;
  }

  .edge-condition {
    margin-left: 8px;
  }
}

.edge-actions {
  display: flex;
  gap: 8px;
}

.node-details {
  flex: 1;

  .node-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 4px;
  }

  .node-name {
    font-size: 14px;
    font-weight: 500;
    color: var(--td-text-primary);
  }

  .node-desc {
    font-size: 12px;
    color: var(--td-text-placeholder);
    margin: 0;
  }
}

.node-actions {
  display: flex;
  gap: 8px;
}
</style>
