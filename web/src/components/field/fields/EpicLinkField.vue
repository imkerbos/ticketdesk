<template>
  <el-select
    v-model="internalValue"
    :placeholder="field.description || `${field.field_name}`"
    :disabled="disabled"
    :readonly="readonly"
    clearable
    filterable
    remote
    :remote-method="searchEpics"
    :loading="loading"
    style="width: 100%"
    @change="handleChange"
  >
    <el-option
      v-for="epic in epics"
      :key="epic.id"
      :label="`${epic.issue_key} - ${epic.title}`"
      :value="epic.id"
    >
      <div class="epic-option">
        <span class="epic-key">{{ epic.issue_key }}</span>
        <span class="epic-title">{{ epic.title }}</span>
      </div>
    </el-option>
  </el-select>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import type { FieldDefinition, FieldSchemeItem } from '@/types/field'
import { getIssueList } from '@/api/issue'
import { getProjectIssueTypes } from '@/api/project'

interface Epic {
  id: number
  issue_key: string
  title: string
}

const props = defineProps<{
  field: FieldDefinition
  scheme?: FieldSchemeItem
  projectKey: string
  modelValue?: number
  readonly?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number | undefined): void
  (e: 'change', value: number | undefined): void
}>()

const internalValue = ref(props.modelValue)
const epics = ref<Epic[]>([])
const loading = ref(false)
const epicTypeId = ref<number>()

// 获取 Epic 工单类型 ID
const loadEpicTypeId = async () => {
  if (!props.projectKey) return
  try {
    const { data } = await getProjectIssueTypes(props.projectKey)
    const epicType = (data.data || []).find((t: any) => t.name.toLowerCase() === 'epic')
    epicTypeId.value = epicType?.id
  } catch {
    // ignored
  }
}

const loadEpics = async (keyword = '') => {
  if (!props.projectKey || !epicTypeId.value) return
  loading.value = true
  try {
    // 通过 issue_type_id 在后端过滤 Epic 类型工单
    const { data } = await getIssueList({
      project_key: props.projectKey,
      issue_type_id: epicTypeId.value,
      keyword,
      page: 1,
      page_size: 50,
    })
    epics.value = (data.data?.items || []).map((item: any) => ({
      id: item.id,
      issue_key: item.issue_key,
      title: item.title,
    }))
  } catch {
    // ignored
  } finally {
    loading.value = false
  }
}

const searchEpics = (query: string) => {
  loadEpics(query)
}

onMounted(async () => {
  await loadEpicTypeId()
  loadEpics()
})

watch(() => props.projectKey, async () => {
  await loadEpicTypeId()
  loadEpics()
})

watch(() => props.modelValue, (newVal) => {
  internalValue.value = newVal
})

watch(internalValue, (newVal) => {
  emit('update:modelValue', newVal)
})

const handleChange = (value: number | undefined) => {
  emit('change', value)
}
</script>

<style scoped>
.epic-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.epic-key {
  color: var(--td-color-primary);
  font-family: var(--td-font-mono);
  font-weight: 500;
}

.epic-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
