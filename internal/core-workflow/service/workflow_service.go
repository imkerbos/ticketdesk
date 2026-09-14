// Package service 提供工作流业务逻辑层
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	"github.com/kerbos/ticketdesk/internal/core-workflow/dto"
	"github.com/kerbos/ticketdesk/internal/core-workflow/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// 业务错误定义
var (
	ErrWorkflowNotFound = errors.New("workflow.not_found")
	ErrNodeNotFound     = errors.New("workflow.node_not_found")
	ErrEdgeNotFound     = errors.New("workflow.edge_not_found")
	ErrInvalidWorkflow  = errors.New("workflow.config_invalid")
	ErrNodeInUse        = errors.New("workflow.node_in_use")
	ErrSchemeNotFound   = errors.New("workflow.scheme_not_found")
	ErrSchemeExists     = errors.New("workflow.scheme_exists")
)

// WorkflowService 工作流服务接口
type WorkflowService interface {
	// 工作流管理
	CreateWorkflow(ctx context.Context, req *dto.CreateWorkflowRequest) (*dto.WorkflowResponse, error)
	GetWorkflow(ctx context.Context, id uint64) (*dto.WorkflowResponse, error)
	UpdateWorkflow(ctx context.Context, id uint64, req *dto.UpdateWorkflowRequest) (*dto.WorkflowResponse, error)
	DeleteWorkflow(ctx context.Context, id uint64) error
	ListWorkflows(ctx context.Context, req *dto.ListWorkflowsRequest) ([]*dto.WorkflowResponse, int64, error)

	// 节点管理
	CreateNode(ctx context.Context, workflowID uint64, req *dto.CreateNodeRequest) (*dto.NodeResponse, error)
	GetNode(ctx context.Context, id uint64) (*dto.NodeResponse, error)
	UpdateNode(ctx context.Context, id uint64, req *dto.UpdateNodeRequest) (*dto.NodeResponse, error)
	DeleteNode(ctx context.Context, id uint64) error
	ListNodes(ctx context.Context, workflowID uint64) ([]*dto.NodeResponse, error)

	// 边管理
	CreateEdge(ctx context.Context, workflowID uint64, req *dto.CreateEdgeRequest) (*dto.EdgeResponse, error)
	GetEdge(ctx context.Context, id uint64) (*dto.EdgeResponse, error)
	UpdateEdge(ctx context.Context, id uint64, req *dto.UpdateEdgeRequest) (*dto.EdgeResponse, error)
	DeleteEdge(ctx context.Context, id uint64) error
	ListEdges(ctx context.Context, workflowID uint64) ([]*dto.EdgeResponse, error)

	// 工作流方案管理
	CreateScheme(ctx context.Context, projectID uint64, req *dto.CreateWorkflowSchemeRequest) (*dto.WorkflowSchemeResponse, error)
	ListSchemes(ctx context.Context, projectID uint64) ([]*dto.WorkflowSchemeResponse, error)
	DeleteScheme(ctx context.Context, projectID, issueTypeID uint64) error
	GetWorkflowByIssueType(ctx context.Context, projectID, issueTypeID uint64) (*model.Workflow, error)
}

// workflowService 工作流服务实现
type workflowService struct {
	workflowRepo  repository.WorkflowRepository
	nodeRepo      repository.NodeRepository
	edgeRepo      repository.EdgeRepository
	schemeRepo    repository.WorkflowSchemeRepository
	issueTypeRepo projectRepo.IssueTypeRepository
}

// NewWorkflowService 创建工作流服务实例
func NewWorkflowService(
	workflowRepo repository.WorkflowRepository,
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	schemeRepo repository.WorkflowSchemeRepository,
	issueTypeRepo projectRepo.IssueTypeRepository,
) WorkflowService {
	return &workflowService{
		workflowRepo:  workflowRepo,
		nodeRepo:      nodeRepo,
		edgeRepo:      edgeRepo,
		schemeRepo:    schemeRepo,
		issueTypeRepo: issueTypeRepo,
	}
}

