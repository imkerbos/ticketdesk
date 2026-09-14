<template>
  <el-input
    v-model="internalValue"
    :placeholder="field.description || t('component.field.timeEstimatePlaceholder')"
    :disabled="disabled"
    :readonly="readonly"
    clearable
    @change="handleChange"
  >
    <template #suffix>
      <el-tooltip
        :content="t('component.field.timeEstimateTip')"
        placement="top"
      >
        <el-icon><QuestionFilled /></el-icon>
      </el-tooltip>
    </template>
  </el-input>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, watch } from 'vue'
import { QuestionFilled } from '@element-plus/icons-vue'
import type { FieldDefinition, FieldSchemeItem } from '@/types/field'

const { t } = useI18n()

const props = defineProps<{
  field: FieldDefinition
  scheme?: FieldSchemeItem
  projectKey: string
  modelValue?: string
  readonly?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'change', value: string): void
}>()

const internalValue = ref(props.modelValue || '')

watch(() => props.modelValue, (newVal) => {
  internalValue.value = newVal || ''
})

watch(internalValue, (newVal) => {
  emit('update:modelValue', newVal)
})

const handleChange = (value: string) => {
  emit('change', value)
}
</script>
