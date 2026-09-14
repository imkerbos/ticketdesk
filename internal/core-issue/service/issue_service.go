// Package service 提供工单业务逻辑层
package service

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/activity/detail"
	"github.com/kerbos/ticketdesk/internal/core-issue/dto"
	"github.com/kerbos/ticketdesk/internal/core-issue/repository"
	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	userRepo "github.com/kerbos/ticketdesk/internal/core-user/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
	"github.com/kerbos/ticketdesk/pkg/sequence"
)

// 业务错误定义
var (
	ErrIssueNotFound     = errors.New("workflow.issue_not_found")
	ErrProjectNotFound   = errors.New("workflow.project_not_found")
	ErrIssueTypeNotFound = errors.New("project.issue_type_not_found")
	ErrUserNotFound      = errors.New("user.not_found")
	ErrCommentNotFound   = errors.New("issue.comment_not_found")
	ErrAlreadyWatching   = errors.New("issue.already_watching")
	ErrNotWatching       = errors.New("issue.not_watching")
	ErrWorklogNotFound   = errors.New("issue.worklog_not_found")
	ErrUnauthorized      = errors.New("issue.no_permission")
	ErrInvalidTimeFormat = errors.New("issue.bad_time_format")
)

type ctxKey string

const ContextUserIDKey ctxKey = "user_id"

// IssueService 工单服务接口
type IssueService interface {
	CreateIssue(ctx context.Context, req *dto.CreateIssueRequest, reporterID uint64) (*dto.IssueResponse, error)
	CreateIssueWithAttachments(ctx context.Context, req *dto.CreateIssueRequest, files []*multipart.FileHeader, reporterID uint64) (*dto.IssueResponse, error)
	GetIssue(ctx context.Context, key string) (*dto.IssueResponse, error)
	UpdateIssue(ctx context.Context, key string, req *dto.UpdateIssueRequest) (*dto.IssueResponse, error)
	DeleteIssue(ctx context.Context, key string) error
	ListIssues(ctx context.Context, req *dto.ListIssuesRequest) ([]*dto.IssueResponse, int64, bool, error)
	AssignIssue(ctx context.Context, key string, assigneeID uint64) (*dto.IssueResponse, error)

	// Dashboard 专用
	ListMyTodoIssues(ctx context.Context, userID uint64, page, pageSize int, projectIDs []uint64) ([]*dto.IssueResponse, int64, bool, error)
	ListMyCreatedIssues(ctx context.Context, userID uint64, page, pageSize int, projectIDs []uint64) ([]*dto.IssueResponse, int64, bool, error)

	// Epic 相关
	ListIssuesInEpic(ctx context.Context, epicKey string) ([]*dto.IssueResponse, error)

	// 子任务相关
	ListSubtasks(ctx context.Context, parentKey string) ([]*dto.IssueResponse, error)

	// 评论
	AddComment(ctx context.Context, issueKey string, req *dto.CreateCommentRequest, userID uint64) (*dto.CommentResponse, error)
	ListComments(ctx context.Context, issueKey string) ([]*dto.CommentResponse, error)
	DeleteComment(ctx context.Context, commentID, userID uint64) error

	// 关注
	AddWatcher(ctx context.Context, issueKey string, userID uint64) error
	RemoveWatcher(ctx context.Context, issueKey string, userID uint64) error
	ListWatchers(ctx context.Context, issueKey string) ([]*dto.WatcherResponse, error)

	// 工作日志
	AddWorklog(ctx context.Context, issueKey string, req *dto.CreateWorklogRequest, userID uint64) (*dto.WorklogResponse, error)
	UpdateWorklog(ctx context.Context, worklogID uint64, req *dto.UpdateWorklogRequest, userID uint64) (*dto.WorklogResponse, error)
	DeleteWorklog(ctx context.Context, worklogID, userID uint64) error
	ListWorklogs(ctx context.Context, issueKey string) ([]*dto.WorklogResponse, error)

	// 统计
	GetIssueListStats(ctx context.Context, req *dto.ListIssuesRequest) (*dto.IssueListStatsResponse, error)
	GetProjectOverviewStats(ctx context.Context, projectKey string) (*dto.ProjectOverviewStatsResponse, error)
}

// issueService 工单服务实现
type issueService struct {
	issueRepo         repository.IssueRepository
	commentRepo       repository.CommentRepository
	watcherRepo       repository.WatcherRepository
	worklogRepo       repository.WorklogRepository
	projectRepo       projectRepo.ProjectRepository
	issueTypeRepo     projectRepo.IssueTypeRepository
	userRepo          userRepo.UserRepository
	db                *gorm.DB           // 用于级联删除事务
	alertSyncSvc      AlertSyncService   // 告警同步服务（可选）
	activityLogger    ActivityLogger     // 活动日志记录器（可选）
	notifSender       NotificationSender // 通知发送服务（可选）
	workflowEngine    WorkflowEngine     // 工作流引擎（可选）
	fieldValueSaver   FieldValueSaver    // 字段值保存服务（可选）
	epicLinkGetter    EpicLinkGetter     // Epic 链接获取服务（可选）
	projectNotifier   ProjectNotifier    // 项目外部通知服务（可选）
	attachmentService AttachmentService  // 附件服务（可选, 通过 setter 注入避免循环依赖）
}

// WorkflowEngine 工作流引擎接口（避免循环依赖）
type WorkflowEngine interface {
	CreateInstance(ctx context.Context, issueID, workflowID uint64) (*model.WorkflowInstance, error)
	// TryCreateInstanceForIssue 根据项目和工单类型查找工作流方案，如果存在则自动创建工作流实例
	TryCreateInstanceForIssue(ctx context.Context, issueID, projectID, issueTypeID uint64) (*model.WorkflowInstance, error)
}

// ActivityLogger 活动日志记录器接口（避免循环依赖）
type ActivityLogger interface {
	LogActivity(ctx context.Context, userID uint64, userName, action, entityType string, entityID uint64, entityKey, details string) error
}

// EpicLinkGetter Epic 链接获取接口（避免循环依赖）
type EpicLinkGetter interface {
	GetIssueIDsByEpicLink(ctx context.Context, epicID uint64) ([]uint64, error)
}

// ProjectNotifier 项目通知接口（向项目的外部渠道发送通知，避免循环依赖）
type ProjectNotifier interface {
	NotifyProject(ctx context.Context, projectID uint64, event string, data any) error
}

// NotificationSender 通知发送接口（避免循环依赖）
type NotificationSender interface {
	CreateNotification(ctx context.Context, req *NotificationRequest) error
}

// FieldValueSaver 字段值保存接口（避免循环依赖）
type FieldValueSaver interface {
	SaveIssueFieldValues(ctx context.Context, issueID uint64, values []CustomFieldValueInput) error
}

// CustomFieldValueInput 自定义字段值输入（避免导入依赖）
type CustomFieldValueInput struct {
	FieldID uint64
	Value   any
}

// NotificationRequest 通知请求（本地定义，避免依赖 notification-inbox/dto）
type NotificationRequest struct {
	UserID uint64
	Type   string
	Title  string
	// TitleKey / TitleArgs 标题的语言包 key 与参数。
	// 用它而不是直接给 Title：标题最终要按收件人的语言渲染，
	// 而这里还不知道收件人想看哪种语言。
	TitleKey    string
	TitleArgs   []any
	Content     string
	ContentKey  string
	ContentArgs []any
	EntityType  string
	EntityID    uint64
	EntityKey   string
	ActorID     uint64
	ActorName   string
}

// NewIssueService 创建工单服务实例
func NewIssueService(
	issueRepo repository.IssueRepository,
	commentRepo repository.CommentRepository,
	watcherRepo repository.WatcherRepository,
	worklogRepo repository.WorklogRepository,
	projectRepo projectRepo.ProjectRepository,
	issueTypeRepo projectRepo.IssueTypeRepository,
	userRepo userRepo.UserRepository,
	db *gorm.DB,
) IssueService {
	return &issueService{
		issueRepo:      issueRepo,
		commentRepo:    commentRepo,
		watcherRepo:    watcherRepo,
		worklogRepo:    worklogRepo,
		projectRepo:    projectRepo,
		issueTypeRepo:  issueTypeRepo,
		userRepo:       userRepo,
		db:             db,
		alertSyncSvc:   nil, // 默认为 nil，可通过 SetAlertSyncService 设置
		activityLogger: nil, // 默认为 nil，可通过 SetActivityLogger 设置
	}
}

// SetAlertSyncService 设置告警同步服务（用于避免循环依赖）
func (s *issueService) SetAlertSyncService(alertSyncSvc AlertSyncService) {
	s.alertSyncSvc = alertSyncSvc
}

// SetActivityLogger 设置活动日志记录器（用于避免循环依赖）
func (s *issueService) SetActivityLogger(activityLogger ActivityLogger) {
	s.activityLogger = activityLogger
}

// SetNotificationService 设置通知发送服务（用于避免循环依赖）
func (s *issueService) SetNotificationService(notifSender NotificationSender) {
	s.notifSender = notifSender
}

// SetWorkflowEngine 设置工作流引擎（用于避免循环依赖）
func (s *issueService) SetWorkflowEngine(workflowEngine WorkflowEngine) {
	s.workflowEngine = workflowEngine
}

// SetFieldValueSaver 设置字段值保存服务（用于避免循环依赖）
func (s *issueService) SetFieldValueSaver(fieldValueSaver FieldValueSaver) {
	s.fieldValueSaver = fieldValueSaver
}

