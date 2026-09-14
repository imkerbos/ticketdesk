// Package handler 提供工作流模块的 HTTP 处理器
package handler

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/activity/detail"
	"github.com/kerbos/ticketdesk/internal/api/response"
	issueRepo "github.com/kerbos/ticketdesk/internal/core-issue/repository"
	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	"github.com/kerbos/ticketdesk/internal/core-workflow/dto"
	"github.com/kerbos/ticketdesk/internal/core-workflow/service"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// ActivityLogger 活动日志记录接口
type ActivityLogger interface {
	LogActivity(ctx context.Context, userID uint64, userName, action, entityType string, entityID uint64, entityKey, details string) error
}

// WorkflowHandler 工作流处理器
type WorkflowHandler struct {
	workflowService service.WorkflowService
	workflowEngine  service.WorkflowEngine
	issueRepo       issueRepo.IssueRepository
	projectRepo     projectRepo.ProjectRepository
	activityLogger  ActivityLogger
}

// NewWorkflowHandler 创建工作流处理器实例
func NewWorkflowHandler(
	workflowService service.WorkflowService,
	workflowEngine service.WorkflowEngine,
	issueRepository issueRepo.IssueRepository,
	projectRepository projectRepo.ProjectRepository,
	activityLogger ActivityLogger,
) *WorkflowHandler {
	return &WorkflowHandler{
		workflowService: workflowService,
		workflowEngine:  workflowEngine,
		issueRepo:       issueRepository,
		projectRepo:     projectRepository,
		activityLogger:  activityLogger,
	}
}

// logActivity 异步记录活动日志（不阻塞主流程）
func (h *WorkflowHandler) logActivity(userID uint64, userName, action, entityKey, details string, entityID uint64) {
	if h.activityLogger == nil {
		return
	}
	go func() {
		defer safego.Recover("workflow.logActivityHandler")
		if err := h.activityLogger.LogActivity(context.Background(), userID, userName, action, "issue", entityID, entityKey, details); err != nil {
			logger.Warn("failed to log workflow activity", zap.Error(err))
		}
	}()
}

// ============ 工作流管理 ============

// HandleCreateWorkflow 创建工作流
// @Summary 创建工作流
// @Description 创建新的工作流定义
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.CreateWorkflowRequest true "创建工作流请求"
// @Success 201 {object} response.Response{data=dto.WorkflowResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/v1/workflows [post]
func (h *WorkflowHandler) HandleCreateWorkflow(c *gin.Context) {
	var req dto.CreateWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.workflowService.CreateWorkflow(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "workflow.create_failed")
		return
	}

	response.Created(c, result)
}

// HandleGetWorkflow 获取工作流详情
// @Summary 获取工作流详情
// @Description 根据 ID 获取工作流定义，包含节点和边
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Success 200 {object} response.Response{data=dto.WorkflowResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id} [get]
func (h *WorkflowHandler) HandleGetWorkflow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	result, err := h.workflowService.GetWorkflow(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.get_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateWorkflow 更新工作流
// @Summary 更新工作流
// @Description 更新工作流名称、描述或状态
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param request body dto.UpdateWorkflowRequest true "更新工作流请求"
// @Success 200 {object} response.Response{data=dto.WorkflowResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id} [put]
func (h *WorkflowHandler) HandleUpdateWorkflow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	var req dto.UpdateWorkflowRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.workflowService.UpdateWorkflow(c.Request.Context(), id, &req)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.update_failed")
		return
	}

	response.Success(c, result)
}

// HandleDeleteWorkflow 删除工作流
// @Summary 删除工作流
// @Description 根据 ID 删除工作流（需要管理员权限）
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id} [delete]
func (h *WorkflowHandler) HandleDeleteWorkflow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	err = h.workflowService.DeleteWorkflow(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.delete_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "workflow.deleted")})
}

