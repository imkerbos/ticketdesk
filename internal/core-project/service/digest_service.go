// Package service 提供项目级每日工单日报服务
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/core-project/repository"
	"github.com/kerbos/ticketdesk/internal/model"
	"github.com/kerbos/ticketdesk/pkg/i18n"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// 日报作用域
const (
	DigestScopeAllOpen      = "all_open"      // 全部未完结
	DigestScopeAssignedOnly = "assigned_only" // 仅含已指派
)

// 终态 status 黑名单（无 workflow 时 fallback 判定）
var terminalIssueStatuses = []string{"done", "closed", "resolved", "cancelled"} //nolint:misspell // 状态值与前端约定保持一致

// DigestService 每日日报服务
type DigestService interface {
	// RunForProject 立即对指定项目跑一次日报（手动触发，不做去重）；空 issues 不发送
	RunForProject(ctx context.Context, projectID uint64) error
	// RunForProjectScheduled cron 触发入口；通过 daily_digest_runs 表做多副本去重，
	// 同一 (project_id, run_date) 只有一个实例能抢到发送权
	RunForProjectScheduled(ctx context.Context, projectID uint64) error
}

// digestService 实现
type digestService struct {
	db          *gorm.DB
	projectRepo repository.ProjectRepository
	channelRepo repository.NotificationChannelRepository
	runRepo     repository.DigestRunRepository
	notifier    NotificationChannelService
}

// NewDigestService 创建日报服务实例
func NewDigestService(
	db *gorm.DB,
	projectRepo repository.ProjectRepository,
	channelRepo repository.NotificationChannelRepository,
	runRepo repository.DigestRunRepository,
	notifier NotificationChannelService,
) DigestService {
	return &digestService{
		db:          db,
		projectRepo: projectRepo,
		channelRepo: channelRepo,
		runRepo:     runRepo,
		notifier:    notifier,
	}
}

// digestIssueRow 内部查询结果
type digestIssueRow struct {
	IssueKey         string
	Title            string
	Priority         string
	Status           string
	IssueTypeName    string
	NodeName         string
	AssigneeID       *uint64
	AssigneeName     string
	AssigneeEmail    string
	AssigneeLarkID   string
	AssigneeTelegram string
}

// RunForProject 立即执行项目日报（手动触发；不做多副本去重）
func (s *digestService) RunForProject(ctx context.Context, projectID uint64) error {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("项目不存在")
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}
	_, err = s.runForProject(ctx, project)
	return err
}

// RunForProjectScheduled cron 触发入口；先在 daily_digest_runs 表抢占，再执行发送
// 多副本部署下同一 (project_id, run_date) 只有一个实例能抢到；其他实例直接 skip
func (s *digestService) RunForProjectScheduled(ctx context.Context, projectID uint64) error {
	project, err := s.projectRepo.GetByID(ctx, projectID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("项目不存在")
		}
		return fmt.Errorf("查询项目失败: %w", err)
	}

	runDate := projectLocalDate(project)

	claimed, err := s.runRepo.TryClaim(ctx, projectID, runDate)
	if err != nil {
		return fmt.Errorf("日报去重占位失败: %w", err)
	}
	if !claimed {
		logger.Info("daily digest skipped (another instance already claimed)",
			zap.Uint64("project_id", projectID),
			zap.String("project_key", project.ProjectKey),
			zap.String("run_date", runDate),
		)
		return nil
	}

	count, err := s.runForProject(ctx, project)
	if err != nil {
		// 抢到即认定本窗口已被处理；发送失败不释放占位，避免渠道半成功后重发。
		// 需要补发时走手动触发入口（RunForProject）。
		logger.Error("daily digest send failed after claim; not releasing to avoid resend",
			zap.Uint64("project_id", projectID),
			zap.String("run_date", runDate),
			zap.Error(err),
		)
		return err
	}

	// 回写工单数（审计字段；失败仅记日志，不影响主流程）
	if err := s.runRepo.UpdateIssueCount(context.Background(), projectID, runDate, count); err != nil {
		logger.Warn("update daily digest issue_count failed",
			zap.Uint64("project_id", projectID),
			zap.String("run_date", runDate),
			zap.Error(err),
		)
	}
	return nil
}

