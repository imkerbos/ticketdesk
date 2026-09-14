// Package router 提供路由配置
package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
	"gorm.io/gorm"

	_ "github.com/kerbos/ticketdesk/docs/swagger" // Swagger 文档 (由 make swagger 生成)
	activityHandler "github.com/kerbos/ticketdesk/internal/activity/handler"
	activityRepo "github.com/kerbos/ticketdesk/internal/activity/repository"
	activityService "github.com/kerbos/ticketdesk/internal/activity/service"
	"github.com/kerbos/ticketdesk/internal/api/middleware"
	"github.com/kerbos/ticketdesk/internal/api/response"
	fieldHandler "github.com/kerbos/ticketdesk/internal/core-field/handler"
	fieldRepo "github.com/kerbos/ticketdesk/internal/core-field/repository"
	fieldService "github.com/kerbos/ticketdesk/internal/core-field/service"
	issueHandler "github.com/kerbos/ticketdesk/internal/core-issue/handler"
	issueRepo "github.com/kerbos/ticketdesk/internal/core-issue/repository"
	issueService "github.com/kerbos/ticketdesk/internal/core-issue/service"
	projectHandler "github.com/kerbos/ticketdesk/internal/core-project/handler"
	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	projectService "github.com/kerbos/ticketdesk/internal/core-project/service"
	userHandler "github.com/kerbos/ticketdesk/internal/core-user/handler"
	userRepo "github.com/kerbos/ticketdesk/internal/core-user/repository"
	userService "github.com/kerbos/ticketdesk/internal/core-user/service"
	workflowHandler "github.com/kerbos/ticketdesk/internal/core-workflow/handler"
	workflowRepo "github.com/kerbos/ticketdesk/internal/core-workflow/repository"
	workflowService "github.com/kerbos/ticketdesk/internal/core-workflow/service"
	alertHandler "github.com/kerbos/ticketdesk/internal/integration-alert/handler"
	alertRepo "github.com/kerbos/ticketdesk/internal/integration-alert/repository"
	alertService "github.com/kerbos/ticketdesk/internal/integration-alert/service"
	notifDto "github.com/kerbos/ticketdesk/internal/notification-inbox/dto"
	notifHandler "github.com/kerbos/ticketdesk/internal/notification-inbox/handler"
	notifRepo "github.com/kerbos/ticketdesk/internal/notification-inbox/repository"
	notifService "github.com/kerbos/ticketdesk/internal/notification-inbox/service"
	ws "github.com/kerbos/ticketdesk/internal/notification-inbox/websocket"
	emailService "github.com/kerbos/ticketdesk/internal/notification/email"
	reportHandler "github.com/kerbos/ticketdesk/internal/reporting/handler"
	reportRepo "github.com/kerbos/ticketdesk/internal/reporting/repository"
	reportService "github.com/kerbos/ticketdesk/internal/reporting/service"
	reqPoolHandler "github.com/kerbos/ticketdesk/internal/requirement-pool/handler"
	reqPoolRepo "github.com/kerbos/ticketdesk/internal/requirement-pool/repository"
	reqPoolService "github.com/kerbos/ticketdesk/internal/requirement-pool/service"
	"github.com/kerbos/ticketdesk/internal/scheduler"
	setupHandler "github.com/kerbos/ticketdesk/internal/setup/handler"
	setupService "github.com/kerbos/ticketdesk/internal/setup/service"
	configHandler "github.com/kerbos/ticketdesk/internal/system-config/handler"
	configRepo "github.com/kerbos/ticketdesk/internal/system-config/repository"
	configService "github.com/kerbos/ticketdesk/internal/system-config/service"
	"github.com/kerbos/ticketdesk/pkg/config"
	"github.com/kerbos/ticketdesk/pkg/jwt"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
	"github.com/kerbos/ticketdesk/pkg/storage"
)

// Router 路由管理器
type Router struct {
	config                 *config.Config
	jwtManager             *jwt.Manager
	db                     *gorm.DB
	logger                 *zap.Logger
	userHandler            *userHandler.UserHandler
	projectHandler         *projectHandler.ProjectHandler
	issueHandler           *issueHandler.IssueHandler
	attachmentHandler      *issueHandler.AttachmentHandler
	workflowHandler        *workflowHandler.WorkflowHandler
	alertHandler           *alertHandler.AlertHandler
	configHandler          *configHandler.ConfigHandler
	reportHandler          *reportHandler.ReportHandler
	activityHandler        *activityHandler.ActivityHandler
	notificationHandler    *notifHandler.NotificationHandler
	wsHandler              *notifHandler.WebSocketHandler
	requirementPoolHandler *reqPoolHandler.RequirementPoolHandler
	requirementHandler     *reqPoolHandler.RequirementHandler
	categoryHandler        *reqPoolHandler.CategoryHandler
	fieldHandler           *fieldHandler.FieldHandler
	rbac                   *middleware.RBACMiddleware
	permChecker            middleware.ProjectPermissionChecker
	memberLister           middleware.MemberProjectLister
	datasourceHandler      *alertHandler.DatasourceHandler
	datasourceService      alertService.DatasourceService
	notifChannelHandler    *projectHandler.NotificationChannelHandler
	digestHandler          *projectHandler.DigestHandler
	configSvc              configService.ConfigService
	ssoHandler             *userHandler.SSOHandler
	apiTokenHandler        *userHandler.APITokenHandler
	apiTokenSvc            userService.APITokenService
	userSvc                userService.UserService
	fileStorage            storage.Storage
	setupSvc               setupService.Service
	setupHandler           *setupHandler.SetupHandler
}

