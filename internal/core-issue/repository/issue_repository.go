// Package repository 提供工单数据访问层
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/internal/model"
)

// IssueRepository 工单数据访问接口
// ListResult 分页查询结果
type ListResult struct {
	Issues  []*model.Issue
	Total   int64
	HasMore bool // true 表示实际总数超过 maxCount，Total 为封顶值
}

type IssueRepository interface {
	Create(ctx context.Context, issue *model.Issue) error
	GetByID(ctx context.Context, id uint64) (*model.Issue, error)
	GetByKey(ctx context.Context, key string) (*model.Issue, error)
	Update(ctx context.Context, issue *model.Issue) error
	Delete(ctx context.Context, id uint64) error
	List(ctx context.Context, filter *IssueFilter, offset, limit int) (*ListResult, error)
	GetNextIssueNumber(ctx context.Context, projectID uint64) (int64, error)
	ListByIDs(ctx context.Context, ids []uint64) ([]*model.Issue, error)
	ListByParentID(ctx context.Context, parentID uint64) ([]*model.Issue, error)
	ListByEpicID(ctx context.Context, epicID uint64) ([]*model.Issue, error)
	ListByMergedIntoIssueID(ctx context.Context, issueID uint64, result *[]model.Issue) error
	GetListStats(ctx context.Context, filter *IssueFilter) (*ListStats, error)
	GetProjectOverviewStats(ctx context.Context, projectID uint64) (*ProjectOverviewStats, error)
}

// ListStats 工单列表聚合统计
type ListStats struct {
	Total           int64
	Resolved        int64
	AvgResolveHours float64
	// Capped 为 true 表示命中扫描上限，统计值为基于前 maxCountLimit 行的近似结果
	Capped bool `gorm:"-"`
}

// ProjectOverviewStats 项目概述统计（按状态分组）
type ProjectOverviewStats struct {
	Pending       int64
	InProgress    int64
	PendingReview int64
	Completed     int64
	Total         int64
}

// IssueFilter 工单过滤条件
type IssueFilter struct {
	ProjectID       *uint64
	ProjectIDs      []uint64 // 限定项目范围（非管理员用户可访问的项目列表）
	LimitByProjects bool     // 是否启用 ProjectIDs 过滤（区分 nil 和空切片）
	Status          []string // 支持多状态过滤
	StatusNotIn     []string // 排除的状态列表
	Priority        string
	AssigneeID      *uint64
	ReporterID      *uint64
	IssueTypeID     *uint64
	EpicID          *uint64
	Keyword         string
	Category        string     // "normal" = 排除告警工单, "alert" = 仅告警工单
	SortBy          string     // 排序字段
	Order           string     // asc / desc
	StartDate       *time.Time // 创建时间起始
	EndDate         *time.Time // 创建时间截止
}

// issueRepository 工单数据访问实现
type issueRepository struct {
	db *gorm.DB
}

// NewIssueRepository 创建工单数据访问实例
func NewIssueRepository(db *gorm.DB) IssueRepository {
	return &issueRepository{db: db}
}

// Create 创建工单
func (r *issueRepository) Create(ctx context.Context, issue *model.Issue) error {
	return r.db.WithContext(ctx).Create(issue).Error
}

// GetByID 根据 ID 获取工单
func (r *issueRepository) GetByID(ctx context.Context, id uint64) (*model.Issue, error) {
	var issue model.Issue
	err := r.db.WithContext(ctx).First(&issue, id).Error
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

// GetByKey 根据工单 Key 获取工单
func (r *issueRepository) GetByKey(ctx context.Context, key string) (*model.Issue, error) {
	var issue model.Issue
	err := r.db.WithContext(ctx).Where("issue_key = ?", key).First(&issue).Error
	if err != nil {
		return nil, err
	}
	return &issue, nil
}

// Update 更新工单
func (r *issueRepository) Update(ctx context.Context, issue *model.Issue) error {
	return r.db.WithContext(ctx).Save(issue).Error
}

// Delete 硬删除工单
func (r *issueRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.Issue{}, id).Error
}

