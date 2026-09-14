// Package service 提供报表业务逻辑层
package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"

	projectRepo "github.com/kerbos/ticketdesk/internal/core-project/repository"
	"github.com/kerbos/ticketdesk/internal/reporting/dto"
	"github.com/kerbos/ticketdesk/internal/reporting/repository"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// SLA 目标配置（分钟）
var slaTargets = map[string]int64{
	"P0": 60,   // 1 小时
	"P1": 240,  // 4 小时
	"P2": 1440, // 24 小时
	"P3": 4320, // 72 小时
}

// ReportService 报表服务接口
type ReportService interface {
	GetDashboardStats(ctx context.Context, req *dto.DashboardStatsRequest) (*dto.DashboardStatsResponse, error)
	GetIssueStats(ctx context.Context, req *dto.IssueStatsRequest) (*dto.IssueStatsResponse, error)
	GetSLAReport(ctx context.Context, req *dto.SLAReportRequest) (*dto.SLAReportResponse, error)
	// GetDeliveryReport 交付报表（周报 / 月报）
	GetDeliveryReport(ctx context.Context, req *dto.DeliveryReportRequest) (*dto.DeliveryReportResponse, error)
	GetAlertStats(ctx context.Context, req *dto.AlertStatsRequest) (*dto.AlertStatsResponse, error)
	GetUserPerformance(ctx context.Context, req *dto.IssueStatsRequest) ([]*dto.UserPerformanceResponse, error)
	GetWorklogStats(ctx context.Context, req *dto.WorklogStatsRequest) (*dto.WorklogStatsResponse, error)
}

// reportService 报表服务实现
type reportService struct {
	reportRepo  repository.ReportRepository
	projectRepo projectRepo.ProjectRepository
}

// NewReportService 创建报表服务实例
func NewReportService(
	reportRepo repository.ReportRepository,
	projectRepo projectRepo.ProjectRepository,
) ReportService {
	return &reportService{
		reportRepo:  reportRepo,
		projectRepo: projectRepo,
	}
}

// GetDashboardStats 获取仪表盘统计数据
func (s *reportService) GetDashboardStats(ctx context.Context, req *dto.DashboardStatsRequest) (*dto.DashboardStatsResponse, error) {
	resp := &dto.DashboardStatsResponse{}

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	// 工单统计
	issueStats, err := s.reportRepo.CountIssuesByStatus(ctx, projectID)
	if err != nil {
		logger.Error("failed to count issues by status", zap.Error(err))
		return nil, err
	}
	resp.IssueStats.TotalOpen = issueStats["open"]
	resp.IssueStats.TotalInProgress = issueStats["in_progress"]
	resp.IssueStats.TotalResolved = issueStats["resolved"]
	resp.IssueStats.TotalClosed = issueStats["closed"]

	// 今日和本周统计
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := todayStart.Add(24 * time.Hour)
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	weekEnd := weekStart.AddDate(0, 0, 7)

	todayCreated, err := s.reportRepo.CountIssuesCreatedByDate(ctx, projectID, todayStart, todayEnd)
	if err != nil {
		logger.Warn("failed to count today created issues", zap.Error(err))
	}
	for _, item := range todayCreated {
		resp.IssueStats.TodayCreated += item.Count
	}

	weekCreated, err := s.reportRepo.CountIssuesCreatedByDate(ctx, projectID, weekStart, weekEnd)
	if err != nil {
		logger.Warn("failed to count week created issues", zap.Error(err))
	}
	for _, item := range weekCreated {
		resp.IssueStats.WeekCreated += item.Count
	}

	weekResolved, err := s.reportRepo.CountIssuesResolvedByDate(ctx, projectID, weekStart, weekEnd)
	if err != nil {
		logger.Warn("failed to count week resolved issues", zap.Error(err))
	}
	for _, item := range weekResolved {
		resp.IssueStats.WeekResolved += item.Count
	}

	// 告警统计
	alertStats, err := s.reportRepo.CountAlertsByStatus(ctx, projectID)
	if err != nil {
		logger.Warn("failed to count alerts by status", zap.Error(err))
	} else {
		resp.AlertStats.TotalFiring = alertStats["firing"]
		resp.AlertStats.TotalAcked = alertStats["acked"]
		resp.AlertStats.TotalResolved = alertStats["resolved"]
	}

	todayAlerts, err := s.reportRepo.CountAlertsByDate(ctx, projectID, todayStart, todayEnd)
	if err != nil {
		logger.Warn("failed to count today alerts", zap.Error(err))
	}
	for _, item := range todayAlerts {
		resp.AlertStats.TodayCreated += item.Count
	}

	weekAlerts, err := s.reportRepo.CountAlertsByDate(ctx, projectID, weekStart, weekEnd)
	if err != nil {
		logger.Warn("failed to count week alerts", zap.Error(err))
	}
	for _, item := range weekAlerts {
		resp.AlertStats.WeekCreated += item.Count
	}

	// 项目统计
	resp.ProjectStats.TotalProjects, err = s.reportRepo.CountProjects(ctx)
	if err != nil {
		logger.Warn("failed to count projects", zap.Error(err))
	}
	resp.ProjectStats.TotalMembers, err = s.reportRepo.CountProjectMembers(ctx)
	if err != nil {
		logger.Warn("failed to count project members", zap.Error(err))
	}

	return resp, nil
}

