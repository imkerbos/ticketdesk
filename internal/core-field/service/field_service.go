// Package service 提供字段业务逻辑层
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-field/dto"
	"github.com/kerbos/ticketdesk/internal/core-field/repository"
	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	userRepo "github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// 业务错误定义
var (
	ErrProjectNotFound     = errors.New("workflow.project_not_found")
	ErrFieldNotFound       = errors.New("field.not_found")
	ErrFieldKeyExists      = errors.New("field.key_exists")
	ErrCannotDeleteSystem  = errors.New("field.system_undeletable")
	ErrCannotModifySystem  = errors.New("field.system_immutable")
	ErrIssueTypeNotFound   = errors.New("project.issue_type_not_found")
	ErrVersionNotFound     = errors.New("field.version_not_found")
	ErrVersionNameExists   = errors.New("field.version_name_exists")
	ErrComponentNotFound   = errors.New("field.component_not_found")
	ErrComponentNameExists = errors.New("field.component_name_exists")
	ErrLabelNotFound       = errors.New("field.label_not_found")
	ErrLabelNameExists     = errors.New("field.label_name_exists")
	ErrTemplateNotFound    = errors.New("field.template_not_found")
	ErrTemplateNameExists  = errors.New("field.template_name_exists")
)

// FieldService 字段服务接口
type FieldService interface {
	// 字段定义
	CreateField(ctx context.Context, projectKey string, req *dto.CreateFieldRequest) (*dto.FieldDefinitionResponse, error)
	UpdateField(ctx context.Context, projectKey string, fieldID uint64, req *dto.UpdateFieldRequest) (*dto.FieldDefinitionResponse, error)
	DeleteField(ctx context.Context, projectKey string, fieldID uint64) error
	GetField(ctx context.Context, fieldID uint64) (*dto.FieldDefinitionResponse, error)
	ListFields(ctx context.Context, projectKey string) ([]*dto.FieldDefinitionResponse, error)

	// 字段方案
	GetFieldScheme(ctx context.Context, projectKey string, issueTypeID uint64) ([]*dto.FieldSchemeResponse, error)
	UpdateFieldScheme(ctx context.Context, projectKey string, issueTypeID uint64, req *dto.UpdateFieldSchemeRequest) ([]*dto.FieldSchemeResponse, error)

	// 字段值
	GetFieldValues(ctx context.Context, issueID uint64) ([]*dto.FieldValueResponse, error)
	SaveFieldValues(ctx context.Context, issueID uint64, values []*dto.CustomFieldValue) error
	SaveIssueFieldValues(ctx context.Context, issueID uint64, values []CustomFieldValueInput) error
	GetIssueIDsByEpicLink(ctx context.Context, epicID uint64) ([]uint64, error)

	// 版本管理
	CreateVersion(ctx context.Context, projectKey string, req *dto.CreateVersionRequest) (*dto.VersionResponse, error)
	UpdateVersion(ctx context.Context, projectKey string, versionID uint64, req *dto.UpdateVersionRequest) (*dto.VersionResponse, error)
	DeleteVersion(ctx context.Context, projectKey string, versionID uint64) error
	ListVersions(ctx context.Context, projectKey string) ([]*dto.VersionResponse, error)

	// 组件管理
	CreateComponent(ctx context.Context, projectKey string, req *dto.CreateComponentRequest) (*dto.ComponentResponse, error)
	UpdateComponent(ctx context.Context, projectKey string, componentID uint64, req *dto.UpdateComponentRequest) (*dto.ComponentResponse, error)
	DeleteComponent(ctx context.Context, projectKey string, componentID uint64) error
	ListComponents(ctx context.Context, projectKey string) ([]*dto.ComponentResponse, error)

	// 标签管理
	CreateLabel(ctx context.Context, projectKey string, req *dto.CreateLabelRequest) (*dto.LabelResponse, error)
	UpdateLabel(ctx context.Context, projectKey string, labelID uint64, req *dto.UpdateLabelRequest) (*dto.LabelResponse, error)
	DeleteLabel(ctx context.Context, projectKey string, labelID uint64) error
	ListLabels(ctx context.Context, projectKey string) ([]*dto.LabelResponse, error)

	// 全局字段管理
	ListGlobalFields(ctx context.Context) ([]*dto.FieldDefinitionResponse, error)
	CreateGlobalField(ctx context.Context, req *dto.CreateFieldRequest) (*dto.FieldDefinitionResponse, error)
	UpdateGlobalField(ctx context.Context, fieldID uint64, req *dto.UpdateFieldRequest) (*dto.FieldDefinitionResponse, error)
	DeleteGlobalField(ctx context.Context, fieldID uint64) error
	GetFieldUsage(ctx context.Context, fieldID uint64) (*dto.FieldUsageResponse, error)

	// 方案模板
	ListTemplates(ctx context.Context) ([]*dto.TemplateResponse, error)
	CreateTemplate(ctx context.Context, userID uint64, req *dto.CreateTemplateRequest) (*dto.TemplateResponse, error)
	GetTemplate(ctx context.Context, templateID uint64) (*dto.TemplateDetailResponse, error)
	UpdateTemplate(ctx context.Context, templateID uint64, req *dto.UpdateTemplateRequest) (*dto.TemplateResponse, error)
	DeleteTemplate(ctx context.Context, templateID uint64) error
	UpdateTemplateItems(ctx context.Context, templateID uint64, req *dto.UpdateTemplateItemsRequest) error

	// 套用模板
	ApplyTemplate(ctx context.Context, projectKey string, issueTypeID uint64, req *dto.ApplyTemplateRequest) ([]*dto.FieldSchemeResponse, error)
}

// fieldService 字段服务实现
type fieldService struct {
	fieldRepo     repository.FieldRepository
	schemeRepo    repository.SchemeRepository
	valueRepo     repository.ValueRepository
	templateRepo  repository.TemplateRepository
	versionRepo   repository.VersionRepository
	componentRepo repository.ComponentRepository
	labelRepo     repository.LabelRepository
	projectRepo   projectRepo.ProjectRepository
	issueTypeRepo projectRepo.IssueTypeRepository
	userRepo      userRepo.UserRepository
	db            *gorm.DB
}

// NewFieldService 创建字段服务实例
func NewFieldService(
	fieldRepo repository.FieldRepository,
	schemeRepo repository.SchemeRepository,
	valueRepo repository.ValueRepository,
	templateRepo repository.TemplateRepository,
	versionRepo repository.VersionRepository,
	componentRepo repository.ComponentRepository,
	labelRepo repository.LabelRepository,
	projectRepo projectRepo.ProjectRepository,
	issueTypeRepo projectRepo.IssueTypeRepository,
	userRepo userRepo.UserRepository,
	db *gorm.DB,
) FieldService {
	return &fieldService{
		fieldRepo:     fieldRepo,
		schemeRepo:    schemeRepo,
		valueRepo:     valueRepo,
		templateRepo:  templateRepo,
		versionRepo:   versionRepo,
		componentRepo: componentRepo,
		labelRepo:     labelRepo,
		projectRepo:   projectRepo,
		issueTypeRepo: issueTypeRepo,
		userRepo:      userRepo,
		db:            db,
	}
}