// NewRouter 创建路由管理器
func NewRouter(cfg *config.Config, jwtManager *jwt.Manager, db *gorm.DB) *Router {
	// ============ 初始化 System Config 模块（需要先初始化，因为邮件服务依赖它）============
	configRepository := configRepo.NewConfigRepository(db)
	webhookRepository := configRepo.NewWebhookRepository(db)
	webhookLogRepository := configRepo.NewWebhookLogRepository(db)
	configSvc := configService.NewConfigService(
		configRepository,
		webhookRepository,
		webhookLogRepository,
	)
	configHdl := configHandler.NewConfigHandler(configSvc)

	// ============ 初始化邮件服务 ============
	emailSvc := emailService.NewEmailService(configSvc)

	// ============ 初始化 User 模块 ============
	userRepository := userRepo.NewUserRepository(db)
	userRoleRepository := userRepo.NewUserRoleRepository(db)
	userSvc := userService.NewUserService(userRepository, userRoleRepository, jwtManager, emailSvc, configSvc, db)
	mfaSvc := userService.NewMFAService(userRepository)
	// MFA 校验器反向注入：登录第二步由 userSvc 统一签发令牌，
	// 需要它能调用 TOTP 校验，同时避免两个 service 构造顺序上的循环依赖
	userSvc.SetMFAVerifier(mfaSvc)
	userHdl := userHandler.NewUserHandler(userSvc, mfaSvc)

	// ============ 初始化 SSO 模块（仅在启用时创建完整的 service/handler）============
	var ssoHdl *userHandler.SSOHandler
	ssoSvc := userService.NewSSOService(configSvc, userRepository, userRoleRepository, jwtManager)
	ssoHdl = userHandler.NewSSOHandler(ssoSvc)

	// ============ 初始化 API Token 模块 ============
	apiTokenRepo := userRepo.NewAPITokenRepository(db)
	apiTokenSvc := userService.NewAPITokenService(apiTokenRepo, userRepository)
	apiTokenHdl := userHandler.NewAPITokenHandler(apiTokenSvc)

	// ============ 初始化 Project 模块 ============
	projectRepository := projectRepo.NewProjectRepository(db)
	projectMemberRepository := projectRepo.NewProjectMemberRepository(db)
	issueTypeRepository := projectRepo.NewIssueTypeRepository(db)
	projectRoleRepository := projectRepo.NewProjectRoleRepository(db)
	projectRoleMemberRepository := projectRepo.NewProjectRoleMemberRepository(db)
	projectRolePermissionRepository := projectRepo.NewProjectRolePermissionRepository(db)
	projectSvc := projectService.NewProjectService(
		projectRepository,
		projectMemberRepository,
		issueTypeRepository,
		projectRoleRepository,
		projectRoleMemberRepository,
		projectRolePermissionRepository,
		userRepository,
		db,
	)
	projectHdl := projectHandler.NewProjectHandler(projectSvc)

	// ============ 初始化 Issue 模块 ============
	issueRepository := issueRepo.NewIssueRepository(db)
	commentRepository := issueRepo.NewCommentRepository(db)
	watcherRepository := issueRepo.NewWatcherRepository(db)
	worklogRepository := issueRepo.NewWorklogRepository(db)
	issueSvc := issueService.NewIssueService(
		issueRepository,
		commentRepository,
		watcherRepository,
		worklogRepository,
		projectRepository,
		issueTypeRepository,
		userRepository,
		db,
	)
	issueHdl := issueHandler.NewIssueHandler(issueSvc)
	issueHdl.SetPermissionChecker(projectSvc)

	// ============ 初始化 Attachment 模块 ============
	attachmentRepository := issueRepo.NewAttachmentRepository(db)
	// 按配置选择存储驱动：local（单副本 + PVC）或 s3（对象存储，多副本必需）
	fileStorage, err := storage.New(cfg.Storage)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize file storage: %v", err))
	}
	logger.Info("file storage initialized", zap.String("driver", fileStorage.Driver()))
	attachmentSvc := issueService.NewAttachmentService(
		attachmentRepository,
		issueRepository,
		userRepository,
		fileStorage,
	)
	attachmentHdl := issueHandler.NewAttachmentHandler(attachmentSvc)

	// 给 issueSvc 注入附件服务以支持创建工单时原子上传附件
	if issueImpl, ok := issueSvc.(interface {
		SetAttachmentService(svc issueService.AttachmentService)
	}); ok {
		issueImpl.SetAttachmentService(attachmentSvc)
	}

	// 注入 LocalStorage 到 ConfigHandler（品牌资源上传）
	configHdl.SetStorage(fileStorage)

	// ============ 初始化 Workflow 模块 ============
	workflowRepository := workflowRepo.NewWorkflowRepository(db)
	nodeRepository := workflowRepo.NewNodeRepository(db)
	edgeRepository := workflowRepo.NewEdgeRepository(db)
	workflowInstanceRepository := workflowRepo.NewWorkflowInstanceRepository(db)
	workflowHistoryRepository := workflowRepo.NewWorkflowHistoryRepository(db)
	approvalRecordRepository := workflowRepo.NewApprovalRecordRepository(db)
	workflowSchemeRepository := workflowRepo.NewWorkflowSchemeRepository(db)

	workflowSvc := workflowService.NewWorkflowService(
		workflowRepository,
		nodeRepository,
		edgeRepository,
		workflowSchemeRepository,
		issueTypeRepository,
	)

	workflowEngine := workflowService.NewWorkflowEngine(
		workflowInstanceRepository,
		workflowHistoryRepository,
		approvalRecordRepository,
		workflowRepository,
		nodeRepository,
		edgeRepository,
		workflowSchemeRepository,
		projectRoleRepository,
		userRepository,
		db,
	)

	// ============ 初始化 Activity 模块（提前初始化，因为 WorkflowHandler 依赖它）============
	activityRepository := activityRepo.NewActivityRepository(db)
	activitySvc := activityService.NewActivityService(activityRepository)
	activityHdl := activityHandler.NewActivityHandler(activitySvc)

	workflowHdl := workflowHandler.NewWorkflowHandler(workflowSvc, workflowEngine, issueRepository, projectRepository, activitySvc)

	// ============ 初始化 Alert 模块 ============
	alertRepository := alertRepo.NewAlertRepository(db)
	alertRuleRepository := alertRepo.NewAlertRuleRepository(db)
	alertSilenceRepository := alertRepo.NewAlertSilenceRepository(db)
	datasourceRepository := alertRepo.NewAlertDatasourceRepository(db)
	alertSvc := alertService.NewAlertService(
		alertRepository,
		alertRuleRepository,
		alertSilenceRepository,
		datasourceRepository,
		issueRepository,
		commentRepository,
		projectRepository,
		issueTypeRepository,
		watcherRepository,
		db,
	)
	alertHdl := alertHandler.NewAlertHandler(alertSvc)

	// 注入工作流创建器到告警服务（告警建单时自动创建工作流实例）
	if alertSvcImpl, ok := alertSvc.(interface {
		SetWorkflowCreator(alertService.WorkflowCreator)
	}); ok {
		alertSvcImpl.SetWorkflowCreator(workflowEngine)
	}

	// ============ 设置告警同步服务（避免循环依赖）============
	// 将 issueSvc 转换为具体类型以调用 SetAlertSyncService
	if issueServiceImpl, ok := issueSvc.(interface {
		SetAlertSyncService(issueService.AlertSyncService)
	}); ok {
		issueServiceImpl.SetAlertSyncService(alertSvc)
	}

	// 将 alertSvc 注入工作流引擎，用于工单状态变更时同步告警和级联合并工单
	if engineImpl, ok := workflowEngine.(interface {
		SetIssueStatusSyncer(workflowService.IssueStatusSyncer)
	}); ok {
		engineImpl.SetIssueStatusSyncer(alertSvc)
	}

	// 将活动日志记录器注入工作流引擎，用于记录状态变更
	if engineImpl, ok := workflowEngine.(interface {
		SetActivityLogger(workflowService.ActivityLogger)
	}); ok {
		engineImpl.SetActivityLogger(activitySvc)
	}

	// ============ 初始化 Report 模块 ============
	reportRepository := reportRepo.NewReportRepository(db)
	reportSvc := reportService.NewReportService(reportRepository, projectRepository)
	reportHdl := reportHandler.NewReportHandler(reportSvc)

	// ============ 设置活动日志记录器（避免循环依赖）============
	if issueServiceImpl, ok := issueSvc.(interface {
		SetActivityLogger(issueService.ActivityLogger)
	}); ok {
		issueServiceImpl.SetActivityLogger(activitySvc)
	}

	// 设置附件服务的活动日志记录器
	if attachmentServiceImpl, ok := attachmentSvc.(interface {
		SetActivityLogger(issueService.ActivityLogger)
	}); ok {
		attachmentServiceImpl.SetActivityLogger(activitySvc)
	}

	// 设置告警服务的活动日志记录器
	if alertSvcImpl, ok := alertSvc.(interface {
		SetActivityLogger(alertService.ActivityLogger)
	}); ok {
		alertSvcImpl.SetActivityLogger(activitySvc)
	}

	// ============ 初始化 Notification 模块 ============
	wsManager := ws.NewManager()
	safego.Go("websocket.managerLoop", wsManager.Run)
	// 订阅跨副本广播：连接表是进程内的，多副本下用户可能连在另一个 Pod 上，
	// 不广播的话那部分实时通知会静默丢失
	wsManager.StartBroadcastSubscriber(context.Background())

	notificationRepository := notifRepo.NewNotificationRepository(db)
	// 初始化向导：管理员、站点信息、平台语言都写数据库，多副本天然一致
	setupSvc := setupService.NewService(db, configSvc)
	setupSvc.EnsureToken(context.Background())

	notificationSvc := notifService.NewNotificationService(notificationRepository, wsManager, &userLocaleAdapter{repo: userRepository})

	// 平台语言以数据库配置为准，配置文件的 app.language 只是首启兜底
	configService.StartLanguageSync(context.Background(), configSvc)
	notificationHdl := notifHandler.NewNotificationHandler(notificationSvc)
	wsHdl := notifHandler.NewWebSocketHandler(wsManager, jwtManager)

	// ============ 设置通知服务（避免循环依赖）============
	notifAdapter := &notificationAdapter{svc: notificationSvc}
	if issueServiceImpl, ok := issueSvc.(interface {
		SetNotificationService(issueService.NotificationSender)
	}); ok {
		issueServiceImpl.SetNotificationService(notifAdapter)
	}

	// ============ 设置工作流引擎（避免循环依赖）============
	if issueServiceImpl, ok := issueSvc.(interface {
		SetWorkflowEngine(issueService.WorkflowEngine)
	}); ok {
		issueServiceImpl.SetWorkflowEngine(workflowEngine)
	}

	// ============ 初始化需求池模块 ============
	// 初始化 logger（如果还没有的话）
	logger, _ := zap.NewProduction()

	reqPoolRepository := reqPoolRepo.NewRequirementPoolRepository(db)
	reqRepository := reqPoolRepo.NewRequirementRepository(db)

	reqPoolSvc := reqPoolService.NewRequirementPoolService(
		reqPoolRepository,
		reqRepository,
		logger,
	)
	reqSvc := reqPoolService.NewRequirementService(
		reqRepository,
		reqPoolRepository,
		db,
		logger,
	)

	// 注入 IssueService 到 RequirementService（setter 注入，避免循环依赖）
	if reqSvcImpl, ok := reqSvc.(interface {
		SetIssueCreator(reqPoolService.IssueCreator)
	}); ok {
		reqSvcImpl.SetIssueCreator(issueSvc)
	}

	reqPoolHdl := reqPoolHandler.NewRequirementPoolHandler(reqPoolSvc, logger)
	reqHdl := reqPoolHandler.NewRequirementHandler(reqSvc, logger)

	// 初始化需求分类管理
	catRepository := reqPoolRepo.NewCategoryRepository(db)
	catSvc := reqPoolService.NewCategoryService(catRepository, logger)
	catHdl := reqPoolHandler.NewCategoryHandler(catSvc, logger)

	// ============ 初始化 Field 模块 ============
	fieldRepository := fieldRepo.NewFieldRepository(db)
	schemeRepository := fieldRepo.NewSchemeRepository(db)
	valueRepository := fieldRepo.NewValueRepository(db)
	templateRepository := fieldRepo.NewTemplateRepository(db)
	versionRepository := fieldRepo.NewVersionRepository(db)
	componentRepository := fieldRepo.NewComponentRepository(db)
	labelRepository := fieldRepo.NewLabelRepository(db)
	fieldSvc := fieldService.NewFieldService(
		fieldRepository,
		schemeRepository,
		valueRepository,
		templateRepository,
		versionRepository,
		componentRepository,
		labelRepository,
		projectRepository,
		issueTypeRepository,
		userRepository,
		db,
	)
	fieldHdl := fieldHandler.NewFieldHandler(fieldSvc)

	// ============ 设置字段值保存服务（避免循环依赖）============
	fieldValueAdapter := &fieldValueSaverAdapter{svc: fieldSvc}
	if issueServiceImpl, ok := issueSvc.(interface {
		SetFieldValueSaver(issueService.FieldValueSaver)
	}); ok {
		issueServiceImpl.SetFieldValueSaver(fieldValueAdapter)
	}

	// 设置 Epic 链接获取服务
	if issueServiceImpl, ok := issueSvc.(interface {
		SetEpicLinkGetter(issueService.EpicLinkGetter)
	}); ok {
		issueServiceImpl.SetEpicLinkGetter(fieldSvc)
	}

	// ============ 初始化项目通知渠道模块 ============
	notifChannelRepo := projectRepo.NewNotificationChannelRepository(db)
	notifChannelSvc := projectService.NewNotificationChannelService(notifChannelRepo, projectRepository, configSvc)
	notifChannelHdl := projectHandler.NewNotificationChannelHandler(notifChannelSvc, projectSvc)

	// ============ 初始化每日日报模块 ============
	digestRunRepo := projectRepo.NewDigestRunRepository(db)
	digestSvc := projectService.NewDigestService(db, projectRepository, notifChannelRepo, digestRunRepo, notifChannelSvc)
	digestHdl := projectHandler.NewDigestHandler(projectSvc, digestSvc)

	// ============ 启动 cron 调度器 + 加载已启用日报项目 ============
	cronSched := scheduler.NewScheduler()
	digestScheduler := projectService.NewDailyDigestScheduler(cronSched, db, digestSvc)
	if err := digestScheduler.LoadAll(context.Background()); err != nil {
		logger.Warn("failed to load daily digest projects", zap.Error(err))
	}
	cronSched.Start()
	// 注入到 projectService，UpdateProject 时同步 Reload
	if projectSvcImpl, ok := projectSvc.(interface {
		SetDigestScheduler(projectService.DigestScheduler)
	}); ok {
		projectSvcImpl.SetDigestScheduler(digestScheduler)
	}

	// ============ 设置项目通知服务（避免循环依赖）============
	if issueServiceImpl, ok := issueSvc.(interface {
		SetProjectNotifier(issueService.ProjectNotifier)
	}); ok {
		issueServiceImpl.SetProjectNotifier(notifChannelSvc)
	}

	// 设置告警服务的项目外部渠道通知
	if alertSvcImpl, ok := alertSvc.(interface {
		SetProjectNotifier(alertService.ProjectNotifier)
	}); ok {
		alertSvcImpl.SetProjectNotifier(notifChannelSvc)
	}

	// 设置工作流引擎的项目外部渠道通知（用于状态变更通知）
	if engineImpl, ok := workflowEngine.(interface {
		SetProjectNotifier(workflowService.ProjectNotifier)
	}); ok {
		engineImpl.SetProjectNotifier(notifChannelSvc)
	}

	// 设置告警服务的站内通知发送器
	alertNotifAdapter := &alertNotificationAdapter{svc: notificationSvc}
	if alertSvcImpl, ok := alertSvc.(interface {
		SetNotificationSender(alertService.NotificationSender)
	}); ok {
		alertSvcImpl.SetNotificationSender(alertNotifAdapter)
	}

	// ============ 初始化 RBAC 中间件 ============
	rbac := middleware.NewRBACMiddleware(userRoleRepository)

	// ============ 初始化数据源服务 ============
	datasourceSvc := alertService.NewDatasourceService(
		datasourceRepository,
		alertRuleRepository,
		alertSvc,
		alertRepository,
		&cfg.Nightingale,
	)
	datasourceHdl := alertHandler.NewDatasourceHandler(datasourceSvc, alertSvc)

	// 启动所有数据源 Poller
	if err := datasourceSvc.StartAllPollers(context.Background()); err != nil {
		logger.Error("failed to start datasource pollers", zap.Error(err))
	}

	return &Router{
		config:                 cfg,
		jwtManager:             jwtManager,
		db:                     db,
		logger:                 logger,
		userHandler:            userHdl,
		projectHandler:         projectHdl,
		issueHandler:           issueHdl,
		attachmentHandler:      attachmentHdl,
		workflowHandler:        workflowHdl,
		alertHandler:           alertHdl,
		configHandler:          configHdl,
		reportHandler:          reportHdl,
		activityHandler:        activityHdl,
		notificationHandler:    notificationHdl,
		wsHandler:              wsHdl,
		requirementPoolHandler: reqPoolHdl,
		requirementHandler:     reqHdl,
		categoryHandler:        catHdl,
		fieldHandler:           fieldHdl,
		rbac:                   rbac,
		permChecker:            projectSvc,
		memberLister:           &memberProjectListerAdapter{memberRepo: projectMemberRepository},
		datasourceHandler:      datasourceHdl,
		datasourceService:      datasourceSvc,
		notifChannelHandler:    notifChannelHdl,
		digestHandler:          digestHdl,
		configSvc:              configSvc,
		ssoHandler:             ssoHdl,
		apiTokenHandler:        apiTokenHdl,
		apiTokenSvc:            apiTokenSvc,
		userSvc:                userSvc,
		fileStorage:            fileStorage,
		setupSvc:               setupSvc,
		setupHandler:           setupHandler.NewSetupHandler(setupSvc),
	}
}

