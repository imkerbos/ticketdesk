// Package handler 提供项目每日日报触发的 HTTP 处理器
package handler

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/internal/api/response"
	"github.com/kerbos/ticketdesk/internal/core-project/service"
)

// DigestHandler 日报处理器
type DigestHandler struct {
	projectService service.ProjectService
	digestService  service.DigestService
}

// NewDigestHandler 创建日报处理器实例
func NewDigestHandler(projectService service.ProjectService, digestService service.DigestService) *DigestHandler {
	return &DigestHandler{
		projectService: projectService,
		digestService:  digestService,
	}
}

// HandleRunDailyDigest 立即触发一次项目日报
// @Summary 立即触发项目日报
// @Description 不依赖 cron，立即生成并推送一次未完结工单日报；空 issues 不发送
// @Tags ProjectDailyDigest
// @Produce json
// @Param key path string true "项目 Key"
// @Success 200 {object} response.Response
// @Failure 404 {object} response.ErrorResponse "项目不存在"
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/projects/{key}/daily-digest/run [post]
// @Security BearerAuth
func (h *DigestHandler) HandleRunDailyDigest(c *gin.Context) {
	key := c.Param("key")
	project, err := h.projectService.GetProject(c.Request.Context(), key)
	if err != nil {
		if errors.Is(err, service.ErrProjectNotFound) {
			response.NotFound(c, "workflow.project_not_found")
			return
		}
		response.InternalError(c, "project.get_failed")
		return
	}

	if err := h.digestService.RunForProject(c.Request.Context(), project.ID); err != nil {
		response.InternalError(c, response.T(c, "project.digest_failed")+err.Error())
		return
	}

	response.Success(c, gin.H{"message": response.T(c, "project.digest_triggered")})
}
