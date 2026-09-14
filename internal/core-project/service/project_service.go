// Package service 提供项目业务逻辑层
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-project/dto"
	"github.com/kerbos/ticketdesk/internal/core-project/repository"
	userRepo "github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/sequence"
)

// 业务错误定义
var (
	ErrProjectNotFound        = errors.New("workflow.project_not_found")
	ErrProjectKeyExists       = errors.New("project.key_exists")
	ErrMemberNotFound         = errors.New("project.member_not_found")
	ErrMemberAlreadyExists    = errors.New("project.member_exists")
	ErrCannotRemoveOwner      = errors.New("project.cannot_remove_owner")
	ErrIssueTypeNotFound      = errors.New("project.issue_type_not_found")
	ErrNoPermission           = errors.New("project.no_permission")
	ErrRoleNotFound           = errors.New("project.role_not_found")
	ErrRoleKeyExists          = errors.New("project.role_key_exists")
	ErrCannotDeleteSystemRole = errors.New("project.role_system")
	ErrRoleMemberExists       = errors.New("project.role_member_exists")
	ErrRoleMemberNotFound     = errors.New("project.role_member_not_found")
)

// ProjectService 项目服务接口
type ProjectService interface {
	CreateProject(ctx context.Context, req *dto.CreateProjectRequest, creatorID uint64) (*dto.ProjectResponse, error)
	GetProject(ctx context.Context, key string) (*dto.ProjectResponse, error)
	GetProjectByID(ctx context.Context, id uint64) (*dto.ProjectResponse, error)
	UpdateProject(ctx context.Context, key string, req *dto.UpdateProjectRequest) (*dto.ProjectResponse, error)
	DeleteProject(ctx context.Context, key string) error
	ListProjects(ctx context.Context, req *dto.ListProjectsRequest, userID uint64, isAdmin bool) ([]*dto.ProjectResponse, int64, error)

	// 成员管理
	AddMember(ctx context.Context, projectKey string, req *dto.AddMemberRequest) (*dto.ProjectMemberResponse, error)
	UpdateMember(ctx context.Context, projectKey string, userID uint64, req *dto.UpdateMemberRequest) (*dto.ProjectMemberResponse, error)
	RemoveMember(ctx context.Context, projectKey string, userID uint64) error
	ListMembers(ctx context.Context, projectKey string) ([]*dto.ProjectMemberResponse, error)
	CheckPermission(ctx context.Context, projectKey string, userID uint64, requiredRoles ...string) (bool, error)

	// 工单类型管理
	CreateIssueType(ctx context.Context, projectKey string, req *dto.CreateIssueTypeRequest) (*dto.IssueTypeResponse, error)
	UpdateIssueType(ctx context.Context, id uint64, req *dto.UpdateIssueTypeRequest) (*dto.IssueTypeResponse, error)
	DeleteIssueType(ctx context.Context, id uint64) error
	ListIssueTypes(ctx context.Context, projectKey string) ([]*dto.IssueTypeResponse, error)
	ListAllIssueTypes(ctx context.Context) ([]*dto.IssueTypeResponse, error)

	// 项目角色管理
	CreateRole(ctx context.Context, projectKey string, req *dto.CreateProjectRoleRequest) (*dto.ProjectRoleResponse, error)
	UpdateRole(ctx context.Context, projectKey string, roleID uint64, req *dto.UpdateProjectRoleRequest) (*dto.ProjectRoleResponse, error)
	DeleteRole(ctx context.Context, projectKey string, roleID uint64) error
	ListRoles(ctx context.Context, projectKey string) ([]*dto.ProjectRoleResponse, error)
	AddRoleMember(ctx context.Context, projectKey string, roleID uint64, req *dto.AddRoleMemberRequest) (*dto.ProjectRoleMemberResponse, error)
	RemoveRoleMember(ctx context.Context, projectKey string, roleID, userID uint64) error
	ListRoleMembers(ctx context.Context, projectKey string, roleID uint64) ([]*dto.ProjectRoleMemberResponse, error)
	GetUserRoles(ctx context.Context, projectKey string, userID uint64) ([]*dto.ProjectRoleResponse, error)
	// GetMyPermissions 返回用户在项目中的完整权限集，供前端决定显示哪些操作
	GetMyPermissions(ctx context.Context, projectKey string, userID uint64, isAdmin bool) (*dto.MyProjectPermissionsResponse, error)

	// 角色权限管理
	GetRolePermissions(ctx context.Context, projectKey string, roleID uint64) ([]string, error)
	SetRolePermissions(ctx context.Context, projectKey string, roleID uint64, permissions []string) error
	CheckUserPermission(ctx context.Context, projectKey string, userID uint64, permission string) (bool, error)
}

// DigestScheduler 用于在项目配置更新后重新注册/注销 cron
// 由 main.go 在 wire 阶段调用 SetDigestScheduler 注入，避免循环依赖
type DigestScheduler interface {
	Reload(project *model.Project)
}

// projectService 项目服务实现
type projectService struct {
	projectRepo     repository.ProjectRepository
	memberRepo      repository.ProjectMemberRepository
	issueTypeRepo   repository.IssueTypeRepository
	roleRepo        repository.ProjectRoleRepository
	roleMemberRepo  repository.ProjectRoleMemberRepository
	permissionRepo  repository.ProjectRolePermissionRepository
	userRepo        userRepo.UserRepository
	db              *gorm.DB
	digestScheduler DigestScheduler
}

// SetDigestScheduler 注入日报调度器（可选）
func (s *projectService) SetDigestScheduler(scheduler DigestScheduler) {
	s.digestScheduler = scheduler
}

// NewProjectService 创建项目服务实例
func NewProjectService(
	projectRepo repository.ProjectRepository,
	memberRepo repository.ProjectMemberRepository,
	issueTypeRepo repository.IssueTypeRepository,
	roleRepo repository.ProjectRoleRepository,
	roleMemberRepo repository.ProjectRoleMemberRepository,
	permissionRepo repository.ProjectRolePermissionRepository,
	userRepo userRepo.UserRepository,
	db *gorm.DB,
) ProjectService {
	return &projectService{
		projectRepo:    projectRepo,
		memberRepo:     memberRepo,
		issueTypeRepo:  issueTypeRepo,
		roleRepo:       roleRepo,
		roleMemberRepo: roleMemberRepo,
		permissionRepo: permissionRepo,
		userRepo:       userRepo,
		db:             db,
	}
}

