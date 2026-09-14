// Package handler 提供工单模块的 HTTP 处理器
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"

	"github.com/kerbos/ticketdesk/internal/api/middleware"
	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-issue/dto"
	"github.com/kerbos/ticketdesk/internal/core-issue/service"
)

// FieldServiceInterface 字段服务接口（避免循环依赖）
type FieldServiceInterface interface {
	GetIssueIDsByEpicLink(ctx context.Context, epicID uint64) ([]uint64, error)
}

// ProjectPermissionChecker 校验用户在指定项目下是否拥有某权限
type ProjectPermissionChecker interface {
	CheckUserPermission(ctx context.Context, projectKey string, userID uint64, permission string) (bool, error)
}

// IssueHandler 工单处理器
type IssueHandler struct {
	issueService service.IssueService
	fieldService FieldServiceInterface
	// permChecker 建单权限校验；项目标识在请求体里，路由中间件取不到，只能在此处校验
	permChecker ProjectPermissionChecker
}

// SetPermissionChecker 注入项目权限校验器
func (h *IssueHandler) SetPermissionChecker(c ProjectPermissionChecker) {
	h.permChecker = c
}

// NewIssueHandler 创建工单处理器实例
func NewIssueHandler(issueService service.IssueService) *IssueHandler {
	return &IssueHandler{
		issueService: issueService,
	}
}

// canCreateInProject 校验当前用户能否在该项目建单；不通过时已写入响应
func (h *IssueHandler) canCreateInProject(c *gin.Context, projectKey string, userID uint64) bool {
	if h.permChecker == nil {
		return true
	}
	// 系统管理员直接放行，与其它权限中间件保持一致
	if middleware.IsAdmin(c) {
		return true
	}
	has, err := h.permChecker.CheckUserPermission(c.Request.Context(), strings.ToUpper(projectKey), userID, "issue:create")
	if err != nil {
		response.InternalErrorT(c, "perm.check_failed")
		return false
	}
	if !has {
		response.ForbiddenT(c, "perm.denied")
		return false
	}
	return true
}

// SetFieldService 设置字段服务（避免循环依赖）
func (h *IssueHandler) SetFieldService(fieldService FieldServiceInterface) {
	h.fieldService = fieldService
}

// ctxWithUser 将 Gin context 中的 user_id 注入到标准 Go context 中
func ctxWithUser(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	userID := c.GetUint64("user_id")
	if userID > 0 {
		ctx = context.WithValue(ctx, service.ContextUserIDKey, userID)
	}
	return ctx
}