// runForProject 内部实现：执行一次日报，返回发送的工单数
// 不做去重；手动触发与 cron 触发都复用此方法
func (s *digestService) runForProject(ctx context.Context, project *model.Project) (int, error) {
	projectID := project.ID

	// 前置检查：项目至少要有一个已启用且订阅了 issue.daily_digest 的渠道
	hasSubscriber, err := s.hasDigestSubscriber(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if !hasSubscriber {
		return 0, fmt.Errorf("项目尚未配置订阅「每日日报」事件的通知渠道；请先在「通知渠道」中添加渠道并勾选「每日日报」事件")
	}

	rows, err := s.queryOpenIssues(ctx, project)
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		logger.Info("daily digest: no open issues, skip",
			zap.Uint64("project_id", projectID),
			zap.String("project_key", project.ProjectKey),
		)
		return 0, fmt.Errorf("当前项目没有未完结工单，未发送日报")
	}

	data := buildDigestData(project, rows)
	if err := s.notifier.NotifyProject(ctx, projectID, "issue.daily_digest", data); err != nil {
		return 0, fmt.Errorf("发送日报通知失败: %w", err)
	}

	logger.Info("daily digest sent",
		zap.Uint64("project_id", projectID),
		zap.String("project_key", project.ProjectKey),
		zap.Int("issue_count", len(rows)),
	)
	return len(rows), nil
}