// CreateWorkflow 创建工作流
func (s *workflowService) CreateWorkflow(ctx context.Context, req *dto.CreateWorkflowRequest) (*dto.WorkflowResponse, error) {
	workflow := &model.Workflow{
		ProjectID:   req.ProjectID,
		Name:        req.Name,
		Description: req.Description,
		Status:      1, // 默认启用
	}

	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		logger.Error("failed to create workflow", zap.Error(err))
		return nil, fmt.Errorf("创建工作流失败: %w", err)
	}

	// 自动创建开始和结束节点
	startNode := &model.WorkflowNode{
		WorkflowID: workflow.ID,
		Name:       "开始",
		NodeType:   "start",
		Config:     "{}",
		PositionX:  100,
		PositionY:  200,
	}
	if err := s.nodeRepo.Create(ctx, startNode); err != nil {
		logger.Warn("failed to create start node", zap.Error(err))
	}

	endNode := &model.WorkflowNode{
		WorkflowID: workflow.ID,
		Name:       "结束",
		NodeType:   "end",
		Config:     "{}",
		PositionX:  500,
		PositionY:  200,
	}
	if err := s.nodeRepo.Create(ctx, endNode); err != nil {
		logger.Warn("failed to create end node", zap.Error(err))
	}

	logger.Info("workflow created successfully", zap.Uint64("workflow_id", workflow.ID))

	return s.toWorkflowResponse(ctx, workflow, true), nil
}

// GetWorkflow 获取工作流详情
func (s *workflowService) GetWorkflow(ctx context.Context, id uint64) (*dto.WorkflowResponse, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	return s.toWorkflowResponse(ctx, workflow, true), nil
}

// UpdateWorkflow 更新工作流
func (s *workflowService) UpdateWorkflow(ctx context.Context, id uint64, req *dto.UpdateWorkflowRequest) (*dto.WorkflowResponse, error) {
	workflow, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	if req.Name != nil {
		workflow.Name = *req.Name
	}
	if req.Description != nil {
		workflow.Description = *req.Description
	}
	if req.Status != nil {
		workflow.Status = *req.Status
	}

	if err := s.workflowRepo.Update(ctx, workflow); err != nil {
		logger.Error("failed to update workflow", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("更新工作流失败: %w", err)
	}

	logger.Info("workflow updated successfully", zap.Uint64("workflow_id", id))

	return s.toWorkflowResponse(ctx, workflow, true), nil
}

// DeleteWorkflow 删除工作流
func (s *workflowService) DeleteWorkflow(ctx context.Context, id uint64) error {
	_, err := s.workflowRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWorkflowNotFound
		}
		return fmt.Errorf("查询工作流失败: %w", err)
	}

	// 删除所有边
	if err := s.edgeRepo.DeleteByWorkflow(ctx, id); err != nil {
		logger.Warn("failed to delete workflow edges", zap.Error(err))
	}

	// 删除所有节点
	if err := s.nodeRepo.DeleteByWorkflow(ctx, id); err != nil {
		logger.Warn("failed to delete workflow nodes", zap.Error(err))
	}

	// 删除工作流
	if err := s.workflowRepo.Delete(ctx, id); err != nil {
		logger.Error("failed to delete workflow", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("删除工作流失败: %w", err)
	}

	logger.Info("workflow deleted successfully", zap.Uint64("workflow_id", id))

	return nil
}

