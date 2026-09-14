<template>
  <el-select
    v-model="internalValue"
    :placeholder="field.description || `${field.field_name}`"
    :disabled="disabled"
    :readonly="readonly"
    clearable
    filterable
    multiple
    allow-create
    style="width: 100%"
    @change="handleChange"
  >
    <el-option
      v-for="label in labels"
      :key="label.id"
      :label="label.name"
      :value="label.id"
    >
      <div class="label-option">
        <span
          class="label-color"
          :style="{ backgroundColor: label.color || 'var(--td-color-info)' }"
        ></span>
        <span class="label-name">{{ label.name }}</span>
      </div>
    </el-option>
  </el-select>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import type { FieldDefinition, FieldSchemeItem, IssueLabel } from '@/types/field'
import { getLabels } from '@/api/field'

const props = defineProps<{
  field: FieldDefinition
  scheme?: FieldSchemeItem
  projectKey: string
  modelValue?: number[]
  readonly?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number[]): void
  (e: 'change', value: number[]): void
}>()

// 确保值为数组类型
function ensureArray(val: unknown): number[] {
  if (Array.isArray(val)) return val
  return []
}

const internalValue = ref<number[]>(ensureArray(props.modelValue))
const labels = ref<IssueLabel[]>([])

const loadLabels = async () => {
  if (!props.projectKey) return
  try {
    const res = await getLabels(props.projectKey)
    labels.value = res.data.data || []
  } catch {
    // ignored
  }
}

onMounted(() => {
  loadLabels()
})

watch(() => props.projectKey, () => {
  loadLabels()
})

watch(() => props.modelValue, (newVal) => {
  const coerced = ensureArray(newVal)
  if (JSON.stringify(coerced) !== JSON.stringify(internalValue.value)) {
    internalValue.value = coerced
  }
})

watch(internalValue, (newVal) => {
  emit('update:modelValue', newVal)
})

const handleChange = (value: number[]) => {
  emit('change', value)
}
</script>

<style scoped>
.label-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.label-color {
  width: 12px;
  height: 12px;
  border-radius: 2px;
}

.label-name {
  flex: 1;
}
</style>
