<template>
  <el-select
    v-model="internalValue"
    multiple
    collapse-tags
    collapse-tags-tooltip
    :placeholder="field.description || `${field.field_name}`"
    :disabled="disabled"
    :readonly="readonly"
    filterable
    style="width: 100%"
    @change="handleChange"
  >
    <el-option
      v-for="user in users"
      :key="user.id"
      :label="user.display_name || user.username"
      :value="user.id"
    >
      <div class="user-option">
        <div class="user-avatar">{{ (user.display_name || user.username).charAt(0) }}</div>
        <span class="user-name">{{ user.display_name || user.username }}</span>
      </div>
    </el-option>
  </el-select>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import type { FieldDefinition, FieldSchemeItem } from '@/types/field'
import type { UserOption } from '@/types/user'
import { getAllUsers } from '@/api/user'

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

const internalValue = ref<number[]>(props.modelValue || [])
const users = ref<UserOption[]>([])

// 加载所有可选用户
const loadUsers = async () => {
  try {
    const { data } = await getAllUsers()
    users.value = data.data
  } catch {
    // ignored
  }
}

onMounted(() => {
  loadUsers()
})

watch(
  () => props.modelValue,
  (newVal) => {
    internalValue.value = newVal || []
  },
)

watch(internalValue, (newVal) => {
  emit('update:modelValue', newVal)
})

const handleChange = (value: number[]) => {
  emit('change', value)
}
</script>

<style scoped>
.user-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-avatar {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: var(--td-tag-primary-bg);
  color: var(--td-tag-primary-text);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10.5px;
  font-weight: 600;
  flex-shrink: 0;
}

.user-name {
  flex: 1;
}
</style>