// ListWorkflows 分页查询工作流列表
func (s *workflowService) ListWorkflows(ctx context.Context, req *dto.ListWorkflowsRequest) ([]*dto.WorkflowResponse, int64, error) {
	page := req.GetDefaultPage()
	pageSize := req.GetDefaultPageSize()
	offset := (page - 1) * pageSize

	filter := &repository.WorkflowFilter{
		ProjectID: req.ProjectID,
		Status:    req.Status,
		Keyword:   req.Keyword,
	}

	workflows, total, err := s.workflowRepo.List(ctx, filter, offset, pageSize)
	if err != nil {
		logger.Error("failed to list workflows", zap.Error(err))
		return nil, 0, fmt.Errorf("查询工作流列表失败: %w", err)
	}

	responses := make([]*dto.WorkflowResponse, len(workflows))
	for i, wf := range workflows {
		responses[i] = s.toWorkflowResponse(ctx, wf, false)
	}

	return responses, total, nil
}

// CreateNode 创建节点
func (s *workflowService) CreateNode(ctx context.Context, workflowID uint64, req *dto.CreateNodeRequest) (*dto.NodeResponse, error) {
	// 验证工作流存在
	_, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	// 序列化配置
	configJSON := "{}"
	if req.Config != nil {
		configBytes, err := json.Marshal(req.Config)
		if err != nil {
			return nil, fmt.Errorf("序列化节点配置失败: %w", err)
		}
		configJSON = string(configBytes)
	}

	node := &model.WorkflowNode{
		WorkflowID: workflowID,
		Name:       req.Name,
		NodeType:   req.NodeType,
		Config:     configJSON,
		PositionX:  req.PositionX,
		PositionY:  req.PositionY,
	}

	if err := s.nodeRepo.Create(ctx, node); err != nil {
		logger.Error("failed to create node", zap.Error(err))
		return nil, fmt.Errorf("创建节点失败: %w", err)
	}

	logger.Info("node created successfully",
		zap.Uint64("workflow_id", workflowID),
		zap.Uint64("node_id", node.ID),
	)

	return s.toNodeResponse(node), nil
}

// GetNode 获取节点详情
func (s *workflowService) GetNode(ctx context.Context, id uint64) (*dto.NodeResponse, error) {
	node, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, fmt.Errorf("查询节点失败: %w", err)
	}

	return s.toNodeResponse(node), nil
}

// UpdateNode 更新节点
func (s *workflowService) UpdateNode(ctx context.Context, id uint64, req *dto.UpdateNodeRequest) (*dto.NodeResponse, error) {
	node, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, fmt.Errorf("查询节点失败: %w", err)
	}

	if req.Name != nil {
		node.Name = *req.Name
	}
	if req.Config != nil {
		configBytes, err := json.Marshal(req.Config)
		if err != nil {
			return nil, fmt.Errorf("序列化节点配置失败: %w", err)
		}
		node.Config = string(configBytes)
	}
	if req.PositionX != nil {
		node.PositionX = *req.PositionX
	}
	if req.PositionY != nil {
		node.PositionY = *req.PositionY
	}

	if err := s.nodeRepo.Update(ctx, node); err != nil {
		logger.Error("failed to update node", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("更新节点失败: %w", err)
	}

	return s.toNodeResponse(node), nil
}

// DeleteNode 删除节点
func (s *workflowService) DeleteNode(ctx context.Context, id uint64) error {
	node, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNodeNotFound
		}
		return fmt.Errorf("查询节点失败: %w", err)
	}

	// 不能删除开始和结束节点
	if node.NodeType == "start" || node.NodeType == "end" {
		return fmt.Errorf("不能删除开始或结束节点")
	}

	// 删除与节点相关的边
	if err := s.edgeRepo.DeleteByNode(ctx, id); err != nil {
		logger.Warn("failed to delete node edges", zap.Error(err))
	}

	if err := s.nodeRepo.Delete(ctx, id); err != nil {
		logger.Error("failed to delete node", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("删除节点失败: %w", err)
	}

	logger.Info("node deleted successfully", zap.Uint64("node_id", id))

	return nil
}

