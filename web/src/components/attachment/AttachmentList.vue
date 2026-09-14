<template>
  <div class="attachment-list">
    <el-empty v-if="attachments.length === 0" :description="t('component.attachment.empty')" />
    <div v-else class="attachment-items">
      <div
        v-for="attachment in attachments"
        :key="attachment.id"
        class="attachment-item"
      >
        <!-- 图片预览 -->
        <div v-if="attachment.is_image" class="attachment-image">
          <el-image
            :src="getImageBlobUrl(attachment.id)"
            :preview-src-list="imageBlobUrls"
            fit="cover"
            class="image-thumbnail"
            :loading="imageLoading[attachment.id] ? 'lazy' : undefined"
          >
            <template #placeholder>
              <div class="image-loading">{{ t('component.attachment.imageLoading') }}</div>
            </template>
            <template #error>
              <div class="image-error">{{ t('component.attachment.imageError') }}</div>
            </template>
          </el-image>
        </div>
        <!-- 文件图标 -->
        <div v-else class="attachment-file">
          <el-icon :size="40" color="var(--td-color-info)">
            <Document />
          </el-icon>
        </div>

        <!-- 文件信息 -->
        <div class="attachment-info">
          <div class="file-name">{{ attachment.file_name }}</div>
          <div class="file-meta">
            <span>{{ formatFileSize(attachment.file_size) }}</span>
            <span class="separator">·</span>
            <span>{{ attachment.uploader?.display_name || t('common.unknownUser') }}</span>
            <span class="separator">·</span>
            <span>{{ formatDate(attachment.created_at) }}</span>
          </div>
        </div>

        <!-- 操作按钮 -->
        <div class="attachment-actions">
          <el-button
            link
            type="primary"
            @click="handleDownload(attachment.id)"
          >
            {{ t('component.attachment.download') }}
          </el-button>
          <el-button
            v-if="canDelete(attachment)"
            link
            type="danger"
            @click="handleDelete(attachment.id)"
          >
            {{ t('common.delete') }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Document } from '@element-plus/icons-vue'
import { deleteAttachment, getAttachmentDownloadUrl } from '@/api/attachment'
import type { Attachment } from '@/types/attachment'
import { useUserStore } from '@/stores/user'

const { t } = useI18n()

const props = defineProps<{
  issueKey: string
  attachments: Attachment[]
}>()

const emit = defineEmits<{
  refresh: []
}>()

const userStore = useUserStore()

// 存储图片的 blob URL
const imageBlobUrls = ref<string[]>([])
const imageBlobUrlMap = ref<Map<number, string>>(new Map())
const imageLoading = ref<Record<number, boolean>>({})

// 获取图片的 blob URL
const getImageBlobUrl = (attachmentId: number) => {
  return imageBlobUrlMap.value.get(attachmentId) || ''
}

// 加载图片 blob
const loadImageBlob = async (attachment: Attachment) => {
  if (!attachment.is_image) return

  imageLoading.value[attachment.id] = true

  try {
    const url = getAttachmentDownloadUrl(props.issueKey, attachment.id)
    const token = localStorage.getItem('token')

    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })

    if (response.ok) {
      const blob = await response.blob()
      const blobUrl = window.URL.createObjectURL(blob)
      imageBlobUrlMap.value.set(attachment.id, blobUrl)
      imageBlobUrls.value.push(blobUrl)
    }
  } catch {
    // ignored
  } finally {
    imageLoading.value[attachment.id] = false
  }
}

// 加载所有图片
const loadAllImages = async () => {
  // 清理旧的 blob URLs
  imageBlobUrls.value.forEach(url => window.URL.revokeObjectURL(url))
  imageBlobUrls.value = []
  imageBlobUrlMap.value.clear()

  // 加载新的图片
  const imageAttachments = props.attachments.filter(a => a.is_image)
  await Promise.all(imageAttachments.map(loadImageBlob))
}