// requirePerm 创建项目权限中间件的快捷方法
func (r *Router) requirePerm(permission string) gin.HandlerFunc {
	return middleware.RequireProjectPermission(r.permChecker, permission)
}

// requireIssuePerm 创建工单权限中间件的快捷方法（从工单 key 提取项目 key）
func (r *Router) requireIssuePerm(permission string) gin.HandlerFunc {
	return middleware.RequireIssuePermission(r.permChecker, permission)
}

// requireIssueListPerm 创建工单列表权限中间件
func (r *Router) requireIssueListPerm() gin.HandlerFunc {
	return middleware.RequireIssueListPermission(r.permChecker, r.memberLister)
}

// StopPollers 停止所有数据源轮询器（用于优雅关闭）
func (r *Router) StopPollers() {
	if r.datasourceService != nil {
		r.datasourceService.StopAllPollers()
	}
}

// Setup 设置路由
func (r *Router) Setup() *gin.Engine {
	// 设置 Gin 模式
	if r.config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// 限定可信反向代理：Gin 默认信任所有代理，会无条件采信 X-Forwarded-For，
	// 导致 c.ClientIP() 可被任意伪造 —— 限流（/auth 20/min、webhook 100/min）形同虚设，
	// 审计日志里的来源 IP 也不可信。
	trusted := r.config.App.EffectiveTrustedProxies()
	if err := engine.SetTrustedProxies(trusted); err != nil {
		logger.Fatal("invalid app.trusted_proxies config", zap.Strings("trusted_proxies", trusted), zap.Error(err))
	}

	// 全局中间件
	// 语言协商要在所有业务中间件之前，否则限流等中间件的报错拿不到语言
	engine.Use(middleware.LocaleMiddleware())
	engine.Use(middleware.RecoveryMiddleware())
	engine.Use(middleware.LoggerMiddleware())
	engine.Use(middleware.CORSMiddleware())
	// 未完成初始化时，除向导与健康检查外一律 503，
	// 避免一个还没有管理员的实例被当成正常实例使用
	engine.Use(middleware.SetupRequired(func(c *gin.Context) bool {
		return r.setupSvc.Initialized(c.Request.Context())
	}))

	// 健康检查
	engine.GET("/health", healthCheck)

	// Swagger UI: /api/v1/swagger/index.html (需登录, Authorization / cookie / query token 三选一)
	// 走 /api 前缀, 复用现有反向代理规则, 无需单独路由
	engine.GET("/api/v1/swagger/*any",
		middleware.SwaggerAuthMiddleware(r.jwtManager, r.apiTokenSvc),
		ginSwagger.WrapHandler(swaggerFiles.Handler),
	)

	// API v1 路由组
	v1 := engine.Group("/api/v1")
	// WebSocket 连接（独立于 API 路由，使用 query 参数认证，不走中间件）
	engine.GET("/ws", r.wsHandler.HandleWebSocket)

	// 注册公开路由
	r.registerPublicRoutes(v1)

	// 注册需要认证的路由
	r.registerProtectedRoutes(v1)

	return engine
}