// HandleListWorkflows 获取工作流列表
// @Summary 获取工作流列表
// @Description 分页查询工作流，支持按项目、状态、关键字筛选
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码（默认 1）"
// @Param page_size query int false "每页数量（默认 20，最大 100）"
// @Param project_id query int false "项目 ID"
// @Param status query int false "状态（0=禁用，1=启用）"
// @Param keyword query string false "关键字搜索"
// @Success 200 {object} response.Response{data=response.PageData}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Router /api/v1/workflows [get]
func (h *WorkflowHandler) HandleListWorkflows(c *gin.Context) {
	var req dto.ListWorkflowsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	workflows, total, err := h.workflowService.ListWorkflows(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "workflow.list_failed")
		return
	}

	response.SuccessWithPage(c, workflows, total, req.GetDefaultPage(), req.GetDefaultPageSize())
}

// ============ 节点管理 ============

// HandleCreateNode 创建节点
// @Summary 创建工作流节点
// @Description 在指定工作流中创建新节点
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param request body dto.CreateNodeRequest true "创建节点请求"
// @Success 201 {object} response.Response{data=dto.NodeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/nodes [post]
func (h *WorkflowHandler) HandleCreateNode(c *gin.Context) {
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	var req dto.CreateNodeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.workflowService.CreateNode(c.Request.Context(), workflowID, &req)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.node_create_failed")
		return
	}

	response.Created(c, result)
}

// HandleGetNode 获取节点详情
// @Summary 获取工作流节点详情
// @Description 根据节点 ID 获取节点配置
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param node_id path int true "节点 ID"
// @Success 200 {object} response.Response{data=dto.NodeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/nodes/{node_id} [get]
func (h *WorkflowHandler) HandleGetNode(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("node_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.node_invalid_id")
		return
	}

	result, err := h.workflowService.GetNode(c.Request.Context(), nodeID)
	if err != nil {
		if errors.Is(err, service.ErrNodeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.node_get_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateNode 更新节点
// @Summary 更新工作流节点
// @Description 更新节点名称、配置或位置
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param node_id path int true "节点 ID"
// @Param request body dto.UpdateNodeRequest true "更新节点请求"
// @Success 200 {object} response.Response{data=dto.NodeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/nodes/{node_id} [put]
func (h *WorkflowHandler) HandleUpdateNode(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("node_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.node_invalid_id")
		return
	}

	var req dto.UpdateNodeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.workflowService.UpdateNode(c.Request.Context(), nodeID, &req)
	if err != nil {
		if errors.Is(err, service.ErrNodeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.node_update_failed")
		return
	}

	response.Success(c, result)
}

// HandleDeleteNode 删除节点
// @Summary 删除工作流节点
// @Description 根据节点 ID 删除节点
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param node_id path int true "节点 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/nodes/{node_id} [delete]
func (h *WorkflowHandler) HandleDeleteNode(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Param("node_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.node_invalid_id")
		return
	}

	err = h.workflowService.DeleteNode(c.Request.Context(), nodeID)
	if err != nil {
		if errors.Is(err, service.ErrNodeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "workflow.node_deleted")})
}

// HandleListNodes 获取工作流的所有节点
// @Summary 获取工作流节点列表
// @Description 获取指定工作流的所有节点
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Success 200 {object} response.Response{data=[]dto.NodeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/nodes [get]
func (h *WorkflowHandler) HandleListNodes(c *gin.Context) {
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	nodes, err := h.workflowService.ListNodes(c.Request.Context(), workflowID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.node_list_failed")
		return
	}

	response.Success(c, nodes)
}

// ============ 边管理 ============

// HandleCreateEdge 创建边
// @Summary 创建工作流边
// @Description 在指定工作流中创建节点之间的连接边
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param request body dto.CreateEdgeRequest true "创建边请求"
// @Success 201 {object} response.Response{data=dto.EdgeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/edges [post]
func (h *WorkflowHandler) HandleCreateEdge(c *gin.Context) {
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	var req dto.CreateEdgeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.workflowService.CreateEdge(c.Request.Context(), workflowID, &req)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, result)
}

// HandleGetEdge 获取边详情
// @Summary 获取工作流边详情
// @Description 根据边 ID 获取边配置
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param edge_id path int true "边 ID"
// @Success 200 {object} response.Response{data=dto.EdgeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/edges/{edge_id} [get]
func (h *WorkflowHandler) HandleGetEdge(c *gin.Context) {
	edgeID, err := strconv.ParseUint(c.Param("edge_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.edge_invalid_id")
		return
	}

	result, err := h.workflowService.GetEdge(c.Request.Context(), edgeID)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.edge_get_failed")
		return
	}

	response.Success(c, result)
}