// HandleCreateIssue 创建工单
// 支持两种 Content-Type:
//   - application/json (兼容现有调用方, 如告警自动建单)
//   - multipart/form-data (含附件): form field "data" 存 JSON, "files" 存文件列表
//
// @Summary 创建工单
// @Description 支持 application/json 和 multipart/form-data 两种方式，后者可同时上传附件
// @Tags Issue
// @Accept json,multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param body body dto.CreateIssueRequest false "application/json 模式: 创建工单请求体"
// @Param data formData string false "multipart 模式: JSON 字符串（与 body 二选一）"
// @Param files formData file false "multipart 模式: 附件文件（可多个）"
// @Success 201 {object} response.Response{data=dto.IssueResponse} "创建成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "项目不存在"
// @Router /api/v1/issues [post]
func (h *IssueHandler) HandleCreateIssue(c *gin.Context) {
	userID := c.GetUint64("user_id")

	var req dto.CreateIssueRequest
	var files []*multipart.FileHeader

	contentType := c.ContentType()
	if strings.HasPrefix(contentType, "multipart/form-data") {
		dataStr := c.PostForm("data")
		if dataStr == "" {
			response.BadRequestT(c, "issue.missing_form_data")
			return
		}
		if err := json.Unmarshal([]byte(dataStr), &req); err != nil {
			response.BadRequestValidation(c, err)
			return
		}
		if err := binding.Validator.ValidateStruct(&req); err != nil {
			response.BadRequestValidation(c, err)
			return
		}
		form, err := c.MultipartForm()
		if err != nil {
			response.BadRequest(c, response.T(c, "issue.multipart_failed_detail")+err.Error())
			return
		}
		if form != nil {
			files = form.File["files"]
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequestValidation(c, err)
			return
		}
	}

	// 建单权限校验：项目标识来自请求体，无法在路由中间件里判定，
	// 缺了这道校验任何登录用户都能往任意项目建单
	if !h.canCreateInProject(c, req.ProjectKey, userID) {
		return
	}

	var (
		result *dto.IssueResponse
		err    error
	)
	if len(files) > 0 {
		result, err = h.issueService.CreateIssueWithAttachments(c.Request.Context(), &req, files, userID)
	} else {
		result, err = h.issueService.CreateIssue(c.Request.Context(), &req, userID)
	}

	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrIssueTypeNotFound):
			response.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrFileTooLarge):
			response.BadRequestT(c, "issue.file_too_large_10mb")
		case errors.Is(err, service.ErrInvalidFileType):
			response.BadRequestT(c, "issue.invalid_file_type")
		default:
			response.InternalErrorT(c, "issue.create_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleGetIssue 获取工单详情
// @Summary 获取工单详情
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response{data=dto.IssueResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key} [get]
func (h *IssueHandler) HandleGetIssue(c *gin.Context) {
	key := c.Param("key")

	result, err := h.issueService.GetIssue(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.load_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateIssue 更新工单
// @Summary 更新工单
// @Tags Issue
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param body body dto.UpdateIssueRequest true "更新工单请求体"
// @Success 200 {object} response.Response{data=dto.IssueResponse} "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key} [put]
func (h *IssueHandler) HandleUpdateIssue(c *gin.Context) {
	key := c.Param("key")

	var req dto.UpdateIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.issueService.UpdateIssue(ctxWithUser(c), key, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrIssueNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrIssueTypeNotFound):
			response.BadRequest(c, err.Error())
		default:
			response.InternalErrorT(c, "issue.update_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleDeleteIssue 删除工单
// @Summary 删除工单
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response "删除成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key} [delete]
func (h *IssueHandler) HandleDeleteIssue(c *gin.Context) {
	key := c.Param("key")

	err := h.issueService.DeleteIssue(ctxWithUser(c), key)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.delete_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "issue.deleted")})
}

// HandleListIssues 获取工单列表
// @Summary 获取工单列表
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param project_key query string false "项目 Key"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Param status query string false "状态，支持逗号分隔多值，如 open,in_progress"
// @Param priority query string false "优先级 (P0/P1/P2/P3)"
// @Param assignee_id query int false "经办人 ID"
// @Param reporter_id query int false "报告人 ID"
// @Param issue_type_id query int false "工单类型 ID"
// @Param epic_id query int false "Epic ID"
// @Param keyword query string false "关键字搜索"
// @Param category query string false "分类 (normal/alert)"
// @Param sort_by query string false "排序字段 (id/priority/status/issue_type_id/created_at/updated_at)"
// @Param order query string false "排序方向 (asc/desc)"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} response.Response{data=response.PageData} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "项目不存在"
// @Router /api/v1/issues [get]
func (h *IssueHandler) HandleListIssues(c *gin.Context) {
	var req dto.ListIssuesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	// 从中间件注入的项目 ID 列表（非管理员且未指定 project_key 时）
	if projectIDs, exists := c.Get("user_project_ids"); exists {
		if ids, ok := projectIDs.([]uint64); ok {
			req.ProjectIDs = ids
		}
	}

	issues, total, hasMore, err := h.issueService.ListIssues(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.list_failed")
		return
	}

	response.SuccessWithPageHasMore(c, issues, total, req.GetDefaultPage(), req.GetDefaultPageSize(), hasMore)
}

// HandleGetIssueListStats 获取工单列表统计数据
// @Summary 获取工单列表统计数据
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param project_key query string false "项目 Key"
// @Success 200 {object} response.Response{data=dto.IssueListStatsResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/issues/stats [get]
func (h *IssueHandler) HandleGetIssueListStats(c *gin.Context) {
	var req dto.ListIssuesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	// 从中间件注入的项目 ID 列表（非管理员且未指定 project_key 时）
	if projectIDs, exists := c.Get("user_project_ids"); exists {
		if ids, ok := projectIDs.([]uint64); ok {
			req.ProjectIDs = ids
		}
	}

	stats, err := h.issueService.GetIssueListStats(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.stats_failed")
		return
	}

	response.Success(c, stats)
}

// HandleGetProjectOverviewStats 获取项目概述统计（按状态分组聚合，替代 4 次列表请求）
// @Summary 获取项目概述统计
// @Description 按状态分组聚合工单数量，用于项目概述页面
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param project_key query string true "项目 Key"
// @Success 200 {object} response.Response{data=dto.ProjectOverviewStatsResponse} "获取成功"
// @Failure 400 {object} response.ErrorResponse "请求参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "项目不存在"
// @Router /api/v1/issues/project-overview-stats [get]
func (h *IssueHandler) HandleGetProjectOverviewStats(c *gin.Context) {
	var req dto.ProjectOverviewStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	stats, err := h.issueService.GetProjectOverviewStats(c.Request.Context(), req.ProjectKey)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.project_stats_failed")
		return
	}

	response.Success(c, stats)
}

// HandleListIssuesInEpic 获取 Epic 下的所有 Issues
// @Summary 获取 Epic 下的关联工单
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "Epic 工单 Key"
// @Success 200 {object} response.Response{data=[]dto.IssueResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "Epic 不存在"
// @Router /api/v1/issues/{key}/epic-issues [get]
func (h *IssueHandler) HandleListIssuesInEpic(c *gin.Context) {
	epicKey := c.Param("key")

	issues, err := h.issueService.ListIssuesInEpic(c.Request.Context(), epicKey)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, "issue.epic_not_found")
			return
		}
		response.InternalErrorT(c, "issue.epic_issues_failed")
		return
	}

	response.Success(c, issues)
}