// healthCheck 健康检查处理器
func healthCheck(c *gin.Context) {
	response.Success(c, gin.H{
		"status": "ok",
		"time":   time.Now().Format("2006-01-02 15:04:05-07:00"),
	})
}

// registerPublicRoutes 注册公开路由（无需认证）
func (r *Router) registerPublicRoutes(rg *gin.RouterGroup) {
	// 初始化向导：完成前公开可访问，靠 setup token 把关；限流同登录接口
	setupGroup := rg.Group("/setup")
	setupGroup.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		KeyPrefix: "rl:setup",
		Limit:     20,
		Window:    1 * time.Minute,
		ConfigKey: "ratelimit.auth_limit",
		ConfigSvc: r.configSvc,
	}))
	{
		setupGroup.GET("/status", r.setupHandler.HandleStatus)
		setupGroup.POST("", r.setupHandler.HandleSetup)
	}

	auth := rg.Group("/auth")
	auth.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		KeyPrefix: "rl:auth",
		Limit:     20,
		Window:    1 * time.Minute,
		ConfigKey: "ratelimit.auth_limit",
		ConfigSvc: r.configSvc,
	}))
	auth.POST("/login", r.userHandler.HandleLogin)
	auth.POST("/register", r.userHandler.HandleRegister)
	auth.POST("/refresh", r.userHandler.HandleRefreshToken)
	auth.POST("/mfa/verify", r.userHandler.HandleVerifyMFA)
	// 忘记密码相关路由
	auth.POST("/forgot-password", r.userHandler.HandleForgotPassword)
	auth.GET("/verify-reset-token", r.userHandler.HandleVerifyResetToken)
	auth.POST("/reset-password", r.userHandler.HandleResetPasswordWithToken)

	// SSO 相关路由
	if r.ssoHandler != nil {
		auth.GET("/sso/config", r.ssoHandler.HandleGetSSOConfig)
		auth.GET("/sso/authorize", r.ssoHandler.HandleSSOAuthorize)
		auth.POST("/sso/callback", r.ssoHandler.HandleSSOCallback)
	}

	// 品牌配置（公开接口，登录页需要）
	rg.GET("/brand", r.configHandler.HandleGetBrandConfig)

	// 品牌资源：走存储抽象层流式下发，而不是挂本地目录。
	//
	// 之所以不用 gin.Static：对象存储驱动下根本没有本地路径可挂。
	// 路由只暴露 brand/ 前缀，绝不能放开到整个存储根 ——
	// attachments/ 下是工单附件，放开等于让附件无需认证即可下载，
	// 绕过 /issues/:key/attachments/:id/download 上的 issue:view 校验。
	//
	// 路径里重复的 /brand 是为了兼容存量配置中已写入的 URL
	// （HandleUploadBrandAsset 存的是 "/api/v1/brand/assets/" + "brand/xxx.svg"），避免数据迁移。
	//
	// 品牌资源允许上传 .svg 且本路由免认证：直接导航打开的 SVG 会在同源下执行脚本，
	// 因此必须带 CSP + nosniff 下发。
	brandAssets := rg.Group("/brand/assets")
	brandAssets.Use(middleware.UploadedAssetSecurityHeaders())
	brandAssets.GET("/brand/*filepath", r.handleBrandAsset)

	// 告警 Webhook（无需认证）
	alerts := rg.Group("/alerts")
	alerts.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		KeyPrefix: "rl:webhook",
		Limit:     100,
		Window:    1 * time.Minute,
		ConfigKey: "ratelimit.webhook_limit",
		ConfigSvc: r.configSvc,
	}))
	// 这些端点对公网开放且不需要认证：先限制请求体大小，再按配置校验 HMAC 签名
	alerts.Use(middleware.BodyLimit(middleware.DefaultWebhookMaxBytes))
	alerts.Use(middleware.WebhookSignature(r.configSvc, configService.KeySecurityWebhookSecret))
	alerts.POST("/webhook", r.alertHandler.HandleWebhook)
	alerts.POST("/nightingale", r.alertHandler.HandleNightingaleWebhook)
	alerts.POST("/datasource/:name/webhook", r.datasourceHandler.HandleDatasourceWebhook)
}

// registerProtectedRoutes 注册需要认证的路由
func (r *Router) registerProtectedRoutes(rg *gin.RouterGroup) {
	protected := rg.Group("")
	protected.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
		KeyPrefix: "rl:api",
		Limit:     300,
		Window:    1 * time.Minute,
		ConfigKey: "ratelimit.api_limit",
		ConfigSvc: r.configSvc,
	}))
	protected.Use(middleware.AuthMiddleware(r.jwtManager, r.apiTokenSvc, r.userSvc))
	protected.Use(r.rbac.LoadUserRoles())

	// Swagger 会话: 已通过 JWT 鉴权, 设 HttpOnly cookie 供后续 swagger UI 资源请求 (doc.json/css/js) 使用
	// 前端 /api-docs 页 iframe 加载前先调此端点
	protected.POST("/auth/swagger-session", func(c *gin.Context) {
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		raw = strings.TrimSpace(raw)
		if raw == "" {
			response.BadRequest(c, "apidoc.need_bearer")
			return
		}
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie(
			"td_swagger_token", // name
			raw,                // value
			7200,               // max-age 2h
			"/api/v1/swagger",  // path
			"",                 // domain (空 = 当前)
			false,              // secure (内网 http 也用; 生产 HTTPS 时浏览器会自动加 Secure 属性)
			true,               // HttpOnly
		)
		response.Success(c, gin.H{"expires_in": 7200})
	})
	// 用户相关
	r.registerUserRoutes(protected)

	// 项目相关
	r.registerProjectRoutes(protected)

	// 工单相关
	r.registerIssueRoutes(protected)

	// 工作流相关
	r.registerWorkflowRoutes(protected)

	// 告警相关
	r.registerAlertRoutes(protected)

	// 报表相关
	r.registerReportRoutes(protected)

	// 活动日志相关
	r.registerActivityRoutes(protected)

	// 系统配置相关
	r.registerConfigRoutes(protected)

	// 通知相关
	r.registerNotificationRoutes(protected)

	// 需求池相关
	r.registerRequirementPoolRoutes(protected)

	// 字段配置相关
	r.registerFieldRoutes(protected)
}

