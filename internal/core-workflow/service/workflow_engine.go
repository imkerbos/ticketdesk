// Package service 提供工作流引擎服务
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/activity/detail"
	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	userRepo "github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/core-workflow/dto"
	"github.com/kerbos/ticketdesk/internal/core-workflow/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
)

// IssueStatusSyncer 工单状态同步接口（用于告警联动，避免循环依赖）
type IssueStatusSyncer interface {
	SyncIssueStatus(ctx context.Context, issueID uint64, issueStatus string) error
	SyncMergedIssueStatus(ctx context.Context, issueID uint64, issueStatus string) error
}

// 工作流引擎错误定义
var (
	ErrWorkflowInstanceNotFound = errors.New("workflow.instance_not_found")
	ErrNotApprover              = errors.New("workflow.not_approver")
	ErrAlreadyApproved          = errors.New("workflow.already_approved")
	ErrNoApproversConfigured    = errors.New("workflow.no_approver")
	// ErrNotApprovalNode 当前节点不是审批节点，无法执行审批/拒绝操作
	ErrNotApprovalNode = errors.New("workflow.not_approval_node")
)

// WorkflowEngine 工作流引擎接口
type WorkflowEngine interface {
	// 创建工作流实例
	CreateInstance(ctx context.Context, issueID, workflowID uint64) (*model.WorkflowInstance, error)
	// 获取工作流实例
	GetInstance(ctx context.Context, instanceID uint64) (*dto.WorkflowInstanceResponse, error)
	// 获取工单的工作流实例
	GetInstanceByIssueID(ctx context.Context, issueID uint64) (*dto.WorkflowInstanceResponse, error)
	// 审批通过
	Approve(ctx context.Context, instanceID, userID uint64, comment string) error
	// 审批拒绝
	Reject(ctx context.Context, instanceID, userID uint64, comment string) error
	// 完成工作节点（result: 条件表达式值，如 approved, rejected, 或自定义条件名称，空=默认流转）
	Complete(ctx context.Context, instanceID, userID uint64, comment, result string) error
	// 获取流转历史
	GetHistory(ctx context.Context, instanceID uint64) ([]*dto.WorkflowHistoryResponse, error)
	// 根据项目和工单类型查找工作流方案，如果存在则自动创建工作流实例
	TryCreateInstanceForIssue(ctx context.Context, issueID, projectID, issueTypeID uint64) (*model.WorkflowInstance, error)
	// 设置工单状态同步器
	SetIssueStatusSyncer(syncer IssueStatusSyncer)
	// 设置活动日志记录器
	SetActivityLogger(logger ActivityLogger)
	// 设置项目外部通知服务
	SetProjectNotifier(notifier ProjectNotifier)
}

// ActivityLogger 活动日志记录接口（避免循环依赖）
type ActivityLogger interface {
	LogActivity(ctx context.Context, userID uint64, userName, action, entityType string, entityID uint64, entityKey, details string) error
}

// ProjectNotifier 项目外部通知接口（避免循环依赖）
type ProjectNotifier interface {
	NotifyProject(ctx context.Context, projectID uint64, event string, data any) error
}

// workflowEngine 工作流引擎实现
type workflowEngine struct {
	instanceRepo      repository.WorkflowInstanceRepository
	historyRepo       repository.WorkflowHistoryRepository
	approvalRepo      repository.ApprovalRecordRepository
	workflowRepo      repository.WorkflowRepository
	nodeRepo          repository.NodeRepository
	edgeRepo          repository.EdgeRepository
	schemeRepo        repository.WorkflowSchemeRepository
	projectRoleRepo   projectRepo.ProjectRoleRepository
	userRepo          userRepo.UserRepository
	db                *gorm.DB
	issueStatusSyncer IssueStatusSyncer // 告警联动（可选）
	activityLogger    ActivityLogger    // 活动日志（可选）
	projectNotifier   ProjectNotifier   // 项目外部通知（可选）
}

// NewWorkflowEngine 创建工作流引擎实例
func NewWorkflowEngine(
	instanceRepo repository.WorkflowInstanceRepository,
	historyRepo repository.WorkflowHistoryRepository,
	approvalRepo repository.ApprovalRecordRepository,
	workflowRepo repository.WorkflowRepository,
	nodeRepo repository.NodeRepository,
	edgeRepo repository.EdgeRepository,
	schemeRepo repository.WorkflowSchemeRepository,
	projectRoleRepo projectRepo.ProjectRoleRepository,
	userRepo userRepo.UserRepository,
	db *gorm.DB,
) WorkflowEngine {
	return &workflowEngine{
		instanceRepo:    instanceRepo,
		historyRepo:     historyRepo,
		approvalRepo:    approvalRepo,
		workflowRepo:    workflowRepo,
		nodeRepo:        nodeRepo,
		edgeRepo:        edgeRepo,
		schemeRepo:      schemeRepo,
		projectRoleRepo: projectRoleRepo,
		userRepo:        userRepo,
		db:              db,
	}
}

// SetIssueStatusSyncer 设置工单状态同步器（告警联动，避免循环依赖）
func (e *workflowEngine) SetIssueStatusSyncer(syncer IssueStatusSyncer) {
	e.issueStatusSyncer = syncer
}

// SetActivityLogger 设置活动日志记录器（避免循环依赖）
func (e *workflowEngine) SetActivityLogger(logger ActivityLogger) {
	e.activityLogger = logger
}

// SetProjectNotifier 设置项目外部通知服务（避免循环依赖）
func (e *workflowEngine) SetProjectNotifier(notifier ProjectNotifier) {
	e.projectNotifier = notifier
}