// HandleUpdateEdge 更新边
// @Summary 更新工作流边
// @Description 更新边的条件表达式
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param edge_id path int true "边 ID"
// @Param request body dto.UpdateEdgeRequest true "更新边请求"
// @Success 200 {object} response.Response{data=dto.EdgeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/edges/{edge_id} [put]
func (h *WorkflowHandler) HandleUpdateEdge(c *gin.Context) {
	edgeID, err := strconv.ParseUint(c.Param("edge_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.edge_invalid_id")
		return
	}

	var req dto.UpdateEdgeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	result, err := h.workflowService.UpdateEdge(c.Request.Context(), edgeID, &req)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.edge_update_failed")
		return
	}

	response.Success(c, result)
}

// HandleDeleteEdge 删除边
// @Summary 删除工作流边
// @Description 根据边 ID 删除节点间的连接
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Param edge_id path int true "边 ID"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/edges/{edge_id} [delete]
func (h *WorkflowHandler) HandleDeleteEdge(c *gin.Context) {
	edgeID, err := strconv.ParseUint(c.Param("edge_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.edge_invalid_id")
		return
	}

	err = h.workflowService.DeleteEdge(c.Request.Context(), edgeID)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.edge_delete_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "workflow.edge_deleted")})
}

// HandleListEdges 获取工作流的所有边
// @Summary 获取工作流边列表
// @Description 获取指定工作流的所有边
// @Tags Workflow
// @Produce json
// @Security BearerAuth
// @Param id path int true "工作流 ID"
// @Success 200 {object} response.Response{data=[]dto.EdgeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/workflows/{id}/edges [get]
func (h *WorkflowHandler) HandleListEdges(c *gin.Context) {
	workflowID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "workflow.invalid_id")
		return
	}

	edges, err := h.workflowService.ListEdges(c.Request.Context(), workflowID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, err.Error())
			return
		}
		response.InternalError(c, "workflow.edge_list_failed")
		return
	}

	response.Success(c, edges)
}

// ============ 工作流实例管理 ============

// HandleGetInstanceByIssue 获取工单的工作流实例
// @Summary 获取工单的工作流实例
// @Description 根据工单 Key 获取工作流实例
// @Tags Workflow
// @Produce json
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response{data=dto.WorkflowInstanceResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/issues/{key}/workflow [get]
// @Security BearerAuth
func (h *WorkflowHandler) HandleGetInstanceByIssue(c *gin.Context) {
	issueKey := c.Param("key")
	if issueKey == "" {
		response.BadRequest(c, "workflow.issue_key_required")
		return
	}

	// 通过 issue key 获取 issue
	issue, err := h.issueRepo.GetByKey(c.Request.Context(), issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.issue_not_found")
			return
		}
		response.InternalError(c, "workflow.issue_query_failed")
		return
	}

	// 获取工作流实例
	instance, err := h.workflowEngine.GetInstanceByIssueID(c.Request.Context(), issue.ID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowInstanceNotFound) {
			// 尝试懒加载创建：如果该工单的项目+类型配置了工作流方案，自动补建实例
			newInstance, createErr := h.workflowEngine.TryCreateInstanceForIssue(
				c.Request.Context(), issue.ID, issue.ProjectID, issue.IssueTypeID,
			)
			if createErr != nil || newInstance == nil {
				response.NotFound(c, "workflow.no_instance")
				return
			}

			// 更新工单的 WorkflowInstanceID
			issue.WorkflowInstanceID = &newInstance.ID
			if updateErr := h.issueRepo.Update(c.Request.Context(), issue); updateErr != nil {
				logger.Warn("failed to update issue workflow_instance_id", zap.Error(updateErr))
			}

			// 重新获取完整的实例响应
			instance, err = h.workflowEngine.GetInstanceByIssueID(c.Request.Context(), issue.ID)
			if err != nil {
				response.InternalError(c, "workflow.instance_failed")
				return
			}
		} else {
			response.InternalError(c, "workflow.instance_failed")
			return
		}
	}

	// 填充 IssueKey
	instance.IssueKey = issueKey

	response.Success(c, instance)
}

