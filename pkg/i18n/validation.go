// Package i18n: 表单校验错误的本地化
package i18n

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// 校验规则对应的提示模板。
//
// 之前 64 个 handler 统一写成 `"请求参数错误: " + err.Error()`，
// 直接把 validator 的原始英文抛给用户：
//
//	请求参数错误: Key: 'LoginRequest.Password' Error:Field validation for 'Password' failed on the 'min' tag
//
// 中文用户读不懂，英文用户看到的是半句中文加一串内部字段路径。
// 这里按规则翻成人话，并且只暴露字段名、不暴露结构体路径。
var validationTemplates = map[string]map[Lang]string{
	"required": {ZhCN: "%s 不能为空", EnUS: "%s is required"},
	"email":    {ZhCN: "%s 不是有效的邮箱地址", EnUS: "%s must be a valid email address"},
	"url":      {ZhCN: "%s 不是有效的链接", EnUS: "%s must be a valid URL"},
	"min":      {ZhCN: "%s 长度不能少于 %s", EnUS: "%s must be at least %s"},
	"max":      {ZhCN: "%s 长度不能超过 %s", EnUS: "%s must be at most %s"},
	"len":      {ZhCN: "%s 长度必须为 %s", EnUS: "%s must be exactly %s characters"},
	"oneof":    {ZhCN: "%s 只能是以下之一：%s", EnUS: "%s must be one of: %s"},
	"gte":      {ZhCN: "%s 不能小于 %s", EnUS: "%s must be %s or greater"},
	"lte":      {ZhCN: "%s 不能大于 %s", EnUS: "%s must be %s or less"},
	"gt":       {ZhCN: "%s 必须大于 %s", EnUS: "%s must be greater than %s"},
	"lt":       {ZhCN: "%s 必须小于 %s", EnUS: "%s must be less than %s"},
	"numeric":  {ZhCN: "%s 必须是数字", EnUS: "%s must be a number"},
	"alphanum": {ZhCN: "%s 只能包含字母和数字", EnUS: "%s may contain only letters and numbers"},
}

// fieldNames 结构体字段 -> 面向用户的名称。
// 只收录会出现在校验错误里的字段；未收录的回落到字段名本身，
// 那比暴露 "LoginRequest.Password" 这种内部路径要好。
var fieldNames = map[string]map[Lang]string{
	"Username":     {ZhCN: "用户名", EnUS: "Username"},
	"Password":     {ZhCN: "密码", EnUS: "Password"},
	"NewPassword":  {ZhCN: "新密码", EnUS: "New password"},
	"OldPassword":  {ZhCN: "原密码", EnUS: "Current password"},
	"Email":        {ZhCN: "邮箱", EnUS: "Email"},
	"DisplayName":  {ZhCN: "显示名称", EnUS: "Display name"},
	"Title":        {ZhCN: "标题", EnUS: "Summary"},
	"Description":  {ZhCN: "描述", EnUS: "Description"},
	"ProjectKey":   {ZhCN: "项目标识", EnUS: "Project key"},
	"Name":         {ZhCN: "名称", EnUS: "Name"},
	"IssueTypeID":  {ZhCN: "工单类型", EnUS: "Issue type"},
	"Priority":     {ZhCN: "优先级", EnUS: "Priority"},
	"Status":       {ZhCN: "状态", EnUS: "Status"},
	"AssigneeID":   {ZhCN: "指派人", EnUS: "Assignee"},
	"LeadUserID":   {ZhCN: "项目负责人", EnUS: "Project lead"},
	"Code":         {ZhCN: "验证码", EnUS: "Code"},
	"MFAToken":     {ZhCN: "验证令牌", EnUS: "Verification token"},
	"RefreshToken": {ZhCN: "刷新令牌", EnUS: "Refresh token"},
	"URL":          {ZhCN: "链接", EnUS: "URL"},
	"Content":      {ZhCN: "内容", EnUS: "Content"},
	"Comment":      {ZhCN: "备注", EnUS: "Comment"},
	"RoleKey":      {ZhCN: "角色", EnUS: "Role"},
	"Permissions":  {ZhCN: "权限", EnUS: "Permissions"},
	"Locale":       {ZhCN: "界面语言", EnUS: "Language"},
	"Language":     {ZhCN: "平台语言", EnUS: "Language"},
	"SiteURL":      {ZhCN: "站点域名", EnUS: "Site URL"},
	"SystemName":   {ZhCN: "系统名称", EnUS: "System name"},
	"Token":        {ZhCN: "令牌", EnUS: "Token"},
}

// FieldName 返回字段面向用户的名称
func FieldName(lang Lang, field string) string {
	if m, ok := fieldNames[field]; ok {
		if s, ok := m[lang]; ok && s != "" {
			return s
		}
		if s, ok := m[DefaultLang]; ok {
			return s
		}
	}
	return field
}

// ValidationError 把 binding 错误翻成用户可读的一句话。
//
// 多个字段同时出错时只报第一条：表单上一次改一处，
// 一次性甩出五条错误反而让人不知道从哪下手。
// 非 validator 错误（如 JSON 语法错误）回落到通用提示，不泄露解析器内部信息。
func ValidationError(ctx context.Context, err error) string {
	lang := FromContext(ctx)
	if err == nil {
		return TLang(lang, "common.bad_request")
	}

	var verrs validator.ValidationErrors
	if !asValidationErrors(err, &verrs) || len(verrs) == 0 {
		// JSON 反序列化失败等：给通用提示即可，
		// 原始错误里可能含有结构体字段与类型信息，不该直接抛给调用方
		return TLang(lang, "common.bad_request")
	}

	fe := verrs[0]
	name := FieldName(lang, fe.Field())

	tpl, ok := validationTemplates[fe.Tag()]
	if !ok {
		return fmt.Sprintf("%s: %s", TLang(lang, "common.bad_request"), name)
	}
	format := tpl[lang]
	if format == "" {
		format = tpl[DefaultLang]
	}

	if strings.Count(format, "%s") >= 2 {
		return fmt.Sprintf(format, name, fe.Param())
	}
	return fmt.Sprintf(format, name)
}

// asValidationErrors 单独抽出便于测试替身
func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	if v, ok := err.(validator.ValidationErrors); ok {
		*target = v
		return true
	}
	return false
}

// newTestValidator 供测试构造校验器，避免测试文件直接依赖 validator 包
func newTestValidator() *validator.Validate { return validator.New() }