// SetEpicLinkGetter 设置 Epic 链接获取服务（用于避免循环依赖）
func (s *issueService) SetEpicLinkGetter(epicLinkGetter EpicLinkGetter) {
	s.epicLinkGetter = epicLinkGetter
}

// SetProjectNotifier 设置项目外部通知服务（用于避免循环依赖）
func (s *issueService) SetProjectNotifier(projectNotifier ProjectNotifier) {
	s.projectNotifier = projectNotifier
}

// SetAttachmentService 设置附件服务（用于避免循环依赖）
func (s *issueService) SetAttachmentService(svc AttachmentService) {
	s.attachmentService = svc
}

// buildAssigneeMention 根据指派人 ID 构造 mentions 列表
// 用于 lark/telegram sender 渲染 @ 提及；email 作为 lark @ 的兜底（邮箱与飞书账号一致时有效）
func (s *issueService) buildAssigneeMention(ctx context.Context, assigneeID *uint64) []map[string]interface{} {
	if assigneeID == nil || *assigneeID == 0 {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, *assigneeID)
	if err != nil || user == nil {
		return nil
	}
	// 三种 @ 方式都没有可用通道时跳过；email 在 lark sender 作为 open_id 兜底
	if user.LarkOpenID == "" && user.TelegramUserID == "" && user.Email == "" {
		return nil
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return []map[string]interface{}{{
		"display_name":     name,
		"lark_open_id":     user.LarkOpenID,
		"email":            user.Email,
		"telegram_user_id": user.TelegramUserID,
	}}
}

// notifyProjectChannels 向项目的外部通知渠道发送通知（异步不阻塞主流程）
func (s *issueService) notifyProjectChannels(projectID uint64, event string, data map[string]interface{}) {
	if s.projectNotifier == nil {
		return
	}
	go func() {
		defer safego.Recover("issue.notifyProjectChannels")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.projectNotifier.NotifyProject(ctx, projectID, event, data); err != nil {
			logger.Warn("failed to notify project channels",
				zap.Uint64("project_id", projectID),
				zap.String("event", event),
				zap.Error(err),
			)
		}
	}()
}

// sendNotification 发送通知（内部辅助方法，异步不阻塞主流程）
func (s *issueService) sendNotification(actorID uint64, actorName string, req *NotificationRequest) {
	if s.notifSender == nil {
		return
	}
	req.ActorID = actorID
	req.ActorName = actorName
	go func() {
		defer safego.Recover("issue.sendNotification")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.notifSender.CreateNotification(ctx, req); err != nil {
			logger.Warn("failed to send notification", zap.Error(err))
		}
	}()
}

// notifyWatchers 通知所有关注者（排除指定用户），返回已通知的用户ID集合
// tmpl 是通知模板：除 UserID 外都已填好，逐个关注者复制一份填上收件人。
// 标题/正文用 key 而不是成文的句子，最终由站内通知服务按各人的语言渲染。
func (s *issueService) notifyWatchers(issue *model.Issue, excludeUserID, actorID uint64, actorName string, tmpl NotificationRequest) map[uint64]bool {
	notified := make(map[uint64]bool)
	if s.notifSender == nil {
		return notified
	}
	watchers, err := s.watcherRepo.ListByIssue(context.Background(), issue.ID)
	if err != nil {
		logger.Warn("failed to list watchers for notification", zap.Error(err))
		return notified
	}
	tmpl.EntityType = "issue"
	tmpl.EntityID = issue.ID
	tmpl.EntityKey = issue.IssueKey
	for _, w := range watchers {
		if w.UserID == excludeUserID {
			continue
		}
		notified[w.UserID] = true
		req := tmpl
		req.UserID = w.UserID
		s.sendNotification(actorID, actorName, &req)
	}
	return notified
}

// getUserFromCtx 从 context 中获取当前用户信息
func (s *issueService) getUserFromCtx(ctx context.Context) (uint64, string) {
	userID, _ := ctx.Value(ContextUserIDKey).(uint64)
	if userID == 0 {
		return 0, "系统"
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return userID, "未知用户"
	}
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return userID, name
}

// logActivity 记录活动日志（内部辅助方法）
func (s *issueService) logActivity(_ context.Context, userID uint64, userName, action, entityKey, details string, entityID uint64) {
	if s.activityLogger == nil {
		return
	}
	// 异步记录，不阻塞主流程
	go func() {
		defer safego.Recover("issue.logActivity")
		if err := s.activityLogger.LogActivity(context.Background(), userID, userName, action, "issue", entityID, entityKey, details); err != nil {
			logger.Warn("failed to log activity", zap.Error(err))
		}
	}()
}

// prepareIssue 准备阶段：校验、生成 Key、解析日期、构造 model.Issue（无数据库写入）
func (s *issueService) prepareIssue(ctx context.Context, req *dto.CreateIssueRequest, reporterID uint64) (*model.Project, *model.Issue, error) {
	// 获取项目
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(req.ProjectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrProjectNotFound
		}
		return nil, nil, fmt.Errorf("查询项目失败: %w", err)
	}

	// 验证工单类型
	issueType, err := s.issueTypeRepo.GetByID(ctx, req.IssueTypeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrIssueTypeNotFound
		}
		return nil, nil, fmt.Errorf("查询工单类型失败: %w", err)
	}

	// 验证指派人
	if req.AssigneeID != nil {
		_, assigneeErr := s.userRepo.GetByID(ctx, *req.AssigneeID)
		if assigneeErr != nil {
			if errors.Is(assigneeErr, gorm.ErrRecordNotFound) {
				return nil, nil, fmt.Errorf("指派人不存在")
			}
			return nil, nil, fmt.Errorf("查询指派人失败: %w", assigneeErr)
		}
	}

	// 生成工单 Key（优先使用 Redis 原子计数器）
	dbFallback := func(ctx context.Context, pKey string) (int64, error) {
		return s.issueRepo.GetNextIssueNumber(ctx, project.ID)
	}
	nextNum, err := sequence.NextIssueNumber(ctx, project.ProjectKey, dbFallback)
	if err != nil {
		return nil, nil, fmt.Errorf("生成工单编号失败: %w", err)
	}
	issueKey := repository.GenerateIssueKey(project.ProjectKey, nextNum)

	// 解析截止日期
	var dueDate *time.Time
	if req.DueDate != nil && *req.DueDate != "" {
		t, err := time.Parse("2006-01-02", *req.DueDate)
		if err != nil {
			return nil, nil, fmt.Errorf("截止日期格式错误")
		}
		dueDate = &t
	}

	// 解析预计开始时间
	var plannedStartDate *time.Time
	if req.PlannedStartDate != nil && *req.PlannedStartDate != "" {
		t, err := time.Parse("2006-01-02", *req.PlannedStartDate)
		if err != nil {
			return nil, nil, fmt.Errorf("预计开始时间格式错误")
		}
		plannedStartDate = &t
	}

	// 解析预计交付时间
	var plannedEndDate *time.Time
	if req.PlannedEndDate != nil && *req.PlannedEndDate != "" {
		t, err := time.Parse("2006-01-02", *req.PlannedEndDate)
		if err != nil {
			return nil, nil, fmt.Errorf("预计交付时间格式错误")
		}
		plannedEndDate = &t
	}

	// 设置默认优先级
	priority := req.Priority
	if priority == "" {
		priority = "P2"
	}

	// 构造工单对象（未持久化）
	issue := &model.Issue{
		IssueKey:         issueKey,
		ProjectID:        project.ID,
		IssueTypeID:      issueType.ID,
		Title:            req.Title,
		Description:      req.Description,
		Priority:         priority,
		Status:           "open",
		ReporterID:       reporterID,
		AssigneeID:       req.AssigneeID,
		ParentID:         req.ParentID,
		EpicID:           req.EpicID,
		DueDate:          dueDate,
		PlannedStartDate: plannedStartDate,
		PlannedEndDate:   plannedEndDate,
	}

	return project, issue, nil
}

// createIssueInTx 事务内写入：仅执行工单记录的数据库插入
func (s *issueService) createIssueInTx(ctx context.Context, tx *gorm.DB, issue *model.Issue) error {
	return tx.WithContext(ctx).Create(issue).Error
}

