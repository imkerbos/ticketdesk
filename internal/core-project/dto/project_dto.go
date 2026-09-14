// Package dto 定义项目模块的数据传输对象
package dto

import "time"

// ============ 请求 DTO ============

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	ProjectKey  string `json:"project_key" binding:"required,min=2,max=10,uppercase"`
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=1000"`
	LeadUserID  uint64 `json:"lead_user_id" binding:"required"`
	Template    string `json:"template"` // "standard" 使用模版（默认），"blank" 空项目
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name                    *string   `json:"name" binding:"omitempty,min=1,max=100"`
	Description             *string   `json:"description" binding:"omitempty,max=1000"`
	LeadUserID              *uint64   `json:"lead_user_id"`
	Status                  *int8     `json:"status" binding:"omitempty,oneof=0 1"`
	DailyDigestEnabled      *bool     `json:"daily_digest_enabled"`
	DailyDigestCron         *string   `json:"daily_digest_cron" binding:"omitempty,max=64"`
	DailyDigestTZ           *string   `json:"daily_digest_tz" binding:"omitempty,max=64"`
	DailyDigestScope        *string   `json:"daily_digest_scope" binding:"omitempty,oneof=all_open assigned_only"`
	DailyDigestIssueTypeIDs *[]uint64 `json:"daily_digest_issue_type_ids"`
}

// ListProjectsRequest 项目列表请求
type ListProjectsRequest struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=100"`
	Keyword  string `form:"keyword" binding:"omitempty,max=50"`
	Status   *int8  `form:"status" binding:"omitempty,oneof=0 1"`
}

// GetDefaultPage 获取默认页码
func (r *ListProjectsRequest) GetDefaultPage() int {
	if r.Page <= 0 {
		return 1
	}
	return r.Page
}

// GetDefaultPageSize 获取默认每页数量
func (r *ListProjectsRequest) GetDefaultPageSize() int {
	if r.PageSize <= 0 {
		return 20
	}
	return r.PageSize
}

// AddMemberRequest 添加项目成员请求
type AddMemberRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

// UpdateMemberRequest 更新项目成员请求
type UpdateMemberRequest struct {
	Role string `json:"role" binding:"required"`
}

// CreateIssueTypeRequest 创建工单类型请求
type CreateIssueTypeRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=50"`
	DisplayName string `json:"display_name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
	Icon        string `json:"icon" binding:"max=50"`
	Color       string `json:"color" binding:"max=20"`
}

// UpdateIssueTypeRequest 更新工单类型请求
type UpdateIssueTypeRequest struct {
	DisplayName *string `json:"display_name" binding:"omitempty,min=1,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Icon        *string `json:"icon" binding:"omitempty,max=50"`
	Color       *string `json:"color" binding:"omitempty,max=20"`
}

// ============ 响应 DTO ============

// ProjectResponse 项目响应
type ProjectResponse struct {
	ID                      uint64     `json:"id"`
	ProjectKey              string     `json:"project_key"`
	Name                    string     `json:"name"`
	Description             string     `json:"description"`
	LeadUserID              uint64     `json:"lead_user_id"`
	LeadUser                *UserBrief `json:"lead_user,omitempty"`
	Status                  int8       `json:"status"`
	DailyDigestEnabled      bool       `json:"daily_digest_enabled"`
	DailyDigestCron         string     `json:"daily_digest_cron,omitempty"`
	DailyDigestTZ           string     `json:"daily_digest_tz,omitempty"`
	DailyDigestScope        string     `json:"daily_digest_scope,omitempty"`
	DailyDigestIssueTypeIDs []uint64   `json:"daily_digest_issue_type_ids"`
	MemberCount             int        `json:"member_count,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

// UserBrief 用户简要信息
type UserBrief struct {
	ID          uint64 `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

// ProjectMemberResponse 项目成员响应
type ProjectMemberResponse struct {
	ID        uint64     `json:"id"`
	ProjectID uint64     `json:"project_id"`
	UserID    uint64     `json:"user_id"`
	User      *UserBrief `json:"user,omitempty"`
	Role      string     `json:"role"`
	RoleName  string     `json:"role_name"`
	CreatedAt time.Time  `json:"created_at"`
}

// IssueTypeResponse 工单类型响应
type IssueTypeResponse struct {
	ID          uint64    `json:"id"`
	ProjectID   *uint64   `json:"project_id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ============ 项目角色相关 DTO ============

// CreateProjectRoleRequest 创建项目角色请求
type CreateProjectRoleRequest struct {
	RoleKey     string `json:"role_key" binding:"required,min=2,max=50"`
	RoleName    string `json:"role_name" binding:"required,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateProjectRoleRequest 更新项目角色请求
type UpdateProjectRoleRequest struct {
	RoleName    *string `json:"role_name" binding:"omitempty,min=1,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
}

// ProjectRoleResponse 项目角色响应
type ProjectRoleResponse struct {
	ID          uint64    `json:"id"`
	ProjectID   uint64    `json:"project_id"`
	RoleKey     string    `json:"role_key"`
	RoleName    string    `json:"role_name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	SortOrder   int       `json:"sort_order"`
	Permissions []string  `json:"permissions,omitempty"`
	MemberCount int       `json:"member_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SetRolePermissionsRequest 设置角色权限请求
type SetRolePermissionsRequest struct {
	Permissions []string `json:"permissions" binding:"required"`
}

// AddRoleMemberRequest 添加角色成员请求
type AddRoleMemberRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

// ProjectRoleMemberResponse 项目角色成员响应
type ProjectRoleMemberResponse struct {
	ID        uint64     `json:"id"`
	ProjectID uint64     `json:"project_id"`
	RoleID    uint64     `json:"role_id"`
	UserID    uint64     `json:"user_id"`
	User      *UserBrief `json:"user,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// MyProjectPermissionsResponse 当前用户在某个项目里的权限。
//
// 前端需要它来决定「显示什么」——原来没有这个接口，项目设置和项目角色页
// 对任何项目成员都照常渲染，包括「危险操作 → 删除此项目」；点下去才被后端
// 403 挡掉。控件摆在那里却一按就报错，比不显示更糟。
type MyProjectPermissionsResponse struct {
	// IsAdmin 系统管理员在所有项目里都拥有全部权限
	IsAdmin bool `json:"is_admin"`
	// IsMember 是否为该项目成员
	IsMember bool `json:"is_member"`
	// IsOwner 项目 owner 拥有全部权限
	IsOwner bool `json:"is_owner"`
	// Permissions 合并后的权限键集合；IsAdmin 或 IsOwner 为真时前端应视为全通过
	Permissions []string `json:"permissions"`
}
