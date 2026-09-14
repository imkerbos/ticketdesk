// Package repository 提供报表数据访问层
package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

// ReportRepository 报表数据访问接口
type ReportRepository interface {
	// 工单统计
	CountIssuesByStatus(ctx context.Context, projectID *uint64) (map[string]int64, error)
	CountIssuesByPriority(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error)
	CountIssuesByType(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error)
	CountIssuesByAssignee(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error)
	CountIssuesByEpic(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error)
	CountIssuesCreatedByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error)
	CountIssuesInProgressByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error)
	CountIssuesResolvedByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error)
	CountIssuesClosedByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error)
	GetAverageResolveTime(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (float64, error)

	// SLA 统计
	GetSLAStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (*SLAStats, error)
	GetSLAStatsByPriority(ctx context.Context, projectID *uint64, startDate, endDate time.Time, slaTargets map[string]int64) ([]PrioritySLAStats, error)
	GetSLAStatsByProject(ctx context.Context, projectID *uint64, startDate, endDate time.Time, slaTargets map[string]int64) ([]ProjectSLAStats, error)
	GetSLAViolations(ctx context.Context, projectID *uint64, startDate, endDate time.Time, slaTargets map[string]int64) ([]SLAViolationRecord, error)

	// 告警统计
	CountAlertsByStatus(ctx context.Context, projectID *uint64) (map[string]int64, error)
	CountAlertsBySeverity(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error)
	CountAlertsByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error)
	GetTopAlerts(ctx context.Context, projectID *uint64, startDate, endDate time.Time, limit int) ([]AlertCount, error)
	GetAverageAckTime(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (float64, error)

	// 项目统计
	CountProjects(ctx context.Context) (int64, error)
	CountProjectMembers(ctx context.Context) (int64, error)

	// 用户绩效
	GetUserPerformance(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]UserPerformance, error)

	// 工时统计
	GetWorklogDailyStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogDailyStat, error)
	GetWorklogUserStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogUserStat, error)
	GetWorklogTypeStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogTypeStat, error)
	GetWorklogSummary(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (*WorklogSummaryData, error)
	GetWorklogDailyUserStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogDailyUserStat, error)

	// 交付报表（周报 / 月报）
	GetDeliveryAggregate(ctx context.Context, projectID *uint64, start, end time.Time) (*DeliveryAggregate, error)
	GetDeliveryLateIssues(ctx context.Context, projectID *uint64, start, end time.Time, limit int) ([]DeliveryVarianceRow, error)
	GetDeliveryByMember(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryMemberRow, error)
	GetDeliveryRisks(ctx context.Context, projectID *uint64, horizonDays, limit int) ([]DeliveryRiskRow, error)
	GetDeliveryByProject(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryProjectRow, error)
	GetDeliveredIssues(ctx context.Context, projectID *uint64, start, end time.Time, limit int) ([]DeliveredIssueRow, error)
	GetDeliveryByPriority(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryDimensionRow, error)
	GetDeliveryByType(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryDimensionRow, error)
	GetWorklogSecondsByUser(ctx context.Context, projectID *uint64, start, end time.Time) (map[uint64]int64, error)
}

// DateCount 日期计数
type DateCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// AlertCount 告警计数
type AlertCount struct {
	AlertName string `json:"alert_name"`
	Severity  string `json:"severity"`
	Count     int64  `json:"count"`
}

// SLAStats SLA 统计数据
type SLAStats struct {
	TotalIssues    int64
	ResolvedIssues int64
	TotalMTTA      float64 // 平均确认时间（分钟）
	TotalMTTR      float64 // 平均解决时间（分钟）
}

// PrioritySLAStats 按优先级的 SLA 统计
type PrioritySLAStats struct {
	Priority string
	Total    int64
	Resolved int64
	MTTA     float64
	MTTR     float64
	SLAMet   int64
}

// ProjectSLAStats 按项目的 SLA 统计
type ProjectSLAStats struct {
	ProjectKey  string
	ProjectName string
	Total       int64
	Resolved    int64
	MTTR        float64
	SLAMet      int64
}

// SLAViolationRecord SLA 违规工单记录
type SLAViolationRecord struct {
	IssueKey     string
	Title        string
	Priority     string
	AssigneeName string
	ActualTime   float64 // 实际耗时（分钟）
}

// UserPerformance 用户绩效数据
type UserPerformance struct {
	UserID         uint64
	Username       string
	DisplayName    string
	Assigned       int64
	Resolved       int64
	AvgResolveTime float64
}

// WorklogDailyStat 每日工时统计
type WorklogDailyStat struct {
	Date         string
	TotalTimeSec int64
	EntryCount   int64
}

// WorklogUserStat 用户工时统计
type WorklogUserStat struct {
	UserID       uint64
	DisplayName  string
	TotalTimeSec int64
	EntryCount   int64
}

// WorklogTypeStat 工时类型统计
type WorklogTypeStat struct {
	WorkType     string
	TotalTimeSec int64
	EntryCount   int64
}

// WorklogSummaryData 工时汇总数据
type WorklogSummaryData struct {
	TotalTimeSec int64
	TotalEntries int64
	ActiveUsers  int64
}