// logIssueActivity 记录工单相关的活动日志
func (e *workflowEngine) logIssueActivity(ctx context.Context, userID uint64, action string, issueID uint64, details string) {
	if e.activityLogger == nil {
		return
	}
	userName := "系统"
	if userID > 0 {
		if user, err := e.userRepo.GetByID(ctx, userID); err == nil {
			userName = user.DisplayName
			if userName == "" {
				userName = user.Username
			}
		}
	}
	// 获取工单 key
	issueKey := ""
	var issue model.Issue
	if err := e.db.WithContext(ctx).Select("issue_key").Where("id = ?", issueID).First(&issue).Error; err == nil {
		issueKey = issue.IssueKey
	}
	go func() {
		defer safego.Recover("workflow.logActivity")
		logCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = e.activityLogger.LogActivity(logCtx, userID, userName, action, "issue", issueID, issueKey, details)
	}()
}

// CreateInstance 创建工作流实例
func (e *workflowEngine) CreateInstance(ctx context.Context, issueID, workflowID uint64) (*model.WorkflowInstance, error) {
	// 获取工作流的开始节点
	startNode, err := e.nodeRepo.GetStartNode(ctx, workflowID)
	if err != nil {
		// 如果开始节点不存在，自动补建开始和结束节点
		logger.Warn("start node not found, auto-creating start/end nodes", zap.Uint64("workflow_id", workflowID))
		startNode, err = e.ensureStartEndNodes(ctx, workflowID)
		if err != nil {
			logger.Error("failed to ensure start/end nodes", zap.Error(err))
			return nil, fmt.Errorf("获取开始节点失败: %w", err)
		}
	}

	// 创建工作流实例
	instance := &model.WorkflowInstance{
		IssueID:       issueID,
		WorkflowID:    workflowID,
		CurrentNodeID: startNode.ID,
		Status:        "active",
		StartedAt:     time.Now(),
	}

	if err := e.instanceRepo.Create(ctx, instance); err != nil {
		logger.Error("failed to create workflow instance", zap.Error(err))
		return nil, fmt.Errorf("创建工作流实例失败: %w", err)
	}

	// 记录流转历史
	history := &model.WorkflowHistory{
		InstanceID: instance.ID,
		FromNodeID: nil,
		ToNodeID:   startNode.ID,
		Action:     "start",
		OperatorID: 0, // 系统操作
		Comment:    "工作流启动",
		OperatedAt: time.Now(),
	}
	if err := e.historyRepo.Create(ctx, history); err != nil {
		logger.Warn("failed to create workflow history", zap.Error(err))
	}

	// 自动流转到下一个节点（如果失败，实例仍然保留在开始节点）
	if err := e.moveToNextNode(ctx, instance, 0); err != nil {
		logger.Warn("failed to move to next node, instance stays at start node",
			zap.Uint64("instance_id", instance.ID),
			zap.Error(err),
		)
	}

	logger.Info("workflow instance created",
		zap.Uint64("instance_id", instance.ID),
		zap.Uint64("issue_id", issueID),
		zap.Uint64("workflow_id", workflowID),
	)

	return instance, nil
}

// GetInstance 获取工作流实例
func (e *workflowEngine) GetInstance(ctx context.Context, instanceID uint64) (*dto.WorkflowInstanceResponse, error) {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowInstanceNotFound
		}
		return nil, fmt.Errorf("查询工作流实例失败: %w", err)
	}

	return e.toInstanceResponse(ctx, instance)
}

// GetInstanceByIssueID 获取工单的工作流实例
func (e *workflowEngine) GetInstanceByIssueID(ctx context.Context, issueID uint64) (*dto.WorkflowInstanceResponse, error) {
	instance, err := e.instanceRepo.GetByIssueID(ctx, issueID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowInstanceNotFound
		}
		return nil, fmt.Errorf("查询工作流实例失败: %w", err)
	}

	return e.toInstanceResponse(ctx, instance)
}

// Approve 审批通过
func (e *workflowEngine) Approve(ctx context.Context, instanceID, userID uint64, comment string) error {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWorkflowInstanceNotFound
		}
		return fmt.Errorf("查询工作流实例失败: %w", err)
	}

	// 获取当前节点
	currentNode, err := e.nodeRepo.GetByID(ctx, instance.CurrentNodeID)
	if err != nil {
		return fmt.Errorf("查询当前节点失败: %w", err)
	}

	// 检查节点类型
	if currentNode.NodeType != "approval" {
		return ErrNotApprovalNode
	}

	// 解析节点配置（提前解析，用于判断是否配置了指定审批人）
	var config dto.NodeConfig
	if currentNode.Config != "" {
		if unmarshalErr := json.Unmarshal([]byte(currentNode.Config), &config); unmarshalErr != nil {
			logger.Error("failed to parse node config", zap.Error(unmarshalErr))
			return fmt.Errorf("解析节点配置失败: %w", unmarshalErr)
		}
	}

	// 获取审批记录
	record, err := e.approvalRepo.GetByInstanceNodeAndApprover(ctx, instanceID, currentNode.ID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 配置了指定审批人时，严格按配置执行，任何人（包括管理员和 Owner）都不能绕过
			if len(config.Approvers) > 0 {
				return ErrNotApprover
			}
			// 未配置审批人时，仅允许系统管理员或项目 Owner 审批
			if !e.isAdminOrProjectOwner(ctx, userID, instance.IssueID) {
				return ErrNotApprover
			}
			// 管理员/Owner：自动创建审批记录以便流程正常推进
			record = &model.ApprovalRecord{
				InstanceID: instanceID,
				NodeID:     currentNode.ID,
				ApproverID: userID,
				Status:     "pending",
			}
			if createErr := e.approvalRepo.Create(ctx, record); createErr != nil {
				logger.Error("failed to create admin approval record", zap.Error(createErr))
				return fmt.Errorf("创建管理员审批记录失败: %w", createErr)
			}
		} else {
			return fmt.Errorf("查询审批记录失败: %w", err)
		}
	}

	// 检查是否已审批
	if record.Status != "pending" {
		return ErrAlreadyApproved
	}

	// 更新审批记录
	now := time.Now()
	record.Status = "approved"
	record.Comment = comment
	record.ApprovedAt = &now

	if updateErr := e.approvalRepo.Update(ctx, record); updateErr != nil {
		logger.Error("failed to update approval record", zap.Error(updateErr))
		return fmt.Errorf("更新审批记录失败: %w", updateErr)
	}

	// 记录流转历史
	history := &model.WorkflowHistory{
		InstanceID: instanceID,
		FromNodeID: &currentNode.ID,
		ToNodeID:   currentNode.ID,
		Action:     "approve",
		OperatorID: userID,
		Comment:    comment,
		OperatedAt: now,
	}
	if historyErr := e.historyRepo.Create(ctx, history); historyErr != nil {
		logger.Warn("failed to create workflow history", zap.Error(historyErr))
	}

	// 检查审批是否完成
	complete, err := e.checkApprovalComplete(ctx, instanceID, currentNode.ID, config.ApprovalType)
	if err != nil {
		return fmt.Errorf("检查审批状态失败: %w", err)
	}

	// 如果审批完成，流转到下一个节点
	if complete {
		if err := e.moveToNextNode(ctx, instance, userID); err != nil {
			return fmt.Errorf("流转到下一节点失败: %w", err)
		}
	}

	logger.Info("approval approved",
		zap.Uint64("instance_id", instanceID),
		zap.Uint64("user_id", userID),
		zap.Bool("complete", complete),
	)

	return nil
}

