// 字段类型常量
export const FieldType = {
  TEXT: 'text',
  TEXTAREA: 'textarea',
  NUMBER: 'number',
  DATE: 'date',
  DATETIME: 'datetime',
  SELECT: 'select',
  MULTISELECT: 'multiselect',
  USER: 'user',
  MULTIUSER: 'multiuser',
  VERSION: 'version',
  COMPONENT: 'component',
  LABEL: 'label',
  EPIC_LINK: 'epic_link',
  TIME_ESTIMATE: 'time_estimate',
  URL: 'url',
  CHECKBOX: 'checkbox',
} as const

export type FieldTypeValue = typeof FieldType[keyof typeof FieldType]

// 字段选项
export interface FieldOption {
  value: string
  label: string
}

// 字段定义
export interface FieldDefinition {
  id: number
  project_id: number | null
  field_key: string
  field_name: string
  field_type: FieldTypeValue
  description: string
  is_system: boolean
  is_active: boolean
  options: string // JSON字符串
  validation: string // JSON字符串
  default_value: string
  sort_order: number
  created_at: string
  updated_at: string
}

// 字段方案项
export interface FieldSchemeItem {
  id: number
  project_id: number
  issue_type_id: number
  field_id: number
  field?: FieldDefinition
  is_required: boolean
  is_visible_create: boolean
  is_visible_edit: boolean
  is_visible_detail: boolean
  sort_order: number
  default_value: string
  created_at: string
  updated_at: string
}

// 字段值
export interface FieldValue {
  field_id: number
  field_key: string
  field_name: string
  field_type: FieldTypeValue
  value: any
  display_value: string
}

// 自定义字段值（用于提交）
export interface CustomFieldValue {
  field_id: number
  value: any
}

// 项目版本
export interface ProjectVersion {
  id: number
  project_id: number
  name: string
  description: string
  release_date: string | null
  status: 'unreleased' | 'released' | 'archived'
  sort_order: number
  created_at: string
  updated_at: string
}

// 项目组件
export interface ProjectComponent {
  id: number
  project_id: number
  name: string
  description: string
  lead_user_id: number | null
  lead_user?: {
    id: number
    username: string
    display_name: string
    avatar_url: string
  }
  created_at: string
  updated_at: string
}

// 工单标签
export interface IssueLabel {
  id: number
  project_id: number
  name: string
  color: string
  description: string
  created_at: string
  updated_at: string
}

// 创建字段请求
export interface CreateFieldRequest {
  field_key: string
  field_name: string
  field_type: FieldTypeValue
  description?: string
  options?: string
  validation?: string
  default_value?: string
}

// 更新字段请求
export interface UpdateFieldRequest {
  field_name?: string
  description?: string
  options?: string
  validation?: string
  default_value?: string
  is_active?: boolean
  sort_order?: number
}

// 更新字段方案请求项
export interface FieldSchemeItemRequest {
  field_id: number
  is_required: boolean
  is_visible_create: boolean
  is_visible_edit: boolean
  is_visible_detail: boolean
  sort_order: number
  default_value?: string
}

// 更新字段方案请求
export interface UpdateFieldSchemeRequest {
  items: FieldSchemeItemRequest[]
}

// 创建版本请求
export interface CreateVersionRequest {
  name: string
  description?: string
  release_date?: string
}

// 更新版本请求
export interface UpdateVersionRequest {
  name?: string
  description?: string
  release_date?: string
  status?: 'unreleased' | 'released' | 'archived'
}

// 创建组件请求
export interface CreateComponentRequest {
  name: string
  description?: string
  lead_user_id?: number
}

// 更新组件请求
export interface UpdateComponentRequest {
  name?: string
  description?: string
  lead_user_id?: number
}

// 创建标签请求
export interface CreateLabelRequest {
  name: string
  color?: string
  description?: string
}

// 更新标签请求
export interface UpdateLabelRequest {
  name?: string
  color?: string
  description?: string
}

// ============ 方案模板类型 ============

// 方案模板
export interface FieldSchemeTemplate {
  id: number
  name: string
  description: string
  created_by: number
  is_active: boolean
  item_count: number
  created_at: string
  updated_at: string
}

