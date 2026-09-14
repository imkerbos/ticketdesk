// 系统配置类型定义

// 配置响应
export interface SystemConfig {
  id: number
  config_key: string
  config_value?: string
  config_type: string
  category: string
  description: string
  is_secret: boolean
  updated_by?: number
  updated_at: string
}

// 邮件配置
export interface EmailConfig {
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password?: string
  from_address: string
  from_name: string
  use_tls: boolean
  enabled: boolean
}

// 安全配置
export interface SecurityConfig {
  mfa_enabled: boolean
  mfa_required: boolean
  password_min_length: number
  password_require_upper: boolean
  password_require_number: boolean
  session_timeout: number
}

// Webhook
export interface Webhook {
  id: number
  name: string
  url: string
  events: string[]
  headers?: Record<string, string>
  status: number
  description: string
  created_by: number
  created_at: string
  updated_at: string
}

// 创建 Webhook 请求
export interface CreateWebhookRequest {
  name: string
  url: string
  secret?: string
  events: string[]
  headers?: Record<string, string>
  description?: string
}

// 更新 Webhook 请求
export interface UpdateWebhookRequest {
  name?: string
  url?: string
  secret?: string
  events?: string[]
  headers?: Record<string, string>
  status?: number
  description?: string
}

// Webhook 日志
export interface WebhookLog {
  id: number
  webhook_id: number
  event: string
  payload: string
  response_code: number
  response_body: string
  status: number
  error_message: string
  retry_count: number
  created_at: string
}

// Webhook 事件类型
export const WebhookEvents = [
  { value: 'issue.created', labelKey: 'project.settings.eventCreated' },
  { value: 'issue.updated', labelKey: 'system.webhookIssueUpdated' },
  { value: 'issue.transitioned', labelKey: 'system.webhookIssueTransitioned' },
  { value: 'issue.assigned', labelKey: 'project.settings.eventAssigned' },
  { value: 'issue.commented', labelKey: 'system.webhookIssueCommented' },
  { value: 'alert.firing', labelKey: 'system.webhookAlertFiring' },
  { value: 'alert.resolved', labelKey: 'system.webhookAlertResolved' },
  { value: 'alert.acked', labelKey: 'system.webhookAlertAcked' },
] as const

export type WebhookEventType = typeof WebhookEvents[number]['value']

// ============ 限流配置 ============

// 限流配置
export interface RateLimitConfig {
  webhook_limit: number
  auth_limit: number
  api_limit: number
}

// ============ 飞书配置 ============

// 飞书配置
export interface LarkConfig {
  enabled: boolean
  webhook_url: string
  secret?: string
}

// 更新飞书配置请求
export interface UpdateLarkConfigRequest {
  enabled: boolean
  webhook_url: string
  secret?: string
}

// ============ Telegram 配置 ============

// Telegram 配置
export interface TelegramConfig {
  enabled: boolean
  bot_token?: string
  chat_id: string
}

// 更新 Telegram 配置请求
export interface UpdateTelegramConfigRequest {
  enabled: boolean
  bot_token?: string
  chat_id: string
}

// ============ SSO 配置 ============

// SSO Claim 映射
export interface SSOClaimMapping {
  local_field: string
  claim_name: string
}

// SSO 配置
export interface SSOAdminConfig {
  enabled: boolean
  provider_name: string
  client_id: string
  client_secret?: string
  issuer_url: string
  redirect_uri: string
  scopes: string
  auto_create_user: boolean
  default_role: string
  claim_mappings: SSOClaimMapping[]
}

// 更新 SSO 配置请求
export interface UpdateSSOConfigRequest {
  enabled: boolean
  provider_name: string
  client_id: string
  client_secret?: string
  issuer_url: string
  redirect_uri: string
  scopes: string
  auto_create_user: boolean
  default_role: string
  claim_mappings: SSOClaimMapping[]
}

// ============ 品牌配置 ============

// 品牌配置
export interface BrandConfig {
  system_name: string
  system_description: string
  copyright_text: string
  logo_url: string
  favicon_url: string
  login_title: string
  login_description: string
}

// 更新品牌配置请求
export interface UpdateBrandConfigRequest {
  system_name: string
  system_description: string
  copyright_text: string
  login_title: string
  login_description: string
}
