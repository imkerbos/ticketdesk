// Package i18n 提供后端消息的多语言支持
//
// 存在的原因：前端可以把界面切成英文，但后端返回的错误信息、通知内容、
// 邮件模板此前全是硬编码中文，切换语言后这部分不会跟着变 ——
// 用户会在一个英文界面里收到中文报错。这个包负责闭上那个环。
//
// 设计取舍：
//   - 不引入 golang.org/x/text/message 那套 catalog，它需要代码生成、
//     对本项目这种规模是过度设计。这里用一张 map，足够且可读。
//   - 语言从请求的 Accept-Language 协商，存进 context 往下传；
//     后台任务（通知、日报）没有请求上下文，回落到默认语言。
//   - 缺失的 key 回落到中文原文而不是显示 key 本身 —— 少一条翻译
//     总比在界面上暴露 "issue.not_found" 要好。
package i18n

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
)

// Lang 支持的语言
type Lang string

const (
	// ZhCN 简体中文，产品基准语言
	ZhCN Lang = "zh-CN"
	// EnUS 英语
	EnUS Lang = "en-US"
)

// DefaultLang 未能协商出语言时使用（编译期基准，翻译缺失时回落到这里）
const DefaultLang = ZhCN

// backgroundLang 后台任务用的语言。
//
// 通知、邮件、日报跑在没有 HTTP 请求的地方，Accept-Language 无从谈起，
// 群消息更是一条发给整群、没有"收件人"可言，所以只能整站一个设置。
//
// 用原子指针而不是普通变量：管理员在系统设置里改语言是运行时写入，
// 而发通知的 goroutine 同时在读 —— 普通变量在这里就是一个 data race。
var backgroundLang atomic.Pointer[Lang]

// SetBackgroundLang 设定后台任务语言，无法识别的值保持原设置不变。
//
// 唯一来源是数据库里的 general.language：启动时由 StartLanguageSync 读一次，
// 管理员在系统设置里改时再次调用。
// 刻意不把无法识别的值当成"重置为默认" —— 库里被写脏一个字母
// 就把整站语言悄悄切回中文，比保持原样更难排查。
func SetBackgroundLang(s string) {
	switch l := Lang(strings.TrimSpace(s)); l {
	case ZhCN, EnUS:
		backgroundLang.Store(&l)
	}
}

// BackgroundLang 返回后台任务语言
func BackgroundLang() Lang {
	if l := backgroundLang.Load(); l != nil {
		return *l
	}
	return DefaultLang
}

// B 取后台任务文案，等价于 TLang(BackgroundLang(), key)
func B(key string) string { return TLang(BackgroundLang(), key) }

// ResolveLang 解析用户偏好语言，无法识别或为空时回落到后台语言。
//
// 站内通知与邮件用它：这两处发给的是具体的人，
// 语言该跟着收件人走，而不是跟着触发操作那个人的浏览器走。
func ResolveLang(pref string) Lang {
	switch Lang(strings.TrimSpace(pref)) {
	case ZhCN:
		return ZhCN
	case EnUS:
		return EnUS
	}
	return BackgroundLang()
}

// U 按用户偏好语言取文案
func U(pref, key string) string { return TLang(ResolveLang(pref), key) }

// Uf 按用户偏好语言取文案并格式化
func Uf(pref, key string, args ...any) string { return fmt.Sprintf(U(pref, key), args...) }

// Bf 带格式化参数的后台任务文案
func Bf(key string, args ...any) string { return fmt.Sprintf(B(key), args...) }

type ctxKey struct{}

// WithLang 把语言写入 context
func WithLang(ctx context.Context, l Lang) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext 取出语言；没有则返回默认语言
func FromContext(ctx context.Context) Lang {
	if ctx == nil {
		return DefaultLang
	}
	if l, ok := ctx.Value(ctxKey{}).(Lang); ok && l != "" {
		return l
	}
	return DefaultLang
}