// 组件挂载时加载图片
onMounted(() => {
  loadAllImages()
})

// 组件卸载时清理 blob URLs
onUnmounted(() => {
  imageBlobUrls.value.forEach(url => window.URL.revokeObjectURL(url))
})

// 监听附件列表变化
watch(() => props.attachments, () => {
  loadAllImages()
}, { deep: true })

// 格式化文件大小
const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
}

// 格式化日期
const formatDate = (dateStr: string) => {
  const date = new Date(dateStr)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const days = Math.floor(diff / (1000 * 60 * 60 * 24))

  if (days === 0) {
    const hours = Math.floor(diff / (1000 * 60 * 60))
    if (hours === 0) {
      const minutes = Math.floor(diff / (1000 * 60))
      return minutes === 0 ? t('component.attachment.justNow') : t('component.attachment.minutesAgo', { n: minutes })
    }
    return t('component.attachment.hoursAgo', { n: hours })
  } else if (days === 1) {
    return t('component.attachment.yesterday')
  } else if (days < 7) {
    return t('component.attachment.daysAgo', { n: days })
  } else {
    return date.toLocaleDateString('zh-CN')
  }
}

// 判断是否可以删除（只有上传者可以删除）
const canDelete = (attachment: Attachment) => {
  return attachment.uploaded_by === userStore.user?.id
}

// 下载附件
const handleDownload = async (attachmentId: number) => {
  try {
    const url = getAttachmentDownloadUrl(props.issueKey, attachmentId)
    const token = localStorage.getItem('token')

    // 使用 fetch 带 token 下载
    const response = await fetch(url, {
      headers: {
        'Authorization': `Bearer ${token}`
      }
    })

    if (!response.ok) {
      throw new Error(t('component.attachment.downloadFailed'))
    }

    // 获取文件名
    const attachment = props.attachments.find(a => a.id === attachmentId)
    const filename = attachment?.file_name || 'download'

    // 创建 blob 并下载
    const blob = await response.blob()
    const blobUrl = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = blobUrl
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(blobUrl)

  } catch (error: any) {
    ElMessage.error(error.message || t('component.attachment.downloadFailed'))
  }
}

// 删除附件
const handleDelete = async (attachmentId: number) => {
  try {
    await ElMessageBox.confirm(t('component.attachment.confirmDelete'), t('issue.msg.tipTitle'), {
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      type: 'warning'
    })

    await deleteAttachment(props.issueKey, attachmentId)
    ElMessage.success(t('issue.msg.deleteSuccess'))
    emit('refresh')
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.message || t('issue.msg.deleteFailed2'))
    }
  }
}
</script>

<style scoped>
.attachment-list {
  width: 100%;
}

.attachment-items {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.attachment-item {
  display: flex;
  align-items: center;
  padding: 12px;
  border: 1px solid var(--td-border-color);
  border-radius: 4px;
  transition: all 150ms ease-out;
}

.attachment-item:hover {
  background-color: var(--td-bg-page);
  border-color: var(--td-border-color-dark);
}

.attachment-image {
  flex-shrink: 0;
  margin-right: 12px;
}

.image-thumbnail {
  width: 60px;
  height: 60px;
  border-radius: 4px;
  cursor: pointer;
}

.attachment-file {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 60px;
  height: 60px;
  margin-right: 12px;
  background-color: var(--td-bg-page);
  border-radius: 4px;
}

.attachment-info {
  flex: 1;
  min-width: 0;
}

.file-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--td-text-primary);
  margin-bottom: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-meta {
  font-size: 12px;
  color: var(--td-color-info);
}

.separator {
  margin: 0 8px;
}

.attachment-actions {
  flex-shrink: 0;
  display: flex;
  gap: 8px;
  margin-left: 12px;
}

.image-loading,
.image-error {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  font-size: 12px;
  color: var(--td-color-info);
}

.image-error {
  color: var(--td-color-danger);
}
</style>