// maxCountLimit 分页计数上限，超过此值显示 "10,000+"
// 避免 COUNT(*) 全表扫描，保证任意数据量下计数查询 <2ms
const maxCountLimit = 10001

// buildFilterQuery 构建公共过滤条件查询（List 和 GetListStats 共用）
// 返回 nil 表示用户无项目权限，应直接返回空结果
func (r *issueRepository) buildFilterQuery(ctx context.Context, filter *IssueFilter) *gorm.DB {
	query := r.db.WithContext(ctx).Model(&model.Issue{})
	if filter == nil {
		return query
	}

	if filter.ProjectID != nil {
		query = query.Where("project_id = ?", *filter.ProjectID)
	}
	if filter.LimitByProjects {
		if len(filter.ProjectIDs) == 0 {
			return nil // ��户没有任何项目权限
		}
		query = query.Where("project_id IN ?", filter.ProjectIDs)
	}
	if len(filter.Status) == 1 {
		query = query.Where("status = ?", filter.Status[0])
	} else if len(filter.Status) > 1 {
		query = query.Where("status IN ?", filter.Status)
	}
	if len(filter.StatusNotIn) > 0 {
		query = query.Where("status NOT IN ?", filter.StatusNotIn)
	}
	if filter.Priority != "" {
		query = query.Where("priority = ?", filter.Priority)
	}
	if filter.AssigneeID != nil {
		query = query.Where("assignee_id = ?", *filter.AssigneeID)
	}
	if filter.ReporterID != nil {
		query = query.Where("reporter_id = ?", *filter.ReporterID)
	}
	if filter.IssueTypeID != nil {
		query = query.Where("issue_type_id = ?", *filter.IssueTypeID)
	}
	if filter.EpicID != nil {
		query = query.Where("epic_id = ?", *filter.EpicID)
	}
	if filter.Keyword != "" {
		// 注意：前导通配符使索引不可用。实测 34 万行全库搜索约 65ms，
		// 但叠加 project_id 过滤后优化器先走项目索引、只扫该项目的行，约 5ms。
		// 曾尝试改用 ngram 全文索引，实测在「项目内搜索」这一主场景下反而慢 20 倍
		// （全文索引先命中大量行再回表过滤），故保留 LIKE。
		// 若将来全库搜索成为瓶颈，正解是接入外部检索引擎而非 MySQL 全文索引。
		keyword := "%" + filter.Keyword + "%"
		query = query.Where("issue_key LIKE ? OR title LIKE ?", keyword, keyword)
	}
	switch filter.Category {
	case "alert":
		query = query.Where("issue_type_id IN (SELECT id FROM issue_types WHERE name = 'Alert')")
	case "normal":
		query = query.Where("issue_type_id NOT IN (SELECT id FROM issue_types WHERE name = 'Alert')")
	}
	if filter.StartDate != nil {
		query = query.Where("created_at >= ?", *filter.StartDate)
	}
	if filter.EndDate != nil {
		query = query.Where("created_at <= ?", *filter.EndDate)
	}

	return query
}

// List 分页查询工单列表
func (r *issueRepository) List(ctx context.Context, filter *IssueFilter, offset, limit int) (*ListResult, error) {
	var issues []*model.Issue
	result := &ListResult{}

	query := r.buildFilterQuery(ctx, filter)
	if query == nil {
		return result, nil
	}

	// 封顶计数：最多扫描 maxCountLimit 行，避免百万级 COUNT 全扫描
	var cappedCount int64
	countSubQuery := query.Session(&gorm.Session{}).Select("1").Limit(maxCountLimit)
	if err := r.db.WithContext(ctx).Table("(?) AS capped", countSubQuery).Count(&cappedCount).Error; err != nil {
		return nil, err
	}

	if cappedCount >= int64(maxCountLimit) {
		result.Total = int64(maxCountLimit - 1) // 10000
		result.HasMore = true
	} else {
		result.Total = cappedCount
	}

	// 排序
	orderClause := "id DESC" // 默认按 ID 倒序
	if filter != nil && filter.SortBy != "" {
		// 白名单校验（DTO 层已校验，这里双重保护防注入）
		allowedSortFields := map[string]string{
			"id":            "id",
			"priority":      "FIELD(priority,'P0','P1','P2','P3')",
			"status":        "FIELD(status,'open','in_progress','pending_review','resolved','closed')",
			"issue_type_id": "issue_type_id",
			"created_at":    "created_at",
			"updated_at":    "updated_at",
		}
		if col, ok := allowedSortFields[filter.SortBy]; ok {
			dir := "DESC"
			if filter.Order == "asc" {
				dir = "ASC"
			}
			orderClause = fmt.Sprintf("%s %s", col, dir)
		}
	}

	// 分页查询
	if err := query.Offset(offset).Limit(limit).Order(orderClause).Find(&issues).Error; err != nil {
		return nil, err
	}
	result.Issues = issues

	return result, nil
}