// CreateProject 创建项目
func (s *projectService) CreateProject(ctx context.Context, req *dto.CreateProjectRequest, creatorID uint64) (*dto.ProjectResponse, error) {
	// 转换为大写
	projectKey := strings.ToUpper(req.ProjectKey)

	// 检查项目 Key 是否已存在
	exists, err := s.projectRepo.ExistsByKey(ctx, projectKey)
	if err != nil {
		logger.Error("failed to check project key existence", zap.Error(err))
		return nil, fmt.Errorf("检查���目 Key 失败: %w", err)
	}
	if exists {
		return nil, ErrProjectKeyExists
	}

	// 检查负责人是否存在
	_, err = s.userRepo.GetByID(ctx, req.LeadUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("负责人不存在")
		}
		return nil, fmt.Errorf("查询负责人失败: %w", err)
	}

	// 创建项目
	project := &model.Project{
		ProjectKey:  projectKey,
		Name:        req.Name,
		Description: req.Description,
		LeadUserID:  req.LeadUserID,
		Status:      1,
	}

	if err := s.projectRepo.Create(ctx, project); err != nil {
		logger.Error("failed to create project", zap.Error(err))
		return nil, fmt.Errorf("创建项目失败: %w", err)
	}

	// 将创建者添加为项目所有者
	member := &model.ProjectMember{
		ProjectID: project.ID,
		UserID:    creatorID,
		Role:      "owner",
	}
	if err := s.memberRepo.Create(ctx, member); err != nil {
		logger.Warn("failed to add creator as owner", zap.Error(err))
	}

	// 如果负责人不是创建者，也添加为成员
	if req.LeadUserID != creatorID {
		leadMember := &model.ProjectMember{
			ProjectID: project.ID,
			UserID:    req.LeadUserID,
			Role:      "administrators",
		}
		if err := s.memberRepo.Create(ctx, leadMember); err != nil {
			logger.Warn("failed to add lead as administrator", zap.Error(err))
		}
	}

	// 初始化项目预置角色
	if err := model.InitProjectRoles(s.db, project.ID); err != nil {
		logger.Warn("failed to init project roles", zap.Error(err))
	} else {
		// 将创建者添加到 administrators 角色
		adminRole, err := s.roleRepo.GetByProjectAndKey(ctx, project.ID, "administrators")
		if err == nil {
			adminMember := &model.ProjectRoleMember{
				ProjectID: project.ID,
				RoleID:    adminRole.ID,
				UserID:    creatorID,
			}
			if createErr := s.roleMemberRepo.Create(ctx, adminMember); createErr != nil {
				logger.Warn("failed to add creator to administrators role", zap.Error(createErr))
			}
		}

		// 如果负责人不是创建者，也添加到 administrators 角色
		if req.LeadUserID != creatorID && err == nil {
			leadAdminMember := &model.ProjectRoleMember{
				ProjectID: project.ID,
				RoleID:    adminRole.ID,
				UserID:    req.LeadUserID,
			}
			if err := s.roleMemberRepo.Create(ctx, leadAdminMember); err != nil {
				logger.Warn("failed to add lead to administrators role", zap.Error(err))
			}
		}
	}

	// 模版项目：初始化字段方案和工作流方案；空项目跳过
	if req.Template != "blank" {
		// 初始化项目字段方案
		if err := model.InitProjectFieldSchemes(s.db, project.ID); err != nil {
			logger.Warn("failed to init project field schemes", zap.Error(err))
		}

		// 初始化项目工作流方案
		if err := model.InitProjectWorkflowSchemes(s.db, project.ID); err != nil {
			logger.Warn("failed to init project workflow schemes", zap.Error(err))
		}
	}

	logger.Info("project created successfully",
		zap.Uint64("project_id", project.ID),
		zap.String("project_key", project.ProjectKey),
	)

	return s.toProjectResponse(ctx, project), nil
}

// GetProject 获取项目详情
func (s *projectService) GetProject(ctx context.Context, key string) (*dto.ProjectResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(key))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		logger.Error("failed to get project", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	return s.toProjectResponse(ctx, project), nil
}

// GetProjectByID 根据 ID 获取项目
func (s *projectService) GetProjectByID(ctx context.Context, id uint64) (*dto.ProjectResponse, error) {
	project, err := s.projectRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	return s.toProjectResponse(ctx, project), nil
}

// UpdateProject 更新项目
func (s *projectService) UpdateProject(ctx context.Context, key string, req *dto.UpdateProjectRequest) (*dto.ProjectResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(key))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 更新字段
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.LeadUserID != nil {
		// 检查负责人是否存在
		_, err = s.userRepo.GetByID(ctx, *req.LeadUserID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("负责人不存在")
			}
			return nil, fmt.Errorf("查询负责人失败: %w", err)
		}
		project.LeadUserID = *req.LeadUserID
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.DailyDigestEnabled != nil {
		project.DailyDigestEnabled = *req.DailyDigestEnabled
	}
	if req.DailyDigestCron != nil {
		project.DailyDigestCron = *req.DailyDigestCron
	}
	if req.DailyDigestTZ != nil {
		project.DailyDigestTZ = *req.DailyDigestTZ
	}
	if req.DailyDigestScope != nil {
		project.DailyDigestScope = *req.DailyDigestScope
	}
	if req.DailyDigestIssueTypeIDs != nil {
		if len(*req.DailyDigestIssueTypeIDs) == 0 {
			project.DailyDigestIssueTypeIDs = ""
		} else {
			ids, err := json.Marshal(*req.DailyDigestIssueTypeIDs)
			if err != nil {
				return nil, fmt.Errorf("序列化工单类型过滤失败: %w", err)
			}
			project.DailyDigestIssueTypeIDs = string(ids)
		}
	}

	if err := s.projectRepo.Update(ctx, project); err != nil {
		logger.Error("failed to update project", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("更新项目失败: %w", err)
	}

	// 重新加载该项目的日报 cron（启用/cron/tz 变化都需要重注册）
	if s.digestScheduler != nil {
		s.digestScheduler.Reload(project)
	}

	logger.Info("project updated successfully", zap.String("project_key", key))

	return s.toProjectResponse(ctx, project), nil
}

