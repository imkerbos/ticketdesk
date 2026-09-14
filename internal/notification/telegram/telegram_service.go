// Package telegram 提供 Telegram 通知服务
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	configService "github.com/kerbos/ticketdesk/internal/system-config/service"
	"github.com/kerbos/ticketdesk/pkg/i18n"
	"github.com/kerbos/ticketdesk/pkg/logger"
)

// TelegramService Telegram 通知服务接口
type TelegramService interface {
	// SendNotification 发送 Telegram 通知
	SendNotification(ctx context.Context, event string, data interface{}) error
	// SendTestMessage 发送测试消息
	SendTestMessage(ctx context.Context) error
	// IsEnabled 检查 Telegram 通知是否启用
	IsEnabled(ctx context.Context) bool
}

// telegramService Telegram 通知服务实现
type telegramService struct {
	configSvc  configService.ConfigService
	httpClient *http.Client
}

// telegramSendMessageRequest Telegram sendMessage API 请求体
type telegramSendMessageRequest struct {
	ChatID             string                  `json:"chat_id"`
	Text               string                  `json:"text"`
	ParseMode          string                  `json:"parse_mode"`
	LinkPreviewOptions *linkPreviewOptions     `json:"link_preview_options,omitempty"`
	ReplyMarkup        *telegramInlineKeyboard `json:"reply_markup,omitempty"`
}

// linkPreviewOptions 链接预览选项
type linkPreviewOptions struct {
	IsDisabled bool `json:"is_disabled"`
}

// telegramInlineKeyboard Telegram 内联键盘
type telegramInlineKeyboard struct {
	InlineKeyboard [][]telegramInlineButton `json:"inline_keyboard"`
}