// ============ 字段定义 ============

// CreateField 创建自定义字段
func (s *fieldService) CreateField(ctx context.Context, projectKey string, req *dto.CreateFieldRequest) (*dto.FieldDefinitionResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查字段Key是否已存在
	_, err = s.fieldRepo.GetByKey(ctx, &project.ID, req.FieldKey)
	if err == nil {
		return nil, ErrFieldKeyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查字段Key失败: %w", err)
	}

	field := &model.FieldDefinition{
		ProjectID:    &project.ID,
		FieldKey:     req.FieldKey,
		FieldName:    req.FieldName,
		FieldType:    req.FieldType,
		Description:  req.Description,
		IsSystem:     false,
		IsActive:     true,
		Options:      req.Options,
		Validation:   req.Validation,
		DefaultValue: req.DefaultValue,
		SortOrder:    100,
	}

	if err := s.fieldRepo.Create(ctx, field); err != nil {
		logger.Error("failed to create field", zap.Error(err))
		return nil, fmt.Errorf("创建字段失败: %w", err)
	}

	logger.Info("field created",
		zap.String("project_key", projectKey),
		zap.String("field_key", req.FieldKey),
	)

	return s.toFieldResponse(field), nil
}

// UpdateField 更新字段定义
func (s *fieldService) UpdateField(ctx context.Context, projectKey string, fieldID uint64, req *dto.UpdateFieldRequest) (*dto.FieldDefinitionResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	field, err := s.fieldRepo.GetByID(ctx, fieldID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFieldNotFound
		}
		return nil, fmt.Errorf("查询字段失败: %w", err)
	}

	// 验证字段属于该项目（系统字段除外）
	if field.ProjectID != nil && *field.ProjectID != project.ID {
		return nil, ErrFieldNotFound
	}

	// 系统字段只能修改部分属性
	if field.IsSystem {
		if req.IsActive != nil {
			field.IsActive = *req.IsActive
		}
		if req.SortOrder != nil {
			field.SortOrder = *req.SortOrder
		}
	} else {
		if req.FieldName != nil {
			field.FieldName = *req.FieldName
		}
		if req.Description != nil {
			field.Description = *req.Description
		}
		if req.Options != nil {
			field.Options = *req.Options
		}
		if req.Validation != nil {
			field.Validation = *req.Validation
		}
		if req.DefaultValue != nil {
			field.DefaultValue = *req.DefaultValue
		}
		if req.IsActive != nil {
			field.IsActive = *req.IsActive
		}
		if req.SortOrder != nil {
			field.SortOrder = *req.SortOrder
		}
	}

	if err := s.fieldRepo.Update(ctx, field); err != nil {
		logger.Error("failed to update field", zap.Error(err))
		return nil, fmt.Errorf("更新字段失败: %w", err)
	}

	return s.toFieldResponse(field), nil
}

// DeleteField 删除字段定义
func (s *fieldService) DeleteField(ctx context.Context, projectKey string, fieldID uint64) error {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	field, err := s.fieldRepo.GetByID(ctx, fieldID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFieldNotFound
		}
		return fmt.Errorf("查询字段失败: %w", err)
	}

	// 不能删除系统字段
	if field.IsSystem {
		return ErrCannotDeleteSystem
	}

	// 验证字段属于该项目
	if field.ProjectID == nil || *field.ProjectID != project.ID {
		return ErrFieldNotFound
	}

	// 事务中级联清理关联数据
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 清理字段方案中的关联（硬删除，避免唯一键冲突）
		if err := tx.Unscoped().Where("field_id = ?", fieldID).Delete(&model.IssueTypeFieldScheme{}).Error; err != nil {
			return fmt.Errorf("清理字段方案失败: %w", err)
		}
		// 清理模板中的引用
		if err := tx.Unscoped().Where("field_id = ?", fieldID).Delete(&model.FieldSchemeTemplateItem{}).Error; err != nil {
			return fmt.Errorf("清理模板字段引用失败: %w", err)
		}
		// 清理字段值中的关联
		if err := tx.Where("field_id = ?", fieldID).Delete(&model.IssueFieldValue{}).Error; err != nil {
			return fmt.Errorf("清理字段值失败: %w", err)
		}
		// 删除字段定义本身
		if err := tx.Delete(&model.FieldDefinition{}, fieldID).Error; err != nil {
			return fmt.Errorf("删除字段失败: %w", err)
		}
		return nil
	})
	if err != nil {
		logger.Error("failed to delete field", zap.Error(err))
		return err
	}

	logger.Info("field deleted",
		zap.String("project_key", projectKey),
		zap.Uint64("field_id", fieldID),
	)

	return nil
}

// GetField 获取字段定义
func (s *fieldService) GetField(ctx context.Context, fieldID uint64) (*dto.FieldDefinitionResponse, error) {
	field, err := s.fieldRepo.GetByID(ctx, fieldID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFieldNotFound
		}
		return nil, fmt.Errorf("查询字段失败: %w", err)
	}

	return s.toFieldResponse(field), nil
}

// ListFields 获取项目可用的所有字段
func (s *fieldService) ListFields(ctx context.Context, projectKey string) ([]*dto.FieldDefinitionResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	fields, err := s.fieldRepo.ListAllAvailable(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询字段列表失败: %w", err)
	}

	responses := make([]*dto.FieldDefinitionResponse, len(fields))
	for i, field := range fields {
		responses[i] = s.toFieldResponse(field)
	}

	return responses, nil
}

// ============ 字段方案 ============

// GetFieldScheme 获取工单类型的字段方案
func (s *fieldService) GetFieldScheme(ctx context.Context, projectKey string, issueTypeID uint64) ([]*dto.FieldSchemeResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 验证工单类型存在
	_, err = s.issueTypeRepo.GetByID(ctx, issueTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueTypeNotFound
		}
		return nil, fmt.Errorf("查询工单类型失败: %w", err)
	}

	schemes, err := s.schemeRepo.ListByProjectAndType(ctx, project.ID, issueTypeID)
	if err != nil {
		return nil, fmt.Errorf("查询字段方案失败: %w", err)
	}

	responses := make([]*dto.FieldSchemeResponse, len(schemes))
	for i, scheme := range schemes {
		responses[i] = s.toSchemeResponse(ctx, scheme)
	}

	return responses, nil
}