// DeleteProject 硬删除项目及所有关联数据
func (s *projectService) DeleteProject(ctx context.Context, key string) error {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, key)
	projectKey := strings.ToUpper(key)
	project, err := s.projectRepo.GetByKey(ctx, projectKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	projectID := project.ID

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 获取项目下所有工单 ID
		var issueIDs []uint64
		if err := tx.Model(&model.Issue{}).Where("project_id = ?", projectID).Pluck("id", &issueIDs).Error; err != nil {
			return fmt.Errorf("查询工单 ID 失败: %w", err)
		}

		if len(issueIDs) > 0 {
			// 2. 删除工单关联数据
			for _, m := range []struct {
				model interface{}
				desc  string
			}{
				{&model.IssueComment{}, "评论"},
				{&model.IssueAttachment{}, "附件"},
				{&model.IssueWatcher{}, "关注人"},
				{&model.IssueFieldValue{}, "字段值"},
				{&model.IssueWorklog{}, "工作日志"},
			} {
				if err := tx.Unscoped().Where("issue_id IN ?", issueIDs).Delete(m.model).Error; err != nil {
					return fmt.Errorf("删除工单%s失败: %w", m.desc, err)
				}
			}

			// 3. 删除工作流实例及关联
			var instanceIDs []uint64
			if err := tx.Model(&model.WorkflowInstance{}).Where("issue_id IN ?", issueIDs).Pluck("id", &instanceIDs).Error; err != nil {
				return fmt.Errorf("查询工作流实例失败: %w", err)
			}
			if len(instanceIDs) > 0 {
				if err := tx.Unscoped().Where("instance_id IN ?", instanceIDs).Delete(&model.WorkflowHistory{}).Error; err != nil {
					return fmt.Errorf("删除工作流历史失败: %w", err)
				}
				if err := tx.Unscoped().Where("instance_id IN ?", instanceIDs).Delete(&model.ApprovalRecord{}).Error; err != nil {
					return fmt.Errorf("删除审批记录失败: %w", err)
				}
				if err := tx.Unscoped().Where("id IN ?", instanceIDs).Delete(&model.WorkflowInstance{}).Error; err != nil {
					return fmt.Errorf("删除工作流实例失败: %w", err)
				}
			}

			// 4. 删除关联告警
			if err := tx.Unscoped().Where("issue_id IN ?", issueIDs).Delete(&model.Alert{}).Error; err != nil {
				return fmt.Errorf("删除关联告警失败: %w", err)
			}

			// 5. 删除通知和活动日志
			if err := tx.Unscoped().Where("entity_type = ? AND entity_id IN ?", "issue", issueIDs).Delete(&model.Notification{}).Error; err != nil {
				return fmt.Errorf("删除通知失败: %w", err)
			}
			if err := tx.Unscoped().Where("entity_type = ? AND entity_id IN ?", "issue", issueIDs).Delete(&model.ActivityLog{}).Error; err != nil {
				return fmt.Errorf("删除活动日志失败: %w", err)
			}

			// 6. 删除工单
			if err := tx.Unscoped().Where("id IN ?", issueIDs).Delete(&model.Issue{}).Error; err != nil {
				return fmt.Errorf("删除工单失败: %w", err)
			}
		}

		// 7. 删除项目级数据：工单类型、字段方案、字段定义
		if err := tx.Unscoped().Where("project_id = ?", projectID).Delete(&model.IssueTypeFieldScheme{}).Error; err != nil {
			return fmt.Errorf("删除字段方案失败: %w", err)
		}
		if err := tx.Unscoped().Where("project_id = ?", projectID).Delete(&model.FieldDefinition{}).Error; err != nil {
			return fmt.Errorf("删除字段定义失败: %w", err)
		}
		if err := tx.Unscoped().Where("project_id = ?", projectID).Delete(&model.IssueType{}).Error; err != nil {
			return fmt.Errorf("删除工单类型失败: %w", err)
		}

		// 8. 删除工作流及关联
		var workflowIDs []uint64
		if err := tx.Model(&model.Workflow{}).Where("project_id = ?", projectID).Pluck("id", &workflowIDs).Error; err != nil {
			return fmt.Errorf("查询工作流失败: %w", err)
		}
		if len(workflowIDs) > 0 {
			if err := tx.Unscoped().Where("workflow_id IN ?", workflowIDs).Delete(&model.WorkflowNode{}).Error; err != nil {
				return fmt.Errorf("删除工作流节点失败: %w", err)
			}
			if err := tx.Unscoped().Where("workflow_id IN ?", workflowIDs).Delete(&model.WorkflowEdge{}).Error; err != nil {
				return fmt.Errorf("删除工作流边失败: %w", err)
			}
			if err := tx.Unscoped().Where("id IN ?", workflowIDs).Delete(&model.Workflow{}).Error; err != nil {
				return fmt.Errorf("删除工作流失败: %w", err)
			}
		}
		if err := tx.Unscoped().Where("project_id = ?", projectID).Delete(&model.WorkflowScheme{}).Error; err != nil {
			return fmt.Errorf("删除工作流方案失败: %w", err)
		}

		// 9. 删除角色及关联
		var roleIDs []uint64
		if err := tx.Model(&model.ProjectRole{}).Where("project_id = ?", projectID).Pluck("id", &roleIDs).Error; err != nil {
			return fmt.Errorf("查询角色失败: %w", err)
		}
		if len(roleIDs) > 0 {
			if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&model.ProjectRoleMember{}).Error; err != nil {
				return fmt.Errorf("删除角色成员失败: %w", err)
			}
			if err := tx.Unscoped().Where("role_id IN ?", roleIDs).Delete(&model.ProjectRolePermission{}).Error; err != nil {
				return fmt.Errorf("删除角色权限失败: %w", err)
			}
			if err := tx.Unscoped().Where("id IN ?", roleIDs).Delete(&model.ProjectRole{}).Error; err != nil {
				return fmt.Errorf("删除角色失败: %w", err)
			}
		}

		// 10. 删除需求池及关联需求数据
		var poolIDs []uint64
		if err := tx.Model(&model.RequirementPool{}).Where("project_id = ?", projectID).Pluck("id", &poolIDs).Error; err != nil {
			return fmt.Errorf("查询需求池失败: %w", err)
		}
		if len(poolIDs) > 0 {
			var requirementIDs []uint64
			if err := tx.Model(&model.Requirement{}).Where("pool_id IN ?", poolIDs).Pluck("id", &requirementIDs).Error; err != nil {
				return fmt.Errorf("查询需求失败: %w", err)
			}
			if len(requirementIDs) > 0 {
				if err := tx.Unscoped().Where("requirement_id IN ?", requirementIDs).Delete(&model.RequirementComment{}).Error; err != nil {
					return fmt.Errorf("删除需求评论失败: %w", err)
				}
				if err := tx.Unscoped().Where("requirement_id IN ?", requirementIDs).Delete(&model.RequirementAttachment{}).Error; err != nil {
					return fmt.Errorf("删除需求附件失败: %w", err)
				}
				if err := tx.Unscoped().Where("id IN ?", requirementIDs).Delete(&model.Requirement{}).Error; err != nil {
					return fmt.Errorf("删除需求失败: %w", err)
				}
			}
			if err := tx.Unscoped().Where("id IN ?", poolIDs).Delete(&model.RequirementPool{}).Error; err != nil {
				return fmt.Errorf("删除需求池失败: %w", err)
			}
		}

		// 11. 删除其他项目级数据
		for _, m := range []struct {
			model interface{}
			desc  string
		}{
			{&model.ProjectMember{}, "项目成员"},
			{&model.ProjectVersion{}, "项目版本"},
			{&model.ProjectComponent{}, "项目组件"},
			{&model.IssueLabel{}, "工单标签"},
			{&model.ProjectNotificationChannel{}, "通知渠道"},
			{&model.AlertRule{}, "告警规则"},
		} {
			if err := tx.Unscoped().Where("project_id = ?", projectID).Delete(m.model).Error; err != nil {
				return fmt.Errorf("删除%s失败: %w", m.desc, err)
			}
		}

		// 12. 删除项目本身
		if err := tx.Unscoped().Delete(&model.Project{}, projectID).Error; err != nil {
			return fmt.Errorf("删除项目失败: %w", err)
		}

		return nil
	}); err != nil {
		logger.Error("failed to cascade delete project", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("删除项目失败: %w", err)
	}

	// 清除 Redis 序号缓存
	sequence.Reset(ctx, projectKey)

	logger.Info("project cascade deleted successfully", zap.String("project_key", key))

	return nil
}