// ParseAcceptLanguage 从 Accept-Language 头协商语言
//
// 只做前缀匹配，不实现完整的 RFC 4647 查找：
// 本项目只支持两种语言，权重排序的复杂度换不来实际收益。
func ParseAcceptLanguage(header string) Lang {
	if header == "" {
		return DefaultLang
	}
	// 形如 "zh-CN,zh;q=0.9,en;q=0.8"，按出现顺序取第一个能识别的
	for _, part := range strings.Split(header, ",") {
		tag := strings.ToLower(strings.TrimSpace(strings.SplitN(part, ";", 2)[0]))
		switch {
		case strings.HasPrefix(tag, "zh"):
			return ZhCN
		case strings.HasPrefix(tag, "en"):
			return EnUS
		}
	}
	return DefaultLang
}

// messages 消息表：key -> 语言 -> 文案
//
// key 用点分域名式，与前端语言包保持同一套命名，便于对照。
// 译法以 docs/i18n-glossary.md 为准。
// register 把某个模块的消息并入总表，重复 key 直接 panic ——
// 与其在运行时静默覆盖，不如启动就炸出来
func register(m map[string]map[Lang]string) {
	for k, v := range m {
		if _, dup := messages[k]; dup {
			panic("i18n: 重复的消息 key " + k)
		}
		messages[k] = v
	}
}

