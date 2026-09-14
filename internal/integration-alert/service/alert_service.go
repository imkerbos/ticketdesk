// Package service 提供告警业务逻辑层
package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kerbos/ticketdesk/internal/activity/detail"

	"go.uber.org/zap"
	"gorm.io/gorm"

	issueRepo "github.com/kerbos/ticketdesk/internal/core-issue/repository"
	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	"github.com/kerbos/ticketdesk/internal/integration-alert/dto"
	"github.com/kerbos/ticketdesk/internal/integration-alert/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/cache"
	"github.com/kerbos/ticketdesk/pkg/logger"
	"github.com/kerbos/ticketdesk/pkg/safego"
	"github.com/kerbos/ticketdesk/pkg/sequence"
)

// 缓存 Key 常量
const (
	cacheKeyEnabledRules   = "alert:rules:enabled"
	cacheKeyFingerprintFmt = "alert:fp:%s"

	cacheTTLRules       = 5 * time.Minute
	cacheTTLFingerprint = 10 * time.Minute
)

// AlertService 告警服务接口
type AlertService interface {
	// Webhook 处理
	HandleWebhook(ctx context.Context, req *dto.AlertWebhookRequest) error
	HandleNightingaleWebhook(ctx context.Context, events []dto.N9eAlertEvent) error
	HandleWebhookWithSource(ctx context.Context, req *dto.AlertWebhookRequest, sourceName string, datasourceID uint64) error
	HandleNightingaleWebhookWithSource(ctx context.Context, events []dto.N9eAlertEvent, sourceName string, datasourceID uint64) error

	// 告警查询
	GetAlert(ctx context.Context, id uint64) (*dto.AlertResponse, error)
	ListAlerts(ctx context.Context, req *dto.AlertListRequest) (*dto.AlertListResponse, error)
	ListAlertsByIssueID(ctx context.Context, issueID uint64) ([]*dto.AlertResponse, error)
	GetAlertStats(ctx context.Context) (*dto.AlertStatsResponse, error)
	GroupAlerts(ctx context.Context, req *dto.AlertGroupRequest) (*dto.AlertGroupResponse, error)
	GetAlertLabelKeys(ctx context.Context) ([]string, error)

	// 告警操作
	AckAlert(ctx context.Context, id uint64, userID uint64, req *dto.AlertAckRequest) error
	ResolveAlert(ctx context.Context, id uint64, userID uint64, req *dto.AlertResolveRequest) error

	// 告警规则管理
	CreateAlertRule(ctx context.Context, req *dto.CreateAlertRuleRequest) (*dto.AlertRuleResponse, error)
	GetAlertRule(ctx context.Context, id uint64) (*dto.AlertRuleResponse, error)
	UpdateAlertRule(ctx context.Context, id uint64, req *dto.UpdateAlertRuleRequest) (*dto.AlertRuleResponse, error)
	DeleteAlertRule(ctx context.Context, id uint64) error
	ListAlertRules(ctx context.Context, req *dto.AlertRuleListRequest) (*dto.AlertRuleListResponse, error)

	// 告警静默管理
	CreateAlertSilence(ctx context.Context, req *dto.CreateAlertSilenceRequest, userID uint64) (*dto.AlertSilenceResponse, error)
	GetAlertSilence(ctx context.Context, id uint64) (*dto.AlertSilenceResponse, error)
	UpdateAlertSilence(ctx context.Context, id uint64, req *dto.UpdateAlertSilenceRequest) (*dto.AlertSilenceResponse, error)
	DeleteAlertSilence(ctx context.Context, id uint64) error
	CancelAlertSilence(ctx context.Context, id uint64) error
	ListAlertSilences(ctx context.Context, req *dto.AlertSilenceListRequest) (*dto.AlertSilenceListResponse, error)

	// 工单状态同步
	SyncIssueStatus(ctx context.Context, issueID uint64, issueStatus string) error
	// SyncMergedIssueStatus 同步状态到被合并的旧工单
	SyncMergedIssueStatus(ctx context.Context, issueID uint64, issueStatus string) error

	// TryAutoResolveIssue 检查工单下所有告警是否都已恢复，如果是则自动关闭工单
	TryAutoResolveIssue(ctx context.Context, issueID uint64) error
}

// alertService 告警服务实现
// WorkflowCreator 工作流实例创建接口（避免直接依赖 workflow engine 整个接口）
type WorkflowCreator interface {
	TryCreateInstanceForIssue(ctx context.Context, issueID, projectID, issueTypeID uint64) (*model.WorkflowInstance, error)
}

// ActivityLogger 活动日志记录器接口
type ActivityLogger interface {
	LogActivity(ctx context.Context, userID uint64, userName, action, entityType string, entityID uint64, entityKey, details string) error
}

// ProjectNotifier 项目外部渠道通知接口
type ProjectNotifier interface {
	NotifyProject(ctx context.Context, projectID uint64, event string, data any) error
}

// NotificationSender 站内通知发送接口（避免循环依赖）
type NotificationSender interface {
	CreateNotification(ctx context.Context, req *AlertNotificationRequest) error
}