// ListProjects 分页查询项目列表
func (s *projectService) ListProjects(ctx context.Context, req *dto.ListProjectsRequest, userID uint64, isAdmin bool) ([]*dto.ProjectResponse, int64, error) {
	page := req.GetDefaultPage()
	pageSize := req.GetDefaultPageSize()
	offset := (page - 1) * pageSize

	// 非管理员只能看到自己是成员的项目
	var projectIDs []uint64
	if !isAdmin {
		members, err := s.memberRepo.ListByUser(ctx, userID)
		if err != nil {
			return nil, 0, fmt.Errorf("查询用户项目失败: %w", err)
		}
		if len(members) == 0 {
			return []*dto.ProjectResponse{}, 0, nil
		}
		projectIDs = make([]uint64, len(members))
		for i, m := range members {
			projectIDs[i] = m.ProjectID
		}
	}

	projects, total, err := s.projectRepo.List(ctx, offset, pageSize, req.Keyword, req.Status, projectIDs)
	if err != nil {
		logger.Error("failed to list projects", zap.Error(err))
		return nil, 0, fmt.Errorf("查询项目列表失败: %w", err)
	}

	responses := make([]*dto.ProjectResponse, len(projects))
	for i, project := range projects {
		responses[i] = s.toProjectResponse(ctx, project)
	}

	return responses, total, nil
}

// AddMember 添加项目成员
func (s *projectService) AddMember(ctx context.Context, projectKey string, req *dto.AddMemberRequest) (*dto.ProjectMemberResponse, error) {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查用户是否存在
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查是否已是成员
	isMember, err := s.memberRepo.IsMember(ctx, project.ID, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("检查成员失败: %w", err)
	}
	if isMember {
		return nil, ErrMemberAlreadyExists
	}

	// 验证角色：owner 是特殊角色，其他必须是项目中存在的角色 key
	roleName := ""
	if req.Role == "owner" {
		roleName = "所有者"
	} else {
		role, err := s.roleRepo.GetByProjectAndKey(ctx, project.ID, req.Role)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("角色 %s 不存在", req.Role)
			}
			return nil, fmt.Errorf("查询角色失败: %w", err)
		}
		roleName = role.RoleName
	}

	// 创建成员
	member := &model.ProjectMember{
		ProjectID: project.ID,
		UserID:    req.UserID,
		Role:      req.Role,
	}

	if err := s.memberRepo.Create(ctx, member); err != nil {
		logger.Error("failed to add member", zap.Error(err))
		return nil, fmt.Errorf("添加成员失败: %w", err)
	}

	// 同步创建 project_role_members 记录（owner 不需要，权限检查时直接放行）
	if req.Role != "owner" {
		role, err := s.roleRepo.GetByProjectAndKey(ctx, project.ID, req.Role)
		if err == nil {
			roleMember := &model.ProjectRoleMember{
				ProjectID: project.ID,
				RoleID:    role.ID,
				UserID:    req.UserID,
			}
			if err := s.roleMemberRepo.Create(ctx, roleMember); err != nil {
				logger.Error("failed to create role member binding",
					zap.Uint64("user_id", req.UserID),
					zap.String("role_key", req.Role),
					zap.Error(err))
			}
		}
	}

	logger.Info("member added successfully",
		zap.String("project_key", projectKey),
		zap.Uint64("user_id", req.UserID),
	)

	resp := s.toMemberResponse(member, user)
	resp.RoleName = roleName
	return resp, nil
}