// HandleAssignIssue 指派工单
// @Summary 指派工单负责人
// @Tags Issue
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param body body dto.AssignIssueRequest true "指派请求体"
// @Success 200 {object} response.Response{data=dto.IssueResponse} "指派成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/assign [post]
func (h *IssueHandler) HandleAssignIssue(c *gin.Context) {
	key := c.Param("key")

	var req dto.AssignIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.issueService.AssignIssue(ctxWithUser(c), key, req.AssigneeID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrIssueNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			response.BadRequest(c, err.Error())
		default:
			response.InternalErrorT(c, "issue.assign_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleListSubtasks 获取工单的所有子任务
// @Summary 获取工单子任务列表
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "父工单 Key"
// @Success 200 {object} response.Response{data=[]dto.IssueResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "父工单不存在"
// @Router /api/v1/issues/{key}/subtasks [get]
func (h *IssueHandler) HandleListSubtasks(c *gin.Context) {
	parentKey := c.Param("key")

	subtasks, err := h.issueService.ListSubtasks(c.Request.Context(), parentKey)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFoundT(c, "issue.parent_not_found")
			return
		}
		response.InternalErrorT(c, "issue.subtasks_failed")
		return
	}

	response.Success(c, subtasks)
}

// HandleAddComment 添加评论
// @Summary 添加工单评论
// @Tags Issue
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param body body dto.CreateCommentRequest true "评论请求体"
// @Success 201 {object} response.Response{data=dto.CommentResponse} "添加成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/comments [post]
func (h *IssueHandler) HandleAddComment(c *gin.Context) {
	key := c.Param("key")

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	result, err := h.issueService.AddComment(c.Request.Context(), key, &req, userID)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.add_comment_failed")
		return
	}

	response.Created(c, result)
}