// Reject 审批拒绝
func (e *workflowEngine) Reject(ctx context.Context, instanceID, userID uint64, comment string) error {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWorkflowInstanceNotFound
		}
		return fmt.Errorf("查询工作流实例失败: %w", err)
	}

	// 获取当前节点
	currentNode, err := e.nodeRepo.GetByID(ctx, instance.CurrentNodeID)
	if err != nil {
		return fmt.Errorf("查询当前节点失败: %w", err)
	}

	// 检查节点类型
	if currentNode.NodeType != "approval" {
		return ErrNotApprovalNode
	}

	// 解析节点配置（提前解析，用于判断是否配置了指定审批人）
	var config dto.NodeConfig
	if currentNode.Config != "" {
		if unmarshalErr := json.Unmarshal([]byte(currentNode.Config), &config); unmarshalErr != nil {
			logger.Error("failed to parse node config", zap.Error(unmarshalErr))
			return fmt.Errorf("解析节点配置失败: %w", unmarshalErr)
		}
	}

	// 获取审批记录
	record, err := e.approvalRepo.GetByInstanceNodeAndApprover(ctx, instanceID, currentNode.ID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 配置了指定审批人时，严格按配置执行，任何人（包括管理员和 Owner）都不能绕过
			if len(config.Approvers) > 0 {
				return ErrNotApprover
			}
			// 未配置审批人时，仅允许系统管理员或项目 Owner 审批
			if !e.isAdminOrProjectOwner(ctx, userID, instance.IssueID) {
				return ErrNotApprover
			}
			// 管理员/Owner：自动创建审批记录以便流程正常推进
			record = &model.ApprovalRecord{
				InstanceID: instanceID,
				NodeID:     currentNode.ID,
				ApproverID: userID,
				Status:     "pending",
			}
			if createErr := e.approvalRepo.Create(ctx, record); createErr != nil {
				logger.Error("failed to create admin approval record for reject", zap.Error(createErr))
				return fmt.Errorf("创建管理员审批记录失败: %w", createErr)
			}
		} else {
			return fmt.Errorf("查询审批记录失败: %w", err)
		}
	}

	// 检查是否已审批
	if record.Status != "pending" {
		return ErrAlreadyApproved
	}

	// 更新审批记录
	now := time.Now()
	record.Status = "rejected"
	record.Comment = comment
	record.ApprovedAt = &now

	if updateErr := e.approvalRepo.Update(ctx, record); updateErr != nil {
		logger.Error("failed to update approval record", zap.Error(updateErr))
		return fmt.Errorf("更新审批记录失败: %w", updateErr)
	}

	// 记录流转历史
	history := &model.WorkflowHistory{
		InstanceID: instanceID,
		FromNodeID: &currentNode.ID,
		ToNodeID:   currentNode.ID,
		Action:     "reject",
		OperatorID: userID,
		Comment:    comment,
		OperatedAt: now,
	}
	if historyErr := e.historyRepo.Create(ctx, history); historyErr != nil {
		logger.Warn("failed to create workflow history", zap.Error(historyErr))
	}

	// 查找是否有 "rejected" 条件的出边
	edges, err := e.edgeRepo.GetOutgoingEdges(ctx, currentNode.ID)
	if err != nil {
		logger.Error("failed to get outgoing edges for reject", zap.Error(err))
	}

	var rejectEdge *model.WorkflowEdge
	for _, edge := range edges {
		if edge.ConditionExpr == "rejected" {
			rejectEdge = edge
			break
		}
	}

	if rejectEdge != nil {
		// 有拒绝分支边，流转到拒绝目标节点
		nextNode, err := e.nodeRepo.GetByID(ctx, rejectEdge.TargetNodeID)
		if err != nil {
			logger.Error("failed to get reject target node", zap.Error(err))
			return fmt.Errorf("查询拒绝目标节点失败: %w", err)
		}

		fromNodeID := instance.CurrentNodeID
		instance.CurrentNodeID = nextNode.ID

		if err := e.instanceRepo.Update(ctx, instance); err != nil {
			return fmt.Errorf("更新工作流实例失败: %w", err)
		}

		// 记录流转历史
		forwardHistory := &model.WorkflowHistory{
			InstanceID: instance.ID,
			FromNodeID: &fromNodeID,
			ToNodeID:   nextNode.ID,
			Action:     "forward",
			OperatorID: userID,
			Comment:    fmt.Sprintf("审批拒绝，流转到节点: %s", nextNode.Name),
			OperatedAt: time.Now(),
		}
		if err := e.historyRepo.Create(ctx, forwardHistory); err != nil {
			logger.Warn("failed to create reject forward history", zap.Error(err))
		}

		// 联动更新工单状态
		e.syncIssueStatus(ctx, instance.IssueID, currentNode, nextNode)

		// 如果目标是结束节点，标记实例完成
		if nextNode.NodeType == "end" {
			instance.Status = "completed"
			completedAt := time.Now()
			instance.CompletedAt = &completedAt
			if err := e.instanceRepo.Update(ctx, instance); err != nil {
				return fmt.Errorf("更新工作流实例失败: %w", err)
			}
		}
	} else {
		// 没有拒绝分支边，回退到原来的行为：直接取消工作流
		instance.Status = "cancelled" //nolint:misspell // 状态值与前端约定保持一致
		completedAt := time.Now()
		instance.CompletedAt = &completedAt

		if err := e.instanceRepo.Update(ctx, instance); err != nil {
			logger.Error("failed to update workflow instance", zap.Error(err))
			return fmt.Errorf("更新工作流实例失败: %w", err)
		}

		// 联动更新工单状态为 closed
		closedAt := time.Now()
		e.db.WithContext(ctx).Model(&model.Issue{}).Where("id = ?", instance.IssueID).Updates(map[string]any{
			"status":          "closed",
			"closed_at":       closedAt,
			"actual_end_date": closedAt,
			"updated_at":      closedAt,
		})

		// 记录状态变更活动日志
		e.logIssueActivity(ctx, userID, "status_changed", instance.IssueID, detail.New("activity.detail.approvalRejectedClosed"))

		// 需求联动：工单关闭时同步关联需求状态为已完成
		result := e.db.WithContext(ctx).
			Model(&model.Requirement{}).
			Where("converted_issue_id = ? AND status NOT IN ?", instance.IssueID, []string{"completed", "rejected"}).
			Updates(map[string]any{
				"status":     "completed",
				"updated_at": closedAt,
			})
		if result.Error != nil {
			logger.Warn("failed to sync requirement status on reject",
				zap.Uint64("issue_id", instance.IssueID),
				zap.Error(result.Error),
			)
		} else if result.RowsAffected > 0 {
			logger.Info("requirement status synced to completed on reject",
				zap.Uint64("issue_id", instance.IssueID),
				zap.Int64("affected", result.RowsAffected),
			)
		}

		// 同步告警状态和级联合并工单（Reject 无退回边时也需要同步）
		if e.issueStatusSyncer != nil {
			issueID := instance.IssueID
			go func() {
				defer safego.Recover("workflow.syncIssueOnReject")
				syncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := e.issueStatusSyncer.SyncIssueStatus(syncCtx, issueID, "closed"); err != nil {
					logger.Warn("failed to sync issue status on reject",
						zap.Uint64("issue_id", issueID), zap.Error(err))
				}
				if err := e.issueStatusSyncer.SyncMergedIssueStatus(syncCtx, issueID, "closed"); err != nil {
					logger.Warn("failed to sync merged issue status on reject",
						zap.Uint64("issue_id", issueID), zap.Error(err))
				}
			}()
		}
	}

	logger.Info("approval rejected",
		zap.Uint64("instance_id", instanceID),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// Complete 完成工作节点，推进到下一个节点
// result: 条件表达式值（如 "approved", "rejected", 或自定义条件名称），空字符串表示默认流转
func (e *workflowEngine) Complete(ctx context.Context, instanceID, userID uint64, comment, result string) error {
	instance, err := e.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWorkflowInstanceNotFound
		}
		return fmt.Errorf("查询工作流实例失败: %w", err)
	}

	// 获取当前节点
	currentNode, err := e.nodeRepo.GetByID(ctx, instance.CurrentNodeID)
	if err != nil {
		return fmt.Errorf("查询当前节点失败: %w", err)
	}

	// 检查节点类型（work 和 start 节点都可以完成）
	if currentNode.NodeType != "work" && currentNode.NodeType != "start" {
		return fmt.Errorf("当前节点不是工作节点，无法执行完成操作")
	}

	// 记录流转历史
	now := time.Now()
	actionText := "complete"
	if result != "" && result != "approved" {
		actionText = "forward"
	}
	history := &model.WorkflowHistory{
		InstanceID: instanceID,
		FromNodeID: &currentNode.ID,
		ToNodeID:   currentNode.ID,
		Action:     actionText,
		OperatorID: userID,
		Comment:    comment,
		OperatedAt: now,
	}
	if err := e.historyRepo.Create(ctx, history); err != nil {
		logger.Warn("failed to create workflow history", zap.Error(err))
	}

	// 根据 result 选择出边流转
	if result != "" {
		// 有指定条件：查找匹配条件的出边
		if err := e.moveToNextNodeByCondition(ctx, instance, userID, result); err != nil {
			return fmt.Errorf("流转到目标节点失败: %w", err)
		}
	} else {
		// 无条件：走默认流转（无条件边或第一条边）
		if err := e.moveToNextNode(ctx, instance, userID); err != nil {
			return fmt.Errorf("流转到下一节点失败: %w", err)
		}
	}

	logger.Info("work node completed",
		zap.Uint64("instance_id", instanceID),
		zap.Uint64("user_id", userID),
		zap.String("result", result),
	)

	return nil
}