// UpdateMember 更新项目成员角色
func (s *projectService) UpdateMember(ctx context.Context, projectKey string, userID uint64, req *dto.UpdateMemberRequest) (*dto.ProjectMemberResponse, error) {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	member, err := s.memberRepo.GetByProjectAndUser(ctx, project.ID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrMemberNotFound
		}
		return nil, fmt.Errorf("查询成员失败: %w", err)
	}

	// 验证角色：owner 是特殊角色，其他必须是项目中存在的角色 key
	roleName := ""
	if req.Role == "owner" {
		roleName = "所有者"
	} else {
		role, err := s.roleRepo.GetByProjectAndKey(ctx, project.ID, req.Role)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("角色 %s 不存在", req.Role)
			}
			return nil, fmt.Errorf("查询角色失败: %w", err)
		}
		roleName = role.RoleName
	}

	oldRole := member.Role
	member.Role = req.Role

	if err := s.memberRepo.Update(ctx, member); err != nil {
		logger.Error("failed to update member", zap.Error(err))
		return nil, fmt.Errorf("更新成员失败: %w", err)
	}

	// 同步更新 project_role_members：先删除旧角色关联，再创建新角色关联
	if oldRole != req.Role {
		// 删除旧角色关联
		if oldRole != "owner" {
			if oldRoleObj, err := s.roleRepo.GetByProjectAndKey(ctx, project.ID, oldRole); err == nil {
				if deleteErr := s.roleMemberRepo.DeleteByRoleAndUser(ctx, oldRoleObj.ID, userID); deleteErr != nil {
					logger.Error("failed to delete old role member binding",
						zap.Uint64("user_id", userID),
						zap.String("old_role", oldRole),
						zap.Error(deleteErr))
				}
			}
		}
		// 创建新角色关联
		if req.Role != "owner" {
			if newRoleObj, err := s.roleRepo.GetByProjectAndKey(ctx, project.ID, req.Role); err == nil {
				roleMember := &model.ProjectRoleMember{
					ProjectID: project.ID,
					RoleID:    newRoleObj.ID,
					UserID:    userID,
				}
				if err := s.roleMemberRepo.Create(ctx, roleMember); err != nil {
					logger.Error("failed to update role member binding",
						zap.Uint64("user_id", userID),
						zap.String("new_role", req.Role),
						zap.Error(err))
				}
			}
		}
	}

	user, userErr := s.userRepo.GetByID(ctx, userID)
	if userErr != nil {
		logger.Warn("failed to query user while updating member",
			zap.Uint64("user_id", userID),
			zap.Error(userErr))
		user = nil
	}

	resp := s.toMemberResponse(member, user)
	resp.RoleName = roleName
	return resp, nil
}

// RemoveMember 移除项目成员
func (s *projectService) RemoveMember(ctx context.Context, projectKey string, userID uint64) error {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	member, err := s.memberRepo.GetByProjectAndUser(ctx, project.ID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		}
		return fmt.Errorf("查询成员失败: %w", err)
	}

	// 不能移除所有者
	if member.Role == "owner" {
		return ErrCannotRemoveOwner
	}

	if err := s.memberRepo.Delete(ctx, member.ID); err != nil {
		logger.Error("failed to remove member", zap.Error(err))
		return fmt.Errorf("移除成员失败: %w", err)
	}

	logger.Info("member removed successfully",
		zap.String("project_key", projectKey),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// ListMembers 获取项目成员列表
func (s *projectService) ListMembers(ctx context.Context, projectKey string) ([]*dto.ProjectMemberResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	members, err := s.memberRepo.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询成员列表失败: %w", err)
	}

	// 构建角色名称映射
	roleNameMap := map[string]string{"owner": "所有者"}
	roles, err := s.roleRepo.ListByProject(ctx, project.ID)
	if err == nil {
		for _, r := range roles {
			roleNameMap[r.RoleKey] = r.RoleName
		}
	}

	responses := make([]*dto.ProjectMemberResponse, len(members))
	for i, member := range members {
		user, userErr := s.userRepo.GetByID(ctx, member.UserID)
		if userErr != nil {
			logger.Warn("failed to query user while listing members",
				zap.Uint64("user_id", member.UserID),
				zap.Error(userErr))
			user = nil
		}
		resp := s.toMemberResponse(member, user)
		if name, ok := roleNameMap[member.Role]; ok {
			resp.RoleName = name
		} else {
			resp.RoleName = member.Role
		}
		responses[i] = resp
	}

	return responses, nil
}

// CheckPermission 检查用户在项目中的权限
func (s *projectService) CheckPermission(ctx context.Context, projectKey string, userID uint64, requiredRoles ...string) (bool, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrProjectNotFound
		}
		return false, err
	}

	role, err := s.memberRepo.GetUserRole(ctx, project.ID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	for _, r := range requiredRoles {
		if role == r {
			return true, nil
		}
	}

	return false, nil
}

// CreateIssueType 创建工单类型
func (s *projectService) CreateIssueType(ctx context.Context, projectKey string, req *dto.CreateIssueTypeRequest) (*dto.IssueTypeResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	issueType := &model.IssueType{
		ProjectID:   &project.ID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Icon:        req.Icon,
		Color:       req.Color,
	}

	if err := s.issueTypeRepo.Create(ctx, issueType); err != nil {
		logger.Error("failed to create issue type", zap.Error(err))
		return nil, fmt.Errorf("创建工单类型失败: %w", err)
	}

	return s.toIssueTypeResponse(issueType), nil
}

