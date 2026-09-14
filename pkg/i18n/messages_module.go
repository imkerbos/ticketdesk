// Package i18n: 各业务模块的消息目录
//
// 与 i18n.go 里的 common.* 分开，是因为那批是跨模块共用的通用文案，
// 这里按模块归拢，改一个模块的措辞不必翻整张大表。
package i18n

func init() {
	register(userMessages)
	register(projectMessages)
	register(workflowMessages)
	register(issueExtraMessages)
	register(alertMessages)
	register(fieldMessages)
	register(requirementMessages)
	register(systemMessages)
	register(miscMessages)
	register(notifyMessages)
	register(statusMessages)
	register(inboxMessages)
	register(mailMessages)
	register(setupMessages)
}

// ---------- 用户 / 认证 ----------
var userMessages = map[string]map[Lang]string{
	"user.not_found":              {ZhCN: "用户不存在", EnUS: "User not found"},
	"user.username_exists":        {ZhCN: "用户名已存在", EnUS: "That username is taken"},
	"user.email_exists":           {ZhCN: "邮箱已存在", EnUS: "That email address is already registered"},
	"user.bad_credentials":        {ZhCN: "用户名或密码错误", EnUS: "Wrong username or password"},
	"user.disabled":               {ZhCN: "用户已被禁用", EnUS: "This account is disabled"},
	"user.login_not_allowed":      {ZhCN: "用户不允许登录，请联系管理员", EnUS: "This account cannot sign in — contact an administrator"},
	"user.already_enabled":        {ZhCN: "用户已启用", EnUS: "The account is already enabled"},
	"user.already_disabled":       {ZhCN: "用户已禁用", EnUS: "The account is already disabled"},
	"user.cannot_delete_self":     {ZhCN: "不能删除自己", EnUS: "You cannot delete your own account"},
	"user.cannot_delete_admin":    {ZhCN: "不能删除系统管理员", EnUS: "The system administrator cannot be deleted"},
	"user.cannot_disable_self":    {ZhCN: "不能禁用自己", EnUS: "You cannot disable your own account"},
	"user.cannot_disable_admin":   {ZhCN: "不能禁用系统管理员", EnUS: "The system administrator cannot be disabled"},
	"user.old_password_wrong":     {ZhCN: "原密码错误", EnUS: "The current password is wrong"},
	"user.password_changed":       {ZhCN: "密码修改成功", EnUS: "Password changed"},
	"user.password_reset":         {ZhCN: "密码重置成功", EnUS: "Password reset"},
	"user.deleted":                {ZhCN: "用户删除成功", EnUS: "User deleted"},
	"user.invalid_id":             {ZhCN: "无效的用户 ID", EnUS: "Invalid user id"},
	"user.info_missing":           {ZhCN: "未获取到用户信息", EnUS: "Could not read the current user"},
	"user.create_failed":          {ZhCN: "创建用户失败", EnUS: "Could not create the user"},
	"user.update_failed":          {ZhCN: "更新用户失败", EnUS: "Could not update the user"},
	"user.profile_update_failed":  {ZhCN: "更新用户资料失败", EnUS: "Could not update the profile"},
	"user.delete_failed":          {ZhCN: "删除用户失败", EnUS: "Could not delete the user"},
	"user.enable_failed":          {ZhCN: "启用用户失败", EnUS: "Could not enable the user"},
	"user.disable_failed":         {ZhCN: "禁用用户失败", EnUS: "Could not disable the user"},
	"user.list_failed":            {ZhCN: "获取用户列表失败", EnUS: "Could not load users"},
	"user.get_failed":             {ZhCN: "获取用户信息失败", EnUS: "Could not load the user"},
	"user.register_failed":        {ZhCN: "注册失败", EnUS: "Registration failed"},
	"user.login_failed":           {ZhCN: "登录失败", EnUS: "Sign-in failed"},
	"user.change_password_failed": {ZhCN: "修改密码失败", EnUS: "Could not change the password"},
	"user.reset_password_failed":  {ZhCN: "重置密码失败", EnUS: "Could not reset the password"},
	"user.request_failed":         {ZhCN: "处理请求失败", EnUS: "The request could not be processed"},
	"user.reset_mail_sent":        {ZhCN: "如果该邮箱已注册，您将收到重置密码的邮件", EnUS: "If that email is registered, a reset link is on its way"},
	"user.reset_token_invalid":    {ZhCN: "重置密码令牌无效", EnUS: "That reset link is not valid"},
	"user.reset_token_expired":    {ZhCN: "重置密码令牌已过期", EnUS: "That reset link has expired"},
	"user.reset_token_bad":        {ZhCN: "重置密码令牌无效或已过期", EnUS: "That reset link is invalid or has expired"},
	"user.session_expired":        {ZhCN: "登录状态已失效，请重新登录", EnUS: "Your session has expired — sign in again"},
	"user.token_invalid":          {ZhCN: "Token 无效或已过期", EnUS: "The token is invalid or has expired"},
	"user.token_valid":            {ZhCN: "令牌有效", EnUS: "The token is valid"},
	"user.verify_token_failed":    {ZhCN: "验证令牌失败", EnUS: "Could not verify the token"},

	// MFA
	"user.mfa_enabled":        {ZhCN: "MFA 已启用", EnUS: "MFA is on"},
	"user.mfa_disabled":       {ZhCN: "MFA 已禁用", EnUS: "MFA is off"},
	"user.mfa_not_enabled":    {ZhCN: "MFA 未启用", EnUS: "MFA is not enabled"},
	"user.mfa_not_started":    {ZhCN: "MFA 设置未开始", EnUS: "MFA setup has not been started"},
	"user.mfa_token_invalid":  {ZhCN: "MFA 挑战令牌无效或已过期，请重新登录", EnUS: "The MFA challenge has expired — sign in again"},
	"user.mfa_setup_failed":   {ZhCN: "设置 MFA 失败", EnUS: "Could not start MFA setup"},
	"user.mfa_enable_failed":  {ZhCN: "启用 MFA 失败", EnUS: "Could not turn on MFA"},
	"user.mfa_disable_failed": {ZhCN: "禁用 MFA 失败", EnUS: "Could not turn off MFA"},
	"user.mfa_status_failed":  {ZhCN: "获取 MFA 状态失败", EnUS: "Could not read the MFA status"},
	"user.mfa_verify_failed":  {ZhCN: "验证 MFA 失败", EnUS: "MFA verification failed"},
	"user.code_wrong":         {ZhCN: "验证码错误", EnUS: "That code is wrong"},
	"user.code_used":          {ZhCN: "该验证码已被使用，请等待下一个验证码", EnUS: "That code has already been used — wait for the next one"},

	// SSO
	"user.sso_disabled":         {ZhCN: "SSO 未启用", EnUS: "SSO is not enabled"},
	"user.sso_login_failed":     {ZhCN: "SSO 登录失败", EnUS: "SSO sign-in failed"},
	"user.sso_auth_failed":      {ZhCN: "SSO 认证失败", EnUS: "SSO authentication failed"},
	"user.sso_expired":          {ZhCN: "SSO 认证请求已过期，请重新登录", EnUS: "The SSO request has expired — sign in again"},
	"user.sso_token_failed":     {ZhCN: "SSO 令牌验证失败", EnUS: "The SSO token could not be verified"},
	"user.sso_id_token_failed":  {ZhCN: "ID Token 验证失败", EnUS: "The ID token could not be verified"},
	"user.sso_nonce_mismatch":   {ZhCN: "nonce 不匹配", EnUS: "The nonce does not match"},
	"user.sso_state_invalid":    {ZhCN: "无效的 state 参数", EnUS: "Invalid state parameter"},
	"user.sso_code_failed":      {ZhCN: "授权码交换失败", EnUS: "Could not exchange the authorization code"},
	"user.sso_url_failed":       {ZhCN: "获取 SSO 授权地址失败", EnUS: "Could not build the SSO authorization URL"},
	"user.sso_bound":            {ZhCN: "该账号已绑定 SSO，请通过 SSO 登录", EnUS: "This account uses SSO — sign in through your identity provider"},
	"user.sso_no_password":      {ZhCN: "SSO 用户不支持修改密码，请通过 SSO 提供方管理密码", EnUS: "SSO accounts manage their password at the identity provider"},
	"user.auth_request_invalid": {ZhCN: "无效的认证请求，请重新登录", EnUS: "Invalid authentication request — sign in again"},

	// API token
	"user.api_token_invalid":   {ZhCN: "无效的 API token", EnUS: "Invalid API token"},
	"user.api_token_expired":   {ZhCN: "API token 已过期", EnUS: "The API token has expired"},
	"user.api_token_bad":       {ZhCN: "API token 无效或已过期", EnUS: "The API token is invalid or has expired"},
	"user.token_not_found":     {ZhCN: "token 不存在", EnUS: "Token not found"},
	"user.token_limit":         {ZhCN: "token 数量超过上限 (20)", EnUS: "You have reached the limit of 20 tokens"},
	"user.token_by_token":      {ZhCN: "不能用 API token 创建新 token，请用账号登录", EnUS: "An API token cannot create another token — sign in with your account"},
	"user.token_invalid_id":    {ZhCN: "无效的 token ID", EnUS: "Invalid token id"},
	"user.token_create_failed": {ZhCN: "创建 token 失败", EnUS: "Could not create the token"},
	"user.token_revoke_failed": {ZhCN: "撤销 token 失败", EnUS: "Could not revoke the token"},
	"user.token_list_failed":   {ZhCN: "查询 token 列表失败", EnUS: "Could not load tokens"},
}