// WorklogDailyUserStat 用户每日工时明细（按工单粒度）
type WorklogDailyUserStat struct {
	UserID       uint64
	DisplayName  string
	Date         string
	IssueKey     string
	IssueTitle   string
	TotalTimeSec int64
}

// reportRepository 报表数据访问实现
type reportRepository struct {
	db *gorm.DB
}

// NewReportRepository 创建报表数据访问实例
func NewReportRepository(db *gorm.DB) ReportRepository {
	return &reportRepository{db: db}
}

// CountIssuesByStatus 按状态统计工单数量
func (r *reportRepository) CountIssuesByStatus(ctx context.Context, projectID *uint64) (map[string]int64, error) {
	type Result struct {
		Status string
		Count  int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("issues").
		Select("status, COUNT(*) as count")

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Group("status").Find(&results).Error
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]int64)
	for _, res := range results {
		statusMap[res.Status] = res.Count
	}

	return statusMap, nil
}

// CountIssuesByPriority 按优先级统计工单数量
func (r *reportRepository) CountIssuesByPriority(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error) {
	type Result struct {
		Priority string
		Count    int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("issues").
		Select("priority, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Group("priority").Find(&results).Error
	if err != nil {
		return nil, err
	}

	priorityMap := make(map[string]int64)
	for _, res := range results {
		priorityMap[res.Priority] = res.Count
	}

	return priorityMap, nil
}

// CountIssuesByType 按类型统计工单数量
func (r *reportRepository) CountIssuesByType(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error) {
	type Result struct {
		TypeName string
		Count    int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("issues").
		Select("issue_types.display_name as type_name, COUNT(*) as count").
		Joins("LEFT JOIN issue_types ON issues.issue_type_id = issue_types.id").
		Where("issues.created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("issue_types.display_name").Find(&results).Error
	if err != nil {
		return nil, err
	}

	typeMap := make(map[string]int64)
	for _, res := range results {
		typeMap[res.TypeName] = res.Count
	}

	return typeMap, nil
}

// CountIssuesByAssignee 按指派人统计工单数量
func (r *reportRepository) CountIssuesByAssignee(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error) {
	type Result struct {
		DisplayName string
		Count       int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("issues").
		Select("COALESCE(users.display_name, users.username, '未指派') as display_name, COUNT(*) as count").
		Joins("LEFT JOIN users ON issues.assignee_id = users.id").
		Where("issues.created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("COALESCE(users.display_name, users.username, '未指派')").Find(&results).Error
	if err != nil {
		return nil, err
	}

	assigneeMap := make(map[string]int64)
	for _, res := range results {
		assigneeMap[res.DisplayName] = res.Count
	}

	return assigneeMap, nil
}

// CountIssuesByEpic 按 Epic 统计工单数量
func (r *reportRepository) CountIssuesByEpic(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error) {
	type Result struct {
		EpicTitle string
		Count     int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("issues").
		Select("COALESCE(epics.title, '无 Epic') as epic_title, COUNT(*) as count").
		Joins("LEFT JOIN issues AS epics ON issues.epic_id = epics.id").
		Where("issues.created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("COALESCE(epics.title, '无 Epic')").Find(&results).Error
	if err != nil {
		return nil, err
	}

	epicMap := make(map[string]int64)
	for _, res := range results {
		epicMap[res.EpicTitle] = res.Count
	}

	return epicMap, nil
}

// CountIssuesCreatedByDate 按日期统计创建的工单数量
func (r *reportRepository) CountIssuesCreatedByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error) {
	var results []DateCount
	query := r.db.WithContext(ctx).Table("issues").
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Group("DATE(created_at)").Order("date").Find(&results).Error
	return results, err
}

// CountIssuesInProgressByDate 按日期统计进入进行中状态的工单数量（以 actual_start_date 为准）
func (r *reportRepository) CountIssuesInProgressByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error) {
	var results []DateCount
	query := r.db.WithContext(ctx).Table("issues").
		Select("DATE(actual_start_date) as date, COUNT(*) as count").
		Where("actual_start_date IS NOT NULL").
		Where("actual_start_date BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Group("DATE(actual_start_date)").Order("date").Find(&results).Error
	return results, err
}

// CountIssuesResolvedByDate 按日期统计解决的工单数量
func (r *reportRepository) CountIssuesResolvedByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error) {
	var results []DateCount
	query := r.db.WithContext(ctx).Table("issues").
		Select("DATE(resolved_at) as date, COUNT(*) as count").
		Where("resolved_at IS NOT NULL").
		Where("resolved_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Group("DATE(resolved_at)").Order("date").Find(&results).Error
	return results, err
}

// CountIssuesClosedByDate 按日期统计关闭的工单数量
func (r *reportRepository) CountIssuesClosedByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error) {
	var results []DateCount
	query := r.db.WithContext(ctx).Table("issues").
		Select("DATE(closed_at) as date, COUNT(*) as count").
		Where("closed_at IS NOT NULL").
		Where("closed_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Group("DATE(closed_at)").Order("date").Find(&results).Error
	return results, err
}

// GetAverageResolveTime 获取平均解决时间（小时）
func (r *reportRepository) GetAverageResolveTime(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (float64, error) {
	var result struct {
		AvgTime float64
	}

	query := r.db.WithContext(ctx).Table("issues").
		Select("AVG(TIMESTAMPDIFF(HOUR, created_at, resolved_at)) as avg_time").
		Where("resolved_at IS NOT NULL").
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Take(&result).Error
	if err != nil {
		return 0, err
	}

	return result.AvgTime, nil
}

// GetSLAStats 获取 SLA 统计数据
func (r *reportRepository) GetSLAStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (*SLAStats, error) {
	var result struct {
		TotalIssues    int64
		ResolvedIssues int64
		AvgMTTA        float64
		AvgMTTR        float64
	}

	query := r.db.WithContext(ctx).Table("issues").
		Select(`
			COUNT(*) as total_issues,
			SUM(CASE WHEN resolved_at IS NOT NULL THEN 1 ELSE 0 END) as resolved_issues,
			AVG(CASE WHEN actual_start_date IS NOT NULL THEN TIMESTAMPDIFF(MINUTE, created_at, actual_start_date) ELSE NULL END) as avg_mtta,
			AVG(CASE WHEN resolved_at IS NOT NULL THEN TIMESTAMPDIFF(MINUTE, created_at, resolved_at) ELSE NULL END) as avg_mttr
		`).
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	err := query.Take(&result).Error
	if err != nil {
		return nil, err
	}

	return &SLAStats{
		TotalIssues:    result.TotalIssues,
		ResolvedIssues: result.ResolvedIssues,
		TotalMTTA:      result.AvgMTTA,
		TotalMTTR:      result.AvgMTTR,
	}, nil
}

// GetSLAStatsByPriority 按优先级获取 SLA 统计（逐条判断 SLA 达标）
func (r *reportRepository) GetSLAStatsByPriority(ctx context.Context, projectID *uint64, startDate, endDate time.Time, slaTargets map[string]int64) ([]PrioritySLAStats, error) {
	deadlineExpr, slaArgs := buildDeadlineExpr(slaTargets, "")

	// 达标 = 解决时刻没晚于承诺截止时刻。不再拿「从开单到解决花了多久」去比一个
	// 按优先级写死的阈值 —— 那样一张排期十天的 P1 必然违规，哪怕它提前两天交付。
	selectClause := fmt.Sprintf(`
		priority,
		COUNT(*) as total,
		SUM(CASE WHEN resolved_at IS NOT NULL THEN 1 ELSE 0 END) as resolved,
		AVG(CASE WHEN actual_start_date IS NOT NULL THEN TIMESTAMPDIFF(MINUTE, created_at, actual_start_date) ELSE NULL END) as avg_mtta,
		AVG(CASE WHEN resolved_at IS NOT NULL THEN TIMESTAMPDIFF(MINUTE, created_at, resolved_at) ELSE NULL END) as avg_mttr,
		SUM(CASE WHEN resolved_at IS NOT NULL AND resolved_at <= (%s) THEN 1 ELSE 0 END) as sla_met
	`, deadlineExpr)

	var results []struct {
		Priority string
		Total    int64
		Resolved int64
		AvgMTTA  float64
		AvgMTTR  float64
		SLAMet   int64 `gorm:"column:sla_met"`
	}

	query := r.db.WithContext(ctx).Table("issues").
		Select(selectClause, slaArgs...).
		Where("created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("project_id = ?", *projectID)
	}

	// 不加 ORDER BY 时 MySQL 的分组结果顺序不保证，前端表格会出现 P0/P1/P3/P2。
	// 优先级是 P0..P3，字典序即严重程度序。
	err := query.Group("priority").Order("priority").Find(&results).Error
	if err != nil {
		return nil, err
	}

	stats := make([]PrioritySLAStats, len(results))
	for i, res := range results {
		stats[i] = PrioritySLAStats{
			Priority: res.Priority,
			Total:    res.Total,
			Resolved: res.Resolved,
			MTTA:     res.AvgMTTA,
			MTTR:     res.AvgMTTR,
			SLAMet:   res.SLAMet,
		}
	}

	return stats, nil
}

// CountAlertsByStatus 按状态统计告警数量
func (r *reportRepository) CountAlertsByStatus(ctx context.Context, projectID *uint64) (map[string]int64, error) {
	type Result struct {
		Status string
		Count  int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("alerts").
		Select("status, COUNT(*) as count")

	err := query.Group("status").Find(&results).Error
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]int64)
	for _, res := range results {
		statusMap[res.Status] = res.Count
	}

	return statusMap, nil
}

// CountAlertsBySeverity 按严重程度统计告警数量
func (r *reportRepository) CountAlertsBySeverity(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (map[string]int64, error) {
	type Result struct {
		Severity string
		Count    int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Table("alerts").
		Select("severity, COUNT(*) as count").
		Where("starts_at BETWEEN ? AND ?", startDate, endDate)

	err := query.Group("severity").Find(&results).Error
	if err != nil {
		return nil, err
	}

	severityMap := make(map[string]int64)
	for _, res := range results {
		severityMap[res.Severity] = res.Count
	}

	return severityMap, nil
}

// CountAlertsByDate 按日期统计告警数量
func (r *reportRepository) CountAlertsByDate(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]DateCount, error) {
	var results []DateCount
	query := r.db.WithContext(ctx).Table("alerts").
		Select("DATE(starts_at) as date, COUNT(*) as count").
		Where("starts_at BETWEEN ? AND ?", startDate, endDate)

	err := query.Group("DATE(starts_at)").Order("date").Find(&results).Error
	return results, err
}

// GetTopAlerts 获取告警排名（按告警名称+严重程度分组）
func (r *reportRepository) GetTopAlerts(ctx context.Context, projectID *uint64, startDate, endDate time.Time, limit int) ([]AlertCount, error) {
	var results []AlertCount
	query := r.db.WithContext(ctx).Table("alerts").
		Select("alert_name, severity, COUNT(*) as count").
		Where("starts_at BETWEEN ? AND ?", startDate, endDate)

	err := query.Group("alert_name, severity").Order("count DESC").Limit(limit).Find(&results).Error
	return results, err
}

// GetAverageAckTime 获取平均确认时间（分钟）
func (r *reportRepository) GetAverageAckTime(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (float64, error) {
	var result struct {
		AvgTime float64
	}

	query := r.db.WithContext(ctx).Table("alerts").
		Select("AVG(TIMESTAMPDIFF(MINUTE, starts_at, acked_at)) as avg_time").
		Where("acked_at IS NOT NULL").
		Where("starts_at BETWEEN ? AND ?", startDate, endDate)

	err := query.Take(&result).Error
	if err != nil {
		return 0, err
	}

	return result.AvgTime, nil
}

// CountProjects 统计项目总数
func (r *reportRepository) CountProjects(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("projects").
		Count(&count).Error
	return count, err
}

// CountProjectMembers 统计项目成员总数（去重）
func (r *reportRepository) CountProjectMembers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("project_members").
		Select("COUNT(DISTINCT user_id)").
		Take(&count).Error
	return count, err
}

// GetUserPerformance 获取用户绩效
func (r *reportRepository) GetUserPerformance(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]UserPerformance, error) {
	var results []struct {
		UserID         uint64
		Username       string
		DisplayName    string
		Assigned       int64
		Resolved       int64
		AvgResolveTime float64
	}

	query := r.db.WithContext(ctx).Table("issues").
		Select(`
			users.id as user_id,
			users.username,
			users.display_name,
			COUNT(*) as assigned,
			SUM(CASE WHEN issues.resolved_at IS NOT NULL THEN 1 ELSE 0 END) as resolved,
			AVG(CASE WHEN issues.resolved_at IS NOT NULL THEN TIMESTAMPDIFF(HOUR, issues.created_at, issues.resolved_at) ELSE NULL END) as avg_resolve_time
		`).
		Joins("JOIN users ON issues.assignee_id = users.id").
		Where("issues.created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("users.id, users.username, users.display_name").Find(&results).Error
	if err != nil {
		return nil, err
	}

	performance := make([]UserPerformance, len(results))
	for i, res := range results {
		performance[i] = UserPerformance{
			UserID:         res.UserID,
			Username:       res.Username,
			DisplayName:    res.DisplayName,
			Assigned:       res.Assigned,
			Resolved:       res.Resolved,
			AvgResolveTime: res.AvgResolveTime,
		}
	}

	return performance, nil
}

// GetWorklogDailyStats 按日期统计工时
func (r *reportRepository) GetWorklogDailyStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogDailyStat, error) {
	var results []WorklogDailyStat
	query := r.db.WithContext(ctx).Table("issue_worklogs").
		Select("DATE(worked_at) as date, SUM(time_spent_sec) as total_time_sec, COUNT(*) as entry_count").
		Where("worked_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Joins("JOIN issues ON issue_worklogs.issue_id = issues.id").
			Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("DATE(worked_at)").Order("date").Find(&results).Error
	return results, err
}

// GetWorklogUserStats 按用户统计工时
func (r *reportRepository) GetWorklogUserStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogUserStat, error) {
	var results []WorklogUserStat
	query := r.db.WithContext(ctx).Table("issue_worklogs").
		Select("issue_worklogs.user_id, COALESCE(users.display_name, users.username, '未知') as display_name, SUM(issue_worklogs.time_spent_sec) as total_time_sec, COUNT(*) as entry_count").
		Joins("LEFT JOIN users ON issue_worklogs.user_id = users.id").
		Where("issue_worklogs.worked_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Joins("JOIN issues ON issue_worklogs.issue_id = issues.id").
			Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("issue_worklogs.user_id").Order("total_time_sec DESC").Find(&results).Error
	return results, err
}

// GetWorklogTypeStats 按工作类型统计工时
func (r *reportRepository) GetWorklogTypeStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogTypeStat, error) {
	var results []WorklogTypeStat
	query := r.db.WithContext(ctx).Table("issue_worklogs").
		Select("COALESCE(NULLIF(work_type, ''), '未分类') as work_type, SUM(time_spent_sec) as total_time_sec, COUNT(*) as entry_count").
		Where("worked_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Joins("JOIN issues ON issue_worklogs.issue_id = issues.id").
			Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("work_type").Order("total_time_sec DESC").Find(&results).Error
	return results, err
}

// GetWorklogSummary 获取工时汇总数据
func (r *reportRepository) GetWorklogSummary(ctx context.Context, projectID *uint64, startDate, endDate time.Time) (*WorklogSummaryData, error) {
	var result WorklogSummaryData
	query := r.db.WithContext(ctx).Table("issue_worklogs").
		Select("COALESCE(SUM(time_spent_sec), 0) as total_time_sec, COUNT(*) as total_entries, COUNT(DISTINCT user_id) as active_users").
		Where("worked_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Joins("JOIN issues ON issue_worklogs.issue_id = issues.id").
			Where("issues.project_id = ?", *projectID)
	}

	err := query.Take(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSLAStatsByProject 按项目获取 SLA 统计
func (r *reportRepository) GetSLAStatsByProject(ctx context.Context, projectID *uint64, startDate, endDate time.Time, slaTargets map[string]int64) ([]ProjectSLAStats, error) {
	deadlineExpr, slaArgs := buildDeadlineExpr(slaTargets, "issues.")

	selectClause := fmt.Sprintf(`
		projects.project_key,
		projects.name as project_name,
		COUNT(*) as total,
		SUM(CASE WHEN issues.resolved_at IS NOT NULL THEN 1 ELSE 0 END) as resolved,
		AVG(CASE WHEN issues.resolved_at IS NOT NULL THEN TIMESTAMPDIFF(MINUTE, issues.created_at, issues.resolved_at) ELSE NULL END) as mttr,
		SUM(CASE WHEN issues.resolved_at IS NOT NULL AND issues.resolved_at <= (%s) THEN 1 ELSE 0 END) as sla_met
	`, deadlineExpr)

	var results []ProjectSLAStats
	query := r.db.WithContext(ctx).Table("issues").
		Select(selectClause, slaArgs...).
		Joins("JOIN projects ON issues.project_id = projects.id").
		Where("issues.created_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("projects.id, projects.project_key, projects.name").Find(&results).Error
	return results, err
}

// GetSLAViolations 查询 SLA 违规工单列表
func (r *reportRepository) GetSLAViolations(ctx context.Context, projectID *uint64, startDate, endDate time.Time, slaTargets map[string]int64) ([]SLAViolationRecord, error) {
	deadlineExpr, slaArgs := buildDeadlineExpr(slaTargets, "issues.")

	actualTimeExpr := "CASE WHEN issues.resolved_at IS NOT NULL THEN TIMESTAMPDIFF(MINUTE, issues.created_at, issues.resolved_at) ELSE TIMESTAMPDIFF(MINUTE, issues.created_at, NOW()) END"
	// 已解决的比解决时刻，没解决的比现在 —— 后者才能把「还欠着且已经超了」捞出来
	settleExpr := "COALESCE(issues.resolved_at, NOW())"

	selectClause := fmt.Sprintf(`
		issues.issue_key,
		issues.title,
		issues.priority,
		COALESCE(users.display_name, users.username, '未指派') as assignee_name,
		(%s) as actual_time
	`, actualTimeExpr)

	whereClause := fmt.Sprintf("(%s) > (%s) AND (issues.resolved_at IS NOT NULL OR issues.status NOT IN ('closed', 'merged'))", settleExpr, deadlineExpr)

	var results []SLAViolationRecord
	query := r.db.WithContext(ctx).Table("issues").
		Select(selectClause).
		Joins("LEFT JOIN users ON issues.assignee_id = users.id").
		Where("issues.created_at BETWEEN ? AND ?", startDate, endDate).
		Where(whereClause, slaArgs...)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Order("actual_time DESC").Limit(50).Find(&results).Error
	return results, err
}

// GetWorklogDailyUserStats 按用户+日期+工单查询工时明细
func (r *reportRepository) GetWorklogDailyUserStats(ctx context.Context, projectID *uint64, startDate, endDate time.Time) ([]WorklogDailyUserStat, error) {
	var results []WorklogDailyUserStat
	query := r.db.WithContext(ctx).Table("issue_worklogs").
		Select(`issue_worklogs.user_id,
			COALESCE(users.display_name, users.username, '未知') as display_name,
			DATE_FORMAT(issue_worklogs.worked_at, '%Y-%m-%d') as date,
			issues.issue_key,
			COALESCE(issues.title, '') as issue_title,
			SUM(issue_worklogs.time_spent_sec) as total_time_sec`).
		Joins("LEFT JOIN users ON issue_worklogs.user_id = users.id").
		Joins("LEFT JOIN issues ON issue_worklogs.issue_id = issues.id").
		Where("issue_worklogs.worked_at BETWEEN ? AND ?", startDate, endDate)

	if projectID != nil {
		query = query.Where("issues.project_id = ?", *projectID)
	}

	err := query.Group("issue_worklogs.user_id, users.display_name, users.username, DATE_FORMAT(issue_worklogs.worked_at, '%Y-%m-%d'), issues.issue_key, issues.title").
		Order("users.display_name, date, total_time_sec DESC").
		Find(&results).Error
	return results, err
}

// buildDeadlineExpr 构建工单的「承诺截止时刻」表达式。
//
// 取值优先级和前端 web/src/utils/sla.ts 保持一致，两边必须是同一套判定：
//
//  1. planned_end_date  开单时评估的交付日期，最硬
//  2. due_date          用户明确设的截止日期
//  3. created_at + 按优先级的默认 SLA
//
// 前两者是「日期」不是「时刻」，按当天 23:59:59 算 —— 截止当天还没过完就不算超时。
//
// 为什么不能只用第 3 条：优先级表达的是「这件事多重要」，不是「几小时内必须交付」。
// 一张排期十天的 P1，按固定 4 小时阈值判必然违规，哪怕它提前两天交付了。
func buildDeadlineExpr(slaTargets map[string]int64, prefix string) (string, []any) {
	targetExpr, args := buildSLATargetExpr(slaTargets, prefix+"priority")
	expr := fmt.Sprintf(`COALESCE(
		CASE WHEN %[1]splanned_end_date IS NOT NULL
		     THEN TIMESTAMP(DATE(%[1]splanned_end_date), '23:59:59') END,
		CASE WHEN %[1]sdue_date IS NOT NULL
		     THEN TIMESTAMP(DATE(%[1]sdue_date), '23:59:59') END,
		%[1]screated_at + INTERVAL (%[2]s) MINUTE
	)`, prefix, targetExpr)
	return expr, args
}

// buildSLATargetExpr 构建基于优先级的 SLA 目标 CASE 表达式（返回分钟数）
func buildSLATargetExpr(slaTargets map[string]int64, priorityColumn string) (string, []interface{}) {
	expr := "CASE " + priorityColumn
	keys := make([]string, 0, len(slaTargets))
	for k := range slaTargets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	args := make([]interface{}, 0, len(slaTargets)*2)
	for _, k := range keys {
		expr += " WHEN ? THEN ?"
		args = append(args, k, slaTargets[k])
	}
	expr += " ELSE 99999 END"
	return expr, args
}

// ============ 交付报表（周报 / 月报）============

// DeliveryAggregate 一个周期内的交付聚合
type DeliveryAggregate struct {
	Delivered       int64
	Terminated      int64
	Created         int64
	OnTime          int64
	Late            int64
	NoCommitment    int64
	AlertIssues     int64
	AvgVarianceDays float64
	AvgDeliveryDays float64
}

// deliveredScope 「本期交付」的口径。
//
// 只认 resolved：这个产品里 closed 显示成「已终止」，是砍掉而不是交付完成
// （见 web 语言包 issue.statusMap）。把终止也算成交付，一张被砍掉的需求
// 就会变成「按时交付」。终止数单独统计，报表里单独一行。
const deliveredScope = "issues.status = 'resolved'"

// varianceDaysExpr 承诺交付日与实际完成日相差几天：正数提前、负数延期。
// 承诺是「日期」不是「时刻」，两边都按自然日算，当天完成即准时。
const varianceDaysExpr = "DATEDIFF(DATE(issues.planned_end_date), DATE(issues.actual_end_date))"

func (r *reportRepository) scopeProject(q *gorm.DB, projectID *uint64) *gorm.DB {
	if projectID != nil {
		return q.Where("issues.project_id = ?", *projectID)
	}
	return q
}

// GetDeliveryAggregate 统计一个周期内的交付情况
func (r *reportRepository) GetDeliveryAggregate(ctx context.Context, projectID *uint64, start, end time.Time) (*DeliveryAggregate, error) {
	var agg DeliveryAggregate

	// 交付侧：按实际完成时间落在周期内统计
	deliveredQ := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Joins("LEFT JOIN issue_types ON issue_types.id = issues.issue_type_id").
		Where(deliveredScope).
		Where("issues.actual_end_date BETWEEN ? AND ?", start, end), projectID)

	var row struct {
		Delivered    int64
		OnTime       int64
		Late         int64
		NoCommitment int64
		AlertIssues  int64
		AvgVariance  float64
		AvgDelivery  float64
	}
	err := deliveredQ.Select(fmt.Sprintf(`
		COUNT(*) as delivered,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[1]s) >= 0 THEN 1 ELSE 0 END) as on_time,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[1]s) <  0 THEN 1 ELSE 0 END) as late,
		SUM(CASE WHEN issues.planned_end_date IS NULL THEN 1 ELSE 0 END) as no_commitment,
		SUM(CASE WHEN issue_types.name = 'Alert' THEN 1 ELSE 0 END) as alert_issues,
		AVG(CASE WHEN issues.planned_end_date IS NOT NULL THEN (%[1]s) ELSE NULL END) as avg_variance,
		AVG(CASE WHEN issues.actual_start_date IS NOT NULL
		         THEN TIMESTAMPDIFF(HOUR, issues.actual_start_date, issues.actual_end_date) / 24
		         ELSE NULL END) as avg_delivery
	`, varianceDaysExpr)).Scan(&row).Error
	if err != nil {
		return nil, err
	}
	agg.Delivered = row.Delivered
	agg.OnTime = row.OnTime
	agg.Late = row.Late
	agg.NoCommitment = row.NoCommitment
	agg.AlertIssues = row.AlertIssues
	agg.AvgVarianceDays = row.AvgVariance
	agg.AvgDeliveryDays = row.AvgDelivery

	// 终止数
	if err := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Where("issues.status = 'closed'").
		Where("issues.closed_at BETWEEN ? AND ?", start, end), projectID).
		Count(&agg.Terminated).Error; err != nil {
		return nil, err
	}

	// 新开数
	if err := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Where("issues.created_at BETWEEN ? AND ?", start, end), projectID).
		Count(&agg.Created).Error; err != nil {
		return nil, err
	}

	return &agg, nil
}

// DeliveryVarianceRow 偏差明细
type DeliveryVarianceRow struct {
	IssueKey     string
	Title        string
	Priority     string
	ProjectKey   string
	AssigneeName string
	PlannedEnd   time.Time
	ActualEnd    time.Time
	VarianceDays int64
}

// GetDeliveryLateIssues 本期延期最多的工单
func (r *reportRepository) GetDeliveryLateIssues(ctx context.Context, projectID *uint64, start, end time.Time, limit int) ([]DeliveryVarianceRow, error) {
	var rows []DeliveryVarianceRow
	q := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Joins("JOIN projects ON projects.id = issues.project_id").
		Joins("LEFT JOIN users ON users.id = issues.assignee_id").
		Where(deliveredScope).
		Where("issues.planned_end_date IS NOT NULL").
		Where("issues.actual_end_date BETWEEN ? AND ?", start, end).
		Where(fmt.Sprintf("(%s) < 0", varianceDaysExpr)), projectID)

	err := q.Select(fmt.Sprintf(`
		issues.issue_key, issues.title, issues.priority,
		projects.project_key,
		COALESCE(users.display_name, users.username, '') as assignee_name,
		issues.planned_end_date as planned_end,
		issues.actual_end_date as actual_end,
		(%s) as variance_days
	`, varianceDaysExpr)).
		Order("variance_days ASC").Limit(limit).Scan(&rows).Error
	return rows, err
}

// DeliveryMemberRow 人员交付明细
type DeliveryMemberRow struct {
	UserID      uint64
	DisplayName string
	Delivered   int64
	OnTime      int64
	Late        int64
}

// GetDeliveryByMember 按指派人统计本期交付
func (r *reportRepository) GetDeliveryByMember(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryMemberRow, error) {
	var rows []DeliveryMemberRow
	q := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Joins("JOIN users ON users.id = issues.assignee_id").
		Where(deliveredScope).
		Where("issues.actual_end_date BETWEEN ? AND ?", start, end), projectID)

	err := q.Select(fmt.Sprintf(`
		users.id as user_id,
		COALESCE(users.display_name, users.username) as display_name,
		COUNT(*) as delivered,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[1]s) >= 0 THEN 1 ELSE 0 END) as on_time,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[1]s) <  0 THEN 1 ELSE 0 END) as late
	`, varianceDaysExpr)).
		Group("users.id, display_name").Order("delivered DESC").Scan(&rows).Error
	return rows, err
}

// DeliveryRiskRow 风险工单
type DeliveryRiskRow struct {
	IssueKey     string
	Title        string
	Priority     string
	ProjectKey   string
	AssigneeName string
	Status       string
	PlannedEnd   time.Time
	DaysLeft     int64
}

// GetDeliveryRisks 未交付且有承诺的工单，按「还剩几天」升序 —— 已经超期的排最前
func (r *reportRepository) GetDeliveryRisks(ctx context.Context, projectID *uint64, horizonDays, limit int) ([]DeliveryRiskRow, error) {
	var rows []DeliveryRiskRow
	q := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Joins("JOIN projects ON projects.id = issues.project_id").
		Joins("LEFT JOIN users ON users.id = issues.assignee_id").
		Where("issues.status NOT IN ('resolved','closed','merged')").
		Where("issues.planned_end_date IS NOT NULL").
		Where("DATEDIFF(DATE(issues.planned_end_date), CURDATE()) <= ?", horizonDays), projectID)

	err := q.Select(`
		issues.issue_key, issues.title, issues.priority, issues.status,
		projects.project_key,
		COALESCE(users.display_name, users.username, '') as assignee_name,
		issues.planned_end_date as planned_end,
		DATEDIFF(DATE(issues.planned_end_date), CURDATE()) as days_left
	`).Order("days_left ASC").Limit(limit).Scan(&rows).Error
	return rows, err
}

// DeliveryProjectRow 项目横向对比
type DeliveryProjectRow struct {
	ProjectKey  string
	ProjectName string
	Delivered   int64
	OnTime      int64
	Late        int64
}

// GetDeliveryByProject 按项目统计本期交付
func (r *reportRepository) GetDeliveryByProject(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryProjectRow, error) {
	var rows []DeliveryProjectRow
	q := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Joins("JOIN projects ON projects.id = issues.project_id").
		Where(deliveredScope).
		Where("issues.actual_end_date BETWEEN ? AND ?", start, end), projectID)

	err := q.Select(fmt.Sprintf(`
		projects.project_key, projects.name as project_name,
		COUNT(*) as delivered,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[1]s) >= 0 THEN 1 ELSE 0 END) as on_time,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[1]s) <  0 THEN 1 ELSE 0 END) as late
	`, varianceDaysExpr)).
		Group("projects.id, projects.project_key, projects.name").
		Order("delivered DESC").Scan(&rows).Error
	return rows, err
}

// GetWorklogSecondsByUser 本期每人填报的工时（秒）
func (r *reportRepository) GetWorklogSecondsByUser(ctx context.Context, projectID *uint64, start, end time.Time) (map[uint64]int64, error) {
	var rows []struct {
		UserID  uint64
		Seconds int64
	}
	q := r.db.WithContext(ctx).Table("issue_worklogs").
		Joins("JOIN issues ON issues.id = issue_worklogs.issue_id").
		Where("issue_worklogs.worked_at BETWEEN ? AND ?", start, end)
	q = r.scopeProject(q, projectID)

	if err := q.Select("issue_worklogs.user_id as user_id, SUM(issue_worklogs.time_spent_sec) as seconds").
		Group("issue_worklogs.user_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[uint64]int64, len(rows))
	for _, row := range rows {
		out[row.UserID] = row.Seconds
	}
	return out, nil
}

// DeliveredIssueRow 本期交付的工单
type DeliveredIssueRow struct {
	IssueKey     string
	Title        string
	TypeName     string
	Priority     string
	ProjectKey   string
	AssigneeName string
	PlannedEnd   *time.Time
	ActualEnd    time.Time
	VarianceDays *int64
}

// GetDeliveredIssues 本期交付的工单清单，按实际完成时间倒序。
// 没有承诺交付日的也要列出来 —— 它们同样是本期的产出，只是不参与准时率。
func (r *reportRepository) GetDeliveredIssues(ctx context.Context, projectID *uint64, start, end time.Time, limit int) ([]DeliveredIssueRow, error) {
	var rows []DeliveredIssueRow
	q := r.scopeProject(r.db.WithContext(ctx).Table("issues").
		Joins("JOIN projects ON projects.id = issues.project_id").
		Joins("LEFT JOIN issue_types ON issue_types.id = issues.issue_type_id").
		Joins("LEFT JOIN users ON users.id = issues.assignee_id").
		Where(deliveredScope).
		Where("issues.actual_end_date BETWEEN ? AND ?", start, end), projectID)

	err := q.Select(fmt.Sprintf(`
		issues.issue_key, issues.title, issues.priority,
		COALESCE(issue_types.display_name, issue_types.name, '') as type_name,
		projects.project_key,
		COALESCE(users.display_name, users.username, '') as assignee_name,
		issues.planned_end_date as planned_end,
		issues.actual_end_date as actual_end,
		CASE WHEN issues.planned_end_date IS NULL THEN NULL ELSE (%s) END as variance_days
	`, varianceDaysExpr)).
		Order("issues.actual_end_date DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}

// DeliveryDimensionRow 按维度切分的交付统计
type DeliveryDimensionRow struct {
	Key       string
	Delivered int64
	OnTime    int64
	Late      int64
}

// deliveryByDimension 按指定列分组统计本期交付。
// 优先级和类型两种切分只差一个分组列，不值得各写一份 SQL。
func (r *reportRepository) deliveryByDimension(ctx context.Context, projectID *uint64, start, end time.Time, keyExpr, groupBy string, extraJoin string) ([]DeliveryDimensionRow, error) {
	var rows []DeliveryDimensionRow
	q := r.db.WithContext(ctx).Table("issues")
	if extraJoin != "" {
		q = q.Joins(extraJoin)
	}
	q = r.scopeProject(q.
		Where(deliveredScope).
		Where("issues.actual_end_date BETWEEN ? AND ?", start, end), projectID)

	err := q.Select(fmt.Sprintf(`
		%[1]s as %[3]s,
		COUNT(*) as delivered,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[2]s) >= 0 THEN 1 ELSE 0 END) as on_time,
		SUM(CASE WHEN issues.planned_end_date IS NOT NULL AND (%[2]s) <  0 THEN 1 ELSE 0 END) as late
	`, keyExpr, varianceDaysExpr, "`key`")).
		Group(groupBy).Order("delivered DESC").Scan(&rows).Error
	return rows, err
}

// GetDeliveryByPriority 按优先级统计本期交付
func (r *reportRepository) GetDeliveryByPriority(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryDimensionRow, error) {
	return r.deliveryByDimension(ctx, projectID, start, end, "issues.priority", "issues.priority", "")
}

// GetDeliveryByType 按工单类型统计本期交付
func (r *reportRepository) GetDeliveryByType(ctx context.Context, projectID *uint64, start, end time.Time) ([]DeliveryDimensionRow, error) {
	return r.deliveryByDimension(ctx, projectID, start, end,
		"COALESCE(issue_types.display_name, issue_types.name, '')",
		"issue_types.id, issue_types.display_name, issue_types.name",
		"LEFT JOIN issue_types ON issue_types.id = issues.issue_type_id")
}