var messages = map[string]map[Lang]string{
	// ---------- 认证 ----------
	"auth.invalid_credentials": {ZhCN: "用户名或密码错误", EnUS: "Incorrect username or password"},
	"auth.user_disabled":       {ZhCN: "用户已被禁用", EnUS: "This account has been disabled"},
	"auth.user_not_found":      {ZhCN: "用户不存在", EnUS: "User not found"},
	"auth.username_exists":     {ZhCN: "用户名已存在", EnUS: "That username is already taken"},
	"auth.email_exists":        {ZhCN: "邮箱已存在", EnUS: "That email address is already registered"},
	"auth.invalid_old_password": {
		ZhCN: "原密码错误",
		EnUS: "Current password is incorrect",
	},
	"auth.sso_password_not_allowed": {
		ZhCN: "SSO 用户不支持修改密码，请通过 SSO 提供方管理密码",
		EnUS: "Password changes are managed by your identity provider",
	},
	"auth.account_uses_sso": {
		ZhCN: "该账号已绑定 SSO，请通过 SSO 登录",
		EnUS: "This account uses single sign-on. Sign in with SSO instead",
	},
	"auth.invalid_reset_token": {
		ZhCN: "重置密码令牌无效或已过期",
		EnUS: "This password reset link is invalid or has expired",
	},
	"auth.reset_token_expired": {
		ZhCN: "重置密码令牌已过期",
		EnUS: "This password reset link has expired",
	},
	"auth.invalid_mfa_token": {
		ZhCN: "MFA 挑战令牌无效或已过期，请重新登录",
		EnUS: "Your verification session expired. Sign in again",
	},
	"auth.token_revoked": {
		ZhCN: "登录状态已失效，请重新登录",
		EnUS: "Your session is no longer valid. Sign in again",
	},
	"auth.token_expired":      {ZhCN: "Token 已过期", EnUS: "Your session has expired"},
	"auth.token_malformed":    {ZhCN: "Token 格式错误", EnUS: "Malformed token"},
	"auth.token_invalid":      {ZhCN: "Token 无效", EnUS: "Invalid token"},
	"auth.token_wrong_type":   {ZhCN: "Token 类型不正确", EnUS: "Token cannot be used for this request"},
	"auth.missing_credential": {ZhCN: "缺少认证信息", EnUS: "Authentication required"},
	"auth.bad_auth_format":    {ZhCN: "认证格式错误", EnUS: "Malformed Authorization header"},
	"auth.api_token_invalid":  {ZhCN: "API token 无效或已过期", EnUS: "API token is invalid or expired"},
	"auth.account_disabled":   {ZhCN: "账号已被禁用", EnUS: "This account has been disabled"},
	"auth.state_check_failed": {ZhCN: "账号状态校验失败", EnUS: "Could not verify account status"},

	// ---------- MFA ----------
	"mfa.already_enabled":   {ZhCN: "MFA 已启用", EnUS: "Two-factor authentication is already enabled"},
	"mfa.not_enabled":       {ZhCN: "MFA 未启用", EnUS: "Two-factor authentication is not enabled"},
	"mfa.invalid_code":      {ZhCN: "验证码错误", EnUS: "That code is not correct"},
	"mfa.setup_not_started": {ZhCN: "MFA 设置未开始", EnUS: "Start two-factor setup first"},
	"mfa.code_reused": {
		ZhCN: "该验证码已被使用，请等待下一个验证码",
		EnUS: "That code was already used. Wait for the next one",
	},

	// ---------- 工单 ----------
	"issue.not_found":      {ZhCN: "工单不存在", EnUS: "Issue not found"},
	"issue.type_not_found": {ZhCN: "工单类型不存在", EnUS: "Issue type not found"},
	"issue.file_too_large": {ZhCN: "文件大小超过限制", EnUS: "File exceeds the size limit"},
	"issue.invalid_file_type": {
		ZhCN: "不支持的文件类型",
		EnUS: "That file type is not supported",
	},
	"issue.attachment_not_found": {ZhCN: "附件不存在", EnUS: "Attachment not found"},

	// ---------- 工单模块（详细）----------
	"issue.parent_not_found":         {ZhCN: "父工单不存在", EnUS: "Parent issue not found"},
	"issue.invalid_attachment_id":    {ZhCN: "无效的附件ID", EnUS: "Invalid attachment ID"},
	"issue.invalid_worklog_id":       {ZhCN: "无效的工作日志ID", EnUS: "Invalid worklog ID"},
	"issue.invalid_comment_id":       {ZhCN: "无效的评论ID", EnUS: "Invalid comment ID"},
	"issue.file_too_large_10mb":      {ZhCN: "文件大小超过限制（最大10MB）", EnUS: "File exceeds the 10 MB limit"},
	"issue.choose_file":              {ZhCN: "请选择要上传的文件", EnUS: "Choose a file to upload"},
	"issue.missing_form_data":        {ZhCN: "缺少表单字段 data", EnUS: "Missing form field: data"},
	"issue.multipart_failed":         {ZhCN: "解析 multipart 表单失败", EnUS: "Could not read the uploaded form"},
	"issue.load_failed":              {ZhCN: "获取工单失败", EnUS: "Could not load the issue"},
	"issue.list_failed":              {ZhCN: "获取工单列表失败", EnUS: "Could not load issues"},
	"issue.stats_failed":             {ZhCN: "获取工单统计失败", EnUS: "Could not load issue statistics"},
	"issue.project_stats_failed":     {ZhCN: "获取项目概述统计失败", EnUS: "Could not load project statistics"},
	"issue.my_todo_failed":           {ZhCN: "获取我的待办工单失败", EnUS: "Could not load your open issues"},
	"issue.my_created_failed":        {ZhCN: "获取我创建的工单失败", EnUS: "Could not load issues you reported"},
	"issue.subtasks_failed":          {ZhCN: "获取子任务失败", EnUS: "Could not load subtasks"},
	"issue.epic_issues_failed":       {ZhCN: "获取 Epic 关联工单失败", EnUS: "Could not load issues in this epic"},
	"issue.comments_failed":          {ZhCN: "获取评论列表失败", EnUS: "Could not load comments"},
	"issue.add_comment_failed":       {ZhCN: "添加评论失败", EnUS: "Could not add the comment"},
	"issue.delete_comment_failed":    {ZhCN: "删除评论失败", EnUS: "Could not delete the comment"},
	"issue.watchers_failed":          {ZhCN: "获取关注人列表失败", EnUS: "Could not load watchers"},
	"issue.already_watching":         {ZhCN: "已经关注该工单", EnUS: "You already watch this issue"},
	"issue.watch_failed":             {ZhCN: "添加关注失败", EnUS: "Could not add the watcher"},
	"issue.unwatch_failed":           {ZhCN: "取消关注失败", EnUS: "Could not remove the watcher"},
	"issue.worklogs_failed":          {ZhCN: "获取工作日志列表失败", EnUS: "Could not load worklogs"},
	"issue.add_worklog_failed":       {ZhCN: "添加工作日志失败", EnUS: "Could not add the worklog"},
	"issue.update_worklog_failed":    {ZhCN: "更新工作日志失败", EnUS: "Could not update the worklog"},
	"issue.delete_worklog_failed":    {ZhCN: "删除工作日志失败", EnUS: "Could not delete the worklog"},
	"issue.attachment_failed":        {ZhCN: "获取附件失败", EnUS: "Could not load the attachment"},
	"issue.attachments_failed":       {ZhCN: "获取附件列表失败", EnUS: "Could not load attachments"},
	"issue.upload_failed":            {ZhCN: "上传附件失败", EnUS: "Could not upload the attachment"},
	"issue.delete_attachment_failed": {ZhCN: "删除附件失败", EnUS: "Could not delete the attachment"},
	"issue.create_failed":            {ZhCN: "创建工单失败", EnUS: "Could not create the issue"},
	"issue.update_failed":            {ZhCN: "更新工单失败", EnUS: "Could not update the issue"},
	"issue.delete_failed":            {ZhCN: "删除工单失败", EnUS: "Could not delete the issue"},
	"issue.assign_failed":            {ZhCN: "指派工单失败", EnUS: "Could not assign the issue"},
	"issue.delete_attachment_denied": {ZhCN: "无权限删除此附件", EnUS: "You can only delete attachments you uploaded"},

	// ---------- 项目 ----------
	"project.not_found": {ZhCN: "项目不存在", EnUS: "Project not found"},
	"project.exists":    {ZhCN: "项目已存在", EnUS: "A project with that key already exists"},

	// ---------- 权限 ----------
	"perm.denied":          {ZhCN: "权限不足", EnUS: "You don't have permission to do that"},
	"perm.check_failed":    {ZhCN: "权限检查失败", EnUS: "Permission check failed"},
	"perm.no_user":         {ZhCN: "未获取到用户信息", EnUS: "Could not identify the current user"},
	"perm.unauthorized":    {ZhCN: "无权限", EnUS: "Not authorized"},
	"perm.missing_project": {ZhCN: "缺少项目标识", EnUS: "Project key is required"},

	// ---------- 通用 ----------
	"common.bad_request":    {ZhCN: "请求参数错误", EnUS: "Invalid request"},
	"common.internal_error": {ZhCN: "服务器内部错误", EnUS: "Something went wrong on our side"},
	"common.too_many_requests": {
		ZhCN: "请求过于频繁，请稍后再试",
		EnUS: "Too many requests. Try again shortly",
	},
	"common.payload_too_large": {
		ZhCN: "请求体过大或读取失败",
		EnUS: "Request body is too large or could not be read",
	},
	"common.signature_failed": {ZhCN: "签名校验失败", EnUS: "Signature verification failed"},
}