// HandleListComments 获取评论列表
// @Summary 获取工单评论列表
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response{data=[]dto.CommentResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/comments [get]
func (h *IssueHandler) HandleListComments(c *gin.Context) {
	key := c.Param("key")

	comments, err := h.issueService.ListComments(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.comments_failed")
		return
	}

	response.Success(c, comments)
}

// HandleDeleteComment 删除评论
// @Summary 删除工单评论
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param comment_id path int true "评论 ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "评论不存在"
// @Router /api/v1/issues/{key}/comments/{comment_id} [delete]
func (h *IssueHandler) HandleDeleteComment(c *gin.Context) {
	commentID, err := strconv.ParseUint(c.Param("comment_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "issue.invalid_comment_id2")
		return
	}

	userID := c.GetUint64("user_id")
	err = h.issueService.DeleteComment(c.Request.Context(), commentID, userID)
	if err != nil {
		if errors.Is(err, service.ErrCommentNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "issue.comment_deleted")})
}

// HandleAddWatcher 添加关注人
// @Summary 添加工单关注人
// @Tags Issue
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param body body dto.AddWatcherRequest false "关注请求体（不传则关注自己）"
// @Success 200 {object} response.Response "关注成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/watchers [post]
func (h *IssueHandler) HandleAddWatcher(c *gin.Context) {
	key := c.Param("key")

	var req dto.AddWatcherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 如果没有传 user_id，默认关注自己
		userID := c.GetUint64("user_id")
		req.UserID = userID
	}

	err := h.issueService.AddWatcher(c.Request.Context(), key, req.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrIssueNotFound):
			response.NotFoundT(c, "issue.not_found")
		case errors.Is(err, service.ErrAlreadyWatching):
			response.BadRequestT(c, "issue.already_watching")
		default:
			response.InternalErrorT(c, "issue.watch_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "issue.watched")})
}

// HandleRemoveWatcher 移除关注人
// @Summary 移除工单关注人
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param user_id path int true "用户 ID"
// @Success 200 {object} response.Response "取消关注成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/watchers/{user_id} [delete]
func (h *IssueHandler) HandleRemoveWatcher(c *gin.Context) {
	key := c.Param("key")
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	err = h.issueService.RemoveWatcher(c.Request.Context(), key, userID)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.unwatch_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "issue.unwatched")})
}

// HandleListWatchers 获取关注人列表
// @Summary 获取工单关注人列表
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response{data=[]dto.WatcherResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/watchers [get]
func (h *IssueHandler) HandleListWatchers(c *gin.Context) {
	key := c.Param("key")

	watchers, err := h.issueService.ListWatchers(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.watchers_failed")
		return
	}

	response.Success(c, watchers)
}

// HandleListMyTodoIssues 获取我的待办工单
// @Summary 获取我的待办工单
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} response.Response{data=[]dto.IssueResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/issues/my-todo [get]
func (h *IssueHandler) HandleListMyTodoIssues(c *gin.Context) {
	userID := c.GetUint64("user_id")

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}

	// 从中间件注入的项目 ID 列表（非管理员用户）
	var projectIDs []uint64
	if ids, exists := c.Get("user_project_ids"); exists {
		if v, ok := ids.([]uint64); ok {
			projectIDs = v
		}
	}

	issues, total, hasMore, err := h.issueService.ListMyTodoIssues(c.Request.Context(), userID, page, pageSize, projectIDs)
	if err != nil {
		response.InternalErrorT(c, "issue.my_todo_failed")
		return
	}

	response.SuccessWithPageHasMore(c, issues, total, page, pageSize, hasMore)
}