// ---------- 项目 ----------
var projectMessages = map[string]map[Lang]string{
	// project.not_found 已在 i18n.go 的通用表里登记
	"project.key_exists":    {ZhCN: "项目 Key 已存在", EnUS: "That project key is taken"},
	"project.deleted":       {ZhCN: "项目删除成功", EnUS: "Project deleted"},
	"project.create_failed": {ZhCN: "创建项目失败", EnUS: "Could not create the project"},
	"project.update_failed": {ZhCN: "更新项目失败", EnUS: "Could not update the project"},
	"project.delete_failed": {ZhCN: "删除项目失败", EnUS: "Could not delete the project"},
	"project.list_failed":   {ZhCN: "获取项目列表失败", EnUS: "Could not load projects"},
	"project.get_failed":    {ZhCN: "获取项目失败", EnUS: "Could not load the project"},
	"project.no_permission": {ZhCN: "没有操作权限", EnUS: "You do not have permission to do that"},

	"project.issue_type_not_found":     {ZhCN: "工单类型不存在", EnUS: "Issue type not found"},
	"project.issue_type_deleted":       {ZhCN: "工单类型删除成功", EnUS: "Issue type deleted"},
	"project.issue_type_invalid_id":    {ZhCN: "无效的工单类型 ID", EnUS: "Invalid issue type id"},
	"project.issue_type_create_failed": {ZhCN: "创建工单类型失败", EnUS: "Could not create the issue type"},
	"project.issue_type_update_failed": {ZhCN: "更新工单类型失败", EnUS: "Could not update the issue type"},
	"project.issue_type_list_failed":   {ZhCN: "获取工单类型列表失败", EnUS: "Could not load issue types"},

	"project.member_not_found":     {ZhCN: "成员不存在", EnUS: "Member not found"},
	"project.member_exists":        {ZhCN: "成员已存在", EnUS: "That user is already a member"},
	"project.member_removed":       {ZhCN: "成员移除成功", EnUS: "Member removed"},
	"project.member_add_failed":    {ZhCN: "添加成员失败", EnUS: "Could not add the member"},
	"project.member_update_failed": {ZhCN: "更新成员失败", EnUS: "Could not update the member"},
	"project.member_remove_failed": {ZhCN: "移除成员失败", EnUS: "Could not remove the member"},
	"project.member_list_failed":   {ZhCN: "获取成员列表失败", EnUS: "Could not load members"},
	"project.cannot_remove_owner":  {ZhCN: "不能移除项目所有者", EnUS: "The project owner cannot be removed"},
	"project.invalid_user_id":      {ZhCN: "无效的用户 ID", EnUS: "Invalid user id"},

	"project.role_not_found":            {ZhCN: "角色不存在", EnUS: "Role not found"},
	"project.role_key_exists":           {ZhCN: "角色 Key 已存在", EnUS: "That role key is taken"},
	"project.role_deleted":              {ZhCN: "角色删除成功", EnUS: "Role deleted"},
	"project.role_system":               {ZhCN: "不能删除系统预置角色", EnUS: "Built-in roles cannot be deleted"},
	"project.role_invalid_id":           {ZhCN: "无效的角色 ID", EnUS: "Invalid role id"},
	"project.role_create_failed":        {ZhCN: "创建角色失败", EnUS: "Could not create the role"},
	"project.role_update_failed":        {ZhCN: "更新角色失败", EnUS: "Could not update the role"},
	"project.role_delete_failed":        {ZhCN: "删除角色失败", EnUS: "Could not delete the role"},
	"project.role_list_failed":          {ZhCN: "获取角色列表失败", EnUS: "Could not load roles"},
	"project.role_member_not_found":     {ZhCN: "角色成员不存在", EnUS: "That user is not in this role"},
	"project.role_member_exists":        {ZhCN: "用户已在该角色中", EnUS: "That user is already in this role"},
	"project.role_member_removed":       {ZhCN: "角色成员移除成功", EnUS: "Removed from the role"},
	"project.role_member_add_failed":    {ZhCN: "添加角色成员失败", EnUS: "Could not add the user to the role"},
	"project.role_member_remove_failed": {ZhCN: "移除角色成员失败", EnUS: "Could not remove the user from the role"},
	"project.role_member_list_failed":   {ZhCN: "获取角色成员列表失败", EnUS: "Could not load role members"},
	"project.user_roles_failed":         {ZhCN: "获取用户角色失败", EnUS: "Could not load the user's roles"},
	"project.perm_saved":                {ZhCN: "权限设置成功", EnUS: "Permissions saved"},
	"project.perm_get_failed":           {ZhCN: "获取角色权限失败", EnUS: "Could not load the role permissions"},
	"project.perm_set_failed":           {ZhCN: "设置角色权限失败", EnUS: "Could not save the permissions"},

	"project.channel_not_found":     {ZhCN: "通知渠道不存在", EnUS: "Notification channel not found"},
	"project.channel_invalid":       {ZhCN: "渠道配置无效", EnUS: "The channel configuration is not valid"},
	"project.channel_invalid_id":    {ZhCN: "无效的渠道 ID", EnUS: "Invalid channel id"},
	"project.channel_create_failed": {ZhCN: "创建通知渠道失败", EnUS: "Could not create the channel"},
	"project.channel_update_failed": {ZhCN: "更新通知渠道失败", EnUS: "Could not update the channel"},
	"project.channel_delete_failed": {ZhCN: "删除通知渠道失败", EnUS: "Could not delete the channel"},
	"project.channel_get_failed":    {ZhCN: "获取通知渠道失败", EnUS: "Could not load the channel"},
	"project.channel_query_failed":  {ZhCN: "查询通知渠道失败", EnUS: "Could not query channels"},
	"project.test_sent":             {ZhCN: "测试消息发送成功", EnUS: "Test message sent"},
	"project.test_failed":           {ZhCN: "测试发送失败: ", EnUS: "Could not send the test message: "},
	"project.digest_triggered":      {ZhCN: "日报已触发", EnUS: "Digest sent"},
	"project.digest_failed":         {ZhCN: "触发日报失败: ", EnUS: "Could not send the digest: "},
	"project.delete_failed_detail":  {ZhCN: "删除项目失败: ", EnUS: "Could not delete the project: "},
}

