// 告警相关类型定义

export interface Alert {
  id: number
  fingerprint: string
  source: string
  alert_name: string
  severity: 'critical' | 'warning' | 'info'
  status: 'firing' | 'resolved'
  labels: Record<string, string>
  annotations: Record<string, string>
  starts_at: string
  ends_at?: string
  issue_id?: number
  issue_key?: string
  ack_at?: string
  ack_by?: number
  ack_by_name?: string
  resolved_at?: string
  resolved_by?: number
  resolved_by_name?: string
  created_at: string
  updated_at: string
}

export interface AlertListRequest {
  page?: number
  page_size?: number
  status?: 'firing' | 'resolved'
  severity?: 'critical' | 'warning' | 'info'
  source?: string
  alert_name?: string
  issue_id?: number
  label_filters?: string
}

export interface AlertListResponse {
  items: Alert[]
  total: number
  page: number
  page_size: number
}

export interface AlertStatsResponse {
  total: number
  firing: number
  resolved: number
  critical: number
  warning: number
  info: number
}

export interface LabelMatcher {
  key: string
  operator: '==' | '!=' | '=~' | '!~'
  value: string
}

export interface AlertRule {
  id: number
  name: string
  description: string
  datasource_id?: number
  datasource_name?: string
  project_id: number
  project_key: string
  project_name: string
  issue_type_id: number
  issue_type_name: string
  label_matchers: LabelMatcher[]
  priority: 'P0' | 'P1' | 'P2' | 'P3'
  assignee_id?: number
  assignee_name?: string
  auto_resolve: boolean
  merge_window: number
  status: number
  created_at: string
  updated_at: string
}

export interface CreateAlertRuleRequest {
  name: string
  description?: string
  datasource_id: number
  project_id: number
  issue_type_id: number
  label_matchers: LabelMatcher[]
  priority: 'P0' | 'P1' | 'P2' | 'P3'
  assignee_id?: number
  auto_resolve: boolean
  merge_window?: number
}

export interface AlertSilence {
  id: number
  name: string
  description: string
  label_matchers: LabelMatcher[]
  silence_type: number // 1-即时静默, 2-预约静默
  starts_at: string
  ends_at?: string
  created_by: number
  created_by_name: string
  comment: string
  status: number // 0-已关闭, 1-生效中, 2-已过期
  created_at: string
  updated_at: string
}

export interface CreateAlertSilenceRequest {
  name: string
  description?: string
  label_matchers: LabelMatcher[]
  silence_type: number // 1-即时静默, 2-预约静默
  starts_at?: string   // 预约静默必填
  ends_at?: string     // 预约静默必填
  comment?: string
}

export interface AlertGroupItem {
  group_value: string
  count: number
  severity: Record<string, number>
  status: Record<string, number>
}

export interface AlertGroupResponse {
  group_by: string
  items: AlertGroupItem[]
  total: number
}

// ============ 告警数据源相关类型 ============

// label 走 i18n key，调用方自行 t()：本文件是类型模块，拿不到组件里的 t
export const DatasourceTypes = [
  { value: 'prometheus', labelKey: 'system.prometheus', icon: '🔥' },
  { value: 'nightingale', labelKey: 'system.nightingale', icon: '🦅' },
] as const

export interface AlertDatasource {
  id: number
  name: string
  type: string
  description: string
  config: Record<string, any>
  push_mode: boolean
  poll_interval: number
  status: number
  webhook_url: string
  last_check_at?: string
  last_check_ok?: boolean
  last_check_msg?: string
  created_by: number
  created_at: string
  updated_at: string
}

export interface CreateDatasourceRequest {
  name: string
  type: string
  description?: string
  config: Record<string, any>
  push_mode: boolean
  poll_interval?: number
}

export interface UpdateDatasourceRequest {
  name?: string
  description?: string
  config?: Record<string, any>
  push_mode?: boolean
  poll_interval?: number
  status?: number
}

export interface TestDatasourceRequest {
  type: string
  config: Record<string, any>
}

export interface TestDatasourceResponse {
  success: boolean
  message: string
  latency_ms: number
}

export interface DatasourceListResponse {
  items: AlertDatasource[]
  total: number
  page: number
  page_size: number
}