// telegramInlineButton Telegram 内联按钮
type telegramInlineButton struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// NewTelegramService 创建 Telegram 通知服务实例
func NewTelegramService(configSvc configService.ConfigService) TelegramService {
	return &telegramService{
		configSvc: configSvc,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsEnabled 检查 Telegram 通知是否启用
func (s *telegramService) IsEnabled(ctx context.Context) bool {
	enabled, err := s.configSvc.GetConfigValue(ctx, configService.KeyTelegramEnabled)
	if err != nil {
		return false
	}
	return enabled == "true"
}

// SendNotification 发送 Telegram 通知
func (s *telegramService) SendNotification(ctx context.Context, event string, data interface{}) error {
	if !s.IsEnabled(ctx) {
		logger.Debug("telegram notification is disabled, skip sending")
		return nil
	}

	botToken, err := s.configSvc.GetConfigValue(ctx, configService.KeyTelegramBotToken)
	if err != nil || botToken == "" {
		return fmt.Errorf("Telegram Bot Token 未配置")
	}

	chatID, err := s.configSvc.GetConfigValue(ctx, configService.KeyTelegramChatID)
	if err != nil || chatID == "" {
		return fmt.Errorf("Telegram Chat ID 未配置")
	}

	// 每日日报独立模板
	if event == "issue.daily_digest" {
		siteURL, _ := s.configSvc.GetConfigValue(ctx, configService.KeyGeneralSiteURL)
		text, replyMarkup := buildTelegramDigest(toMap(data), siteURL)
		return s.doSend(ctx, botToken, chatID, text, replyMarkup)
	}

	// 构建消息
	text, replyMarkup := s.buildMessage(event, data)
	text = appendTelegramMentions(text, toMap(data))

	return s.doSend(ctx, botToken, chatID, text, replyMarkup)
}

// SendTestMessage 发送测试消息
func (s *telegramService) SendTestMessage(ctx context.Context) error {
	botToken, err := s.configSvc.GetConfigValue(ctx, configService.KeyTelegramBotToken)
	if err != nil || botToken == "" {
		return fmt.Errorf("Telegram Bot Token 未配置")
	}

	chatID, err := s.configSvc.GetConfigValue(ctx, configService.KeyTelegramChatID)
	if err != nil || chatID == "" {
		return fmt.Errorf("Telegram Chat ID 未配置")
	}

	text := "🔔 <b>" + i18n.B("notify.test_title") + "</b>\n\n" +
		"✅ " + i18n.B("notify.test_tg_ok") + "\n\n" +
		fmt.Sprintf("<i>%s</i>", html.EscapeString(time.Now().Format("2006-01-02 15:04:05")))

	return s.doSend(ctx, botToken, chatID, text, nil)
}

// buildMessage 根据事件类型构建 Telegram 消息（HTML 格式）
func (s *telegramService) buildMessage(event string, data interface{}) (string, *telegramInlineKeyboard) {
	dataMap := toMap(data)

	// 获取站点 URL
	ctx := context.Background()
	siteURL, _ := s.configSvc.GetConfigValue(ctx, configService.KeyGeneralSiteURL)
	if siteURL == "" {
		siteURL = "https://ticketdesk.example.com"
	}
	siteURL = strings.TrimRight(siteURL, "/")

	title := s.getEventTitle(event)
	var text string
	var replyMarkup *telegramInlineKeyboard

	// 告警来源的工单创建，使用独立标题
	if event == "issue.created" {
		if source, _ := dataMap["source"].(string); source == "alert" {
			title = "🚨 " + i18n.B("notify.title_alert_issue")
		}
	}

	switch {
	case event == "issue.created" && dataMap["source"] == "alert":
		text, replyMarkup = buildAlertIssueTelegram(title, dataMap, siteURL)

	// 工单流转：专用模板，显示新旧节点对比
	case event == "issue.transitioned":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		projectName, _ := dataMap["project_name"].(string)
		priority, _ := dataMap["priority"].(string)
		assignee, _ := dataMap["assignee"].(string)
		dueDate, _ := dataMap["due_date"].(string)
		statusName := tgGetStatusDisplayName(dataMap, "status", "status_name")
		oldStatusName := tgGetStatusDisplayName(dataMap, "old_status", "old_status_name")

		// 优先使用工作流节点名称，fallback 到状态名
		nodeName, _ := dataMap["node_name"].(string)
		oldNodeName, _ := dataMap["old_node_name"].(string)
		displayNew := nodeName
		displayOld := oldNodeName
		if displayNew == "" {
			displayNew = statusName
		}
		if displayOld == "" {
			displayOld = oldStatusName
		}

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if displayOld != "" && displayNew != "" {
			text += fmt.Sprintf("📊 %s → <b>%s</b>\n", html.EscapeString(displayOld), html.EscapeString(displayNew))
		} else if displayNew != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：<b>%s</b>\n", html.EscapeString(displayNew))
		}
		if projectName != "" {
			text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：%s\n", html.EscapeString(projectName))
		}
		if priority != "" {
			text += fmt.Sprintf("🔴 "+i18n.B("notify.label_priority")+"：%s\n", html.EscapeString(priority))
		}
		if assignee != "" {
			text += fmt.Sprintf("👤 "+i18n.B("notify.label_handler")+"：%s\n", html.EscapeString(assignee))
		}
		if dueDate != "" {
			text += fmt.Sprintf("⏰ "+i18n.B("notify.label_due")+"：<b>%s</b>\n", html.EscapeString(dueDate))
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	// 工单指派：专用模板，显示操作人和指派人
	case event == "issue.assigned":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		projectName, _ := dataMap["project_name"].(string)
		priority, _ := dataMap["priority"].(string)
		assignee, _ := dataMap["assignee"].(string)
		operator, _ := dataMap["operator"].(string)
		dueDate, _ := dataMap["due_date"].(string)
		statusName := tgGetStatusDisplayName(dataMap, "status", "status_name")

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if projectName != "" {
			text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：%s\n", html.EscapeString(projectName))
		}
		if operator != "" && assignee != "" {
			text += fmt.Sprintf("👤 "+i18n.B("notify.assigned_by")+" <b>%s</b>\n", html.EscapeString(operator), html.EscapeString(assignee))
		} else if assignee != "" {
			text += fmt.Sprintf("👤 "+i18n.B("notify.label_assign_to")+"：<b>%s</b>\n", html.EscapeString(assignee))
		}
		if priority != "" {
			text += fmt.Sprintf("🔴 "+i18n.B("notify.label_priority")+"：%s\n", html.EscapeString(priority))
		}
		if statusName != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：%s\n", html.EscapeString(statusName))
		}
		if dueDate != "" {
			text += fmt.Sprintf("⏰ "+i18n.B("notify.label_due")+"：<b>%s</b>\n", html.EscapeString(dueDate))
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	// 告警合并：专用模板，显示合并详情
	case event == "alert.merged":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		alertName, _ := dataMap["alert_name"].(string)
		instance, _ := dataMap["instance"].(string)
		var alertCount string
		switch v := dataMap["alert_count"].(type) {
		case float64:
			alertCount = fmt.Sprintf("%d", int(v))
		case int:
			alertCount = fmt.Sprintf("%d", v)
		case int64:
			alertCount = fmt.Sprintf("%d", v)
		}

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if alertName != "" {
			text += fmt.Sprintf("⚠️ "+i18n.B("notify.label_alert")+"：%s\n", html.EscapeString(alertName))
		}
		if instance != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_instance")+"：<b>%s</b>\n", html.EscapeString(instance))
		}
		if alertCount != "" {
			text += fmt.Sprintf("🔢 "+i18n.B("notify.label_count")+"：<b>%s</b>\n", alertCount)
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	// 通用工单事件
	case len(event) > 6 && event[:6] == "issue.":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		projectName, _ := dataMap["project_name"].(string)
		priority, _ := dataMap["priority"].(string)
		statusName := tgGetStatusDisplayName(dataMap, "status", "status_name")

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if projectName != "" {
			text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：%s\n", html.EscapeString(projectName))
		}
		if statusName != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：%s\n", html.EscapeString(statusName))
		}
		if priority != "" {
			text += fmt.Sprintf("🔴 "+i18n.B("notify.label_priority")+"：%s\n", html.EscapeString(priority))
		}
		if comment, ok := dataMap["comment"].(string); ok && comment != "" {
			if len(comment) > 200 {
				comment = comment[:200] + "..."
			}
			text += fmt.Sprintf("💬 "+i18n.B("notify.label_comment")+"：%s\n", html.EscapeString(comment))
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	case len(event) > 6 && event[:6] == "alert.":
		alertName, _ := dataMap["alert_name"].(string)
		severity, _ := dataMap["severity"].(string)
		alertStatus, _ := dataMap["status"].(string)
		issueKey, _ := dataMap["issue_key"].(string)

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b>\n\n", html.EscapeString(alertName))
		if severity != "" {
			text += fmt.Sprintf("⚠️ "+i18n.B("notify.label_severity")+"：%s\n", html.EscapeString(severity))
		}
		if alertStatus != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：%s\n", html.EscapeString(alertStatus))
		}
		if issueKey != "" {
			text += fmt.Sprintf("🎫 "+i18n.B("notify.label_issue")+"：%s\n", html.EscapeString(issueKey))
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	default:
		text = fmt.Sprintf("%s\n\n", title)
		jsonBytes, _ := json.MarshalIndent(dataMap, "", "  ")
		text += fmt.Sprintf("<pre>%s</pre>", html.EscapeString(string(jsonBytes)))
	}

	return text, replyMarkup
}

// buildAlertIssueTelegram 构建告警建单的 Telegram 消息
func buildAlertIssueTelegram(title string, data map[string]interface{}, siteURL string) (string, *telegramInlineKeyboard) {
	issueKey, _ := data["issue_key"].(string)
	alertName, _ := data["alert_name"].(string)
	projectName, _ := data["project_name"].(string)
	if projectName == "" {
		projectName, _ = data["project_key"].(string)
	}
	status, _ := data["status"].(string)
	priority, _ := data["priority"].(string)
	severity, _ := data["severity"].(string)
	assignee, _ := data["assignee"].(string)
	alertTime, _ := data["alert_time"].(string)

	priEmoji := "🟡"
	switch priority {
	case "P0":
		priEmoji = "🔴"
	case "P1":
		priEmoji = "🟠"
	case "P2":
		priEmoji = "🟡"
	case "P3":
		priEmoji = "🟢"
	}

	text := fmt.Sprintf("%s\n\n", title)
	text += fmt.Sprintf("<b>%s</b>\n", html.EscapeString(issueKey))
	text += fmt.Sprintf("%s\n\n", html.EscapeString(alertName))
	text += fmt.Sprintf("%s "+i18n.B("notify.label_priority")+"：<b>%s</b>　　📊 "+i18n.B("notify.label_status")+"：<b>%s</b>\n", priEmoji, html.EscapeString(priority), html.EscapeString(status))
	if projectName != "" || assignee != "" {
		text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：<b>%s</b>", html.EscapeString(projectName))
		if assignee != "" {
			text += fmt.Sprintf("　　👤 "+i18n.B("notify.label_assign")+"：<b>%s</b>", html.EscapeString(assignee))
		}
		text += "\n"
	}
	if severity != "" {
		text += fmt.Sprintf("⚠️ "+i18n.B("notify.label_severity")+"：<b>%s</b>\n", html.EscapeString(severity))
	}
	if alertTime != "" {
		text += fmt.Sprintf("⏰ "+i18n.B("notify.label_alert_time")+"：%s\n", html.EscapeString(alertTime))
	}

	var replyMarkup *telegramInlineKeyboard
	if issueKey != "" {
		replyMarkup = &telegramInlineKeyboard{
			InlineKeyboard: [][]telegramInlineButton{
				{
					{
						Text: "📋 " + i18n.B("notify.btn_view_issue"),
						URL:  fmt.Sprintf("%s/issues/%s", siteURL, issueKey),
					},
				},
			},
		}
	}

	return text, replyMarkup
}

// getEventTitle 获取事件标题
func (s *telegramService) getEventTitle(event string) string {
	switch event {
	case "issue.created":
		return "🎫 <b>" + i18n.B("notify.title_issue_created") + "</b>"
	case "issue.updated":
		return "✏️ <b>" + i18n.B("notify.title_issue_updated") + "</b>"
	case "issue.transitioned":
		return "🔄 <b>" + i18n.B("notify.title_issue_transitioned") + "</b>"
	case "issue.assigned":
		return "👤 <b>" + i18n.B("notify.title_issue_assigned") + "</b>"
	case "issue.commented":
		return "💬 <b>" + i18n.B("notify.title_issue_commented") + "</b>"
	case "alert.firing":
		return "🔥 <b>" + i18n.B("notify.title_alert_firing") + "</b>"
	case "alert.resolved":
		return "✅ <b>" + i18n.B("notify.title_alert_resolved") + "</b>"
	case "alert.merged":
		return "🔗 <b>" + i18n.B("notify.title_alert_merged") + "</b>"
	case "alert.acked":
		return "👁️ <b>" + i18n.B("notify.title_alert_acked") + "</b>"
	default:
		return "📢 <b>" + i18n.B("notify.title_system") + "</b>"
	}
}

// doSend 执行 Telegram API 调用
func (s *telegramService) doSend(ctx context.Context, botToken, chatID, text string, replyMarkup *telegramInlineKeyboard) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	reqBody := telegramSendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &linkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: replyMarkup,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化 Telegram 消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("创建 Telegram 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logger.Error("failed to send telegram notification", zap.Error(err))
		return fmt.Errorf("发送 Telegram 通知失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// 解析 Telegram API 响应
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && !result.OK {
		logger.Error("telegram API returned error",
			zap.String("description", result.Description),
			zap.Int("status_code", resp.StatusCode),
		)
		return fmt.Errorf("Telegram API 错误: %s", result.Description)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	logger.Info("telegram notification sent successfully", zap.String("chat_id", chatID))
	return nil
}

// ============ Direct Sender（项目级通知渠道使用，不依赖 ConfigService）============

// DirectTelegramSender 直接 Telegram 发送器（接受凭据参数，不依赖系统配置）
type DirectTelegramSender struct {
	botToken   string
	chatID     string
	siteURL    string
	httpClient *http.Client
}

// NewDirectTelegramSender 创建直接 Telegram 发送器
func NewDirectTelegramSender(botToken, chatID string, siteURL ...string) *DirectTelegramSender {
	url := ""
	if len(siteURL) > 0 {
		url = siteURL[0]
	}
	return &DirectTelegramSender{
		botToken: botToken,
		chatID:   chatID,
		siteURL:  url,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendNotification 发送 Telegram 通知
func (d *DirectTelegramSender) SendNotification(ctx context.Context, event string, data interface{}) error {
	text, replyMarkup := d.buildMessage(event, data)
	return d.doSend(ctx, text, replyMarkup)
}

// SendTestMessage 发送测试消息（使用与真实通知相同的模板）
func (d *DirectTelegramSender) SendTestMessage(ctx context.Context) error {
	testData := map[string]interface{}{
		"issue_key":    "TEST-1",
		"issue_title":  i18n.B("notify.test_issue_tg"),
		"project_name": i18n.B("notify.test_project"),
		"status":       i18n.B("notify.test_status"),
		"priority":     "P2",
	}
	return d.SendNotification(ctx, "issue.created", testData)
}

// buildMessage 根据事件类型构建 Telegram 消息（HTML 格式）
func (d *DirectTelegramSender) buildMessage(event string, data interface{}) (string, *telegramInlineKeyboard) {
	dataMap := toMap(data)
	siteURL := d.siteURL
	if siteURL == "" {
		siteURL = "https://ticketdesk.example.com"
	}
	siteURL = strings.TrimRight(siteURL, "/")

	title := directGetEventTitle(event)
	var text string
	var replyMarkup *telegramInlineKeyboard

	// 告警来源的工单创建，使用独立标题
	if event == "issue.created" {
		if source, _ := dataMap["source"].(string); source == "alert" {
			title = "🚨 <b>" + i18n.B("notify.title_alert_issue") + "</b>"
		}
	}

	switch {
	case event == "issue.created" && dataMap["source"] == "alert":
		text, replyMarkup = buildAlertIssueTelegram(title, dataMap, siteURL)

	// 工单流转：专用模板，显示新旧节点对比
	case event == "issue.transitioned":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		projectName, _ := dataMap["project_name"].(string)
		priority, _ := dataMap["priority"].(string)
		assignee, _ := dataMap["assignee"].(string)
		dueDate, _ := dataMap["due_date"].(string)
		statusName := tgGetStatusDisplayName(dataMap, "status", "status_name")
		oldStatusName := tgGetStatusDisplayName(dataMap, "old_status", "old_status_name")

		// 优先使用工作流节点名称，fallback 到状态名
		nodeName, _ := dataMap["node_name"].(string)
		oldNodeName, _ := dataMap["old_node_name"].(string)
		displayNew := nodeName
		displayOld := oldNodeName
		if displayNew == "" {
			displayNew = statusName
		}
		if displayOld == "" {
			displayOld = oldStatusName
		}

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if displayOld != "" && displayNew != "" {
			text += fmt.Sprintf("📊 %s → <b>%s</b>\n", html.EscapeString(displayOld), html.EscapeString(displayNew))
		} else if displayNew != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：<b>%s</b>\n", html.EscapeString(displayNew))
		}
		if projectName != "" {
			text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：%s\n", html.EscapeString(projectName))
		}
		if priority != "" {
			text += fmt.Sprintf("🔴 "+i18n.B("notify.label_priority")+"：%s\n", html.EscapeString(priority))
		}
		if assignee != "" {
			text += fmt.Sprintf("👤 "+i18n.B("notify.label_handler")+"：%s\n", html.EscapeString(assignee))
		}
		if dueDate != "" {
			text += fmt.Sprintf("⏰ "+i18n.B("notify.label_due")+"：<b>%s</b>\n", html.EscapeString(dueDate))
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	// 工单指派：专用模板，显示操作人和指派人
	case event == "issue.assigned":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		projectName, _ := dataMap["project_name"].(string)
		priority, _ := dataMap["priority"].(string)
		assignee, _ := dataMap["assignee"].(string)
		operator, _ := dataMap["operator"].(string)
		dueDate, _ := dataMap["due_date"].(string)
		statusName := tgGetStatusDisplayName(dataMap, "status", "status_name")

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if projectName != "" {
			text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：%s\n", html.EscapeString(projectName))
		}
		if operator != "" && assignee != "" {
			text += fmt.Sprintf("👤 "+i18n.B("notify.assigned_by")+" <b>%s</b>\n", html.EscapeString(operator), html.EscapeString(assignee))
		} else if assignee != "" {
			text += fmt.Sprintf("👤 "+i18n.B("notify.label_assign_to")+"：<b>%s</b>\n", html.EscapeString(assignee))
		}
		if priority != "" {
			text += fmt.Sprintf("🔴 "+i18n.B("notify.label_priority")+"：%s\n", html.EscapeString(priority))
		}
		if statusName != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：%s\n", html.EscapeString(statusName))
		}
		if dueDate != "" {
			text += fmt.Sprintf("⏰ "+i18n.B("notify.label_due")+"：<b>%s</b>\n", html.EscapeString(dueDate))
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	// 告警合并：专用模板，显示合并详情
	case event == "alert.merged":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		alertName, _ := dataMap["alert_name"].(string)
		instance, _ := dataMap["instance"].(string)
		var alertCount string
		switch v := dataMap["alert_count"].(type) {
		case float64:
			alertCount = fmt.Sprintf("%d", int(v))
		case int:
			alertCount = fmt.Sprintf("%d", v)
		case int64:
			alertCount = fmt.Sprintf("%d", v)
		}

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if alertName != "" {
			text += fmt.Sprintf("⚠️ "+i18n.B("notify.label_alert")+"：%s\n", html.EscapeString(alertName))
		}
		if instance != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_instance")+"：<b>%s</b>\n", html.EscapeString(instance))
		}
		if alertCount != "" {
			text += fmt.Sprintf("🔢 "+i18n.B("notify.label_count")+"：<b>%s</b>\n", alertCount)
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	// 通用工单事件
	case len(event) > 6 && event[:6] == "issue.":
		issueKey, _ := dataMap["issue_key"].(string)
		issueTitle, _ := dataMap["issue_title"].(string)
		projectName, _ := dataMap["project_name"].(string)
		priority, _ := dataMap["priority"].(string)
		statusName := tgGetStatusDisplayName(dataMap, "status", "status_name")

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b> %s\n\n", html.EscapeString(issueKey), html.EscapeString(issueTitle))
		if projectName != "" {
			text += fmt.Sprintf("📁 "+i18n.B("notify.label_project")+"：%s\n", html.EscapeString(projectName))
		}
		if statusName != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：%s\n", html.EscapeString(statusName))
		}
		if priority != "" {
			text += fmt.Sprintf("🔴 "+i18n.B("notify.label_priority")+"：%s\n", html.EscapeString(priority))
		}
		if comment, ok := dataMap["comment"].(string); ok && comment != "" {
			if len(comment) > 200 {
				comment = comment[:200] + "..."
			}
			text += fmt.Sprintf("💬 "+i18n.B("notify.label_comment")+"：%s\n", html.EscapeString(comment))
		}
		if issueKey != "" {
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	case len(event) > 6 && event[:6] == "alert.":
		alertName, _ := dataMap["alert_name"].(string)
		severity, _ := dataMap["severity"].(string)
		alertStatus, _ := dataMap["status"].(string)
		issueKey, _ := dataMap["issue_key"].(string)

		text = fmt.Sprintf("%s\n\n", title)
		text += fmt.Sprintf("<b>%s</b>\n\n", html.EscapeString(alertName))
		if severity != "" {
			text += fmt.Sprintf("⚠️ "+i18n.B("notify.label_severity")+"：%s\n", html.EscapeString(severity))
		}
		if alertStatus != "" {
			text += fmt.Sprintf("📊 "+i18n.B("notify.label_status")+"：%s\n", html.EscapeString(alertStatus))
		}
		if issueKey != "" {
			text += fmt.Sprintf("🎫 "+i18n.B("notify.label_issue")+"：%s\n", html.EscapeString(issueKey))
			replyMarkup = buildTelegramIssueButton(siteURL, issueKey)
		}

	default:
		text = fmt.Sprintf("%s\n\n", title)
		jsonBytes, _ := json.MarshalIndent(dataMap, "", "  ")
		text += fmt.Sprintf("<pre>%s</pre>", html.EscapeString(string(jsonBytes)))
	}

	return text, replyMarkup
}

// doSend 执行 Telegram API 调用
func (d *DirectTelegramSender) doSend(ctx context.Context, text string, replyMarkup *telegramInlineKeyboard) error {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", d.botToken)

	reqBody := telegramSendMessageRequest{
		ChatID:    d.chatID,
		Text:      text,
		ParseMode: "HTML",
		LinkPreviewOptions: &linkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: replyMarkup,
	}

	payloadBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("序列化 Telegram 消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("创建 Telegram 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送 Telegram 通知失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(respBody, &result); err == nil && !result.OK {
		return fmt.Errorf("Telegram API 错误: %s", result.Description)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Telegram 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// directGetEventTitle 获取事件标题
func directGetEventTitle(event string) string {
	switch event {
	case "issue.created":
		return "🎫 <b>" + i18n.B("notify.title_issue_created") + "</b>"
	case "issue.updated":
		return "✏️ <b>" + i18n.B("notify.title_issue_updated") + "</b>"
	case "issue.transitioned":
		return "🔄 <b>" + i18n.B("notify.title_issue_transitioned") + "</b>"
	case "issue.assigned":
		return "👤 <b>" + i18n.B("notify.title_issue_assigned") + "</b>"
	case "issue.commented":
		return "💬 <b>" + i18n.B("notify.title_issue_commented") + "</b>"
	case "alert.firing":
		return "🔥 <b>" + i18n.B("notify.title_alert_firing") + "</b>"
	case "alert.resolved":
		return "✅ <b>" + i18n.B("notify.title_alert_resolved") + "</b>"
	case "alert.merged":
		return "🔗 <b>" + i18n.B("notify.title_alert_merged") + "</b>"
	case "alert.acked":
		return "👁️ <b>" + i18n.B("notify.title_alert_acked") + "</b>"
	default:
		return "📢 <b>" + i18n.B("notify.title_system") + "</b>"
	}
}

// tgStatusDisplayKeys 状态到语言包 key 的映射；文案在发送时按后台语言取
var tgStatusDisplayKeys = map[string]string{
	"open":           "status.open",
	"in_progress":    "status.in_progress",
	"resolved":       "status.resolved",
	"closed":         "status.closed",
	"reviewing":      "status.reviewing",
	"pending_review": "status.pending_review",
	"merged":         "status.merged",
}

// tgGetStatusDisplayName 获取状态的中文显示名，优先使用 nameKey，fallback 到 statusKey 的映射
func tgGetStatusDisplayName(data map[string]interface{}, statusKey, nameKey string) string {
	if name, _ := data[nameKey].(string); name != "" {
		return name
	}
	if status, _ := data[statusKey].(string); status != "" {
		if key, ok := tgStatusDisplayKeys[status]; ok {
			return i18n.B(key)
		}
		return status
	}
	return ""
}

// buildTelegramIssueButton 构建查看工单的内联按钮
func buildTelegramIssueButton(siteURL, issueKey string) *telegramInlineKeyboard {
	return &telegramInlineKeyboard{
		InlineKeyboard: [][]telegramInlineButton{
			{
				{
					Text: "📋 " + i18n.B("notify.btn_view_issue"),
					URL:  fmt.Sprintf("%s/issues/%s", siteURL, issueKey),
				},
			},
		},
	}
}

// buildTelegramDigest 构造每日日报 Telegram 消息（HTML 格式 + 按 assignee 分组）
func buildTelegramDigest(data map[string]interface{}, siteURL string) (string, *telegramInlineKeyboard) {
	if siteURL == "" {
		siteURL = "https://ticketdesk.example.com"
	}
	siteURL = strings.TrimRight(siteURL, "/")

	projectKey, _ := data["project_key"].(string)
	projectName, _ := data["project_name"].(string)
	total := tgToInt(data["total"])

	var sb strings.Builder
	sb.WriteString("📋 <b>" + i18n.B("notify.digest_title") + "</b>\n")
	fmt.Fprintf(&sb, "<b>[%s] %s</b> "+i18n.B("notify.digest_open_count")+"\n",
		html.EscapeString(projectKey), html.EscapeString(projectName), total)

	groups := tgExtractGroupList(data)
	for _, g := range groups {
		sb.WriteString("\n")
		assigneeName, _ := g["assignee_name"].(string)
		// 分组标题：纯文本指派人名称（每行末尾会单独 @ 对应人）
		fmt.Fprintf(&sb, "<b>%s</b>\n", html.EscapeString(assigneeName))

		items := tgExtractItemList(g["items"])
		for _, it := range items {
			key, _ := it["issue_key"].(string)
			title, _ := it["title"].(string)
			typ, _ := it["issue_type"].(string)
			priority, _ := it["priority"].(string)
			status, _ := it["status"].(string)
			nodeName, _ := it["node_name"].(string)
			display := status
			if nodeName != "" {
				display = nodeName
			}
			meta := tgJoinNonEmpty([]string{typ, priority, display}, " | ")
			line := fmt.Sprintf("• <a href=\"%s/issues/%s\">%s</a> [%s] %s",
				siteURL, key, html.EscapeString(key), html.EscapeString(meta), html.EscapeString(title))
			if m, ok := it["mention"].(map[string]interface{}); ok {
				if at := telegramMentionFromMap(m); at != "" {
					line += " " + at
				}
			}
			sb.WriteString(line + "\n")
		}
	}

	replyMarkup := &telegramInlineKeyboard{
		InlineKeyboard: [][]telegramInlineButton{
			{
				{Text: "📊 " + i18n.B("notify.btn_view_project"), URL: fmt.Sprintf("%s/projects/%s", siteURL, projectKey)},
			},
		},
	}
	return sb.String(), replyMarkup
}

// telegramMentionFromMap 把单个 mention map 渲染为 @ 段
// 有 telegram_user_id 用 tg://user?id=xxx 深链；否则纯文本 @display_name
func telegramMentionFromMap(m map[string]interface{}) string {
	name, _ := m["display_name"].(string)
	tgID, _ := m["telegram_user_id"].(string)
	escName := html.EscapeString(name)
	if tgID != "" {
		return fmt.Sprintf("<a href=\"tg://user?id=%s\">@%s</a>", html.EscapeString(tgID), escName)
	}
	if name != "" {
		return "@" + escName
	}
	return ""
}

// tgExtractGroupList 兼容 []map[string]any / []any
func tgExtractGroupList(data map[string]interface{}) []map[string]interface{} {
	raw, ok := data["groups"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []map[string]interface{}:
		return v
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// tgExtractItemList 兼容 []map[string]any / []any
func tgExtractItemList(raw interface{}) []map[string]interface{} {
	switch v := raw.(type) {
	case []map[string]interface{}:
		return v
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// tgToInt 把 any 数字字段转 int
func tgToInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// tgJoinNonEmpty 拼接非空字符串
func tgJoinNonEmpty(parts []string, sep string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// appendTelegramMentions 在消息尾部追加 @ 提及行
// 期望 data["mentions"] 为 []any 或 []map[string]any，每项含 display_name / telegram_user_id
// 缺 telegram_user_id 时退化为纯文本 @display_name（HTML 转义）
func appendTelegramMentions(text string, data map[string]interface{}) string {
	mentions := extractMentionList(data)
	if len(mentions) == 0 {
		return text
	}

	var parts []string
	for _, m := range mentions {
		name, _ := m["display_name"].(string)
		tgID, _ := m["telegram_user_id"].(string)
		escName := html.EscapeString(name)
		if tgID != "" {
			parts = append(parts, fmt.Sprintf("<a href=\"tg://user?id=%s\">@%s</a>", html.EscapeString(tgID), escName))
		} else if name != "" {
			parts = append(parts, "@"+escName)
		}
	}
	if len(parts) == 0 {
		return text
	}
	return text + "\n" + strings.Join(parts, " ")
}

// extractMentionList 兼容 []any 与 []map[string]any 两种形态
func extractMentionList(data map[string]interface{}) []map[string]interface{} {
	raw, ok := data["mentions"]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []map[string]interface{}:
		return v
	case []interface{}:
		out := make([]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// toMap 将 interface{} 转换为 map[string]interface{}
func toMap(data interface{}) map[string]interface{} {
	if m, ok := data.(map[string]interface{}); ok {
		return m
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return map[string]interface{}{}
	}
	var m map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}