// afterIssueCreated 事务后副作用：字段值、关注人、通知、工作流实例、活动日志
// 返回可能被工作流引擎刷新过的 issue 指针
func (s *issueService) afterIssueCreated(ctx context.Context, issue *model.Issue, project *model.Project, req *dto.CreateIssueRequest, reporterID uint64) *model.Issue {
	// 保存自定义字段值
	logger.Info("CreateIssue: checking custom fields",
		zap.Bool("fieldValueSaver_nil", s.fieldValueSaver == nil),
		zap.Int("custom_fields_count", len(req.CustomFields)),
	)
	if s.fieldValueSaver != nil && len(req.CustomFields) > 0 {
		inputs := make([]CustomFieldValueInput, len(req.CustomFields))
		for i, v := range req.CustomFields {
			inputs[i] = CustomFieldValueInput{FieldID: v.FieldID, Value: v.Value}
			logger.Info("CreateIssue: custom field",
				zap.Uint64("field_id", v.FieldID),
				zap.Any("value", v.Value),
			)
		}
		if err := s.fieldValueSaver.SaveIssueFieldValues(ctx, issue.ID, inputs); err != nil {
			logger.Warn("failed to save custom field values", zap.Error(err), zap.String("issue_key", issue.IssueKey))
			// 不阻塞工单创建，只记录警告
		} else {
			logger.Info("CreateIssue: custom fields saved successfully", zap.String("issue_key", issue.IssueKey))
		}
	}

	// 自动添加报告人为关注人
	watcher := &model.IssueWatcher{
		IssueID: issue.ID,
		UserID:  reporterID,
	}
	_ = s.watcherRepo.Create(ctx, watcher)

	// 获取报告人信息（用于通知）
	reporter, _ := s.userRepo.GetByID(ctx, reporterID)
	reporterName := ""
	if reporter != nil {
		reporterName = reporter.DisplayName
		if reporterName == "" {
			reporterName = reporter.Username
		}
	}

	// 通知：工单被指派
	if issue.AssigneeID != nil && *issue.AssigneeID != reporterID {
		s.sendNotification(reporterID, reporterName, &NotificationRequest{
			UserID:     *issue.AssigneeID,
			Type:       "issue_assigned",
			TitleKey:   "inbox.issue_assigned_to_you",
			TitleArgs:  []any{issue.IssueKey},
			Content:    issue.Title,
			EntityType: "issue",
			EntityID:   issue.ID,
			EntityKey:  issue.IssueKey,
		})
	}

	// 通知项目外部渠道（飞书/Telegram）
	createdData := map[string]interface{}{
		"issue_key":    issue.IssueKey,
		"issue_title":  issue.Title,
		"project_name": project.Name,
		"status":       issue.Status,
		"priority":     issue.Priority,
	}
	if mentions := s.buildAssigneeMention(ctx, issue.AssigneeID); mentions != nil {
		createdData["mentions"] = mentions
	}
	s.notifyProjectChannels(project.ID, "issue.created", createdData)

	// 创建工作流实例（根据项目+工单类型查找工作流方案）
	if s.workflowEngine != nil {
		instance, err := s.workflowEngine.TryCreateInstanceForIssue(ctx, issue.ID, project.ID, issue.IssueTypeID)
		if err != nil {
			logger.Warn("failed to create workflow instance", zap.Error(err), zap.String("issue_key", issue.IssueKey))
		} else if instance != nil {
			// 重新读取工单（工作流引擎可能已联动更新了状态），避免覆盖
			if freshIssue, err := s.issueRepo.GetByID(ctx, issue.ID); err == nil {
				freshIssue.WorkflowInstanceID = &instance.ID
				_ = s.issueRepo.Update(ctx, freshIssue)
				issue = freshIssue // 用最新数据返回
			}
			logger.Info("workflow instance created for issue",
				zap.String("issue_key", issue.IssueKey),
				zap.Uint64("instance_id", instance.ID),
			)
		}
	}

	logger.Info("issue created successfully",
		zap.String("issue_key", issue.IssueKey),
		zap.Uint64("reporter_id", reporterID),
	)

	// 记录活动日志
	if s.activityLogger != nil {
		actReporter, _ := s.userRepo.GetByID(ctx, reporterID)
		actReporterName := "未知用户"
		if actReporter != nil {
			actReporterName = actReporter.DisplayName
		}
		_ = s.activityLogger.LogActivity(
			ctx,
			reporterID,
			actReporterName,
			"issue_created",
			"issue",
			issue.ID,
			issue.IssueKey,
			detail.New("activity.detail.issueCreated", "title", issue.Title),
		)
	}

	return issue
}

// CreateIssue 创建工单
func (s *issueService) CreateIssue(ctx context.Context, req *dto.CreateIssueRequest, reporterID uint64) (*dto.IssueResponse, error) {
	project, issue, err := s.prepareIssue(ctx, req, reporterID)
	if err != nil {
		return nil, err
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.createIssueInTx(ctx, tx, issue)
	}); err != nil {
		logger.Error("failed to create issue", zap.Error(err))
		return nil, fmt.Errorf("创建工单失败: %w", err)
	}

	issue = s.afterIssueCreated(ctx, issue, project, req, reporterID)
	return s.toIssueResponse(ctx, issue, project.ProjectKey), nil
}

// CreateIssueWithAttachments 在单事务内原子创建工单 + 附件
// 任一环节失败 → 事务回滚 + 删除已落盘文件
func (s *issueService) CreateIssueWithAttachments(
	ctx context.Context,
	req *dto.CreateIssueRequest,
	files []*multipart.FileHeader,
	reporterID uint64,
) (*dto.IssueResponse, error) {
	// 前置校验所有文件 (大小 + 扩展名), 任一不合规直接返回
	for _, fh := range files {
		if fh.Size > MaxAttachmentSize {
			return nil, ErrFileTooLarge
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if !IsAllowedAttachmentExt(ext) {
			return nil, ErrInvalidFileType
		}
	}

	if len(files) > 0 && s.attachmentService == nil {
		return nil, fmt.Errorf("附件服务未注入")
	}

	// 准备工单数据 (无 DB 写)
	project, issue, err := s.prepareIssue(ctx, req, reporterID)
	if err != nil {
		return nil, err
	}

	var savedPaths []string
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// 工单写入
		if err := s.createIssueInTx(ctx, tx, issue); err != nil {
			return err
		}
		// 逐个落盘 + 写附件记录
		for _, fh := range files {
			relPath, err := s.attachmentService.SaveFile(ctx, fh)
			if err != nil {
				return err
			}
			savedPaths = append(savedPaths, relPath)

			if _, err := s.attachmentService.CreateRecordInTx(ctx, tx, issue.ID, fh, relPath, reporterID); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		// 事务回滚 → 清理所有已落盘文件
		for _, p := range savedPaths {
			if delErr := s.attachmentService.DeletePath(ctx, p); delErr != nil {
				logger.Warn("failed to delete file after rollback",
					zap.String("path", p),
					zap.Error(delErr),
				)
			}
		}
		logger.Error("failed to create issue with attachments", zap.Error(err))
		return nil, err
	}

	// 事务外副作用
	issue = s.afterIssueCreated(ctx, issue, project, req, reporterID)
	return s.toIssueResponse(ctx, issue, project.ProjectKey), nil
}

// GetIssue 获取工单详情
func (s *issueService) GetIssue(ctx context.Context, key string) (*dto.IssueResponse, error) {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(key))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}

	project, _ := s.projectRepo.GetByID(ctx, issue.ProjectID)
	projectKey := ""
	if project != nil {
		projectKey = project.ProjectKey
	}

	resp := s.toIssueResponse(ctx, issue, projectKey)

	// 加载评论
	comments, _ := s.commentRepo.ListByIssue(ctx, issue.ID)
	resp.Comments = make([]*dto.CommentResponse, len(comments))
	for i, c := range comments {
		resp.Comments[i] = s.toCommentResponse(ctx, c)
	}

	// 加载关注人
	watchers, _ := s.watcherRepo.ListByIssue(ctx, issue.ID)
	resp.Watchers = make([]*dto.UserBrief, len(watchers))
	for i, w := range watchers {
		if user, err := s.userRepo.GetByID(ctx, w.UserID); err == nil {
			resp.Watchers[i] = &dto.UserBrief{
				ID:          user.ID,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				AvatarURL:   user.AvatarURL,
			}
		}
	}

	return resp, nil
}