// registerUserRoutes 注册用户路由
func (r *Router) registerUserRoutes(rg *gin.RouterGroup) {
	users := rg.Group("/users")
	// 当前用户相关
	users.GET("/me", r.userHandler.HandleGetCurrentUser)
	users.PUT("/me", r.userHandler.HandleUpdateCurrentUser)
	users.PUT("/me/password", r.userHandler.HandleUpdatePassword)

	// MFA 相关
	users.GET("/me/mfa", r.userHandler.HandleGetMFAStatus)
	users.POST("/me/mfa/setup", r.userHandler.HandleSetupMFA)
	users.POST("/me/mfa/enable", r.userHandler.HandleEnableMFA)
	users.POST("/me/mfa/disable", r.userHandler.HandleDisableMFA)

	// 用户个人 API token 管理（需 JWT 登录，拒绝 PAT 自调）
	userTokens := users.Group("/me/tokens")
	{
		userTokens.POST("", r.apiTokenHandler.HandleCreate)
		userTokens.GET("", r.apiTokenHandler.HandleList)
		userTokens.DELETE("/:id", r.apiTokenHandler.HandleDelete)
	}

	// 获取所有用户（用于选择器，必须在 /:id 之前）
	users.GET("/all", r.userHandler.HandleListAllUsers)

	// 用户管理（需要管理员权限）
	//
	// 列表和详情原来没挂 RequireAdmin：写操作全都挡住了，读却是敞开的，
	// 任何登录用户都能拿到全体成员的邮箱、账号状态、角色、MFA 开关和认证来源。
	// 需要选人的地方走上面的 /all（只返回 id / username / display_name）。
	users.GET("", r.rbac.RequireAdmin(), r.userHandler.HandleListUsers)
	users.POST("", r.rbac.RequireAdmin(), r.userHandler.HandleCreateUser)
	users.GET("/:id", r.rbac.RequireAdmin(), r.userHandler.HandleGetUser)
	users.PUT("/:id", r.rbac.RequireAdmin(), r.userHandler.HandleUpdateUser)
	users.POST("/:id/enable", r.rbac.RequireAdmin(), r.userHandler.HandleEnableUser)
	users.POST("/:id/disable", r.rbac.RequireAdmin(), r.userHandler.HandleDisableUser)
	users.POST("/:id/reset-password", r.rbac.RequireAdmin(), r.userHandler.HandleResetPassword)
	users.DELETE("/:id", r.rbac.RequireAdmin(), r.userHandler.HandleDeleteUser)
}

// registerProjectRoutes 注册项目路由
func (r *Router) registerProjectRoutes(rg *gin.RouterGroup) {
	// 获取所有工单类型（用于筛选器，独立于项目）
	rg.GET("/issue-types", r.projectHandler.HandleListAllIssueTypes)

	projects := rg.Group("/projects")
	{
		// 获取所有项目（用于选择器，必须在 /:key 之前）
		projects.GET("/all", r.projectHandler.HandleListAllProjects)

		// 项目 CRUD
		projects.GET("", r.projectHandler.HandleListProjects)
		projects.POST("", r.rbac.RequireProjectAdmin(), r.projectHandler.HandleCreateProject)
		projects.GET("/:key", r.requirePerm("project:view"), r.projectHandler.HandleGetProject)
		projects.PUT("/:key", r.requirePerm("project:manage"), r.projectHandler.HandleUpdateProject)
		projects.DELETE("/:key", r.rbac.RequireAdmin(), r.projectHandler.HandleDeleteProject)

		// 项目成员管理
		projects.GET("/:key/members", r.requirePerm("member:view"), r.projectHandler.HandleListMembers)
		projects.POST("/:key/members", r.requirePerm("member:manage"), r.projectHandler.HandleAddMember)
		projects.PUT("/:key/members/:user_id", r.requirePerm("member:manage"), r.projectHandler.HandleUpdateMember)
		projects.DELETE("/:key/members/:user_id", r.requirePerm("member:manage"), r.projectHandler.HandleRemoveMember)

		// 工单类型管理
		projects.GET("/:key/issue-types", r.requirePerm("project:view"), r.projectHandler.HandleListIssueTypes)
		projects.POST("/:key/issue-types", r.requirePerm("project:manage"), r.projectHandler.HandleCreateIssueType)
		projects.PUT("/:key/issue-types/:id", r.requirePerm("project:manage"), r.projectHandler.HandleUpdateIssueType)
		projects.DELETE("/:key/issue-types/:id", r.requirePerm("project:manage"), r.projectHandler.HandleDeleteIssueType)

		// 项目角色管理
		projects.GET("/:key/roles", r.requirePerm("role:view"), r.projectHandler.HandleListRoles)
		projects.POST("/:key/roles", r.requirePerm("role:manage"), r.projectHandler.HandleCreateRole)
		projects.PUT("/:key/roles/:id", r.requirePerm("role:manage"), r.projectHandler.HandleUpdateRole)
		projects.DELETE("/:key/roles/:id", r.requirePerm("role:manage"), r.projectHandler.HandleDeleteRole)

		// 角色成员管理
		projects.GET("/:key/roles/:id/members", r.requirePerm("role:view"), r.projectHandler.HandleListRoleMembers)
		projects.POST("/:key/roles/:id/members", r.requirePerm("role:manage"), r.projectHandler.HandleAddRoleMember)
		projects.DELETE("/:key/roles/:id/members/:user_id", r.requirePerm("role:manage"), r.projectHandler.HandleRemoveRoleMember)

		// 角色权限管理
		projects.GET("/:key/roles/:id/permissions", r.requirePerm("role:view"), r.projectHandler.HandleGetRolePermissions)
		projects.PUT("/:key/roles/:id/permissions", r.requirePerm("role:manage"), r.projectHandler.HandleSetRolePermissions)

		// 我在这个项目有哪些权限（前端据此决定显示哪些操作入口）。
		// 不挂 requirePerm：非成员同样要能问出「我没有权限」，挂上就成了先有鸡还是先有蛋。
		projects.GET("/:key/my-permissions", r.projectHandler.HandleGetMyPermissions)

		// 用户角色查询
		projects.GET("/:key/users/:user_id/roles", r.requirePerm("member:view"), r.projectHandler.HandleGetUserRoles)

		// 通知渠道管理
		notifChannels := projects.Group("/:key/notification-channels")
		notifChannels.GET("", r.requirePerm("project:view"), r.notifChannelHandler.HandleListChannels)
		notifChannels.POST("", r.requirePerm("project:manage"), r.notifChannelHandler.HandleCreateChannel)
		notifChannels.GET("/:id", r.requirePerm("project:view"), r.notifChannelHandler.HandleGetChannel)
		notifChannels.PUT("/:id", r.requirePerm("project:manage"), r.notifChannelHandler.HandleUpdateChannel)
		notifChannels.DELETE("/:id", r.requirePerm("project:manage"), r.notifChannelHandler.HandleDeleteChannel)
		notifChannels.POST("/:id/test", r.requirePerm("project:manage"), r.notifChannelHandler.HandleTestChannel)

		// 每日日报手动触发
		projects.POST("/:key/daily-digest/run", r.requirePerm("project:manage"), r.digestHandler.HandleRunDailyDigest)

		// 工作流方案管理
		projects.GET("/:key/workflow-schemes", r.requirePerm("workflow:view"), r.workflowHandler.HandleListSchemes)
		projects.POST("/:key/workflow-schemes", r.requirePerm("workflow:manage"), r.workflowHandler.HandleCreateScheme)
		projects.DELETE("/:key/workflow-schemes/:type_id", r.requirePerm("workflow:manage"), r.workflowHandler.HandleDeleteScheme)
	}
}