// ---------- 工作流 ----------
var workflowMessages = map[string]map[Lang]string{
	"workflow.not_found":      {ZhCN: "工作流不存在", EnUS: "Workflow not found"},
	"workflow.deleted":        {ZhCN: "工作流删除成功", EnUS: "Workflow deleted"},
	"workflow.invalid_id":     {ZhCN: "无效的工作流 ID", EnUS: "Invalid workflow id"},
	"workflow.create_failed":  {ZhCN: "创建工作流失败", EnUS: "Could not create the workflow"},
	"workflow.update_failed":  {ZhCN: "更新工作流失败", EnUS: "Could not update the workflow"},
	"workflow.delete_failed":  {ZhCN: "删除工作流失败", EnUS: "Could not delete the workflow"},
	"workflow.list_failed":    {ZhCN: "获取工作流列表失败", EnUS: "Could not load workflows"},
	"workflow.get_failed":     {ZhCN: "获取工作流失败", EnUS: "Could not load the workflow"},
	"workflow.config_invalid": {ZhCN: "工作流配置无效", EnUS: "The workflow configuration is not valid"},

	"workflow.instance_not_found": {ZhCN: "工作流实例不存在", EnUS: "Workflow instance not found"},
	"workflow.instance_failed":    {ZhCN: "获取工作流实例失败", EnUS: "Could not load the workflow instance"},
	"workflow.no_instance":        {ZhCN: "该工单没有关联的工作流实例", EnUS: "This issue has no workflow instance"},

	"workflow.scheme_not_found":     {ZhCN: "工作流方案不存在", EnUS: "Workflow scheme not found"},
	"workflow.scheme_exists":        {ZhCN: "工作流方案已存在", EnUS: "That workflow scheme already exists"},
	"workflow.scheme_deleted":       {ZhCN: "工作流方案删除成功", EnUS: "Workflow scheme deleted"},
	"workflow.scheme_type_taken":    {ZhCN: "该工单类型已配置工作流方案", EnUS: "That issue type already has a workflow scheme"},
	"workflow.scheme_create_failed": {ZhCN: "创建工作流方案失败", EnUS: "Could not create the workflow scheme"},
	"workflow.scheme_delete_failed": {ZhCN: "删除工作流方案失败", EnUS: "Could not delete the workflow scheme"},
	"workflow.scheme_list_failed":   {ZhCN: "获取工作流方案列表失败", EnUS: "Could not load workflow schemes"},

	"workflow.node_not_found":     {ZhCN: "节点不存在", EnUS: "Step not found"},
	"workflow.node_deleted":       {ZhCN: "节点删除成功", EnUS: "Step deleted"},
	"workflow.node_in_use":        {ZhCN: "节点正在使用中", EnUS: "That step is still in use"},
	"workflow.node_invalid_id":    {ZhCN: "无效的节点 ID", EnUS: "Invalid step id"},
	"workflow.node_create_failed": {ZhCN: "创建节点失败", EnUS: "Could not create the step"},
	"workflow.node_update_failed": {ZhCN: "更新节点失败", EnUS: "Could not update the step"},
	"workflow.node_list_failed":   {ZhCN: "获取节点列表失败", EnUS: "Could not load steps"},
	"workflow.node_get_failed":    {ZhCN: "获取节点失败", EnUS: "Could not load the step"},

	"workflow.edge_not_found":     {ZhCN: "边不存在", EnUS: "Edge not found"},
	"workflow.edge_deleted":       {ZhCN: "边删除成功", EnUS: "Edge deleted"},
	"workflow.edge_invalid_id":    {ZhCN: "无效的边 ID", EnUS: "Invalid edge id"},
	"workflow.edge_delete_failed": {ZhCN: "删除边失败", EnUS: "Could not delete the edge"},
	"workflow.edge_update_failed": {ZhCN: "更新边失败", EnUS: "Could not update the edge"},
	"workflow.edge_list_failed":   {ZhCN: "获取边列表失败", EnUS: "Could not load edges"},
	"workflow.edge_get_failed":    {ZhCN: "获取边失败", EnUS: "Could not load the edge"},

	"workflow.approved":           {ZhCN: "审批通过", EnUS: "Approved"},
	"workflow.rejected":           {ZhCN: "审批已拒绝", EnUS: "Rejected"},
	"workflow.already_approved":   {ZhCN: "已经审批过了", EnUS: "This step has already been decided"},
	"workflow.not_approver":       {ZhCN: "当前用户不是审批人", EnUS: "You are not an approver for this step"},
	"workflow.no_approver":        {ZhCN: "未配置审批人", EnUS: "No approver is configured"},
	"workflow.not_approval_node":  {ZhCN: "当前节点不是审批节点", EnUS: "The current step is not an approval step"},
	"workflow.approve_wrong_node": {ZhCN: "当前节点不是审批节点，无法执行审批操作", EnUS: "The current step is not an approval step, so it cannot be approved"},
	"workflow.reject_wrong_node":  {ZhCN: "当前节点不是审批节点，无法执行拒绝操作", EnUS: "The current step is not an approval step, so it cannot be rejected"},
	"workflow.work_node_done":     {ZhCN: "工作节点已完成", EnUS: "Step completed"},
	"workflow.approve_failed":     {ZhCN: "审批操作失败: ", EnUS: "Approval failed: "},
	"workflow.reject_failed":      {ZhCN: "拒绝操作失败: ", EnUS: "Rejection failed: "},
	"workflow.complete_failed":    {ZhCN: "完成操作失败: ", EnUS: "Could not complete the step: "},
	"workflow.history_failed":     {ZhCN: "获取流转历史失败", EnUS: "Could not load the transition history"},

	"workflow.issue_key_required":    {ZhCN: "工单 Key 不能为空", EnUS: "An issue key is required"},
	"workflow.project_key_required":  {ZhCN: "项目 Key 不能为空", EnUS: "A project key is required"},
	"workflow.issue_not_found":       {ZhCN: "工单不存在", EnUS: "Issue not found"},
	"workflow.issue_query_failed":    {ZhCN: "查询工单失败", EnUS: "Could not load the issue"},
	"workflow.project_not_found":     {ZhCN: "项目不存在", EnUS: "Project not found"},
	"workflow.project_query_failed":  {ZhCN: "查询项目失败", EnUS: "Could not load the project"},
	"workflow.issue_type_invalid_id": {ZhCN: "无效的工单类型 ID", EnUS: "Invalid issue type id"},
	"workflow.user_missing":          {ZhCN: "未获取到用户信息", EnUS: "Could not read the current user"},
}