// UpdateFieldScheme 更新工单类型的字段方案
func (s *fieldService) UpdateFieldScheme(ctx context.Context, projectKey string, issueTypeID uint64, req *dto.UpdateFieldSchemeRequest) ([]*dto.FieldSchemeResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 验证工单类型存在
	_, err = s.issueTypeRepo.GetByID(ctx, issueTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueTypeNotFound
		}
		return nil, fmt.Errorf("查询工单类型失败: %w", err)
	}

	// 验证所有 FieldID 是否存在
	if len(req.Items) > 0 {
		fieldIDs := make([]uint64, len(req.Items))
		for i, item := range req.Items {
			fieldIDs[i] = item.FieldID
		}
		existingFields, err := s.fieldRepo.ListByIDs(ctx, fieldIDs)
		if err != nil {
			return nil, fmt.Errorf("验证字段ID失败: %w", err)
		}
		existingFieldMap := make(map[uint64]bool, len(existingFields))
		for _, f := range existingFields {
			existingFieldMap[f.ID] = true
		}
		for _, id := range fieldIDs {
			if !existingFieldMap[id] {
				return nil, fmt.Errorf("字段ID %d 不存在", id)
			}
		}
	}

	// 使用事务更新
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := s.schemeRepo.WithTx(tx)

		// 删除旧的方案
		if err := txRepo.DeleteByProjectAndType(ctx, project.ID, issueTypeID); err != nil {
			return fmt.Errorf("删除旧方案失败: %w", err)
		}

		// 创建新的方案
		schemes := make([]*model.IssueTypeFieldScheme, len(req.Items))
		for i, f := range req.Items {
			schemes[i] = &model.IssueTypeFieldScheme{
				ProjectID:       project.ID,
				IssueTypeID:     issueTypeID,
				FieldID:         f.FieldID,
				IsRequired:      f.IsRequired,
				IsVisibleCreate: f.IsVisibleCreate,
				IsVisibleEdit:   f.IsVisibleEdit,
				IsVisibleDetail: f.IsVisibleDetail,
				SortOrder:       f.SortOrder,
				DefaultValue:    f.DefaultValue,
			}
		}

		if err := txRepo.BatchCreate(ctx, schemes); err != nil {
			return fmt.Errorf("创建新方案失败: %w", err)
		}

		return nil
	})

	if err != nil {
		logger.Error("failed to update field scheme", zap.Error(err))
		return nil, err
	}

	logger.Info("field scheme updated",
		zap.String("project_key", projectKey),
		zap.Uint64("issue_type_id", issueTypeID),
	)

	return s.GetFieldScheme(ctx, projectKey, issueTypeID)
}

// ============ 字段值 ============

// GetFieldValues 获取工单的自定义字段值
func (s *fieldService) GetFieldValues(ctx context.Context, issueID uint64) ([]*dto.FieldValueResponse, error) {
	values, err := s.valueRepo.ListByIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("查询字段值失败: %w", err)
	}

	// 批量获取字段定义，消除 N+1
	fieldIDs := make([]uint64, 0, len(values))
	for _, v := range values {
		fieldIDs = append(fieldIDs, v.FieldID)
	}
	fields, err := s.fieldRepo.ListByIDs(ctx, fieldIDs)
	if err != nil {
		return nil, fmt.Errorf("批量查询字段定义失败: %w", err)
	}
	fieldMap := make(map[uint64]*model.FieldDefinition, len(fields))
	for _, f := range fields {
		fieldMap[f.ID] = f
	}

	responses := make([]*dto.FieldValueResponse, 0, len(values))
	for _, v := range values {
		field, ok := fieldMap[v.FieldID]
		if !ok {
			continue
		}

		resp := &dto.FieldValueResponse{
			FieldID:   v.FieldID,
			FieldKey:  field.FieldKey,
			FieldName: field.FieldName,
			FieldType: field.FieldType,
		}

		// 根据字段类型获取值
		switch field.FieldType {
		case model.FieldTypeNumber:
			if v.ValueNumber != nil {
				resp.Value = *v.ValueNumber
				resp.DisplayValue = fmt.Sprintf("%v", *v.ValueNumber)
			}
		case model.FieldTypeDate:
			if v.ValueDate != nil {
				resp.Value = v.ValueDate.Format("2006-01-02")
				resp.DisplayValue = v.ValueDate.Format("2006-01-02")
			}
		case model.FieldTypeDateTime:
			if v.ValueDate != nil {
				resp.Value = v.ValueDate.Format("2006-01-02T15:04")
				resp.DisplayValue = v.ValueDate.Format("2006-01-02 15:04")
			}
		case model.FieldTypeMultiSelect, model.FieldTypeLabel, model.FieldTypeComponent, model.FieldTypeMultiUser:
			if v.ValueJSON != nil && *v.ValueJSON != "" {
				var arr []any
				if err := json.Unmarshal([]byte(*v.ValueJSON), &arr); err == nil {
					resp.Value = arr
					resp.DisplayValue = *v.ValueJSON
				}
			}
		case model.FieldTypeEpicLink:
			// Epic Link 特殊处理：查询关联的 Epic 信息
			if v.ValueNumber != nil {
				epicID := uint64(*v.ValueNumber)
				var epicIssue model.Issue
				if err := s.db.Where("id = ?", epicID).First(&epicIssue).Error; err == nil {
					resp.Value = map[string]any{
						"id":        epicIssue.ID,
						"issue_key": epicIssue.IssueKey,
						"title":     epicIssue.Title,
					}
					resp.DisplayValue = fmt.Sprintf("%s: %s", epicIssue.IssueKey, epicIssue.Title)
				} else {
					resp.Value = epicID
					resp.DisplayValue = fmt.Sprintf("%d", epicID)
				}
			}
		case model.FieldTypeURL:
			if v.ValueText != nil {
				resp.Value = *v.ValueText
				resp.DisplayValue = *v.ValueText
			}
		case model.FieldTypeCheckbox:
			if v.ValueNumber != nil {
				boolVal := *v.ValueNumber != 0
				resp.Value = boolVal
				if boolVal {
					resp.DisplayValue = "是"
				} else {
					resp.DisplayValue = "否"
				}
			}
		default:
			if v.ValueText != nil {
				resp.Value = *v.ValueText
				resp.DisplayValue = *v.ValueText
			}
		}

		responses = append(responses, resp)
	}

	return responses, nil
}

// builtinFieldKeys 内置字段（对应 Issue 表列，不应写入 EAV）
var builtinFieldKeys = map[string]bool{
	"description": true, "priority": true, "assignee": true,
	"planned_start_date": true, "planned_end_date": true, "epic_link": true,
}

