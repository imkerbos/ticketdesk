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

// CategoryService 需求分类业务逻辑接口
type CategoryService interface {
	List(ctx context.Context) ([]*dto.CategoryResponse, error)
	Create(ctx context.Context, req *dto.CreateCategoryRequest) (*dto.CategoryResponse, error)
	Update(ctx context.Context, id uint64, req *dto.UpdateCategoryRequest) error
	Delete(ctx context.Context, id uint64) error
}

// categoryService 需求分类业务逻辑实现
type categoryService struct {
	repo   repository.CategoryRepository
	logger *zap.Logger
}

// NewCategoryService 创建需求分类业务逻辑实例
func NewCategoryService(repo repository.CategoryRepository, logger *zap.Logger) CategoryService {
	return &categoryService{
		repo:   repo,
		logger: logger,
	}
}

// List 获取所有分类
func (s *categoryService) List(ctx context.Context) ([]*dto.CategoryResponse, error) {
	categories, err := s.repo.List(ctx)
	if err != nil {
		s.logger.Error("failed to list requirement categories", zap.Error(err))
		return nil, fmt.Errorf("获取需求分类失败: %w", err)
	}

	responses := make([]*dto.CategoryResponse, 0, len(categories))
	for _, cat := range categories {
		responses = append(responses, toCategoryResponse(cat))
	}
	return responses, nil
}

// Create 创建分类
func (s *categoryService) Create(ctx context.Context, req *dto.CreateCategoryRequest) (*dto.CategoryResponse, error) {
	// 检查名称是否已存在
	existing, err := s.repo.GetByName(ctx, req.Name)
	if err == nil && existing.ID > 0 {
		return nil, errors.New("requirement.category_exists")
	}

	cat := &model.RequirementCategoryDef{
		Name:  req.Name,
		Label: req.Label,
		Color: req.Color,
	}

	if err := s.repo.Create(ctx, cat); err != nil {
		s.logger.Error("failed to create requirement category", zap.Error(err), zap.String("name", req.Name))
		return nil, fmt.Errorf("创建需求分类失败: %w", err)
	}

	s.logger.Info("requirement category created", zap.Uint64("id", cat.ID), zap.String("name", cat.Name))
	return toCategoryResponse(cat), nil
}

// Update 更新分类
func (s *categoryService) Update(ctx context.Context, id uint64, req *dto.UpdateCategoryRequest) error {
	cat, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("requirement.category_not_found")
		}
		return fmt.Errorf("获取分类失败: %w", err)
	}

	fields := make(map[string]any)
	if req.Label != nil {
		fields["label"] = *req.Label
	}
	if req.Color != nil {
		fields["color"] = *req.Color
	}
	if req.SortOrder != nil {
		fields["sort_order"] = *req.SortOrder
	}
	if req.IsDefault != nil {
		fields["is_default"] = *req.IsDefault
	}

	if len(fields) == 0 {
		return nil
	}

	if err := s.repo.UpdateFields(ctx, id, fields); err != nil {
		s.logger.Error("failed to update requirement category", zap.Error(err), zap.Uint64("id", id))
		return fmt.Errorf("更新需求分类失败: %w", err)
	}

	s.logger.Info("requirement category updated", zap.Uint64("id", id), zap.String("name", cat.Name))
	return nil
}

// Delete 删除分类
func (s *categoryService) Delete(ctx context.Context, id uint64) error {
	cat, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("requirement.category_not_found")
		}
		return fmt.Errorf("获取分类失败: %w", err)
	}

	// 系统预置分类不可删除
	if cat.IsSystem {
		return errors.New("requirement.category_system")
	}

	// 检查是否有关联需求
	count, err := s.repo.CountRequirementsByCategory(ctx, cat.Name)
	if err != nil {
		return fmt.Errorf("检查关联需求失败: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("该分类下有 %d 个需求，无法删除", count)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("failed to delete requirement category", zap.Error(err), zap.Uint64("id", id))
		return fmt.Errorf("删除需求分类失败: %w", err)
	}

	s.logger.Info("requirement category deleted", zap.Uint64("id", id), zap.String("name", cat.Name))
	return nil
}

// toCategoryResponse 转换为分类响应
func toCategoryResponse(cat *model.RequirementCategoryDef) *dto.CategoryResponse {
	return &dto.CategoryResponse{
		ID:        cat.ID,
		Name:      cat.Name,
		Label:     cat.Label,
		Color:     cat.Color,
		SortOrder: cat.SortOrder,
		IsDefault: cat.IsDefault,
		IsSystem:  cat.IsSystem,
	}
}