// UpdateIssue 更新工单
func (s *issueService) UpdateIssue(ctx context.Context, key string, req *dto.UpdateIssueRequest) (*dto.IssueResponse, error) {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(key))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}

	// 更新字段
	if req.Title != nil {
		issue.Title = *req.Title
	}
	if req.Description != nil {
		issue.Description = *req.Description
	}
	if req.Priority != nil {
		issue.Priority = *req.Priority
	}
	if req.Resolution != nil {
		issue.Resolution = *req.Resolution
	}
	if req.EpicID != nil {
		// 验证 Epic 是否存在
		if *req.EpicID != 0 {
			epicIssue, err := s.issueRepo.GetByID(ctx, *req.EpicID)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, fmt.Errorf("Epic 不存在")
				}
				return nil, fmt.Errorf("查询 Epic 失败: %w", err)
			}
			// 验证是否为 Epic 类型
			epicType, _ := s.issueTypeRepo.GetByID(ctx, epicIssue.IssueTypeID)
			if epicType == nil || !strings.EqualFold(epicType.Name, "epic") {
				return nil, fmt.Errorf("指定的工单不是 Epic 类型")
			}
			issue.EpicID = req.EpicID
		} else {
			issue.EpicID = nil
		}
	}
	if req.AssigneeID != nil {
		// 验证指派人
		_, err := s.userRepo.GetByID(ctx, *req.AssigneeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("指派人不存在")
			}
			return nil, fmt.Errorf("查询指派人失败: %w", err)
		}
		issue.AssigneeID = req.AssigneeID
	}
	if req.DueDate != nil {
		if *req.DueDate == "" {
			issue.DueDate = nil
		} else {
			t, err := time.Parse("2006-01-02", *req.DueDate)
			if err != nil {
				return nil, fmt.Errorf("截止日期格式错误")
			}
			issue.DueDate = &t
		}
	}
	if req.PlannedStartDate != nil {
		if *req.PlannedStartDate == "" {
			issue.PlannedStartDate = nil
		} else {
			t, err := time.Parse("2006-01-02", *req.PlannedStartDate)
			if err != nil {
				return nil, fmt.Errorf("预计开始时间格式错误")
			}
			issue.PlannedStartDate = &t
		}
	}
	if req.PlannedEndDate != nil {
		if *req.PlannedEndDate == "" {
			issue.PlannedEndDate = nil
		} else {
			t, err := time.Parse("2006-01-02", *req.PlannedEndDate)
			if err != nil {
				return nil, fmt.Errorf("预计交付时间格式错误")
			}
			issue.PlannedEndDate = &t
		}
	}
	if req.IssueTypeID != nil {
		_, err := s.issueTypeRepo.GetByID(ctx, *req.IssueTypeID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrIssueTypeNotFound
			}
			return nil, fmt.Errorf("查询工单类型失败: %w", err)
		}
		issue.IssueTypeID = *req.IssueTypeID
	}

	if err := s.issueRepo.Update(ctx, issue); err != nil {
		logger.Error("failed to update issue", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("更新工单失败: %w", err)
	}

	// 保存自定义字段值
	if s.fieldValueSaver != nil && len(req.CustomFields) > 0 {
		inputs := make([]CustomFieldValueInput, len(req.CustomFields))
		for i, v := range req.CustomFields {
			inputs[i] = CustomFieldValueInput{FieldID: v.FieldID, Value: v.Value}
		}
		if err := s.fieldValueSaver.SaveIssueFieldValues(ctx, issue.ID, inputs); err != nil {
			logger.Warn("failed to save custom field values on update", zap.Error(err), zap.String("issue_key", key))
		}
	}

	project, _ := s.projectRepo.GetByID(ctx, issue.ProjectID)
	projectKey := ""
	if project != nil {
		projectKey = project.ProjectKey
	}

	logger.Info("issue updated successfully", zap.String("issue_key", key))

	// 记录活动日志 - 记录具体的变更
	if s.activityLogger != nil {
		userID, userName := s.getUserFromCtx(ctx)

		// 每个变更存成一条带 key 的子项，拼接和翻译都留给前端 ——
		// 这里拼出中文就等于把写入方的语言烧进库里（见 internal/activity/detail）
		var changes []detail.Detail
		if req.Title != nil {
			changes = append(changes, detail.Item("activity.detail.field.title"))
		}
		if req.Description != nil {
			changes = append(changes, detail.Item("activity.detail.field.description"))
		}
		if req.Priority != nil {
			changes = append(changes, detail.Item("activity.detail.field.priority", "value", *req.Priority))
		}
		if req.Resolution != nil {
			changes = append(changes, detail.Item("activity.detail.field.resolution", "value", *req.Resolution))
		}
		if req.AssigneeID != nil {
			if assignee, err := s.userRepo.GetByID(ctx, *req.AssigneeID); err == nil {
				changes = append(changes, detail.Item("activity.detail.field.assignee", "value", assignee.DisplayName))
			}
		}
		if req.PlannedStartDate != nil {
			if *req.PlannedStartDate == "" {
				changes = append(changes, detail.Item("activity.detail.field.plannedStartCleared"))
			} else {
				changes = append(changes, detail.Item("activity.detail.field.plannedStart", "value", *req.PlannedStartDate))
			}
		}
		if req.PlannedEndDate != nil {
			if *req.PlannedEndDate == "" {
				changes = append(changes, detail.Item("activity.detail.field.plannedEndCleared"))
			} else {
				changes = append(changes, detail.Item("activity.detail.field.plannedEnd", "value", *req.PlannedEndDate))
			}
		}
		if req.DueDate != nil {
			if *req.DueDate == "" {
				changes = append(changes, detail.Item("activity.detail.field.dueDateCleared"))
			} else {
				changes = append(changes, detail.Item("activity.detail.field.dueDate", "value", *req.DueDate))
			}
		}
		if req.EpicID != nil {
			if *req.EpicID == 0 {
				changes = append(changes, detail.Item("activity.detail.field.epicCleared"))
			} else {
				changes = append(changes, detail.Item("activity.detail.field.epic", "value", *req.EpicID))
			}
		}
		if len(req.CustomFields) > 0 {
			changes = append(changes, detail.Item("activity.detail.field.customFields"))
		}

		if len(changes) > 0 {
			details := detail.List("activity.detail.issueUpdated", changes)
			_ = s.activityLogger.LogActivity(
				ctx,
				userID,
				userName,
				"issue_updated",
				"issue",
				issue.ID,
				issue.IssueKey,
				details,
			)

		}
	}

	return s.toIssueResponse(ctx, issue, projectKey), nil
}

// DeleteIssue 硬删除工单及所有关联数据
func (s *issueService) DeleteIssue(ctx context.Context, key string) error {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(key))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIssueNotFound
		}
		return fmt.Errorf("查询工单失败: %w", err)
	}

	issueID := issue.ID
	issueKey := issue.IssueKey
	issueTitle := issue.Title

	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 删除评论、附件、关注人、字段值、工作日志
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
			if err := tx.Unscoped().Where("issue_id = ?", issueID).Delete(m.model).Error; err != nil {
				return fmt.Errorf("删除工单%s失败: %w", m.desc, err)
			}
		}

		// 2. 删除工作流实例及关联
		var instanceIDs []uint64
		if err := tx.Model(&model.WorkflowInstance{}).Where("issue_id = ?", issueID).Pluck("id", &instanceIDs).Error; err != nil {
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

		// 3. 解除告警关联
		if err := tx.Model(&model.Alert{}).Where("issue_id = ?", issueID).Update("issue_id", nil).Error; err != nil {
			return fmt.Errorf("解除告警关联失败: %w", err)
		}

		// 4. 删除通知和活动日志
		if err := tx.Unscoped().Where("entity_type = ? AND entity_id = ?", "issue", issueID).Delete(&model.Notification{}).Error; err != nil {
			return fmt.Errorf("删除通知失败: %w", err)
		}
		if err := tx.Unscoped().Where("entity_type = ? AND entity_id = ?", "issue", issueID).Delete(&model.ActivityLog{}).Error; err != nil {
			return fmt.Errorf("删除活动日志失败: %w", err)
		}

		// 5. 处理合并关联：将指向本工单的 merged_into_issue_id 置 nil
		if err := tx.Model(&model.Issue{}).Where("merged_into_issue_id = ?", issueID).Update("merged_into_issue_id", nil).Error; err != nil {
			return fmt.Errorf("清除合并关联失败: %w", err)
		}

		// 6. 删除工单本身
		if err := tx.Unscoped().Delete(&model.Issue{}, issueID).Error; err != nil {
			return fmt.Errorf("删除工单失败: %w", err)
		}

		return nil
	}); err != nil {
		logger.Error("failed to cascade delete issue", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("删除工单失败: %w", err)
	}

	logger.Info("issue cascade deleted successfully", zap.String("issue_key", issueKey))

	// 记录活动日志（工单已删，只记录操作）
	userID, userName := s.getUserFromCtx(ctx)
	s.logActivity(ctx, userID, userName, "issue_deleted", issueKey, detail.New("activity.detail.issueDeleted", "title", issueTitle), issueID)

	return nil
}

// ListIssues 分页查询工单列表
func (s *issueService) ListIssues(ctx context.Context, req *dto.ListIssuesRequest) ([]*dto.IssueResponse, int64, bool, error) {
	page := req.GetDefaultPage()
	pageSize := req.GetDefaultPageSize()
	offset := (page - 1) * pageSize

	filter, err := s.buildIssueFilter(ctx, req)
	if err != nil {
		return nil, 0, false, err
	}

	result, err := s.issueRepo.List(ctx, filter, offset, pageSize)
	if err != nil {
		logger.Error("failed to list issues", zap.Error(err))
		return nil, 0, false, fmt.Errorf("查询工单列表失败: %w", err)
	}

	return s.batchToIssueResponses(ctx, result.Issues), result.Total, result.HasMore, nil
}

// buildIssueFilter 根据 ListIssuesRequest 构建 IssueFilter（ListIssues 和 GetIssueListStats 共用）
func (s *issueService) buildIssueFilter(ctx context.Context, req *dto.ListIssuesRequest) (*repository.IssueFilter, error) {
	// 解析逗号分隔的状态值
	var statuses []string
	if req.Status != "" {
		for _, s := range strings.Split(req.Status, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				statuses = append(statuses, s)
			}
		}
	}

	filter := &repository.IssueFilter{
		Status:          statuses,
		Priority:        req.Priority,
		AssigneeID:      req.AssigneeID,
		ReporterID:      req.ReporterID,
		IssueTypeID:     req.IssueTypeID,
		EpicID:          req.EpicID,
		Keyword:         req.Keyword,
		Category:        req.Category,
		ProjectIDs:      req.ProjectIDs,
		LimitByProjects: req.ProjectIDs != nil,
		SortBy:          req.SortBy,
		Order:           req.Order,
	}

	// 解析日期范围
	if req.StartDate != "" {
		t, err := time.ParseInLocation("2006-01-02", req.StartDate, time.Local)
		if err != nil {
			return nil, fmt.Errorf("start_date 格式错误: %w", err)
		}
		filter.StartDate = &t
	}
	if req.EndDate != "" {
		t, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
		if err != nil {
			return nil, fmt.Errorf("end_date 格式错误: %w", err)
		}
		// 结束日期取当天 23:59:59
		endOfDay := t.Add(24*time.Hour - time.Second)
		filter.EndDate = &endOfDay
	}

	// 如果指定了项目 Key，获取项目 ID
	if req.ProjectKey != "" {
		project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(req.ProjectKey))
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrProjectNotFound
			}
			return nil, fmt.Errorf("查询项目失败: %w", err)
		}
		filter.ProjectID = &project.ID
	}

	return filter, nil
}