// SaveFieldValues 保存工单的自定义字段值
func (s *fieldService) SaveFieldValues(ctx context.Context, issueID uint64, values []*dto.CustomFieldValue) error {
	if len(values) == 0 {
		return nil
	}

	// 批量查询字段定义
	fieldIDs := make([]uint64, 0, len(values))
	for _, v := range values {
		fieldIDs = append(fieldIDs, v.FieldID)
	}
	fields, err := s.fieldRepo.ListByIDs(ctx, fieldIDs)
	if err != nil {
		return fmt.Errorf("批量查询字段定义失败: %w", err)
	}
	fieldDefMap := make(map[uint64]*model.FieldDefinition, len(fields))
	for _, f := range fields {
		fieldDefMap[f.ID] = f
	}

	fieldValues := make([]*model.IssueFieldValue, 0, len(values))

	for _, v := range values {
		field, ok := fieldDefMap[v.FieldID]
		if !ok {
			return fmt.Errorf("字段ID %d 不存在", v.FieldID)
		}

		// 跳过内置字段，防止写入 EAV 表
		if builtinFieldKeys[field.FieldKey] {
			continue
		}

		fv := &model.IssueFieldValue{
			IssueID: issueID,
			FieldID: v.FieldID,
		}

		// 根据字段类型设置值
		switch field.FieldType {
		case model.FieldTypeNumber:
			if num, ok := v.Value.(float64); ok {
				fv.ValueNumber = &num
			}
		case model.FieldTypeDate:
			if dateStr, ok := v.Value.(string); ok {
				if t, err := time.Parse("2006-01-02", dateStr); err == nil {
					fv.ValueDate = &t
				}
			}
		case model.FieldTypeMultiSelect, model.FieldTypeLabel, model.FieldTypeComponent, model.FieldTypeMultiUser:
			switch val := v.Value.(type) {
			case []any:
				if jsonBytes, err := json.Marshal(val); err == nil {
					jsonStr := string(jsonBytes)
					fv.ValueJSON = &jsonStr
				}
			case string:
				// 前端可能发送 JSON 字符串格式的数组
				var arr []any
				if json.Unmarshal([]byte(val), &arr) == nil {
					fv.ValueJSON = &val
				}
			}
		case model.FieldTypeDateTime:
			// 支持多种时间格式，依次尝试解析
			if str, ok := v.Value.(string); ok && str != "" {
				for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
					if t, err := time.Parse(layout, str); err == nil {
						fv.ValueDate = &t
						break
					}
				}
			}
		case model.FieldTypeURL:
			// URL 宽松校验，仅要求非空（前端负责格式校验）
			if str, ok := v.Value.(string); ok && str != "" {
				fv.ValueText = &str
			}
		case model.FieldTypeCheckbox:
			// checkbox 存储为 0/1 数值
			num := parseCheckboxValue(v.Value)
			fv.ValueNumber = &num
		case model.FieldTypeEpicLink:
			// Epic Link 保存为 number 类型（Issue ID）
			switch val := v.Value.(type) {
			case float64:
				fv.ValueNumber = &val
			case int:
				num := float64(val)
				fv.ValueNumber = &num
			case string:
				// 如果是字符串，尝试转换为数字
				if num, err := strconv.ParseFloat(val, 64); err == nil {
					fv.ValueNumber = &num
				}
			}
		default:
			if str, ok := v.Value.(string); ok {
				fv.ValueText = &str
			} else if v.Value != nil {
				str := fmt.Sprintf("%v", v.Value)
				fv.ValueText = &str
			}
		}

		fieldValues = append(fieldValues, fv)
	}

	if err := s.valueRepo.BatchUpsert(ctx, fieldValues); err != nil {
		logger.Error("failed to save field values", zap.Error(err))
		return fmt.Errorf("保存字段值失败: %w", err)
	}

	return nil
}

// CustomFieldValueInput 用于避免循环依赖的输入类型
type CustomFieldValueInput struct {
	FieldID uint64
	Value   any
}

// SaveIssueFieldValues 保存工单字段值（满足 issue_service.FieldValueSaver 接口）
func (s *fieldService) SaveIssueFieldValues(ctx context.Context, issueID uint64, values []CustomFieldValueInput) error {
	// 转换为内部 DTO 类型
	dtoValues := make([]*dto.CustomFieldValue, len(values))
	for i, v := range values {
		dtoValues[i] = &dto.CustomFieldValue{
			FieldID: v.FieldID,
			Value:   v.Value,
		}
	}
	return s.SaveFieldValues(ctx, issueID, dtoValues)
}

// GetIssueIDsByEpicLink 获取链接到指定 Epic 的所有 Issue IDs
func (s *fieldService) GetIssueIDsByEpicLink(ctx context.Context, epicID uint64) ([]uint64, error) {
	// 查询 epic_link 字段的定义
	var epicLinkField model.FieldDefinition
	if err := s.db.Where("field_key = ?", "epic_link").First(&epicLinkField).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []uint64{}, nil
		}
		return nil, fmt.Errorf("查询 epic_link 字段失败: %w", err)
	}

	// 查询所有 epic_link 字段值指向该 Epic 的 Issues
	var fieldValues []model.IssueFieldValue
	if err := s.db.Where("field_id = ? AND value_number = ?", epicLinkField.ID, float64(epicID)).
		Find(&fieldValues).Error; err != nil {
		logger.Error("failed to query epic link field values", zap.Error(err))
		return nil, fmt.Errorf("查询 Epic 关联工单失败: %w", err)
	}

	issueIDs := make([]uint64, len(fieldValues))
	for i, fv := range fieldValues {
		issueIDs[i] = fv.IssueID
	}

	return issueIDs, nil
}

// ============ 版本管理 ============

// CreateVersion 创建项目版本
func (s *fieldService) CreateVersion(ctx context.Context, projectKey string, req *dto.CreateVersionRequest) (*dto.VersionResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查版本名称是否已存在
	_, err = s.versionRepo.GetByName(ctx, project.ID, req.Name)
	if err == nil {
		return nil, ErrVersionNameExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查版本名称失败: %w", err)
	}

	version := &model.ProjectVersion{
		ProjectID:   project.ID,
		Name:        req.Name,
		Description: req.Description,
		Status:      model.VersionStatusUnreleased,
	}

	if req.ReleaseDate != nil {
		if t, err := time.Parse("2006-01-02", *req.ReleaseDate); err == nil {
			version.ReleaseDate = &t
		}
	}

	if err := s.versionRepo.Create(ctx, version); err != nil {
		logger.Error("failed to create version", zap.Error(err))
		return nil, fmt.Errorf("创建版本失败: %w", err)
	}

	logger.Info("version created",
		zap.String("project_key", projectKey),
		zap.String("version_name", req.Name),
	)

	return s.toVersionResponse(version), nil
}

// UpdateVersion 更新项目版本
func (s *fieldService) UpdateVersion(ctx context.Context, projectKey string, versionID uint64, req *dto.UpdateVersionRequest) (*dto.VersionResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	version, err := s.versionRepo.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVersionNotFound
		}
		return nil, fmt.Errorf("查询版本失败: %w", err)
	}

	// 验证版本属于该项目
	if version.ProjectID != project.ID {
		return nil, ErrVersionNotFound
	}

	if req.Name != nil {
		// 检查新名称是否已存在
		existing, err := s.versionRepo.GetByName(ctx, project.ID, *req.Name)
		if err == nil && existing.ID != versionID {
			return nil, ErrVersionNameExists
		}
		version.Name = *req.Name
	}
	if req.Description != nil {
		version.Description = *req.Description
	}
	if req.ReleaseDate != nil {
		if t, err := time.Parse("2006-01-02", *req.ReleaseDate); err == nil {
			version.ReleaseDate = &t
		}
	}
	if req.Status != nil {
		version.Status = *req.Status
	}

	if err := s.versionRepo.Update(ctx, version); err != nil {
		logger.Error("failed to update version", zap.Error(err))
		return nil, fmt.Errorf("更新版本失败: %w", err)
	}

	return s.toVersionResponse(version), nil
}

