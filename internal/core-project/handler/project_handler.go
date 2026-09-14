// Package handler 提供项目模块的 HTTP 处理器
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/middleware"
	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-project/dto"
	"github.com/kerbos/ticketdesk/internal/core-project/service"
)

// ProjectHandler 项目处理器
type ProjectHandler struct {
	projectService service.ProjectService
}

// NewProjectHandler 创建项目处理器实例
func NewProjectHandler(projectService service.ProjectService) *ProjectHandler {
	return &ProjectHandler{projectService: projectService}
}

// HandleCreateProject 创建项目
// @Summary 创建项目
// @Description 创建一个新项目
// @Tags Project
// @Accept json
// @Produce json
// @Param request body dto.CreateProjectRequest true "创建项目请求"
// @Success 201 {object} response.Response{data=dto.ProjectResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse "未认证"
// @Failure 403 {object} response.ErrorResponse "需项目管理员"
// @Router /api/v1/projects [post]
// @Security BearerAuth
func (h *ProjectHandler) HandleCreateProject(c *gin.Context) {
	var req dto.CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	result, err := h.projectService.CreateProject(c.Request.Context(), &req, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectKeyExists) {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "project.create_failed")
		return
	}

	response.Created(c, result)
}

// HandleGetProject 获取项目详情
// @Summary 获取项目详情
// @Description 根据项目 Key 获取项目详细信息
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response{data=dto.ProjectResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key} [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleGetProject(c *gin.Context) {
	key := c.Param("key")

	result, err := h.projectService.GetProject(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.get_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateProject 更新项目
// @Summary 更新项目
// @Description 更新项目信息
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param request body dto.UpdateProjectRequest true "更新项目请求"
// @Success 200 {object} response.Response{data=dto.ProjectResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key} [put]
// @Security BearerAuth
func (h *ProjectHandler) HandleUpdateProject(c *gin.Context) {
	key := c.Param("key")

	var req dto.UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.projectService.UpdateProject(c.Request.Context(), key, &req)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.update_failed")
		return
	}

	response.Success(c, result)
}

// HandleDeleteProject 删除项目
// @Summary 删除项目
// @Description 硬删除项目及所有关联数据
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key} [delete]
// @Security BearerAuth
func (h *ProjectHandler) HandleDeleteProject(c *gin.Context) {
	key := c.Param("key")

	err := h.projectService.DeleteProject(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		// 记录详细错误以便排查
		response.InternalError(c, response.T(c, "project.delete_failed_detail")+err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.deleted")})
}

// HandleListProjects 获取项目列表
// @Summary 获取项目列表
// @Description 分页获取项目列表
// @Tags Project
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param keyword query string false "搜索关键字"
// @Param status query int false "项目状态 (0-归档, 1-活跃)"
// @Success 200 {object} response.Response{data=response.PageData}
// @Router /api/v1/projects [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListProjects(c *gin.Context) {
	var req dto.ListProjectsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	userID := c.GetUint64("user_id")
	isAdmin := middleware.IsAdmin(c)

	projects, total, err := h.projectService.ListProjects(c.Request.Context(), &req, userID, isAdmin)
	if err != nil {
		response.InternalError(c, "project.list_failed")
		return
	}

	response.SuccessWithPage(c, projects, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// HandleAddMember 添加项目成员
// @Summary 添加项目成员
// @Description 向项目添加新成员
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param request body dto.AddMemberRequest true "添加成员请求"
// @Success 201 {object} response.Response{data=dto.ProjectMemberResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/members [post]
// @Security BearerAuth
func (h *ProjectHandler) HandleAddMember(c *gin.Context) {
	key := c.Param("key")

	var req dto.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.projectService.AddMember(c.Request.Context(), key, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrMemberAlreadyExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "project.member_add_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleUpdateMember 更新项目成员
// @Summary 更新项目成员
// @Description 更新项目成员角色
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param user_id path int true "用户 ID"
// @Param request body dto.UpdateMemberRequest true "更新成员请求"
// @Success 200 {object} response.Response{data=dto.ProjectMemberResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/members/{user_id} [put]
// @Security BearerAuth
func (h *ProjectHandler) HandleUpdateMember(c *gin.Context) {
	key := c.Param("key")
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	var req dto.UpdateMemberRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.projectService.UpdateMember(c.Request.Context(), key, userID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrMemberNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "project.member_update_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleRemoveMember 移除项目成员
// @Summary 移除项目成员
// @Description 从项目中移除成员
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Param user_id path int true "用户 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/members/{user_id} [delete]
// @Security BearerAuth
func (h *ProjectHandler) HandleRemoveMember(c *gin.Context) {
	key := c.Param("key")
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	err = h.projectService.RemoveMember(c.Request.Context(), key, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrMemberNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrCannotRemoveOwner):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "project.member_remove_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.member_removed")})
}

// HandleListMembers 获取项目成员列表
// @Summary 获取项目成员列表
// @Description 获取项目的所有成员
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response{data=[]dto.ProjectMemberResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/members [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListMembers(c *gin.Context) {
	key := c.Param("key")

	members, err := h.projectService.ListMembers(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.member_list_failed")
		return
	}

	response.Success(c, members)
}

// HandleCreateIssueType 创建工单类型
// @Summary 创建工单类型
// @Description 为项目创建自定义工单类型
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param request body dto.CreateIssueTypeRequest true "创建工单类型请求"
// @Success 201 {object} response.Response{data=dto.IssueTypeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/issue-types [post]
// @Security BearerAuth
func (h *ProjectHandler) HandleCreateIssueType(c *gin.Context) {
	key := c.Param("key")

	var req dto.CreateIssueTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.projectService.CreateIssueType(c.Request.Context(), key, &req)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.issue_type_create_failed")
		return
	}

	response.Created(c, result)
}

// HandleUpdateIssueType 更新工单类型
// @Summary 更新工单类型
// @Description 更新工单类型信息
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "工单类型 ID"
// @Param request body dto.UpdateIssueTypeRequest true "更新工单类型请求"
// @Success 200 {object} response.Response{data=dto.IssueTypeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/issue-types/{id} [put]
// @Security BearerAuth
func (h *ProjectHandler) HandleUpdateIssueType(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.issue_type_invalid_id")
		return
	}

	var req dto.UpdateIssueTypeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.projectService.UpdateIssueType(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrIssueTypeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.issue_type_update_failed")
		return
	}

	response.Success(c, result)
}

// HandleDeleteIssueType 删除工单类型
// @Summary 删除工单类型
// @Description 删除项目自定义的工单类型
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "工单类型 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/issue-types/{id} [delete]
// @Security BearerAuth
func (h *ProjectHandler) HandleDeleteIssueType(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.issue_type_invalid_id")
		return
	}

	err = h.projectService.DeleteIssueType(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrIssueTypeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.issue_type_deleted")})
}

// HandleListIssueTypes 获取项目工单类型列表
// @Summary 获取项目工单类型列表
// @Description 获取项目可用的所有工单类型（包括全局和项目自定义）
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response{data=[]dto.IssueTypeResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/issue-types [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListIssueTypes(c *gin.Context) {
	key := c.Param("key")

	issueTypes, err := h.projectService.ListIssueTypes(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.issue_type_list_failed")
		return
	}

	response.Success(c, issueTypes)
}

// HandleListAllIssueTypes 获取所有工单类型（用于筛选器）
// @Summary 获取所有工单类型
// @Description 获取所有工单类型列表（全局 + 项目自定义，用于筛选器）
// @Tags IssueType
// @Produce json
// @Success 200 {object} response.Response{data=[]dto.IssueTypeResponse}
// @Router /api/v1/issue-types [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListAllIssueTypes(c *gin.Context) {
	issueTypes, err := h.projectService.ListAllIssueTypes(c.Request.Context())
	if err != nil {
		response.InternalError(c, "project.issue_type_list_failed")
		return
	}

	response.Success(c, issueTypes)
}

// HandleListAllProjects 获取所有项目（用于选择器）
// @Summary 获取所有项目
// @Description 获取所有项目列表（不分页，用于选择器）
// @Tags Project
// @Produce json
// @Success 200 {object} response.Response{data=[]dto.ProjectResponse}
// @Router /api/v1/projects/all [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListAllProjects(c *gin.Context) {
	userID := c.GetUint64("user_id")
	isAdmin := middleware.IsAdmin(c)

	// 使用大页面获取所有项目
	req := &dto.ListProjectsRequest{
		Page:     1,
		PageSize: 1000, // 获取足够多的项目
	}

	projects, _, err := h.projectService.ListProjects(c.Request.Context(), req, userID, isAdmin)
	if err != nil {
		response.InternalError(c, "project.list_failed")
		return
	}

	response.Success(c, projects)
}

// ============ 项目角色管理 ============

// HandleCreateRole 创建项目角色
// @Summary 创建项目角色
// @Description 为项目创建自定义角色
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param request body dto.CreateProjectRoleRequest true "创建角色请求"
// @Success 201 {object} response.Response{data=dto.ProjectRoleResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles [post]
// @Security BearerAuth
func (h *ProjectHandler) HandleCreateRole(c *gin.Context) {
	key := c.Param("key")

	var req dto.CreateProjectRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.projectService.CreateRole(c.Request.Context(), key, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleKeyExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "project.role_create_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleUpdateRole 更新项目角色
// @Summary 更新项目角色
// @Description 更新项目角色信息
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "角色 ID"
// @Param request body dto.UpdateProjectRoleRequest true "更新角色请求"
// @Success 200 {object} response.Response{data=dto.ProjectRoleResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles/{id} [put]
// @Security BearerAuth
func (h *ProjectHandler) HandleUpdateRole(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}

	var req dto.UpdateProjectRoleRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.projectService.UpdateRole(c.Request.Context(), key, roleID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "project.role_update_failed")
		}
		return
	}

	response.Success(c, result)
}

// HandleDeleteRole 删除项目角色
// @Summary 删除项目角色
// @Description 删除项目自定义角色
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "角色 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles/{id} [delete]
// @Security BearerAuth
func (h *ProjectHandler) HandleDeleteRole(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}

	err = h.projectService.DeleteRole(c.Request.Context(), key, roleID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrCannotDeleteSystemRole):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "project.role_delete_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.role_deleted")})
}

