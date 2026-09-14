<template>
  <div class="url-field">
    <el-input
      v-model="internalValue"
      :placeholder="field.description || 'https://...'"
      :disabled="disabled"
      :readonly="readonly"
      clearable
      @change="handleChange"
      @blur="validate"
    >
      <template v-if="readonly && isValid" #append>
        <el-button link type="primary" @click="open">{{ t('component.field.open') }}</el-button>
      </template>
    </el-input>
    <div v-if="errorMsg" class="error">{{ errorMsg }}</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed, watch } from 'vue'
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
const errorMsg = ref('')

// 验证是否为合法 http/https URL
const isValid = computed(() => {
  if (!internalValue.value) return true
  try {
    const u = new URL(internalValue.value)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
})

const validate = () => {
  if (!internalValue.value) {
    errorMsg.value = ''
    return
  }
  if (!isValid.value) {
    errorMsg.value = t('component.field.urlInvalid')
  } else {
    errorMsg.value = ''
  }
}

// 在新标签页安全打开链接
const open = () => {
  if (isValid.value) {
    window.open(internalValue.value, '_blank', 'noopener,noreferrer')
  }
}

watch(
  () => props.modelValue,
  (newVal) => {
    internalValue.value = newVal || ''
  },
)

watch(internalValue, (newVal) => {
  emit('update:modelValue', newVal)
})

const handleChange = (value: string) => {
  validate()
  emit('change', value)
}
</script>

<style scoped lang="scss">
.url-field {
  width: 100%;
}

.error {
  color: var(--td-color-danger, var(--td-color-danger));
  font-size: 12px;
  margin-top: 4px;
}
</style>