// ---------- 工单（补充）----------
var issueExtraMessages = map[string]map[Lang]string{
	"issue.epic_not_found":          {ZhCN: "Epic 不存在", EnUS: "Epic not found"},
	"issue.worklog_not_found":       {ZhCN: "工作日志不存在", EnUS: "Worklog not found"},
	"issue.comment_not_found":       {ZhCN: "评论不存在", EnUS: "Comment not found"},
	"issue.user_not_found":          {ZhCN: "用户不存在", EnUS: "User not found"},
	"issue.project_not_found":       {ZhCN: "项目不存在", EnUS: "Project not found"},
	"issue.not_watching":            {ZhCN: "未关注该工单", EnUS: "You are not watching this issue"},
	"issue.watched":                 {ZhCN: "关注成功", EnUS: "Watching this issue"},
	"issue.unwatched":               {ZhCN: "取消关注成功", EnUS: "No longer watching"},
	"issue.deleted":                 {ZhCN: "工单删除成功", EnUS: "Issue deleted"},
	"issue.comment_deleted":         {ZhCN: "评论删除成功", EnUS: "Comment deleted"},
	"issue.invalid_user_id":         {ZhCN: "无效的用户 ID", EnUS: "Invalid user id"},
	"issue.invalid_comment_id2":     {ZhCN: "无效的评论 ID", EnUS: "Invalid comment id"},
	"issue.no_permission":           {ZhCN: "无权限操作", EnUS: "You do not have permission to do that"},
	"issue.bad_time_format":         {ZhCN: "时间格式错误", EnUS: "Invalid time format"},
	"issue.unsupported_type":        {ZhCN: "不支持的文件类型", EnUS: "That file type is not supported"},
	"issue.multipart_failed_detail": {ZhCN: "解析 multipart 表单失败: ", EnUS: "Could not read the uploaded form: "},
}