// DeleteVersion 删除项目版本
func (s *fieldService) DeleteVersion(ctx context.Context, projectKey string, versionID uint64) error {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	version, err := s.versionRepo.GetByID(ctx, versionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrVersionNotFound
		}
		return fmt.Errorf("查询版本失败: %w", err)
	}

	// 验证版本属于该项目
	if version.ProjectID != project.ID {
		return ErrVersionNotFound
	}

	if err := s.versionRepo.Delete(ctx, versionID); err != nil {
		logger.Error("failed to delete version", zap.Error(err))
		return fmt.Errorf("删除版本失败: %w", err)
	}

	logger.Info("version deleted",
		zap.String("project_key", projectKey),
		zap.Uint64("version_id", versionID),
	)

	return nil
}

// ListVersions 获取项目版本列表
func (s *fieldService) ListVersions(ctx context.Context, projectKey string) ([]*dto.VersionResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	versions, err := s.versionRepo.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询版本列表失败: %w", err)
	}

	responses := make([]*dto.VersionResponse, len(versions))
	for i, v := range versions {
		responses[i] = s.toVersionResponse(v)
	}

	return responses, nil
}

// ============ 组件管理 ============

// CreateComponent 创建项目组件
func (s *fieldService) CreateComponent(ctx context.Context, projectKey string, req *dto.CreateComponentRequest) (*dto.ComponentResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查组件名称是否已存在
	_, err = s.componentRepo.GetByName(ctx, project.ID, req.Name)
	if err == nil {
		return nil, ErrComponentNameExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查组件名称失败: %w", err)
	}

	component := &model.ProjectComponent{
		ProjectID:   project.ID,
		Name:        req.Name,
		Description: req.Description,
		LeadUserID:  req.LeadUserID,
	}

	if err := s.componentRepo.Create(ctx, component); err != nil {
		logger.Error("failed to create component", zap.Error(err))
		return nil, fmt.Errorf("创建组件失败: %w", err)
	}

	logger.Info("component created",
		zap.String("project_key", projectKey),
		zap.String("component_name", req.Name),
	)

	return s.toComponentResponse(ctx, component), nil
}

// UpdateComponent 更新项目组件
func (s *fieldService) UpdateComponent(ctx context.Context, projectKey string, componentID uint64, req *dto.UpdateComponentRequest) (*dto.ComponentResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	component, err := s.componentRepo.GetByID(ctx, componentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrComponentNotFound
		}
		return nil, fmt.Errorf("查询组件失败: %w", err)
	}

	// 验证组件属于该项目
	if component.ProjectID != project.ID {
		return nil, ErrComponentNotFound
	}

	if req.Name != nil {
		// 检查新名称是否已存在
		existing, err := s.componentRepo.GetByName(ctx, project.ID, *req.Name)
		if err == nil && existing.ID != componentID {
			return nil, ErrComponentNameExists
		}
		component.Name = *req.Name
	}
	if req.Description != nil {
		component.Description = *req.Description
	}
	if req.LeadUserID != nil {
		component.LeadUserID = req.LeadUserID
	}

	if err := s.componentRepo.Update(ctx, component); err != nil {
		logger.Error("failed to update component", zap.Error(err))
		return nil, fmt.Errorf("更新组件失败: %w", err)
	}

	return s.toComponentResponse(ctx, component), nil
}

// DeleteComponent 删除项目组件
func (s *fieldService) DeleteComponent(ctx context.Context, projectKey string, componentID uint64) error {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	component, err := s.componentRepo.GetByID(ctx, componentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrComponentNotFound
		}
		return fmt.Errorf("查询组件失败: %w", err)
	}

	// 验证组件属于该项目
	if component.ProjectID != project.ID {
		return ErrComponentNotFound
	}

	if err := s.componentRepo.Delete(ctx, componentID); err != nil {
		logger.Error("failed to delete component", zap.Error(err))
		return fmt.Errorf("删除组件失败: %w", err)
	}

	logger.Info("component deleted",
		zap.String("project_key", projectKey),
		zap.Uint64("component_id", componentID),
	)

	return nil
}

// ListComponents 获取项目组件列表
func (s *fieldService) ListComponents(ctx context.Context, projectKey string) ([]*dto.ComponentResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	components, err := s.componentRepo.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询组件列表失败: %w", err)
	}

	responses := make([]*dto.ComponentResponse, len(components))
	for i, c := range components {
		responses[i] = s.toComponentResponse(ctx, c)
	}

	return responses, nil
}

// ============ 标签管理 ============

// CreateLabel 创建标签
func (s *fieldService) CreateLabel(ctx context.Context, projectKey string, req *dto.CreateLabelRequest) (*dto.LabelResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查标签名称是否已存在
	_, err = s.labelRepo.GetByName(ctx, project.ID, req.Name)
	if err == nil {
		return nil, ErrLabelNameExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查标签名称失败: %w", err)
	}

	label := &model.IssueLabel{
		ProjectID:   project.ID,
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	}

	if err := s.labelRepo.Create(ctx, label); err != nil {
		logger.Error("failed to create label", zap.Error(err))
		return nil, fmt.Errorf("创建标签失败: %w", err)
	}

	logger.Info("label created",
		zap.String("project_key", projectKey),
		zap.String("label_name", req.Name),
	)

	return s.toLabelResponse(label), nil
}

// UpdateLabel 更新标签
func (s *fieldService) UpdateLabel(ctx context.Context, projectKey string, labelID uint64, req *dto.UpdateLabelRequest) (*dto.LabelResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	label, err := s.labelRepo.GetByID(ctx, labelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLabelNotFound
		}
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}

	// 验证标签属于该项目
	if label.ProjectID != project.ID {
		return nil, ErrLabelNotFound
	}

	if req.Name != nil {
		// 检查新名称是否已存在
		existing, err := s.labelRepo.GetByName(ctx, project.ID, *req.Name)
		if err == nil && existing.ID != labelID {
			return nil, ErrLabelNameExists
		}
		label.Name = *req.Name
	}
	if req.Color != nil {
		label.Color = *req.Color
	}
	if req.Description != nil {
		label.Description = *req.Description
	}

	if err := s.labelRepo.Update(ctx, label); err != nil {
		logger.Error("failed to update label", zap.Error(err))
		return nil, fmt.Errorf("更新标签失败: %w", err)
	}

	return s.toLabelResponse(label), nil
}

// DeleteLabel 删除标签
func (s *fieldService) DeleteLabel(ctx context.Context, projectKey string, labelID uint64) error {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	label, err := s.labelRepo.GetByID(ctx, labelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLabelNotFound
		}
		return fmt.Errorf("查询标签失败: %w", err)
	}

	// 验证标签属于该项目
	if label.ProjectID != project.ID {
		return ErrLabelNotFound
	}

	if err := s.labelRepo.Delete(ctx, labelID); err != nil {
		logger.Error("failed to delete label", zap.Error(err))
		return fmt.Errorf("删除标签失败: %w", err)
	}

	logger.Info("label deleted",
		zap.String("project_key", projectKey),
		zap.Uint64("label_id", labelID),
	)

	return nil
}