// GetIssueStats 获取工单统计数据
func (s *reportService) GetIssueStats(ctx context.Context, req *dto.IssueStatsRequest) (*dto.IssueStatsResponse, error) {
	resp := &dto.IssueStatsResponse{}

	// 解析日期范围
	dateRange := s.parseDateRange(req.StartDate, req.EndDate)

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	// 获取状态分布
	statusMap, err := s.reportRepo.CountIssuesByStatus(ctx, projectID)
	if err != nil {
		return nil, err
	}
	resp.Summary.Open = statusMap["open"]
	resp.Summary.InProgress = statusMap["in_progress"]
	resp.Summary.Resolved = statusMap["resolved"]
	resp.Summary.Closed = statusMap["closed"]
	resp.Summary.Total = resp.Summary.Open + resp.Summary.InProgress + resp.Summary.Resolved + resp.Summary.Closed

	// 获取平均解决时间
	resp.Summary.AvgResolveTime, err = s.reportRepo.GetAverageResolveTime(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get average resolve time", zap.Error(err))
	}

	// 获取时间线数据
	createdByDate, err := s.reportRepo.CountIssuesCreatedByDate(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get created issue timeline", zap.Error(err))
	}
	inProgressByDate, err := s.reportRepo.CountIssuesInProgressByDate(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get in-progress issue timeline", zap.Error(err))
	}
	resolvedByDate, err := s.reportRepo.CountIssuesResolvedByDate(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get resolved issue timeline", zap.Error(err))
	}
	closedByDate, err := s.reportRepo.CountIssuesClosedByDate(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get closed issue timeline", zap.Error(err))
	}

	// 合并时间线数据
	resp.Timeline = s.mergeTimelineData(createdByDate, inProgressByDate, resolvedByDate, closedByDate)

	// 获取优先级分布
	priorityMap, err := s.reportRepo.CountIssuesByPriority(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to count issues by priority", zap.Error(err))
	}
	resp.PriorityDistribution = s.mapToDistribution(priorityMap)

	// 获取类型分布
	typeMap, err := s.reportRepo.CountIssuesByType(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to count issues by type", zap.Error(err))
	}
	resp.TypeDistribution = s.mapToDistribution(typeMap)

	// 获取状态分布
	resp.StatusDistribution = s.mapToDistribution(statusMap)

	// 获取指派人分布
	assigneeMap, err := s.reportRepo.CountIssuesByAssignee(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to count issues by assignee", zap.Error(err))
	}
	resp.AssigneeDistribution = s.mapToDistribution(assigneeMap)

	// 获取 Epic 分布
	epicMap, err := s.reportRepo.CountIssuesByEpic(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to count issues by epic", zap.Error(err))
	}
	resp.EpicDistribution = s.mapToDistribution(epicMap)

	return resp, nil
}