// ---------- 告警 ----------
var alertMessages = map[string]map[Lang]string{
	"alert.not_found":                   {ZhCN: "告警不存在", EnUS: "Alert not found"},
	"alert.rule_not_found":              {ZhCN: "告警规则不存在", EnUS: "Alert rule not found"},
	"alert.silence_not_found":           {ZhCN: "告警静默不存在", EnUS: "Silence not found"},
	"alert.datasource_not_found":        {ZhCN: "数据源不存在", EnUS: "Data source not found"},
	"alert.datasource_disabled":         {ZhCN: "数据源已禁用", EnUS: "That data source is disabled"},
	"alert.invalid_id":                  {ZhCN: "无效的告警 ID", EnUS: "Invalid alert id"},
	"alert.invalid_rule_id":             {ZhCN: "无效的规则 ID", EnUS: "Invalid rule id"},
	"alert.invalid_silence_id":          {ZhCN: "无效的静默 ID", EnUS: "Invalid silence id"},
	"alert.invalid_datasource_id":       {ZhCN: "无效的数据源 ID", EnUS: "Invalid data source id"},
	"alert.invalid_user_ctx":            {ZhCN: "无效的用户上下文", EnUS: "Invalid user context"},
	"alert.unauthenticated":             {ZhCN: "未认证", EnUS: "Not signed in"},
	"alert.list_failed":                 {ZhCN: "获取告警列表失败", EnUS: "Could not load alerts"},
	"alert.stats_failed":                {ZhCN: "获取告警统计失败", EnUS: "Could not load alert statistics"},
	"alert.group_failed":                {ZhCN: "分组统计告警失败", EnUS: "Could not group the alerts"},
	"alert.labels_failed":               {ZhCN: "获取标签列表失败", EnUS: "Could not load labels"},
	"alert.rule_list_failed":            {ZhCN: "获取告警规则列表失败", EnUS: "Could not load alert rules"},
	"alert.rule_create_failed":          {ZhCN: "创建告警规则失败", EnUS: "Could not create the alert rule"},
	"alert.rule_update_failed":          {ZhCN: "更新告警规则失败", EnUS: "Could not update the alert rule"},
	"alert.rule_delete_failed":          {ZhCN: "删除告警规则失败", EnUS: "Could not delete the alert rule"},
	"alert.silence_list_failed":         {ZhCN: "获取告警静默列表失败", EnUS: "Could not load silences"},
	"alert.silence_create_failed":       {ZhCN: "创建告警静默失败", EnUS: "Could not create the silence"},
	"alert.silence_update_failed":       {ZhCN: "更新告警静默失败", EnUS: "Could not update the silence"},
	"alert.silence_delete_failed":       {ZhCN: "删除告警静默失败", EnUS: "Could not delete the silence"},
	"alert.silence_cancel_failed":       {ZhCN: "取消告警静默失败", EnUS: "Could not cancel the silence"},
	"alert.datasource_list_failed":      {ZhCN: "获取数据源列表失败", EnUS: "Could not load data sources"},
	"alert.test_failed":                 {ZhCN: "测试连接失败", EnUS: "Connection test failed"},
	"alert.webhook_failed":              {ZhCN: "处理 Webhook 失败", EnUS: "Could not process the webhook"},
	"alert.n9e_webhook_failed":          {ZhCN: "处理夜莺 Webhook 失败", EnUS: "Could not process the Nightingale webhook"},
	"alert.unsupported_datasource":      {ZhCN: "不支持的数据源类型: ", EnUS: "Unsupported data source type: "},
	"alert.datasource_not_found_detail": {ZhCN: "数据源不存在: ", EnUS: "Data source not found: "},
	"alert.datasource_create_failed":    {ZhCN: "创建数据源失败: ", EnUS: "Could not create the data source: "},
	"alert.datasource_update_failed":    {ZhCN: "更新数据源失败: ", EnUS: "Could not update the data source: "},
	"alert.ack_failed":                  {ZhCN: "确认告警失败: ", EnUS: "Could not acknowledge the alert: "},
	"alert.resolve_failed":              {ZhCN: "解决告警失败: ", EnUS: "Could not resolve the alert: "},
	"alert.read_body_failed":            {ZhCN: "读取请求体失败: ", EnUS: "Could not read the request body: "},
}

// ---------- 字段 ----------
var fieldMessages = map[string]map[Lang]string{
	"field.not_found":             {ZhCN: "字段不存在", EnUS: "Field not found"},
	"field.key_exists":            {ZhCN: "字段Key已存在", EnUS: "That field key is taken"},
	"field.system_immutable":      {ZhCN: "不能修改系统字段", EnUS: "Built-in fields cannot be edited"},
	"field.system_undeletable":    {ZhCN: "不能删除系统字段", EnUS: "Built-in fields cannot be deleted"},
	"field.invalid_id":            {ZhCN: "无效的字段ID", EnUS: "Invalid field id"},
	"field.invalid_issue_id":      {ZhCN: "无效的工单ID", EnUS: "Invalid issue id"},
	"field.invalid_issue_type_id": {ZhCN: "无效的工单类型ID", EnUS: "Invalid issue type id"},
	"field.issue_type_not_found":  {ZhCN: "工单类型不存在", EnUS: "Issue type not found"},
	"field.project_not_found":     {ZhCN: "项目不存在", EnUS: "Project not found"},
	"field.template_not_found":    {ZhCN: "模板不存在", EnUS: "Template not found"},
	"field.template_name_exists":  {ZhCN: "模板名称已存在", EnUS: "That template name is taken"},
	"field.invalid_template_id":   {ZhCN: "无效的模板ID", EnUS: "Invalid template id"},
	"field.label_not_found":       {ZhCN: "标签不存在", EnUS: "Label not found"},
	"field.label_name_exists":     {ZhCN: "标签名称已存在", EnUS: "That label name is taken"},
	"field.invalid_label_id":      {ZhCN: "无效的标签ID", EnUS: "Invalid label id"},
	"field.version_not_found":     {ZhCN: "版本不存在", EnUS: "Version not found"},
	"field.version_name_exists":   {ZhCN: "版本名称已存在", EnUS: "That version name is taken"},
	"field.invalid_version_id":    {ZhCN: "无效的版本ID", EnUS: "Invalid version id"},
	"field.component_not_found":   {ZhCN: "组件不存在", EnUS: "Component not found"},
	"field.component_name_exists": {ZhCN: "组件名称已存在", EnUS: "That component name is taken"},
	"field.invalid_component_id":  {ZhCN: "无效的组件ID", EnUS: "Invalid component id"},
}

