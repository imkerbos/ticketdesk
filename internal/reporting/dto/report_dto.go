// Package dto 定义报表模块的数据传输对象
package dto

import "time"

// ============ 请求 DTO ============

// DashboardStatsRequest 仪表盘统计请求
type DashboardStatsRequest struct {
	ProjectKey string `form:"project_key" binding:"omitempty"` // 项目 Key（可选）
}

// IssueStatsRequest 工单统计请求
type IssueStatsRequest struct {
	ProjectKey string `form:"project_key" binding:"omitempty"`
	StartDate  string `form:"start_date" binding:"omitempty"`                    // 开始日期 YYYY-MM-DD
	EndDate    string `form:"end_date" binding:"omitempty"`                      // 结束日期 YYYY-MM-DD
	GroupBy    string `form:"group_by" binding:"omitempty,oneof=day week month"` // 分组方式
}

// SLAReportRequest SLA 报表请求
type SLAReportRequest struct {
	ProjectKey string `form:"project_key" binding:"omitempty"`
	StartDate  string `form:"start_date" binding:"required"` // 开始日期 YYYY-MM-DD
	EndDate    string `form:"end_date" binding:"required"`   // 结束日期 YYYY-MM-DD
}

// AlertStatsRequest 告警统计请求
type AlertStatsRequest struct {
	ProjectKey string `form:"project_key" binding:"omitempty"`
	StartDate  string `form:"start_date" binding:"omitempty"`
	EndDate    string `form:"end_date" binding:"omitempty"`
	GroupBy    string `form:"group_by" binding:"omitempty,oneof=day week month"`
}

// ExportRequest 数据导出请求
type ExportRequest struct {
	Type       string `form:"type" binding:"required,oneof=issues alerts"` // 导出类型
	ProjectKey string `form:"project_key" binding:"omitempty"`
	StartDate  string `form:"start_date" binding:"omitempty"`
	EndDate    string `form:"end_date" binding:"omitempty"`
	Format     string `form:"format" binding:"omitempty,oneof=csv xlsx"` // 导出格式
}

// ============ 响应 DTO ============

// DashboardStatsResponse 仪表盘统计响应
type DashboardStatsResponse struct {
	// 工单统计
	IssueStats struct {
		TotalOpen       int64 `json:"total_open"`        // 待处理工单
		TotalInProgress int64 `json:"total_in_progress"` // 进行中工单
		TotalResolved   int64 `json:"total_resolved"`    // 已解决工单
		TotalClosed     int64 `json:"total_closed"`      // 已关闭工单
		TodayCreated    int64 `json:"today_created"`     // 今日创建
		WeekCreated     int64 `json:"week_created"`      // 本周创建
		WeekResolved    int64 `json:"week_resolved"`     // 本周解决
	} `json:"issue_stats"`

	// 告警统计
	AlertStats struct {
		TotalFiring   int64 `json:"total_firing"`   // 触发中告警
		TotalAcked    int64 `json:"total_acked"`    // 已确认告警
		TotalResolved int64 `json:"total_resolved"` // 已恢复告警
		TodayCreated  int64 `json:"today_created"`  // 今日告警
		WeekCreated   int64 `json:"week_created"`   // 本周告警
	} `json:"alert_stats"`

	// 项目统计
	ProjectStats struct {
		TotalProjects int64 `json:"total_projects"` // 总项目数
		TotalMembers  int64 `json:"total_members"`  // 总成员数
	} `json:"project_stats"`
}

// IssueStatsResponse 工单统计响应
type IssueStatsResponse struct {
	Summary struct {
		Total          int64   `json:"total"`            // 总数
		Open           int64   `json:"open"`             // 待处理
		InProgress     int64   `json:"in_progress"`      // 进行中
		Resolved       int64   `json:"resolved"`         // 已解决
		Closed         int64   `json:"closed"`           // 已关闭
		AvgResolveTime float64 `json:"avg_resolve_time"` // 平均解决时间（小时）
	} `json:"summary"`

	// 按时间分组的统计
	Timeline []TimelineItem `json:"timeline"`

	// 按优先级分布
	PriorityDistribution []DistributionItem `json:"priority_distribution"`

	// 按类型分布
	TypeDistribution []DistributionItem `json:"type_distribution"`

	// 按状态分布
	StatusDistribution []DistributionItem `json:"status_distribution"`

	// 按指派人分布
	AssigneeDistribution []DistributionItem `json:"assignee_distribution"`

	// 按 Epic 分布
	EpicDistribution []DistributionItem `json:"epic_distribution"`
}