// ListMyTodoIssues 获取我的待办工单（指派给我的待处理/进行中工单）
func (s *issueService) ListMyTodoIssues(ctx context.Context, userID uint64, page, pageSize int, projectIDs []uint64) ([]*dto.IssueResponse, int64, bool, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	filter := &repository.IssueFilter{
		AssigneeID:      &userID,
		StatusNotIn:     []string{"closed", "resolved"},
		ProjectIDs:      projectIDs,
		LimitByProjects: projectIDs != nil,
	}

	result, err := s.issueRepo.List(ctx, filter, offset, pageSize)
	if err != nil {
		logger.Error("failed to list my todo issues", zap.Error(err))
		return nil, 0, false, fmt.Errorf("查询我的待办工单失败: %w", err)
	}

	return s.batchToIssueResponses(ctx, result.Issues), result.Total, result.HasMore, nil
}

// ListMyCreatedIssues 获取我创建的工单
func (s *issueService) ListMyCreatedIssues(ctx context.Context, userID uint64, page, pageSize int, projectIDs []uint64) ([]*dto.IssueResponse, int64, bool, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	offset := (page - 1) * pageSize

	filter := &repository.IssueFilter{
		ReporterID:      &userID,
		ProjectIDs:      projectIDs,
		LimitByProjects: projectIDs != nil,
	}

	result, err := s.issueRepo.List(ctx, filter, offset, pageSize)
	if err != nil {
		logger.Error("failed to list my created issues", zap.Error(err))
		return nil, 0, false, fmt.Errorf("查询我创建的工单失败: %w", err)
	}

	return s.batchToIssueResponses(ctx, result.Issues), result.Total, result.HasMore, nil
}

// GetIssueListStats 获取工单列表统计数据
func (s *issueService) GetIssueListStats(ctx context.Context, req *dto.ListIssuesRequest) (*dto.IssueListStatsResponse, error) {
	filter, err := s.buildIssueFilter(ctx, req)
	if err != nil {
		return nil, err
	}

	stats, err := s.issueRepo.GetListStats(ctx, filter)
	if err != nil {
		logger.Error("failed to get issue list stats", zap.Error(err))
		return nil, fmt.Errorf("查询工单统计失败: %w", err)
	}

	var completionRate float64
	if stats.Total > 0 {
		completionRate = float64(stats.Resolved) / float64(stats.Total) * 100
		// 保留一位小数
		completionRate = float64(int(completionRate*10)) / 10
	}

	return &dto.IssueListStatsResponse{
		Total:           stats.Total,
		Resolved:        stats.Resolved,
		CompletionRate:  completionRate,
		AvgResolveHours: float64(int(stats.AvgResolveHours*10)) / 10,
	}, nil
}

// GetProjectOverviewStats 获取项目概述统计（按状态分组聚合）
func (s *issueService) GetProjectOverviewStats(ctx context.Context, projectKey string) (*dto.ProjectOverviewStatsResponse, error) {
	project, err := s.projectRepo.GetByKey(ctx, strings.ToUpper(projectKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectNotFound
		}
		return nil, fmt.Errorf("查询项目失败: %w", err)
	}

	stats, err := s.issueRepo.GetProjectOverviewStats(ctx, project.ID)
	if err != nil {
		logger.Error("failed to get project overview stats", zap.Uint64("project_id", project.ID), zap.Error(err))
		return nil, fmt.Errorf("查询项目概述统计失败: %w", err)
	}

	return &dto.ProjectOverviewStatsResponse{
		Pending:       stats.Pending,
		InProgress:    stats.InProgress,
		PendingReview: stats.PendingReview,
		Completed:     stats.Completed,
		Total:         stats.Total,
	}, nil
}

// ListIssuesInEpic 获取 Epic 下的所有 Issues
func (s *issueService) ListIssuesInEpic(ctx context.Context, epicKey string) ([]*dto.IssueResponse, error) {
	// 获取 Epic Issue
	epicIssue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(epicKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询 Epic 失败: %w", err)
	}

	// 直接查询 epic_id 字段（新方式）
	issues, err := s.issueRepo.ListByEpicID(ctx, epicIssue.ID)
	if err != nil {
		logger.Error("failed to query issues by epic_id", zap.Error(err))
		return nil, fmt.Errorf("查询 Epic 关联工单失败: %w", err)
	}

	// 如果新方式没有数据，尝试通过旧的字段表方式查询（兼容迁移期间）
	if len(issues) == 0 && s.epicLinkGetter != nil {
		issueIDs, err := s.epicLinkGetter.GetIssueIDsByEpicLink(ctx, epicIssue.ID)
		if err == nil && len(issueIDs) > 0 {
			issues, err = s.issueRepo.ListByIDs(ctx, issueIDs)
			if err != nil {
				logger.Error("failed to query issues", zap.Error(err))
				return nil, fmt.Errorf("查询工单失败: %w", err)
			}
		}
	}

	if len(issues) == 0 {
		return []*dto.IssueResponse{}, nil
	}

	return s.batchToIssueResponses(ctx, issues), nil
}

// AssignIssue 指派工单
func (s *issueService) AssignIssue(ctx context.Context, key string, assigneeID uint64) (*dto.IssueResponse, error) {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(key))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}

	// 验证指派人
	_, err = s.userRepo.GetByID(ctx, assigneeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	oldAssigneeID := issue.AssigneeID
	issue.AssigneeID = &assigneeID
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return nil, fmt.Errorf("指派工单失败: %w", err)
	}

	// 通知：新指派人（如果指派人有变化）
	if oldAssigneeID == nil || *oldAssigneeID != assigneeID {
		// 从上下文获取操作人信息
		actorID, _ := ctx.Value(ContextUserIDKey).(uint64)
		actorName := ""
		if actor, err := s.userRepo.GetByID(ctx, actorID); err == nil {
			actorName = actor.DisplayName
			if actorName == "" {
				actorName = actor.Username
			}
		}
		s.sendNotification(actorID, actorName, &NotificationRequest{
			UserID:     assigneeID,
			Type:       "issue_assigned",
			TitleKey:   "inbox.issue_assigned_to_you",
			TitleArgs:  []any{issue.IssueKey},
			Content:    issue.Title,
			EntityType: "issue",
			EntityID:   issue.ID,
			EntityKey:  issue.IssueKey,
		})

		// 通知创建者
		if issue.ReporterID != actorID && issue.ReporterID != assigneeID {
			s.sendNotification(actorID, actorName, &NotificationRequest{
				UserID:     issue.ReporterID,
				Type:       "issue_updated",
				TitleKey:   "inbox.issue_reassigned",
				TitleArgs:  []any{issue.IssueKey},
				Content:    issue.Title,
				EntityType: "issue",
				EntityID:   issue.ID,
				EntityKey:  issue.IssueKey,
			})
		}
	}

	project, _ := s.projectRepo.GetByID(ctx, issue.ProjectID)
	projectKey := ""
	if project != nil {
		projectKey = project.ProjectKey
	}

	// 查询指派人名称（通知和活动日志共用）
	assignee, _ := s.userRepo.GetByID(ctx, assigneeID)
	assigneeName := "未知用户"
	if assignee != nil {
		assigneeName = assignee.DisplayName
		if assigneeName == "" {
			assigneeName = assignee.Username
		}
	}

	// 查询操作人名称
	operatorName := "系统"
	if opID, ok := ctx.Value(ContextUserIDKey).(uint64); ok && opID > 0 {
		if op, err := s.userRepo.GetByID(ctx, opID); err == nil {
			operatorName = op.DisplayName
			if operatorName == "" {
				operatorName = op.Username
			}
		}
	}

	// 通知项目外部渠道（飞书/Telegram）
	if project != nil {
		notifData := map[string]interface{}{
			"issue_key":    issue.IssueKey,
			"issue_title":  issue.Title,
			"project_name": project.Name,
			"status":       issue.Status,
			"priority":     issue.Priority,
			"assignee":     assigneeName,
			"operator":     operatorName,
		}
		if issue.DueDate != nil {
			notifData["due_date"] = issue.DueDate.Format("2006-01-02 15:04")
		}
		if mentions := s.buildAssigneeMention(ctx, &assigneeID); mentions != nil {
			notifData["mentions"] = mentions
		}
		s.notifyProjectChannels(project.ID, "issue.assigned", notifData)
	}

	// 记录活动日志
	if s.activityLogger != nil {
		actorID, actorName := s.getUserFromCtx(ctx)
		details := detail.New("activity.detail.issueAssigned", "name", assigneeName)
		s.logActivity(ctx, actorID, actorName, "issue_assigned", issue.IssueKey, details, issue.ID)
	}

	return s.toIssueResponse(ctx, issue, projectKey), nil
}