// ListLabels 获取项目标签列表
func (s *fieldService) ListLabels(ctx context.Context, projectKey string) ([]*dto.LabelResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	labels, err := s.labelRepo.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询标签列表失败: %w", err)
	}

	responses := make([]*dto.LabelResponse, len(labels))
	for i, l := range labels {
		responses[i] = s.toLabelResponse(l)
	}

	return responses, nil
}

// ============ 转换方法 ============

func (s *fieldService) toFieldResponse(field *model.FieldDefinition) *dto.FieldDefinitionResponse {
	return &dto.FieldDefinitionResponse{
		ID:           field.ID,
		ProjectID:    field.ProjectID,
		FieldKey:     field.FieldKey,
		FieldName:    field.FieldName,
		FieldType:    field.FieldType,
		Description:  field.Description,
		IsSystem:     field.IsSystem,
		IsActive:     field.IsActive,
		Options:      field.Options,
		Validation:   field.Validation,
		DefaultValue: field.DefaultValue,
		SortOrder:    field.SortOrder,
		CreatedAt:    field.CreatedAt,
		UpdatedAt:    field.UpdatedAt,
	}
}

func (s *fieldService) toSchemeResponse(ctx context.Context, scheme *model.IssueTypeFieldScheme) *dto.FieldSchemeResponse {
	resp := &dto.FieldSchemeResponse{
		ID:              scheme.ID,
		ProjectID:       scheme.ProjectID,
		IssueTypeID:     scheme.IssueTypeID,
		FieldID:         scheme.FieldID,
		IsRequired:      scheme.IsRequired,
		IsVisibleCreate: scheme.IsVisibleCreate,
		IsVisibleEdit:   scheme.IsVisibleEdit,
		IsVisibleDetail: scheme.IsVisibleDetail,
		SortOrder:       scheme.SortOrder,
		DefaultValue:    scheme.DefaultValue,
		CreatedAt:       scheme.CreatedAt,
		UpdatedAt:       scheme.UpdatedAt,
	}

	// 获取字段定义
	if field, err := s.fieldRepo.GetByID(ctx, scheme.FieldID); err == nil {
		resp.Field = s.toFieldResponse(field)
	}

	return resp
}

func (s *fieldService) toVersionResponse(version *model.ProjectVersion) *dto.VersionResponse {
	return &dto.VersionResponse{
		ID:          version.ID,
		ProjectID:   version.ProjectID,
		Name:        version.Name,
		Description: version.Description,
		ReleaseDate: version.ReleaseDate,
		Status:      version.Status,
		SortOrder:   version.SortOrder,
		CreatedAt:   version.CreatedAt,
		UpdatedAt:   version.UpdatedAt,
	}
}

func (s *fieldService) toComponentResponse(ctx context.Context, component *model.ProjectComponent) *dto.ComponentResponse {
	resp := &dto.ComponentResponse{
		ID:          component.ID,
		ProjectID:   component.ProjectID,
		Name:        component.Name,
		Description: component.Description,
		LeadUserID:  component.LeadUserID,
		CreatedAt:   component.CreatedAt,
		UpdatedAt:   component.UpdatedAt,
	}

	// 获取负责人信息
	if component.LeadUserID != nil {
		if user, err := s.userRepo.GetByID(ctx, *component.LeadUserID); err == nil {
			resp.LeadUser = &dto.UserBrief{
				ID:          user.ID,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				AvatarURL:   user.AvatarURL,
			}
		}
	}

	return resp
}

func (s *fieldService) toLabelResponse(label *model.IssueLabel) *dto.LabelResponse {
	return &dto.LabelResponse{
		ID:          label.ID,
		ProjectID:   label.ProjectID,
		Name:        label.Name,
		Color:       label.Color,
		Description: label.Description,
		CreatedAt:   label.CreatedAt,
		UpdatedAt:   label.UpdatedAt,
	}
}

// ============ 全局字段管理 ============

// ListGlobalFields 获取全局字段列表
func (s *fieldService) ListGlobalFields(ctx context.Context) ([]*dto.FieldDefinitionResponse, error) {
	fields, err := s.fieldRepo.ListGlobalFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询全局字段列表失败: %w", err)
	}

	responses := make([]*dto.FieldDefinitionResponse, len(fields))
	for i, field := range fields {
		responses[i] = s.toFieldResponse(field)
	}

	return responses, nil
}

// CreateGlobalField 创建全局自定义字段
func (s *fieldService) CreateGlobalField(ctx context.Context, req *dto.CreateFieldRequest) (*dto.FieldDefinitionResponse, error) {
	// 检查字段Key是否已存在
	_, err := s.fieldRepo.GetByKey(ctx, nil, req.FieldKey)
	if err == nil {
		return nil, ErrFieldKeyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查字段Key失败: %w", err)
	}

	field := &model.FieldDefinition{
		ProjectID:    nil,
		FieldKey:     req.FieldKey,
		FieldName:    req.FieldName,
		FieldType:    req.FieldType,
		Description:  req.Description,
		IsSystem:     false,
		IsActive:     true,
		Options:      req.Options,
		Validation:   req.Validation,
		DefaultValue: req.DefaultValue,
		SortOrder:    100,
	}

	// MySQL JSON 类型不接受空字符串
	if field.Options == "" {
		field.Options = "{}"
	}
	if field.Validation == "" {
		field.Validation = "{}"
	}

	if err := s.fieldRepo.Create(ctx, field); err != nil {
		logger.Error("failed to create global field", zap.Error(err))
		return nil, fmt.Errorf("创建全局字段失败: %w", err)
	}

	logger.Info("global field created", zap.String("field_key", req.FieldKey))

	return s.toFieldResponse(field), nil
}

// UpdateGlobalField 更新全局字段
func (s *fieldService) UpdateGlobalField(ctx context.Context, fieldID uint64, req *dto.UpdateFieldRequest) (*dto.FieldDefinitionResponse, error) {
	field, err := s.fieldRepo.GetByID(ctx, fieldID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFieldNotFound
		}
		return nil, fmt.Errorf("查询字段失败: %w", err)
	}

	// 只能操作全局字段
	if field.ProjectID != nil {
		return nil, ErrFieldNotFound
	}

	// 系统字段只能修改部分属性
	if field.IsSystem {
		if req.IsActive != nil {
			field.IsActive = *req.IsActive
		}
		if req.SortOrder != nil {
			field.SortOrder = *req.SortOrder
		}
	} else {
		if req.FieldName != nil {
			field.FieldName = *req.FieldName
		}
		if req.Description != nil {
			field.Description = *req.Description
		}
		if req.Options != nil {
			field.Options = *req.Options
		}
		if req.Validation != nil {
			field.Validation = *req.Validation
		}
		if req.DefaultValue != nil {
			field.DefaultValue = *req.DefaultValue
		}
		if req.IsActive != nil {
			field.IsActive = *req.IsActive
		}
		if req.SortOrder != nil {
			field.SortOrder = *req.SortOrder
		}
	}

	if err := s.fieldRepo.Update(ctx, field); err != nil {
		logger.Error("failed to update global field", zap.Error(err))
		return nil, fmt.Errorf("更新全局字段失败: %w", err)
	}

	return s.toFieldResponse(field), nil
}