// ListNodes 获取工作流的所有节点
func (s *workflowService) ListNodes(ctx context.Context, workflowID uint64) ([]*dto.NodeResponse, error) {
	// 验证工作流存在
	_, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	nodes, err := s.nodeRepo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("查询节点列表失败: %w", err)
	}

	responses := make([]*dto.NodeResponse, len(nodes))
	for i, node := range nodes {
		responses[i] = s.toNodeResponse(node)
	}

	return responses, nil
}

// CreateEdge 创建边
func (s *workflowService) CreateEdge(ctx context.Context, workflowID uint64, req *dto.CreateEdgeRequest) (*dto.EdgeResponse, error) {
	// 验证工作流存在
	_, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	// 验证源节点存在
	sourceNode, err := s.nodeRepo.GetByID(ctx, req.SourceNodeID)
	if err != nil {
		return nil, fmt.Errorf("源节点不存在")
	}
	if sourceNode.WorkflowID != workflowID {
		return nil, fmt.Errorf("源节点不属于该工作流")
	}

	// 验证目标节点存在
	targetNode, err := s.nodeRepo.GetByID(ctx, req.TargetNodeID)
	if err != nil {
		return nil, fmt.Errorf("目标节点不存在")
	}
	if targetNode.WorkflowID != workflowID {
		return nil, fmt.Errorf("目标节点不属于该工作流")
	}

	edge := &model.WorkflowEdge{
		WorkflowID:    workflowID,
		SourceNodeID:  req.SourceNodeID,
		TargetNodeID:  req.TargetNodeID,
		ConditionExpr: req.ConditionExpr,
	}

	if err := s.edgeRepo.Create(ctx, edge); err != nil {
		logger.Error("failed to create edge", zap.Error(err))
		return nil, fmt.Errorf("创建边失败: %w", err)
	}

	logger.Info("edge created successfully",
		zap.Uint64("workflow_id", workflowID),
		zap.Uint64("edge_id", edge.ID),
	)

	return s.toEdgeResponse(edge), nil
}

// GetEdge 获取边详情
func (s *workflowService) GetEdge(ctx context.Context, id uint64) (*dto.EdgeResponse, error) {
	edge, err := s.edgeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEdgeNotFound
		}
		return nil, fmt.Errorf("查询边失败: %w", err)
	}

	return s.toEdgeResponse(edge), nil
}

// UpdateEdge 更新边
func (s *workflowService) UpdateEdge(ctx context.Context, id uint64, req *dto.UpdateEdgeRequest) (*dto.EdgeResponse, error) {
	edge, err := s.edgeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEdgeNotFound
		}
		return nil, fmt.Errorf("查询边失败: %w", err)
	}

	if req.ConditionExpr != nil {
		edge.ConditionExpr = *req.ConditionExpr
	}

	if err := s.edgeRepo.Update(ctx, edge); err != nil {
		logger.Error("failed to update edge", zap.Uint64("id", id), zap.Error(err))
		return nil, fmt.Errorf("更新边失败: %w", err)
	}

	return s.toEdgeResponse(edge), nil
}

// DeleteEdge 删除边
func (s *workflowService) DeleteEdge(ctx context.Context, id uint64) error {
	_, err := s.edgeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrEdgeNotFound
		}
		return fmt.Errorf("查询边失败: %w", err)
	}

	if err := s.edgeRepo.Delete(ctx, id); err != nil {
		logger.Error("failed to delete edge", zap.Uint64("id", id), zap.Error(err))
		return fmt.Errorf("删除边失败: %w", err)
	}

	logger.Info("edge deleted successfully", zap.Uint64("edge_id", id))

	return nil
}

// ListEdges 获取工作流的所有边
func (s *workflowService) ListEdges(ctx context.Context, workflowID uint64) ([]*dto.EdgeResponse, error) {
	// 验证工作流存在
	_, err := s.workflowRepo.GetByID(ctx, workflowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	edges, err := s.edgeRepo.ListByWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("查询边列表失败: %w", err)
	}

	responses := make([]*dto.EdgeResponse, len(edges))
	for i, edge := range edges {
		responses[i] = s.toEdgeResponse(edge)
	}

	return responses, nil
}