// ListSubtasks 获取工单的所有子任务
func (s *issueService) ListSubtasks(ctx context.Context, parentKey string) ([]*dto.IssueResponse, error) {
	// 获取父工单
	parentIssue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(parentKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询父工单失败: %w", err)
	}

	// 查询所有子任务
	subtasks, err := s.issueRepo.ListByParentID(ctx, parentIssue.ID)
	if err != nil {
		logger.Error("failed to query subtasks", zap.Error(err))
		return nil, fmt.Errorf("查询子任务失败: %w", err)
	}

	if len(subtasks) == 0 {
		return []*dto.IssueResponse{}, nil
	}

	return s.batchToIssueResponses(ctx, subtasks), nil
}

// AddComment 添加评论
func (s *issueService) AddComment(ctx context.Context, issueKey string, req *dto.CreateCommentRequest, userID uint64) (*dto.CommentResponse, error) {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(issueKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}

	comment := &model.IssueComment{
		IssueID: issue.ID,
		UserID:  userID,
		Content: req.Content,
	}

	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("添加评论失败: %w", err)
	}

	// 记录活动日志
	user, _ := s.userRepo.GetByID(ctx, userID)
	userName := ""
	if user != nil {
		userName = user.DisplayName
		if userName == "" {
			userName = user.Username
		}
		s.logActivity(ctx, userID, userName, "comment_added", issue.IssueKey, "", issue.ID)
	}

	// 通知关注者
	notifiedUsers := s.notifyWatchers(issue, userID, userID, userName, NotificationRequest{
		Type:      "issue_commented",
		TitleKey:  "inbox.issue_commented",
		TitleArgs: []any{issue.IssueKey},
		Content:   req.Content, // 评论正文是用户写的，原样落库不翻译
	})

	// 通知创建者（如果不是评论人，且未作为关注人被通知）
	if issue.ReporterID != userID && !notifiedUsers[issue.ReporterID] {
		notifiedUsers[issue.ReporterID] = true
		s.sendNotification(userID, userName, &NotificationRequest{
			UserID:     issue.ReporterID,
			Type:       "issue_commented",
			TitleKey:   "inbox.your_issue_commented",
			TitleArgs:  []any{issue.IssueKey},
			Content:    req.Content,
			EntityType: "issue",
			EntityID:   issue.ID,
			EntityKey:  issue.IssueKey,
		})
	}

	// 解析 @提及
	mentions := extractMentions(req.Content)
	for _, username := range mentions {
		mentionedUser, err := s.userRepo.GetByUsername(ctx, username)
		if err != nil || mentionedUser == nil {
			continue
		}
		// 不重复通知自己和已通知的用户
		if mentionedUser.ID == userID || notifiedUsers[mentionedUser.ID] {
			continue
		}
		notifiedUsers[mentionedUser.ID] = true
		s.sendNotification(userID, userName, &NotificationRequest{
			UserID:     mentionedUser.ID,
			Type:       "mention",
			TitleKey:   "inbox.mentioned_you",
			TitleArgs:  []any{userName, issue.IssueKey},
			Content:    req.Content,
			EntityType: "issue",
			EntityID:   issue.ID,
			EntityKey:  issue.IssueKey,
		})
	}

	return s.toCommentResponse(ctx, comment), nil
}

// ListComments 获取评论列表
func (s *issueService) ListComments(ctx context.Context, issueKey string) ([]*dto.CommentResponse, error) {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(issueKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}

	comments, err := s.commentRepo.ListByIssue(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("查询评论失败: %w", err)
	}

	responses := make([]*dto.CommentResponse, len(comments))
	for i, c := range comments {
		responses[i] = s.toCommentResponse(ctx, c)
	}

	return responses, nil
}

// DeleteComment 删除评论
func (s *issueService) DeleteComment(ctx context.Context, commentID, userID uint64) error {
	comment, err := s.commentRepo.GetByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommentNotFound
		}
		return fmt.Errorf("查询评论失败: %w", err)
	}

	// 只有评论作者可以删除
	if comment.UserID != userID {
		return fmt.Errorf("只能删除自己的评论")
	}

	if err := s.commentRepo.Delete(ctx, commentID); err != nil {
		return err
	}

	// 记录活动日志
	if s.activityLogger != nil {
		issue, _ := s.issueRepo.GetByID(ctx, comment.IssueID)
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "未知用户"
		issueKey := ""
		if user != nil {
			userName = user.DisplayName
			if userName == "" {
				userName = user.Username
			}
		}
		if issue != nil {
			issueKey = issue.IssueKey
		}
		content := comment.Content
		if len(content) > 50 {
			content = content[:50] + "..."
		}
		s.logActivity(ctx, userID, userName, "comment_deleted", issueKey, detail.New("activity.detail.commentDeleted", "content", content), comment.IssueID)
	}

	return nil
}

// AddWatcher 添加关注人
func (s *issueService) AddWatcher(ctx context.Context, issueKey string, userID uint64) error {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(issueKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIssueNotFound
		}
		return fmt.Errorf("查询工单失败: %w", err)
	}

	// 检查是否已关注
	watching, err := s.watcherRepo.IsWatching(ctx, issue.ID, userID)
	if err != nil {
		return fmt.Errorf("检查关注状态失败: %w", err)
	}
	if watching {
		return ErrAlreadyWatching
	}

	watcher := &model.IssueWatcher{
		IssueID: issue.ID,
		UserID:  userID,
	}

	if err := s.watcherRepo.Create(ctx, watcher); err != nil {
		return err
	}

	// 记录活动日志
	if s.activityLogger != nil {
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "未知用户"
		if user != nil {
			userName = user.DisplayName
		}
		_ = s.activityLogger.LogActivity(
			ctx,
			userID,
			userName,
			"issue_watched",
			"issue",
			issue.ID,
			issue.IssueKey,
			detail.New("activity.detail.issueWatched", "name", userName),
		)
	}

	return nil
}

// RemoveWatcher 移除关注人
func (s *issueService) RemoveWatcher(ctx context.Context, issueKey string, userID uint64) error {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(issueKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIssueNotFound
		}
		return fmt.Errorf("查询工单失败: %w", err)
	}

	// 记录活动日志
	if s.activityLogger != nil {
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "未知用户"
		if user != nil {
			userName = user.DisplayName
		}
		_ = s.activityLogger.LogActivity(
			ctx,
			userID,
			userName,
			"issue_unwatched",
			"issue",
			issue.ID,
			issue.IssueKey,
			detail.New("activity.detail.issueUnwatched", "name", userName),
		)
	}

	return s.watcherRepo.DeleteByIssueAndUser(ctx, issue.ID, userID)
}

// ListWatchers 获取关注人列表
func (s *issueService) ListWatchers(ctx context.Context, issueKey string) ([]*dto.WatcherResponse, error) {
	issue, err := s.issueRepo.GetByKey(ctx, strings.ToUpper(issueKey))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("查询工单失败: %w", err)
	}

	watchers, err := s.watcherRepo.ListByIssue(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("查询关注人失败: %w", err)
	}

	responses := make([]*dto.WatcherResponse, 0, len(watchers))
	for _, w := range watchers {
		resp := &dto.WatcherResponse{
			ID:      w.ID,
			IssueID: w.IssueID,
			UserID:  w.UserID,
		}
		if user, err := s.userRepo.GetByID(ctx, w.UserID); err == nil {
			resp.User = &dto.UserBrief{
				ID:          user.ID,
				Username:    user.Username,
				DisplayName: user.DisplayName,
				AvatarURL:   user.AvatarURL,
			}
		}
		responses = append(responses, resp)
	}

	return responses, nil
}

// mentionRegex 匹配 @username 格式的提及
var mentionRegex = regexp.MustCompile(`@(\w+)`)

// extractMentions 从文本中提取 @提及的用户名
func extractMentions(content string) []string {
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var usernames []string
	for _, match := range matches {
		if len(match) >= 2 && !seen[match[1]] {
			seen[match[1]] = true
			usernames = append(usernames, match[1])
		}
	}
	return usernames
}

// relatedCache 批量预加载的关联数据缓存
type relatedCache struct {
	users      map[uint64]*model.User
	issueTypes map[uint64]*model.IssueType
	issues     map[uint64]*model.Issue // Epic/Parent 关联工单
	projects   map[uint64]*model.Project
}

// batchPreload 批量预加载工单列表的关联数据，消除 N+1 查询
func (s *issueService) batchPreload(ctx context.Context, issues []*model.Issue) *relatedCache {
	cache := &relatedCache{
		users:      make(map[uint64]*model.User),
		issueTypes: make(map[uint64]*model.IssueType),
		issues:     make(map[uint64]*model.Issue),
		projects:   make(map[uint64]*model.Project),
	}

	if len(issues) == 0 {
		return cache
	}

	// 收集所有需要的 ID
	userIDSet := make(map[uint64]struct{})
	issueTypeIDSet := make(map[uint64]struct{})
	relatedIssueIDSet := make(map[uint64]struct{})
	projectIDSet := make(map[uint64]struct{})

	for _, issue := range issues {
		userIDSet[issue.ReporterID] = struct{}{}
		if issue.AssigneeID != nil {
			userIDSet[*issue.AssigneeID] = struct{}{}
		}
		issueTypeIDSet[issue.IssueTypeID] = struct{}{}
		if issue.EpicID != nil {
			relatedIssueIDSet[*issue.EpicID] = struct{}{}
		}
		if issue.ParentID != nil {
			relatedIssueIDSet[*issue.ParentID] = struct{}{}
		}
		if issue.MergedIntoIssueID != nil {
			relatedIssueIDSet[*issue.MergedIntoIssueID] = struct{}{}
		}
		projectIDSet[issue.ProjectID] = struct{}{}
	}

	// 批量加载用户
	userIDs := mapKeysToSlice(userIDSet)
	if len(userIDs) > 0 {
		if users, err := s.userRepo.GetByIDs(ctx, userIDs); err == nil {
			for _, u := range users {
				cache.users[u.ID] = u
			}
		}
	}

	// 批量加载工单类型
	issueTypeIDs := mapKeysToSlice(issueTypeIDSet)
	if len(issueTypeIDs) > 0 {
		if types, err := s.issueTypeRepo.GetByIDs(ctx, issueTypeIDs); err == nil {
			for _, t := range types {
				cache.issueTypes[t.ID] = t
			}
		}
	}

	// 批量加载关联工单（Epic/Parent）
	relatedIssueIDs := mapKeysToSlice(relatedIssueIDSet)
	if len(relatedIssueIDs) > 0 {
		if relIssues, err := s.issueRepo.ListByIDs(ctx, relatedIssueIDs); err == nil {
			for _, ri := range relIssues {
				cache.issues[ri.ID] = ri
			}
		}
	}

	// 批量加载项目
	projectIDs := mapKeysToSlice(projectIDSet)
	for _, pid := range projectIDs {
		if p, err := s.projectRepo.GetByID(ctx, pid); err == nil {
			cache.projects[p.ID] = p
		}
	}

	return cache
}

