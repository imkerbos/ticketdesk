// Package handler 提供报表模块的 HTTP 处理器
package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/reporting/dto"
	"github.com/kerbos/ticketdesk/internal/reporting/service"
)

// ReportHandler 报表处理器
type ReportHandler struct {
	reportService service.ReportService
}

// NewReportHandler 创建报表处理器实例
func NewReportHandler(reportService service.ReportService) *ReportHandler {
	return &ReportHandler{reportService: reportService}
}

// HandleGetDashboardStats 获取仪表盘统计
// @Summary 获取仪表盘统计
// @Description 获取仪表盘的综合统计数据
// @Tags Report
// @Produce json
// @Param project_key query string false "项目 Key"
// @Success 200 {object} response.Response{data=dto.DashboardStatsResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/reports/dashboard [get]
// @Security BearerAuth
func (h *ReportHandler) HandleGetDashboardStats(c *gin.Context) {
	var req dto.DashboardStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.reportService.GetDashboardStats(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "report.dashboard_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetIssueStats 获取工单统计
// @Summary 获取工单统计
// @Description 获取工单的详细统计数据
// @Tags Report
// @Produce json
// @Param project_key query string false "项目 Key"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Param group_by query string false "分组方式 (day/week/month)"
// @Success 200 {object} response.Response{data=dto.IssueStatsResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/reports/issues [get]
// @Security BearerAuth
func (h *ReportHandler) HandleGetIssueStats(c *gin.Context) {
	var req dto.IssueStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.reportService.GetIssueStats(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "report.issue_stats_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetSLAReport 获取 SLA 报表
// @Summary 获取 SLA 报表
// @Description 获取 SLA 统计报表
// @Tags Report
// @Produce json
// @Param project_key query string false "项目 Key"
// @Param start_date query string true "开始日期 (YYYY-MM-DD)"
// @Param end_date query string true "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} response.Response{data=dto.SLAReportResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/reports/sla [get]
// @Security BearerAuth
func (h *ReportHandler) HandleGetSLAReport(c *gin.Context) {
	var req dto.SLAReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.reportService.GetSLAReport(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "report.sla_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetAlertStats 获取告警统计
// @Summary 获取告警统计
// @Description 获取告警的详细统计数据
// @Tags Report
// @Produce json
// @Param project_key query string false "项目 Key"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Param group_by query string false "分组方式 (day/week/month)"
// @Success 200 {object} internal_reporting_dto.AlertStatsResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/reports/alerts [get]
// @Security BearerAuth
func (h *ReportHandler) HandleGetAlertStats(c *gin.Context) {
	var req dto.AlertStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.reportService.GetAlertStats(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "alert.stats_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetWorklogStats 获取工时统计
// @Summary 获取工时统计
// @Description 获取工时统计数据（按日/按人/按类型汇总）
// @Tags Report
// @Produce json
// @Param project_key query string false "项目 Key"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} response.Response{data=dto.WorklogStatsResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/reports/worklogs [get]
// @Security BearerAuth
func (h *ReportHandler) HandleGetWorklogStats(c *gin.Context) {
	var req dto.WorklogStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.reportService.GetWorklogStats(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "report.worklog_stats_failed")
		return
	}

	response.Success(c, result)
}

// HandleGetUserPerformance 获取用户绩效
// @Summary 获取用户绩效
// @Description 获取用户处理工单的绩效数据
// @Tags Report
// @Produce json
// @Param project_key query string false "项目 Key"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} response.Response{data=[]dto.UserPerformanceResponse}
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/reports/user-performance [get]
// @Security BearerAuth
func (h *ReportHandler) HandleGetUserPerformance(c *gin.Context) {
	var req dto.IssueStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequestValidation(c, err)
		return
	}

	result, err := h.reportService.GetUserPerformance(c.Request.Context(), &req)
	if err != nil {
		response.InternalError(c, "report.performance_failed")
		return
	}

	response.Success(c, result)
}