// T 按 context 中的语言取文案；key 不存在时原样返回 key（便于排查）
func T(ctx context.Context, key string) string {
	return TLang(FromContext(ctx), key)
}

// TLang 按指定语言取文案
func TLang(lang Lang, key string) string {
	entry, ok := messages[key]
	if !ok {
		return key
	}
	if s, ok := entry[lang]; ok && s != "" {
		return s
	}
	// 缺翻译时回落到基准语言，而不是暴露 key
	if s, ok := entry[DefaultLang]; ok {
		return s
	}
	return key
}

// Tf 带格式化参数的取文案
func Tf(ctx context.Context, key string, args ...any) string {
	return fmt.Sprintf(T(ctx, key), args...)
}

// Has 判断 key 是否已登记
//
// 给 response 层用：service 层的哨兵错误现在带的是消息 key 而不是成文的中文，
// handler 里那一大批 `response.NotFound(c, err.Error())` 不必逐个改写，
// 由响应层判断"这串是不是 key"来决定翻不翻。
// key 都是 ASCII 点分名，和任何一句人话都不会撞。
func Has(key string) bool {
	_, ok := messages[key]
	return ok
}

// Keys 返回全部消息 key，供一致性测试使用
func Keys() []string {
	out := make([]string, 0, len(messages))
	for k := range messages {
		out = append(out, k)
	}
	return out
}

// Langs 返回全部支持的语言
func Langs() []Lang { return []Lang{ZhCN, EnUS} }
