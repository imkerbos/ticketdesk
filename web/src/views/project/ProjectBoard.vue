<template>
  <div class="page project-board-container">
    <!-- 页头和项目其它页一致：标题 + 项目内 tab -->
    <div class="page-head">
      <div class="detail-title">
        <h1>{{ project?.name || projectKey }}</h1>
        <div class="detail-meta"><span class="key static">{{ projectKey }}</span></div>
      </div>
      <div class="grow"></div>
      <button class="btn secondary" @click="$router.push('/projects')">{{ t('project.roles.back') }}</button>
    </div>

    <div class="tabs" role="tablist">
      <button class="tab" role="tab" aria-selected="false" @click="$router.push(`/projects/${projectKey}`)">{{ t('project.overview') }}</button>
      <button class="tab" role="tab" aria-selected="true">{{ t('project.board') }}</button>
      <button class="tab" role="tab" aria-selected="false" @click="$router.push(`/projects/${projectKey}/settings`)">{{ t('project.settingsLabel') }}</button>
    </div>

    <!-- 分栏主体 -->
    <div class="board-body">
      <!-- 左侧工单列表 -->
      <div class="board-left">
        <BoardIssueList
          ref="boardListRef"
          :project-key="projectKey"
          :selected-key="selectedIssueKey"
          :initial-status="initialStatus"
          @select="handleSelectIssue"
          @create="handleCreateIssue"
        />
      </div>

      <!-- 右侧详情 -->
      <div class="board-right">
        <IssueDetail
          v-if="selectedIssueKey"
          :key="selectedIssueKey"
          embedded
          :issue-key="selectedIssueKey"
          :on-navigate-issue="handleNavigateIssue"
          :on-deleted="handleDeleted"
        />
        <div v-else class="empty-detail">
          <TdEmptyState
            preset="first-time"
            tone="primary"
            :title="t('project.pickIssue')"
            :description="t('project.pickIssueDesc')"
          >
            <!-- 左栏顶部已有一颗实心「+创建」，同一个动作不再来第二颗实心 -->
            <el-button @click="handleCreateIssue">
              <el-icon><Plus /></el-icon>{{ t('issue.createIssue') }}
            </el-button>
          </TdEmptyState>
        </div>
      </div>
    </div>

    <!-- 创建工单对话框 -->
    <CreateIssueDialog
      v-model="createDialogVisible"
      :fixed-project-key="projectKey"
      @created="handleIssueCreated"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import BoardIssueList from './components/BoardIssueList.vue'
import IssueDetail from '@/views/issue/IssueDetail.vue'
import { getProjectDetail } from '@/api/project'
import type { Project } from '@/types/project'
import CreateIssueDialog from '@/components/CreateIssueDialog.vue'

const { t } = useI18n()

const route = useRoute()
const router = useRouter()

const projectKey = computed(() => route.params.key as string)
const initialStatus = computed(() => (route.query.status as string) || '')
const selectedIssueKey = ref('')
const project = ref<Project | null>(null)
const boardListRef = ref<InstanceType<typeof BoardIssueList> | null>(null)

// 创建工单
const createDialogVisible = ref(false)

// 初始化选中工单
const initSelectedKey = () => {
  const issueKey = route.params.issueKey as string
  selectedIssueKey.value = issueKey || ''
}

const loadProject = async () => {
  try {
    const { data } = await getProjectDetail(projectKey.value)
    project.value = data.data
  } catch {
    // ignored
  }
}

const handleSelectIssue = (issueKey: string) => {
  selectedIssueKey.value = issueKey
  router.replace(`/projects/${projectKey.value}/board/${issueKey}`)
}

// 创建工单后: 刷新列表并选中新工单
const handleIssueCreated = async (issueKey: string) => {
  await boardListRef.value?.reload()
  handleSelectIssue(issueKey)
}

const handleNavigateIssue = (issueKey: string) => {
  selectedIssueKey.value = issueKey
  router.replace(`/projects/${projectKey.value}/board/${issueKey}`)
}

const handleDeleted = () => {
  selectedIssueKey.value = ''
  router.replace(`/projects/${projectKey.value}/board`)
}

const handleCreateIssue = () => {
  createDialogVisible.value = true
}

// 浏览器前进/后退
watch(
  () => route.params.issueKey,
  (newKey) => {
    const key = newKey as string
    if (key !== selectedIssueKey.value) {
      selectedIssueKey.value = key || ''
    }
  }
)

onMounted(() => {
  initSelectedKey()
  loadProject()
})
</script>

<style scoped lang="scss">
/* 吃掉 .ap .content 的 20/24/24，高度按 52px 顶栏算 —— 和工作流设计器同一套。
   原先写的 100vh - 64px + margin -24px 是旧 shell 的尺寸，底部会被切掉一截。 */
.project-board-container {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 52px);
  width: calc(100% + var(--td-content-pad-x) * 2);
  /* 抵消 .content 的内边距做满幅。跟着变量走 —— 写死 24px 的话，
     窄屏下 .content 变成 16px，两边各多撑 8px，页面主体就横滚了。 */
  margin: calc(var(--td-content-pad-top) * -1) calc(var(--td-content-pad-x) * -1)
    calc(var(--td-content-pad-bottom) * -1);
  padding: 20px 24px 0;
  background: var(--td-bg-page);
  overflow: hidden;
}

.board-body {
  display: flex;
  flex: 1;
  min-height: 0;
  margin: 0 -24px;
  border-top: 1px solid var(--td-border-color);
  overflow: hidden;
}

.board-left {
  width: clamp(320px, 26vw, 460px);
  border-right: 1px solid var(--td-border-color);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-card);
}

.board-right {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  background: var(--td-bg-page);
}

/* 空态靠上放，视线不用往下扫整屏 */
.empty-detail {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  height: 100%;
  padding-top: 12vh;
}
</style>
