// Package model 定义基础数据模型
package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型，所有模型都应嵌入此结构
type BaseModel struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// User 用户模型
type User struct {
	BaseModel
	Username             string     `gorm:"size:50;uniqueIndex;not null" json:"username"`
	Email                string     `gorm:"size:100;uniqueIndex;not null" json:"email"`
	PasswordHash         string     `gorm:"size:255;not null" json:"-"`
	DisplayName          string     `gorm:"size:100" json:"display_name"`
	AvatarURL            string     `gorm:"size:255" json:"avatar_url"`
	Status               int8       `gorm:"default:1;index" json:"status"`                               // 0-禁用, 1-启用
	LastLoginAt          *time.Time `json:"last_login_at,omitempty"`                                     // 最后登录时间
	MFAEnabled           bool       `gorm:"default:false" json:"mfa_enabled"`                            // 是否启用 MFA
	MFASecret            string     `gorm:"size:64" json:"-"`                                            // TOTP 密钥
	MFAVerifiedAt        *time.Time `json:"mfa_verified_at,omitempty"`                                   // MFA 验证时间
	ResetPasswordToken   string     `gorm:"size:64;index" json:"-"`                                      // 重置密码令牌
	ResetPasswordExpires *time.Time `json:"-"`                                                           // 重置密码令牌过期时间
	SSOProvider          string     `gorm:"size:50;index:idx_sso_subject" json:"sso_provider,omitempty"` // SSO 提供方标识（如 "eiam"）
	SSOSubject           string     `gorm:"size:255;index:idx_sso_subject" json:"sso_subject,omitempty"` // OIDC sub claim
	ExtraAttributes      string     `gorm:"type:json" json:"extra_attributes,omitempty"`                 // SSO 扩展属性（JSON）
	TokenVersion         int        `gorm:"not null;default:1" json:"-"`                                 // 令牌版本号；改密码/禁用/登出全部设备时递增，使存量 JWT 立即失效
	LarkOpenID           string     `gorm:"size:64" json:"lark_open_id,omitempty"`                       // 飞书 open_id（用于 webhook @ 用户）
	TelegramUserID       string     `gorm:"size:32" json:"telegram_user_id,omitempty"`                   // Telegram 数字 user ID（用于 tg://user?id=xxx 深链）
	// Locale 用户偏好语言（zh-CN / en-US）。
	// 空 = 跟随 app.language 配置。站内通知与邮件按这个字段渲染 ——
	// 那两处没有请求上下文，Accept-Language 无从谈起，只能落在人身上。
	Locale string `gorm:"size:10" json:"locale,omitempty"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// BeforeSave GORM 钩子：确保 JSON 字段不为空字符串
// 适用 Create 与 Update，避免 SSO 登录 / 用户更新时把 "" 写入 mysql json 列被拒
// （之前只有 BeforeCreate，导致旧用户 update 时若 ExtraAttributes 为空写不进去，
// SSO callback 的 Update 静默失败 → user 看到「SSO 登录但 DB 仍是本地」的现象）
func (u *User) BeforeSave(tx *gorm.DB) error {
	if u.ExtraAttributes == "" {
		u.ExtraAttributes = "{}"
	}
	return nil
}

// Role 角色模型
type Role struct {
	BaseModel
	Name        string `gorm:"size:50;uniqueIndex;not null" json:"name"`
	DisplayName string `gorm:"size:100" json:"display_name"`
	Description string `gorm:"type:text" json:"description"`
}

// TableName 指定表名
func (Role) TableName() string {
	return "roles"
}

// UserRole 用户角色关联模型
type UserRole struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_user_role;index;not null" json:"user_id"`
	RoleID    uint64    `gorm:"uniqueIndex:uk_user_role;index;not null" json:"role_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (UserRole) TableName() string {
	return "user_roles"
}

// Project 项目模型
type Project struct {
	BaseModel
	ProjectKey              string `gorm:"size:20;uniqueIndex;not null" json:"project_key"`
	Name                    string `gorm:"size:100;not null" json:"name"`
	Description             string `gorm:"type:text" json:"description"`
	LeadUserID              uint64 `gorm:"index" json:"lead_user_id"`
	Status                  int8   `gorm:"default:1;index" json:"status"` // 0-归档, 1-活跃
	DailyDigestEnabled      bool   `gorm:"default:false" json:"daily_digest_enabled"`
	DailyDigestCron         string `gorm:"size:64;default:'0 9 * * *'" json:"daily_digest_cron"`   // 5 段 cron 表达式
	DailyDigestTZ           string `gorm:"size:64;default:'Asia/Shanghai'" json:"daily_digest_tz"` // IANA 时区
	DailyDigestScope        string `gorm:"size:32;default:'all_open'" json:"daily_digest_scope"`   // all_open / assigned_only
	DailyDigestIssueTypeIDs string `gorm:"type:json" json:"daily_digest_issue_type_ids"`           // JSON 数组：仅包含这些工单类型 ID；空/NULL = 全部
}

// TableName 指定表名
func (Project) TableName() string {
	return "projects"
}

// BeforeSave GORM 钩子：确保 JSON 字段不写入空字符串
//
// daily_digest_issue_type_ids 是 mysql json 列，GORM 会把 Go 侧的零值 ""
// 一并写进 INSERT，而 MySQL 对 json 列拒绝空串：
//
//	Error 3140 (22032): Invalid JSON text: "The document is empty."
//
// 结果是在任何全新库上「新建项目」直接 500。
// 存量部署没暴露，只是因为老项目建于该列加入之前。
// 与 User.BeforeSave 同一处理方式。
func (p *Project) BeforeSave(tx *gorm.DB) error {
	if p.DailyDigestIssueTypeIDs == "" {
		p.DailyDigestIssueTypeIDs = "[]"
	}
	return nil
}

// ProjectMember 项目成员模型
type ProjectMember struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID uint64    `gorm:"uniqueIndex:uk_project_member;index;not null" json:"project_id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_project_member;index;not null" json:"user_id"`
	Role      string    `gorm:"size:50;default:viewers;index" json:"role"` // owner 或项目角色 role_key（如 administrators, developers, testers, viewers）
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (ProjectMember) TableName() string {
	return "project_members"
}

// IssueType 工单类型模型
type IssueType struct {
	BaseModel
	ProjectID   *uint64 `gorm:"index" json:"project_id"` // NULL 表示全局类型
	Name        string  `gorm:"size:50;index;not null" json:"name"`
	DisplayName string  `gorm:"size:100" json:"display_name"`
	Description string  `gorm:"type:text" json:"description"`
	Icon        string  `gorm:"size:50" json:"icon"`
	Color       string  `gorm:"size:20" json:"color"`
}

// TableName 指定表名
func (IssueType) TableName() string {
	return "issue_types"
}

// Issue 工单模型
type Issue struct {
	BaseModel
	IssueKey string `gorm:"size:30;uniqueIndex;not null" json:"issue_key"`
	// 注意：project_id / assignee_id / reporter_id / epic_id / priority / resolution
	// 的单列索引已移除——它们都是 migrate.go 中复合索引的最左前缀，
	// 保留只会放大写入代价（issues 是最热的写表），优化器也几乎不会选中。
	// 复合索引统一在 createCompositeIndexes 中维护。
	ProjectID          uint64     `gorm:"not null" json:"project_id"`
	IssueTypeID        uint64     `gorm:"index;not null" json:"issue_type_id"`
	Title              string     `gorm:"size:200;not null" json:"title"`
	Description        string     `gorm:"type:text" json:"description"`
	Priority           string     `gorm:"size:10;default:P2" json:"priority"`
	Status             string     `gorm:"size:30;default:open;index" json:"status"`
	Resolution         string     `gorm:"size:30" json:"resolution"` // 解决结果：fixed, wont_fix, duplicate, cannot_reproduce, works_as_designed, incomplete, done
	ReporterID         uint64     `gorm:"not null" json:"reporter_id"`
	AssigneeID         *uint64    `json:"assignee_id"`
	ParentID           *uint64    `gorm:"index" json:"parent_id"`
	EpicID             *uint64    `json:"epic_id"` // Epic 关联（从扩展字段迁移为默认字段）
	WorkflowInstanceID *uint64    `gorm:"index" json:"workflow_instance_id,omitempty"`
	MergedIntoIssueID  *uint64    `gorm:"index" json:"merged_into_issue_id,omitempty"` // 合并目标工单 ID（扁平化：所有旧工单直接指向最终合并目标）
	DueDate            *time.Time `json:"due_date"`
	PlannedStartDate   *time.Time `json:"planned_start_date"` // 预计开始时间
	PlannedEndDate     *time.Time `json:"planned_end_date"`   // 预计交付时间
	ActualStartDate    *time.Time `json:"actual_start_date"`  // 实际开始时间（状态→in_progress 时自动记录）
	ActualEndDate      *time.Time `json:"actual_end_date"`    // 实际完成时间（状态→closed 时自动记录）
	ResolvedAt         *time.Time `json:"resolved_at"`
	ClosedAt           *time.Time `json:"closed_at"`
}

// TableName 指定表名
func (Issue) TableName() string {
	return "issues"
}

// IssueComment 工单评论模型
type IssueComment struct {
	BaseModel
	IssueID uint64 `gorm:"index;not null" json:"issue_id"`
	UserID  uint64 `gorm:"index;not null" json:"user_id"`
	Content string `gorm:"type:text;not null" json:"content"`
}

// TableName 指定表名
func (IssueComment) TableName() string {
	return "issue_comments"
}

// IssueAttachment 工单附件模型
type IssueAttachment struct {
	BaseModel
	IssueID    uint64 `gorm:"index;not null" json:"issue_id"`
	FileName   string `gorm:"size:255;not null" json:"file_name"`
	FilePath   string `gorm:"size:500;not null" json:"file_path"`
	FileSize   int64  `gorm:"not null" json:"file_size"`
	FileType   string `gorm:"size:100" json:"file_type"`
	IsImage    bool   `gorm:"default:false" json:"is_image"`
	UploadedBy uint64 `gorm:"index;not null" json:"uploaded_by"`
}

// TableName 指定表名
func (IssueAttachment) TableName() string {
	return "issue_attachments"
}

// IssueWatcher 工单关注人模型
type IssueWatcher struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	IssueID   uint64    `gorm:"uniqueIndex:uk_issue_watcher;index;not null" json:"issue_id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_issue_watcher;index;not null" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (IssueWatcher) TableName() string {
	return "issue_watchers"
}

// Workflow 工作流定义模型
type Workflow struct {
	BaseModel
	ProjectID   *uint64 `gorm:"index" json:"project_id"`
	Name        string  `gorm:"size:100;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	Status      int8    `gorm:"default:1;index" json:"status"` // 0-禁用, 1-启用
}

// TableName 指定表名
func (Workflow) TableName() string {
	return "workflows"
}

// WorkflowNode 工作流节点模型
type WorkflowNode struct {
	BaseModel
	WorkflowID uint64 `gorm:"index;not null" json:"workflow_id"`
	Name       string `gorm:"size:100;not null" json:"name"`
	NodeType   string `gorm:"size:20;index;not null" json:"node_type"` // start, end, approval, work, system
	Config     string `gorm:"type:json" json:"config"`
	PositionX  int    `gorm:"default:0" json:"position_x"`
	PositionY  int    `gorm:"default:0" json:"position_y"`
}

// TableName 指定表名
func (WorkflowNode) TableName() string {
	return "workflow_nodes"
}

// WorkflowEdge 工作流边模型
type WorkflowEdge struct {
	BaseModel
	WorkflowID    uint64 `gorm:"index;not null" json:"workflow_id"`
	SourceNodeID  uint64 `gorm:"index;not null" json:"source_node_id"`
	TargetNodeID  uint64 `gorm:"index;not null" json:"target_node_id"`
	ConditionExpr string `gorm:"size:500" json:"condition_expr"`
}

// TableName 指定表名
func (WorkflowEdge) TableName() string {
	return "workflow_edges"
}

// Alert 告警模型
type Alert struct {
	BaseModel
	Fingerprint  string     `gorm:"size:64;uniqueIndex;not null" json:"fingerprint"`
	Source       string     `gorm:"size:50;index;not null" json:"source"`
	AlertName    string     `gorm:"size:200;not null" json:"alert_name"`
	Severity     string     `gorm:"size:20;default:warning;index" json:"severity"`
	Status       string     `gorm:"size:20;default:firing;index;index:idx_alert_issue_status,priority:2" json:"status"`
	Labels       string     `gorm:"type:json" json:"labels"`
	Annotations  string     `gorm:"type:json" json:"annotations"`
	StartsAt     time.Time  `gorm:"index;not null" json:"starts_at"`
	EndsAt       *time.Time `json:"ends_at"`
	IssueID      *uint64    `gorm:"index:idx_alert_issue_status,priority:1" json:"issue_id"`
	DatasourceID *uint64    `gorm:"index" json:"datasource_id"` // 关联数据源 ID
	AckAt        *time.Time `json:"ack_at"`                     // 确认时间
	AckBy        *uint64    `gorm:"index" json:"ack_by"`        // 确认人
	ResolvedAt   *time.Time `json:"resolved_at"`                // 解决时间
	ResolvedBy   *uint64    `gorm:"index" json:"resolved_by"`   // 解决人
}

// TableName 指定表名
func (Alert) TableName() string {
	return "alerts"
}

// AlertRule 告警规则模型（自动建单规则）
type AlertRule struct {
	BaseModel
	Name          string  `gorm:"size:100;not null" json:"name"`
	Description   string  `gorm:"type:text" json:"description"`
	DatasourceID  *uint64 `gorm:"index" json:"datasource_id"` // 绑定的数据源 ID
	ProjectID     uint64  `gorm:"index;not null" json:"project_id"`
	IssueTypeID   uint64  `gorm:"index;not null" json:"issue_type_id"`
	LabelMatchers string  `gorm:"type:json;not null" json:"label_matchers"` // 标签匹配规则
	Priority      string  `gorm:"size:10;default:P2" json:"priority"`
	AssigneeID    *uint64 `gorm:"index" json:"assignee_id"`          // 默认指派人
	AutoResolve   bool    `gorm:"default:false" json:"auto_resolve"` // 告警恢复时自动解决工单
	MergeWindow   int     `gorm:"default:3600" json:"merge_window"`  // 告警合并时间窗口（秒），0表示不合并
	Status        int8    `gorm:"default:1;index" json:"status"`     // 0-禁用, 1-启用
}

// TableName 指定表名
func (AlertRule) TableName() string {
	return "alert_rules"
}

// AlertSilence 告警静默模型
type AlertSilence struct {
	BaseModel
	Name          string     `gorm:"size:100;not null" json:"name"`
	Description   string     `gorm:"type:text" json:"description"`
	LabelMatchers string     `gorm:"type:json;not null" json:"label_matchers"`     // 标签匹配规则
	SilenceType   int8       `gorm:"default:1;index;not null" json:"silence_type"` // 1-即时静默, 2-预约静默
	StartsAt      time.Time  `gorm:"index;not null" json:"starts_at"`              // 静默开始时间
	EndsAt        *time.Time `gorm:"index" json:"ends_at"`                         // 静默结束时间，即时静默为 NULL
	CreatedBy     uint64     `gorm:"index;not null" json:"created_by"`             // 创建人
	Comment       string     `gorm:"type:text" json:"comment"`                     // 静默原因
	Status        int8       `gorm:"default:1;index" json:"status"`                // 0-已关闭, 1-生效中, 2-已过期
}

// TableName 指定表名
func (AlertSilence) TableName() string {
	return "alert_silences"
}

// SystemConfig 系统配置模型
type SystemConfig struct {
	BaseModel
	ConfigKey   string  `gorm:"size:100;uniqueIndex;not null" json:"config_key"` // 配置键
	ConfigValue string  `gorm:"type:text" json:"config_value"`                   // 配置值
	ConfigType  string  `gorm:"size:20;default:string" json:"config_type"`       // 类型: string, number, boolean, json
	Category    string  `gorm:"size:50;index" json:"category"`                   // 分类: email, webhook, security, general
	Description string  `gorm:"size:500" json:"description"`                     // 描述
	IsSecret    bool    `gorm:"default:false" json:"is_secret"`                  // 是否为敏感配置（密码等）
	UpdatedBy   *uint64 `gorm:"index" json:"updated_by"`                         // 最后修改人
}

// TableName 指定表名
func (SystemConfig) TableName() string {
	return "system_configs"
}

// Webhook 外发 Webhook 配置模型
type Webhook struct {
	BaseModel
	Name        string `gorm:"size:100;not null" json:"name"`    // Webhook 名称
	URL         string `gorm:"size:500;not null" json:"url"`     // Webhook URL
	Secret      string `gorm:"size:255" json:"-"`                // HMAC 签名密钥
	Events      string `gorm:"type:json;not null" json:"events"` // 订阅的事件类型 JSON 数组
	Headers     string `gorm:"type:json" json:"headers"`         // 自定义请求头 JSON 对象
	Status      int8   `gorm:"default:1;index" json:"status"`    // 0-禁用, 1-启用
	Description string `gorm:"type:text" json:"description"`     // 描述
	CreatedBy   uint64 `gorm:"index;not null" json:"created_by"` // 创建人
}

// TableName 指定表名
func (Webhook) TableName() string {
	return "webhooks"
}

// WebhookLog Webhook 发送日志
type WebhookLog struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	WebhookID    uint64    `gorm:"index;not null" json:"webhook_id"`    // 关联的 Webhook
	Event        string    `gorm:"size:50;index;not null" json:"event"` // 事件类型
	Payload      string    `gorm:"type:text" json:"payload"`            // 发送的数据
	ResponseCode int       `gorm:"default:0" json:"response_code"`      // 响应状态码
	ResponseBody string    `gorm:"type:text" json:"response_body"`      // 响应内容
	Status       int8      `gorm:"default:0;index" json:"status"`       // 0-待发送, 1-成功, 2-失败
	ErrorMessage string    `gorm:"type:text" json:"error_message"`      // 错误信息
	RetryCount   int       `gorm:"default:0" json:"retry_count"`        // 重试次数
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (WebhookLog) TableName() string {
	return "webhook_logs"
}

// Notification 站内通知模型
type Notification struct {
	BaseModel
	UserID     uint64     `gorm:"index:idx_notification_user_read,priority:1;not null" json:"user_id"`      // 接收通知的用户
	Type       string     `gorm:"size:50;not null;index" json:"type"`                                       // 通知类型
	Title      string     `gorm:"size:200;not null" json:"title"`                                           // 通知标题
	Content    string     `gorm:"type:text" json:"content"`                                                 // 通知内容
	EntityType string     `gorm:"size:30;not null" json:"entity_type"`                                      // 实体类型: issue, comment
	EntityID   uint64     `gorm:"index;not null" json:"entity_id"`                                          // 实体ID
	EntityKey  string     `gorm:"size:50;index" json:"entity_key"`                                          // 实体标识（工单编号）
	ActorID    uint64     `gorm:"index" json:"actor_id"`                                                    // 触发者ID
	ActorName  string     `gorm:"size:50" json:"actor_name"`                                                // 触发者名称（冗余）
	IsRead     bool       `gorm:"default:false;index:idx_notification_user_read,priority:2" json:"is_read"` // 是否已读
	ReadAt     *time.Time `json:"read_at,omitempty"`                                                        // 阅读时间
}

// TableName 指定表名
func (Notification) TableName() string {
	return "notifications"
}

// 通知类型常量
const (
	NotificationTypeIssueAssigned      = "issue_assigned"       // 工单被指派
	NotificationTypeIssueUpdated       = "issue_updated"        // 工单更新
	NotificationTypeIssueCommented     = "issue_commented"      // 工单评论
	NotificationTypeMention            = "mention"              // @提及
	NotificationTypeIssueStatusChanged = "issue_status_changed" // 状态变更
)

// IssueWorklog 工单工作日志模型
type IssueWorklog struct {
	BaseModel
	IssueID      uint64    `gorm:"index;not null" json:"issue_id"`
	UserID       uint64    `gorm:"index;not null" json:"user_id"`
	Description  string    `gorm:"type:text;not null" json:"description"`
	TimeSpent    string    `gorm:"size:50;not null" json:"time_spent"` // 格式：2h 30m
	TimeSpentSec int       `gorm:"not null" json:"time_spent_sec"`     // 秒数（用于统计）
	WorkedAt     time.Time `gorm:"index;not null" json:"worked_at"`
	WorkType     string    `gorm:"size:30" json:"work_type"` // 工作类型：开发、测试、调试、文档、故障排查、监控运维、部署发布、配置变更、巡检、安全响应等
}

// TableName 指定表名
func (IssueWorklog) TableName() string {
	return "issue_worklogs"
}

// ActivityLog 活动日志模型
type ActivityLog struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     uint64    `gorm:"index;not null" json:"user_id"`             // 操作用户 ID
	UserName   string    `gorm:"size:50;not null" json:"user_name"`         // 操作用户名（冗余字段，避免关联查询）
	Action     string    `gorm:"size:50;not null;index" json:"action"`      // 操作类型：created, updated, closed, commented, assigned 等
	EntityType string    `gorm:"size:30;not null;index" json:"entity_type"` // 实体类型：issue, alert, project 等
	EntityID   uint64    `gorm:"index;not null" json:"entity_id"`           // 实体 ID
	EntityKey  string    `gorm:"size:50;index" json:"entity_key"`           // 实体标识（如工单编号）
	Details    string    `gorm:"type:text" json:"details"`                  // 详细信息（JSON 格式）
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

// TableName 指定表名
func (ActivityLog) TableName() string {
	return "activity_logs"
}

// ProjectRole 项目角色定义模型
type ProjectRole struct {
	BaseModel
	ProjectID   uint64 `gorm:"not null;uniqueIndex:uk_project_role,priority:1" json:"project_id"`
	RoleKey     string `gorm:"size:50;not null;uniqueIndex:uk_project_role,priority:2" json:"role_key"` // developers, testers, pm
	RoleName    string `gorm:"size:100;not null" json:"role_name"`                                      // 开发人员, 测试人员, 项目经理
	Description string `gorm:"size:500" json:"description"`
	IsSystem    bool   `gorm:"default:false" json:"is_system"` // 系统预置角色不可删除
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
}

// TableName 指定表名
func (ProjectRole) TableName() string {
	return "project_roles"
}

// ProjectRoleMember 项目角色成员模型
type ProjectRoleMember struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID uint64    `gorm:"not null;uniqueIndex:uk_prm,priority:1;index" json:"project_id"`
	RoleID    uint64    `gorm:"not null;uniqueIndex:uk_prm,priority:2;index" json:"role_id"`
	UserID    uint64    `gorm:"not null;uniqueIndex:uk_prm,priority:3;index" json:"user_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (ProjectRoleMember) TableName() string {
	return "project_role_members"
}

// WorkflowInstance 工作流实例模型
type WorkflowInstance struct {
	BaseModel
	IssueID       uint64     `gorm:"not null;uniqueIndex" json:"issue_id"`
	WorkflowID    uint64     `gorm:"not null;index" json:"workflow_id"`
	CurrentNodeID uint64     `gorm:"not null;index" json:"current_node_id"`
	Status        string     `gorm:"size:20;not null;index;index:idx_wi_issue_status,priority:2" json:"status"` // active, completed, reviewing, 已取消
	StartedAt     time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}

// TableName 指定表名
func (WorkflowInstance) TableName() string {
	return "workflow_instances"
}

// WorkflowHistory 流转历史模型
type WorkflowHistory struct {
	BaseModel
	InstanceID uint64    `gorm:"not null;index" json:"instance_id"`
	FromNodeID *uint64   `gorm:"index" json:"from_node_id,omitempty"`
	ToNodeID   uint64    `gorm:"not null;index" json:"to_node_id"`
	Action     string    `gorm:"size:30;not null" json:"action"` // forward, approve, reject, cancel
	OperatorID uint64    `gorm:"not null;index" json:"operator_id"`
	Comment    string    `gorm:"type:text" json:"comment"`
	OperatedAt time.Time `gorm:"not null" json:"operated_at"`
}

// TableName 指定表名
func (WorkflowHistory) TableName() string {
	return "workflow_histories"
}

// ApprovalRecord 审批记录模型
type ApprovalRecord struct {
	BaseModel
	InstanceID uint64     `gorm:"not null;index" json:"instance_id"`
	NodeID     uint64     `gorm:"not null;index" json:"node_id"`
	ApproverID uint64     `gorm:"not null;index" json:"approver_id"`
	Status     string     `gorm:"size:20;not null;index" json:"status"` // pending, approved, rejected
	Comment    string     `gorm:"type:text" json:"comment"`
	ApprovedAt *time.Time `json:"approved_at,omitempty"`
}

// TableName 指定表名
func (ApprovalRecord) TableName() string {
	return "approval_records"
}

// WorkflowScheme 工作流方案模型
type WorkflowScheme struct {
	BaseModel
	ProjectID   uint64 `gorm:"not null;uniqueIndex:uk_project_issue_type,priority:1;index" json:"project_id"`
	IssueTypeID uint64 `gorm:"not null;uniqueIndex:uk_project_issue_type,priority:2;index" json:"issue_type_id"`
	WorkflowID  uint64 `gorm:"not null;index" json:"workflow_id"`
}

// TableName 指定表名
func (WorkflowScheme) TableName() string {
	return "workflow_schemes"
}

// FieldDefinition 字段定义模型
type FieldDefinition struct {
	BaseModel
	ProjectID    *uint64 `gorm:"index" json:"project_id"`                 // NULL=全局系统字段
	FieldKey     string  `gorm:"size:50;not null;index" json:"field_key"` // severity, environment, epic_link
	FieldName    string  `gorm:"size:100;not null" json:"field_name"`     // 严重程度, 环境, Epic链接
	FieldType    string  `gorm:"size:30;not null" json:"field_type"`      // text/textarea/number/date/select/multiselect/user/version/component/label/epic_link/time_estimate
	Description  string  `gorm:"size:500" json:"description"`             // 字段描述
	IsSystem     bool    `gorm:"default:false" json:"is_system"`          // 系统字段不可删除
	IsActive     bool    `gorm:"default:true" json:"is_active"`           // 是否启用
	Options      string  `gorm:"type:json" json:"options"`                // 选项配置JSON（select类型）
	Validation   string  `gorm:"type:json" json:"validation"`             // 校验规则JSON
	DefaultValue string  `gorm:"size:500" json:"default_value"`           // 默认值
	SortOrder    int     `gorm:"default:0" json:"sort_order"`             // 排序
}

// TableName 指定表名
func (FieldDefinition) TableName() string {
	return "field_definitions"
}

// IssueTypeFieldScheme 工单类型字段方案模型
type IssueTypeFieldScheme struct {
	BaseModel
	ProjectID       uint64 `gorm:"not null;uniqueIndex:uk_type_field,priority:1;index" json:"project_id"`
	IssueTypeID     uint64 `gorm:"not null;uniqueIndex:uk_type_field,priority:2;index" json:"issue_type_id"`
	FieldID         uint64 `gorm:"not null;uniqueIndex:uk_type_field,priority:3;index" json:"field_id"`
	IsRequired      bool   `gorm:"default:false" json:"is_required"`      // 是否必填
	IsVisibleCreate bool   `gorm:"default:true" json:"is_visible_create"` // 创建时显示
	IsVisibleEdit   bool   `gorm:"default:true" json:"is_visible_edit"`   // 编辑时显示
	IsVisibleDetail bool   `gorm:"default:true" json:"is_visible_detail"` // 详情时显示
	SortOrder       int    `gorm:"default:0" json:"sort_order"`           // 排序
	DefaultValue    string `gorm:"size:500" json:"default_value"`         // 该类型的默认值（覆盖字段默认值）
}

// TableName 指定表名
func (IssueTypeFieldScheme) TableName() string {
	return "issue_type_field_schemes"
}

// IssueFieldValue 工单字段值模型（EAV模式）
type IssueFieldValue struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	IssueID     uint64     `gorm:"not null;uniqueIndex:uk_issue_field,priority:1;index" json:"issue_id"`
	FieldID     uint64     `gorm:"not null;uniqueIndex:uk_issue_field,priority:2;index" json:"field_id"`
	ValueText   *string    `gorm:"type:text" json:"value_text"` // 文本值
	ValueNumber *float64   `json:"value_number"`                // 数值
	ValueDate   *time.Time `json:"value_date"`                  // 日期值
	ValueJSON   *string    `gorm:"type:json" json:"value_json"` // JSON值（多选、标签等）
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName 指定表名
func (IssueFieldValue) TableName() string {
	return "issue_field_values"
}

// ProjectVersion 项目版本模型
type ProjectVersion struct {
	BaseModel
	ProjectID   uint64     `gorm:"not null;index" json:"project_id"`
	Name        string     `gorm:"size:50;not null" json:"name"`                   // 版本名称
	Description string     `gorm:"size:500" json:"description"`                    // 版本描述
	ReleaseDate *time.Time `json:"release_date"`                                   // 发布日期
	Status      string     `gorm:"size:20;default:unreleased;index" json:"status"` // unreleased, released, archived
	SortOrder   int        `gorm:"default:0" json:"sort_order"`
}

// TableName 指定表名
func (ProjectVersion) TableName() string {
	return "project_versions"
}

// ProjectComponent 项目组件模型
type ProjectComponent struct {
	BaseModel
	ProjectID   uint64  `gorm:"not null;index" json:"project_id"`
	Name        string  `gorm:"size:100;not null" json:"name"` // 组件名称
	Description string  `gorm:"size:500" json:"description"`   // 组件描述
	LeadUserID  *uint64 `gorm:"index" json:"lead_user_id"`     // 组件负责人
}

// TableName 指定表名
func (ProjectComponent) TableName() string {
	return "project_components"
}

// IssueLabel 工单标签模型
type IssueLabel struct {
	BaseModel
	ProjectID   uint64 `gorm:"not null;index" json:"project_id"`
	Name        string `gorm:"size:50;not null" json:"name"` // 标签名称
	Color       string `gorm:"size:20" json:"color"`         // 标签颜色
	Description string `gorm:"size:200" json:"description"`  // 标签描述
}

// TableName 指定表名
func (IssueLabel) TableName() string {
	return "issue_labels"
}

// 数据源类型常量
const (
	DatasourceTypePrometheus  = "prometheus"
	DatasourceTypeNightingale = "nightingale"
)

// AlertDatasource 告警数据源模型
type AlertDatasource struct {
	BaseModel
	Name         string     `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Type         string     `gorm:"size:50;not null;index" json:"type"`
	Description  string     `gorm:"type:text" json:"description"`
	Config       string     `gorm:"type:json;not null" json:"config"`
	PushMode     bool       `gorm:"default:false" json:"push_mode"`
	PollInterval int        `gorm:"default:30" json:"poll_interval"`
	Status       int8       `gorm:"default:1;index" json:"status"`
	LastCheckAt  *time.Time `json:"last_check_at,omitempty"`
	LastCheckOk  *bool      `json:"last_check_ok,omitempty"`
	LastCheckMsg string     `gorm:"size:500" json:"last_check_msg,omitempty"`
	CreatedBy    uint64     `gorm:"index;not null" json:"created_by"`
}

// TableName 指定表名
func (AlertDatasource) TableName() string {
	return "alert_datasources"
}

// ProjectNotificationChannel 项目通知渠道模型
type ProjectNotificationChannel struct {
	BaseModel
	ProjectID   uint64 `gorm:"index;not null" json:"project_id"`
	ChannelType string `gorm:"size:20;not null;index" json:"channel_type"` // lark, telegram
	Name        string `gorm:"size:100;not null" json:"name"`              // 渠道名称，如"运维飞书群"
	Config      string `gorm:"type:json;not null" json:"config"`           // JSON 配置
	EventTypes  string `gorm:"type:json" json:"event_types"`               // 订阅的事件类型 JSON 数组
	MentionAll  bool   `gorm:"default:false" json:"mention_all"`           // 是否 @ 全员（仅飞书 lark_md 卡片有效；Telegram 不支持）
	Enabled     bool   `gorm:"default:true" json:"enabled"`
	CreatedBy   uint64 `gorm:"index" json:"created_by"`
}

// TableName 指定表名
func (ProjectNotificationChannel) TableName() string {
	return "project_notification_channels"
}

// ProjectRolePermission 项目角色权限关联模型
type ProjectRolePermission struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleID        uint64    `gorm:"not null;uniqueIndex:uk_role_permission,priority:1" json:"role_id"`
	PermissionKey string    `gorm:"size:100;not null;uniqueIndex:uk_role_permission,priority:2" json:"permission_key"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (ProjectRolePermission) TableName() string {
	return "project_role_permissions"
}

// FieldSchemeTemplate 字段方案模板
type FieldSchemeTemplate struct {
	BaseModel
	Name        string `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Description string `gorm:"size:500" json:"description"`
	CreatedBy   uint64 `gorm:"not null" json:"created_by"`
	IsActive    bool   `gorm:"default:true" json:"is_active"`
}

// TableName 指定表名
func (FieldSchemeTemplate) TableName() string {
	return "field_scheme_templates"
}

// FieldSchemeTemplateItem 模板字段项
type FieldSchemeTemplateItem struct {
	BaseModel
	TemplateID      uint64 `gorm:"not null;uniqueIndex:uk_tpl_field,priority:1;index" json:"template_id"`
	FieldID         uint64 `gorm:"not null;uniqueIndex:uk_tpl_field,priority:2;index" json:"field_id"`
	IsRequired      bool   `gorm:"default:false" json:"is_required"`
	IsVisibleCreate bool   `gorm:"default:true" json:"is_visible_create"`
	IsVisibleEdit   bool   `gorm:"default:true" json:"is_visible_edit"`
	IsVisibleDetail bool   `gorm:"default:true" json:"is_visible_detail"`
	SortOrder       int    `gorm:"default:0" json:"sort_order"`
	DefaultValue    string `gorm:"size:500" json:"default_value"`
}

// TableName 指定表名
func (FieldSchemeTemplateItem) TableName() string {
	return "field_scheme_template_items"
}

// 字段类型常量
const (
	FieldTypeText         = "text"          // 单行文本
	FieldTypeTextarea     = "textarea"      // 多行文本
	FieldTypeNumber       = "number"        // 数字
	FieldTypeDate         = "date"          // 日期
	FieldTypeDateTime     = "datetime"      // 日期时间 (精确到分钟)
	FieldTypeSelect       = "select"        // 单选
	FieldTypeMultiSelect  = "multiselect"   // 多选
	FieldTypeUser         = "user"          // 用户选择
	FieldTypeMultiUser    = "multiuser"     // 多用户选择
	FieldTypeVersion      = "version"       // 版本选择
	FieldTypeComponent    = "component"     // 组件选择
	FieldTypeLabel        = "label"         // 标签选择
	FieldTypeEpicLink     = "epic_link"     // Epic链接
	FieldTypeTimeEstimate = "time_estimate" // 时间估算
	FieldTypeURL          = "url"           // URL 链接
	FieldTypeCheckbox     = "checkbox"      // 复选框 (布尔)
)

// 版本状态常量
const (
	VersionStatusUnreleased = "unreleased"
	VersionStatusReleased   = "released"
	VersionStatusArchived   = "archived"
)

// APIToken 用户 API 访问令牌 (Personal Access Token)
type APIToken struct {
	BaseModel
	UserID     uint64     `gorm:"not null;index" json:"user_id"`
	Name       string     `gorm:"size:100;not null" json:"name"`
	TokenHash  string     `gorm:"size:64;not null;uniqueIndex" json:"-"`
	Prefix     string     `gorm:"size:16;not null" json:"prefix"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// TableName 指定表名
func (APIToken) TableName() string { return "api_tokens" }

// DailyDigestRun 每日日报执行记录
// 多副本部署去重：唯一键 (project_id, run_date) + INSERT IGNORE 抢占
// 兼审计：记录每次实际发送的日期、工单数、时间
// 注意：不嵌 BaseModel，避免软删除残留行导致唯一键永远冲突
type DailyDigestRun struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	ProjectID  uint64    `gorm:"not null;uniqueIndex:uk_digest_project_run_date,priority:1" json:"project_id"`
	RunDate    string    `gorm:"size:10;not null;uniqueIndex:uk_digest_project_run_date,priority:2" json:"run_date"` // YYYY-MM-DD（按项目 TZ）
	IssueCount int       `gorm:"not null;default:0" json:"issue_count"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// TableName 指定表名
func (DailyDigestRun) TableName() string { return "daily_digest_runs" }