// projectLocalDate 按项目时区计算「今天」字符串（YYYY-MM-DD）
// 用作 daily_digest_runs 表去重键的一部分
func projectLocalDate(project *model.Project) string {
	tz := project.DailyDigestTZ
	if tz == "" {
		tz = defaultDigestTZ
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	return time.Now().In(loc).Format("2006-01-02")
}

// hasDigestSubscriber 判断项目是否存在已启用且订阅 issue.daily_digest 的渠道
func (s *digestService) hasDigestSubscriber(ctx context.Context, projectID uint64) (bool, error) {
	channels, err := s.channelRepo.ListEnabledByProject(ctx, projectID)
	if err != nil {
		return false, fmt.Errorf("查询项目通知渠道失败: %w", err)
	}
	for _, ch := range channels {
		if channelSubscribesEvent(ch, "issue.daily_digest") {
			return true, nil
		}
	}
	return false, nil
}

// parseDigestIssueTypeFilter 解析项目配置的工单类型白名单
// 返回 nil 表示「不过滤」（包含所有类型）
func parseDigestIssueTypeFilter(raw string) []uint64 {
	if raw == "" {
		return nil
	}
	var ids []uint64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// queryOpenIssues 查询项目未完结工单（含 workflow 节点判断 + status fallback）
func (s *digestService) queryOpenIssues(ctx context.Context, project *model.Project) ([]digestIssueRow, error) {
	q := s.db.WithContext(ctx).Table("issues i").
		Select(`i.issue_key, i.title, i.priority, i.status,
			it.display_name AS issue_type_name,
			COALESCE(wn.name, '') AS node_name,
			i.assignee_id,
			COALESCE(u.display_name, u.username, '') AS assignee_name,
			COALESCE(u.email, '') AS assignee_email,
			COALESCE(u.lark_open_id, '') AS assignee_lark_id,
			COALESCE(u.telegram_user_id, '') AS assignee_telegram`).
		Joins("LEFT JOIN issue_types it ON i.issue_type_id = it.id").
		Joins("LEFT JOIN workflow_instances wi ON i.workflow_instance_id = wi.id").
		Joins("LEFT JOIN workflow_nodes wn ON wi.current_node_id = wn.id").
		Joins("LEFT JOIN users u ON i.assignee_id = u.id").
		Where("i.project_id = ?", project.ID).
		Where(`(
			(i.workflow_instance_id IS NOT NULL AND wn.node_type IS NOT NULL AND wn.node_type != 'end')
			OR
			(i.workflow_instance_id IS NULL AND i.status NOT IN ?)
		)`, terminalIssueStatuses).
		Order("i.assignee_id ASC, i.priority ASC, i.id ASC")

	if project.DailyDigestScope == DigestScopeAssignedOnly {
		q = q.Where("i.assignee_id IS NOT NULL")
	}

	// 工单类型白名单：项目配置非空时只查这些类型；空表示全部
	if typeIDs := parseDigestIssueTypeFilter(project.DailyDigestIssueTypeIDs); len(typeIDs) > 0 {
		q = q.Where("i.issue_type_id IN ?", typeIDs)
	}

	var rows []digestIssueRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("查询未完结工单失败: %w", err)
	}
	return rows, nil
}

// buildDigestData 构造发送给 sender 的 data map
// sender 端约定字段：
//   - project_name, project_key, total
//   - groups: []map{"label","assignee_name","mention":{display_name,lark_open_id,email,telegram_user_id},"items":[]map{"issue_key","title","issue_type","priority","status"}}
//   - mentions: 汇总所有有 ID 的指派人（用于消息尾部统一 @）
func buildDigestData(project *model.Project, rows []digestIssueRow) map[string]any {
	// 按 assignee 分组
	type group struct {
		AssigneeID   *uint64
		AssigneeName string
		Mention      map[string]any
		Items        []map[string]any
	}
	groupMap := make(map[uint64]*group)
	var unassigned *group
	for _, r := range rows {
		item := map[string]any{
			"issue_key":  r.IssueKey,
			"title":      r.Title,
			"issue_type": r.IssueTypeName,
			"priority":   r.Priority,
			"status":     r.Status,
			"node_name":  r.NodeName,
		}
		// assignee_name 为空意味着 user 已被删除（LEFT JOIN users 未命中）
		// 此时不再造「用户#X」分组，直接归到「未指派」组
		userMissing := r.AssigneeID != nil && *r.AssigneeID != 0 && r.AssigneeName == ""
		if r.AssigneeID == nil || *r.AssigneeID == 0 || userMissing {
			if unassigned == nil {
				unassigned = &group{AssigneeName: i18n.B("notify.digest_unassigned")}
			}
			unassigned.Items = append(unassigned.Items, item)
			continue
		}
		// 每条 issue 自带 mention，sender 渲染时每行末尾 @ 对应指派人
		if r.AssigneeLarkID != "" || r.AssigneeEmail != "" || r.AssigneeTelegram != "" {
			item["mention"] = map[string]any{
				"display_name":     r.AssigneeName,
				"lark_open_id":     r.AssigneeLarkID,
				"email":            r.AssigneeEmail,
				"telegram_user_id": r.AssigneeTelegram,
			}
		}
		g, ok := groupMap[*r.AssigneeID]
		if !ok {
			name := r.AssigneeName
			if name == "" {
				name = i18n.Bf("notify.digest_user_fallback", *r.AssigneeID)
			}
			g = &group{
				AssigneeID:   r.AssigneeID,
				AssigneeName: name,
			}
			if r.AssigneeLarkID != "" || r.AssigneeEmail != "" || r.AssigneeTelegram != "" {
				g.Mention = map[string]any{
					"display_name":     name,
					"lark_open_id":     r.AssigneeLarkID,
					"email":            r.AssigneeEmail,
					"telegram_user_id": r.AssigneeTelegram,
				}
			}
			groupMap[*r.AssigneeID] = g
		}
		g.Items = append(g.Items, item)
	}

	// 已指派分组按 name 排序，未指派排末尾
	groups := make([]*group, 0, len(groupMap)+1)
	for _, g := range groupMap {
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].AssigneeName < groups[j].AssigneeName
	})
	if unassigned != nil {
		groups = append(groups, unassigned)
	}

	// 序列化为 []map[string]any
	outGroups := make([]map[string]any, 0, len(groups))
	mentions := make([]map[string]any, 0)
	for _, g := range groups {
		gm := map[string]any{
			"assignee_name": g.AssigneeName,
			"items":         g.Items,
		}
		if g.Mention != nil {
			gm["mention"] = g.Mention
			mentions = append(mentions, g.Mention)
		}
		outGroups = append(outGroups, gm)
	}

	return map[string]any{
		"project_key":  project.ProjectKey,
		"project_name": project.Name,
		"total":        len(rows),
		"groups":       outGroups,
		"mentions":     mentions,
	}
}