// GetListStats 根据过滤条件聚合统计工单数据
//
// 与 List 一样对扫描行数封顶：原实现对同一组过滤条件做无上限的
// COUNT + SUM + AVG(TIMESTAMPDIFF)，在大表上是一次全量扫描，
// 恰好抵消了 List 里做的封顶计数优化。
// 这里先用子查询取最多 maxCountLimit 行，再在其上聚合。
func (r *issueRepository) GetListStats(ctx context.Context, filter *IssueFilter) (*ListStats, error) {
	query := r.buildFilterQuery(ctx, filter)
	if query == nil {
		return &ListStats{}, nil
	}

	sub := query.Session(&gorm.Session{}).
		Select("status", "created_at", "resolved_at").
		Limit(maxCountLimit)

	var stats ListStats
	err := r.db.WithContext(ctx).Table("(?) AS capped", sub).Select(
		"COUNT(*) AS total",
		"SUM(CASE WHEN status IN ('resolved','closed') THEN 1 ELSE 0 END) AS resolved",
		"COALESCE(AVG(CASE WHEN resolved_at IS NOT NULL THEN TIMESTAMPDIFF(SECOND, created_at, resolved_at) END) / 3600, 0) AS avg_resolve_hours",
	).Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	if stats.Total >= int64(maxCountLimit) {
		stats.Total = int64(maxCountLimit - 1)
		stats.Capped = true
	}
	return &stats, nil
}

// GetNextIssueNumber 获取项目下一个工单编号（包含软删除记录，避免唯一键冲突）
func (r *issueRepository) GetNextIssueNumber(ctx context.Context, projectID uint64) (int64, error) {
	var maxNum sql.NullInt64
	// 从 issue_key 中提取最大序号，格式为 "PROJECT-123"
	err := r.db.WithContext(ctx).Unscoped().Model(&model.Issue{}).
		Where("project_id = ?", projectID).
		Select("MAX(CAST(SUBSTRING_INDEX(issue_key, '-', -1) AS UNSIGNED))").
		Scan(&maxNum).Error
	if err != nil {
		return 0, err
	}
	if !maxNum.Valid {
		return 1, nil
	}
	return maxNum.Int64 + 1, nil
}

// ListByIDs 根据 ID 列表批量查询工单
func (r *issueRepository) ListByIDs(ctx context.Context, ids []uint64) ([]*model.Issue, error) {
	var issues []*model.Issue
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&issues).Error
	return issues, err
}

// ListByParentID 根据父工单 ID 获取所有子任务
func (r *issueRepository) ListByParentID(ctx context.Context, parentID uint64) ([]*model.Issue, error) {
	var issues []*model.Issue
	err := r.db.WithContext(ctx).Where("parent_id = ?", parentID).Order("created_at ASC").Find(&issues).Error
	return issues, err
}

// ListByEpicID 根据 Epic ID 获取所有关联工单
func (r *issueRepository) ListByEpicID(ctx context.Context, epicID uint64) ([]*model.Issue, error) {
	var issues []*model.Issue
	err := r.db.WithContext(ctx).Where("epic_id = ?", epicID).Order("created_at ASC").Find(&issues).Error
	return issues, err
}

