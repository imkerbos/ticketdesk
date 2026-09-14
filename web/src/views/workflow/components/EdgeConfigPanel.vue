<template>
  <el-dialog v-model="visible" :title="t('workflow.edge.title')" width="450px" destroy-on-close @close="$emit('close')">
    <el-form label-position="top">
      <el-form-item :label="t('workflow.sourceNode')">
        <el-input :model-value="sourceNodeName" disabled />
      </el-form-item>
      <el-form-item :label="t('workflow.targetNode')">
        <el-input :model-value="targetNodeName" disabled />
      </el-form-item>

      <!-- 流转条件：统一使用预设 + 自定义 -->
      <el-form-item :label="t('workflow.edge.condition')">
        <!-- 条件模式选择 -->
        <el-radio-group v-model="conditionMode" style="margin-bottom: 8px; width: 100%">
          <el-radio-button value="none">{{ t('workflow.edge.none') }}</el-radio-button>
          <el-radio-button value="preset">{{ t('workflow.edge.preset') }}</el-radio-button>
          <el-radio-button value="custom">{{ t('workflow.edge.custom') }}</el-radio-button>
        </el-radio-group>

        <!-- 预设条件选择 -->
        <el-select
          v-if="conditionMode === 'preset'"
          v-model="localCondition"
          :placeholder="t('workflow.edge.selectCondition')"
          style="width: 100%"
        >
          <el-option-group :label="t('workflow.edge.commonGroup')">
            <el-option :label="t('workflow.edge.approvedOption')" value="approved" />
            <el-option :label="t('workflow.edge.rejectedOption')" value="rejected" />
          </el-option-group>
        </el-select>

        <!-- 自定义条件输入 -->
        <el-input
          v-if="conditionMode === 'custom'"
          v-model="localCondition"
          :placeholder="t('workflow.edge.customPlaceholder')"
          clearable
        />

        <!-- 提示信息 -->
        <div class="form-hint">
          <template v-if="conditionMode === 'none'">
            {{ t('workflow.edge.noneHint', { name: targetNodeName }) }}
          </template>
          <template v-else-if="conditionMode === 'preset' || conditionMode === 'custom'">
            <template v-if="localCondition">
              {{ t('workflow.edge.condHint', { cond: conditionDisplayText, name: targetNodeName }) }}
            </template>
            <template v-else>
              {{ t('workflow.edge.setCondition') }}
            </template>
          </template>
        </div>
      </el-form-item>

      <!-- 显示标签 -->
      <el-form-item :label="t('workflow.edge.label')">
        <el-input
          v-model="localLabel"
          :placeholder="t('workflow.edge.labelPlaceholder')"
          clearable
        />
        <div class="form-hint">{{ t('workflow.edge.labelHint') }}</div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="danger" plain @click="handleDelete">{{ t('workflow.edge.deleteEdge') }}</el-button>
      <el-button type="primary" @click="handleSave">{{ t('common.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, watch, computed } from 'vue'

const { t } = useI18n()

interface FlowEdge {
  id: string
  source: string
  target: string
  label?: string
  data?: {
    conditionExpr?: string
    backendId?: number
  }
}

interface FlowNode {
  id: string
  data: {
    label: string
    nodeType?: string
    [key: string]: any
  }
  [key: string]: any
}

const props = defineProps<{
  edge: FlowEdge | null
  nodes: FlowNode[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update', payload: { id: string; conditionExpr: string; label: string }): void
  (e: 'delete', id: string): void
}>()

const visible = ref(false)
const localCondition = ref('')
const localLabel = ref('')
const conditionMode = ref<'none' | 'preset' | 'custom'>('none')

// 预设条件的显示名称映射
const presetDisplayNames = (): Record<string, string> => ({
  approved: t('workflow.edge.approved'),
  rejected: t('workflow.edge.rejected'),
})

// 判断条件是否是预设条件
const isPresetCondition = (condition: string) => {
  return condition === 'approved' || condition === 'rejected'
}

// 条件的显示文本
const conditionDisplayText = computed(() => {
  if (!localCondition.value) return ''
  return presetDisplayNames()[localCondition.value] || localCondition.value
})

watch(
  () => props.edge,
  (newEdge) => {
    if (newEdge) {
      visible.value = true
      const condition = newEdge.data?.conditionExpr || ''
      localCondition.value = condition
      localLabel.value = (newEdge.label as string) || ''

      // 根据已有条件确定模式
      if (!condition) {
        conditionMode.value = 'none'
      } else if (isPresetCondition(condition)) {
        conditionMode.value = 'preset'
      } else {
        conditionMode.value = 'custom'
      }
    } else {
      visible.value = false
    }
  },
  { immediate: true }
)

// 当模式切换时，清空条件
watch(conditionMode, (newMode, oldMode) => {
  if (newMode === 'none') {
    localCondition.value = ''
  } else if (newMode === 'preset' && oldMode !== 'preset') {
    // 如果从其他模式切到预设，且当前值不是预设值，清空
    if (!isPresetCondition(localCondition.value)) {
      localCondition.value = ''
    }
  } else if (newMode === 'custom' && oldMode !== 'custom') {
    // 如果从预设切到自定义，且当前值是预设值，清空让用户输入
    if (isPresetCondition(localCondition.value)) {
      localCondition.value = ''
    }
  }
})

const sourceNodeName = computed(() => {
  if (!props.edge) return ''
  const node = props.nodes.find(n => n.id === props.edge!.source)
  return node?.data.label || props.edge.source
})

const targetNodeName = computed(() => {
  if (!props.edge) return ''
  const node = props.nodes.find(n => n.id === props.edge!.target)
  return node?.data.label || props.edge.target
})

const handleSave = () => {
  if (!props.edge) return

  const condition = conditionMode.value === 'none' ? '' : localCondition.value

  // 自动生成标签：如果用户没有填标签，使用条件的显示名称
  let label = localLabel.value
  if (!label && condition) {
    label = presetDisplayNames()[condition] || condition
  }

  emit('update', {
    id: props.edge.id,
    conditionExpr: condition,
    label: label,
  })
  visible.value = false
}

const handleDelete = () => {
  if (!props.edge) return
  emit('delete', props.edge.id)
  visible.value = false
}
</script>

<style scoped lang="scss">
.form-hint {
  font-size: 11px;
  color: var(--td-text-placeholder);
  margin-top: 4px;
}
</style>
