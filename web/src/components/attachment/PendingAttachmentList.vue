<template>
  <div
    class="pending-attachment-list"
    :class="{ 'is-drag-active': dragActive }"
    @dragover.prevent="dragActive = true"
    @dragleave.prevent="dragActive = false"
    @drop.prevent="handleDrop"
  >
    <div class="add-area" @click="triggerFileInput">
      <el-icon class="add-icon"><Plus /></el-icon>
      <span class="add-text">{{ t('component.attachment.addHint') }}</span>
      <span class="add-hint">{{ t('component.attachment.sizeHint') }}</span>
    </div>
    <input
      ref="fileInputRef"
      type="file"
      multiple
      style="display: none"
      @change="handleFileInput"
    />

    <div v-if="modelValue.length > 0" class="file-list">
      <div v-for="(file, i) in modelValue" :key="`${file.name}-${file.size}-${i}`" class="file-item">
        <img v-if="isImageFile(file)" :src="previewUrl(file)" class="thumb" alt="" />
        <el-icon v-else class="file-icon"><Document /></el-icon>
        <div class="meta">
          <div class="name" :title="file.name">{{ file.name }}</div>
          <div class="size">{{ formatSize(file.size) }}</div>
        </div>
        <el-icon class="remove" @click.stop="removeFile(i)"><Close /></el-icon>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, onBeforeUnmount } from 'vue'
import { Plus, Close, Document } from '@element-plus/icons-vue'
import { validateFile, isImage, formatSize } from '@/utils/attachment'

const { t } = useI18n()

const props = defineProps<{
  modelValue: File[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', files: File[]): void
}>()

const fileInputRef = ref<HTMLInputElement | null>(null)
const dragActive = ref(false)
// 用 File 对象本身作 key, 避免删除后索引偏移导致预览串号
const previewUrls = new Map<File, string>()

const isImageFile = (f: File) => isImage(f)

const previewUrl = (file: File): string => {
  let url = previewUrls.get(file)
  if (!url) {
    url = URL.createObjectURL(file)
    previewUrls.set(file, url)
  }
  return url
}

const triggerFileInput = () => {
  fileInputRef.value?.click()
}

const handleFileInput = (e: Event) => {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  addFiles(Array.from(input.files))
  input.value = '' // 允许重复选择同名文件
}

const handleDrop = (e: DragEvent) => {
  dragActive.value = false
  if (!e.dataTransfer?.files) return
  addFiles(Array.from(e.dataTransfer.files))
}

const addFiles = (files: File[]) => {
  const accepted: File[] = []
  for (const f of files) {
    // 图片粘贴常无文件名, 给默认名
    const file = f.name
      ? f
      : new File([f], `screenshot-${Date.now()}.png`, { type: f.type || 'image/png' })
    if (!validateFile(file)) continue
    accepted.push(file)
  }
  if (accepted.length > 0) {
    emit('update:modelValue', [...props.modelValue, ...accepted])
  }
}

const removeFile = (idx: number) => {
  const file = props.modelValue[idx]
  const url = previewUrls.get(file)
  if (url) {
    URL.revokeObjectURL(url)
    previewUrls.delete(file)
  }
  const next = props.modelValue.filter((_, i) => i !== idx)
  emit('update:modelValue', next)
}

// 暴露 addFiles 供父组件粘贴监听调用
defineExpose({ addFiles })

onBeforeUnmount(() => {
  for (const url of previewUrls.values()) {
    URL.revokeObjectURL(url)
  }
  previewUrls.clear()
})
</script>

<style scoped lang="scss">
/* token 一律不带 #hex 回退：回退值是亮色写死的，暗色下命中回退就是浅底深字。
   token 本身在 theme.scss 里一定有定义，回退只会掩盖拼错的变量名。 */
.pending-attachment-list {
  width: 100%;
  border: 1px dashed var(--td-border-color);
  border-radius: 8px;
  padding: 10px;
  transition: border-color 150ms ease-out, background-color 150ms ease-out;

  &.is-drag-active {
    border-color: var(--td-color-primary);
    background-color: var(--td-tag-primary-bg);
  }
}

.add-area {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 10px;
  border-radius: 7px;
  cursor: pointer;
  font-size: 13px;
  color: var(--td-text-secondary);
  transition: background-color 150ms ease-out;

  &:hover {
    background-color: var(--td-bg-section);
  }

  .add-icon {
    font-size: 15px;
  }

  .add-hint {
    margin-left: auto;
    font-size: 11.5px;
    color: var(--td-text-placeholder);
  }
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 5px 9px;
  background: var(--td-bg-section);
  border-radius: 7px;
  transition: background-color 150ms ease-out;

  &:hover {
    background: var(--td-bg-card-hover);
  }

  .thumb {
    width: 28px;
    height: 28px;
    object-fit: cover;
    border-radius: 5px;
    flex-shrink: 0;
  }

  .file-icon {
    font-size: 18px;
    color: var(--td-text-placeholder);
    flex-shrink: 0;
  }

  .meta {
    flex: 1;
    min-width: 0;

    .name {
      font-size: 12.5px;
      color: var(--td-text-primary);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .size {
      font-size: 11.5px;
      color: var(--td-text-placeholder);
      font-variant-numeric: tabular-nums;
    }
  }

  .remove {
    font-size: 15px;
    color: var(--td-text-placeholder);
    cursor: pointer;
    flex-shrink: 0;
    transition: color 150ms ease-out;

    &:hover {
      color: var(--td-color-danger);
    }
  }
}

@media (prefers-reduced-motion: reduce) {
  .pending-attachment-list,
  .add-area,
  .file-item,
  .file-item .remove {
    transition: none;
  }
}
</style>
