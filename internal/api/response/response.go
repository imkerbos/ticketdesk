// Package response 提供统一的 API 响应格式
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/ticketdesk/pkg/i18n"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResponse 分页响应别名 (供 Swagger 注释引用, 等价于 PageData)
type PageResponse = PageData

// PageData 分页数据结构
type PageData struct {
	Items    interface{} `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	HasMore  bool        `json:"has_more,omitempty"` // true 表示实际总数超过 Total，前端显示 "10,000+"
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// Success 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Created 创建成功响应
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// SuccessWithPage 分页成功响应
func SuccessWithPage(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: PageData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
		},
	})
}

// SuccessWithPageHasMore 分页成功响应（带封顶计数标记）
func SuccessWithPageHasMore(c *gin.Context, items interface{}, total int64, page, pageSize int, hasMore bool) {
	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: PageData{
			Items:    items,
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  hasMore,
		},
	})
}

// Error 错误响应
//
// message 命中语言包 key 时按请求语言翻译，否则原样输出。
// 这样 service 层把哨兵错误的文案换成 key 之后，
// handler 里 `response.NotFound(c, err.Error())` 这类写法自动就本地化了，
// 而拼接出来的动态消息（如 "字段 xxx 不合法"）仍旧原样返回。
func Error(c *gin.Context, httpCode int, code, message string) {
	c.JSON(httpCode, ErrorResponse{
		Code:    code,
		Message: localize(c, message),
	})
}

// localize 只翻译已登记的 key，避免误伤普通文案
func localize(c *gin.Context, message string) string {
	if i18n.Has(message) {
		return i18n.T(c.Request.Context(), message)
	}
	return message
}

// ErrorWithDetails 带详情的错误响应
func ErrorWithDetails(c *gin.Context, httpCode int, code, message string, details interface{}) {
	c.JSON(httpCode, ErrorResponse{
		Code:    code,
		Message: localize(c, message),
		Details: details,
	})
}

// BadRequest 请求参数错误
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "BAD_REQUEST", message)
}

// Unauthorized 未认证
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// Forbidden 无权限
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "FORBIDDEN", message)
}

// NotFound 资源不存在
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "NOT_FOUND", message)
}

// InternalError 服务器内部错误
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// ============ 本地化响应 ============
//
// 上面那批 BadRequest / NotFound / ... 接收的是已经成文的字符串，
// 调用方直接写中文。要出英文版就必须让文案在响应层才定型 ——
// 下面这组接收消息 key，按请求协商出的语言取文案。
//
// 迁移策略：新代码一律用 *T 版本；旧调用点逐步替换，
// 两套并存期间行为完全一致（中文环境下输出同样的中文）。

// T 取本地化文案，供 handler 组装复杂消息时使用
func T(c *gin.Context, key string) string {
	return i18n.T(c.Request.Context(), key)
}

// ErrorT 按消息 key 返回错误响应
func ErrorT(c *gin.Context, httpCode int, code, msgKey string) {
	Error(c, httpCode, code, i18n.T(c.Request.Context(), msgKey))
}

// BadRequestT 参数错误
func BadRequestT(c *gin.Context, msgKey string) {
	ErrorT(c, http.StatusBadRequest, "BAD_REQUEST", msgKey)
}

// UnauthorizedT 未认证
func UnauthorizedT(c *gin.Context, msgKey string) {
	ErrorT(c, http.StatusUnauthorized, "UNAUTHORIZED", msgKey)
}

// ForbiddenT 无权限
func ForbiddenT(c *gin.Context, msgKey string) {
	ErrorT(c, http.StatusForbidden, "FORBIDDEN", msgKey)
}

// NotFoundT 资源不存在
func NotFoundT(c *gin.Context, msgKey string) {
	ErrorT(c, http.StatusNotFound, "NOT_FOUND", msgKey)
}

// InternalErrorT 服务器内部错误
func InternalErrorT(c *gin.Context, msgKey string) {
	ErrorT(c, http.StatusInternalServerError, "INTERNAL_ERROR", msgKey)
}

// BadRequestValidation 把 binding 校验错误翻成用户可读的一句话
//
// 替代此前遍布 64 个 handler 的 `response.BadRequest(c, "请求参数错误: "+err.Error())`。
// 那种写法会把 validator 的原始英文（含结构体路径）直接抛给用户，
// 中文用户读不懂，英文用户看到的是半句中文加内部字段名。
func BadRequestValidation(c *gin.Context, err error) {
	Error(c, http.StatusBadRequest, "BAD_REQUEST", i18n.ValidationError(c.Request.Context(), err))
}