// HandleListRoles 获取项目角色列表
// @Summary 获取项目角色列表
// @Description 获取项目的所有角色
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response{data=[]dto.ProjectRoleResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListRoles(c *gin.Context) {
	key := c.Param("key")

	roles, err := h.projectService.ListRoles(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.role_list_failed")
		return
	}

	response.Success(c, roles)
}

// HandleAddRoleMember 添加角色成员
// @Summary 添加角色成员
// @Description 向角色添加成员
// @Tags Project
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "角色 ID"
// @Param request body dto.AddRoleMemberRequest true "添加成员请求"
// @Success 201 {object} response.Response{data=dto.ProjectRoleMemberResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles/{id}/members [post]
// @Security BearerAuth
func (h *ProjectHandler) HandleAddRoleMember(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}

	var req dto.AddRoleMemberRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.projectService.AddRoleMember(c.Request.Context(), key, roleID, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleMemberExists):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, "project.role_member_add_failed")
		}
		return
	}

	response.Created(c, result)
}

// HandleRemoveRoleMember 移除角色成员
// @Summary 移除角色成员
// @Description 从角色中移除成员
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "角色 ID"
// @Param user_id path int true "用户 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles/{id}/members/{user_id} [delete]
// @Security BearerAuth
func (h *ProjectHandler) HandleRemoveRoleMember(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	err = h.projectService.RemoveRoleMember(c.Request.Context(), key, roleID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleMemberNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "project.role_member_remove_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.role_member_removed")})
}