// registerIssueRoutes 注册工单路由
func (r *Router) registerIssueRoutes(rg *gin.RouterGroup) {
	issues := rg.Group("/issues")
	// Dashboard 专用（必须放在 /:key 路由之前，避免被匹配为 key）
	issues.GET("/my-todo", r.requireIssueListPerm(), r.issueHandler.HandleListMyTodoIssues)
	issues.GET("/my-created", r.requireIssueListPerm(), r.issueHandler.HandleListMyCreatedIssues)
	issues.GET("/stats", r.requireIssueListPerm(), r.issueHandler.HandleGetIssueListStats)
	issues.GET("/project-overview-stats", r.requireIssueListPerm(), r.issueHandler.HandleGetProjectOverviewStats)

	// 工单列表（通过 query 参数 project_key 检查权限）
	issues.GET("", r.requireIssueListPerm(), r.issueHandler.HandleListIssues)
	issues.POST("", r.issueHandler.HandleCreateIssue)

	// 工单 CRUD（通过工单 key 提取项目 key 检查权限）
	issues.GET("/:key", r.requireIssuePerm("issue:view"), r.issueHandler.HandleGetIssue)
	issues.PUT("/:key", r.requireIssuePerm("issue:edit"), r.issueHandler.HandleUpdateIssue)
	issues.DELETE("/:key", r.requireIssuePerm("issue:delete"), r.issueHandler.HandleDeleteIssue)

	issues.POST("/:key/assign", r.requireIssuePerm("issue:assign"), r.issueHandler.HandleAssignIssue)

	// Epic 相关
	issues.GET("/:key/epic-issues", r.requireIssuePerm("issue:view"), r.issueHandler.HandleListIssuesInEpic)

	// 子任务相关
	issues.GET("/:key/subtasks", r.requireIssuePerm("issue:view"), r.issueHandler.HandleListSubtasks)

	// 评论管理
	issues.GET("/:key/comments", r.requireIssuePerm("issue:view"), r.issueHandler.HandleListComments)
	issues.POST("/:key/comments", r.requireIssuePerm("issue:view"), r.issueHandler.HandleAddComment)
	issues.DELETE("/:key/comments/:comment_id", r.requireIssuePerm("issue:edit"), r.issueHandler.HandleDeleteComment)

	// 关注人管理
	issues.GET("/:key/watchers", r.requireIssuePerm("issue:view"), r.issueHandler.HandleListWatchers)
	issues.POST("/:key/watchers", r.requireIssuePerm("issue:view"), r.issueHandler.HandleAddWatcher)
	issues.DELETE("/:key/watchers/:user_id", r.requireIssuePerm("issue:edit"), r.issueHandler.HandleRemoveWatcher)

	// 工作日志管理
	issues.GET("/:key/worklogs", r.requireIssuePerm("issue:view"), r.issueHandler.HandleListWorklogs)
	issues.POST("/:key/worklogs", r.requireIssuePerm("issue:view"), r.issueHandler.HandleAddWorklog)
	issues.PUT("/:key/worklogs/:worklog_id", r.requireIssuePerm("issue:edit"), r.issueHandler.HandleUpdateWorklog)
	issues.DELETE("/:key/worklogs/:worklog_id", r.requireIssuePerm("issue:edit"), r.issueHandler.HandleDeleteWorklog)

	// 附件管理
	issues.POST("/:key/attachments", r.requireIssuePerm("issue:edit"), r.attachmentHandler.HandleUploadAttachment)
	issues.GET("/:key/attachments", r.requireIssuePerm("issue:view"), r.attachmentHandler.HandleListAttachments)
	issues.DELETE("/:key/attachments/:id", r.requireIssuePerm("issue:edit"), r.attachmentHandler.HandleDeleteAttachment)
	issues.GET("/:key/attachments/:id/download", r.requireIssuePerm("issue:view"), r.attachmentHandler.HandleDownloadAttachment)

	// 工作流实例（通过工单 key 访问）
	issues.GET("/:key/workflow", r.requireIssuePerm("issue:view"), r.workflowHandler.HandleGetInstanceByIssue)
	issues.POST("/:key/workflow/approve", r.requireIssuePerm("issue:edit"), r.workflowHandler.HandleApprove)
	issues.POST("/:key/workflow/reject", r.requireIssuePerm("issue:edit"), r.workflowHandler.HandleReject)
	issues.POST("/:key/workflow/complete", r.requireIssuePerm("issue:edit"), r.workflowHandler.HandleComplete)
	issues.GET("/:key/workflow/history", r.requireIssuePerm("issue:view"), r.workflowHandler.HandleGetHistory)
}

// registerWorkflowRoutes 注册工作流路由
func (r *Router) registerWorkflowRoutes(rg *gin.RouterGroup) {
	workflows := rg.Group("/workflows")

	// 只读接口：认证用户即可访问（工单详情页渲染工作流图需要）
	workflows.GET("", r.workflowHandler.HandleListWorkflows)
	workflows.GET("/:id", r.workflowHandler.HandleGetWorkflow)
	workflows.GET("/:id/nodes", r.workflowHandler.HandleListNodes)
	workflows.GET("/:id/nodes/:node_id", r.workflowHandler.HandleGetNode)
	workflows.GET("/:id/edges", r.workflowHandler.HandleListEdges)
	workflows.GET("/:id/edges/:edge_id", r.workflowHandler.HandleGetEdge)

	// 写操作：需要项目管理员权限
	workflowsAdmin := workflows.Group("")
	workflowsAdmin.Use(r.rbac.RequireProjectAdmin())
	workflowsAdmin.POST("", r.workflowHandler.HandleCreateWorkflow)
	workflowsAdmin.PUT("/:id", r.workflowHandler.HandleUpdateWorkflow)
	workflowsAdmin.DELETE("/:id", r.rbac.RequireAdmin(), r.workflowHandler.HandleDeleteWorkflow)

	// 节点管理
	workflowsAdmin.POST("/:id/nodes", r.workflowHandler.HandleCreateNode)
	workflowsAdmin.PUT("/:id/nodes/:node_id", r.workflowHandler.HandleUpdateNode)
	workflowsAdmin.DELETE("/:id/nodes/:node_id", r.workflowHandler.HandleDeleteNode)

	// 边管理
	workflowsAdmin.POST("/:id/edges", r.workflowHandler.HandleCreateEdge)
	workflowsAdmin.PUT("/:id/edges/:edge_id", r.workflowHandler.HandleUpdateEdge)
	workflowsAdmin.DELETE("/:id/edges/:edge_id", r.workflowHandler.HandleDeleteEdge)
}

// registerAlertRoutes 注册告警路由
func (r *Router) registerAlertRoutes(rg *gin.RouterGroup) {
	alerts := rg.Group("/alerts")
	// 告警查询
	alerts.GET("", r.alertHandler.HandleListAlerts)
	alerts.GET("/stats", r.alertHandler.HandleGetAlertStats)
	alerts.GET("/group", r.alertHandler.HandleGroupAlerts)
	alerts.GET("/label-keys", r.alertHandler.HandleGetAlertLabelKeys)
	alerts.GET("/:id", r.alertHandler.HandleGetAlert)

	// 告警操作
	alerts.POST("/:id/ack", r.alertHandler.HandleAckAlert)
	alerts.POST("/:id/resolve", r.alertHandler.HandleResolveAlert)

	// Webhook（无需认证，但需要在公开路由中注册）
	// 这里暂时放在受保护路由中，实际使用时可能需要移到公开路由

	// 告警规则管理（需要项目管理员权限）
	alertRules := rg.Group("/alert-rules")
	alertRules.Use(r.rbac.RequireProjectAdmin())
	alertRules.GET("", r.alertHandler.HandleListAlertRules)
	alertRules.POST("", r.alertHandler.HandleCreateAlertRule)
	alertRules.GET("/:id", r.alertHandler.HandleGetAlertRule)
	alertRules.PUT("/:id", r.alertHandler.HandleUpdateAlertRule)
	alertRules.DELETE("/:id", r.rbac.RequireAdmin(), r.alertHandler.HandleDeleteAlertRule)

	// 告警数据源管理（需要管理员权限）
	alertDatasources := rg.Group("/alert-datasources")
	alertDatasources.Use(r.rbac.RequireAdmin())
	alertDatasources.GET("", r.datasourceHandler.HandleListDatasources)
	alertDatasources.POST("", r.datasourceHandler.HandleCreateDatasource)
	alertDatasources.GET("/:id", r.datasourceHandler.HandleGetDatasource)
	alertDatasources.PUT("/:id", r.datasourceHandler.HandleUpdateDatasource)
	alertDatasources.DELETE("/:id", r.datasourceHandler.HandleDeleteDatasource)
	alertDatasources.POST("/test", r.datasourceHandler.HandleTestConnection)
	alertDatasources.POST("/:id/test", r.datasourceHandler.HandleTestConnectionByID)

	// 告警静默管理（需要项目管理员权限）
	alertSilences := rg.Group("/alert-silences")
	alertSilences.Use(r.rbac.RequireProjectAdmin())
	{
		alertSilences.GET("", r.alertHandler.HandleListAlertSilences)
		alertSilences.POST("", r.alertHandler.HandleCreateAlertSilence)
		alertSilences.GET("/:id", r.alertHandler.HandleGetAlertSilence)
		alertSilences.PUT("/:id", r.alertHandler.HandleUpdateAlertSilence)
		alertSilences.DELETE("/:id", r.rbac.RequireAdmin(), r.alertHandler.HandleDeleteAlertSilence)
		alertSilences.POST("/:id/cancel", r.alertHandler.HandleCancelAlertSilence)
	}
}