// TimelineItem 时间线统计项
type TimelineItem struct {
	Date       string `json:"date"`        // 日期
	Created    int64  `json:"created"`     // 创建数量
	InProgress int64  `json:"in_progress"` // 进行中数量
	Resolved   int64  `json:"resolved"`    // 解决数量
	Closed     int64  `json:"closed"`      // 关闭数量
}

// DistributionItem 分布统计项
type DistributionItem struct {
	Name  string  `json:"name"`  // 名称
	Value int64   `json:"value"` // 数量
	Ratio float64 `json:"ratio"` // 占比
}

// SLAReportResponse SLA 报表响应
type SLAReportResponse struct {
	Summary struct {
		TotalIssues    int64   `json:"total_issues"`    // 总工单数
		ResolvedIssues int64   `json:"resolved_issues"` // 已解决工单数
		MTTA           float64 `json:"mtta"`            // 平均确认时间（分钟）
		MTTR           float64 `json:"mttr"`            // 平均解决时间（分钟）
		SLAMet         int64   `json:"sla_met"`         // SLA 达标数
		SLAViolated    int64   `json:"sla_violated"`    // SLA 违规数
		SLARate        float64 `json:"sla_rate"`        // SLA 达标率（%）
	} `json:"summary"`

	// 按优先级的 SLA 统计
	ByPriority []SLAPriorityStats `json:"by_priority"`

	// 按项目的 SLA 统计
	ByProject []SLAProjectStats `json:"by_project"`

	// SLA 违规工单列表
	Violations []SLAViolation `json:"violations"`
}

// SLAViolation SLA 违规工单
type SLAViolation struct {
	IssueKey     string  `json:"issue_key"`     // 工单号
	Title        string  `json:"title"`         // 标题
	Priority     string  `json:"priority"`      // 优先级
	AssigneeName string  `json:"assignee_name"` // 经办人
	SLATarget    int64   `json:"sla_target"`    // SLA 目标（分钟）
	ActualTime   float64 `json:"actual_time"`   // 实际耗时（分钟）
	OverdueBy    float64 `json:"overdue_by"`    // 超时时长（分钟）
}

// SLAPriorityStats 按优先级的 SLA 统计
type SLAPriorityStats struct {
	Priority  string  `json:"priority"`   // 优先级
	Total     int64   `json:"total"`      // 总数
	Resolved  int64   `json:"resolved"`   // 已解决
	MTTA      float64 `json:"mtta"`       // 平均确认时间
	MTTR      float64 `json:"mttr"`       // 平均解决时间
	SLATarget int64   `json:"sla_target"` // SLA 目标（分钟）
	SLAMet    int64   `json:"sla_met"`    // 达标数
	SLARate   float64 `json:"sla_rate"`   // 达标率
}

// SLAProjectStats 按项目的 SLA 统计
type SLAProjectStats struct {
	ProjectKey  string  `json:"project_key"`  // 项目 Key
	ProjectName string  `json:"project_name"` // 项目名称
	Total       int64   `json:"total"`        // 总数
	Resolved    int64   `json:"resolved"`     // 已解决
	MTTR        float64 `json:"mttr"`         // 平均解决时间
	SLARate     float64 `json:"sla_rate"`     // 达标率
}

// AlertStatsResponse 告警统计响应
type AlertStatsResponse struct {
	Summary struct {
		Total      int64   `json:"total"`        // 总数
		Firing     int64   `json:"firing"`       // 触发中
		Acked      int64   `json:"acked"`        // 已确认
		Resolved   int64   `json:"resolved"`     // 已恢复
		AvgAckTime float64 `json:"avg_ack_time"` // 平均确认时间（分钟）
	} `json:"summary"`

	// 按时间分组的统计
	Timeline []AlertTimelineItem `json:"timeline"`

	// 按严重程度分布
	SeverityDistribution []DistributionItem `json:"severity_distribution"`

	// 按告警名称 Top10
	TopAlerts []TopAlertItem `json:"top_alerts"`
}

