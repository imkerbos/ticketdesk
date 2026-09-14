// 项目相关类型定义

// 用户简要信息（项目负责人）
export interface UserBrief {
  id: number
  username: string
  display_name: string
  avatar_url?: string
}

export interface Project {
  id: number
  project_key: string
  name: string
  description: string
  lead_user_id: number
  lead_user?: UserBrief
  status: number // 0-禁用, 1-启用
  daily_digest_enabled?: boolean
  daily_digest_cron?: string // 5 段 cron 表达式
  daily_digest_tz?: string // IANA 时区
  daily_digest_scope?: 'all_open' | 'assigned_only'
  daily_digest_issue_type_ids?: number[] // 仅推送这些工单类型；空数组 = 全部类型
  member_count?: number
  created_at: string
  updated_at: string
}

export interface ProjectListRequest {
  page?: number
  page_size?: number
  keyword?: string
  status?: number
}

export interface ProjectListResponse {
  items: Project[]
  total: number
  page: number
  page_size: number
}

export interface CreateProjectRequest {
  project_key: string
  name: string
  description?: string
  lead_user_id?: number
  template?: 'standard' | 'blank' // standard=模版项目（默认），blank=空项目
}

export interface UpdateProjectRequest {
  name?: string
  description?: string
  lead_user_id?: number
  status?: number
  daily_digest_enabled?: boolean
  daily_digest_cron?: string
  daily_digest_tz?: string
  daily_digest_scope?: 'all_open' | 'assigned_only'
  daily_digest_issue_type_ids?: number[]
}

export interface ProjectMember {
  id: number
  project_id: number
  user_id: number
  user?: UserBrief
  role: string // owner 或项目角色 role_key（如 administrators, developers, testers, viewers）
  role_name: string
  created_at: string
}

export interface AddProjectMemberRequest {
  user_id: number
  role: string
}

export interface ProjectIssueType {
  id: number
  project_id?: number
  name: string
  display_name: string
  description: string
  icon: string
  color: string
  created_at: string
  updated_at: string
}

export interface CreateIssueTypeRequest {
  name: string
  display_name: string
  description?: string
  icon?: string
  color?: string
}

// ========== 项目角色相关类型 ==========

export interface ProjectRole {
  id: number
  project_id: number
  role_key: string
  role_name: string
  description: string
  is_system: boolean
  sort_order: number
  permissions?: string[]
  member_count?: number
  created_at: string
  updated_at: string
}

export interface CreateProjectRoleRequest {
  role_key: string
  role_name: string
  description?: string
  sort_order?: number
}

export interface UpdateProjectRoleRequest {
  role_name?: string
  description?: string
  sort_order?: number
}

export interface ProjectRoleMember {
  id: number
  project_id: number
  role_id: number
  user_id: number
  user?: UserBrief
  created_at: string
}

export interface AddRoleMemberRequest {
  user_id: number
}

export interface UserRolesResponse {
  roles: ProjectRole[]
}

// ========== 项目通知渠道相关类型 ==========

// 渠道支持的事件类型
export type NotificationEventType =
  | 'issue.created'
  | 'issue.transitioned'
  | 'issue.assigned'
  | 'alert.merged'
  | 'issue.daily_digest'

export interface NotificationChannel {
  id: number
  project_id: number
  channel_type: 'lark' | 'telegram'
  name: string
  config: LarkChannelConfig | TelegramChannelConfig | Record<string, any>
  event_types: NotificationEventType[]
  mention_all: boolean
  enabled: boolean
  created_by: number
  created_at: string
  updated_at: string
}

export interface LarkChannelConfig {
  webhook_url: string
  secret?: string
}

export interface TelegramChannelConfig {
  bot_token?: string
  chat_id: string
}

export interface CreateNotificationChannelRequest {
  channel_type: 'lark' | 'telegram'
  name: string
  config: LarkChannelConfig | TelegramChannelConfig
  event_types: NotificationEventType[]
  mention_all?: boolean
  enabled: boolean
}

export interface UpdateNotificationChannelRequest {
  name?: string
  config?: LarkChannelConfig | TelegramChannelConfig
  event_types?: NotificationEventType[]
  mention_all?: boolean
  enabled?: boolean
}


/** 当前用户在某个项目里的权限，用来决定界面显示哪些操作入口 */
export interface MyProjectPermissions {
  /** 系统管理员在所有项目里都拥有全部权限 */
  is_admin: boolean
  is_member: boolean
  /** 项目 owner 拥有全部权限 */
  is_owner: boolean
  permissions: string[]
}