// mapKeysToSlice 将 map 的 key 转为 slice
func mapKeysToSlice(m map[uint64]struct{}) []uint64 {
	s := make([]uint64, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
}

// toIssueResponseCached 使用预加载缓存转换工单（无额外 DB 查询）
func (s *issueService) toIssueResponseCached(issue *model.Issue, cache *relatedCache) *dto.IssueResponse {
	projectKey := ""
	if p, ok := cache.projects[issue.ProjectID]; ok {
		projectKey = p.ProjectKey
	}

	resp := &dto.IssueResponse{
		ID:               issue.ID,
		IssueKey:         issue.IssueKey,
		ProjectID:        issue.ProjectID,
		ProjectKey:       projectKey,
		IssueTypeID:      issue.IssueTypeID,
		Title:            issue.Title,
		Description:      issue.Description,
		Priority:         issue.Priority,
		Status:           issue.Status,
		Resolution:       issue.Resolution,
		ReporterID:       issue.ReporterID,
		AssigneeID:       issue.AssigneeID,
		ParentID:         issue.ParentID,
		EpicID:           issue.EpicID,
		DueDate:          issue.DueDate,
		PlannedStartDate: issue.PlannedStartDate,
		PlannedEndDate:   issue.PlannedEndDate,
		ActualStartDate:  issue.ActualStartDate,
		ActualEndDate:    issue.ActualEndDate,
		ResolvedAt:       issue.ResolvedAt,
		ClosedAt:         issue.ClosedAt,
		CreatedAt:        issue.CreatedAt,
		UpdatedAt:        issue.UpdatedAt,
	}

	// 从缓存加载 Epic Key
	if issue.EpicID != nil {
		if epic, ok := cache.issues[*issue.EpicID]; ok {
			resp.EpicKey = epic.IssueKey
			resp.EpicTitle = epic.Title
		}
	}

	// 从缓存加载工单类型
	if it, ok := cache.issueTypes[issue.IssueTypeID]; ok {
		resp.IssueType = &dto.IssueTypeBrief{
			ID:          it.ID,
			Name:        it.Name,
			DisplayName: it.DisplayName,
			Icon:        it.Icon,
			Color:       it.Color,
		}
	}

	// 从缓存加载报告人
	if reporter, ok := cache.users[issue.ReporterID]; ok {
		resp.Reporter = &dto.UserBrief{
			ID:          reporter.ID,
			Username:    reporter.Username,
			DisplayName: reporter.DisplayName,
			AvatarURL:   reporter.AvatarURL,
		}
	}

	// 从缓存加载指派人
	if issue.AssigneeID != nil {
		if assignee, ok := cache.users[*issue.AssigneeID]; ok {
			resp.Assignee = &dto.UserBrief{
				ID:          assignee.ID,
				Username:    assignee.Username,
				DisplayName: assignee.DisplayName,
				AvatarURL:   assignee.AvatarURL,
			}
		}
	}

	// 从缓存加载父工单 Key
	if issue.ParentID != nil {
		if parent, ok := cache.issues[*issue.ParentID]; ok {
			resp.ParentKey = parent.IssueKey
		}
	}

	// 从缓存加载合并目标工单 Key
	if issue.MergedIntoIssueID != nil {
		resp.MergedIntoIssueID = issue.MergedIntoIssueID
		if target, ok := cache.issues[*issue.MergedIntoIssueID]; ok {
			resp.MergedIntoIssueKey = target.IssueKey
		}
	}

	return resp
}

// batchToIssueResponses 批量转换工单列表为响应（预加载 + 零额外查询）
func (s *issueService) batchToIssueResponses(ctx context.Context, issues []*model.Issue) []*dto.IssueResponse {
	cache := s.batchPreload(ctx, issues)
	responses := make([]*dto.IssueResponse, len(issues))
	for i, issue := range issues {
		responses[i] = s.toIssueResponseCached(issue, cache)
	}
	return responses
}

// toIssueResponse 将工单模型转换为响应 DTO
func (s *issueService) toIssueResponse(ctx context.Context, issue *model.Issue, projectKey string) *dto.IssueResponse {
	resp := &dto.IssueResponse{
		ID:               issue.ID,
		IssueKey:         issue.IssueKey,
		ProjectID:        issue.ProjectID,
		ProjectKey:       projectKey,
		IssueTypeID:      issue.IssueTypeID,
		Title:            issue.Title,
		Description:      issue.Description,
		Priority:         issue.Priority,
		Status:           issue.Status,
		Resolution:       issue.Resolution,
		ReporterID:       issue.ReporterID,
		AssigneeID:       issue.AssigneeID,
		ParentID:         issue.ParentID,
		EpicID:           issue.EpicID,
		DueDate:          issue.DueDate,
		PlannedStartDate: issue.PlannedStartDate,
		PlannedEndDate:   issue.PlannedEndDate,
		ActualStartDate:  issue.ActualStartDate,
		ActualEndDate:    issue.ActualEndDate,
		ResolvedAt:       issue.ResolvedAt,
		ClosedAt:         issue.ClosedAt,
		CreatedAt:        issue.CreatedAt,
		UpdatedAt:        issue.UpdatedAt,
	}

	// 加载 Epic Key
	if issue.EpicID != nil {
		if epicIssue, err := s.issueRepo.GetByID(ctx, *issue.EpicID); err == nil {
			resp.EpicKey = epicIssue.IssueKey
			resp.EpicTitle = epicIssue.Title
		}
	}

	// 加载工单类型
	if issueType, err := s.issueTypeRepo.GetByID(ctx, issue.IssueTypeID); err == nil {
		resp.IssueType = &dto.IssueTypeBrief{
			ID:          issueType.ID,
			Name:        issueType.Name,
			DisplayName: issueType.DisplayName,
			Icon:        issueType.Icon,
			Color:       issueType.Color,
		}
	}

	// 加载报告人
	if reporter, err := s.userRepo.GetByID(ctx, issue.ReporterID); err == nil {
		resp.Reporter = &dto.UserBrief{
			ID:          reporter.ID,
			Username:    reporter.Username,
			DisplayName: reporter.DisplayName,
			AvatarURL:   reporter.AvatarURL,
		}
	}

	// 加载指派人
	if issue.AssigneeID != nil {
		if assignee, err := s.userRepo.GetByID(ctx, *issue.AssigneeID); err == nil {
			resp.Assignee = &dto.UserBrief{
				ID:          assignee.ID,
				Username:    assignee.Username,
				DisplayName: assignee.DisplayName,
				AvatarURL:   assignee.AvatarURL,
			}
		}
	}

	// 加载父工单 Key
	if issue.ParentID != nil {
		if parent, err := s.issueRepo.GetByID(ctx, *issue.ParentID); err == nil {
			resp.ParentKey = parent.IssueKey
		}
	}

	// 加载合并关联
	if issue.MergedIntoIssueID != nil {
		resp.MergedIntoIssueID = issue.MergedIntoIssueID
		if target, err := s.issueRepo.GetByID(ctx, *issue.MergedIntoIssueID); err == nil {
			resp.MergedIntoIssueKey = target.IssueKey
		}
	}

	// 加载被合并来源（当前工单作为合并目标时，查找所有指向它的旧工单）
	if issue.Status != "merged" {
		var mergedFrom []model.Issue
		if err := s.issueRepo.ListByMergedIntoIssueID(ctx, issue.ID, &mergedFrom); err == nil && len(mergedFrom) > 0 {
			resp.MergedFromIssueKeys = make([]string, len(mergedFrom))
			for i := range mergedFrom {
				resp.MergedFromIssueKeys[i] = mergedFrom[i].IssueKey
			}
		}
	}

	return resp
}

// toCommentResponse 将评论模型转换为响应 DTO
func (s *issueService) toCommentResponse(ctx context.Context, comment *model.IssueComment) *dto.CommentResponse {
	resp := &dto.CommentResponse{
		ID:        comment.ID,
		IssueID:   comment.IssueID,
		UserID:    comment.UserID,
		Content:   comment.Content,
		CreatedAt: comment.CreatedAt,
		UpdatedAt: comment.UpdatedAt,
	}

	if user, err := s.userRepo.GetByID(ctx, comment.UserID); err == nil {
		resp.User = &dto.UserBrief{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
		}
	}

	return resp
}

// ============ 工作日志相关方法 ============

// AddWorklog 添加工作日志
func (s *issueService) AddWorklog(ctx context.Context, issueKey string, req *dto.CreateWorklogRequest, userID uint64) (*dto.WorklogResponse, error) {
	// 获取工单
	issue, err := s.issueRepo.GetByKey(ctx, issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	// 解析时间字符串
	timeSpentSec, err := parseTimeSpent(req.TimeSpent)
	if err != nil {
		return nil, err
	}

	// 解析工作日期
	workedAt, err := time.Parse(time.RFC3339, req.WorkedAt)
	if err != nil {
		// 直接返回哨兵，不做 fmt.Errorf 包裹：哨兵携带的是语言包 key，
		// 一旦被包进更长的字符串，响应层就认不出这是 key，
		// 用户会看到裸的 "issue.bad_time_format"。具体格式问题进日志即可
		logger.Warn("invalid worked_at format", zap.String("value", req.WorkedAt), zap.Error(err))
		return nil, ErrInvalidTimeFormat
	}

	// 创建工作日志
	worklog := &model.IssueWorklog{
		IssueID:      issue.ID,
		UserID:       userID,
		Description:  req.Description,
		TimeSpent:    req.TimeSpent,
		TimeSpentSec: timeSpentSec,
		WorkedAt:     workedAt,
		WorkType:     req.WorkType,
	}

	if err := s.worklogRepo.Create(ctx, worklog); err != nil {
		return nil, fmt.Errorf("failed to create worklog: %w", err)
	}

	// 记录活动日志
	if s.activityLogger != nil {
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "Unknown"
		if user != nil {
			userName = user.DisplayName
		}
		details := detail.New("activity.detail.worklogAdded", "time", req.TimeSpent)
		_ = s.activityLogger.LogActivity(ctx, userID, userName, "worklog_added", "issue", issue.ID, issue.IssueKey, details)
	}

	// 通知关注者
	if s.notifSender != nil {
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "Unknown"
		if user != nil {
			userName = user.DisplayName
		}
		s.notifyWatchers(issue, userID, userID, userName, NotificationRequest{
			Type:        model.NotificationTypeIssueUpdated,
			TitleKey:    "inbox.worklog_added",
			TitleArgs:   []any{issue.IssueKey},
			ContentKey:  "inbox.worklog_added_by",
			ContentArgs: []any{userName, req.TimeSpent},
		})
	}

	return s.toWorklogResponse(ctx, worklog), nil
}

// UpdateWorklog 更新工作日志
func (s *issueService) UpdateWorklog(ctx context.Context, worklogID uint64, req *dto.UpdateWorklogRequest, userID uint64) (*dto.WorklogResponse, error) {
	// 获取工作日志
	worklog, err := s.worklogRepo.GetByID(ctx, worklogID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorklogNotFound
		}
		return nil, fmt.Errorf("failed to get worklog: %w", err)
	}

	// 权限检查：只有创建者可以编辑
	if worklog.UserID != userID {
		return nil, ErrUnauthorized
	}

	// 解析时间字符串
	timeSpentSec, err := parseTimeSpent(req.TimeSpent)
	if err != nil {
		return nil, err
	}

	// 解析工作日期
	workedAt, err := time.Parse(time.RFC3339, req.WorkedAt)
	if err != nil {
		// 直接返回哨兵，不做 fmt.Errorf 包裹：哨兵携带的是语言包 key，
		// 一旦被包进更长的字符串，响应层就认不出这是 key，
		// 用户会看到裸的 "issue.bad_time_format"。具体格式问题进日志即可
		logger.Warn("invalid worked_at format", zap.String("value", req.WorkedAt), zap.Error(err))
		return nil, ErrInvalidTimeFormat
	}

	// 更新字段
	worklog.Description = req.Description
	worklog.TimeSpent = req.TimeSpent
	worklog.TimeSpentSec = timeSpentSec
	worklog.WorkedAt = workedAt
	worklog.WorkType = req.WorkType

	if err := s.worklogRepo.Update(ctx, worklog); err != nil {
		return nil, fmt.Errorf("failed to update worklog: %w", err)
	}

	// 记录活动日志
	if s.activityLogger != nil {
		issue, _ := s.issueRepo.GetByID(ctx, worklog.IssueID)
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "Unknown"
		issueKey := ""
		if user != nil {
			userName = user.DisplayName
		}
		if issue != nil {
			issueKey = issue.IssueKey
		}
		details := detail.New("activity.detail.worklogUpdated", "time", req.TimeSpent)
		_ = s.activityLogger.LogActivity(ctx, userID, userName, "worklog_updated", "issue", worklog.IssueID, issueKey, details)
	}

	return s.toWorklogResponse(ctx, worklog), nil
}

// DeleteWorklog 删除工作日志
func (s *issueService) DeleteWorklog(ctx context.Context, worklogID, userID uint64) error {
	// 获取工作日志
	worklog, err := s.worklogRepo.GetByID(ctx, worklogID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWorklogNotFound
		}
		return fmt.Errorf("failed to get worklog: %w", err)
	}

	// 权限检查：只有创建者可以删除
	if worklog.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.worklogRepo.Delete(ctx, worklogID); err != nil {
		return fmt.Errorf("failed to delete worklog: %w", err)
	}

	// 记录活动日志
	if s.activityLogger != nil {
		issue, _ := s.issueRepo.GetByID(ctx, worklog.IssueID)
		user, _ := s.userRepo.GetByID(ctx, userID)
		userName := "Unknown"
		issueKey := ""
		if user != nil {
			userName = user.DisplayName
		}
		if issue != nil {
			issueKey = issue.IssueKey
		}
		details := detail.New("activity.detail.worklogDeleted", "time", worklog.TimeSpent)
		_ = s.activityLogger.LogActivity(ctx, userID, userName, "worklog_deleted", "issue", worklog.IssueID, issueKey, details)
	}

	return nil
}

