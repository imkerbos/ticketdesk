import { ElMessage } from 'element-plus'
import { i18n } from '@/i18n'

// 工具函数在组件外调用，只能走 i18n 全局实例
const t = i18n.global.t

// 单文件大小上限 (10MB), 与后端 MaxAttachmentSize 保持一致
export const MAX_FILE_SIZE = 10 * 1024 * 1024

// 允许的附件扩展名 (小写带点), 与后端 allowedAttachmentExts 保持一致
export const ALLOWED_EXTS = [
  '.jpg', '.jpeg', '.png', '.gif', '.bmp', '.webp',
  '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx',
  '.txt', '.md', '.csv',
  '.zip', '.rar', '.7z', '.tar', '.gz',
  '.log', '.json', '.xml', '.yaml', '.yml',
] as const

// 图片扩展名 (小写带点)
export const IMAGE_EXTS = ['.jpg', '.jpeg', '.png', '.gif', '.bmp', '.webp'] as const

/** 取文件扩展名 (小写, 带点); 无扩展名返回空字符串 */
export function getExt(filename: string): string {
  const idx = filename.lastIndexOf('.')
  if (idx < 0) return ''
  return filename.substring(idx).toLowerCase()
}

/** 判断文件是否为图片 (按扩展名) */
export function isImage(file: File): boolean {
  return (IMAGE_EXTS as readonly string[]).includes(getExt(file.name))
}

/** 把字节数格式化为带单位的字符串 */
export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

/**
 * 校验文件;
 * 不合规给出 ElMessage 错误提示并返回 false; 合规返回 true
 */
export function validateFile(file: File): boolean {
  if (file.size > MAX_FILE_SIZE) {
    ElMessage.error(t('common.fileTooLarge', { name: file.name }))
    return false
  }
  const ext = getExt(file.name)
  if (!(ALLOWED_EXTS as readonly string[]).includes(ext)) {
    ElMessage.error(t('common.fileTypeUnsupported', { name: file.name }))
    return false
  }
  return true
}