// ---------- 需求池 ----------
var requirementMessages = map[string]map[Lang]string{
	"requirement.not_found":             {ZhCN: "需求不存在", EnUS: "Requirement not found"},
	"requirement.pool_not_found":        {ZhCN: "需求池不存在", EnUS: "Requirement pool not found"},
	"requirement.pool_archived":         {ZhCN: "需求池已归档，无法添加需求", EnUS: "This pool is archived and cannot take new requirements"},
	"requirement.global_no_project":     {ZhCN: "全局需求池不能关联项目", EnUS: "A global pool cannot be linked to a project"},
	"requirement.project_needs_project": {ZhCN: "项目级需求池必须关联项目", EnUS: "A project pool must be linked to a project"},
	"requirement.category_not_found":    {ZhCN: "分类不存在", EnUS: "Category not found"},
	"requirement.category_exists":       {ZhCN: "分类名称已存在", EnUS: "That category name is taken"},
	"requirement.category_system":       {ZhCN: "系统预置分类不可删除", EnUS: "Built-in categories cannot be deleted"},
	"requirement.invalid_id":            {ZhCN: "无效的需求ID", EnUS: "Invalid requirement id"},
	"requirement.invalid_pool_id":       {ZhCN: "无效的需求池ID", EnUS: "Invalid pool id"},
	"requirement.invalid_category_id":   {ZhCN: "无效的分类ID", EnUS: "Invalid category id"},
	"requirement.already_converted":     {ZhCN: "该需求已转化为工单，不允许重复转化", EnUS: "This requirement has already been converted to an issue"},
	"requirement.completed_no_convert":  {ZhCN: "已完成的需求不能转化为工单", EnUS: "A completed requirement cannot be converted"},
	"requirement.rejected_no_convert":   {ZhCN: "已拒绝的需求不能转化为工单", EnUS: "A rejected requirement cannot be converted"},
	"requirement.service_uninitialized": {ZhCN: "工单创建服务未初始化", EnUS: "The issue service is not available"},
	"requirement.deleted":               {ZhCN: "删除成功", EnUS: "Deleted"},
	"requirement.updated":               {ZhCN: "更新成功", EnUS: "Updated"},
}

// ---------- 系统配置 ----------
var systemMessages = map[string]map[Lang]string{
	"system.config_not_found":       {ZhCN: "配置不存在", EnUS: "Setting not found"},
	"system.config_key_empty":       {ZhCN: "配置键不能为空", EnUS: "A setting key is required"},
	"system.config_no_access":       {ZhCN: "无权访问该配置项", EnUS: "You do not have access to that setting"},
	"system.category_empty":         {ZhCN: "分类不能为空", EnUS: "A category is required"},
	"system.config_update_failed":   {ZhCN: "更新配置失败: ", EnUS: "Could not update the setting: "},
	"system.webhook_not_found":      {ZhCN: "Webhook 不存在", EnUS: "Webhook not found"},
	"system.invalid_webhook_id":     {ZhCN: "无效的 Webhook ID", EnUS: "Invalid webhook id"},
	"system.choose_file":            {ZhCN: "请选择文件上传", EnUS: "Choose a file to upload"},
	"system.file_name_invalid":      {ZhCN: "文件名不合法", EnUS: "That file name is not allowed"},
	"system.file_too_large_2mb":     {ZhCN: "文件大小不能超过 2MB", EnUS: "The file must be 2MB or smaller"},
	"system.unsupported_image":      {ZhCN: "不支持的文件格式，仅支持 SVG、PNG、ICO、JPG、WEBP", EnUS: "Unsupported format — use SVG, PNG, ICO, JPG or WEBP"},
	"system.asset_type_invalid":     {ZhCN: "类型参数错误，仅支持 logo 或 favicon", EnUS: "Invalid asset type — use logo or favicon"},
	"system.open_file_failed":       {ZhCN: "打开文件失败", EnUS: "Could not open the file"},
	"system.save_file_failed":       {ZhCN: "保存文件失败: ", EnUS: "Could not save the file: "},
	"system.storage_uninitialized":  {ZhCN: "文件存储服务未初始化", EnUS: "File storage is not available"},
	"system.lark_uninitialized":     {ZhCN: "飞书服务未初始化", EnUS: "The Lark integration is not available"},
	"system.lark_test_failed":       {ZhCN: "飞书测试消息发送失败: ", EnUS: "Could not send the Lark test message: "},
	"system.telegram_uninitialized": {ZhCN: "Telegram 服务未初始化", EnUS: "The Telegram integration is not available"},
	"system.telegram_test_failed":   {ZhCN: "Telegram 测试消息发送失败: ", EnUS: "Could not send the Telegram test message: "},
}