// AlertTimelineItem 告警时间线统计项
type AlertTimelineItem struct {
	Date     string `json:"date"`     // 日期
	Firing   int64  `json:"firing"`   // 触发数量
	Resolved int64  `json:"resolved"` // 恢复数量
}

// TopAlertItem 告警排名项
type TopAlertItem struct {
	AlertName string `json:"alert_name"` // 告警名称
	Severity  string `json:"severity"`   // 严重程度
	Count     int64  `json:"count"`      // 数量
}

// UserPerformanceResponse 用户绩效响应
type UserPerformanceResponse struct {
	UserID         uint64  `json:"user_id"`
	Username       string  `json:"username"`
	DisplayName    string  `json:"display_name"`
	Assigned       int64   `json:"assigned"`         // 指派数
	Resolved       int64   `json:"resolved"`         // 解决数
	AvgResolveTime float64 `json:"avg_resolve_time"` // 平均解决时间
}

// WorklogStatsRequest 工时统计请求
type WorklogStatsRequest struct {
	ProjectKey string `form:"project_key" binding:"omitempty"`
	StartDate  string `form:"start_date" binding:"omitempty"`
	EndDate    string `form:"end_date" binding:"omitempty"`
	GridMonth  string `form:"grid_month" binding:"omitempty"` // 工时明细月份 YYYY-MM，默认当月
}

// WorklogStatsResponse 工时统计响应
type WorklogStatsResponse struct {
	Summary    WorklogSummary     `json:"summary"`
	DailyStats []DailyWorklogStat `json:"daily_stats"`
	UserStats  []UserWorklogStat  `json:"user_stats"`
	TypeStats  []WorklogTypeStat  `json:"type_stats"`
	Grid       []WorklogGridRow   `json:"grid"`        // 用户×日期工时明细
	GridDates  []string           `json:"grid_dates"`  // 有序日期列表
	GridTotals map[string]int64   `json:"grid_totals"` // 每日合计（秒）
}

// WorklogGridRow 工时网格行（一个用户的每日工时）
type WorklogGridRow struct {
	UserID       uint64                     `json:"user_id"`
	DisplayName  string                     `json:"display_name"`
	TotalSec     int64                      `json:"total_sec"`     // 该用户总工时（秒）
	Daily        map[string]int64           `json:"daily"`         // date -> 秒
	DailyDetails map[string][]WorklogDetail `json:"daily_details"` // date -> 工单明细
}

// WorklogDetail 单个工单的工时明细
type WorklogDetail struct {
	IssueKey string `json:"issue_key"`
	Title    string `json:"title"`
	TimeSec  int64  `json:"time_sec"`
}

// WorklogSummary 工时汇总
type WorklogSummary struct {
	TotalTimeSec    int64   `json:"total_time_sec"`
	TotalEntries    int64   `json:"total_entries"`
	ActiveUsers     int64   `json:"active_users"`
	AvgDailyTimeSec float64 `json:"avg_daily_time_sec"`
}

// DailyWorklogStat 每日工时统计
type DailyWorklogStat struct {
	Date         string `json:"date"`
	TotalTimeSec int64  `json:"total_time_sec"`
	EntryCount   int64  `json:"entry_count"`
}

// UserWorklogStat 用户工时统计
type UserWorklogStat struct {
	UserID       uint64 `json:"user_id"`
	DisplayName  string `json:"display_name"`
	TotalTimeSec int64  `json:"total_time_sec"`
	EntryCount   int64  `json:"entry_count"`
}

// WorklogTypeStat 工时类型统计
type WorklogTypeStat struct {
	WorkType     string `json:"work_type"`
	TotalTimeSec int64  `json:"total_time_sec"`
	EntryCount   int64  `json:"entry_count"`
}

// DateRange 日期范围
type DateRange struct {
	StartDate time.Time
	EndDate   time.Time
}

// ============ 交付报表（周报 / 月报）============