// GetSLAReport 获取 SLA 报表
func (s *reportService) GetSLAReport(ctx context.Context, req *dto.SLAReportRequest) (*dto.SLAReportResponse, error) {
	resp := &dto.SLAReportResponse{}

	// 解析日期
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("开始日期格式错误，应为 YYYY-MM-DD: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("结束日期格式错误，应为 YYYY-MM-DD: %w", err)
	}
	endDate = endDate.Add(24*time.Hour - time.Second) // 包含结束日期当天

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	// 获取 SLA 统计
	slaStats, err := s.reportRepo.GetSLAStats(ctx, projectID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	resp.Summary.TotalIssues = slaStats.TotalIssues
	resp.Summary.ResolvedIssues = slaStats.ResolvedIssues
	resp.Summary.MTTA = slaStats.TotalMTTA
	resp.Summary.MTTR = slaStats.TotalMTTR

	// 按优先级统计（逐条判断 SLA 达标）
	priorityStats, err := s.reportRepo.GetSLAStatsByPriority(ctx, projectID, startDate, endDate, slaTargets)
	if err != nil {
		logger.Warn("failed to get sla stats by priority", zap.Error(err))
	}
	resp.ByPriority = make([]dto.SLAPriorityStats, len(priorityStats))
	for i, ps := range priorityStats {
		slaTarget := slaTargets[ps.Priority]
		slaRate := float64(0)
		if ps.Resolved > 0 {
			slaRate = float64(ps.SLAMet) / float64(ps.Resolved) * 100
		}

		resp.ByPriority[i] = dto.SLAPriorityStats{
			Priority:  ps.Priority,
			Total:     ps.Total,
			Resolved:  ps.Resolved,
			MTTA:      ps.MTTA,
			MTTR:      ps.MTTR,
			SLATarget: slaTarget,
			SLAMet:    ps.SLAMet,
			SLARate:   slaRate,
		}

		resp.Summary.SLAMet += ps.SLAMet
	}

	if resp.Summary.ResolvedIssues > 0 {
		resp.Summary.SLARate = float64(resp.Summary.SLAMet) / float64(resp.Summary.ResolvedIssues) * 100
	}
	resp.Summary.SLAViolated = resp.Summary.ResolvedIssues - resp.Summary.SLAMet

	// 按项目统计
	projectStats, err := s.reportRepo.GetSLAStatsByProject(ctx, projectID, startDate, endDate, slaTargets)
	if err != nil {
		logger.Warn("failed to get sla stats by project", zap.Error(err))
	}
	resp.ByProject = make([]dto.SLAProjectStats, len(projectStats))
	for i, ps := range projectStats {
		slaRate := float64(0)
		if ps.Resolved > 0 {
			slaRate = float64(ps.SLAMet) / float64(ps.Resolved) * 100
		}
		resp.ByProject[i] = dto.SLAProjectStats{
			ProjectKey:  ps.ProjectKey,
			ProjectName: ps.ProjectName,
			Total:       ps.Total,
			Resolved:    ps.Resolved,
			MTTR:        ps.MTTR,
			SLARate:     slaRate,
		}
	}

	// SLA 违规工单列表
	violations, err := s.reportRepo.GetSLAViolations(ctx, projectID, startDate, endDate, slaTargets)
	if err != nil {
		logger.Warn("failed to get sla violations", zap.Error(err))
	}
	resp.Violations = make([]dto.SLAViolation, len(violations))
	for i, v := range violations {
		slaTarget := slaTargets[v.Priority]
		overdueBy := v.ActualTime - float64(slaTarget)
		if overdueBy < 0 {
			overdueBy = 0
		}
		resp.Violations[i] = dto.SLAViolation{
			IssueKey:     v.IssueKey,
			Title:        v.Title,
			Priority:     v.Priority,
			AssigneeName: v.AssigneeName,
			SLATarget:    slaTarget,
			ActualTime:   v.ActualTime,
			OverdueBy:    overdueBy,
		}
	}

	return resp, nil
}

// GetAlertStats 获取告警统计
func (s *reportService) GetAlertStats(ctx context.Context, req *dto.AlertStatsRequest) (*dto.AlertStatsResponse, error) {
	resp := &dto.AlertStatsResponse{}

	dateRange := s.parseDateRange(req.StartDate, req.EndDate)

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	// 获取状态分布
	statusMap, err := s.reportRepo.CountAlertsByStatus(ctx, projectID)
	if err != nil {
		return nil, err
	}
	resp.Summary.Firing = statusMap["firing"]
	resp.Summary.Acked = statusMap["acked"]
	resp.Summary.Resolved = statusMap["resolved"]
	resp.Summary.Total = resp.Summary.Firing + resp.Summary.Acked + resp.Summary.Resolved

	// 获取平均确认时间
	resp.Summary.AvgAckTime, err = s.reportRepo.GetAverageAckTime(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get average ack time", zap.Error(err))
	}

	// 获取时间线数据
	alertsByDate, err := s.reportRepo.CountAlertsByDate(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to count alerts by date", zap.Error(err))
	}
	resp.Timeline = make([]dto.AlertTimelineItem, len(alertsByDate))
	for i, item := range alertsByDate {
		resp.Timeline[i] = dto.AlertTimelineItem{
			Date:   item.Date,
			Firing: item.Count,
		}
	}

	// 获取严重程度分布
	severityMap, err := s.reportRepo.CountAlertsBySeverity(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to count alerts by severity", zap.Error(err))
	}
	resp.SeverityDistribution = s.mapToDistribution(severityMap)

	// 获取 Top 告警
	topAlerts, err := s.reportRepo.GetTopAlerts(ctx, projectID, dateRange.StartDate, dateRange.EndDate, 10)
	if err != nil {
		logger.Warn("failed to get top alerts", zap.Error(err))
	}
	resp.TopAlerts = make([]dto.TopAlertItem, len(topAlerts))
	for i, item := range topAlerts {
		resp.TopAlerts[i] = dto.TopAlertItem{
			AlertName: item.AlertName,
			Severity:  item.Severity,
			Count:     item.Count,
		}
	}

	return resp, nil
}

// GetUserPerformance 获取用户绩效
func (s *reportService) GetUserPerformance(ctx context.Context, req *dto.IssueStatsRequest) ([]*dto.UserPerformanceResponse, error) {
	dateRange := s.parseDateRange(req.StartDate, req.EndDate)

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	performance, err := s.reportRepo.GetUserPerformance(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		return nil, err
	}

	resp := make([]*dto.UserPerformanceResponse, len(performance))
	for i, p := range performance {
		resp[i] = &dto.UserPerformanceResponse{
			UserID:         p.UserID,
			Username:       p.Username,
			DisplayName:    p.DisplayName,
			Assigned:       p.Assigned,
			Resolved:       p.Resolved,
			AvgResolveTime: p.AvgResolveTime,
		}
	}

	return resp, nil
}

// parseDateRange 解析日期范围
func (s *reportService) parseDateRange(startDateStr, endDateStr string) dto.DateRange {
	now := time.Now()
	endDate := now

	// 默认最近 30 天
	startDate := now.AddDate(0, 0, -30)

	if startDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = parsed
		}
	}

	if endDateStr != "" {
		if parsed, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = parsed.Add(24*time.Hour - time.Second)
		}
	}

	return dto.DateRange{
		StartDate: startDate,
		EndDate:   endDate,
	}
}