// HandleApprove 审批通过
// @Summary 审批通过
// @Description 对工作流实例进行审批通过操作
// @Tags Workflow
// @Accept json
// @Produce json
// @Param key path string true "工单 Key"
// @Param request body dto.ApproveRequest true "审批请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/issues/{key}/workflow/approve [post]
// @Security BearerAuth
func (h *WorkflowHandler) HandleApprove(c *gin.Context) {
	issueKey := c.Param("key")
	if issueKey == "" {
		response.BadRequest(c, "workflow.issue_key_required")
		return
	}

	// 获取当前用户 ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	// 解析请求体
	var req dto.ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	// 通过 issue key 获取 issue
	issue, err := h.issueRepo.GetByKey(c.Request.Context(), issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.issue_not_found")
			return
		}
		response.InternalError(c, "workflow.issue_query_failed")
		return
	}

	// 获取工作流实例
	instance, err := h.workflowEngine.GetInstanceByIssueID(c.Request.Context(), issue.ID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowInstanceNotFound) {
			response.NotFound(c, "workflow.no_instance")
			return
		}
		response.InternalError(c, "workflow.instance_failed")
		return
	}

	// 执行审批通过
	uid := userID.(uint64)
	if err := h.workflowEngine.Approve(c.Request.Context(), instance.ID, uid, req.Comment); err != nil {
		if errors.Is(err, service.ErrNotApprover) {
			response.Forbidden(c, "workflow.not_approver")
			return
		}
		if errors.Is(err, service.ErrAlreadyApproved) {
			response.BadRequest(c, "workflow.already_approved")
			return
		}
		if errors.Is(err, service.ErrNotApprovalNode) {
			response.BadRequest(c, "workflow.approve_wrong_node")
			return
		}
		logger.Error("approve operation failed", zap.Error(err))
		response.InternalError(c, response.T(c, "workflow.approve_failed")+err.Error())
		return
	}

	// 记录活动日志
	userName, _ := c.Get("username")
	nodeName := ""
	if instance.CurrentNode != nil {
		nodeName = instance.CurrentNode.Name
	}
	details := detail.New(commentedKey("activity.detail.workflowNode", req.Comment),
		"node", nodeName, "comment", req.Comment)
	h.logActivity(uid, fmt.Sprint(userName), "approval_approved", issueKey, details, issue.ID)

	response.Success(c, gin.H{"message": response.T(c, "workflow.approved")})
}

// commentedKey 在「带评论」和「不带评论」两条文案之间选一条。
// 没有评论时不能沿用同一条文案 —— 末尾会留一个孤零零的破折号。
func commentedKey(base, comment string) string {
	if comment == "" {
		return base
	}
	return base + "WithComment"
}

// HandleReject 审批拒绝
// @Summary 审批拒绝
// @Description 对工作流实例进行审批拒绝操作
// @Tags Workflow
// @Accept json
// @Produce json
// @Param key path string true "工单 Key"
// @Param request body dto.RejectRequest true "拒绝请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/issues/{key}/workflow/reject [post]
// @Security BearerAuth
func (h *WorkflowHandler) HandleReject(c *gin.Context) {
	issueKey := c.Param("key")
	if issueKey == "" {
		response.BadRequest(c, "workflow.issue_key_required")
		return
	}

	// 获取当前用户 ID
	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	// 解析请求体
	var req dto.RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	// 通过 issue key 获取 issue
	issue, err := h.issueRepo.GetByKey(c.Request.Context(), issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.issue_not_found")
			return
		}
		response.InternalError(c, "workflow.issue_query_failed")
		return
	}

	// 获取工作流实例
	instance, err := h.workflowEngine.GetInstanceByIssueID(c.Request.Context(), issue.ID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowInstanceNotFound) {
			response.NotFound(c, "workflow.no_instance")
			return
		}
		response.InternalError(c, "workflow.instance_failed")
		return
	}

	// 执行审批拒绝
	uid := userID.(uint64)
	if err := h.workflowEngine.Reject(c.Request.Context(), instance.ID, uid, req.Comment); err != nil {
		if errors.Is(err, service.ErrNotApprover) {
			response.Forbidden(c, "workflow.not_approver")
			return
		}
		if errors.Is(err, service.ErrAlreadyApproved) {
			response.BadRequest(c, "workflow.already_approved")
			return
		}
		if errors.Is(err, service.ErrNotApprovalNode) {
			response.BadRequest(c, "workflow.reject_wrong_node")
			return
		}
		logger.Error("reject operation failed", zap.Error(err))
		response.InternalError(c, response.T(c, "workflow.reject_failed")+err.Error())
		return
	}

	// 记录活动日志
	userName, _ := c.Get("username")
	nodeName := ""
	if instance.CurrentNode != nil {
		nodeName = instance.CurrentNode.Name
	}
	details := detail.New(commentedKey("activity.detail.workflowNode", req.Comment),
		"node", nodeName, "comment", req.Comment)
	h.logActivity(uid, fmt.Sprint(userName), "approval_rejected", issueKey, details, issue.ID)

	response.Success(c, gin.H{"message": response.T(c, "workflow.rejected")})
}