// DeliveryReportRequest 交付报表请求
type DeliveryReportRequest struct {
	// Period 统计周期：week / month
	Period string `form:"period" binding:"required,oneof=week month"`
	// Date 周期内的任意一天，据此推出周期起止；留空取今天
	Date       string `form:"date"`
	ProjectKey string `form:"project_key"`
}

// DeliverySummary 交付总览
type DeliverySummary struct {
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`

	// Delivered 本期交付数（状态为已完成；「已终止」不算交付，单独统计）
	Delivered int64 `json:"delivered"`
	// Terminated 本期终止数
	Terminated int64 `json:"terminated"`
	// Created 本期新开数
	Created int64 `json:"created"`

	// OnTime / Late 只统计「既有承诺交付日、又有实际完成时间」的单
	OnTime int64 `json:"on_time"`
	Late   int64 `json:"late"`
	// NoCommitment 交付了但没有承诺交付日的单数 —— 它们不进准时率，
	// 必须显式报出来，否则一个漂亮的准时率会骗人
	NoCommitment int64 `json:"no_commitment"`
	// OnTimeRate 准时率（%），分母是 OnTime + Late
	OnTimeRate float64 `json:"on_time_rate"`

	// AvgVarianceDays 平均偏差天数，正数表示平均提前，负数表示平均延期
	AvgVarianceDays float64 `json:"avg_variance_days"`
	// AvgDeliveryDays 平均交付时长（实际开始 → 实际完成，天）
	AvgDeliveryDays float64 `json:"avg_delivery_days"`

	// AlertIssues 本期告警自动开的单数，不进准时率
	AlertIssues int64 `json:"alert_issues"`
}

// DeliveryVarianceItem 延期最多的工单
type DeliveryVarianceItem struct {
	IssueKey     string `json:"issue_key"`
	Title        string `json:"title"`
	Priority     string `json:"priority"`
	ProjectKey   string `json:"project_key"`
	AssigneeName string `json:"assignee_name"`
	PlannedEnd   string `json:"planned_end"`
	ActualEnd    string `json:"actual_end"`
	// VarianceDays 正数提前、负数延期
	VarianceDays int64 `json:"variance_days"`
}

// DeliveryMemberStat 人员交付情况
type DeliveryMemberStat struct {
	UserID      uint64  `json:"user_id"`
	DisplayName string  `json:"display_name"`
	Delivered   int64   `json:"delivered"`
	OnTime      int64   `json:"on_time"`
	Late        int64   `json:"late"`
	OnTimeRate  float64 `json:"on_time_rate"`
	// WorkSeconds 本期填报的工时
	WorkSeconds int64 `json:"work_seconds"`
}

// DeliveryRiskItem 风险工单：已延期未交付 / 即将到期
type DeliveryRiskItem struct {
	IssueKey     string `json:"issue_key"`
	Title        string `json:"title"`
	Priority     string `json:"priority"`
	ProjectKey   string `json:"project_key"`
	AssigneeName string `json:"assignee_name"`
	Status       string `json:"status"`
	PlannedEnd   string `json:"planned_end"`
	// DaysLeft 距承诺交付还有几天，负数表示已经超了
	DaysLeft int64 `json:"days_left"`
}

// DeliveryProjectStat 项目横向对比
type DeliveryProjectStat struct {
	ProjectKey  string  `json:"project_key"`
	ProjectName string  `json:"project_name"`
	Delivered   int64   `json:"delivered"`
	OnTime      int64   `json:"on_time"`
	Late        int64   `json:"late"`
	OnTimeRate  float64 `json:"on_time_rate"`
}

// DeliveryReportResponse 交付报表响应
type DeliveryReportResponse struct {
	Summary DeliverySummary `json:"summary"`
	// PrevOnTimeRate 上一周期的准时率，用于给出环比
	PrevOnTimeRate float64                `json:"prev_on_time_rate"`
	PrevDelivered  int64                  `json:"prev_delivered"`
	TopLate        []DeliveryVarianceItem `json:"top_late"`
	Members        []DeliveryMemberStat   `json:"members"`
	Risks          []DeliveryRiskItem     `json:"risks"`
	Projects       []DeliveryProjectStat  `json:"projects"`
}