// ---------- 站内通知 / 报表 / API 文档 / 对象存储 ----------
var miscMessages = map[string]map[Lang]string{
	"inbox.invalid_id":      {ZhCN: "无效的通知ID", EnUS: "Invalid notification id"},
	"inbox.token_invalid":   {ZhCN: "Token 无效", EnUS: "Invalid token"},
	"inbox.no_auth":         {ZhCN: "缺少认证信息", EnUS: "Missing credentials"},
	"inbox.not_logged_in":   {ZhCN: "未登录或用户信息缺失", EnUS: "Not signed in"},
	"inbox.user_type_error": {ZhCN: "用户信息类型错误", EnUS: "Malformed user context"},
	"inbox.bad_request":     {ZhCN: "参数错误", EnUS: "Invalid request"},

	"report.issue_stats_failed":   {ZhCN: "获取工单统计失败", EnUS: "Could not load issue statistics"},
	"report.alert_stats_failed":   {ZhCN: "获取告警统计失败", EnUS: "Could not load alert statistics"},
	"report.dashboard_failed":     {ZhCN: "获取仪表盘统计失败", EnUS: "Could not load the dashboard"},
	"report.sla_failed":           {ZhCN: "获取 SLA 报表失败", EnUS: "Could not load the SLA report"},
	"report.delivery_failed":      {ZhCN: "获取交付报表失败", EnUS: "Could not load the delivery report"},
	"report.worklog_stats_failed": {ZhCN: "获取工时统计失败", EnUS: "Could not load time statistics"},
	"report.performance_failed":   {ZhCN: "获取用户绩效失败", EnUS: "Could not load the performance report"},

	"apidoc.login_required":  {ZhCN: "请登录后访问 API 文档", EnUS: "Sign in to view the API reference"},
	"apidoc.need_bearer":     {ZhCN: "需 Bearer token", EnUS: "A bearer token is required"},
	"apidoc.token_invalid":   {ZhCN: "API token 无效或已过期", EnUS: "The API token is invalid or has expired"},
	"apidoc.session_expired": {ZhCN: "登录已过期, 请重新登录", EnUS: "Your session has expired — sign in again"},
	"apidoc.not_found":       {ZhCN: "资源不存在", EnUS: "Not found"},
	"apidoc.read_failed":     {ZhCN: "读取资源失败", EnUS: "Could not read the resource"},

	"storage.object_not_found": {ZhCN: "对象不存在", EnUS: "Object not found"},
	"storage.invalid_key":      {ZhCN: "非法的对象键", EnUS: "Invalid object key"},
	"storage.no_bucket":        {ZhCN: "storage.s3.bucket 未配置", EnUS: "storage.s3.bucket is not configured"},
	"storage.no_endpoint":      {ZhCN: "storage.s3.endpoint 未配置", EnUS: "storage.s3.endpoint is not configured"},
}

// ---------- 通知模板（飞书 / Telegram / 邮件 / 日报）----------
//
// 这批文案跑在没有 HTTP 请求的后台任务里，语言取 app.language 配置，
// 用 B / Bf 取，不是 T / Tf。
var notifyMessages = map[string]map[Lang]string{
	// 卡片标题
	"notify.title_issue_created":      {ZhCN: "工单创建", EnUS: "Issue created"},
	"notify.title_issue_updated":      {ZhCN: "工单更新", EnUS: "Issue updated"},
	"notify.title_issue_transitioned": {ZhCN: "工单流转", EnUS: "Issue moved"},
	"notify.title_issue_assigned":     {ZhCN: "工单指派", EnUS: "Issue assigned"},
	"notify.title_issue_commented":    {ZhCN: "工单评论", EnUS: "New comment"},
	"notify.title_alert_firing":       {ZhCN: "告警触发", EnUS: "Alert firing"},
	"notify.title_alert_resolved":     {ZhCN: "告警恢复", EnUS: "Alert resolved"},
	"notify.title_alert_merged":       {ZhCN: "告警合并", EnUS: "Alert merged"},
	"notify.title_alert_acked":        {ZhCN: "告警确认", EnUS: "Alert acknowledged"},
	"notify.title_system":             {ZhCN: "系统通知", EnUS: "Notification"},
	"notify.title_alert_issue":        {ZhCN: "告警建单", EnUS: "Issue opened from an alert"},

	// 字段标签
	"notify.label_status":     {ZhCN: "状态", EnUS: "Status"},
	"notify.label_project":    {ZhCN: "项目", EnUS: "Project"},
	"notify.label_priority":   {ZhCN: "优先级", EnUS: "Priority"},
	"notify.label_handler":    {ZhCN: "处理人", EnUS: "Assignee"},
	"notify.label_due":        {ZhCN: "截止时间", EnUS: "Due"},
	"notify.label_assign_to":  {ZhCN: "指派给", EnUS: "Assigned to"},
	"notify.label_assign":     {ZhCN: "指派", EnUS: "Assignee"},
	"notify.label_alert":      {ZhCN: "告警", EnUS: "Alert"},
	"notify.label_instance":   {ZhCN: "新增实例", EnUS: "New instance"},
	"notify.label_count":      {ZhCN: "当前实例数", EnUS: "Instances"},
	"notify.label_comment":    {ZhCN: "评论", EnUS: "Comment"},
	"notify.label_severity":   {ZhCN: "级别", EnUS: "Severity"},
	"notify.label_issue":      {ZhCN: "关联工单", EnUS: "Issue"},
	"notify.label_alert_time": {ZhCN: "告警时间", EnUS: "Fired at"},
	"notify.assigned_by":      {ZhCN: "%s 指派给", EnUS: "%s assigned it to"},

	// 按钮
	"notify.btn_view_issue":   {ZhCN: "查看工单", EnUS: "Open issue"},
	"notify.btn_view_project": {ZhCN: "查看项目", EnUS: "Open project"},

	// 测试消息
	"notify.test_title":      {ZhCN: "TicketDesk 通知测试", EnUS: "TicketDesk notification test"},
	"notify.test_lark_ok":    {ZhCN: "恭喜！飞书通知配置成功。\n\n此消息由 TicketDesk 系统发送，用于验证飞书通知功能是否正常工作。", EnUS: "Your Lark channel works.\n\nTicketDesk sent this message to check the integration."},
	"notify.test_tg_ok":      {ZhCN: "恭喜！Telegram 通知配置成功。\n\n此消息由 TicketDesk 系统发送，用于验证 Telegram 通知功能是否正常工作。", EnUS: "Your Telegram channel works.\n\nTicketDesk sent this message to check the integration."},
	"notify.test_from":       {ZhCN: "来自 TicketDesk · %s", EnUS: "From TicketDesk · %s"},
	"notify.test_issue_lark": {ZhCN: "这是一条测试通知，用于验证飞书通知渠道是否配置正确", EnUS: "A test notification, to check the Lark channel is set up correctly"},
	"notify.test_issue_tg":   {ZhCN: "这是一条测试通知，用于验证 Telegram 通知渠道是否配置正确", EnUS: "A test notification, to check the Telegram channel is set up correctly"},
	"notify.test_project":    {ZhCN: "测试项目", EnUS: "Test project"},
	"notify.test_status":     {ZhCN: "待处理", EnUS: "Open"},

	// 日报
	"notify.digest_unassigned":    {ZhCN: "未指派", EnUS: "Unassigned"},
	"notify.digest_user_fallback": {ZhCN: "用户#%d", EnUS: "User #%d"},
	"notify.digest_title":         {ZhCN: "每日工单日报", EnUS: "Daily issue digest"},
	"notify.digest_open_count":    {ZhCN: "未完结工单共 <b>%d</b> 条", EnUS: "has <b>%d</b> open issues"},
	"notify.digest_header":        {ZhCN: "**[%s] %s** 未完结工单共 **%d** 条", EnUS: "**[%s] %s** — **%d** open issues"},
}

