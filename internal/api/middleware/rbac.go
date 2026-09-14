// Package middleware 提供 RBAC 权限中间件
package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-user/repository"
)

// RBACMiddleware RBAC 权限中间件
type RBACMiddleware struct {
	userRoleRepo repository.UserRoleRepository
}

// NewRBACMiddleware 创建 RBAC 中间件实例
func NewRBACMiddleware(userRoleRepo repository.UserRoleRepository) *RBACMiddleware {
	return &RBACMiddleware{userRoleRepo: userRoleRepo}
}

// RequireRoles 要求用户具有指定角色之一
func (m *RBACMiddleware) RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("user_id")
		if userID == 0 {
			response.UnauthorizedT(c, "perm.no_user")
			c.Abort()
			return
		}

		// 获取用户角色
		userRoles, err := m.userRoleRepo.GetUserRoleNames(c.Request.Context(), userID)
		if err != nil {
			response.InternalErrorT(c, "perm.check_failed")
			c.Abort()
			return
		}

		// 检查是否拥有所需角色之一
		hasRole := false
		for _, requiredRole := range roles {
			for _, userRole := range userRoles {
				if userRole == requiredRole {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			response.ForbiddenT(c, "perm.denied")
			c.Abort()
			return
		}

		// 将用户角色存入上下文
		c.Set("user_roles", userRoles)
		c.Next()
	}
}

// RequireAdmin 要求管理员权限
func (m *RBACMiddleware) RequireAdmin() gin.HandlerFunc {
	return m.RequireRoles("admin")
}

// RequireProjectAdmin 要求项目管理员或系统管理员权限
func (m *RBACMiddleware) RequireProjectAdmin() gin.HandlerFunc {
	return m.RequireRoles("admin", "project_admin")
}

// LoadUserRoles 加载用户角色到上下文（不做权限检查）
func (m *RBACMiddleware) LoadUserRoles() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("user_id")
		if userID == 0 {
			c.Next()
			return
		}

		// 获取用户角色
		userRoles, err := m.userRoleRepo.GetUserRoleNames(c.Request.Context(), userID)
		if err == nil {
			c.Set("user_roles", userRoles)
		}

		c.Next()
	}
}

// HasRole 检查上下文中的用户是否拥有指定角色
func HasRole(c *gin.Context, role string) bool {
	roles, exists := c.Get("user_roles")
	if !exists {
		return false
	}

	userRoles, ok := roles.([]string)
	if !ok {
		return false
	}

	for _, r := range userRoles {
		if r == role {
			return true
		}
	}
	return false
}

// IsAdmin 检查上下文中的用户是否是管理员
func IsAdmin(c *gin.Context) bool {
	return HasRole(c, "admin")
}

// ProjectPermissionChecker 项目权限检查接口（用于 RequireProjectPermission）
type ProjectPermissionChecker interface {
	CheckUserPermission(ctx context.Context, projectKey string, userID uint64, permission string) (bool, error)
}

// RequireProjectPermission 要求用户在项目中拥有指定权限（本期实现但不接入路由）
func RequireProjectPermission(checker ProjectPermissionChecker, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("user_id")
		if userID == 0 {
			response.UnauthorizedT(c, "perm.no_user")
			c.Abort()
			return
		}

		// 系统管理员直接放行
		if IsAdmin(c) {
			c.Next()
			return
		}

		projectKey := c.Param("key")
		if projectKey == "" {
			response.BadRequestT(c, "perm.missing_project")
			c.Abort()
			return
		}

		has, err := checker.CheckUserPermission(c.Request.Context(), projectKey, userID, permission)
		if err != nil {
			response.InternalErrorT(c, "perm.check_failed")
			c.Abort()
			return
		}

		if !has {
			response.ForbiddenT(c, "perm.denied")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireIssuePermission 通过工单 key 提取项目 key 并检查权限
// 工单 key 格式为 "PROJ-123"，提取 "-" 前面的部分作为项目 key
func RequireIssuePermission(checker ProjectPermissionChecker, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("user_id")
		if userID == 0 {
			response.UnauthorizedT(c, "perm.no_user")
			c.Abort()
			return
		}

		// 系统管理员直接放行
		if IsAdmin(c) {
			c.Next()
			return
		}

		issueKey := c.Param("key")
		if issueKey == "" {
			c.Next()
			return
		}

		// 从工单 key 提取项目 key（PROJ-123 → PROJ）
		parts := strings.SplitN(issueKey, "-", 2)
		if len(parts) < 2 {
			response.BadRequestT(c, "common.bad_request")
			c.Abort()
			return
		}
		projectKey := parts[0]

		has, err := checker.CheckUserPermission(c.Request.Context(), projectKey, userID, permission)
		if err != nil {
			response.InternalErrorT(c, "perm.check_failed")
			c.Abort()
			return
		}

		if !has {
			response.ForbiddenT(c, "perm.denied")
			c.Abort()
			return
		}

		c.Next()
	}
}

// MemberProjectLister 获取用户所属项目 ID 列表的接口
type MemberProjectLister interface {
	ListUserProjectIDs(ctx context.Context, userID uint64) ([]uint64, error)
}

// RequireIssueListPermission 检查工单列表查询的项目权限
// 如果指定了 project_key，检查该项目的 issue:view 权限
// 如果未指定，将用户可访问的项目 ID 列表注入上下文
func RequireIssueListPermission(checker ProjectPermissionChecker, lister MemberProjectLister) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint64("user_id")
		if userID == 0 {
			response.UnauthorizedT(c, "perm.no_user")
			c.Abort()
			return
		}

		// 系统管理员直接放行
		if IsAdmin(c) {
			c.Next()
			return
		}

		projectKey := c.Query("project_key")
		if projectKey != "" {
			has, err := checker.CheckUserPermission(c.Request.Context(), projectKey, userID, "issue:view")
			if err != nil {
				response.InternalErrorT(c, "perm.check_failed")
				c.Abort()
				return
			}
			if !has {
				response.ForbiddenT(c, "perm.denied")
				c.Abort()
				return
			}
			c.Next()
			return
		}

		// 未指定项目，注入用户可访问的项目 ID 列表
		projectIDs, err := lister.ListUserProjectIDs(c.Request.Context(), userID)
		if err != nil {
			response.InternalErrorT(c, "perm.check_failed")
			c.Abort()
			return
		}
		c.Set("user_project_ids", projectIDs)
		c.Next()
	}
}