// parseGridMonth 解析工时明细月份，返回该月第一天和最后一天
func (s *reportService) parseGridMonth(gridMonth string) (time.Time, time.Time) {
	now := time.Now()
	year, month := now.Year(), now.Month()
	if gridMonth != "" {
		if parsed, err := time.Parse("2006-01", gridMonth); err == nil {
			year, month = parsed.Year(), parsed.Month()
		}
	}
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
	lastDay := firstDay.AddDate(0, 1, -1)
	return firstDay, lastDay
}

// mergeTimelineData 合并时间线数据
func (s *reportService) mergeTimelineData(created, inProgress, resolved, closed []repository.DateCount) []dto.TimelineItem {
	dateMap := make(map[string]*dto.TimelineItem)

	for _, item := range created {
		if _, exists := dateMap[item.Date]; !exists {
			dateMap[item.Date] = &dto.TimelineItem{Date: item.Date}
		}
		dateMap[item.Date].Created = item.Count
	}

	for _, item := range inProgress {
		if _, exists := dateMap[item.Date]; !exists {
			dateMap[item.Date] = &dto.TimelineItem{Date: item.Date}
		}
		dateMap[item.Date].InProgress = item.Count
	}

	for _, item := range resolved {
		if _, exists := dateMap[item.Date]; !exists {
			dateMap[item.Date] = &dto.TimelineItem{Date: item.Date}
		}
		dateMap[item.Date].Resolved = item.Count
	}

	for _, item := range closed {
		if _, exists := dateMap[item.Date]; !exists {
			dateMap[item.Date] = &dto.TimelineItem{Date: item.Date}
		}
		dateMap[item.Date].Closed = item.Count
	}

	result := make([]dto.TimelineItem, 0, len(dateMap))
	for _, item := range dateMap {
		result = append(result, *item)
	}

	// 四个来源合并时走的是 map，Go 的 map 迭代顺序是随机的，
	// 直接返回会让前端的「工单趋势」表格日期乱序（09-02、08-18、08-19…）。
	// 日期是 YYYY-MM-DD，字典序即时间序。
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result
}