// GetHistory 获取流转历史
func (e *workflowEngine) GetHistory(ctx context.Context, instanceID uint64) ([]*dto.WorkflowHistoryResponse, error) {
	histories, err := e.historyRepo.ListByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("查询流转历史失败: %w", err)
	}

	responses := make([]*dto.WorkflowHistoryResponse, len(histories))
	for i, history := range histories {
		responses[i] = e.toHistoryResponse(ctx, history)
	}

	return responses, nil
}

// moveToNextNode 流转到下一个节点
func (e *workflowEngine) moveToNextNode(ctx context.Context, instance *model.WorkflowInstance, operatorID uint64) error {
	// 获取当前节点的出边
	edges, err := e.edgeRepo.GetOutgoingEdges(ctx, instance.CurrentNodeID)
	if err != nil {
		return fmt.Errorf("查询出边失败: %w", err)
	}

	if len(edges) == 0 {
		// 没有出边，可能是结束节点
		currentNode, _ := e.nodeRepo.GetByID(ctx, instance.CurrentNodeID)
		if currentNode != nil && currentNode.NodeType == "end" {
			instance.Status = "completed"
			completedAt := time.Now()
			instance.CompletedAt = &completedAt
			if updateErr := e.instanceRepo.Update(ctx, instance); updateErr != nil {
				return updateErr
			}
			e.syncIssueStatus(ctx, instance.IssueID, nil, currentNode)
			return nil
		}
		return fmt.Errorf("当前节点没有出边")
	}

	// 选择合适的出边
	var nextEdge *model.WorkflowEdge

	// 获取当前节点信息，判断是否是审批节点
	currentNode, _ := e.nodeRepo.GetByID(ctx, instance.CurrentNodeID)

	if currentNode != nil && currentNode.NodeType == "approval" {
		// 审批节点：选择 "approved" 条件的边，或无条件的边
		for _, edge := range edges {
			if edge.ConditionExpr == "approved" {
				nextEdge = edge
				break
			}
		}
		// 如果没有 "approved" 边，选择非 "rejected" 的边
		if nextEdge == nil {
			for _, edge := range edges {
				if edge.ConditionExpr != "rejected" {
					nextEdge = edge
					break
				}
			}
		}
	}

	// 非审批节点或未找到匹配边：选择无条件边或第一条边
	if nextEdge == nil {
		for _, edge := range edges {
			if edge.ConditionExpr == "" {
				nextEdge = edge
				break
			}
		}
	}
	if nextEdge == nil {
		nextEdge = edges[0]
	}

	nextNode, err := e.nodeRepo.GetByID(ctx, nextEdge.TargetNodeID)
	if err != nil {
		return fmt.Errorf("查询下一节点失败: %w", err)
	}

	// 更新实例的当前节点
	fromNodeID := instance.CurrentNodeID
	instance.CurrentNodeID = nextNode.ID

	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("更新工作流实例失败: %w", err)
	}

	// 记录流转历史
	history := &model.WorkflowHistory{
		InstanceID: instance.ID,
		FromNodeID: &fromNodeID,
		ToNodeID:   nextNode.ID,
		Action:     "forward",
		OperatorID: operatorID,
		Comment:    fmt.Sprintf("流转到节点: %s", nextNode.Name),
		OperatedAt: time.Now(),
	}
	if err := e.historyRepo.Create(ctx, history); err != nil {
		logger.Warn("failed to create workflow history", zap.Error(err))
	}

	// 联动更新工单状态
	e.syncIssueStatus(ctx, instance.IssueID, currentNode, nextNode)

	// 如果是审批节点，创建审批记录
	if nextNode.NodeType == "approval" {
		if err := e.createApprovalRecords(ctx, instance, nextNode); err != nil {
			logger.Error("failed to create approval records", zap.Error(err))
			return fmt.Errorf("创建审批记录失败: %w", err)
		}
	}

	// 如果是结束节点，标记实例完成
	if nextNode.NodeType == "end" {
		instance.Status = "completed"
		completedAt := time.Now()
		instance.CompletedAt = &completedAt
		if err := e.instanceRepo.Update(ctx, instance); err != nil {
			return fmt.Errorf("更新工作流实例失败: %w", err)
		}
	}

	return nil
}