// toWorkflowResponse 将工作流模型转换为响应 DTO
func (s *workflowService) toWorkflowResponse(ctx context.Context, workflow *model.Workflow, loadDetails bool) *dto.WorkflowResponse {
	resp := &dto.WorkflowResponse{
		ID:          workflow.ID,
		ProjectID:   workflow.ProjectID,
		Name:        workflow.Name,
		Description: workflow.Description,
		Status:      workflow.Status,
		CreatedAt:   workflow.CreatedAt,
		UpdatedAt:   workflow.UpdatedAt,
	}

	if loadDetails {
		// 加载节点
		if nodes, err := s.nodeRepo.ListByWorkflow(ctx, workflow.ID); err == nil {
			resp.Nodes = make([]*dto.NodeResponse, len(nodes))
			for i, node := range nodes {
				resp.Nodes[i] = s.toNodeResponse(node)
			}
		}

		// 加载边
		if edges, err := s.edgeRepo.ListByWorkflow(ctx, workflow.ID); err == nil {
			resp.Edges = make([]*dto.EdgeResponse, len(edges))
			for i, edge := range edges {
				resp.Edges[i] = s.toEdgeResponse(edge)
			}
		}
	}

	return resp
}

// toNodeResponse 将节点模型转换为响应 DTO
func (s *workflowService) toNodeResponse(node *model.WorkflowNode) *dto.NodeResponse {
	resp := &dto.NodeResponse{
		ID:         node.ID,
		WorkflowID: node.WorkflowID,
		Name:       node.Name,
		NodeType:   node.NodeType,
		PositionX:  node.PositionX,
		PositionY:  node.PositionY,
		CreatedAt:  node.CreatedAt,
		UpdatedAt:  node.UpdatedAt,
	}

	// 解析配置
	if node.Config != "" {
		var config dto.NodeConfig
		if err := json.Unmarshal([]byte(node.Config), &config); err == nil {
			resp.Config = &config
		}
	}

	return resp
}

// toEdgeResponse 将边模型转换为响应 DTO
func (s *workflowService) toEdgeResponse(edge *model.WorkflowEdge) *dto.EdgeResponse {
	return &dto.EdgeResponse{
		ID:            edge.ID,
		WorkflowID:    edge.WorkflowID,
		SourceNodeID:  edge.SourceNodeID,
		TargetNodeID:  edge.TargetNodeID,
		ConditionExpr: edge.ConditionExpr,
		CreatedAt:     edge.CreatedAt,
		UpdatedAt:     edge.UpdatedAt,
	}
}

// ============ 工作流方案管理 ============