// GetWorklogStats 获取工时统计数据
func (s *reportService) GetWorklogStats(ctx context.Context, req *dto.WorklogStatsRequest) (*dto.WorklogStatsResponse, error) {
	resp := &dto.WorklogStatsResponse{}

	dateRange := s.parseDateRange(req.StartDate, req.EndDate)

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	// 获取汇总数据
	summary, err := s.reportRepo.GetWorklogSummary(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Error("failed to get worklog summary", zap.Error(err))
		return nil, err
	}
	resp.Summary.TotalTimeSec = summary.TotalTimeSec
	resp.Summary.TotalEntries = summary.TotalEntries
	resp.Summary.ActiveUsers = summary.ActiveUsers

	// 计算日均工时
	days := dateRange.EndDate.Sub(dateRange.StartDate).Hours() / 24
	if days > 0 && summary.TotalTimeSec > 0 {
		resp.Summary.AvgDailyTimeSec = float64(summary.TotalTimeSec) / days
	}

	// 获取每日统计
	dailyStats, err := s.reportRepo.GetWorklogDailyStats(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get worklog daily stats", zap.Error(err))
	} else {
		resp.DailyStats = make([]dto.DailyWorklogStat, len(dailyStats))
		for i, d := range dailyStats {
			resp.DailyStats[i] = dto.DailyWorklogStat{
				Date:         d.Date,
				TotalTimeSec: d.TotalTimeSec,
				EntryCount:   d.EntryCount,
			}
		}
	}

	// 获取用户统计
	userStats, err := s.reportRepo.GetWorklogUserStats(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get worklog user stats", zap.Error(err))
	} else {
		resp.UserStats = make([]dto.UserWorklogStat, len(userStats))
		for i, u := range userStats {
			resp.UserStats[i] = dto.UserWorklogStat{
				UserID:       u.UserID,
				DisplayName:  u.DisplayName,
				TotalTimeSec: u.TotalTimeSec,
				EntryCount:   u.EntryCount,
			}
		}
	}

	// 获取类型统计
	typeStats, err := s.reportRepo.GetWorklogTypeStats(ctx, projectID, dateRange.StartDate, dateRange.EndDate)
	if err != nil {
		logger.Warn("failed to get worklog type stats", zap.Error(err))
	} else {
		resp.TypeStats = make([]dto.WorklogTypeStat, len(typeStats))
		for i, t := range typeStats {
			resp.TypeStats[i] = dto.WorklogTypeStat{
				WorkType:     t.WorkType,
				TotalTimeSec: t.TotalTimeSec,
				EntryCount:   t.EntryCount,
			}
		}
	}

	// 构建用户×日期工时明细网格（按月显示完整日历）
	gridStart, gridEnd := s.parseGridMonth(req.GridMonth)
	totalDays := int(gridEnd.Sub(gridStart).Hours()/24) + 1
	allDates := make([]string, 0, totalDays)
	for d := gridStart; !d.After(gridEnd); d = d.AddDate(0, 0, 1) {
		allDates = append(allDates, d.Format("2006-01-02"))
	}
	resp.GridDates = allDates
	resp.GridTotals = make(map[string]int64)

	dailyUserStats, err := s.reportRepo.GetWorklogDailyUserStats(ctx, projectID, gridStart, gridEnd)
	if err != nil {
		logger.Warn("failed to get worklog daily user stats", zap.Error(err))
	} else {
		userMap := make(map[uint64]*dto.WorklogGridRow)

		for _, row := range dailyUserStats {
			gr, ok := userMap[row.UserID]
			if !ok {
				gr = &dto.WorklogGridRow{
					UserID:       row.UserID,
					DisplayName:  row.DisplayName,
					Daily:        make(map[string]int64),
					DailyDetails: make(map[string][]dto.WorklogDetail),
				}
				userMap[row.UserID] = gr
			}
			gr.Daily[row.Date] += row.TotalTimeSec
			gr.TotalSec += row.TotalTimeSec
			resp.GridTotals[row.Date] += row.TotalTimeSec
			gr.DailyDetails[row.Date] = append(gr.DailyDetails[row.Date], dto.WorklogDetail{
				IssueKey: row.IssueKey,
				Title:    row.IssueTitle,
				TimeSec:  row.TotalTimeSec,
			})
		}

		// 用户行按 DisplayName 排序
		grid := make([]dto.WorklogGridRow, 0, len(userMap))
		for _, gr := range userMap {
			grid = append(grid, *gr)
		}
		sort.Slice(grid, func(i, j int) bool {
			return grid[i].DisplayName < grid[j].DisplayName
		})
		resp.Grid = grid
	}

	return resp, nil
}