// HandleComplete 完成工作节点
// @Summary 完成工作节点
// @Description 操作人完成当前工作节点，推进工作流流转
// @Tags Workflow
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param key path string true "工单 Key"
// @Param request body dto.CompleteRequest true "完成节点请求"
// @Success 200 {object} response.Response
// @Failure 400 {object} response.ErrorResponse
// @Failure 401 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/issues/{key}/workflow/complete [post]
func (h *WorkflowHandler) HandleComplete(c *gin.Context) {
	issueKey := c.Param("key")
	if issueKey == "" {
		response.BadRequest(c, "workflow.issue_key_required")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user.info_missing")
		return
	}

	var req dto.CompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	issue, err := h.issueRepo.GetByKey(c.Request.Context(), issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.issue_not_found")
			return
		}
		response.InternalError(c, "workflow.issue_query_failed")
		return
	}

	instance, err := h.workflowEngine.GetInstanceByIssueID(c.Request.Context(), issue.ID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowInstanceNotFound) {
			response.NotFound(c, "workflow.no_instance")
			return
		}
		response.InternalError(c, "workflow.instance_failed")
		return
	}

	uid := userID.(uint64)
	if err := h.workflowEngine.Complete(c.Request.Context(), instance.ID, uid, req.Comment, req.Result); err != nil {
		response.InternalError(c, response.T(c, "workflow.complete_failed")+err.Error())
		return
	}

	// 记录活动日志
	userName, _ := c.Get("username")
	nodeName := ""
	if instance.CurrentNode != nil {
		nodeName = instance.CurrentNode.Name
	}
	// 预置条件值走语言包，自定义分支名是用户自己填的，原样带过去
	resultKey := "activity.detail.result.done"
	resultText := ""
	switch req.Result {
	case "":
		// 保持 done
	case "approved":
		resultKey = "activity.detail.result.approved"
	case "rejected":
		resultKey = "activity.detail.result.rejected"
	default:
		resultKey = ""
		resultText = req.Result
	}
	resultKeys := map[string]string{}
	if resultKey != "" {
		resultKeys["result"] = resultKey
	}
	details := detail.NewWithKeys(commentedKey("activity.detail.workflowNodeResult", req.Comment),
		resultKeys, "node", nodeName, "result", resultText, "comment", req.Comment)
	h.logActivity(uid, fmt.Sprint(userName), "node_completed", issueKey, details, issue.ID)

	response.Success(c, gin.H{"message": response.T(c, "workflow.work_node_done")})
}

// HandleGetHistory 获取流转历史
// @Summary 获取流转历史
// @Description 获取工作流实例的流转历史
// @Tags Workflow
// @Produce json
// @Param key path string true "工单 Key"
// @Success 200 {object} response.Response{data=[]dto.WorkflowHistoryResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/issues/{key}/workflow/history [get]
// @Security BearerAuth
func (h *WorkflowHandler) HandleGetHistory(c *gin.Context) {
	issueKey := c.Param("key")
	if issueKey == "" {
		response.BadRequest(c, "workflow.issue_key_required")
		return
	}

	// 通过 issue key 获取 issue
	issue, err := h.issueRepo.GetByKey(c.Request.Context(), issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.issue_not_found")
			return
		}
		response.InternalError(c, "workflow.issue_query_failed")
		return
	}

	// 获取工作流实例
	instance, err := h.workflowEngine.GetInstanceByIssueID(c.Request.Context(), issue.ID)
	if err != nil {
		if errors.Is(err, service.ErrWorkflowInstanceNotFound) {
			response.NotFound(c, "workflow.no_instance")
			return
		}
		response.InternalError(c, "workflow.instance_failed")
		return
	}

	// 获取流转历史
	history, err := h.workflowEngine.GetHistory(c.Request.Context(), instance.ID)
	if err != nil {
		response.InternalError(c, "workflow.history_failed")
		return
	}

	response.Success(c, history)
}

