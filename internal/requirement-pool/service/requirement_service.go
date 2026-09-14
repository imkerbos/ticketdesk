package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/activity/detail"
	issueDto "github.com/kerbos/ticketdesk/internal/core-issue/dto"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/internal/requirement-pool/dto"
	"github.com/kerbos/ticketdesk/internal/requirement-pool/repository"
	"github.com/kerbos/ticketdesk/pkg/cache"
)

// IssueCreator 工单创建接口（最小依赖，避免耦合完整 IssueService）
type IssueCreator interface {
	CreateIssue(ctx context.Context, req *issueDto.CreateIssueRequest, reporterID uint64) (*issueDto.IssueResponse, error)
}

// RequirementService 需求业务逻辑接口
type RequirementService interface {
	Create(ctx context.Context, req *dto.CreateRequirementRequest, userID uint64) (*dto.RequirementResponse, error)
	GetByID(ctx context.Context, id uint64) (*dto.RequirementResponse, error)
	Update(ctx context.Context, id uint64, req *dto.UpdateRequirementRequest, userID uint64) error
	Delete(ctx context.Context, id, userID uint64) error
	List(ctx context.Context, req *dto.RequirementListRequest) ([]*dto.RequirementResponse, int64, error)
	ConvertToIssue(ctx context.Context, id uint64, req *dto.ConvertToIssueRequest, userID uint64) (*dto.ConvertToIssueResponse, error)
	AddComment(ctx context.Context, id uint64, req *dto.RequirementCommentRequest, userID uint64) (*dto.RequirementCommentResponse, error)
	GetKanban(ctx context.Context, req *dto.KanbanRequest) (*dto.KanbanResponse, error)
	GetReport(ctx context.Context, req *dto.ReportRequest) (*dto.ReportResponse, error)
}

// requirementService 需求业务逻辑实现
type requirementService struct {
	repo     repository.RequirementRepository
	poolRepo repository.RequirementPoolRepository
	db       *gorm.DB
	logger   *zap.Logger
	issueSvc IssueCreator
}

// SetIssueCreator 注入工单创建服务（setter 注入，避免循环依赖）
func (s *requirementService) SetIssueCreator(ic IssueCreator) {
	s.issueSvc = ic
}

// NewRequirementService 创建需求业务逻辑实例
func NewRequirementService(
	repo repository.RequirementRepository,
	poolRepo repository.RequirementPoolRepository,
	db *gorm.DB,
	logger *zap.Logger,
) RequirementService {
	return &requirementService{
		repo:     repo,
		poolRepo: poolRepo,
		db:       db,
		logger:   logger,
	}
}

// Create 创建需求
func (s *requirementService) Create(ctx context.Context, req *dto.CreateRequirementRequest, userID uint64) (*dto.RequirementResponse, error) {
	// 验证需求池是否存在
	pool, err := s.poolRepo.GetByID(ctx, req.PoolID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("requirement.pool_not_found")
		}
		return nil, fmt.Errorf("获取需求池失败: %w", err)
	}

	// 验证需求池状态
	if pool.Status != model.RequirementPoolStatusActive {
		return nil, errors.New("requirement.pool_archived")
	}

	// 处理标签
	tags := ""
	if len(req.Tags) > 0 {
		tags = strings.Join(req.Tags, ",")
	}

	// 解析开始时间
	var startDate *time.Time
	if req.StartDate != nil && *req.StartDate != "" {
		t, err := dto.ParseDateTime(*req.StartDate)
		if err != nil {
			return nil, fmt.Errorf("开始时间格式错误: %w", err)
		}
		startDate = &t
	}

	// 解析结束时间
	var endDate *time.Time
	if req.EndDate != nil && *req.EndDate != "" {
		t, err := dto.ParseDateTime(*req.EndDate)
		if err != nil {
			return nil, fmt.Errorf("结束时间格式错误: %w", err)
		}
		endDate = &t
	}

	requirement := &model.Requirement{
		PoolID:          req.PoolID,
		Title:           req.Title,
		Description:     req.Description,
		Priority:        req.Priority,
		Status:          model.RequirementStatusPendingReview,
		Category:        req.Category,
		ReporterID:      req.ReporterID,
		AssigneeID:      req.AssigneeID,
		StartDate:       startDate,
		EndDate:         endDate,
		TargetProjectID: req.TargetProjectID,
		CreatedBy:       userID,
		Tags:            tags,
	}

	if err := s.repo.Create(ctx, requirement); err != nil {
		s.logger.Error("failed to create requirement",
			zap.Error(err),
			zap.String("title", req.Title),
			zap.Uint64("user_id", userID),
		)
		return nil, fmt.Errorf("创建需求失败: %w", err)
	}

	s.logger.Info("requirement created",
		zap.Uint64("requirement_id", requirement.ID),
		zap.String("title", requirement.Title),
		zap.Uint64("pool_id", requirement.PoolID),
		zap.Uint64("user_id", userID),
	)

	return s.GetByID(ctx, requirement.ID)
}

