<template>
  <div class="attachment-upload">
    <el-upload
      ref="uploadRef"
      :action="uploadUrl"
      :headers="uploadHeaders"
      :on-success="handleSuccess"
      :on-error="handleError"
      :before-upload="beforeUpload"
      :show-file-list="false"
      :drag="true"
      multiple
    >
      <el-icon class="el-icon--upload"><upload-filled /></el-icon>
      <div class="el-upload__text">
        {{ t('component.attachment.dropHint') }}<em>{{ t('component.attachment.clickUpload') }}</em>
      </div>
      <template #tip>
        <div class="el-upload__tip">
          {{ t('component.attachment.typeHint') }}
        </div>
      </template>
    </el-upload>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import type { UploadInstance } from 'element-plus'

const { t } = useI18n()

const props = defineProps<{
  issueKey: string
}>()

const emit = defineEmits<{
  success: []
}>()

const uploadRef = ref<UploadInstance>()

const uploadUrl = computed(() => {
  return `/api/v1/issues/${props.issueKey}/attachments`
})

const uploadHeaders = computed(() => {
  const token = localStorage.getItem('token')
  return {
    Authorization: `Bearer ${token}`
  }
})

const beforeUpload = (file: File) => {
  // 检查文件大小（10MB）
  const maxSize = 10 * 1024 * 1024
  if (file.size > maxSize) {
    ElMessage.error(t('component.attachment.tooLarge'))
    return false
  }

  // 检查文件类型
  const allowedTypes = [
    '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.webp',
    '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx',
    '.txt', '.md', '.csv',
    '.zip', '.rar', '.7z', '.tar', '.gz',
    '.log', '.json', '.xml', '.yaml', '.yml'
  ]

  const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()
  if (!allowedTypes.includes(ext)) {
    ElMessage.error(t('component.attachment.typeUnsupported'))
    return false
  }

  return true
}

const handleSuccess = () => {
  ElMessage.success(t('component.attachment.uploadSuccess'))
  emit('success')
}

const handleError = () => {
  ElMessage.error(t('component.attachment.uploadFailed'))
}
</script>

<style scoped>
.attachment-upload {
  margin-bottom: 20px;
}

.el-icon--upload {
  font-size: 67px;
  color: var(--td-text-placeholder);
  margin: 40px 0 16px;
  line-height: 50px;
}

.el-upload__text {
  color: var(--td-text-secondary);
  font-size: 14px;
  text-align: center;
}

.el-upload__text em {
  color: var(--td-color-primary);
  font-style: normal;
}

.el-upload__tip {
  font-size: 12px;
  color: var(--td-color-info);
  margin-top: 7px;
}
</style>