// moveToNextNodeByCondition 根据指定条件选择出边流转
func (e *workflowEngine) moveToNextNodeByCondition(ctx context.Context, instance *model.WorkflowInstance, operatorID uint64, condition string) error {
	// 获取当前节点的出边
	edges, err := e.edgeRepo.GetOutgoingEdges(ctx, instance.CurrentNodeID)
	if err != nil {
		return fmt.Errorf("查询出边失败: %w", err)
	}

	// 查找匹配条件的边
	var targetEdge *model.WorkflowEdge
	for _, edge := range edges {
		if edge.ConditionExpr == condition {
			targetEdge = edge
			break
		}
	}

	if targetEdge == nil {
		// 没有匹配的条件边，回退到默认流转
		return e.moveToNextNode(ctx, instance, operatorID)
	}

	nextNode, err := e.nodeRepo.GetByID(ctx, targetEdge.TargetNodeID)
	if err != nil {
		return fmt.Errorf("查询目标节点失败: %w", err)
	}

	// 更新实例的当前节点
	fromNodeID := instance.CurrentNodeID
	fromNode, _ := e.nodeRepo.GetByID(ctx, fromNodeID)
	instance.CurrentNodeID = nextNode.ID

	if err := e.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("更新工作流实例失败: %w", err)
	}

	// 记录流转历史
	conditionText := condition
	// 预设条件翻译为中文
	presetLabels := map[string]string{
		"approved": "通过",
		"rejected": "退回",
	}
	if label, ok := presetLabels[condition]; ok {
		conditionText = label
	}
	history := &model.WorkflowHistory{
		InstanceID: instance.ID,
		FromNodeID: &fromNodeID,
		ToNodeID:   nextNode.ID,
		Action:     "forward",
		OperatorID: operatorID,
		Comment:    fmt.Sprintf("%s，流转到节点: %s", conditionText, nextNode.Name),
		OperatedAt: time.Now(),
	}
	if err := e.historyRepo.Create(ctx, history); err != nil {
		logger.Warn("failed to create workflow history", zap.Error(err))
	}

	// 联动更新工单状态
	e.syncIssueStatus(ctx, instance.IssueID, fromNode, nextNode)

	// 如果是审批节点，创建审批记录
	if nextNode.NodeType == "approval" {
		if err := e.createApprovalRecords(ctx, instance, nextNode); err != nil {
			logger.Error("failed to create approval records", zap.Error(err))
			return fmt.Errorf("创建审批记录失败: %w", err)
		}
	}

	// 如果是结束节点，标记实例完成
	if nextNode.NodeType == "end" {
		instance.Status = "completed"
		completedAt := time.Now()
		instance.CompletedAt = &completedAt
		if err := e.instanceRepo.Update(ctx, instance); err != nil {
			return fmt.Errorf("更新工作流实例失败: %w", err)
		}
	}

	return nil
}

