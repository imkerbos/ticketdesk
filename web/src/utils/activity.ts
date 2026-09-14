// 活动流的 action 和 details 在数据库里存的是稳定 key，展示时才翻译。
//
// 为什么不在后端渲染：活动是持久化的，写入时的界面语言不等于读取时的。
// 张三用中文改了工单、李四用英文看活动流，后端渲染就把张三的语言烧进了库。
//
// 早期版本直接存了中文，这些历史数据映射不到 key，原样显示即可 ——
// 显示一句中文比显示一个 code 好，也不该为此去改动已有数据。
import { i18n } from '@/i18n'

const ACTION_KEYS: Record<string, string> = {
  issue_created: 'activity.issueCreated',
  issue_updated: 'activity.issueUpdated',
  issue_deleted: 'activity.issueDeleted',
  issue_assigned: 'activity.issueAssigned',
  issue_merged: 'activity.issueMerged',
  issue_watched: 'activity.issueWatched',
  issue_unwatched: 'activity.issueUnwatched',
  attachment_uploaded: 'activity.attachmentUploaded',
  attachment_deleted: 'activity.attachmentDeleted',
  comment_added: 'activity.commentAdded',
  comment_deleted: 'activity.commentDeleted',
  status_changed: 'activity.statusChanged',
  worklog_added: 'activity.worklogAdded',
  worklog_updated: 'activity.worklogUpdated',
  worklog_deleted: 'activity.worklogDeleted',
  approval_approved: 'activity.approvalApproved',
  approval_rejected: 'activity.approvalRejected',
  node_completed: 'activity.nodeCompleted',
  node_transitioned: 'activity.nodeTransitioned',
  alert_acked: 'activity.alertAcked',
  alert_resolved: 'activity.alertResolved',
  requirement_converted: 'activity.requirementConverted',
}

export const formatActivityAction = (action: string): string => {
  const key = ACTION_KEYS[action]
  return key ? i18n.global.t(key) : action
}

interface ActivityDetail {
  key?: string
  params?: Record<string, unknown>
  /** 这里的值本身是语言包 key，插值前先翻译一层（如「节点 X (通过)」里的「通过」） */
  keys?: Record<string, string>
  items?: ActivityDetail[]
}

const renderOne = (d: ActivityDetail): string => {
  if (!d?.key) return ''
  const params: Record<string, unknown> = { ...(d.params ?? {}) }
  for (const [name, key] of Object.entries(d.keys ?? {})) {
    if (key) params[name] = i18n.global.t(key)
  }
  return i18n.global.t(d.key, params)
}

/**
 * 详情存的是 {"key":"...","params":{...}}，渲染时才翻译。
 * 解析不出来就原样返回 —— 早期版本存的是自由文本中文，那些照常显示。
 */
export const formatActivityDetails = (raw?: string): string => {
  if (!raw) return ''
  if (!raw.startsWith('{')) return raw

  let parsed: ActivityDetail
  try {
    parsed = JSON.parse(raw)
  } catch {
    return raw
  }
  if (!parsed || typeof parsed !== 'object' || !parsed.key) return raw

  // 「更新了: 标题, 优先级 → P1」这类：子项各自翻译，再按当前语言的分隔符拼。
  // 分隔符放在语言包里，中文用「、」英文用「, 」，后端不参与排版。
  if (parsed.items?.length) {
    const sep = i18n.global.t('activity.detailSeparator')
    const body = parsed.items.map(renderOne).filter(Boolean).join(sep)
    return i18n.global.t(parsed.key, { items: body })
  }

  return renderOne(parsed)
}