// mapToDistribution 将 map 转换为分布数据
func (s *reportService) mapToDistribution(data map[string]int64) []dto.DistributionItem {
	var total int64
	for _, count := range data {
		total += count
	}

	result := make([]dto.DistributionItem, 0, len(data))
	for name, count := range data {
		ratio := float64(0)
		if total > 0 {
			ratio = float64(count) / float64(total) * 100
		}
		result = append(result, dto.DistributionItem{
			Name:  name,
			Value: count,
			Ratio: ratio,
		})
	}

	return result
}

// ============ 交付报表（周报 / 月报）============

// resolvePeriod 把「周期类型 + 周期内任意一天」换算成起止时刻，以及上一周期的起止。
//
// 周按 ISO 周算（周一到周日），月按自然月。返回的是左闭右闭的时刻区间，
// 右端取当天 23:59:59 —— 报表按自然日汇总，不该把周日当天的交付漏掉。
func resolvePeriod(period, dateStr string) (start, end, prevStart, prevEnd time.Time, err error) {
	base := time.Now()
	if dateStr != "" {
		base, err = time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("日期格式错误，应为 YYYY-MM-DD: %w", err)
		}
	}

	switch period {
	case "week":
		// Go 的 Weekday 周日是 0，这里要周一开头
		offset := (int(base.Weekday()) + 6) % 7
		start = time.Date(base.Year(), base.Month(), base.Day()-offset, 0, 0, 0, 0, base.Location())
		end = start.AddDate(0, 0, 7).Add(-time.Second)
		prevStart = start.AddDate(0, 0, -7)
		prevEnd = start.Add(-time.Second)
	case "month":
		start = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location())
		end = start.AddDate(0, 1, 0).Add(-time.Second)
		prevStart = start.AddDate(0, -1, 0)
		prevEnd = start.Add(-time.Second)
	default:
		return time.Time{}, time.Time{}, time.Time{}, time.Time{}, fmt.Errorf("不支持的统计周期: %s", period)
	}
	return start, end, prevStart, prevEnd, nil
}