// ListByMergedIntoIssueID 查找所有合并到指定工单的旧工单
func (r *issueRepository) ListByMergedIntoIssueID(ctx context.Context, issueID uint64, result *[]model.Issue) error {
	return r.db.WithContext(ctx).
		Where("merged_into_issue_id = ?", issueID).
		Order("created_at ASC").
		Find(result).Error
}

// GetProjectOverviewStats 获取项目概述统计（单条 SQL 条件聚合，按状态分组）
func (r *issueRepository) GetProjectOverviewStats(ctx context.Context, projectID uint64) (*ProjectOverviewStats, error) {
	var stats ProjectOverviewStats
	err := r.db.WithContext(ctx).Model(&model.Issue{}).
		Where("project_id = ?", projectID).
		Select(
			"COUNT(*) AS total",
			"SUM(CASE WHEN status IN ('open','reopened') THEN 1 ELSE 0 END) AS pending",
			"SUM(CASE WHEN status = 'in_progress' THEN 1 ELSE 0 END) AS in_progress",
			"SUM(CASE WHEN status = 'pending_review' THEN 1 ELSE 0 END) AS pending_review",
			"SUM(CASE WHEN status IN ('resolved','closed') THEN 1 ELSE 0 END) AS completed",
		).Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// CommentRepository 评论数据访问接口
type CommentRepository interface {
	Create(ctx context.Context, comment *model.IssueComment) error
	GetByID(ctx context.Context, id uint64) (*model.IssueComment, error)
	Update(ctx context.Context, comment *model.IssueComment) error
	Delete(ctx context.Context, id uint64) error
	ListByIssue(ctx context.Context, issueID uint64) ([]*model.IssueComment, error)
}

// commentRepository 评论数据访问实现
type commentRepository struct {
	db *gorm.DB
}

// NewCommentRepository 创建评论数据访问实例
func NewCommentRepository(db *gorm.DB) CommentRepository {
	return &commentRepository{db: db}
}

// Create 创建评论
func (r *commentRepository) Create(ctx context.Context, comment *model.IssueComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

// GetByID 根据 ID 获取评论
func (r *commentRepository) GetByID(ctx context.Context, id uint64) (*model.IssueComment, error) {
	var comment model.IssueComment
	err := r.db.WithContext(ctx).First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// Update 更新评论
func (r *commentRepository) Update(ctx context.Context, comment *model.IssueComment) error {
	return r.db.WithContext(ctx).Save(comment).Error
}

// Delete 硬删除评论
func (r *commentRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.IssueComment{}, id).Error
}

// ListByIssue 获取工单的所有评论
func (r *commentRepository) ListByIssue(ctx context.Context, issueID uint64) ([]*model.IssueComment, error) {
	var comments []*model.IssueComment
	err := r.db.WithContext(ctx).Where("issue_id = ?", issueID).Order("id ASC").Find(&comments).Error
	return comments, err
}

// WatcherRepository 关注人数据访问接口
type WatcherRepository interface {
	Create(ctx context.Context, watcher *model.IssueWatcher) error
	Delete(ctx context.Context, id uint64) error
	DeleteByIssueAndUser(ctx context.Context, issueID, userID uint64) error
	ListByIssue(ctx context.Context, issueID uint64) ([]*model.IssueWatcher, error)
	IsWatching(ctx context.Context, issueID, userID uint64) (bool, error)
	GetByIssueAndUser(ctx context.Context, issueID, userID uint64) (*model.IssueWatcher, error)
}

// watcherRepository 关注人数据访问实现
type watcherRepository struct {
	db *gorm.DB
}

// NewWatcherRepository 创建关注人数据访问实例
func NewWatcherRepository(db *gorm.DB) WatcherRepository {
	return &watcherRepository{db: db}
}

// Create 创建关注人
func (r *watcherRepository) Create(ctx context.Context, watcher *model.IssueWatcher) error {
	return r.db.WithContext(ctx).Create(watcher).Error
}

// Delete 硬删除关注人
func (r *watcherRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.IssueWatcher{}, id).Error
}

// DeleteByIssueAndUser 根据工单和用户硬删除关注
func (r *watcherRepository) DeleteByIssueAndUser(ctx context.Context, issueID, userID uint64) error {
	return r.db.WithContext(ctx).Unscoped().Where("issue_id = ? AND user_id = ?", issueID, userID).Delete(&model.IssueWatcher{}).Error
}

// ListByIssue 获取工单的所有关注人
func (r *watcherRepository) ListByIssue(ctx context.Context, issueID uint64) ([]*model.IssueWatcher, error) {
	var watchers []*model.IssueWatcher
	err := r.db.WithContext(ctx).Where("issue_id = ?", issueID).Order("id ASC").Find(&watchers).Error
	return watchers, err
}

// IsWatching 检查用户是否关注工单
func (r *watcherRepository) IsWatching(ctx context.Context, issueID, userID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.IssueWatcher{}).
		Where("issue_id = ? AND user_id = ?", issueID, userID).Count(&count).Error
	return count > 0, err
}