// ListWorklogs 获取工单的工作日志列表
func (s *issueService) ListWorklogs(ctx context.Context, issueKey string) ([]*dto.WorklogResponse, error) {
	// 获取工单
	issue, err := s.issueRepo.GetByKey(ctx, issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrIssueNotFound
		}
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	// 获取工作日志列表
	worklogs, err := s.worklogRepo.ListByIssue(ctx, issue.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list worklogs: %w", err)
	}

	// 转换为响应 DTO
	responses := make([]*dto.WorklogResponse, 0, len(worklogs))
	for _, worklog := range worklogs {
		responses = append(responses, s.toWorklogResponse(ctx, worklog))
	}

	return responses, nil
}

// parseTimeSpent 解析时间字符串（如 "2h 30m"）为秒数
func parseTimeSpent(timeStr string) (int, error) {
	timeStr = strings.TrimSpace(timeStr)
	if timeStr == "" {
		return 0, ErrInvalidTimeFormat
	}

	// 支持格式：1d, 2h, 30m, 1d 2h, 2h 30m, 1d 2h 30m
	// 1d = 8小时（工作日）
	// 1h = 3600秒
	// 1m = 60秒

	totalSeconds := 0

	// 匹配天数
	dayRegex := regexp.MustCompile(`(\d+)d`)
	if matches := dayRegex.FindStringSubmatch(timeStr); len(matches) > 1 {
		days := 0
		_, _ = fmt.Sscanf(matches[1], "%d", &days)
		totalSeconds += days * 8 * 3600 // 1天 = 8小时
	}

	// 匹配小时
	hourRegex := regexp.MustCompile(`(\d+)h`)
	if matches := hourRegex.FindStringSubmatch(timeStr); len(matches) > 1 {
		hours := 0
		_, _ = fmt.Sscanf(matches[1], "%d", &hours)
		totalSeconds += hours * 3600
	}

	// 匹配分钟
	minRegex := regexp.MustCompile(`(\d+)m`)
	if matches := minRegex.FindStringSubmatch(timeStr); len(matches) > 1 {
		minutes := 0
		_, _ = fmt.Sscanf(matches[1], "%d", &minutes)
		totalSeconds += minutes * 60
	}

	if totalSeconds == 0 {
		logger.Warn("unparseable time_spent", zap.String("value", timeStr))
		return 0, ErrInvalidTimeFormat
	}

	return totalSeconds, nil
}

// toWorklogResponse 将工作日志模型转换为响应 DTO
func (s *issueService) toWorklogResponse(ctx context.Context, worklog *model.IssueWorklog) *dto.WorklogResponse {
	resp := &dto.WorklogResponse{
		ID:           worklog.ID,
		IssueID:      worklog.IssueID,
		UserID:       worklog.UserID,
		Description:  worklog.Description,
		TimeSpent:    worklog.TimeSpent,
		TimeSpentSec: worklog.TimeSpentSec,
		WorkedAt:     worklog.WorkedAt,
		WorkType:     worklog.WorkType,
		CreatedAt:    worklog.CreatedAt,
		UpdatedAt:    worklog.UpdatedAt,
	}

	if user, err := s.userRepo.GetByID(ctx, worklog.UserID); err == nil {
		resp.User = &dto.UserBrief{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
			AvatarURL:   user.AvatarURL,
		}
	}

	return resp
}
