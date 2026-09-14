package service

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/internal/requirement-pool/dto"
	"github.com/kerbos/ticketdesk/internal/requirement-pool/repository"
)

// RequirementPoolService 需求池业务逻辑接口
type RequirementPoolService interface {
	Create(ctx context.Context, req *dto.CreateRequirementPoolRequest, userID uint64) (*dto.RequirementPoolResponse, error)
	GetByID(ctx context.Context, id uint64) (*dto.RequirementPoolResponse, error)
	Update(ctx context.Context, id uint64, req *dto.UpdateRequirementPoolRequest, userID uint64) error
	Delete(ctx context.Context, id, userID uint64) error
	List(ctx context.Context, req *dto.RequirementPoolListRequest) ([]*dto.RequirementPoolResponse, int64, error)
	GetByProjectID(ctx context.Context, projectID uint64) ([]*dto.RequirementPoolResponse, error)
}

// requirementPoolService 需求池业务逻辑实现
type requirementPoolService struct {
	repo    repository.RequirementPoolRepository
	reqRepo repository.RequirementRepository
	logger  *zap.Logger
}

// NewRequirementPoolService 创建需求池业务逻辑实例
func NewRequirementPoolService(
	repo repository.RequirementPoolRepository,
	reqRepo repository.RequirementRepository,
	logger *zap.Logger,
) RequirementPoolService {
	return &requirementPoolService{
		repo:    repo,
		reqRepo: reqRepo,
		logger:  logger,
	}
}

// Create 创建需求池
func (s *requirementPoolService) Create(ctx context.Context, req *dto.CreateRequirementPoolRequest, userID uint64) (*dto.RequirementPoolResponse, error) {
	// 验证项目级需求池必须关联项目
	if req.Type == model.RequirementPoolTypeProject && req.ProjectID == nil {
		return nil, errors.New("requirement.project_needs_project")
	}

	// 验证全局需求池不能关联项目
	if req.Type == model.RequirementPoolTypeGlobal && req.ProjectID != nil {
		return nil, errors.New("requirement.global_no_project")
	}

	pool := &model.RequirementPool{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		ProjectID:   req.ProjectID,
		OwnerID:     req.OwnerID,
		Status:      model.RequirementPoolStatusActive,
	}

	if err := s.repo.Create(ctx, pool); err != nil {
		s.logger.Error("failed to create requirement pool",
			zap.Error(err),
			zap.String("name", req.Name),
			zap.Uint64("user_id", userID),
		)
		return nil, fmt.Errorf("创建需求池失败: %w", err)
	}

	s.logger.Info("requirement pool created",
		zap.Uint64("pool_id", pool.ID),
		zap.String("name", pool.Name),
		zap.Uint64("user_id", userID),
	)

	return s.GetByID(ctx, pool.ID)
}

// GetByID 根据ID获取需求池
func (s *requirementPoolService) GetByID(ctx context.Context, id uint64) (*dto.RequirementPoolResponse, error) {
	pool, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("requirement.pool_not_found")
		}
		s.logger.Error("failed to get requirement pool",
			zap.Error(err),
			zap.Uint64("pool_id", id),
		)
		return nil, fmt.Errorf("获取需求池失败: %w", err)
	}

	// 统计需求数量
	requirements, err := s.reqRepo.GetByPoolID(ctx, id)
	if err != nil {
		s.logger.Error("failed to count requirements",
			zap.Error(err),
			zap.Uint64("pool_id", id),
		)
	}

	return s.toPoolResponse(pool, int64(len(requirements))), nil
}