// GetByID 根据ID获取需求
func (s *requirementService) GetByID(ctx context.Context, id uint64) (*dto.RequirementResponse, error) {
	requirement, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("requirement.not_found")
		}
		s.logger.Error("failed to get requirement",
			zap.Error(err),
			zap.Uint64("requirement_id", id),
		)
		return nil, fmt.Errorf("获取需求失败: %w", err)
	}

	return s.toRequirementResponse(requirement), nil
}

// Update 更新需求
func (s *requirementService) Update(ctx context.Context, id uint64, req *dto.UpdateRequirementRequest, userID uint64) error {
	requirement, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("requirement.not_found")
		}
		return fmt.Errorf("获取需求失败: %w", err)
	}

	// 使用 map 收集变更字段，避免 GORM Save + Preload 关联对象干扰
	fields := make(map[string]interface{})

	if req.Title != nil {
		fields["title"] = *req.Title
	}
	if req.Description != nil {
		fields["description"] = *req.Description
	}
	if req.Priority != nil {
		fields["priority"] = *req.Priority
	}
	if req.Status != nil {
		// 验证状态转换
		if err := s.validateStatusTransition(requirement.Status, *req.Status); err != nil {
			return err
		}
		fields["status"] = *req.Status
	}
	if req.Category != nil {
		fields["category"] = *req.Category
	}
	// 可空字段：前端全量提交，通过 RawFields 判断字段是否出现在 JSON 中
	if req.HasField("reporter_id") {
		fields["reporter_id"] = req.ReporterID // nil 表示清空
	}
	if req.HasField("assignee_id") {
		fields["assignee_id"] = req.AssigneeID
	}
	if req.HasField("target_project_id") {
		fields["target_project_id"] = req.TargetProjectID
	}
	if req.HasField("start_date") {
		if req.StartDate == nil || *req.StartDate == "" {
			fields["start_date"] = nil
		} else {
			t, err := dto.ParseDateTime(*req.StartDate)
			if err != nil {
				return fmt.Errorf("开始时间格式错误: %w", err)
			}
			fields["start_date"] = &t
		}
	}
	if req.HasField("end_date") {
		if req.EndDate == nil || *req.EndDate == "" {
			fields["end_date"] = nil
		} else {
			t, err := dto.ParseDateTime(*req.EndDate)
			if err != nil {
				return fmt.Errorf("结束时间格式错误: %w", err)
			}
			fields["end_date"] = &t
		}
	}
	if req.Progress != nil {
		fields["progress"] = *req.Progress
	}
	if req.Result != nil {
		fields["result"] = *req.Result
	}
	if req.Tags != nil {
		fields["tags"] = strings.Join(req.Tags, ",")
	}

	if len(fields) == 0 {
		return nil
	}

	if err := s.repo.UpdateFields(ctx, id, fields); err != nil {
		s.logger.Error("failed to update requirement",
			zap.Error(err),
			zap.Uint64("requirement_id", id),
			zap.Uint64("user_id", userID),
		)
		return fmt.Errorf("更新需求失败: %w", err)
	}

	s.logger.Info("requirement updated",
		zap.Uint64("requirement_id", id),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// Delete 删除需求
func (s *requirementService) Delete(ctx context.Context, id, userID uint64) error {
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("requirement.not_found")
		}
		return fmt.Errorf("获取需求失败: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete requirement",
			zap.Error(err),
			zap.Uint64("requirement_id", id),
			zap.Uint64("user_id", userID),
		)
		return fmt.Errorf("删除需求失败: %w", err)
	}

	s.logger.Info("requirement deleted",
		zap.Uint64("requirement_id", id),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// List 获取需求列表
func (s *requirementService) List(ctx context.Context, req *dto.RequirementListRequest) ([]*dto.RequirementResponse, int64, error) {
	filter := &repository.RequirementFilter{
		PoolID:     req.PoolID,
		Status:     req.Status,
		Priority:   req.Priority,
		Category:   req.Category,
		AssigneeID: req.AssigneeID,
		CreatedBy:  req.CreatedBy,
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
		Keyword:    req.Keyword,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}

	// 设置默认分页
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}

	requirements, total, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list requirements",
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取需求列表失败: %w", err)
	}

	responses := make([]*dto.RequirementResponse, 0, len(requirements))
	for _, requirement := range requirements {
		responses = append(responses, s.toRequirementResponse(requirement))
	}

	return responses, total, nil
}