// DeleteGlobalField 删除全局自定义字段
func (s *fieldService) DeleteGlobalField(ctx context.Context, fieldID uint64) error {
	field, err := s.fieldRepo.GetByID(ctx, fieldID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFieldNotFound
		}
		return fmt.Errorf("查询字段失败: %w", err)
	}

	// 不能删除系统字段
	if field.IsSystem {
		return ErrCannotDeleteSystem
	}

	// 只能操作全局字段
	if field.ProjectID != nil {
		return ErrFieldNotFound
	}

	// 事务中级联清理关联数据
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("field_id = ?", fieldID).Delete(&model.IssueTypeFieldScheme{}).Error; err != nil {
			return fmt.Errorf("清理字段方案失败: %w", err)
		}
		if err := tx.Unscoped().Where("field_id = ?", fieldID).Delete(&model.FieldSchemeTemplateItem{}).Error; err != nil {
			return fmt.Errorf("清理模板字段引用失败: %w", err)
		}
		if err := tx.Where("field_id = ?", fieldID).Delete(&model.IssueFieldValue{}).Error; err != nil {
			return fmt.Errorf("清理字段值失败: %w", err)
		}
		if err := tx.Delete(&model.FieldDefinition{}, fieldID).Error; err != nil {
			return fmt.Errorf("删除字段失败: %w", err)
		}
		return nil
	})
	if err != nil {
		logger.Error("failed to delete global field", zap.Error(err))
		return err
	}

	logger.Info("global field deleted", zap.Uint64("field_id", fieldID))
	return nil
}

// GetFieldUsage 获取字段使用情况
func (s *fieldService) GetFieldUsage(ctx context.Context, fieldID uint64) (*dto.FieldUsageResponse, error) {
	_, err := s.fieldRepo.GetByID(ctx, fieldID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFieldNotFound
		}
		return nil, fmt.Errorf("查询字段失败: %w", err)
	}

	var schemeCount int64
	s.db.Model(&model.IssueTypeFieldScheme{}).Where("field_id = ?", fieldID).Count(&schemeCount)

	var valueCount int64
	s.db.Model(&model.IssueFieldValue{}).Where("field_id = ?", fieldID).Count(&valueCount)

	templateCount, _ := s.templateRepo.CountByFieldID(ctx, fieldID)

	return &dto.FieldUsageResponse{
		SchemeCount:   schemeCount,
		ValueCount:    valueCount,
		TemplateCount: templateCount,
	}, nil
}

// ============ 方案模板 ============

// ListTemplates 获取所有方案模板
func (s *fieldService) ListTemplates(ctx context.Context) ([]*dto.TemplateResponse, error) {
	templates, err := s.templateRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询模板列表失败: %w", err)
	}

	responses := make([]*dto.TemplateResponse, len(templates))
	for i, tpl := range templates {
		items, _ := s.templateRepo.ListItemsByTemplate(ctx, tpl.ID)
		responses[i] = &dto.TemplateResponse{
			ID:          tpl.ID,
			Name:        tpl.Name,
			Description: tpl.Description,
			CreatedBy:   tpl.CreatedBy,
			IsActive:    tpl.IsActive,
			ItemCount:   len(items),
			CreatedAt:   tpl.CreatedAt,
			UpdatedAt:   tpl.UpdatedAt,
		}
	}

	return responses, nil
}

// CreateTemplate 创建方案模板
func (s *fieldService) CreateTemplate(ctx context.Context, userID uint64, req *dto.CreateTemplateRequest) (*dto.TemplateResponse, error) {
	// 检查名称唯一
	_, err := s.templateRepo.GetByName(ctx, req.Name)
	if err == nil {
		return nil, ErrTemplateNameExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查模板名称失败: %w", err)
	}

	tpl := &model.FieldSchemeTemplate{
		Name:        req.Name,
		Description: req.Description,
		CreatedBy:   userID,
		IsActive:    true,
	}

	if err := s.templateRepo.Create(ctx, tpl); err != nil {
		logger.Error("failed to create template", zap.Error(err))
		return nil, fmt.Errorf("创建模板失败: %w", err)
	}

	logger.Info("template created", zap.String("name", req.Name))

	return &dto.TemplateResponse{
		ID:          tpl.ID,
		Name:        tpl.Name,
		Description: tpl.Description,
		CreatedBy:   tpl.CreatedBy,
		IsActive:    tpl.IsActive,
		ItemCount:   0,
		CreatedAt:   tpl.CreatedAt,
		UpdatedAt:   tpl.UpdatedAt,
	}, nil
}

// GetTemplate 获取模板详情（含字段项）
func (s *fieldService) GetTemplate(ctx context.Context, templateID uint64) (*dto.TemplateDetailResponse, error) {
	tpl, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}

	items, err := s.templateRepo.ListItemsByTemplate(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("查询模板字段项失败: %w", err)
	}

	// 批量获取字段定义
	fieldIDs := make([]uint64, len(items))
	for i, item := range items {
		fieldIDs[i] = item.FieldID
	}
	fields, _ := s.fieldRepo.ListByIDs(ctx, fieldIDs)
	fieldMap := make(map[uint64]*model.FieldDefinition, len(fields))
	for _, f := range fields {
		fieldMap[f.ID] = f
	}

	itemResponses := make([]*dto.TemplateItemResponse, len(items))
	for i, item := range items {
		ir := &dto.TemplateItemResponse{
			ID:              item.ID,
			TemplateID:      item.TemplateID,
			FieldID:         item.FieldID,
			IsRequired:      item.IsRequired,
			IsVisibleCreate: item.IsVisibleCreate,
			IsVisibleEdit:   item.IsVisibleEdit,
			IsVisibleDetail: item.IsVisibleDetail,
			SortOrder:       item.SortOrder,
			DefaultValue:    item.DefaultValue,
		}
		if f, ok := fieldMap[item.FieldID]; ok {
			ir.Field = s.toFieldResponse(f)
		}
		itemResponses[i] = ir
	}

	return &dto.TemplateDetailResponse{
		ID:          tpl.ID,
		Name:        tpl.Name,
		Description: tpl.Description,
		CreatedBy:   tpl.CreatedBy,
		IsActive:    tpl.IsActive,
		Items:       itemResponses,
		CreatedAt:   tpl.CreatedAt,
		UpdatedAt:   tpl.UpdatedAt,
	}, nil
}