// UpdateIssueType 更新工单类型
func (s *projectService) UpdateIssueType(ctx context.Context, id uint64, req *dto.UpdateIssueTypeRequest) (*dto.IssueTypeResponse, error) {
	issueType, err := s.issueTypeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueTypeNotFound
		}
		return nil, fmt.Errorf("查询工单类型失败: %w", err)
	}

	if req.DisplayName != nil {
		issueType.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		issueType.Description = *req.Description
	}
	if req.Icon != nil {
		issueType.Icon = *req.Icon
	}
	if req.Color != nil {
		issueType.Color = *req.Color
	}

	if err := s.issueTypeRepo.Update(ctx, issueType); err != nil {
		return nil, fmt.Errorf("更新工单类型失败: %w", err)
	}

	return s.toIssueTypeResponse(issueType), nil
}

// DeleteIssueType 删除工单类型
func (s *projectService) DeleteIssueType(ctx context.Context, id uint64) error {
	issueType, err := s.issueTypeRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIssueTypeNotFound
		}
		return fmt.Errorf("查询工单类型失败: %w", err)
	}

	// 不能删除全局工单类型
	if issueType.ProjectID == nil {
		return fmt.Errorf("不能删除全局工单类型")
	}

	if err := s.issueTypeRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除工单类型失败: %w", err)
	}

	return nil
}

// ListIssueTypes 获取项目可用的工单类型
func (s *projectService) ListIssueTypes(ctx context.Context, projectKey string) ([]*dto.IssueTypeResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	issueTypes, err := s.issueTypeRepo.ListAvailableForProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询工单类型失败: %w", err)
	}

	responses := make([]*dto.IssueTypeResponse, len(issueTypes))
	for i, it := range issueTypes {
		responses[i] = s.toIssueTypeResponse(it)
	}

	return responses, nil
}

// ListAllIssueTypes 获取所有工单类型（用于筛选器）
func (s *projectService) ListAllIssueTypes(ctx context.Context) ([]*dto.IssueTypeResponse, error) {
	issueTypes, err := s.issueTypeRepo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询所有工单类型失败: %w", err)
	}

	responses := make([]*dto.IssueTypeResponse, len(issueTypes))
	for i, it := range issueTypes {
		responses[i] = s.toIssueTypeResponse(it)
	}

	return responses, nil
}

// toProjectResponse 将项目模型转换为响应 DTO
func (s *projectService) toProjectResponse(ctx context.Context, project *model.Project) *dto.ProjectResponse {
	resp := &dto.ProjectResponse{
		ID:                 project.ID,
		ProjectKey:         project.ProjectKey,
		Name:               project.Name,
		Description:        project.Description,
		LeadUserID:         project.LeadUserID,
		Status:             project.Status,
		DailyDigestEnabled: project.DailyDigestEnabled,
		DailyDigestCron:    project.DailyDigestCron,
		DailyDigestTZ:      project.DailyDigestTZ,
		DailyDigestScope:   project.DailyDigestScope,
		CreatedAt:          project.CreatedAt,
		UpdatedAt:          project.UpdatedAt,
	}
	// 反序列化日报工单类型过滤
	if project.DailyDigestIssueTypeIDs != "" {
		var ids []uint64
		if err := json.Unmarshal([]byte(project.DailyDigestIssueTypeIDs), &ids); err == nil {
			resp.DailyDigestIssueTypeIDs = ids
		}
	}
	if resp.DailyDigestIssueTypeIDs == nil {
		resp.DailyDigestIssueTypeIDs = []uint64{}
	}

	// 获取负责人信息
	if leadUser, err := s.userRepo.GetByID(ctx, project.LeadUserID); err == nil {
		resp.LeadUser = &dto.UserBrief{
			ID:          leadUser.ID,
			Username:    leadUser.Username,
			DisplayName: leadUser.DisplayName,
			AvatarURL:   leadUser.AvatarURL,
		}
	}

	// 获取成员数量
	if count, err := s.memberRepo.CountByProject(ctx, project.ID); err == nil {
		resp.MemberCount = int(count)
	}

	return resp
}

// toMemberResponse 将成员模型转换为响应 DTO
func (s *projectService) toMemberResponse(member *model.ProjectMember, user *model.User) *dto.ProjectMemberResponse {
	resp := &dto.ProjectMemberResponse{
		ID:        member.ID,
		ProjectID: member.ProjectID,
		UserID:    member.UserID,
		Role:      member.Role,
		CreatedAt: member.CreatedAt,
	}

	if user != nil {
		resp.User = &dto.UserBrief{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
		}
	}

	return resp
}

// toIssueTypeResponse 将工单类型模型转换为响应 DTO
func (s *projectService) toIssueTypeResponse(issueType *model.IssueType) *dto.IssueTypeResponse {
	return &dto.IssueTypeResponse{
		ID:          issueType.ID,
		ProjectID:   issueType.ProjectID,
		Name:        issueType.Name,
		DisplayName: issueType.DisplayName,
		Description: issueType.Description,
		Icon:        issueType.Icon,
		Color:       issueType.Color,
		CreatedAt:   issueType.CreatedAt,
		UpdatedAt:   issueType.UpdatedAt,
	}
}

// ============ 项目角色管理 ============

// CreateRole 创建项目角色
func (s *projectService) CreateRole(ctx context.Context, projectKey string, req *dto.CreateProjectRoleRequest) (*dto.ProjectRoleResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查角色 Key 是否已存在
	_, err = s.roleRepo.GetByProjectAndKey(ctx, project.ID, req.RoleKey)
	if err == nil {
		return nil, ErrRoleKeyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("检查角色失败: %w", err)
	}

	role := &model.ProjectRole{
		ProjectID:   project.ID,
		RoleKey:     req.RoleKey,
		RoleName:    req.RoleName,
		Description: req.Description,
		IsSystem:    false,
		SortOrder:   100, // 自定义角色排在后面
	}

	if err := s.roleRepo.Create(ctx, role); err != nil {
		logger.Error("failed to create project role", zap.Error(err))
		return nil, fmt.Errorf("创建角色失败: %w", err)
	}

	logger.Info("project role created",
		zap.String("project_key", projectKey),
		zap.String("role_key", req.RoleKey),
	)

	return s.toRoleResponse(ctx, role), nil
}