// onTimeRate 准时率。分母只算「有承诺交付日」的单 —— 没排期的既不算准时也不算延期。
func onTimeRate(onTime, late int64) float64 {
	judged := onTime + late
	if judged == 0 {
		return 0
	}
	return float64(onTime) / float64(judged) * 100
}

// deliveryDimension 把按维度切分的交付统计转成响应结构。
// Label 留空交给前端按语言渲染 —— 优先级是 P0..P3 这种稳定 code，
// 类型名则是项目里配出来的，本身就是展示名。
func (s *reportService) deliveryDimension(ctx context.Context, dim string, projectID *uint64, start, end time.Time) []dto.DeliveryDimensionStat {
	var (
		rows []repository.DeliveryDimensionRow
		err  error
	)
	switch dim {
	case "priority":
		rows, err = s.reportRepo.GetDeliveryByPriority(ctx, projectID, start, end)
	case "type":
		rows, err = s.reportRepo.GetDeliveryByType(ctx, projectID, start, end)
	}
	if err != nil {
		logger.Warn("failed to aggregate delivery dimension",
			zap.String("dimension", dim), zap.Error(err))
		return nil
	}

	out := make([]dto.DeliveryDimensionStat, len(rows))
	for i, row := range rows {
		out[i] = dto.DeliveryDimensionStat{
			Key:        row.Key,
			Label:      row.Key,
			Delivered:  row.Delivered,
			OnTime:     row.OnTime,
			Late:       row.Late,
			OnTimeRate: onTimeRate(row.OnTime, row.Late),
		}
	}
	// 优先级按 P0..P3 排，不按交付量 —— 严重程度的顺序本身是信息
	if dim == "priority" {
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	}
	return out
}