// UpdateTemplate 更新模板基本信息
func (s *fieldService) UpdateTemplate(ctx context.Context, templateID uint64, req *dto.UpdateTemplateRequest) (*dto.TemplateResponse, error) {
	tpl, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}

	if req.Name != nil {
		// 检查新名称唯一
		existing, err := s.templateRepo.GetByName(ctx, *req.Name)
		if err == nil && existing.ID != templateID {
			return nil, ErrTemplateNameExists
		}
		tpl.Name = *req.Name
	}
	if req.Description != nil {
		tpl.Description = *req.Description
	}
	if req.IsActive != nil {
		tpl.IsActive = *req.IsActive
	}

	if err := s.templateRepo.Update(ctx, tpl); err != nil {
		logger.Error("failed to update template", zap.Error(err))
		return nil, fmt.Errorf("更新模板失败: %w", err)
	}

	items, _ := s.templateRepo.ListItemsByTemplate(ctx, templateID)

	return &dto.TemplateResponse{
		ID:          tpl.ID,
		Name:        tpl.Name,
		Description: tpl.Description,
		CreatedBy:   tpl.CreatedBy,
		IsActive:    tpl.IsActive,
		ItemCount:   len(items),
		CreatedAt:   tpl.CreatedAt,
		UpdatedAt:   tpl.UpdatedAt,
	}, nil
}

// DeleteTemplate 删除模板
func (s *fieldService) DeleteTemplate(ctx context.Context, templateID uint64) error {
	_, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTemplateNotFound
		}
		return fmt.Errorf("查询模板失败: %w", err)
	}

	if err := s.templateRepo.Delete(ctx, templateID); err != nil {
		logger.Error("failed to delete template", zap.Error(err))
		return fmt.Errorf("删除模板失败: %w", err)
	}

	logger.Info("template deleted", zap.Uint64("template_id", templateID))
	return nil
}

// UpdateTemplateItems 更新模板字段项（全量替换）
func (s *fieldService) UpdateTemplateItems(ctx context.Context, templateID uint64, req *dto.UpdateTemplateItemsRequest) error {
	_, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTemplateNotFound
		}
		return fmt.Errorf("查询模板失败: %w", err)
	}

	// 验证所有 FieldID 存在
	if len(req.Items) > 0 {
		fieldIDs := make([]uint64, len(req.Items))
		for i, item := range req.Items {
			fieldIDs[i] = item.FieldID
		}
		existingFields, err := s.fieldRepo.ListByIDs(ctx, fieldIDs)
		if err != nil {
			return fmt.Errorf("验证字段ID失败: %w", err)
		}
		existingFieldMap := make(map[uint64]bool, len(existingFields))
		for _, f := range existingFields {
			existingFieldMap[f.ID] = true
		}
		for _, id := range fieldIDs {
			if !existingFieldMap[id] {
				return fmt.Errorf("字段ID %d 不存在", id)
			}
		}
	}

	// 事务中替换
	err = s.db.Transaction(func(tx *gorm.DB) error {
		txRepo := s.templateRepo.WithTx(tx)

		if err := txRepo.DeleteItemsByTemplate(ctx, templateID); err != nil {
			return fmt.Errorf("删除旧模板项失败: %w", err)
		}

		items := make([]*model.FieldSchemeTemplateItem, len(req.Items))
		for i, item := range req.Items {
			items[i] = &model.FieldSchemeTemplateItem{
				TemplateID:      templateID,
				FieldID:         item.FieldID,
				IsRequired:      item.IsRequired,
				IsVisibleCreate: item.IsVisibleCreate,
				IsVisibleEdit:   item.IsVisibleEdit,
				IsVisibleDetail: item.IsVisibleDetail,
				SortOrder:       item.SortOrder,
				DefaultValue:    item.DefaultValue,
			}
		}

		if err := txRepo.BatchCreateItems(ctx, items); err != nil {
			return fmt.Errorf("创建新模板项失败: %w", err)
		}

		return nil
	})

	if err != nil {
		logger.Error("failed to update template items", zap.Error(err))
		return err
	}

	logger.Info("template items updated", zap.Uint64("template_id", templateID))
	return nil
}

// ============ 套用模板 ============

// ApplyTemplate 将模板套用到项目的工单类型
func (s *fieldService) ApplyTemplate(ctx context.Context, projectKey string, issueTypeID uint64, req *dto.ApplyTemplateRequest) ([]*dto.FieldSchemeResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 验证工单类型
	_, err = s.issueTypeRepo.GetByID(ctx, issueTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueTypeNotFound
		}
		return nil, fmt.Errorf("查询工单类型失败: %w", err)
	}

	// 获取模板和模板项
	tpl, err := s.templateRepo.GetByID(ctx, req.TemplateID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, fmt.Errorf("查询模板失败: %w", err)
	}
	_ = tpl

	items, err := s.templateRepo.ListItemsByTemplate(ctx, req.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("查询模板项失败: %w", err)
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		txSchemeRepo := s.schemeRepo.WithTx(tx)

		if req.Mode == "replace" {
			// 替换模式：删除旧方案
			if err := txSchemeRepo.DeleteByProjectAndType(ctx, project.ID, issueTypeID); err != nil {
				return fmt.Errorf("删除旧方案失败: %w", err)
			}
		}

		// 合并模式下获取已有字段ID
		existingFieldIDs := make(map[uint64]bool)
		if req.Mode == "merge" {
			existing, err := txSchemeRepo.ListByProjectAndType(ctx, project.ID, issueTypeID)
			if err != nil {
				return fmt.Errorf("查询已有方案失败: %w", err)
			}
			for _, s := range existing {
				existingFieldIDs[s.FieldID] = true
			}
		}

		// 将模板项复制为方案记录
		schemes := make([]*model.IssueTypeFieldScheme, 0, len(items))
		for _, item := range items {
			if req.Mode == "merge" && existingFieldIDs[item.FieldID] {
				continue // 合并模式下跳过已存在的字段
			}
			schemes = append(schemes, &model.IssueTypeFieldScheme{
				ProjectID:       project.ID,
				IssueTypeID:     issueTypeID,
				FieldID:         item.FieldID,
				IsRequired:      item.IsRequired,
				IsVisibleCreate: item.IsVisibleCreate,
				IsVisibleEdit:   item.IsVisibleEdit,
				IsVisibleDetail: item.IsVisibleDetail,
				SortOrder:       item.SortOrder,
				DefaultValue:    item.DefaultValue,
			})
		}

		if len(schemes) > 0 {
			if err := txSchemeRepo.BatchCreate(ctx, schemes); err != nil {
				return fmt.Errorf("创建方案失败: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		logger.Error("failed to apply template", zap.Error(err))
		return nil, err
	}

	logger.Info("template applied",
		zap.String("project_key", projectKey),
		zap.Uint64("issue_type_id", issueTypeID),
		zap.Uint64("template_id", req.TemplateID),
		zap.String("mode", req.Mode),
	)

	return s.GetFieldScheme(ctx, projectKey, issueTypeID)
}

// parseCheckboxValue 把任意输入解析为 0 / 1，容忍 bool / float64 / string 三种前端类型
func parseCheckboxValue(v any) float64 {
	switch val := v.(type) {
	case bool:
		if val {
			return 1
		}
	case float64:
		if val != 0 {
			return 1
		}
	case string:
		if val == "true" || val == "1" {
			return 1
		}
	}
	return 0
}