// AlertNotificationRequest 告警通知请求（本地定义，避免依赖 notification-inbox/dto）
type AlertNotificationRequest struct {
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

type alertService struct {
	alertRepo        repository.AlertRepository
	alertRuleRepo    repository.AlertRuleRepository
	alertSilenceRepo repository.AlertSilenceRepository
	datasourceRepo   repository.AlertDatasourceRepository
	issueRepo        issueRepo.IssueRepository
	commentRepo      issueRepo.CommentRepository
	projectRepo      projectRepo.ProjectRepository
	issueTypeRepo    projectRepo.IssueTypeRepository
	watcherRepo      issueRepo.WatcherRepository
	db               *gorm.DB
	workflowCreator  WorkflowCreator    // 可选，用于告警建单时自动创建工作流实例
	activityLogger   ActivityLogger     // 可选，活动日志
	projectNotifier  ProjectNotifier    // 可选，项目外部渠道通知
	notifSender      NotificationSender // 可选，站内通知
}

// SetWorkflowCreator 设置工作流创建器（延迟注入，避免循环依赖）
func (s *alertService) SetWorkflowCreator(wc WorkflowCreator) {
	s.workflowCreator = wc
}

// SetActivityLogger 设置活动日志记录器
func (s *alertService) SetActivityLogger(al ActivityLogger) {
	s.activityLogger = al
}

// SetProjectNotifier 设置项目外部渠道通知服务
func (s *alertService) SetProjectNotifier(pn ProjectNotifier) {
	s.projectNotifier = pn
}

// SetNotificationSender 设置站内通知发送服务
func (s *alertService) SetNotificationSender(ns NotificationSender) {
	s.notifSender = ns
}

// NewAlertService 创建告警服务
func NewAlertService(
	alertRepo repository.AlertRepository,
	alertRuleRepo repository.AlertRuleRepository,
	alertSilenceRepo repository.AlertSilenceRepository,
	datasourceRepo repository.AlertDatasourceRepository,
	issueRepo issueRepo.IssueRepository,
	commentRepo issueRepo.CommentRepository,
	projectRepo projectRepo.ProjectRepository,
	issueTypeRepo projectRepo.IssueTypeRepository,
	watcherRepo issueRepo.WatcherRepository,
	db *gorm.DB,
) AlertService {
	return &alertService{
		alertRepo:        alertRepo,
		alertRuleRepo:    alertRuleRepo,
		alertSilenceRepo: alertSilenceRepo,
		datasourceRepo:   datasourceRepo,
		issueRepo:        issueRepo,
		commentRepo:      commentRepo,
		projectRepo:      projectRepo,
		issueTypeRepo:    issueTypeRepo,
		watcherRepo:      watcherRepo,
		db:               db,
	}
}

// HandleWebhook 处理 Webhook 告警
// maxAlertsPerRequest 单次 Webhook 请求允许处理的告警条数上限。
//
// 每条告警都要做静默匹配、指纹查询、落库，还可能自动建单并推送飞书/Telegram。
// 这些端点无需认证，一个携带十万条告警的 POST 就能让请求持续数分钟、
// 灌满工单表并触发通知风暴。超出部分直接丢弃并告警，宁可漏也不能被拖垮。
const maxAlertsPerRequest = 500

// capAlerts 截断超限的告警批次，返回实际处理的条数上限
func capAlerts(total int, source string) int {
	if total <= maxAlertsPerRequest {
		return total
	}
	logger.Warn("alert batch exceeds limit, extra alerts dropped",
		zap.String("source", source),
		zap.Int("received", total),
		zap.Int("processed", maxAlertsPerRequest),
	)
	return maxAlertsPerRequest
}

func (s *alertService) HandleWebhook(ctx context.Context, req *dto.AlertWebhookRequest) error {
	logger.Info("received alert webhook",
		zap.String("status", req.Status),
		zap.Int("alert_count", len(req.Alerts)),
	)

	for _, alertItem := range req.Alerts[:capAlerts(len(req.Alerts), "prometheus")] {
		if err := s.processAlertWithSource(ctx, &alertItem, "prometheus", 0); err != nil {
			logger.Error("failed to process alert",
				zap.String("fingerprint", alertItem.Fingerprint),
				zap.Error(err),
			)
			// 继续处理其他告警，不中断
			continue
		}
	}

	return nil
}

// HandleNightingaleWebhook 处理夜莺 Webhook 告警
func (s *alertService) HandleNightingaleWebhook(ctx context.Context, events []dto.N9eAlertEvent) error {
	logger.Info("received nightingale webhook",
		zap.Int("event_count", len(events)),
	)

	for _, event := range events[:capAlerts(len(events), "nightingale")] {
		alertItem := event.ToAlertWebhookItem()
		if err := s.processAlertWithSource(ctx, alertItem, "nightingale", 0); err != nil {
			logger.Error("failed to process nightingale alert",
				zap.String("hash", event.Hash),
				zap.String("rule_name", event.RuleName),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// HandleWebhookWithSource 处理 Webhook 告警（带数据源名称）
func (s *alertService) HandleWebhookWithSource(ctx context.Context, req *dto.AlertWebhookRequest, sourceName string, datasourceID uint64) error {
	logger.Info("received alert webhook",
		zap.String("source", sourceName),
		zap.String("status", req.Status),
		zap.Int("alert_count", len(req.Alerts)),
	)

	for _, alertItem := range req.Alerts[:capAlerts(len(req.Alerts), sourceName)] {
		if err := s.processAlertWithSource(ctx, &alertItem, sourceName, datasourceID); err != nil {
			logger.Error("failed to process alert",
				zap.String("source", sourceName),
				zap.String("fingerprint", alertItem.Fingerprint),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// HandleNightingaleWebhookWithSource 处理夜莺 Webhook 告警（带数据源名称）
func (s *alertService) HandleNightingaleWebhookWithSource(ctx context.Context, events []dto.N9eAlertEvent, sourceName string, datasourceID uint64) error {
	logger.Info("received nightingale webhook",
		zap.String("source", sourceName),
		zap.Int("event_count", len(events)),
	)

	for _, event := range events[:capAlerts(len(events), sourceName)] {
		alertItem := event.ToAlertWebhookItem()
		if err := s.processAlertWithSource(ctx, alertItem, sourceName, datasourceID); err != nil {
			logger.Error("failed to process nightingale alert",
				zap.String("source", sourceName),
				zap.String("hash", event.Hash),
				zap.String("rule_name", event.RuleName),
				zap.Error(err),
			)
			continue
		}
	}

	return nil
}

// getCachedEnabledRules 获取启用的告警规则（优先从缓存读取）
func (s *alertService) getCachedEnabledRules(ctx context.Context) ([]*model.AlertRule, error) {
	var rules []*model.AlertRule
	if cache.GetJSON(ctx, cacheKeyEnabledRules, &rules) {
		return rules, nil
	}

	// 缓存未命中，从 DB 查询
	rules, err := s.alertRuleRepo.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}

	cache.SetJSON(ctx, cacheKeyEnabledRules, rules, cacheTTLRules)
	return rules, nil
}

// getCachedAlertByFingerprint 按指纹获取告警（优先从缓存读取）
func (s *alertService) getCachedAlertByFingerprint(ctx context.Context, fingerprint string) (*model.Alert, error) {
	key := fmt.Sprintf(cacheKeyFingerprintFmt, fingerprint)

	var alert model.Alert
	if cache.GetJSON(ctx, key, &alert) {
		return &alert, nil
	}

	// 缓存未命中，从 DB 查询
	dbAlert, err := s.alertRepo.GetByFingerprint(ctx, fingerprint)
	if err != nil {
		return nil, err
	}

	// 只缓存已存在的告警，不缓存 miss
	if dbAlert != nil {
		cache.SetJSON(ctx, key, dbAlert, cacheTTLFingerprint)
	}
	return dbAlert, nil
}

// invalidateRulesCache 失效告警规则缓存
func (s *alertService) invalidateRulesCache(ctx context.Context) {
	cache.Del(ctx, cacheKeyEnabledRules)
}

// invalidateFingerprintCache 失效指纹缓存
func (s *alertService) invalidateFingerprintCache(ctx context.Context, fingerprint string) {
	cache.Del(ctx, fmt.Sprintf(cacheKeyFingerprintFmt, fingerprint))
}

// processAlertWithSource 处理单个告警（带来源参数）
func (s *alertService) processAlertWithSource(ctx context.Context, alertItem *dto.AlertWebhookAlertItem, source string, datasourceID uint64) error {
	// 1. 检查告警是否被静默
	if silenced, err := s.isAlertSilenced(ctx, alertItem.Labels); err != nil {
		logger.Error("failed to check alert silence", zap.Error(err))
	} else if silenced {
		logger.Info("alert is silenced, skipping",
			zap.String("alert_name", alertItem.Labels["alertname"]),
		)
		return nil
	}

	// 2. 计算告警指纹（如果 Webhook 没有提供）
	fingerprint := alertItem.Fingerprint
	if fingerprint == "" {
		fingerprint = s.calculateFingerprint(alertItem.Labels)
	}

	// 3. 查询是否已存在该告警（优先从缓存获取）
	existingAlert, err := s.getCachedAlertByFingerprint(ctx, fingerprint)
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to get alert by fingerprint: %w", err)
	}

	// 4. 如果告警已存在，更新状态
	if existingAlert != nil {
		return s.updateExistingAlert(ctx, existingAlert, alertItem, datasourceID)
	}

	// 5. 创建新告警
	return s.createNewAlert(ctx, fingerprint, alertItem, source, datasourceID)
}

// calculateFingerprint 计算告警指纹
func (s *alertService) calculateFingerprint(labels map[string]string) string {
	// 排序标签键
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建指纹字符串
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}
	fingerprintStr := strings.Join(parts, ";")

	// 计算 SHA256
	hash := sha256.Sum256([]byte(fingerprintStr))
	return fmt.Sprintf("%x", hash)
}

// updateExistingAlert 更新已存在的告警
func (s *alertService) updateExistingAlert(ctx context.Context, alert *model.Alert, alertItem *dto.AlertWebhookAlertItem, datasourceID uint64) error {
	// 失效指纹缓存（状态变更后下次查询需从 DB 获取最新数据）
	s.invalidateFingerprintCache(ctx, alert.Fingerprint)

	// 更新告警状态
	oldStatus := alert.Status
	alert.Status = alertItem.Status
	alert.EndsAt = alertItem.EndsAt

	// 更新数据源 ID（数据源重建后 ID 会变化）
	if datasourceID > 0 {
		alert.DatasourceID = &datasourceID
	}

	// 如果告警恢复，检查是否所有关联告警都已恢复，若是则自动解决工单
	if alertItem.Status == "resolved" && oldStatus != "resolved" && alert.IssueID != nil {
		logger.Info("alert resolved, checking if all alerts resolved for issue",
			zap.Uint64("alert_id", alert.ID),
			zap.Uint64("issue_id", *alert.IssueID),
		)

		// 先保存当前告警状态（因为 TryAutoResolveIssue 需要读到最新状态）
		if err := s.alertRepo.Update(ctx, alert); err != nil {
			return fmt.Errorf("failed to update alert: %w", err)
		}

		if err := s.TryAutoResolveIssue(ctx, *alert.IssueID); err != nil {
			logger.Error("failed to try auto resolve issue",
				zap.Uint64("issue_id", *alert.IssueID),
				zap.Error(err),
			)
		}

		return nil // 已在上面保存过，提前返回
	}

	// 如果告警仍在触发，检查是否需要建单
	if alert.Status == "firing" {
		// 如果关联的工单已经是终态（resolved/closed/merged），清除关联让告警重新建单
		if alert.IssueID != nil {
			var linkedIssue model.Issue
			if err := s.db.WithContext(ctx).Select("id, status").Where("id = ?", *alert.IssueID).First(&linkedIssue).Error; err == nil {
				if linkedIssue.Status == "resolved" || linkedIssue.Status == "closed" || linkedIssue.Status == "merged" {
					logger.Info("alert re-fired but linked issue is in terminal status, unlinking for re-creation",
						zap.Uint64("alert_id", alert.ID),
						zap.Uint64("old_issue_id", *alert.IssueID),
						zap.String("issue_status", linkedIssue.Status),
					)
					alert.IssueID = nil
				}
			}
		}

		if alert.IssueID == nil {
			if err := s.tryAutoCreateIssue(ctx, alert); err != nil {
				logger.Error("failed to auto create issue for existing alert",
					zap.Uint64("alert_id", alert.ID),
					zap.Error(err),
				)
			}
		}
	}

	return s.alertRepo.Update(ctx, alert)
}

// createNewAlert 创建新告警
func (s *alertService) createNewAlert(ctx context.Context, fingerprint string, alertItem *dto.AlertWebhookAlertItem, source string, datasourceID uint64) error {
	// 提取告警名称和严重程度
	alertName := alertItem.Labels["alertname"]
	severity := alertItem.Labels["severity"]
	if severity == "" {
		severity = "warning"
	}

	// 序列化标签和注解
	labelsJSON := repository.LabelsToJSON(alertItem.Labels)
	annotationsJSON := repository.LabelsToJSON(alertItem.Annotations)

	// 创建告警记录
	alert := &model.Alert{
		Fingerprint: fingerprint,
		Source:      source,
		AlertName:   alertName,
		Severity:    severity,
		Status:      alertItem.Status,
		Labels:      labelsJSON,
		Annotations: annotationsJSON,
		StartsAt:    alertItem.StartsAt,
		EndsAt:      alertItem.EndsAt,
	}
	if datasourceID > 0 {
		alert.DatasourceID = &datasourceID
	}

	if err := s.alertRepo.Create(ctx, alert); err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	// 写入指纹缓存
	cache.SetJSON(ctx, fmt.Sprintf(cacheKeyFingerprintFmt, fingerprint), alert, cacheTTLFingerprint)

	// 尝试自动建单
	if err := s.tryAutoCreateIssue(ctx, alert); err != nil {
		logger.Error("failed to auto create issue",
			zap.Uint64("alert_id", alert.ID),
			zap.Error(err),
		)
		// 不中断告警创建流程
	}

	// 建单后失效指纹缓存，确保后续请求从 DB 读到最新的 IssueID
	s.invalidateFingerprintCache(ctx, fingerprint)

	return nil
}

// tryAutoCreateIssue 尝试自动建单
func (s *alertService) tryAutoCreateIssue(ctx context.Context, alert *model.Alert) error {
	// 获取所有启用的告警规则（优先从缓存获取）
	rules, err := s.getCachedEnabledRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to list enabled rules: %w", err)
	}

	// 解析告警标签
	labels := repository.JSONToLabels(alert.Labels)
	annotations := repository.JSONToLabels(alert.Annotations)

	// 匹配规则
	for _, rule := range rules {
		// 规则有绑定数据源时，只匹配对应数据源的告警
		if rule.DatasourceID != nil && *rule.DatasourceID > 0 {
			if alert.DatasourceID == nil || *alert.DatasourceID != *rule.DatasourceID {
				continue
			}
		}

		if s.matchRule(rule, labels) {
			logger.Info("matched alert rule",
				zap.Uint64("alert_id", alert.ID),
				zap.Uint64("rule_id", rule.ID),
			)

			// 使用分布式锁保护合并窗口查询+建单的原子性，防止并发重复建单
			lockKey := fmt.Sprintf("alert:lock:issue:%s:%d", alert.AlertName, rule.ID)
			lockValue := cache.TryLockWithRetry(ctx, lockKey, 30*time.Second, 3, 200*time.Millisecond)
			if lockValue == "" {
				logger.Warn("failed to acquire lock for alert issue creation, skipping",
					zap.String("alert_name", alert.AlertName),
					zap.Uint64("rule_id", rule.ID),
				)
				return nil
			}
			err := func() error {
				defer cache.UnlockWithValue(ctx, lockKey, lockValue)

				// 检查是否需要合并到现有工单
				if rule.MergeWindow > 0 {
					existingIssueID, findErr := s.findMergeableIssue(ctx, rule, alert)
					if findErr != nil {
						logger.Error("failed to find mergeable issue", zap.Error(findErr))
					} else if existingIssueID > 0 {
						// 合并到现有工单
						logger.Info("merging alert to existing issue",
							zap.Uint64("alert_id", alert.ID),
							zap.Uint64("issue_id", existingIssueID),
						)
						alert.IssueID = &existingIssueID
						if updateErr := s.alertRepo.Update(ctx, alert); updateErr != nil {
							return fmt.Errorf("failed to update alert with issue_id: %w", updateErr)
						}
						// 追加实例信息到工单
						if appendErr := s.appendAlertToIssue(ctx, existingIssueID, alert, labels); appendErr != nil {
							logger.Error("failed to append alert info to issue",
								zap.Uint64("issue_id", existingIssueID),
								zap.Error(appendErr),
							)
						}
						// 如果工单处于 pending_review 状态，重新激活（新告警触发说明问题未解决）
						s.reactivateIssueIfPendingReview(ctx, existingIssueID)
						return nil
					}
				}

				// 创建新工单
				issueID, newIssueKey, createErr := s.createIssueFromAlert(ctx, alert, rule, labels, annotations)
				if createErr != nil {
					return fmt.Errorf("failed to create issue: %w", createErr)
				}

				// 将同名旧工单标记为 merged，并双向关联
				if rule.MergeWindow > 0 {
					s.mergeOldIssues(ctx, rule, alert, issueID, newIssueKey)
				}

				// 更新告警关联的工单 ID
				alert.IssueID = &issueID
				if updateErr := s.alertRepo.Update(ctx, alert); updateErr != nil {
					return fmt.Errorf("failed to update alert with issue_id: %w", updateErr)
				}

				logger.Info("issue created from alert",
					zap.Uint64("alert_id", alert.ID),
					zap.Uint64("issue_id", issueID),
				)

				return nil
			}()
			if err != nil {
				return err
			}

			break
		}
	}

	return nil
}

// createIssueFromAlert 从告警创建工单
func (s *alertService) createIssueFromAlert(
	ctx context.Context,
	alert *model.Alert,
	rule *model.AlertRule,
	labels map[string]string,
	annotations map[string]string,
) (uint64, string, error) {
	// 获取项目信息
	project, err := s.projectRepo.GetByID(ctx, rule.ProjectID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get project: %w", err)
	}

	// 获取工单类型（用于验证）
	_, err = s.issueTypeRepo.GetByID(ctx, rule.IssueTypeID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get issue type: %w", err)
	}

	// 构建工单标题和描述
	title := fmt.Sprintf("[告警] %s (1 个实例)", alert.AlertName)

	description := s.buildIssueDescription(alert, labels, annotations)

	// 生成工单 Key
	issueKey, err := s.generateIssueKey(ctx, project.ProjectKey)
	if err != nil {
		return 0, "", fmt.Errorf("failed to generate issue key: %w", err)
	}

	// 确定指派人：规则配置 > 项目负责人（lead_user_id）
	assigneeID := rule.AssigneeID
	if assigneeID == nil && project.LeadUserID > 0 {
		assigneeID = &project.LeadUserID
	}

	// 获取告警机器人用户 ID 作为创建人
	var reporterID uint64 = 1
	var botUser model.User
	if err := s.db.Where("username = ?", "alert-bot").First(&botUser).Error; err == nil {
		reporterID = botUser.ID
	}

	// 创建工单
	issue := &model.Issue{
		IssueKey:    issueKey,
		ProjectID:   rule.ProjectID,
		IssueTypeID: rule.IssueTypeID,
		Title:       title,
		Description: description,
		Priority:    rule.Priority,
		Status:      "open",
		ReporterID:  reporterID,
		AssigneeID:  assigneeID,
	}

	if err := s.issueRepo.Create(ctx, issue); err != nil {
		return 0, "", fmt.Errorf("failed to create issue: %w", err)
	}

	// 创建工作流实例（根据项目+工单类型查找工作流方案）
	if s.workflowCreator != nil {
		instance, err := s.workflowCreator.TryCreateInstanceForIssue(ctx, issue.ID, rule.ProjectID, rule.IssueTypeID)
		if err != nil {
			logger.Error("failed to create workflow instance for alert issue",
				zap.Error(err), zap.String("issue_key", issueKey))
			// 在工单描述中标注工作流创建失败，便于人工排查
			issue.Description += "\n\n---\n> ⚠️ **注意**: 工作流实例创建失败，请手动检查工作流配置\n"
			_ = s.issueRepo.Update(ctx, issue)
		} else if instance != nil {
			// 重新读取工单（工作流引擎可能已联动更新了状态）
			if freshIssue, err := s.issueRepo.GetByID(ctx, issue.ID); err == nil {
				freshIssue.WorkflowInstanceID = &instance.ID
				_ = s.issueRepo.Update(ctx, freshIssue)
			}
			logger.Info("workflow instance created for alert issue",
				zap.String("issue_key", issueKey),
				zap.Uint64("instance_id", instance.ID))
		}
	}

	// 自动添加指派人为关注人
	if assigneeID != nil {
		watcher := &model.IssueWatcher{
			IssueID: issue.ID,
			UserID:  *assigneeID,
		}
		_ = s.watcherRepo.Create(ctx, watcher)
	}

	// 记录活动日志
	if s.activityLogger != nil {
		_ = s.activityLogger.LogActivity(
			ctx, reporterID, "alert-bot", "issue_created", "issue",
			issue.ID, issueKey,
			detail.New("activity.detail.issueCreatedByAlert", "title", issue.Title),
		)
	}

	// 站内通知指派人
	if s.notifSender != nil && assigneeID != nil {
		go func() {
			defer safego.Recover("alert.notifyAssignee")
			notifCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.notifSender.CreateNotification(notifCtx, &AlertNotificationRequest{
				UserID:     *assigneeID,
				Type:       "issue_assigned",
				TitleKey:   "inbox.alert_issue_assigned",
				TitleArgs:  []any{issueKey},
				Content:    issue.Title,
				EntityType: "issue",
				EntityID:   issue.ID,
				EntityKey:  issueKey,
				ActorID:    reporterID,
				ActorName:  "alert-bot",
			}); err != nil {
				logger.Warn("failed to send in-app notification for alert issue",
					zap.String("issue_key", issueKey), zap.Error(err))
			}
		}()
	}

	// 通知项目外部渠道（飞书/Telegram）
	if s.projectNotifier != nil {
		// 获取指派人名称
		assigneeName := ""
		if assigneeID != nil {
			var user model.User
			if err := s.db.Select("username", "display_name").Where("id = ?", *assigneeID).First(&user).Error; err == nil {
				assigneeName = user.DisplayName
				if assigneeName == "" {
					assigneeName = user.Username
				}
			}
		}
		go func() {
			defer safego.Recover("alert.notifyProjectChannels")
			notifCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.projectNotifier.NotifyProject(notifCtx, rule.ProjectID, "issue.created", map[string]any{
				"issue_key":    issueKey,
				"issue_title":  issue.Title,
				"project_key":  project.ProjectKey,
				"project_name": project.Name,
				"status":       issue.Status,
				"priority":     issue.Priority,
				"source":       "alert",
				"alert_name":   alert.AlertName,
				"severity":     alert.Severity,
				"alert_time":   alert.StartsAt.Format("2006-01-02 15:04:05"),
				"assignee":     assigneeName,
			}); err != nil {
				logger.Warn("failed to notify project channels for alert issue",
					zap.String("issue_key", issueKey), zap.Error(err))
			}
		}()
	}

	return issue.ID, issueKey, nil
}

// buildIssueDescription 构建工单描述
func (s *alertService) buildIssueDescription(
	alert *model.Alert,
	labels map[string]string,
	annotations map[string]string,
) string {
	var desc strings.Builder

	// 告警基本信息
	desc.WriteString("## 告警信息\n\n")
	fmt.Fprintf(&desc, "- **告警名称**: %s\n", alert.AlertName)
	fmt.Fprintf(&desc, "- **严重程度**: %s\n", alert.Severity)
	fmt.Fprintf(&desc, "- **告警状态**: %s\n", alert.Status)
	fmt.Fprintf(&desc, "- **开始时间**: %s\n", alert.StartsAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&desc, "- **告警指纹**: %s\n", alert.Fingerprint)

	// 告警描述
	if description, ok := annotations["description"]; ok {
		fmt.Fprintf(&desc, "\n**描述**: %s\n", description)
	}

	// 告警标签
	if len(labels) > 0 {
		desc.WriteString("\n## 标签\n\n")
		for k, v := range labels {
			fmt.Fprintf(&desc, "- **%s**: %s\n", k, v)
		}
	}

	// 其他注解
	if len(annotations) > 0 {
		desc.WriteString("\n## 详细信息\n\n")
		for k, v := range annotations {
			if k != "summary" && k != "description" {
				fmt.Fprintf(&desc, "- **%s**: %s\n", k, v)
			}
		}
	}

	desc.WriteString("\n---\n")
	desc.WriteString("*此工单由告警系统自动创建*\n")

	return desc.String()
}

// generateIssueKey 生成工单 Key（优先使用 Redis 原子计数器）
func (s *alertService) generateIssueKey(ctx context.Context, projectKey string) (string, error) {
	// DB 降级回调：从数据库查询当前最大序号
	dbFallback := func(ctx context.Context, pKey string) (int64, error) {
		var maxSeq int
		err := s.db.WithContext(ctx).
			Unscoped().
			Model(&model.Issue{}).
			Where("issue_key LIKE ?", pKey+"-%").
			Select("COALESCE(MAX(CAST(SUBSTRING(issue_key, LENGTH(?) + 2) AS UNSIGNED)), 0)", pKey).
			Scan(&maxSeq).Error
		if err != nil {
			return 0, err
		}
		return int64(maxSeq) + 1, nil
	}

	nextNum, err := sequence.NextIssueNumber(ctx, projectKey, dbFallback)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%d", projectKey, nextNum), nil
}

// appendAlertToIssue 合并告警时更新工单标题计数，不再追加描述
func (s *alertService) appendAlertToIssue(
	ctx context.Context,
	issueID uint64,
	alert *model.Alert,
	labels map[string]string,
) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// 统计该工单关联的告警数量
	linkedAlerts, err := s.alertRepo.ListByIssueID(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to count linked alerts: %w", err)
	}
	alertCount := len(linkedAlerts)

	// 只更新标题计数：[告警] AlertName (N 个实例)
	issue.Title = fmt.Sprintf("[告警] %s (%d 个实例)", alert.AlertName, alertCount)

	if err := s.issueRepo.Update(ctx, issue); err != nil {
		return fmt.Errorf("failed to update issue: %w", err)
	}

	// 添加系统评论通知有新告警合并
	instance := labels["instance"]
	if instance == "" {
		instance = labels["target_ident"]
	}
	commentContent := detail.New("comment.system.alertInstanceMerged",
		"instance", instance, "fingerprint", alert.Fingerprint,
		"time", alert.StartsAt.Format("2006-01-02 15:04:05"))
	if err := s.addSystemComment(ctx, issueID, commentContent); err != nil {
		logger.Warn("failed to add merge notification comment",
			zap.Uint64("issue_id", issueID),
			zap.Error(err),
		)
	}

	logger.Info("alert merged to issue",
		zap.Uint64("issue_id", issueID),
		zap.String("instance", instance),
		zap.Int("alert_count", alertCount),
	)

	// 站内通知指派人和关注人：有新告警合并
	if s.notifSender != nil {
		go func() {
			defer safego.Recover("alert.notifyMergeWatchers")
			notifCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// 收集需要通知的用户（指派人 + 关注人）
			notifyUserIDs := make(map[uint64]bool)
			if issue.AssigneeID != nil {
				notifyUserIDs[*issue.AssigneeID] = true
			}
			if watchers, err := s.watcherRepo.ListByIssue(notifCtx, issueID); err == nil {
				for _, w := range watchers {
					notifyUserIDs[w.UserID] = true
				}
			}

			for uid := range notifyUserIDs {
				_ = s.notifSender.CreateNotification(notifCtx, &AlertNotificationRequest{
					UserID:      uid,
					Type:        "alert_merged",
					TitleKey:    "inbox.alert_merged",
					TitleArgs:   []any{issue.IssueKey, alertCount},
					ContentKey:  "inbox.alert_merged_body",
					ContentArgs: []any{instance, alert.Fingerprint},
					EntityType:  "issue",
					EntityID:    issueID,
					EntityKey:   issue.IssueKey,
					ActorID:     0,
					ActorName:   "alert-bot",
				})
			}
		}()
	}

	// 通知项目外部渠道（飞书/Telegram）
	if s.projectNotifier != nil {
		go func() {
			defer safego.Recover("alert.notifyMergeChannels")
			notifCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = s.projectNotifier.NotifyProject(notifCtx, issue.ProjectID, "alert.merged", map[string]any{
				"issue_key":   issue.IssueKey,
				"issue_title": issue.Title,
				"alert_name":  alert.AlertName,
				"alert_count": alertCount,
				"instance":    instance,
				"fingerprint": alert.Fingerprint,
				"source":      "alert",
			})
		}()
	}

	return nil
}

// addSystemComment 添加系统评论（UserID=0 表示系统）
func (s *alertService) addSystemComment(ctx context.Context, issueID uint64, content string) error {
	comment := &model.IssueComment{
		IssueID: issueID,
		UserID:  0,
		Content: content,
	}
	if err := s.commentRepo.Create(ctx, comment); err != nil {
		return fmt.Errorf("failed to create system comment: %w", err)
	}
	return nil
}

// autoUpdateIssueOnRecovery 告警恢复后更新工单状态
// autoResolve=true → resolved（自动关闭）；autoResolve=false → pending_review（待确认）
func (s *alertService) autoUpdateIssueOnRecovery(ctx context.Context, issueID uint64, autoResolve bool) error {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// 如果工单已经是终态，不需要处理（pending_review 不算终态，告警恢复时应允许自动 resolve）
	if issue.Status == "resolved" || issue.Status == "closed" || issue.Status == "merged" {
		logger.Info("autoUpdateIssueOnRecovery: issue already in terminal status, skipping",
			zap.Uint64("issue_id", issueID),
			zap.String("current_status", issue.Status),
			zap.Bool("auto_resolve", autoResolve),
		)
		return nil
	}

	now := time.Now()

	if autoResolve {
		issue.Status = "resolved"
		issue.ResolvedAt = &now
		if err := s.issueRepo.Update(ctx, issue); err != nil {
			return fmt.Errorf("failed to update issue status: %w", err)
		}

		// 同步完成工作流实例，避免工单状态与工作流状态不一致
		s.forceCompleteWorkflow(ctx, issueID)

		// 级联同步合并工单状态（合并工单也应标记为 resolved）
		if err := s.SyncMergedIssueStatus(ctx, issueID, "resolved"); err != nil {
			logger.Warn("failed to cascade resolved status to merged issues on auto-resolve",
				zap.Uint64("issue_id", issueID), zap.Error(err))
		}

		commentContent := detail.New("comment.system.alertRecoveredClosed")
		if err := s.addSystemComment(ctx, issueID, commentContent); err != nil {
			logger.Error("failed to add system comment for auto-resolve",
				zap.Uint64("issue_id", issueID),
				zap.Error(err),
			)
		}

		// 记录活动日志
		if s.activityLogger != nil {
			_ = s.activityLogger.LogActivity(ctx, 0, "alert-bot", "status_changed",
				"issue", issueID, issue.IssueKey, detail.New("activity.detail.alertRecoveredClosed"))
		}

		logger.Info("issue auto-resolved by alert recovery",
			zap.Uint64("issue_id", issueID),
		)
	} else {
		issue.Status = "pending_review"
		if err := s.issueRepo.Update(ctx, issue); err != nil {
			return fmt.Errorf("failed to update issue status: %w", err)
		}

		// 同步工作流实例为验收中，告警已恢复，等待处理人确认是否关闭工单
		s.setWorkflowReviewing(ctx, issueID)

		commentContent := detail.New("comment.system.alertRecoveredPendingReview", "time", now.Format("15:04"))
		if err := s.addSystemComment(ctx, issueID, commentContent); err != nil {
			logger.Error("failed to add system comment for pending_review",
				zap.Uint64("issue_id", issueID),
				zap.Error(err),
			)
		}

		// 记录活动日志
		if s.activityLogger != nil {
			_ = s.activityLogger.LogActivity(ctx, 0, "alert-bot", "status_changed",
				"issue", issueID, issue.IssueKey, detail.New("activity.detail.alertRecoveredPendingReview"))
		}

		logger.Info("issue set to pending_review by alert recovery",
			zap.Uint64("issue_id", issueID),
		)
	}

	return nil
}

// forceCompleteWorkflow 强制完成工单关联的工作流实例
// 当告警恢复自动关闭工单时，需要同步完成工作流，避免工单"已完成"但工作流仍"进行中"的不一致
func (s *alertService) forceCompleteWorkflow(ctx context.Context, issueID uint64) {
	// 先查出工作流实例，获取 workflow_id 以便找到结束节点
	var instance model.WorkflowInstance
	err := s.db.WithContext(ctx).
		Where("issue_id = ? AND status IN ?", issueID, []string{"active", "reviewing"}).
		First(&instance).Error
	if err != nil {
		logger.Debug("forceCompleteWorkflow: no active/reviewing workflow instance found",
			zap.Uint64("issue_id", issueID),
		)
		return
	}

	now := time.Now()
	updates := map[string]any{
		"status":       "completed",
		"completed_at": now,
		"updated_at":   now,
	}

	// 查找结束节点，更新 current_node_id 使前端工作流图正确显示
	var endNode model.WorkflowNode
	if err := s.db.WithContext(ctx).
		Where("workflow_id = ? AND node_type = ?", instance.WorkflowID, "end").
		First(&endNode).Error; err == nil {
		updates["current_node_id"] = endNode.ID
	}

	result := s.db.WithContext(ctx).
		Model(&model.WorkflowInstance{}).
		Where("id = ?", instance.ID).
		Updates(updates)
	if result.Error != nil {
		logger.Warn("failed to force-complete workflow instance on alert recovery",
			zap.Uint64("issue_id", issueID),
			zap.Error(result.Error),
		)
	} else {
		logger.Info("workflow instance force-completed by alert recovery",
			zap.Uint64("issue_id", issueID),
			zap.Uint64("instance_id", instance.ID),
		)
	}
}

// setWorkflowReviewing 告警恢复时将工作流流转到"待确认"节点
// 通过工作流定义中的节点驱动，不再直接修改工作流状态为 reviewing
func (s *alertService) setWorkflowReviewing(ctx context.Context, issueID uint64) {
	// 查找工作流实例
	var instance model.WorkflowInstance
	if err := s.db.WithContext(ctx).
		Where("issue_id = ? AND status = ?", issueID, "active").
		First(&instance).Error; err != nil {
		logger.Debug("setWorkflowReviewing: no active workflow instance found",
			zap.Uint64("issue_id", issueID))
		return
	}

	// 查找"待确认"节点
	var reviewNode model.WorkflowNode
	if err := s.db.WithContext(ctx).
		Where("workflow_id = ? AND name = ?", instance.WorkflowID, "待确认").
		First(&reviewNode).Error; err != nil {
		// 没有"待确认"节点（旧工作流），回退到直接改状态
		logger.Warn("review node not found, falling back to status change",
			zap.Uint64("workflow_id", instance.WorkflowID))
		now := time.Now()
		s.db.WithContext(ctx).Model(&model.WorkflowInstance{}).
			Where("id = ?", instance.ID).
			Updates(map[string]any{"status": "active", "updated_at": now})
		return
	}

	// 流转到"待确认"节点
	now := time.Now()
	s.db.WithContext(ctx).Model(&model.WorkflowInstance{}).
		Where("id = ?", instance.ID).
		Updates(map[string]any{
			"current_node_id": reviewNode.ID,
			"updated_at":      now,
		})

	logger.Info("workflow instance moved to review node by alert recovery",
		zap.Uint64("issue_id", issueID),
		zap.Uint64("instance_id", instance.ID),
		zap.Uint64("review_node_id", reviewNode.ID))
}

// reactivateIssueIfPendingReview 当新告警合并到 pending_review 工单时，重新激活工单和工作流
// 新告警触发说明问题未真正解决，需要将工单从"待确认"恢复为"进行中"
func (s *alertService) reactivateIssueIfPendingReview(ctx context.Context, issueID uint64) {
	issue, err := s.issueRepo.GetByID(ctx, issueID)
	if err != nil || issue.Status != "pending_review" {
		return
	}

	// 恢复工单状态为 in_progress
	issue.Status = "in_progress"
	if err := s.issueRepo.Update(ctx, issue); err != nil {
		logger.Error("failed to reactivate issue from pending_review",
			zap.Uint64("issue_id", issueID), zap.Error(err))
		return
	}

	// 查找工作流实例，将 current_node_id 移回"处理中"节点
	var instance model.WorkflowInstance
	if err := s.db.WithContext(ctx).
		Where("issue_id = ? AND status = ?", issueID, "active").
		First(&instance).Error; err != nil {
		// 兼容旧的 reviewing 状态
		if err2 := s.db.WithContext(ctx).
			Where("issue_id = ? AND status = ?", issueID, "reviewing").
			First(&instance).Error; err2 != nil {
			logger.Debug("reactivateIssueIfPendingReview: no active/reviewing workflow instance",
				zap.Uint64("issue_id", issueID))
			goto comment
		}
	}

	{
		// 查找"处理中"节点
		var workNode model.WorkflowNode
		if err := s.db.WithContext(ctx).
			Where("workflow_id = ? AND name = ? AND node_type = ?", instance.WorkflowID, "处理中", "work").
			First(&workNode).Error; err == nil {
			now := time.Now()
			s.db.WithContext(ctx).Model(&model.WorkflowInstance{}).
				Where("id = ?", instance.ID).
				Updates(map[string]any{
					"current_node_id": workNode.ID,
					"status":          "active",
					"updated_at":      now,
				})
		} else {
			// 没有"处理中"节点，回退到直接改状态
			now := time.Now()
			s.db.WithContext(ctx).Model(&model.WorkflowInstance{}).
				Where("id = ?", instance.ID).
				Updates(map[string]any{"status": "active", "updated_at": now})
		}
	}

comment:
	// 添加系统评论
	_ = s.addSystemComment(ctx, issueID, detail.New("comment.system.issueReactivated"))

	logger.Info("issue reactivated from pending_review due to new alert",
		zap.Uint64("issue_id", issueID))
}

// TryAutoResolveIssue 检查工单关联的所有告警是否都已恢复，如果是则根据规则设置更新工单状态
func (s *alertService) TryAutoResolveIssue(ctx context.Context, issueID uint64) error {
	// 获取该工单下所有告警
	alerts, err := s.alertRepo.ListByIssueID(ctx, issueID)
	if err != nil {
		return fmt.Errorf("failed to list alerts by issue_id: %w", err)
	}

	if len(alerts) == 0 {
		return nil
	}

	// 检查是否所有告警都已恢复
	for _, a := range alerts {
		if a.Status == "firing" {
			return nil // 还有活跃告警，不关闭工单
		}
	}

	// 遍历所有告警尝试匹配规则（不同告警可能匹配不同规则）
	autoResolve := false
	matchedAny := false
	for _, a := range alerts {
		rule, err := s.getMatchedRule(ctx, a)
		if err != nil {
			logger.Error("TryAutoResolveIssue: failed to get matched rule",
				zap.Uint64("alert_id", a.ID),
				zap.Error(err),
			)
			continue
		}
		if rule != nil {
			matchedAny = true
			logger.Info("TryAutoResolveIssue: matched rule",
				zap.Uint64("issue_id", issueID),
				zap.Uint64("alert_id", a.ID),
				zap.Uint64("rule_id", rule.ID),
				zap.String("rule_name", rule.Name),
				zap.Bool("rule_auto_resolve", rule.AutoResolve),
			)
			if rule.AutoResolve {
				autoResolve = true
				break // 只要有一条规则是 AutoResolve=true，就自动关闭
			}
		}
	}

	if !matchedAny {
		// 无匹配规则，默认设为待确认
		logger.Info("TryAutoResolveIssue: no matching rule found, setting to pending_review",
			zap.Uint64("issue_id", issueID),
		)
	}

	// 所有告警都已恢复，根据 autoResolve 设置更新工单
	if err := s.autoUpdateIssueOnRecovery(ctx, issueID, autoResolve); err != nil {
		return fmt.Errorf("failed to auto update issue on recovery: %w", err)
	}

	logger.Info("TryAutoResolveIssue: all alerts resolved, issue updated",
		zap.Uint64("issue_id", issueID),
		zap.Bool("auto_resolve", autoResolve),
		zap.Int("alert_count", len(alerts)),
	)
	return nil
}

// getMatchedRule 获取匹配的告警规则
func (s *alertService) getMatchedRule(ctx context.Context, alert *model.Alert) (*model.AlertRule, error) {
	rules, err := s.getCachedEnabledRules(ctx)
	if err != nil {
		return nil, err
	}

	labels := repository.JSONToLabels(alert.Labels)
	for _, rule := range rules {
		if s.matchRule(rule, labels) {
			return rule, nil
		}
	}

	return nil, nil
}

// matchRule 匹配告警规则
func (s *alertService) matchRule(rule *model.AlertRule, labels map[string]string) bool {
	// 解析规则的标签匹配器
	var matchers []dto.LabelMatcher
	if err := json.Unmarshal([]byte(rule.LabelMatchers), &matchers); err != nil {
		logger.Error("failed to unmarshal label matchers", zap.Error(err))
		return false
	}

	// 检查所有匹配器
	for _, matcher := range matchers {
		labelValue, exists := labels[matcher.Key]

		switch matcher.Operator {
		case "==":
			if !exists || labelValue != matcher.Value {
				return false
			}
		case "!=":
			if exists && labelValue == matcher.Value {
				return false
			}
		case "=~":
			if !exists {
				return false
			}
			matched, err := regexp.MatchString(matcher.Value, labelValue)
			if err != nil || !matched {
				return false
			}
		case "!~":
			if !exists {
				continue
			}
			matched, err := regexp.MatchString(matcher.Value, labelValue)
			if err != nil || matched {
				return false
			}
		}
	}

	return true
}

// GetAlert 获取告警详情
func (s *alertService) GetAlert(ctx context.Context, id uint64) (*dto.AlertResponse, error) {
	alert, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.toAlertResponse(alert), nil
}

// ListAlertsByIssueID 根据工单 ID 获取关联的告警列表
func (s *alertService) ListAlertsByIssueID(ctx context.Context, issueID uint64) ([]*dto.AlertResponse, error) {
	alerts, err := s.alertRepo.ListByIssueID(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("failed to list alerts by issue_id: %w", err)
	}

	items := make([]*dto.AlertResponse, 0, len(alerts))
	for _, alert := range alerts {
		items = append(items, s.toAlertResponse(alert))
	}

	return items, nil
}

// ListAlerts 获取告警列表
func (s *alertService) ListAlerts(ctx context.Context, req *dto.AlertListRequest) (*dto.AlertListResponse, error) {
	alerts, total, err := s.alertRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	// 批量获取关联工单的 issue_key，避免 N+1 查询
	issueKeyMap := s.batchGetIssueKeys(ctx, alerts)

	items := make([]dto.AlertResponse, 0, len(alerts))
	for _, alert := range alerts {
		resp := s.toAlertResponse(alert)
		if alert.IssueID != nil {
			if key, ok := issueKeyMap[*alert.IssueID]; ok {
				resp.IssueKey = &key
			}
		}
		items = append(items, *resp)
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	return &dto.AlertListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// batchGetIssueKeys 批量获取告警关联工单的 issue_key
func (s *alertService) batchGetIssueKeys(ctx context.Context, alerts []*model.Alert) map[uint64]string {
	issueIDs := make([]uint64, 0)
	for _, a := range alerts {
		if a.IssueID != nil {
			issueIDs = append(issueIDs, *a.IssueID)
		}
	}
	if len(issueIDs) == 0 {
		return nil
	}

	var issues []model.Issue
	s.db.WithContext(ctx).Select("id, issue_key").Where("id IN ?", issueIDs).Find(&issues)

	keyMap := make(map[uint64]string, len(issues))
	for _, iss := range issues {
		keyMap[iss.ID] = iss.IssueKey
	}
	return keyMap
}

// GetAlertStats 获取告警统计数据
func (s *alertService) GetAlertStats(ctx context.Context) (*dto.AlertStatsResponse, error) {
	return s.alertRepo.Stats(ctx)
}

// GetAlertLabelKeys 获取所有告警中出现过的标签 key
func (s *alertService) GetAlertLabelKeys(ctx context.Context) ([]string, error) {
	return s.alertRepo.ListLabelKeys(ctx)
}

// AckAlert 确认告警
func (s *alertService) AckAlert(ctx context.Context, id uint64, userID uint64, req *dto.AlertAckRequest) error {
	// 检查告警是否存在
	alert, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否已确认
	if alert.AckAt != nil {
		return fmt.Errorf("alert already acknowledged")
	}

	// 确认告警
	now := time.Now()
	if err := s.alertRepo.Ack(ctx, id, userID, now); err != nil {
		return err
	}

	// 记录活动日志（关联工单）
	if s.activityLogger != nil && alert.IssueID != nil {
		if issue, err := s.issueRepo.GetByID(ctx, *alert.IssueID); err == nil && issue != nil {
			_ = s.activityLogger.LogActivity(ctx, userID, "", "alert_acked",
				"issue", *alert.IssueID, issue.IssueKey,
				detail.New("activity.detail.alertAcked", "name", alert.AlertName))
		}
	}

	return nil
}

// ResolveAlert 解决告警
func (s *alertService) ResolveAlert(ctx context.Context, id uint64, userID uint64, req *dto.AlertResolveRequest) error {
	// 检查告警是否存在
	alert, err := s.alertRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 检查是否已解决
	if alert.Status == "resolved" {
		return fmt.Errorf("alert already resolved")
	}

	// 解决告警
	now := time.Now()
	if err := s.alertRepo.Resolve(ctx, id, userID, now); err != nil {
		return err
	}

	// 记录活动日志（关联工单）
	if s.activityLogger != nil && alert.IssueID != nil {
		if issue, err := s.issueRepo.GetByID(ctx, *alert.IssueID); err == nil && issue != nil {
			_ = s.activityLogger.LogActivity(ctx, userID, "", "alert_resolved",
				"issue", *alert.IssueID, issue.IssueKey,
				detail.New("activity.detail.alertResolved", "name", alert.AlertName))
		}
	}

	// 手动解决告警后，检查关联工单是否可以自动处理
	if alert.IssueID != nil {
		if err := s.TryAutoResolveIssue(ctx, *alert.IssueID); err != nil {
			logger.Error("failed to try auto resolve issue after manual alert resolve",
				zap.Uint64("alert_id", id),
				zap.Uint64("issue_id", *alert.IssueID),
				zap.Error(err),
			)
		}
	}

	return nil
}

// toAlertResponse 转换为告警响应
func (s *alertService) toAlertResponse(alert *model.Alert) *dto.AlertResponse {
	resp := &dto.AlertResponse{
		ID:          alert.ID,
		Fingerprint: alert.Fingerprint,
		Source:      alert.Source,
		AlertName:   alert.AlertName,
		Severity:    alert.Severity,
		Status:      alert.Status,
		Labels:      repository.JSONToLabels(alert.Labels),
		Annotations: repository.JSONToLabels(alert.Annotations),
		StartsAt:    alert.StartsAt,
		EndsAt:      alert.EndsAt,
		IssueID:     alert.IssueID,
		AckAt:       alert.AckAt,
		AckBy:       alert.AckBy,
		ResolvedAt:  alert.ResolvedAt,
		ResolvedBy:  alert.ResolvedBy,
		CreatedAt:   alert.CreatedAt,
		UpdatedAt:   alert.UpdatedAt,
	}

	// 填充关联工单 Key
	if alert.IssueID != nil {
		if issue, err := s.issueRepo.GetByID(context.Background(), *alert.IssueID); err == nil && issue != nil {
			resp.IssueKey = &issue.IssueKey
		}
	}

	return resp
}

// ============ 告警规则管理 ============

// CreateAlertRule 创建告警规则
func (s *alertService) CreateAlertRule(ctx context.Context, req *dto.CreateAlertRuleRequest) (*dto.AlertRuleResponse, error) {
	// 序列化标签匹配器
	matchersJSON, err := json.Marshal(req.LabelMatchers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal label matchers: %w", err)
	}

	mergeWindow := req.MergeWindow
	if mergeWindow == 0 {
		mergeWindow = 3600 // 默认 1 小时
	}

	rule := &model.AlertRule{
		Name:          req.Name,
		Description:   req.Description,
		DatasourceID:  &req.DatasourceID,
		ProjectID:     req.ProjectID,
		IssueTypeID:   req.IssueTypeID,
		LabelMatchers: string(matchersJSON),
		Priority:      req.Priority,
		AssigneeID:    req.AssigneeID,
		AutoResolve:   req.AutoResolve,
		MergeWindow:   mergeWindow,
		Status:        1, // 默认启用
	}

	if err := s.alertRuleRepo.Create(ctx, rule); err != nil {
		return nil, err
	}

	s.invalidateRulesCache(ctx)
	return s.GetAlertRule(ctx, rule.ID)
}

// GetAlertRule 获取告警规则详情
func (s *alertService) GetAlertRule(ctx context.Context, id uint64) (*dto.AlertRuleResponse, error) {
	rule, err := s.alertRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return s.toAlertRuleResponse(rule), nil
}

// UpdateAlertRule 更新告警规则
func (s *alertService) UpdateAlertRule(ctx context.Context, id uint64, req *dto.UpdateAlertRuleRequest) (*dto.AlertRuleResponse, error) {
	rule, err := s.alertRuleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}
	if req.DatasourceID != nil {
		rule.DatasourceID = req.DatasourceID
	}
	if req.LabelMatchers != nil {
		matchersJSON, err := json.Marshal(req.LabelMatchers)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal label matchers: %w", err)
		}
		rule.LabelMatchers = string(matchersJSON)
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		rule.AssigneeID = req.AssigneeID
	}
	if req.AutoResolve != nil {
		rule.AutoResolve = *req.AutoResolve
	}
	if req.MergeWindow != nil {
		rule.MergeWindow = *req.MergeWindow
	}
	if req.Status != nil {
		rule.Status = *req.Status
	}

	if err := s.alertRuleRepo.Update(ctx, rule); err != nil {
		return nil, err
	}

	s.invalidateRulesCache(ctx)
	return s.GetAlertRule(ctx, id)
}

// DeleteAlertRule 删除告警规则
func (s *alertService) DeleteAlertRule(ctx context.Context, id uint64) error {
	if err := s.alertRuleRepo.Delete(ctx, id); err != nil {
		return err
	}
	s.invalidateRulesCache(ctx)
	return nil
}

// ListAlertRules 获取告警规则列表
func (s *alertService) ListAlertRules(ctx context.Context, req *dto.AlertRuleListRequest) (*dto.AlertRuleListResponse, error) {
	rules, total, err := s.alertRuleRepo.List(ctx, req)
	if err != nil {
		return nil, err
	}

	items := make([]dto.AlertRuleResponse, 0, len(rules))
	for _, rule := range rules {
		items = append(items, *s.toAlertRuleResponse(rule))
	}

	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	return &dto.AlertRuleListResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// toAlertRuleResponse 转换为告警规则响应
func (s *alertService) toAlertRuleResponse(rule *model.AlertRule) *dto.AlertRuleResponse {
	var matchers []dto.LabelMatcher
	_ = json.Unmarshal([]byte(rule.LabelMatchers), &matchers)

	resp := &dto.AlertRuleResponse{
		ID:            rule.ID,
		Name:          rule.Name,
		Description:   rule.Description,
		DatasourceID:  rule.DatasourceID,
		ProjectID:     rule.ProjectID,
		IssueTypeID:   rule.IssueTypeID,
		LabelMatchers: matchers,
		Priority:      rule.Priority,
		AssigneeID:    rule.AssigneeID,
		AutoResolve:   rule.AutoResolve,
		MergeWindow:   rule.MergeWindow,
		Status:        rule.Status,
		CreatedAt:     rule.CreatedAt,
		UpdatedAt:     rule.UpdatedAt,
	}

	// 填充数据源名称
	if rule.DatasourceID != nil && *rule.DatasourceID > 0 {
		if ds, err := s.datasourceRepo.GetByID(context.Background(), *rule.DatasourceID); err == nil && ds != nil {
			resp.DatasourceName = ds.Name
		}
	}

	// 填充项目名称
	if project, err := s.projectRepo.GetByID(context.Background(), rule.ProjectID); err == nil && project != nil {
		resp.ProjectName = project.Name
	}

	// 填充工单类型名称
	if issueType, err := s.issueTypeRepo.GetByID(context.Background(), rule.IssueTypeID); err == nil && issueType != nil {
		if issueType.DisplayName != "" {
			resp.IssueTypeName = issueType.DisplayName
		} else {
			resp.IssueTypeName = issueType.Name
		}
	}

	return resp
}