// UpdateRole 更新项目角色
func (s *projectService) UpdateRole(ctx context.Context, projectKey string, roleID uint64, req *dto.UpdateProjectRoleRequest) (*dto.ProjectRoleResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}

	// 验证角色属于该项目
	if role.ProjectID != project.ID {
		return nil, ErrRoleNotFound
	}

	if req.RoleName != nil {
		role.RoleName = *req.RoleName
	}
	if req.Description != nil {
		role.Description = *req.Description
	}

	if err := s.roleRepo.Update(ctx, role); err != nil {
		logger.Error("failed to update project role", zap.Error(err))
		return nil, fmt.Errorf("更新角色失败: %w", err)
	}

	return s.toRoleResponse(ctx, role), nil
}

// DeleteRole 删除项目角色
func (s *projectService) DeleteRole(ctx context.Context, projectKey string, roleID uint64) error {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("查询角色失败: %w", err)
	}

	// 验证角色属于该项目
	if role.ProjectID != project.ID {
		return ErrRoleNotFound
	}

	// 不能删除系统预置角色
	if role.IsSystem {
		return ErrCannotDeleteSystemRole
	}

	if err := s.roleRepo.Delete(ctx, roleID); err != nil {
		logger.Error("failed to delete project role", zap.Error(err))
		return fmt.Errorf("删除角色失败: %w", err)
	}

	logger.Info("project role deleted",
		zap.String("project_key", projectKey),
		zap.Uint64("role_id", roleID),
	)

	return nil
}

// ListRoles 获取项目角色列表
func (s *projectService) ListRoles(ctx context.Context, projectKey string) ([]*dto.ProjectRoleResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	roles, err := s.roleRepo.ListByProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("查询角色列表失败: %w", err)
	}

	responses := make([]*dto.ProjectRoleResponse, len(roles))
	for i, role := range roles {
		responses[i] = s.toRoleResponse(ctx, role)
	}

	return responses, nil
}

// AddRoleMember 添加角色成员
func (s *projectService) AddRoleMember(ctx context.Context, projectKey string, roleID uint64, req *dto.AddRoleMemberRequest) (*dto.ProjectRoleMemberResponse, error) {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}

	// 验证角色属于该项目
	if role.ProjectID != project.ID {
		return nil, ErrRoleNotFound
	}

	// 检查用户是否存在
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查是否已是角色成员
	exists, err := s.roleMemberRepo.Exists(ctx, roleID, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("检查角色成员失败: %w", err)
	}
	if exists {
		return nil, ErrRoleMemberExists
	}

	member := &model.ProjectRoleMember{
		ProjectID: project.ID,
		RoleID:    roleID,
		UserID:    req.UserID,
	}

	if err := s.roleMemberRepo.Create(ctx, member); err != nil {
		logger.Error("failed to add role member", zap.Error(err))
		return nil, fmt.Errorf("添加角色成员失败: %w", err)
	}

	logger.Info("role member added",
		zap.String("project_key", projectKey),
		zap.Uint64("role_id", roleID),
		zap.Uint64("user_id", req.UserID),
	)

	return s.toRoleMemberResponse(member, user), nil
}

// RemoveRoleMember 移除角色成员
func (s *projectService) RemoveRoleMember(ctx context.Context, projectKey string, roleID, userID uint64) error {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("查询角色失败: %w", err)
	}

	// 验证角色属于该项目
	if role.ProjectID != project.ID {
		return ErrRoleNotFound
	}

	// 检查成员是否存在
	_, err = s.roleMemberRepo.GetByRoleAndUser(ctx, roleID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleMemberNotFound
		}
		return fmt.Errorf("查询角色成员失败: %w", err)
	}

	if err := s.roleMemberRepo.DeleteByRoleAndUser(ctx, roleID, userID); err != nil {
		logger.Error("failed to remove role member", zap.Error(err))
		return fmt.Errorf("移除角色成员失败: %w", err)
	}

	logger.Info("role member removed",
		zap.String("project_key", projectKey),
		zap.Uint64("role_id", roleID),
		zap.Uint64("user_id", userID),
	)

	return nil
}

// ListRoleMembers 获取角色成员列表
func (s *projectService) ListRoleMembers(ctx context.Context, projectKey string, roleID uint64) ([]*dto.ProjectRoleMemberResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}

	// 验证角色属于该项目
	if role.ProjectID != project.ID {
		return nil, ErrRoleNotFound
	}

	members, err := s.roleMemberRepo.ListByRole(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("查询角色成员列表失败: %w", err)
	}

	responses := make([]*dto.ProjectRoleMemberResponse, len(members))
	for i, member := range members {
		user, userErr := s.userRepo.GetByID(ctx, member.UserID)
		if userErr != nil {
			logger.Warn("failed to query user while listing role members",
				zap.Uint64("user_id", member.UserID),
				zap.Error(userErr))
			user = nil
		}
		responses[i] = s.toRoleMemberResponse(member, user)
	}

	return responses, nil
}

// GetUserRoles 获取用户在项目中的角色
func (s *projectService) GetUserRoles(ctx context.Context, projectKey string, userID uint64) ([]*dto.ProjectRoleResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	members, err := s.roleMemberRepo.ListByProjectAndUser(ctx, project.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("查询用户角色失败: %w", err)
	}

	responses := make([]*dto.ProjectRoleResponse, 0, len(members))
	for _, member := range members {
		role, err := s.roleRepo.GetByID(ctx, member.RoleID)
		if err != nil {
			continue
		}
		responses = append(responses, s.toRoleResponse(ctx, role))
	}

	return responses, nil
}