// GetDeliveryReport 生成交付报表（周报 / 月报）
func (s *reportService) GetDeliveryReport(ctx context.Context, req *dto.DeliveryReportRequest) (*dto.DeliveryReportResponse, error) {
	start, end, prevStart, prevEnd, err := resolvePeriod(req.Period, req.Date)
	if err != nil {
		return nil, err
	}

	var projectID *uint64
	if req.ProjectKey != "" {
		project, projectErr := s.projectRepo.GetByKey(ctx, req.ProjectKey)
		if projectErr != nil {
			return nil, projectErr
		}
		projectID = &project.ID
	}

	agg, err := s.reportRepo.GetDeliveryAggregate(ctx, projectID, start, end)
	if err != nil {
		return nil, fmt.Errorf("统计交付情况失败: %w", err)
	}

	resp := &dto.DeliveryReportResponse{
		Summary: dto.DeliverySummary{
			PeriodStart:     start.Format("2006-01-02"),
			PeriodEnd:       end.Format("2006-01-02"),
			Delivered:       agg.Delivered,
			Terminated:      agg.Terminated,
			Created:         agg.Created,
			OnTime:          agg.OnTime,
			Late:            agg.Late,
			NoCommitment:    agg.NoCommitment,
			OnTimeRate:      onTimeRate(agg.OnTime, agg.Late),
			AvgVarianceDays: agg.AvgVarianceDays,
			AvgDeliveryDays: agg.AvgDeliveryDays,
			AlertIssues:     agg.AlertIssues,
		},
	}

	// 上期对比：只要准时率和交付量，失败不致命
	if prev, prevErr := s.reportRepo.GetDeliveryAggregate(ctx, projectID, prevStart, prevEnd); prevErr == nil {
		resp.PrevOnTimeRate = onTimeRate(prev.OnTime, prev.Late)
		resp.PrevDelivered = prev.Delivered
	} else {
		logger.Warn("failed to aggregate previous period", zap.Error(prevErr))
	}

	// 交付清单：周报正文直接抄这张表。上限给到 500，一个周期交付超过这个数
	// 的团队，靠清单写周报本来也不现实，该看的是上面的汇总。
	if issues, issueErr := s.reportRepo.GetDeliveredIssues(ctx, projectID, start, end, 500); issueErr == nil {
		resp.DeliveredIssues = make([]dto.DeliveryIssueItem, len(issues))
		for i, it := range issues {
			item := dto.DeliveryIssueItem{
				IssueKey:     it.IssueKey,
				Title:        it.Title,
				TypeName:     it.TypeName,
				Priority:     it.Priority,
				ProjectKey:   it.ProjectKey,
				AssigneeName: it.AssigneeName,
				ActualEnd:    it.ActualEnd.Format("2006-01-02"),
			}
			if it.PlannedEnd != nil && it.VarianceDays != nil {
				item.HasCommitment = true
				item.PlannedEnd = it.PlannedEnd.Format("2006-01-02")
				item.VarianceDays = *it.VarianceDays
			}
			resp.DeliveredIssues[i] = item
		}
	} else {
		logger.Warn("failed to list delivered issues", zap.Error(issueErr))
	}

	resp.ByPriority = s.deliveryDimension(ctx, "priority", projectID, start, end)
	resp.ByType = s.deliveryDimension(ctx, "type", projectID, start, end)

	if lateItems, lateErr := s.reportRepo.GetDeliveryLateIssues(ctx, projectID, start, end, 10); lateErr == nil {
		resp.TopLate = make([]dto.DeliveryVarianceItem, len(lateItems))
		for i, it := range lateItems {
			resp.TopLate[i] = dto.DeliveryVarianceItem{
				IssueKey:     it.IssueKey,
				Title:        it.Title,
				Priority:     it.Priority,
				ProjectKey:   it.ProjectKey,
				AssigneeName: it.AssigneeName,
				PlannedEnd:   it.PlannedEnd.Format("2006-01-02"),
				ActualEnd:    it.ActualEnd.Format("2006-01-02"),
				VarianceDays: it.VarianceDays,
			}
		}
	} else {
		logger.Warn("failed to list late issues", zap.Error(lateErr))
	}

	if members, memberErr := s.reportRepo.GetDeliveryByMember(ctx, projectID, start, end); memberErr == nil {
		worklogs, _ := s.reportRepo.GetWorklogSecondsByUser(ctx, projectID, start, end)
		resp.Members = make([]dto.DeliveryMemberStat, len(members))
		for i, m := range members {
			resp.Members[i] = dto.DeliveryMemberStat{
				UserID:      m.UserID,
				DisplayName: m.DisplayName,
				Delivered:   m.Delivered,
				OnTime:      m.OnTime,
				Late:        m.Late,
				OnTimeRate:  onTimeRate(m.OnTime, m.Late),
				WorkSeconds: worklogs[m.UserID],
			}
		}
	} else {
		logger.Warn("failed to aggregate by member", zap.Error(memberErr))
	}

	// 风险：已经超期的 + 未来七天内到期的
	if risks, riskErr := s.reportRepo.GetDeliveryRisks(ctx, projectID, 7, 20); riskErr == nil {
		resp.Risks = make([]dto.DeliveryRiskItem, len(risks))
		for i, rk := range risks {
			resp.Risks[i] = dto.DeliveryRiskItem{
				IssueKey:     rk.IssueKey,
				Title:        rk.Title,
				Priority:     rk.Priority,
				ProjectKey:   rk.ProjectKey,
				AssigneeName: rk.AssigneeName,
				Status:       rk.Status,
				PlannedEnd:   rk.PlannedEnd.Format("2006-01-02"),
				DaysLeft:     rk.DaysLeft,
			}
		}
	} else {
		logger.Warn("failed to list delivery risks", zap.Error(riskErr))
	}

	if projects, projErr := s.reportRepo.GetDeliveryByProject(ctx, projectID, start, end); projErr == nil {
		resp.Projects = make([]dto.DeliveryProjectStat, len(projects))
		for i, p := range projects {
			resp.Projects[i] = dto.DeliveryProjectStat{
				ProjectKey:  p.ProjectKey,
				ProjectName: p.ProjectName,
				Delivered:   p.Delivered,
				OnTime:      p.OnTime,
				Late:        p.Late,
				OnTimeRate:  onTimeRate(p.OnTime, p.Late),
			}
		}
	} else {
		logger.Warn("failed to aggregate by project", zap.Error(projErr))
	}

	return resp, nil
}