// Update 更新需求池
func (s *requirementPoolService) Update(ctx context.Context, id uint64, req *dto.UpdateRequirementPoolRequest, userID uint64) error {
	pool, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("requirement.pool_not_found")
		}
		return fmt.Errorf("获取需求池失败: %w", err)
	}

	// 更新字段
	if req.Name != nil {
		pool.Name = *req.Name
	}
	if req.Description != nil {
		pool.Description = *req.Description
	}
	if req.OwnerID != nil {
		pool.OwnerID = *req.OwnerID
	}
	if req.Status != nil {
		pool.Status = *req.Status
	}

	if err := s.repo.Update(ctx, pool); err != nil {
		s.logger.Error("failed to update requirement pool",
			zap.Error(err),
			zap.Uint64("pool_id", id),
			zap.Uint64("user_id", userID),
		)
		return fmt.Errorf("更新需求池失败: %w", err)
	}

	s.logger.Info("requirement pool updated",
		zap.Uint64("pool_id", id),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// Delete 删除需求池
func (s *requirementPoolService) Delete(ctx context.Context, id, userID uint64) error {
	// 检查需求池是否存在
	pool, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("requirement.pool_not_found")
		}
		return fmt.Errorf("获取需求池失败: %w", err)
	}

	// 检查是否有未完成的需求
	requirements, err := s.reqRepo.GetByPoolID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取需求列表失败: %w", err)
	}

	activeCount := 0
	for _, req := range requirements {
		if req.Status != model.RequirementStatusCompleted && req.Status != model.RequirementStatusRejected {
			activeCount++
		}
	}

	if activeCount > 0 {
		return fmt.Errorf("需求池中还有 %d 个未完成的需求，无法删除", activeCount)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete requirement pool",
			zap.Error(err),
			zap.Uint64("pool_id", id),
			zap.Uint64("user_id", userID),
		)
		return fmt.Errorf("删除需求池失败: %w", err)
	}

	s.logger.Info("requirement pool deleted",
		zap.Uint64("pool_id", id),
		zap.String("pool_name", pool.Name),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// List 获取需求池列表
func (s *requirementPoolService) List(ctx context.Context, req *dto.RequirementPoolListRequest) ([]*dto.RequirementPoolResponse, int64, error) {
	filter := &repository.RequirementPoolFilter{
		Type:      req.Type,
		Status:    req.Status,
		ProjectID: req.ProjectID,
		OwnerID:   req.OwnerID,
		Page:      req.Page,
		PageSize:  req.PageSize,
	}

	// 设置默认分页
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 20
	}

	pools, total, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list requirement pools",
			zap.Error(err),
		)
		return nil, 0, fmt.Errorf("获取需求池列表失败: %w", err)
	}

	responses := make([]*dto.RequirementPoolResponse, 0, len(pools))
	for _, pool := range pools {
		// 统计需求数量
		requirements, reqErr := s.reqRepo.GetByPoolID(ctx, pool.ID)
		if reqErr != nil {
			s.logger.Warn("failed to count requirements by pool",
				zap.Uint64("pool_id", pool.ID),
				zap.Error(reqErr),
			)
			requirements = nil
		}
		responses = append(responses, s.toPoolResponse(pool, int64(len(requirements))))
	}

	return responses, total, nil
}

// GetByProjectID 根据项目ID获取需求池列表
func (s *requirementPoolService) GetByProjectID(ctx context.Context, projectID uint64) ([]*dto.RequirementPoolResponse, error) {
	pools, err := s.repo.GetByProjectID(ctx, projectID)
	if err != nil {
		s.logger.Error("failed to get requirement pools by project",
			zap.Error(err),
			zap.Uint64("project_id", projectID),
		)
		return nil, fmt.Errorf("获取项目需求池失败: %w", err)
	}

	responses := make([]*dto.RequirementPoolResponse, 0, len(pools))
	for _, pool := range pools {
		// 统计需求数量
		requirements, reqErr := s.reqRepo.GetByPoolID(ctx, pool.ID)
		if reqErr != nil {
			s.logger.Warn("failed to count requirements by pool",
				zap.Uint64("pool_id", pool.ID),
				zap.Error(reqErr),
			)
			requirements = nil
		}
		responses = append(responses, s.toPoolResponse(pool, int64(len(requirements))))
	}

	return responses, nil
}

// toPoolResponse 转换为响应DTO
func (s *requirementPoolService) toPoolResponse(pool *model.RequirementPool, requirementCount int64) *dto.RequirementPoolResponse {
	resp := &dto.RequirementPoolResponse{
		ID:               pool.ID,
		Name:             pool.Name,
		Description:      pool.Description,
		Type:             pool.Type,
		ProjectID:        pool.ProjectID,
		OwnerID:          pool.OwnerID,
		Status:           pool.Status,
		RequirementCount: requirementCount,
		CreatedAt:        pool.CreatedAt,
		UpdatedAt:        pool.UpdatedAt,
	}

	if pool.Owner != nil {
		resp.OwnerName = pool.Owner.DisplayName
	}

	if pool.Project != nil {
		resp.ProjectName = pool.Project.Name
	}

	return resp
}