// 模板详情（含字段项）
export interface FieldSchemeTemplateDetail {
  id: number
  name: string
  description: string
  created_by: number
  is_active: boolean
  items: FieldSchemeTemplateItem[]
  created_at: string
  updated_at: string
}

// 模板字段项
export interface FieldSchemeTemplateItem {
  id: number
  template_id: number
  field_id: number
  field?: FieldDefinition
  is_required: boolean
  is_visible_create: boolean
  is_visible_edit: boolean
  is_visible_detail: boolean
  sort_order: number
  default_value: string
}

// 字段使用情况
export interface FieldUsage {
  scheme_count: number
  value_count: number
  template_count: number
}

// 创建模板请求
export interface CreateTemplateRequest {
  name: string
  description?: string
}

// 更新模板请求
export interface UpdateTemplateRequest {
  name?: string
  description?: string
  is_active?: boolean
}

// 模板字段项输入
export interface TemplateItemInput {
  field_id: number
  is_required: boolean
  is_visible_create: boolean
  is_visible_edit: boolean
  is_visible_detail: boolean
  sort_order: number
  default_value?: string
}

// 套用模板请求
export interface ApplyTemplateRequest {
  template_id: number
  mode: 'replace' | 'merge'
}

// ============ 内置字段常量 ============

// 内置字段 key 列表（对应 Issue 表列，不存 EAV）
export const BUILTIN_FIELD_KEYS = [
  'description', 'priority', 'assignee',
  'planned_start_date', 'planned_end_date', 'epic_link'
] as const

// 内置字段 key → CreateIssueRequest/UpdateIssueRequest 中的属性名映射
export const BUILTIN_FIELD_MAP: Record<string, string> = {
  'description': 'description',
  'priority': 'priority',
  'assignee': 'assignee_id',
  'planned_start_date': 'planned_start_date',
  'planned_end_date': 'planned_end_date',
  'epic_link': 'epic_id',
}

// 判断是否为内置字段
export function isBuiltinField(fieldKey: string): boolean {
  return (BUILTIN_FIELD_KEYS as readonly string[]).includes(fieldKey)
}

// 解析字段选项（容错处理各种 JSON 格式）
export function parseFieldOptions(optionsJson: string): FieldOption[] {
  if (!optionsJson) return []
  try {
    const parsed = JSON.parse(optionsJson)
    if (!Array.isArray(parsed)) return []
    return parsed.map((item: any) => {
      // 支持纯字符串格式: ["opt1", "opt2"]
      if (typeof item === 'string') {
        return { value: item, label: item }
      }
      // 支持对象格式，value/label 互相 fallback
      if (typeof item === 'object' && item !== null) {
        return {
          value: String(item.value ?? item.label ?? ''),
          label: String(item.label ?? item.value ?? ''),
        }
      }
      // 其他类型转字符串
      return { value: String(item), label: String(item) }
    })
  } catch {
    return []
  }
}

// 字段类型对应的 i18n key
//
// 这里只返回 key、不返回文案：本文件是纯类型/常量模块，拿不到组件里的 t，
// 写死中文会在切语言时不跟随。
export function getFieldTypeLabelKey(type: FieldTypeValue): string {
  const keys: Record<FieldTypeValue, string> = {
    [FieldType.TEXT]: 'field.typeMap.text',
    [FieldType.TEXTAREA]: 'field.typeMap.textarea',
    [FieldType.NUMBER]: 'field.typeMap.number',
    [FieldType.DATE]: 'field.typeMap.date',
    [FieldType.DATETIME]: 'field.typeMap.datetime',
    [FieldType.SELECT]: 'field.typeMap.select',
    [FieldType.MULTISELECT]: 'field.typeMap.multiselect',
    [FieldType.USER]: 'field.typeMap.user',
    [FieldType.MULTIUSER]: 'field.typeMap.multiuser',
    [FieldType.VERSION]: 'field.typeMap.version',
    [FieldType.COMPONENT]: 'field.typeMap.component',
    [FieldType.LABEL]: 'field.typeMap.label',
    [FieldType.EPIC_LINK]: 'field.typeMap.epic_link',
    [FieldType.TIME_ESTIMATE]: 'field.typeMap.time_estimate',
    [FieldType.URL]: 'field.typeMap.url',
    [FieldType.CHECKBOX]: 'field.typeMap.checkbox',
  }
  return keys[type] || type
}