// ConvertToIssue 转化为工单（通过 IssueService.CreateIssue 走标准建单流程）
func (s *requirementService) ConvertToIssue(ctx context.Context, id uint64, req *dto.ConvertToIssueRequest, userID uint64) (*dto.ConvertToIssueResponse, error) {
	if s.issueSvc == nil {
		return nil, errors.New("requirement.service_uninitialized")
	}

	// 分布式锁防止同一需求被并发转化
	lockKey := fmt.Sprintf("requirement:convert:%d", id)
	lockVal := cache.TryLock(ctx, lockKey, 30*time.Second)
	if lockVal != "" {
		defer cache.UnlockWithValue(ctx, lockKey, lockVal)
	}

	requirement, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("requirement.not_found")
		}
		return nil, fmt.Errorf("获取需求失败: %w", err)
	}

	// 终态不允许转化
	if requirement.Status == model.RequirementStatusCompleted {
		return nil, errors.New("requirement.completed_no_convert")
	}
	if requirement.Status == model.RequirementStatusRejected {
		return nil, errors.New("requirement.rejected_no_convert")
	}

	// 已转化的不允许再次转化
	if requirement.ConvertedIssueID != nil {
		return nil, errors.New("requirement.already_converted")
	}

	// 通过 IssueService 标准流程创建工单（包含工作流实例、关注人、通知等）
	// 将需求的日期转换为工单的日期格式
	var plannedStartDate, plannedEndDate *string
	if requirement.StartDate != nil {
		s := requirement.StartDate.Format("2006-01-02")
		plannedStartDate = &s
	}
	if requirement.EndDate != nil {
		s := requirement.EndDate.Format("2006-01-02")
		plannedEndDate = &s
	}

	issueReq := &issueDto.CreateIssueRequest{
		ProjectKey:       req.ProjectKey,
		IssueTypeID:      req.IssueTypeID,
		Title:            requirement.Title,
		Description:      requirement.Description,
		Priority:         string(requirement.Priority),
		AssigneeID:       req.AssigneeID,
		PlannedStartDate: plannedStartDate,
		PlannedEndDate:   plannedEndDate,
	}

	issueResp, err := s.issueSvc.CreateIssue(ctx, issueReq, userID)
	if err != nil {
		s.logger.Error("failed to create issue from requirement",
			zap.Error(err),
			zap.Uint64("requirement_id", id),
			zap.Uint64("user_id", userID),
		)
		return nil, fmt.Errorf("创建工单失败: %w", err)
	}

	// 事务中更新需求状态和关联
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 构造只更新变化字段的 map，避免 Save 写回全部列
		updates := map[string]interface{}{
			"converted_issue_id": issueResp.ID,
		}

		// 非终态且非 in_progress 的需求自动流转到 in_progress
		if requirement.Status == model.RequirementStatusPendingReview ||
			requirement.Status == model.RequirementStatusPlanning ||
			requirement.Status == model.RequirementStatusOnHold {
			updates["status"] = model.RequirementStatusInProgress
			requirement.Status = model.RequirementStatusInProgress
		}

		if updateErr := tx.Model(requirement).Updates(updates).Error; updateErr != nil {
			return fmt.Errorf("更新需求关联失败: %w", updateErr)
		}

		// 创建需求侧活动记录（工单侧由 IssueService 内部处理）
		var user model.User
		if queryErr := tx.First(&user, userID).Error; queryErr == nil {
			activity := &model.ActivityLog{
				UserID:     userID,
				UserName:   user.DisplayName,
				Action:     "requirement_converted",
				EntityType: "requirement",
				EntityID:   requirement.ID,
				EntityKey:  requirement.Title,
				Details: detail.New("activity.detail.requirementConverted",
					"issueId", issueResp.ID, "issueKey", issueResp.IssueKey),
			}
			if createErr := tx.Create(activity).Error; createErr != nil {
				s.logger.Warn("failed to create requirement activity log", zap.Error(createErr))
			}
		}

		return nil
	})

	if err != nil {
		s.logger.Error("failed to update requirement after issue creation",
			zap.Error(err),
			zap.Uint64("requirement_id", id),
			zap.Uint64("issue_id", issueResp.ID),
			zap.Uint64("user_id", userID),
		)
		return nil, err
	}

	s.logger.Info("requirement converted to issue",
		zap.Uint64("requirement_id", id),
		zap.Uint64("issue_id", issueResp.ID),
		zap.String("issue_key", issueResp.IssueKey),
		zap.Uint64("user_id", userID),
	)

	return &dto.ConvertToIssueResponse{
		IssueID:  issueResp.ID,
		IssueKey: issueResp.IssueKey,
	}, nil
}