// ============ 工作流方案管理 ============

// HandleCreateScheme 创建工作流方案
// @Summary 创建工作流方案
// @Description 为项目的工单类型配置工作流
// @Tags Workflow
// @Accept json
// @Produce json
// @Param key path string true "项目 Key"
// @Param request body dto.CreateWorkflowSchemeRequest true "创建方案请求"
// @Success 201 {object} response.Response{data=dto.WorkflowSchemeResponse}
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/workflow-schemes [post]
// @Security BearerAuth
func (h *WorkflowHandler) HandleCreateScheme(c *gin.Context) {
	projectKey := c.Param("key")
	if projectKey == "" {
		response.BadRequest(c, "workflow.project_key_required")
		return
	}

	// 通过项目 key 获取项目
	project, err := h.projectRepo.GetByKey(c.Request.Context(), projectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.project_not_found")
			return
		}
		response.InternalError(c, "workflow.project_query_failed")
		return
	}

	// 解析请求体
	var req dto.CreateWorkflowSchemeRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequestValidation(c, bindErr)
		return
	}

	// 创建方案
	result, err := h.workflowService.CreateScheme(c.Request.Context(), project.ID, &req)
	if err != nil {
		if errors.Is(err, service.ErrSchemeExists) {
			response.BadRequest(c, "workflow.scheme_type_taken")
			return
		}
		if errors.Is(err, service.ErrWorkflowNotFound) {
			response.NotFound(c, "workflow.not_found")
			return
		}
		response.InternalError(c, "workflow.scheme_create_failed")
		return
	}

	response.Created(c, result)
}

// HandleListSchemes 获取项目的工作流方案列表
// @Summary 获取工作流方案列表
// @Description 获取项目的所有工作流方案
// @Tags Workflow
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response{data=[]dto.WorkflowSchemeResponse}
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/workflow-schemes [get]
// @Security BearerAuth
func (h *WorkflowHandler) HandleListSchemes(c *gin.Context) {
	projectKey := c.Param("key")
	if projectKey == "" {
		response.BadRequest(c, "workflow.project_key_required")
		return
	}

	// 通过项目 key 获取项目
	project, err := h.projectRepo.GetByKey(c.Request.Context(), projectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.project_not_found")
			return
		}
		response.InternalError(c, "workflow.project_query_failed")
		return
	}

	// 获取方案列表
	schemes, err := h.workflowService.ListSchemes(c.Request.Context(), project.ID)
	if err != nil {
		response.InternalError(c, "workflow.scheme_list_failed")
		return
	}

	response.Success(c, schemes)
}

// HandleDeleteScheme 删除工作流方案
// @Summary 删除工作流方案
// @Description 删除项目工单类型的工作流方案
// @Tags Workflow
// @Produce json
// @Param key path string true "项目 Key"
// @Param type_id path int true "工单类型 ID"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/workflow-schemes/{type_id} [delete]
// @Security BearerAuth
func (h *WorkflowHandler) HandleDeleteScheme(c *gin.Context) {
	projectKey := c.Param("key")
	if projectKey == "" {
		response.BadRequest(c, "workflow.project_key_required")
		return
	}

	typeID, err := strconv.ParseUint(c.Param("type_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "project.issue_type_invalid_id")
		return
	}

	// 通过项目 key 获取项目
	project, err := h.projectRepo.GetByKey(c.Request.Context(), projectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "workflow.project_not_found")
			return
		}
		response.InternalError(c, "workflow.project_query_failed")
		return
	}

	// 删除方案
	if err := h.workflowService.DeleteScheme(c.Request.Context(), project.ID, typeID); err != nil {
		if errors.Is(err, service.ErrSchemeNotFound) {
			response.NotFound(c, "workflow.scheme_not_found")
			return
		}
		response.InternalError(c, "workflow.scheme_delete_failed")
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "workflow.scheme_deleted")})
}
