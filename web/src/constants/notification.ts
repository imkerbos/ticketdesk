import type { NotificationEventType } from '@/types/project'

// 通知事件类型选项（用于渠道订阅配置）
//
// 这里只放 i18n key，不放文案：常量在组件外求值，拿不到 useI18n 的 t，
// 写死文案会在切语言时不跟随。
export interface NotificationEventOption {
  value: NotificationEventType
  labelKey: string
  descKey: string
}

export const NOTIFICATION_EVENT_OPTIONS: NotificationEventOption[] = [
  { value: 'issue.created', labelKey: 'project.settings.eventCreated', descKey: 'project.settings.eventCreatedDesc' },
  { value: 'issue.transitioned', labelKey: 'project.settings.eventTransitioned', descKey: 'project.settings.eventTransitionedDesc' },
  { value: 'issue.assigned', labelKey: 'project.settings.eventAssigned', descKey: 'project.settings.eventAssignedDesc' },
  { value: 'alert.merged', labelKey: 'project.settings.eventAlertMerged', descKey: 'project.settings.eventAlertMergedDesc' },
  { value: 'issue.daily_digest', labelKey: 'project.settings.eventDigest', descKey: 'project.settings.eventDigestDesc' },
]

export const DEFAULT_NOTIFICATION_EVENTS: NotificationEventType[] =
  NOTIFICATION_EVENT_OPTIONS.map((o) => o.value)

// 返回 i18n key，调用方自行 t()；未知事件原样返回，界面上能看出是哪个事件
export function getNotificationEventLabelKey(value: string): string {
  return NOTIFICATION_EVENT_OPTIONS.find((o) => o.value === value)?.labelKey || value
}
