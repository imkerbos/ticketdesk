/**
 * 工单展示相关的纯函数。
 *
 * 抽出来的原因：详情页拆分后父子组件都要用这几个格式化函数，
 * 各写一份必然改一处漏一处（状态文案此前就已经在列表、详情、报表、
 * 看板里各有一份）。这里只放不依赖组件状态的逻辑。
 */
import dayjs from 'dayjs'
import { i18n } from '@/i18n'

export type TagType = 'primary' | 'success' | 'warning' | 'info' | 'danger'

const t = (key: string) => i18n.global.t(key)

/** 状态文案统一走语言包 */
export const getStatusText = (status: string) => t(`issue.statusMap.${status}`)

export const getPriorityType = (priority: string): TagType => {
  const map: Record<string, TagType> = { P0: 'danger', P1: 'warning', P2: 'info', P3: 'success' }
  return map[priority] || 'info'
}

/** 解决结果：认识的走语言包，不认识的原样显示（历史数据可能有自定义值） */
export const getResolutionText = (resolution: string) => {
  const known = ['fixed', 'wont_fix', 'duplicate', 'cannot_reproduce', 'works_as_designed', 'incomplete', 'done']
  return known.includes(resolution) ? t(`issue.resolutionMap.${resolution}`) : resolution
}

export const formatTime = (time: string) => dayjs(time).format('YYYY-MM-DD HH:mm')

export const formatDate = (date: string) => dayjs(date).format('YYYY-MM-DD')

/** 工时按 8 小时一个工作日折算 */
export const formatTimeSpent = (seconds: number) => {
  const days = Math.floor(seconds / (8 * 3600))
  const hours = Math.floor((seconds % (8 * 3600)) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  const parts = []
  if (days > 0) parts.push(`${days}d`)
  if (hours > 0) parts.push(`${hours}h`)
  if (minutes > 0) parts.push(`${minutes}m`)

  return parts.length > 0 ? parts.join(' ') : '0m'
}