// CreateScheme 创建工作流方案
func (s *workflowService) CreateScheme(ctx context.Context, projectID uint64, req *dto.CreateWorkflowSchemeRequest) (*dto.WorkflowSchemeResponse, error) {
	// 检查是否已存在方案（未删除的）
	existing, err := s.schemeRepo.GetByProjectAndIssueType(ctx, projectID, req.IssueTypeID)
	if err == nil && existing != nil {
		return nil, ErrSchemeExists
	}

	// 清理可能存在的软删除记录，避免唯一索引冲突
	if cleanupErr := s.schemeRepo.HardDeleteByProjectAndIssueType(ctx, projectID, req.IssueTypeID); cleanupErr != nil {
		logger.Error("failed to hard delete soft-deleted workflow scheme", zap.Error(cleanupErr))
	}

	// 验证工作流是否存在
	workflow, err := s.workflowRepo.GetByID(ctx, req.WorkflowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	// 创建方案
	scheme := &model.WorkflowScheme{
		ProjectID:   projectID,
		IssueTypeID: req.IssueTypeID,
		WorkflowID:  req.WorkflowID,
	}

	if err := s.schemeRepo.Create(ctx, scheme); err != nil {
		logger.Error("failed to create workflow scheme", zap.Error(err))
		return nil, fmt.Errorf("创建工作流方案失败: %w", err)
	}

	logger.Info("workflow scheme created",
		zap.Uint64("project_id", projectID),
		zap.Uint64("issue_type_id", req.IssueTypeID),
		zap.Uint64("workflow_id", req.WorkflowID),
	)

	return &dto.WorkflowSchemeResponse{
		ID:            scheme.ID,
		ProjectID:     scheme.ProjectID,
		IssueTypeID:   scheme.IssueTypeID,
		IssueTypeName: s.getIssueTypeName(ctx, scheme.IssueTypeID),
		WorkflowID:    scheme.WorkflowID,
		WorkflowName:  workflow.Name,
		CreatedAt:     scheme.CreatedAt,
		UpdatedAt:     scheme.UpdatedAt,
	}, nil
}

// getIssueTypeName 获取工单类型显示名称
func (s *workflowService) getIssueTypeName(ctx context.Context, issueTypeID uint64) string {
	if issueType, err := s.issueTypeRepo.GetByID(ctx, issueTypeID); err == nil {
		if issueType.DisplayName != "" {
			return issueType.DisplayName
		}
		return issueType.Name
	}
	return ""
}

// ListSchemes 获取项目的工作流方案列表
func (s *workflowService) ListSchemes(ctx context.Context, projectID uint64) ([]*dto.WorkflowSchemeResponse, error) {
	schemes, err := s.schemeRepo.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("查询工作流方案失败: %w", err)
	}

	responses := make([]*dto.WorkflowSchemeResponse, len(schemes))
	for i, scheme := range schemes {
		resp := &dto.WorkflowSchemeResponse{
			ID:          scheme.ID,
			ProjectID:   scheme.ProjectID,
			IssueTypeID: scheme.IssueTypeID,
			WorkflowID:  scheme.WorkflowID,
			CreatedAt:   scheme.CreatedAt,
			UpdatedAt:   scheme.UpdatedAt,
		}

		// 获取工作流名称
		if workflow, err := s.workflowRepo.GetByID(ctx, scheme.WorkflowID); err == nil {
			resp.WorkflowName = workflow.Name
		}

		// 获取工单类型名称
		resp.IssueTypeName = s.getIssueTypeName(ctx, scheme.IssueTypeID)

		responses[i] = resp
	}

	return responses, nil
}

// DeleteScheme 删除工作流方案
func (s *workflowService) DeleteScheme(ctx context.Context, projectID, issueTypeID uint64) error {
	// 检查方案是否存在
	_, err := s.schemeRepo.GetByProjectAndIssueType(ctx, projectID, issueTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSchemeNotFound
		}
		return fmt.Errorf("查询工作流方案失败: %w", err)
	}

	if err := s.schemeRepo.DeleteByProjectAndIssueType(ctx, projectID, issueTypeID); err != nil {
		logger.Error("failed to delete workflow scheme", zap.Error(err))
		return fmt.Errorf("删除工作流方案失败: %w", err)
	}

	logger.Info("workflow scheme deleted",
		zap.Uint64("project_id", projectID),
		zap.Uint64("issue_type_id", issueTypeID),
	)

	return nil
}

// GetWorkflowByIssueType 根据项目和工单类型获取工作流
func (s *workflowService) GetWorkflowByIssueType(ctx context.Context, projectID, issueTypeID uint64) (*model.Workflow, error) {
	scheme, err := s.schemeRepo.GetByProjectAndIssueType(ctx, projectID, issueTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSchemeNotFound
		}
		return nil, fmt.Errorf("查询工作流方案失败: %w", err)
	}

	workflow, err := s.workflowRepo.GetByID(ctx, scheme.WorkflowID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, fmt.Errorf("查询工作流失败: %w", err)
	}

	return workflow, nil
}