// AddComment 添加评论
func (s *requirementService) AddComment(ctx context.Context, id uint64, req *dto.RequirementCommentRequest, userID uint64) (*dto.RequirementCommentResponse, error) {
	// 验证需求是否存在
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("requirement.not_found")
		}
		return nil, fmt.Errorf("获取需求失败: %w", err)
	}

	comment := &model.RequirementComment{
		RequirementID: id,
		UserID:        userID,
		Content:       req.Content,
	}

	if err := s.db.WithContext(ctx).Create(comment).Error; err != nil {
		s.logger.Error("failed to create requirement comment",
			zap.Error(err),
			zap.Uint64("requirement_id", id),
			zap.Uint64("user_id", userID),
		)
		return nil, fmt.Errorf("添加评论失败: %w", err)
	}

	s.logger.Info("requirement comment created",
		zap.Uint64("comment_id", comment.ID),
		zap.Uint64("requirement_id", id),
		zap.Uint64("user_id", userID),
	)

	// 预加载用户信息
	if err := s.db.WithContext(ctx).Preload("User").First(comment, comment.ID).Error; err != nil {
		return nil, fmt.Errorf("获取评论失败: %w", err)
	}

	return s.toCommentResponse(comment), nil
}

// GetKanban 获取看板数据
func (s *requirementService) GetKanban(ctx context.Context, req *dto.KanbanRequest) (*dto.KanbanResponse, error) {
	filter := &repository.KanbanFilter{
		PoolID:    req.PoolID,
		GroupBy:   req.GroupBy,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	data, err := s.repo.GetKanbanData(ctx, filter)
	if err != nil {
		s.logger.Error("failed to get kanban data",
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取看板数据失败: %w", err)
	}

	// 构建响应
	response := &dto.KanbanResponse{
		GroupBy: req.GroupBy,
		Columns: make([]*dto.KanbanColumn, 0),
		Total:   0,
	}

	// 根据分组类型生成列
	switch req.GroupBy {
	case "status":
		statuses := []model.RequirementStatus{
			model.RequirementStatusPendingReview,
			model.RequirementStatusPlanning,
			model.RequirementStatusInProgress,
			model.RequirementStatusCompleted,
			model.RequirementStatusOnHold,
			model.RequirementStatusRejected,
		}
		statusTitles := map[model.RequirementStatus]string{
			model.RequirementStatusPendingReview: "待评估",
			model.RequirementStatusPlanning:      "规划中",
			model.RequirementStatusInProgress:    "进行中",
			model.RequirementStatusCompleted:     "已完成",
			model.RequirementStatusOnHold:        "已搁置",
			model.RequirementStatusRejected:      "已拒绝",
		}
		for _, status := range statuses {
			key := string(status)
			requirements := data[key]
			column := &dto.KanbanColumn{
				Key:          key,
				Title:        statusTitles[status],
				Count:        int64(len(requirements)),
				Requirements: make([]*dto.RequirementResponse, 0),
			}
			for _, req := range requirements {
				column.Requirements = append(column.Requirements, s.toRequirementResponse(req))
			}
			response.Columns = append(response.Columns, column)
			response.Total += column.Count
		}

	case "priority":
		priorities := []model.RequirementPriority{
			model.RequirementPriorityP0,
			model.RequirementPriorityP1,
			model.RequirementPriorityP2,
			model.RequirementPriorityP3,
		}
		priorityTitles := map[model.RequirementPriority]string{
			model.RequirementPriorityP0: "P0 - 紧急",
			model.RequirementPriorityP1: "P1 - 高",
			model.RequirementPriorityP2: "P2 - 中",
			model.RequirementPriorityP3: "P3 - 低",
		}
		for _, priority := range priorities {
			key := string(priority)
			requirements := data[key]
			column := &dto.KanbanColumn{
				Key:          key,
				Title:        priorityTitles[priority],
				Count:        int64(len(requirements)),
				Requirements: make([]*dto.RequirementResponse, 0),
			}
			for _, req := range requirements {
				column.Requirements = append(column.Requirements, s.toRequirementResponse(req))
			}
			response.Columns = append(response.Columns, column)
			response.Total += column.Count
		}

	case "assignee", "timeline":
		// 动态生成列
		for key, requirements := range data {
			column := &dto.KanbanColumn{
				Key:          key,
				Title:        key,
				Count:        int64(len(requirements)),
				Requirements: make([]*dto.RequirementResponse, 0),
			}
			for _, req := range requirements {
				column.Requirements = append(column.Requirements, s.toRequirementResponse(req))
			}
			response.Columns = append(response.Columns, column)
			response.Total += column.Count
		}
	}

	return response, nil
}

// GetReport 获取报告数据
func (s *requirementService) GetReport(ctx context.Context, req *dto.ReportRequest) (*dto.ReportResponse, error) {
	filter := &repository.StatisticsFilter{
		PoolID:    req.PoolID,
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
	}

	stats, err := s.repo.GetStatistics(ctx, filter)
	if err != nil {
		s.logger.Error("failed to get statistics",
			zap.Error(err),
		)
		return nil, fmt.Errorf("获取统计数据失败: %w", err)
	}

	response := &dto.ReportResponse{
		PoolID:         req.PoolID,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		TotalCreated:   stats.TotalCreated,
		TotalCompleted: stats.TotalCompleted,
		TotalRejected:  stats.TotalRejected,
		AvgProcessDays: stats.AvgProcessDays,
	}

	// 获取需求池名称
	if req.PoolID != nil {
		pool, poolErr := s.poolRepo.GetByID(ctx, *req.PoolID)
		if poolErr == nil {
			response.PoolName = pool.Name
		}
	}

	// 按状态统计
	poolID := uint64(0)
	if req.PoolID != nil {
		poolID = *req.PoolID
	}
	statusCounts, err := s.repo.CountByStatus(ctx, poolID)
	if err == nil {
		response.StatusSummary = make([]*dto.StatusSummary, 0)
		for status, count := range statusCounts {
			response.StatusSummary = append(response.StatusSummary, &dto.StatusSummary{
				Status: status,
				Count:  count,
			})
		}
	}

	// 按优先级统计
	priorityCounts, err := s.repo.CountByPriority(ctx, poolID)
	if err == nil {
		response.PrioritySummary = make([]*dto.PrioritySummary, 0)
		for priority, count := range priorityCounts {
			response.PrioritySummary = append(response.PrioritySummary, &dto.PrioritySummary{
				Priority: priority,
				Count:    count,
			})
		}
	}

	// TODO: 实现负责人统计和趋势数据
	response.AssigneeSummary = make([]*dto.AssigneeSummary, 0)
	response.TrendData = make([]*dto.TrendData, 0)

	return response, nil
}

// validateStatusTransition 验证状态转换
func (s *requirementService) validateStatusTransition(from, to model.RequirementStatus) error {
	// 定义允许的状态转换
	allowedTransitions := map[model.RequirementStatus][]model.RequirementStatus{
		model.RequirementStatusPendingReview: {
			model.RequirementStatusPlanning,
			model.RequirementStatusInProgress,
			model.RequirementStatusRejected,
			model.RequirementStatusOnHold,
		},
		model.RequirementStatusPlanning: {
			model.RequirementStatusInProgress,
			model.RequirementStatusOnHold,
			model.RequirementStatusRejected,
			model.RequirementStatusPendingReview,
		},
		model.RequirementStatusInProgress: {
			model.RequirementStatusCompleted,
			model.RequirementStatusOnHold,
			model.RequirementStatusRejected,
		},
		model.RequirementStatusOnHold: {
			model.RequirementStatusPlanning,
			model.RequirementStatusInProgress,
			model.RequirementStatusPendingReview,
		},
		model.RequirementStatusCompleted: {},
		model.RequirementStatusRejected: {
			model.RequirementStatusPendingReview,
		},
	}

	allowed, exists := allowedTransitions[from]
	if !exists {
		return fmt.Errorf("无效的状态: %s", from)
	}

	for _, status := range allowed {
		if status == to {
			return nil
		}
	}

	return fmt.Errorf("不允许从 %s 转换到 %s", from, to)
}

// toRequirementResponse 转换为需求响应DTO
func (s *requirementService) toRequirementResponse(req *model.Requirement) *dto.RequirementResponse {
	resp := &dto.RequirementResponse{
		ID:               req.ID,
		PoolID:           req.PoolID,
		Title:            req.Title,
		Description:      req.Description,
		Priority:         req.Priority,
		Status:           req.Status,
		Category:         req.Category,
		ReporterID:       req.ReporterID,
		AssigneeID:       req.AssigneeID,
		StartDate:        req.StartDate,
		EndDate:          req.EndDate,
		Progress:         req.Progress,
		Result:           req.Result,
		ConvertedIssueID: req.ConvertedIssueID,
		TargetProjectID:  req.TargetProjectID,
		CreatedBy:        req.CreatedBy,
		CreatedAt:        req.CreatedAt,
		UpdatedAt:        req.UpdatedAt,
	}

	// 处理标签
	if req.Tags != "" {
		resp.Tags = strings.Split(req.Tags, ",")
	} else {
		resp.Tags = []string{}
	}

	// 关联数据
	if req.Pool != nil {
		resp.PoolName = req.Pool.Name
	}
	if req.Reporter != nil {
		resp.ReporterName = req.Reporter.DisplayName
	}
	if req.Assignee != nil {
		resp.AssigneeName = req.Assignee.DisplayName
	}
	if req.Creator != nil {
		resp.CreatorName = req.Creator.DisplayName
	}
	if req.ConvertedIssue != nil {
		resp.ConvertedIssueKey = req.ConvertedIssue.IssueKey
		resp.ConvertedIssueStatus = req.ConvertedIssue.Status
	}
	if req.TargetProject != nil {
		resp.TargetProjectName = req.TargetProject.Name
	}

	// 评论数量
	if req.Comments != nil {
		resp.CommentCount = int64(len(req.Comments))
	}

	return resp
}

// toCommentResponse 转换为评论响应DTO
func (s *requirementService) toCommentResponse(comment *model.RequirementComment) *dto.RequirementCommentResponse {
	resp := &dto.RequirementCommentResponse{
		ID:            comment.ID,
		RequirementID: comment.RequirementID,
		UserID:        comment.UserID,
		Content:       comment.Content,
		CreatedAt:     comment.CreatedAt,
	}

	if comment.User != nil {
		resp.UserName = comment.User.DisplayName
	}

	return resp
}