// ---------- 工单状态（通知模板用）----------
var statusMessages = map[string]map[Lang]string{
	"status.open":           {ZhCN: "待处理", EnUS: "Open"},
	"status.in_progress":    {ZhCN: "进行中", EnUS: "In progress"},
	"status.pending_review": {ZhCN: "待确认", EnUS: "In review"},
	"status.reviewing":      {ZhCN: "待确认", EnUS: "In review"},
	"status.resolved":       {ZhCN: "已解决", EnUS: "Resolved"},
	"status.closed":         {ZhCN: "已关闭", EnUS: "Closed"},
	"status.merged":         {ZhCN: "已合并", EnUS: "Merged"},
}

// ---------- 站内通知标题（按收件人语言渲染）----------
//
// 与 notify.* 的区别：那批发到群里，一条消息给一群人，只能用站点语言；
// 这批发给具体的人，标题在建单时就渲染好落库，能按收件人的 locale 走。
var inboxMessages = map[string]map[Lang]string{
	"inbox.issue_assigned_to_you": {ZhCN: "工单 %s 被指派给您", EnUS: "Issue %s was assigned to you"},
	"inbox.issue_reassigned":      {ZhCN: "您创建的工单 %s 被重新指派", EnUS: "Issue %s you reported was reassigned"},
	"inbox.issue_commented":       {ZhCN: "工单 %s 有新评论", EnUS: "New comment on issue %s"},
	"inbox.your_issue_commented":  {ZhCN: "您创建的工单 %s 有新评论", EnUS: "New comment on issue %s you reported"},
	"inbox.mentioned_you":         {ZhCN: "%s 在工单 %s 中提及了您", EnUS: "%s mentioned you on issue %s"},
	"inbox.alert_issue_assigned":  {ZhCN: "告警工单 %s 已指派给您", EnUS: "Alert issue %s was assigned to you"},
	"inbox.alert_merged":          {ZhCN: "🔔 工单 %s 有新告警合并 (%d 个实例)", EnUS: "🔔 An alert merged into issue %s (%d instances)"},
	"inbox.worklog_added":         {ZhCN: "工单 %s 添加了工作日志", EnUS: "Work logged on issue %s"},
	"inbox.worklog_added_by":      {ZhCN: "%s 添加了工作日志（%s）", EnUS: "%s logged %s"},
	"inbox.alert_merged_body":     {ZhCN: "新告警实例: %s (指纹: %s)", EnUS: "New alert instance: %s (fingerprint: %s)"},
	"inbox.issue_updated":         {ZhCN: "工单 %s 已更新", EnUS: "Issue %s was updated"},
}

// ---------- 邮件（按收件人语言渲染）----------
var mailMessages = map[string]map[Lang]string{
	"mail.reset_subject":  {ZhCN: "重置密码 - TicketDesk", EnUS: "Reset your password - TicketDesk"},
	"mail.reset_title":    {ZhCN: "重置密码", EnUS: "Reset your password"},
	"mail.reset_greeting": {ZhCN: "您好，%s！", EnUS: "Hi %s,"},
	"mail.reset_intro":    {ZhCN: "我们收到了您的密码重置请求。请点击下面的按钮重置您的密码：", EnUS: "We received a request to reset your password. Use the button below to set a new one:"},
	"mail.reset_button":   {ZhCN: "重置密码", EnUS: "Reset password"},
	"mail.reset_or_copy":  {ZhCN: "或者复制以下链接到浏览器中打开：", EnUS: "Or paste this link into your browser:"},
	"mail.reset_warning":  {ZhCN: "安全提示：", EnUS: "Before you do:"},
	"mail.reset_expiry":   {ZhCN: "此链接将在 30 分钟后失效", EnUS: "This link expires in 30 minutes"},
	"mail.reset_ignore":   {ZhCN: "如果您没有请求重置密码，请忽略此邮件", EnUS: "If you did not ask for this, ignore this email"},
	"mail.reset_private":  {ZhCN: "请勿将此链接分享给他人", EnUS: "Do not share this link with anyone"},
}

// ---------- 初始化向导 ----------
var setupMessages = map[string]map[Lang]string{
	"setup.required":            {ZhCN: "系统尚未初始化，请先完成初始化向导", EnUS: "This instance is not set up yet — complete the setup wizard first"},
	"setup.already_initialized": {ZhCN: "系统已初始化，无法重复初始化", EnUS: "This instance is already set up"},
	"setup.bad_token":           {ZhCN: "初始化令牌不正确", EnUS: "That setup token is not correct"},
	"setup.failed":              {ZhCN: "初始化失败", EnUS: "Setup failed"},
}