// GetByIssueAndUser 根据工单和用户获取关注记录
func (r *watcherRepository) GetByIssueAndUser(ctx context.Context, issueID, userID uint64) (*model.IssueWatcher, error) {
	var watcher model.IssueWatcher
	err := r.db.WithContext(ctx).Where("issue_id = ? AND user_id = ?", issueID, userID).First(&watcher).Error
	if err != nil {
		return nil, err
	}
	return &watcher, nil
}

// WorklogRepository 工作日志数据访问接口
type WorklogRepository interface {
	Create(ctx context.Context, worklog *model.IssueWorklog) error
	GetByID(ctx context.Context, id uint64) (*model.IssueWorklog, error)
	Update(ctx context.Context, worklog *model.IssueWorklog) error
	Delete(ctx context.Context, id uint64) error
	ListByIssue(ctx context.Context, issueID uint64) ([]*model.IssueWorklog, error)
	GetTotalTimeSpent(ctx context.Context, issueID uint64) (int, error)
}

// worklogRepository 工作日志数据访问实现
type worklogRepository struct {
	db *gorm.DB
}

// NewWorklogRepository 创建工作日志数据访问实例
func NewWorklogRepository(db *gorm.DB) WorklogRepository {
	return &worklogRepository{db: db}
}

// Create 创建工作日志
func (r *worklogRepository) Create(ctx context.Context, worklog *model.IssueWorklog) error {
	return r.db.WithContext(ctx).Create(worklog).Error
}

// GetByID 根据 ID 获取工作日志
func (r *worklogRepository) GetByID(ctx context.Context, id uint64) (*model.IssueWorklog, error) {
	var worklog model.IssueWorklog
	err := r.db.WithContext(ctx).First(&worklog, id).Error
	if err != nil {
		return nil, err
	}
	return &worklog, nil
}

// Update 更新工作日志
func (r *worklogRepository) Update(ctx context.Context, worklog *model.IssueWorklog) error {
	return r.db.WithContext(ctx).Save(worklog).Error
}

// Delete 硬删除工作日志
func (r *worklogRepository) Delete(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&model.IssueWorklog{}, id).Error
}

// ListByIssue 获取工单的所有工作日志
func (r *worklogRepository) ListByIssue(ctx context.Context, issueID uint64) ([]*model.IssueWorklog, error) {
	var worklogs []*model.IssueWorklog
	err := r.db.WithContext(ctx).
		Where("issue_id = ?", issueID).
		Order("worked_at DESC, id DESC").
		Find(&worklogs).Error
	return worklogs, err
}

// GetTotalTimeSpent 获取工单的总工作时长（秒）
func (r *worklogRepository) GetTotalTimeSpent(ctx context.Context, issueID uint64) (int, error) {
	var total int
	err := r.db.WithContext(ctx).
		Model(&model.IssueWorklog{}).
		Where("issue_id = ?", issueID).
		Select("COALESCE(SUM(time_spent_sec), 0)").
		Scan(&total).Error
	return total, err
}

// GenerateIssueKey 生成工单 Key
func GenerateIssueKey(projectKey string, number int64) string {
	return fmt.Sprintf("%s-%d", projectKey, number)
}