// registerReportRoutes 注册报表路由
func (r *Router) registerReportRoutes(rg *gin.RouterGroup) {
	reports := rg.Group("/reports")
	{
		reports.GET("/dashboard", r.reportHandler.HandleGetDashboardStats)
		reports.GET("/issues", r.reportHandler.HandleGetIssueStats)
		reports.GET("/sla", r.reportHandler.HandleGetSLAReport)
		reports.GET("/delivery", r.reportHandler.HandleGetDeliveryReport)
		reports.GET("/alerts", r.reportHandler.HandleGetAlertStats)
		reports.GET("/worklogs", r.reportHandler.HandleGetWorklogStats)
		reports.GET("/user-performance", r.reportHandler.HandleGetUserPerformance)
	}
}

// registerActivityRoutes 注册活动日志路由
func (r *Router) registerActivityRoutes(rg *gin.RouterGroup) {
	activities := rg.Group("/activities")
	{
		activities.GET("", r.activityHandler.HandleListActivities)
		activities.GET("/recent", r.activityHandler.HandleGetRecentActivities)
	}
}

// registerConfigRoutes 注册系统配置路由
func (r *Router) registerConfigRoutes(rg *gin.RouterGroup) {
	// 公共配置（普通认证用户即可访问，白名单限制）
	publicConfigs := rg.Group("/system/configs/public")
	{
		publicConfigs.GET("/:key", r.configHandler.HandleGetPublicConfig)
	}

	// 系统配置（需要管理员权限）
	configs := rg.Group("/system/configs")
	configs.Use(r.rbac.RequireAdmin())
	{
		configs.GET("", r.configHandler.HandleGetAllConfigs)
		configs.GET("/category", r.configHandler.HandleGetConfigsByCategory)
		configs.GET("/:key", r.configHandler.HandleGetConfig)
		configs.PUT("/:key", r.configHandler.HandleUpdateConfig)
		configs.PUT("", r.configHandler.HandleBatchUpdateConfigs)
	}

	// 邮件配置（需要管理员权限）
	email := rg.Group("/system/email")
	email.Use(r.rbac.RequireAdmin())
	{
		email.GET("", r.configHandler.HandleGetEmailConfig)
		email.PUT("", r.configHandler.HandleUpdateEmailConfig)
	}

	// 安全配置（需要管理员权限）
	security := rg.Group("/system/security")
	security.Use(r.rbac.RequireAdmin())
	{
		security.GET("", r.configHandler.HandleGetSecurityConfig)
		security.PUT("", r.configHandler.HandleUpdateSecurityConfig)
	}

	// 限流配置（需要管理员权限）
	ratelimit := rg.Group("/system/ratelimit")
	ratelimit.Use(r.rbac.RequireAdmin())
	{
		ratelimit.GET("", r.configHandler.HandleGetRateLimitConfig)
		ratelimit.PUT("", r.configHandler.HandleUpdateRateLimitConfig)
	}

	// SSO 配置（需要管理员权限）
	sso := rg.Group("/system/sso")
	sso.Use(r.rbac.RequireAdmin())
	{
		sso.GET("", r.configHandler.HandleGetSSOConfig)
		sso.PUT("", r.configHandler.HandleUpdateSSOConfig)
	}

	// Lark 通知配置 (需管理员权限)
	larkCfg := rg.Group("/system/lark")
	larkCfg.Use(r.rbac.RequireAdmin())
	{
		larkCfg.GET("", r.configHandler.HandleGetLarkConfig)
		larkCfg.PUT("", r.configHandler.HandleUpdateLarkConfig)
		larkCfg.POST("/test", r.configHandler.HandleTestLark)
	}

	// Telegram 通知配置 (需管理员权限)
	telegramCfg := rg.Group("/system/telegram")
	telegramCfg.Use(r.rbac.RequireAdmin())
	{
		telegramCfg.GET("", r.configHandler.HandleGetTelegramConfig)
		telegramCfg.PUT("", r.configHandler.HandleUpdateTelegramConfig)
		telegramCfg.POST("/test", r.configHandler.HandleTestTelegram)
	}

	// 品牌配置管理（需要管理员权限）
	brand := rg.Group("/system/brand")
	brand.Use(r.rbac.RequireAdmin())
	{
		brand.PUT("", r.configHandler.HandleUpdateBrandConfig)
		brand.POST("/upload", r.configHandler.HandleUploadBrandAsset)
	}

	// Webhook 管理（需要管理员权限）
	webhooks := rg.Group("/system/webhooks")
	webhooks.Use(r.rbac.RequireAdmin())
	{
		webhooks.GET("", r.configHandler.HandleListWebhooks)
		webhooks.POST("", r.configHandler.HandleCreateWebhook)
		webhooks.GET("/:id", r.configHandler.HandleGetWebhook)
		webhooks.PUT("/:id", r.configHandler.HandleUpdateWebhook)
		webhooks.DELETE("/:id", r.configHandler.HandleDeleteWebhook)
	}

	// Webhook 日志（需要管理员权限）
	webhookLogs := rg.Group("/system/webhook-logs")
	webhookLogs.Use(r.rbac.RequireAdmin())
	{
		webhookLogs.GET("", r.configHandler.HandleListWebhookLogs)
	}
}

// registerNotificationRoutes 注册通知路由
func (r *Router) registerNotificationRoutes(rg *gin.RouterGroup) {
	notifications := rg.Group("/notifications")
	{
		notifications.GET("", r.notificationHandler.HandleListNotifications)
		notifications.GET("/unread-count", r.notificationHandler.HandleGetUnreadCount)
		notifications.PUT("/:id/read", r.notificationHandler.HandleMarkAsRead)
		notifications.PUT("/read-all", r.notificationHandler.HandleMarkAllAsRead)
		notifications.DELETE("/:id", r.notificationHandler.HandleDeleteNotification)
	}
}

// ============ 通知适配器（桥接 issue_service.NotificationSender 和 notifService.NotificationService）============

// notificationAdapter 将 NotificationService 适配为 issue_service.NotificationSender
type notificationAdapter struct {
	svc notifService.NotificationService
}

// CreateNotification 适配创建通知调用
func (a *notificationAdapter) CreateNotification(ctx context.Context, req *issueService.NotificationRequest) error {
	return a.svc.CreateNotification(ctx, &notifDto.CreateNotificationRequest{
		UserID:      req.UserID,
		Type:        req.Type,
		Title:       req.Title,
		TitleKey:    req.TitleKey,
		TitleArgs:   req.TitleArgs,
		Content:     req.Content,
		ContentKey:  req.ContentKey,
		ContentArgs: req.ContentArgs,
		EntityType:  req.EntityType,
		EntityID:    req.EntityID,
		EntityKey:   req.EntityKey,
		ActorID:     req.ActorID,
		ActorName:   req.ActorName,
	})
}

// ============ 用户语言适配器（桥接 notifService.UserLocaleReader 和 UserRepository）============

// userLocaleAdapter 只暴露"读某人的偏好语言"，不把整个用户仓储递给通知模块
type userLocaleAdapter struct {
	repo userRepo.UserRepository
}