// HandleListRoleMembers 获取角色成员列表
// @Summary 获取角色成员列表
// @Description 获取角色的所有成员
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Param id path int true "角色 ID"
// @Success 200 {object} response.Response{data=[]dto.ProjectRoleMemberResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/roles/{id}/members [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleListRoleMembers(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}

	members, err := h.projectService.ListRoleMembers(c.Request.Context(), key, roleID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "project.role_member_list_failed")
		}
		return
	}

	response.Success(c, members)
}

// HandleGetUserRoles 获取用户在项目中的角色
// @Summary 获取用户角色
// @Description 获取用户在项目中的所有角色
// @Tags Project
// @Produce json
// @Param key path string true "项目 Key"
// @Param user_id path int true "用户 ID"
// @Success 200 {object} response.Response{data=[]dto.ProjectRoleResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/users/{user_id}/roles [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleGetUserRoles(c *gin.Context) {
	key := c.Param("key")
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "user.invalid_id")
		return
	}

	roles, err := h.projectService.GetUserRoles(c.Request.Context(), key, userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "project.user_roles_failed")
		return
	}

	response.Success(c, roles)
}

// HandleGetMyPermissions 获取当前用户在项目中的权限
// @Summary 获取我在项目中的权限
// @Description 返回当前用户在指定项目中的权限集合，供前端决定显示哪些操作入口
// @Tags 项目
// @Produce json
// @Param key path string true "项目标识"
// @Success 200 {object} response.Response{data=dto.MyProjectPermissionsResponse}
// @Router /projects/{key}/my-permissions [get]
// @Security BearerAuth
func (h *ProjectHandler) HandleGetMyPermissions(c *gin.Context) {
	key := c.Param("key")
	userID := c.GetUint64("user_id")
	if userID == 0 {
		response.Unauthorized(c, "perm.no_user")
		return
	}

	perms, err := h.projectService.GetMyPermissions(c.Request.Context(), key, userID, middleware.IsAdmin(c))
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "perm.check_failed")
		return
	}

	response.Success(c, perms)
}

// ============ 角色权限管理 ============

// HandleGetRolePermissions 获取角色权限
func (h *ProjectHandler) HandleGetRolePermissions(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}

	perms, err := h.projectService.GetRolePermissions(c.Request.Context(), key, roleID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "project.perm_get_failed")
		}
		return
	}

	response.Success(c, perms)
}

// HandleSetRolePermissions 设置角色权限
func (h *ProjectHandler) HandleSetRolePermissions(c *gin.Context) {
	key := c.Param("key")
	roleID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.role_invalid_id")
		return
	}

	var req dto.SetRolePermissionsRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	err = h.projectService.SetRolePermissions(c.Request.Context(), key, roleID, req.Permissions)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			response.NotFound(c, err.Error())
		case errors.Is(err, service.ErrRoleNotFound):
			response.NotFound(c, err.Error())
		default:
			response.InternalError(c, "project.perm_set_failed")
		}
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.perm_saved")})
}