// createApprovalRecords 创建审批记录
func (e *workflowEngine) createApprovalRecords(ctx context.Context, instance *model.WorkflowInstance, node *model.WorkflowNode) error {
	// 解析节点配置
	var config dto.NodeConfig
	if node.Config != "" {
		if err := json.Unmarshal([]byte(node.Config), &config); err != nil {
			logger.Error("failed to parse node config", zap.Error(err))
			return fmt.Errorf("解析节点配置失败: %w", err)
		}
	}

	// 解析审批人
	approverIDs, err := e.resolveApprovers(ctx, instance, &config)
	if err != nil {
		return err
	}

	if len(approverIDs) == 0 {
		return ErrNoApproversConfigured
	}

	// 创建审批记录
	for _, approverID := range approverIDs {
		record := &model.ApprovalRecord{
			InstanceID: instance.ID,
			NodeID:     node.ID,
			ApproverID: approverID,
			Status:     "pending",
		}
		if err := e.approvalRepo.Create(ctx, record); err != nil {
			logger.Error("failed to create approval record", zap.Error(err))
			return fmt.Errorf("创建审批记录失败: %w", err)
		}
	}

	return nil
}

// resolveApprovers 解析审批人
func (e *workflowEngine) resolveApprovers(ctx context.Context, instance *model.WorkflowInstance, config *dto.NodeConfig) ([]uint64, error) {
	// 1. 直接指定用户
	if len(config.Approvers) > 0 {
		return config.Approvers, nil
	}

	// 2. 按项目角色解析
	if config.ApproverRole != "" {
		// 通过 issue 获取 project_id
		var issue model.Issue
		if err := e.db.WithContext(ctx).Select("project_id").Where("id = ?", instance.IssueID).First(&issue).Error; err != nil {
			return nil, fmt.Errorf("查询工单项目失败: %w", err)
		}

		userIDs, err := e.projectRoleRepo.GetUserIDsByRoleKey(ctx, issue.ProjectID, config.ApproverRole)
		if err != nil {
			return nil, fmt.Errorf("查询角色成员失败: %w", err)
		}
		if len(userIDs) > 0 {
			return userIDs, nil
		}
		return nil, ErrNoApproversConfigured
	}

	return nil, ErrNoApproversConfigured
}

// checkApprovalComplete 检查审批是否完成
func (e *workflowEngine) checkApprovalComplete(ctx context.Context, instanceID, nodeID uint64, approvalType string) (bool, error) {
	records, err := e.approvalRepo.ListByInstanceAndNode(ctx, instanceID, nodeID)
	if err != nil {
		return false, err
	}

	switch approvalType {
	case "single", "or_sign", "":
		// 任意一人通过即完成
		for _, r := range records {
			if r.Status == "approved" {
				return true, nil
			}
		}
		return false, nil

	case "countersign":
		// 所有人必须通过
		for _, r := range records {
			if r.Status != "approved" {
				return false, nil
			}
		}
		return len(records) > 0, nil

	default:
		return false, fmt.Errorf("未知的审批类型: %s", approvalType)
	}
}

// toInstanceResponse 转换为实例响应 DTO
func (e *workflowEngine) toInstanceResponse(ctx context.Context, instance *model.WorkflowInstance) (*dto.WorkflowInstanceResponse, error) {
	resp := &dto.WorkflowInstanceResponse{
		ID:            instance.ID,
		IssueID:       instance.IssueID,
		WorkflowID:    instance.WorkflowID,
		CurrentNodeID: instance.CurrentNodeID,
		Status:        instance.Status,
		StartedAt:     instance.StartedAt,
		CompletedAt:   instance.CompletedAt,
		CreatedAt:     instance.CreatedAt,
		UpdatedAt:     instance.UpdatedAt,
	}

	// 获取当前节点信息
	if currentNode, err := e.nodeRepo.GetByID(ctx, instance.CurrentNodeID); err == nil {
		resp.CurrentNode = e.toNodeResponse(currentNode)
	}

	// 获取工作流信息
	if workflow, err := e.workflowRepo.GetByID(ctx, instance.WorkflowID); err == nil {
		resp.WorkflowName = workflow.Name
	}

	// 获取审批记录
	if approvals, err := e.approvalRepo.ListByInstance(ctx, instance.ID); err == nil {
		resp.Approvals = make([]*dto.ApprovalRecordResponse, len(approvals))
		for i, approval := range approvals {
			resp.Approvals[i] = e.toApprovalResponse(ctx, approval)
		}
	}

	return resp, nil
}

// toHistoryResponse 转换为历史响应 DTO
func (e *workflowEngine) toHistoryResponse(ctx context.Context, history *model.WorkflowHistory) *dto.WorkflowHistoryResponse {
	resp := &dto.WorkflowHistoryResponse{
		ID:         history.ID,
		InstanceID: history.InstanceID,
		FromNodeID: history.FromNodeID,
		ToNodeID:   history.ToNodeID,
		Action:     history.Action,
		OperatorID: history.OperatorID,
		Comment:    history.Comment,
		OperatedAt: history.OperatedAt,
		CreatedAt:  history.CreatedAt,
	}

	// 获取操作人信息
	if history.OperatorID > 0 {
		if user, err := e.userRepo.GetByID(ctx, history.OperatorID); err == nil {
			resp.OperatorName = user.DisplayName
		}
	}

	// 获取节点信息
	if history.FromNodeID != nil {
		if node, err := e.nodeRepo.GetByID(ctx, *history.FromNodeID); err == nil {
			resp.FromNode = e.toNodeResponse(node)
		}
	}
	if node, err := e.nodeRepo.GetByID(ctx, history.ToNodeID); err == nil {
		resp.ToNode = e.toNodeResponse(node)
	}

	return resp
}

