<template>
  <el-switch
    v-model="internalValue"
    :disabled="disabled || readonly"
    :active-text="t('component.field.yes')"
    :inactive-text="t('component.field.no')"
    @change="handleChange"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, watch } from 'vue'
import type { FieldDefinition, FieldSchemeItem } from '@/types/field'

const { t } = useI18n()

const props = defineProps<{
  field: FieldDefinition
  scheme?: FieldSchemeItem
  projectKey: string
  modelValue?: boolean
  readonly?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'change', value: boolean): void
}>()

const internalValue = ref<boolean>(!!props.modelValue)

watch(
  () => props.modelValue,
  (newVal) => {
    internalValue.value = !!newVal
  },
)

watch(internalValue, (newVal) => {
  emit('update:modelValue', newVal)
})

// el-switch 的 @change 类型签名是 (string | number | boolean), 这里强制布尔语义
const handleChange = (value: string | number | boolean) => {
  emit('change', Boolean(value))
}
</script>
