// Package dto 定义初始化向导的数据传输对象
package dto

// StatusResponse 初始化状态
//
// 只回一个布尔：是否配置了 token、token 从哪来，都不该让未认证的调用方知道。
type StatusResponse struct {
	Initialized bool `json:"initialized"`
}

// SetupRequest 初始化请求
type SetupRequest struct {
	// Token 首次部署时下发的一次性令牌
	Token string `json:"token" binding:"required"`

	// 管理员账号
	Username    string `json:"username" binding:"required,min=3,max=50"`
	Password    string `json:"password" binding:"required,min=6,max=50"`
	DisplayName string `json:"display_name" binding:"required,max=100"`
	Email       string `json:"email" binding:"required,email,max=100"`

	// 站点配置
	SystemName string `json:"system_name" binding:"required,max=50"`
	SiteURL    string `json:"site_url" binding:"required,url,max=255"`
	Language   string `json:"language" binding:"required,oneof=zh-CN en-US"`
}
