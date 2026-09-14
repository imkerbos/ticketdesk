// 工单超时判定。
//
// 原来这套逻辑在工单列表和工单详情里各写了一份，两份都只看 planned_end_date，
// 把用户明确设置的 due_date 漏掉了 —— 结果是详情页左边写「已超时 30 天」、
// 右边写「截止时间 2026-09-16」（还没到），同一张卡自相矛盾。
// 抽到这里共用，避免改一处漏一处。

import dayjs, { type Dayjs } from 'dayjs'

/** SLA 目标（分钟），与后端 reporting 模块的 slaTargets 保持一致 */
export const SLA_TARGETS: Record<string, number> = { P0: 60, P1: 240, P2: 1440, P3: 4320 }

/**
 * 超时到什么程度才值得标红：超出 SLA 目标 25% 以内算轻微，用中性色。
 * 一屏里十几行全是红色「已超时」，等于没有一行是红色的 —— 真正烧手的那几单反而淹掉了。
 */
export const SEVERE_OVERDUE_RATIO = 0.25

/** 这些状态下工单已经收尾，不再计算超时 */
const SETTLED_STATUSES = ['resolved', 'closed', 'merged']

/** 截止时间是从哪个字段推出来的 —— 展示时可以据此说明依据 */
export type SlaSource = 'planned_end_date' | 'due_date' | 'priority'

export type SlaLevel = 'overdue' | 'overdue_mild' | 'due_soon' | 'normal'

export interface SlaInput {
  status: string
  priority: string
  created_at: string
  due_date?: string | null
  planned_end_date?: string | null
}

export interface SlaState {
  level: SlaLevel
  source: SlaSource
  deadline: Dayjs
  /** 从创建到截止的总时长（分钟），用于算「超出多少算严重」 */
  slaMinutes: number
  /** 距截止还有多少分钟，负数表示已超时 */
  minutesLeft: number
}

/**
 * 截止时间的取值优先级：
 *   1. planned_end_date  排期承诺，最硬
 *   2. due_date          用户明确设的截止时间
 *   3. 按优先级的默认 SLA，从创建时间起算
 *
 * 前两者都是「日期」而非「时刻」，按当天 23:59:59 算 —— 截止日当天还没过完就不算超时。
 */
export const resolveSlaDeadline = (issue: SlaInput): { deadline: Dayjs; slaMinutes: number; source: SlaSource } => {
  const createdAt = dayjs(issue.created_at)
  const fallbackMinutes = SLA_TARGETS[issue.priority] || SLA_TARGETS.P2

  const fromDate = (value: string, source: SlaSource) => {
    const deadline = dayjs(value).endOf('day')
    const span = deadline.diff(createdAt, 'minute')
    // 截止时间早于创建时间属于脏数据；总时长取兜底值，
    // 否则「超出 25% 才算严重」这个比例会除到一个负数上
    return { deadline, slaMinutes: span > 0 ? span : fallbackMinutes, source }
  }

  if (issue.planned_end_date) return fromDate(issue.planned_end_date, 'planned_end_date')
  if (issue.due_date) return fromDate(issue.due_date, 'due_date')

  return { deadline: createdAt.add(fallbackMinutes, 'minute'), slaMinutes: fallbackMinutes, source: 'priority' }
}

/**
 * 计算超时状态。已收尾的工单返回 null —— 一张已终止的单子标「超时」没有意义。
 */
export const getSlaState = (issue: SlaInput, now: Dayjs = dayjs()): SlaState | null => {
  if (SETTLED_STATUSES.includes(issue.status)) return null

  const { deadline, slaMinutes, source } = resolveSlaDeadline(issue)
  const minutesLeft = deadline.diff(now, 'minute', true)

  let level: SlaLevel
  if (minutesLeft < 0) {
    level = -minutesLeft > slaMinutes * SEVERE_OVERDUE_RATIO ? 'overdue' : 'overdue_mild'
  } else if (minutesLeft < slaMinutes * SEVERE_OVERDUE_RATIO) {
    level = 'due_soon'
  } else {
    level = 'normal'
  }

  return { level, source, deadline, slaMinutes, minutesLeft }
}

/**
 * 超时时长的紧凑写法：20m / 5h / 3d。
 * 比只写「已超时」多给一个量级，扫读时能直接排出轻重。
 */
export const formatOverdueSpan = (minutes: number): string => {
  const v = Math.abs(minutes)
  if (v < 60) return `${Math.round(v)}m`
  if (v < 1440) return `${Math.round(v / 60)}h`
  return `${Math.round(v / 1440)}d`
}