// toRoleResponse 将角色模型转换为响应 DTO
func (s *projectService) toRoleResponse(ctx context.Context, role *model.ProjectRole) *dto.ProjectRoleResponse {
	resp := &dto.ProjectRoleResponse{
		ID:          role.ID,
		ProjectID:   role.ProjectID,
		RoleKey:     role.RoleKey,
		RoleName:    role.RoleName,
		Description: role.Description,
		IsSystem:    role.IsSystem,
		SortOrder:   role.SortOrder,
		CreatedAt:   role.CreatedAt,
		UpdatedAt:   role.UpdatedAt,
	}

	// 获取成员数量
	if count, err := s.roleRepo.CountMembersByRole(ctx, role.ID); err == nil {
		resp.MemberCount = int(count)
	}

	// 获取角色权限
	if perms, err := s.permissionRepo.ListByRole(ctx, role.ID); err == nil {
		resp.Permissions = perms
	}

	return resp
}

// toRoleMemberResponse 将角色成员模型转换为响应 DTO
func (s *projectService) toRoleMemberResponse(member *model.ProjectRoleMember, user *model.User) *dto.ProjectRoleMemberResponse {
	resp := &dto.ProjectRoleMemberResponse{
		ID:        member.ID,
		ProjectID: member.ProjectID,
		RoleID:    member.RoleID,
		UserID:    member.UserID,
		CreatedAt: member.CreatedAt,
	}

	if user != nil {
		resp.User = &dto.UserBrief{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
		}
	}

	return resp
}

// ============ 角色权限管理 ============

// GetRolePermissions 获取角色权限
func (s *projectService) GetRolePermissions(ctx context.Context, projectKey string, roleID uint64) ([]string, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("查询角色失败: %w", err)
	}

	if role.ProjectID != project.ID {
		return nil, ErrRoleNotFound
	}

	perms, err := s.permissionRepo.ListByRole(ctx, roleID)
	if err != nil {
		return nil, fmt.Errorf("查询角色权限失败: %w", err)
	}

	return perms, nil
}

// SetRolePermissions 设置角色权限
func (s *projectService) SetRolePermissions(ctx context.Context, projectKey string, roleID uint64, permissions []string) error {
	// 权限相关变更后立即失效该项目的权限缓存。
	// 用 defer 覆盖所有返回路径：失败路径多失效一次只是一次缓存回源，
	// 而漏失效会让被移除的成员在 TTL 内仍然通过校验。
	defer InvalidateProjectPermissions(ctx, projectKey)
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrProjectNotFound
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	role, err := s.roleRepo.GetByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return fmt.Errorf("查询角色失败: %w", err)
	}

	if role.ProjectID != project.ID {
		return ErrRoleNotFound
	}

	if err := s.permissionRepo.SetPermissions(ctx, roleID, permissions); err != nil {
		logger.Error("failed to set role permissions", zap.Error(err))
		return fmt.Errorf("设置角色权限失败: %w", err)
	}

	logger.Info("role permissions updated",
		zap.String("project_key", projectKey),
		zap.Uint64("role_id", roleID),
		zap.Int("permission_count", len(permissions)),
	)

	return nil
}

// CheckUserPermission 检查用户在项目中是否有指定权限
//
// 命中缓存时零数据库查询；未命中才回源，回源结果整体缓存。
// 权限相关变更通过 InvalidateProjectPermissions 递增项目版本号即时失效。
// GetMyPermissions 返回用户在项目中的完整权限集。
//
// 走的是 CheckUserPermission 同一条缓存路径，不另开一套判定逻辑 ——
// 前端拿到的必须和中间件实际执行的是同一份，否则界面显示得了的操作后端会拒。
func (s *projectService) GetMyPermissions(ctx context.Context, projectKey string, userID uint64, isAdmin bool) (*dto.MyProjectPermissionsResponse, error) {
	if isAdmin {
		// 系统管理员在中间件里是直接放行的，这里保持一致
		return &dto.MyProjectPermissionsResponse{IsAdmin: true, IsMember: true, IsOwner: true}, nil
	}

	cached := loadCachedPermissions(ctx, projectKey, userID)
	if cached == nil {
		loaded, err := s.loadUserProjectPermissions(ctx, projectKey, userID)
		if err != nil {
			return nil, err
		}
		storeCachedPermissions(ctx, projectKey, userID, loaded)
		cached = loaded
	}

	return &dto.MyProjectPermissionsResponse{
		IsMember:    cached.IsMember,
		IsOwner:     cached.IsOwner,
		Permissions: cached.Permissions,
	}, nil
}

func (s *projectService) CheckUserPermission(ctx context.Context, projectKey string, userID uint64, permission string) (bool, error) {
	if cached := loadCachedPermissions(ctx, projectKey, userID); cached != nil {
		return cached.has(permission), nil
	}

	cached, err := s.loadUserProjectPermissions(ctx, projectKey, userID)
	if err != nil {
		return false, err
	}
	storeCachedPermissions(ctx, projectKey, userID, cached)
	return cached.has(permission), nil
}

// loadUserProjectPermissions 从数据库读取用户在项目中的完整权限集
func (s *projectService) loadUserProjectPermissions(ctx context.Context, projectKey string, userID uint64) (*cachedPermissions, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 检查是否为项目 owner，owner 拥有全部权限
	member, err := s.memberRepo.GetByProjectAndUser(ctx, project.ID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 非项目成员：同样缓存该结论，避免无权限用户反复穿透到数据库
			return &cachedPermissions{IsMember: false}, nil
		}
		return nil, fmt.Errorf("查询成员失败: %w", err)
	}
	if member.Role == "owner" {
		return &cachedPermissions{IsMember: true, IsOwner: true}, nil
	}

	// 获取用户在项目中的所有角色
	roleMembers, err := s.roleMemberRepo.ListByProjectAndUser(ctx, project.ID, userID)
	if err != nil {
		return nil, fmt.Errorf("查询用户角色失败: %w", err)
	}

	if len(roleMembers) == 0 {
		return &cachedPermissions{IsMember: true}, nil
	}

	roleIDs := make([]uint64, len(roleMembers))
	for i, rm := range roleMembers {
		roleIDs[i] = rm.RoleID
	}

	// 获取合并权限
	perms, err := s.permissionRepo.ListByRoles(ctx, roleIDs)
	if err != nil {
		return nil, fmt.Errorf("查询角色权限失败: %w", err)
	}

	return &cachedPermissions{IsMember: true, Permissions: perms}, nil
}