// toApprovalResponse 转换为审批记录响应 DTO
func (e *workflowEngine) toApprovalResponse(ctx context.Context, approval *model.ApprovalRecord) *dto.ApprovalRecordResponse {
	resp := &dto.ApprovalRecordResponse{
		ID:         approval.ID,
		InstanceID: approval.InstanceID,
		NodeID:     approval.NodeID,
		ApproverID: approval.ApproverID,
		Status:     approval.Status,
		Comment:    approval.Comment,
		ApprovedAt: approval.ApprovedAt,
		CreatedAt:  approval.CreatedAt,
		UpdatedAt:  approval.UpdatedAt,
	}

	// 获取审批人信息
	if user, err := e.userRepo.GetByID(ctx, approval.ApproverID); err == nil {
		resp.ApproverName = user.DisplayName
	}

	return resp
}

// toNodeResponse 转换为节点响应 DTO
func (e *workflowEngine) toNodeResponse(node *model.WorkflowNode) *dto.NodeResponse {
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

// syncIssueStatus 根据工作流节点联动更新工单状态
// fromNode 为流转前的节点（可为 nil），toNode 为目标节点
func (e *workflowEngine) syncIssueStatus(ctx context.Context, issueID uint64, fromNode, toNode *model.WorkflowNode) {
	// 优先使用节点配置中的 target_status
	targetStatus := ""
	if toNode.Config != "" && toNode.Config != "{}" {
		var config dto.NodeConfig
		if err := json.Unmarshal([]byte(toNode.Config), &config); err == nil && config.TargetStatus != "" {
			targetStatus = config.TargetStatus
		}
	}

	// 如果节点没有配置 target_status，使用默认映射
	if targetStatus == "" {
		switch toNode.NodeType {
		case "approval":
			targetStatus = "open" // 审批中工单保持 open，审批通过后才流转
		case "work":
			targetStatus = "in_progress"
		case "end":
			targetStatus = "resolved"
		default:
			return // start、system 等节点不改变状态
		}
	}

	// 状态中文显示名映射
	statusNames := map[string]string{
		"open": "待处理", "in_progress": "进行中", "resolved": "已解决",
		"closed": "已关闭", "reviewing": "待确认", "pending_review": "待确认",
	}

	// 更新前先查询旧状态（用于通知和日志）
	var oldIssue model.Issue
	_ = e.db.WithContext(ctx).Select("id, issue_key, title, project_id, priority, status, assignee_id, due_date").
		Where("id = ?", issueID).First(&oldIssue).Error
	oldStatus := oldIssue.Status

	statusChanged := oldStatus != targetStatus

	// 仅在状态实际变化时更新数据库
	if statusChanged {
		now := time.Now()
		updates := map[string]any{
			"status":     targetStatus,
			"updated_at": now,
		}

		// 根据目标状态更新相关时间字段
		switch targetStatus {
		case "in_progress":
			updates["actual_start_date"] = now
		case "resolved":
			updates["resolved_at"] = now
			updates["actual_end_date"] = now
			updates["resolution"] = "done"
		case "closed":
			updates["closed_at"] = now
			updates["actual_end_date"] = now
		}

		if err := e.db.WithContext(ctx).Model(&model.Issue{}).Where("id = ?", issueID).Updates(updates).Error; err != nil {
			logger.Warn("failed to sync issue status from workflow",
				zap.Uint64("issue_id", issueID),
				zap.String("target_status", targetStatus),
				zap.Error(err),
			)
			return
		}
		logger.Info("issue status synced from workflow",
			zap.Uint64("issue_id", issueID),
			zap.String("target_status", targetStatus),
			zap.String("node_type", toNode.NodeType),
		)
	}

	// 无论状态是否变化，只要节点发生了流转，都记录日志和发送通知
	oldStatusName := statusNames[oldStatus]
	if oldStatusName == "" {
		oldStatusName = oldStatus
	}
	statusName := statusNames[targetStatus]
	if statusName == "" {
		statusName = targetStatus
	}

	// 获取来源节点名称
	fromNodeName := ""
	if fromNode != nil {
		fromNodeName = fromNode.Name
	}

	// 记录活动日志
	if statusChanged {
		// 状态名走 key，不存译文 —— statusNames 是中文映射，存进去就把语言烧死了
		e.logIssueActivity(ctx, 0, "status_changed", issueID,
			detail.NewWithKeys("activity.detail.nodeTransitionedWithStatus",
				map[string]string{"status": "issue.statusMap." + targetStatus},
				"node", toNode.Name))
	} else {
		e.logIssueActivity(ctx, 0, "node_transitioned", issueID,
			detail.New("activity.detail.nodeTransitioned", "node", toNode.Name))
	}

	// 项目外部通知：工单流转（始终发送，即使状态未变化）
	if e.projectNotifier != nil {
		// 查询项目名称
		var projectName string
		if oldIssue.ProjectID > 0 {
			var proj model.Project
			if err := e.db.WithContext(ctx).Select("id, name").Where("id = ?", oldIssue.ProjectID).First(&proj).Error; err == nil {
				projectName = proj.Name
			}
		}
		// 查询指派人（用于显示名称 + 通知 @ 提及）
		var assigneeName string
		var assigneeLarkID, assigneeEmail, assigneeTelegramID string
		if oldIssue.AssigneeID != nil {
			var user model.User
			if err := e.db.WithContext(ctx).
				Select("id, display_name, username, email, lark_open_id, telegram_user_id").
				Where("id = ?", *oldIssue.AssigneeID).First(&user).Error; err == nil {
				assigneeName = user.DisplayName
				if assigneeName == "" {
					assigneeName = user.Username
				}
				assigneeLarkID = user.LarkOpenID
				assigneeEmail = user.Email
				assigneeTelegramID = user.TelegramUserID
			}
		}

		go func() {
			defer safego.Recover("workflow.notifyApprovers")
			notifCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			notifData := map[string]any{
				"issue_key":       oldIssue.IssueKey,
				"issue_title":     oldIssue.Title,
				"status":          targetStatus,
				"status_name":     statusName,
				"old_status":      oldStatus,
				"old_status_name": oldStatusName,
				"project_name":    projectName,
				"priority":        oldIssue.Priority,
				"assignee":        assigneeName,
				"node_name":       toNode.Name,
				"old_node_name":   fromNodeName,
			}
			if oldIssue.DueDate != nil {
				notifData["due_date"] = oldIssue.DueDate.Format("2006-01-02 15:04")
			}
			if assigneeLarkID != "" || assigneeTelegramID != "" || assigneeEmail != "" {
				notifData["mentions"] = []map[string]any{{
					"display_name":     assigneeName,
					"lark_open_id":     assigneeLarkID,
					"email":            assigneeEmail,
					"telegram_user_id": assigneeTelegramID,
				}}
			}
			_ = e.projectNotifier.NotifyProject(notifCtx, oldIssue.ProjectID, "issue.transitioned", notifData)
		}()
	}

	// 以下联动仅在状态实际变化时执行
	if !statusChanged {
		return
	}

	// 需求联动：工单终态时同步关联需求状态为已完成
	if targetStatus == "resolved" || targetStatus == "closed" {
		now := time.Now()
		result := e.db.WithContext(ctx).
			Model(&model.Requirement{}).
			Where("converted_issue_id = ? AND status NOT IN ?", issueID, []string{"completed", "rejected"}).
			Updates(map[string]any{
				"status":     "completed",
				"updated_at": now,
			})
		if result.Error != nil {
			logger.Warn("failed to sync requirement status from issue",
				zap.Uint64("issue_id", issueID),
				zap.Error(result.Error),
			)
		} else if result.RowsAffected > 0 {
			logger.Info("requirement status synced to completed",
				zap.Uint64("issue_id", issueID),
				zap.Int64("affected", result.RowsAffected),
			)
		}
	}

	// 告警联动：同步工单状态到告警 + 级联更新被合并的旧工单
	if e.issueStatusSyncer != nil {
		syncIssueID := issueID
		syncStatus := targetStatus
		go func() {
			defer safego.Recover("workflow.syncIssueStatus")
			syncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := e.issueStatusSyncer.SyncIssueStatus(syncCtx, syncIssueID, syncStatus); err != nil {
				logger.Warn("failed to sync issue status to alerts",
					zap.Uint64("issue_id", syncIssueID),
					zap.Error(err),
				)
			}
			if err := e.issueStatusSyncer.SyncMergedIssueStatus(syncCtx, syncIssueID, syncStatus); err != nil {
				logger.Warn("failed to cascade status to merged issues",
					zap.Uint64("issue_id", syncIssueID),
					zap.Error(err),
				)
			}
		}()
	}
}

// ensureStartEndNodes 确保工作流有开始和结束节点，如果没有则自动创建
func (e *workflowEngine) ensureStartEndNodes(ctx context.Context, workflowID uint64) (*model.WorkflowNode, error) {
	// 创建开始节点
	startNode := &model.WorkflowNode{
		WorkflowID: workflowID,
		Name:       "开始",
		NodeType:   "start",
		Config:     "{}",
		PositionX:  100,
		PositionY:  200,
	}
	if err := e.nodeRepo.Create(ctx, startNode); err != nil {
		return nil, fmt.Errorf("创建开始节点失败: %w", err)
	}

	// 检查是否有结束节点，没有则创建
	endNodes, _ := e.nodeRepo.GetEndNodes(ctx, workflowID)
	if len(endNodes) == 0 {
		endNode := &model.WorkflowNode{
			WorkflowID: workflowID,
			Name:       "结束",
			NodeType:   "end",
			Config:     "{}",
			PositionX:  500,
			PositionY:  200,
		}
		if err := e.nodeRepo.Create(ctx, endNode); err != nil {
			logger.Warn("failed to create end node", zap.Error(err))
		}
	}

	return startNode, nil
}

// TryCreateInstanceForIssue 根据项目和工单类型查找工作流方案，如果存在则自动创建工作流实例
func (e *workflowEngine) TryCreateInstanceForIssue(ctx context.Context, issueID, projectID, issueTypeID uint64) (*model.WorkflowInstance, error) {
	// 查找工作流方案
	scheme, err := e.schemeRepo.GetByProjectAndIssueType(ctx, projectID, issueTypeID)
	if err != nil {
		// 没有配置工作流方案，返回 nil（不是错误）
		return nil, nil
	}

	// 创建工作流实例
	return e.CreateInstance(ctx, issueID, scheme.WorkflowID)
}

// isAdminOrProjectOwner 检查用户是否为系统管理员或项目 Owner
// 仅在未配置指定审批人时作为兜底权限，不包含项目 administrators
func (e *workflowEngine) isAdminOrProjectOwner(ctx context.Context, userID, issueID uint64) bool {
	// 1. 检查是否为系统管理员（通过 user_roles 表关联的 roles 表）
	var adminCount int64
	e.db.WithContext(ctx).
		Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.name = ?", userID, "admin").
		Count(&adminCount)
	if adminCount > 0 {
		return true
	}

	// 2. 检查是否为项目 Owner（通过 project_members 表，仅 owner 角色）
	var issue model.Issue
	if err := e.db.WithContext(ctx).Select("project_id").First(&issue, issueID).Error; err != nil {
		return false
	}

	var memberCount int64
	e.db.WithContext(ctx).
		Table("project_members").
		Where("user_id = ? AND project_id = ? AND role = ?", userID, issue.ProjectID, "owner").
		Count(&memberCount)

	return memberCount > 0
}