// HandleListMyCreatedIssues 获取我创建的工单
// @Summary 获取我创建的工单
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} response.Response{data=[]dto.IssueResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Router /api/v1/issues/my-created [get]
func (h *IssueHandler) HandleListMyCreatedIssues(c *gin.Context) {
	userID := c.GetUint64("user_id")

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page <= 0 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil || pageSize <= 0 {
		pageSize = 10
	}

	// 从中间件注入的项目 ID 列表（非管理员用户）
	var projectIDs []uint64
	if ids, exists := c.Get("user_project_ids"); exists {
		if v, ok := ids.([]uint64); ok {
			projectIDs = v
		}
	}

	issues, total, hasMore, err := h.issueService.ListMyCreatedIssues(c.Request.Context(), userID, page, pageSize, projectIDs)
	if err != nil {
		response.InternalErrorT(c, "issue.my_created_failed")
		return
	}

	response.SuccessWithPageHasMore(c, issues, total, page, pageSize, hasMore)
}

// ============ 工作日志相关处理器 ============

// HandleAddWorklog 添加工作日志
// @Summary 添加工作日志
// @Tags Issue
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param body body dto.CreateWorklogRequest true "工作日志请求体"
// @Success 201 {object} response.Response{data=dto.WorklogResponse} "添加成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/worklogs [post]
func (h *IssueHandler) HandleAddWorklog(c *gin.Context) {
	issueKey := c.Param("key")
	userID := c.GetUint64("user_id")

	var req dto.CreateWorklogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.issueService.AddWorklog(c.Request.Context(), issueKey, &req, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrIssueNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrInvalidTimeFormat):
			response.BadRequest(c, err.Error())
		default:
			response.InternalErrorT(c, "issue.add_worklog_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleUpdateWorklog 更新工作日志
// @Summary 更新工作日志
// @Tags Issue
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param worklog_id path int true "工作日志 ID"
// @Param body body dto.UpdateWorklogRequest true "更新工作日志请求体"
// @Success 200 {object} response.Response{data=dto.WorklogResponse} "更新成功"
// @Failure 400 {object} response.ErrorResponse "参数错误"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 403 {object} response.ErrorResponse "无权限"
// @Failure 404 {object} response.ErrorResponse "工作日志不存在"
// @Router /api/v1/issues/{key}/worklogs/{worklog_id} [put]
func (h *IssueHandler) HandleUpdateWorklog(c *gin.Context) {
	worklogIDStr := c.Param("worklog_id")
	worklogID, err := strconv.ParseUint(worklogIDStr, 10, 64)
	if err != nil {
		response.BadRequestT(c, "issue.invalid_worklog_id")
		return
	}

	userID := c.GetUint64("user_id")

	var req dto.UpdateWorklogRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.issueService.UpdateWorklog(c.Request.Context(), worklogID, &req, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWorklogNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrUnauthorized):
			response.ForbiddenT(c, "perm.denied")
		case errors.Is(err, service.ErrInvalidTimeFormat):
			response.BadRequest(c, err.Error())
		default:
			response.InternalErrorT(c, "issue.update_worklog_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleDeleteWorklog 删除工作日志
// @Summary 删除工作日志
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param worklog_id path int true "工作日志 ID"
// @Success 200 {object} response.Response "删除成功"
// @Failure 400 {object} response.ErrorResponse "无效的工作日志 ID"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 403 {object} response.ErrorResponse "无权限"
// @Failure 404 {object} response.ErrorResponse "工作日志不存在"
// @Router /api/v1/issues/{key}/worklogs/{worklog_id} [delete]
func (h *IssueHandler) HandleDeleteWorklog(c *gin.Context) {
	worklogIDStr := c.Param("worklog_id")
	worklogID, err := strconv.ParseUint(worklogIDStr, 10, 64)
	if err != nil {
		response.BadRequestT(c, "issue.invalid_worklog_id")
		return
	}

	userID := c.GetUint64("user_id")

	err = h.issueService.DeleteWorklog(c.Request.Context(), worklogID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWorklogNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrUnauthorized):
			response.ForbiddenT(c, "perm.denied")
		default:
			response.InternalErrorT(c, "issue.delete_worklog_failed")
		}
		return
	}

	response.Success(c, nil)
}

// HandleListWorklogs 获取工作日志列表
// @Summary 获取工作日志列表
// @Tags Issue
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response{data=[]dto.WorklogResponse} "获取成功"
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 404 {object} response.ErrorResponse "工单不存在"
// @Router /api/v1/issues/{key}/worklogs [get]
func (h *IssueHandler) HandleListWorklogs(c *gin.Context) {
	issueKey := c.Param("key")

	worklogs, err := h.issueService.ListWorklogs(c.Request.Context(), issueKey)
	if err != nil {
		if errors.Is(err, service.ErrIssueNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalErrorT(c, "issue.worklogs_failed")
		return
	}

	response.Success(c, worklogs)
}