// LocaleOf 返回用户的偏好语言；查不到时返回空串，由 i18n 回落到站点语言
func (a *userLocaleAdapter) LocaleOf(ctx context.Context, userID uint64) string {
	if userID == 0 {
		return ""
	}
	user, err := a.repo.GetByID(ctx, userID)
	if err != nil || user == nil {
		return ""
	}
	return user.Locale
}

// ============ 字段值保存适配器（桥接 issue_service.FieldValueSaver 和 fieldService.FieldService）============

// fieldValueSaverAdapter 将 FieldService 适配为 issue_service.FieldValueSaver
type fieldValueSaverAdapter struct {
	svc fieldService.FieldService
}

// SaveIssueFieldValues 适配保存字段值调用
func (a *fieldValueSaverAdapter) SaveIssueFieldValues(ctx context.Context, issueID uint64, values []issueService.CustomFieldValueInput) error {
	// 转换类型
	fieldValues := make([]fieldService.CustomFieldValueInput, len(values))
	for i, v := range values {
		fieldValues[i] = fieldService.CustomFieldValueInput{
			FieldID: v.FieldID,
			Value:   v.Value,
		}
	}
	return a.svc.SaveIssueFieldValues(ctx, issueID, fieldValues)
}

// ============ 告警通知适配器（桥接 alertService.NotificationSender 和 notifService.NotificationService）============

// alertNotificationAdapter 将 NotificationService 适配为 alertService.NotificationSender
type alertNotificationAdapter struct {
	svc notifService.NotificationService
}

// CreateNotification 适配创建通知调用
func (a *alertNotificationAdapter) CreateNotification(ctx context.Context, req *alertService.AlertNotificationRequest) error {
	return a.svc.CreateNotification(ctx, &notifDto.CreateNotificationRequest{
		UserID:      req.UserID,
		Type:        req.Type,
		Title:       req.Title,
		TitleKey:    req.TitleKey,
		TitleArgs:   req.TitleArgs,
		Content:     req.Content,
		ContentKey:  req.ContentKey,
		ContentArgs: req.ContentArgs,
		EntityType:  req.EntityType,
		EntityID:    req.EntityID,
		EntityKey:   req.EntityKey,
		ActorID:     req.ActorID,
		ActorName:   req.ActorName,
	})
}

// ============ 兼容旧的 Setup 函数 ============

// memberProjectListerAdapter 将 ProjectMemberRepository 适配为 middleware.MemberProjectLister
type memberProjectListerAdapter struct {
	memberRepo projectRepo.ProjectMemberRepository
}

// ListUserProjectIDs 获取用户所属项目 ID 列表
func (a *memberProjectListerAdapter) ListUserProjectIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	members, err := a.memberRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(members))
	for i, m := range members {
		ids[i] = m.ProjectID
	}
	return ids, nil
}

// Setup 设置路由（兼容旧接口）
func Setup(cfg *config.Config, jwtManager *jwt.Manager) *gin.Engine {
	panic("请使用 NewRouter(cfg, jwtManager, db).Setup() 方式初始化路由")
}

// registerFieldRoutes 注册字段配置路由
func (r *Router) registerFieldRoutes(rg *gin.RouterGroup) {
	// ============ 全局字段管理（需要项目管理员权限）============
	adminFields := rg.Group("/admin/fields")
	adminFields.Use(r.rbac.RequireProjectAdmin())
	{
		adminFields.GET("", r.fieldHandler.HandleListGlobalFields)
		adminFields.POST("", r.fieldHandler.HandleCreateGlobalField)
		adminFields.PUT("/:id", r.fieldHandler.HandleUpdateGlobalField)
		adminFields.DELETE("/:id", r.fieldHandler.HandleDeleteGlobalField)
		adminFields.GET("/:id/usage", r.fieldHandler.HandleGetFieldUsage)
	}

	// ============ 方案模板管理（需要项目管理员权限）============
	templates := rg.Group("/admin/field-scheme-templates")
	templates.Use(r.rbac.RequireProjectAdmin())
	{
		templates.GET("", r.fieldHandler.HandleListTemplates)
		templates.POST("", r.fieldHandler.HandleCreateTemplate)
		templates.GET("/:id", r.fieldHandler.HandleGetTemplate)
		templates.PUT("/:id", r.fieldHandler.HandleUpdateTemplate)
		templates.DELETE("/:id", r.fieldHandler.HandleDeleteTemplate)
		templates.PUT("/:id/items", r.fieldHandler.HandleUpdateTemplateItems)
	}

	// 字段定义（在项目下）
	projectFields := rg.Group("/projects/:key/fields")
	{
		projectFields.GET("", r.fieldHandler.HandleListFields)
		projectFields.POST("", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleCreateField)
		projectFields.PUT("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleUpdateField)
		projectFields.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleDeleteField)
	}

	// 字段方案（在项目的工单类型下）
	fieldScheme := rg.Group("/projects/:key/issue-types/:id/field-scheme")
	{
		fieldScheme.GET("", r.fieldHandler.HandleGetFieldScheme)
		fieldScheme.PUT("", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleUpdateFieldScheme)
	}

	// 套用模板
	rg.POST("/projects/:key/issue-types/:id/apply-template", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleApplyTemplate)

	// 版本管理
	versions := rg.Group("/projects/:key/versions")
	{
		versions.GET("", r.fieldHandler.HandleListVersions)
		versions.POST("", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleCreateVersion)
		versions.PUT("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleUpdateVersion)
		versions.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleDeleteVersion)
	}

	// 组件管理
	components := rg.Group("/projects/:key/components")
	{
		components.GET("", r.fieldHandler.HandleListComponents)
		components.POST("", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleCreateComponent)
		components.PUT("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleUpdateComponent)
		components.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleDeleteComponent)
	}

	// 标签管理
	labels := rg.Group("/projects/:key/labels")
	{
		labels.GET("", r.fieldHandler.HandleListLabels)
		labels.POST("", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleCreateLabel)
		labels.PUT("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleUpdateLabel)
		labels.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.fieldHandler.HandleDeleteLabel)
	}

	// 工单字段值
	rg.GET("/issue-field-values/:issue_id", r.fieldHandler.HandleGetIssueFieldValues)
}

// registerRequirementPoolRoutes 注册需求池路由
func (r *Router) registerRequirementPoolRoutes(rg *gin.RouterGroup) {
	// 需求分类管理
	categories := rg.Group("/requirement-categories")
	{
		categories.GET("", r.categoryHandler.HandleList)
		categories.POST("", r.rbac.RequireProjectAdmin(), r.categoryHandler.HandleCreate)
		categories.PUT("/:id", r.rbac.RequireProjectAdmin(), r.categoryHandler.HandleUpdate)
		categories.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.categoryHandler.HandleDelete)
	}

	// 需求池管理
	pools := rg.Group("/requirement-pools")
	{
		pools.GET("", r.requirementPoolHandler.HandleList)
		pools.POST("", r.rbac.RequireProjectAdmin(), r.requirementPoolHandler.HandleCreate)
		pools.GET("/:id", r.requirementPoolHandler.HandleGetByID)
		pools.PUT("/:id", r.rbac.RequireProjectAdmin(), r.requirementPoolHandler.HandleUpdate)
		pools.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.requirementPoolHandler.HandleDelete)
	}

	// 需求管理
	requirements := rg.Group("/requirements")
	{
		// 看板和报告（必须在 /:id 之前）
		requirements.GET("/kanban", r.requirementHandler.HandleGetKanban)
		requirements.GET("/report", r.requirementHandler.HandleGetReport)

		// 需求 CRUD
		requirements.GET("", r.requirementHandler.HandleList)
		requirements.POST("", r.rbac.RequireProjectAdmin(), r.requirementHandler.HandleCreate)
		requirements.GET("/:id", r.requirementHandler.HandleGetByID)
		requirements.PUT("/:id", r.rbac.RequireProjectAdmin(), r.requirementHandler.HandleUpdate)
		requirements.DELETE("/:id", r.rbac.RequireProjectAdmin(), r.requirementHandler.HandleDelete)

		// 需求操作
		requirements.POST("/:id/convert", r.rbac.RequireProjectAdmin(), r.requirementHandler.HandleConvertToIssue)
		requirements.POST("/:id/comments", r.requirementHandler.HandleAddComment)
	}
}
