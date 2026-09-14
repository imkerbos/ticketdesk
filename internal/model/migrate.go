// Package model 提供数据库迁移功能
package model

import (
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/kerbos/ticketdesk/pkg/logger"
)

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	logger.Info("starting database auto migration...")

	models := []interface{}{
		&User{},
		&Role{},
		&UserRole{},
		&Project{},
		&ProjectMember{},
		&ProjectRole{},
		&ProjectRoleMember{},
		&IssueType{},
		&Issue{},
		&IssueComment{},
		&IssueAttachment{},
		&IssueWatcher{},
		&IssueWorklog{},
		&Workflow{},
		&WorkflowNode{},
		&WorkflowEdge{},
		&WorkflowInstance{},
		&WorkflowHistory{},
		&ApprovalRecord{},
		&WorkflowScheme{},
		&Alert{},
		&AlertRule{},
		&AlertSilence{},
		&SystemConfig{},
		&Webhook{},
		&WebhookLog{},
		&ActivityLog{},
		&Notification{},
		&RequirementPool{},
		&Requirement{},
		&RequirementComment{},
		&RequirementAttachment{},
		&FieldDefinition{},
		&IssueTypeFieldScheme{},
		&IssueFieldValue{},
		&FieldSchemeTemplate{},
		&FieldSchemeTemplateItem{},
		&ProjectVersion{},
		&ProjectComponent{},
		&IssueLabel{},
		&AlertDatasource{},
		&ProjectNotificationChannel{},
		&ProjectRolePermission{},
		&RequirementCategoryDef{},
		&APIToken{},
		&DailyDigestRun{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			logger.Error("failed to migrate model", zap.Error(err))
			return err
		}
	}

	logger.Info("database auto migration completed")

	// 创建复合索引（提升工单查询性能）
	createCompositeIndexes(db)

	// 迁移需求旧状态值到新状态值
	if err := migrateRequirementStatus(db); err != nil {
		logger.Error("failed to migrate requirement status", zap.Error(err))
		return err
	}

	// 迁移项目成员旧角色值到项目角色 key
	if err := migrateMemberRoles(db); err != nil {
		logger.Error("failed to migrate member roles", zap.Error(err))
		return err
	}

	// 为旧的项目通知渠道补齐订阅事件（默认全订阅）
	if err := migrateNotificationChannelEventTypes(db); err != nil {
		logger.Error("failed to migrate notification channel event types", zap.Error(err))
		return err
	}

	return nil
}

// migrateNotificationChannelEventTypes 为旧的通知渠道补默认订阅事件
// 仅影响 event_types 为 NULL 或空字符串的旧数据，新建渠道由 service 层显式写入
func migrateNotificationChannelEventTypes(db *gorm.DB) error {
	const defaultEvents = `["issue.created","issue.transitioned","issue.assigned","alert.merged"]`
	result := db.Exec(
		"UPDATE project_notification_channels SET event_types = ? WHERE event_types IS NULL OR event_types = ''",
		defaultEvents,
	)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		logger.Info("migrated notification channel event types",
			zap.Int64("count", result.RowsAffected),
			zap.String("default_events", defaultEvents),
		)
	}
	return nil
}

// createCompositeIndexes 创建复合索引（提升查询性能）
func createCompositeIndexes(db *gorm.DB) {
	// 清理旧的次优索引（不包含排序列，无法避免 filesort）
	oldIndexes := []struct {
		table string
		name  string
	}{
		{"issues", "idx_issues_project_status"},
		{"issues", "idx_issues_assignee_status"},
		{"issues", "idx_issues_reporter_created"},
		{"issues", "idx_issues_project_created"},
		{"issues", "idx_issues_status_created"},
		// deleted_at 单列索引：99.99% 行为 NULL，无过滤效果，反而误导优化器选错索引
		{"issues", "idx_issues_deleted_at"},
		// 以下单列索引均为下方复合索引的最左前缀，或基数极低（priority 4 值、resolution 7 值），
		// 优化器基本不会选中，却让 issues 这张最热写表每次写入多维护一棵 B+ 树。
		{"issues", "idx_issues_project_id"},
		{"issues", "idx_issues_assignee_id"},
		{"issues", "idx_issues_reporter_id"},
		{"issues", "idx_issues_epic_id"},
		{"issues", "idx_issues_priority"},
		{"issues", "idx_issues_resolution"},
		// 与 idx_issues_project_type_id 完全重复（后者多带 id DESC，严格更优）
		{"issues", "idx_issue_project_type_status"},
	}

	for _, idx := range oldIndexes {
		var count int64
		db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			idx.table, idx.name).Scan(&count)
		if count == 0 {
			continue // 索引不存在，跳过
		}
		if err := db.Exec("DROP INDEX " + idx.name + " ON " + idx.table).Error; err != nil {
			logger.Warn("failed to drop old index", zap.String("index", idx.name), zap.Error(err))
		}
	}

	indexes := []struct {
		table string
		name  string
		cols  string
	}{
		// === issues 表 ===
		// 列表查询核心索引：覆盖 WHERE 过滤 + ORDER BY id DESC，避免 filesort
		{"issues", "idx_issues_project_status_id", "project_id, status, id DESC"},
		{"issues", "idx_issues_project_priority_id", "project_id, priority, id DESC"},
		{"issues", "idx_issues_project_type_id", "project_id, issue_type_id, id DESC"},
		{"issues", "idx_issues_assignee_status_id", "assignee_id, status, id DESC"},
		{"issues", "idx_issues_reporter_id_desc", "reporter_id, id DESC"},
		// 与 reporter 同形状：(assignee_id, status, id) 在不带 status 过滤时无法用于排序，
		// 「我的工单」这类只按指派人筛选的查询需要这个索引才能免掉 filesort
		{"issues", "idx_issues_assignee_id_desc", "assignee_id, id DESC"},
		{"issues", "idx_issues_epic_status_id", "epic_id, status, id DESC"},
		// "我的工单"场景：项目 + 指派人
		{"issues", "idx_issues_project_assignee_id", "project_id, assignee_id, id DESC"},

		// === alerts 表 ===
		{"alerts", "idx_alerts_status_starts", "status, starts_at DESC"},
		{"alerts", "idx_alerts_source_status", "source, status"},

		// === notifications 表 ===
		{"notifications", "idx_notifications_user_unread", "user_id, is_read, created_at DESC"},

		// === activity_logs 表 ===
		{"activity_logs", "idx_activity_entity", "entity_type, entity_id, created_at DESC"},
	}

	// 唯一索引（使用 COALESCE 处理 NULL 值：MySQL UNIQUE 无法约束 NULL != NULL）
	uniqueIndexes := []struct {
		table string
		name  string
		expr  string
	}{
		{"issue_types", "uk_issue_type_name_project", "name, (COALESCE(project_id, 0))"},
		{"workflows", "uk_workflow_name_project", "name, (COALESCE(project_id, 0))"},
	}

	for _, idx := range indexes {
		// MySQL 8.4 不支持 CREATE INDEX IF NOT EXISTS，先检查索引是否存在
		var count int64
		db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			idx.table, idx.name).Scan(&count)
		if count > 0 {
			continue // 索引已存在，跳过
		}

		sql := "CREATE INDEX " + idx.name + " ON " + idx.table + " (" + idx.cols + ")"
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn("failed to create composite index",
				zap.String("index", idx.name),
				zap.Error(err),
			)
		}
	}

	// 创建唯一索引
	for _, idx := range uniqueIndexes {
		var count int64
		db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
			idx.table, idx.name).Scan(&count)
		if count > 0 {
			continue
		}

		sql := "CREATE UNIQUE INDEX " + idx.name + " ON " + idx.table + " (" + idx.expr + ")"
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn("failed to create unique index",
				zap.String("index", idx.name),
				zap.Error(err),
			)
		}
	}

	logger.Info("composite indexes created/verified")
}

// migrateRequirementStatus 迁移需求表中旧的状态值
func migrateRequirementStatus(db *gorm.DB) error {
	statusMapping := map[string]string{
		"pending":   "pending_review",
		"reviewing": "pending_review",
		"accepted":  "planning",
		"converted": "in_progress",
	}

	for oldStatus, newStatus := range statusMapping {
		result := db.Model(&Requirement{}).
			Where("status = ?", oldStatus).
			Update("status", newStatus)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			logger.Info("migrated requirement status",
				zap.String("from", oldStatus),
				zap.String("to", newStatus),
				zap.Int64("count", result.RowsAffected),
			)
		}
	}

	return nil
}

// migrateMemberRoles 迁移项目成员旧角色值（admin/member）到项目角色 key
func migrateMemberRoles(db *gorm.DB) error {
	roleMapping := map[string]string{
		"admin":  "administrators",
		"member": "viewers",
	}

	for oldRole, newRole := range roleMapping {
		result := db.Model(&ProjectMember{}).
			Where("role = ?", oldRole).
			Update("role", newRole)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			logger.Info("migrated member role",
				zap.String("from", oldRole),
				zap.String("to", newRole),
				zap.Int64("count", result.RowsAffected),
			)
		}
	}

	return nil
}

// SeedData 初始化种子数据
func SeedData(db *gorm.DB) error {
	logger.Info("seeding initial data...")

	// 初始化默认角色
	roles := []Role{
		{Name: "admin", DisplayName: "管理员", Description: "系统管理员，拥有所有权限"},
		{Name: "user", DisplayName: "普通用户", Description: "普通用户，基本操作权限"},
		{Name: "project_admin", DisplayName: "项目管理员", Description: "项目管理员，管理项目相关配置"},
	}

	for _, role := range roles {
		result := db.Where("name = ?", role.Name).FirstOrCreate(&role)
		if result.Error != nil {
			logger.Error("failed to seed role", zap.String("name", role.Name), zap.Error(result.Error))
			return result.Error
		}
	}

	// 去重全局工单类型和工作流（一次性迁移，在初始化工单类型前执行）
	if err := deduplicateGlobalRecords(db); err != nil {
		logger.Error("failed to deduplicate global records", zap.Error(err))
		return err
	}

	// 初始化默认工单类型
	issueTypes := []IssueType{
		{Name: "Epic", DisplayName: "Epic", Description: "阶段性目标/大型需求", Icon: "lightning", Color: "#6554C0"},
		{Name: "Task", DisplayName: "任务", Description: "普通任务", Icon: "check-square", Color: "#4FADE6"},
		{Name: "Subtask", DisplayName: "子任务", Description: "子任务", Icon: "minus-square", Color: "#4FADE6"},
		{Name: "Bug", DisplayName: "缺陷", Description: "研发缺陷", Icon: "bug", Color: "#E5493A"},
		{Name: "Fault", DisplayName: "故障", Description: "生产故障/告警工单", Icon: "alert-triangle", Color: "#FF5630"},
		{Name: "Change", DisplayName: "变更", Description: "变更工单", Icon: "git-branch", Color: "#36B37E"},
		{Name: "ServiceRequest", DisplayName: "服务请求", Description: "服务申请", Icon: "help-circle", Color: "#00B8D9"},
		{Name: "Alert", DisplayName: "告警", Description: "监控告警自动建单", Icon: "bell", Color: "#FF5630"},
	}

	for _, issueType := range issueTypes {
		result := db.Where("name = ? AND project_id IS NULL", issueType.Name).FirstOrCreate(&issueType)
		if result.Error != nil {
			logger.Error("failed to seed issue type", zap.String("name", issueType.Name), zap.Error(result.Error))
			return result.Error
		}
	}

	// 判定初始化状态
	//
	// 这里不再预置 admin/admin123 —— 那是个人尽皆知的默认口令，
	// 部署完不立刻改就等于敞开。管理员改由首次启动的初始化向导当场创建。
	if err := seedSetupState(db); err != nil {
		return err
	}

	// 初始化告警机器人用户
	var botUser User
	botResult := db.Where("username = ?", "alert-bot").First(&botUser)
	if botResult.Error != nil {
		if botResult.Error == gorm.ErrRecordNotFound {
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte("alert-bot-no-login"), bcrypt.DefaultCost)
			if err != nil {
				logger.Error("failed to hash bot password", zap.Error(err))
				return err
			}

			botUser = User{
				Username:     "alert-bot",
				Email:        "alert-bot@ticketdesk.local",
				PasswordHash: string(hashedPassword),
				DisplayName:  "告警机器人",
				Status:       1,
			}

			if err := db.Create(&botUser).Error; err != nil {
				logger.Error("failed to create alert bot user", zap.Error(err))
				return err
			}

			logger.Info("alert bot user created", zap.Uint64("user_id", botUser.ID))
		}
	}

	// 初始化默认系统配置
	defaultConfigs := []SystemConfig{
		{
			ConfigKey:   "general.site_url",
			ConfigValue: "http://localhost:5173",
			ConfigType:  "string",
			Category:    "general",
			Description: "站点域名（用于生成邮件中的链接）",
			IsSecret:    false,
		},
		{
			ConfigKey:   "general.language",
			ConfigValue: "zh-CN",
			ConfigType:  "string",
			Category:    "general",
			Description: "平台默认语言（zh-CN / en-US）",
			IsSecret:    false,
		},
		{
			ConfigKey:   "worklog.work_types",
			ConfigValue: `[{"value":"开发","label":"开发"},{"value":"测试","label":"测试"},{"value":"调试","label":"调试"},{"value":"文档","label":"文档"},{"value":"故障排查","label":"故障排查"},{"value":"监控运维","label":"监控运维"},{"value":"部署发布","label":"部署发布"},{"value":"配置变更","label":"配置变更"},{"value":"巡检","label":"巡检"},{"value":"安全响应","label":"安全响应"},{"value":"其他","label":"其他"}]`,
			ConfigType:  "json",
			Category:    "worklog",
			Description: "工时记录的工作类型选项列表",
			IsSecret:    false,
		},
	}

	for _, config := range defaultConfigs {
		result := db.Where("config_key = ?", config.ConfigKey).FirstOrCreate(&config)
		if result.Error != nil {
			logger.Error("failed to seed system config", zap.String("key", config.ConfigKey), zap.Error(result.Error))
			return result.Error
		}
	}

	// 初始化系统字段
	if err := SeedSystemFields(db); err != nil {
		return err
	}

	// 初始化默认告警静默规则模板
	// createdBy 传 0 表示系统创建 —— 此时还没有管理员（由初始化向导创建），
	// 而 0 在本项目里本就是「系统」的既定表示（见前端 operator_id === 0 的处理）
	if err := seedDefaultAlertSilences(db, 0); err != nil {
		return err
	}

	// 初始化默认工作流
	if err := SeedDefaultWorkflows(db); err != nil {
		return err
	}

	// 一次性迁移：为现有全局工作流添加审批节点（通过 system_configs 标记只执行一次）
	if err := MigrateWorkflowApprovalNodes(db); err != nil {
		logger.Warn("failed to migrate workflow approval nodes", zap.Error(err))
	}

	// 为已有项目初始化工作流方案（幂等）
	if err := seedExistingProjectWorkflowSchemes(db); err != nil {
		logger.Warn("failed to seed workflow schemes for existing projects", zap.Error(err))
	}

	// 一次性迁移：为已有项目补充 Alert 工作流方案并修复无工作流实例的告警工单
	if err := MigrateAlertWorkflowSchemes(db); err != nil {
		logger.Warn("failed to migrate alert workflow schemes", zap.Error(err))
	}

	// 一次性迁移：为现有工作流的审批节点添加拒绝分支（已取消节点 + rejected 边）
	if err := MigrateApprovalRejectBranch(db); err != nil {
		logger.Warn("failed to migrate approval reject branches", zap.Error(err))
	}

	// 一次性迁移：为告警工作流添加"待确认"节点和条件边
	if err := MigrateAlertWorkflowReviewNode(db); err != nil {
		logger.Warn("failed to migrate alert workflow review node", zap.Error(err))
	}

	// 为已有项目初始化角色权限（幂等）
	if err := seedExistingProjectRolePermissions(db); err != nil {
		logger.Warn("failed to seed role permissions for existing projects", zap.Error(err))
	}

	// 初始化默认字段方案模板
	if err := SeedDefaultFieldSchemeTemplates(db, 0); err != nil {
		logger.Warn("failed to seed default field scheme templates", zap.Error(err))
	}

	// 一次性迁移：为已有项目补充内置字段到字段方案
	if err := MigrateBuiltinFieldSchemes(db); err != nil {
		logger.Warn("failed to migrate builtin field schemes", zap.Error(err))
	}

	if err := MigratePlannedEndRequired(db); err != nil {
		logger.Error("failed to migrate planned end required", zap.Error(err))
	}

	// 一次性迁移：修复 Alert 类型 priority 字段可见性
	if err := MigrateAlertPriorityVisibility(db); err != nil {
		logger.Warn("failed to migrate alert priority visibility", zap.Error(err))
	}

	// 初始化默认需求分类
	if err := seedRequirementCategories(db); err != nil {
		logger.Warn("failed to seed requirement categories", zap.Error(err))
	}

	logger.Info("seed data completed")
	return nil
}

// seedRequirementCategories 初始化默认需求分类
func seedRequirementCategories(db *gorm.DB) error {
	categories := []RequirementCategoryDef{
		{Name: "feature", Label: "功能需求", Color: "primary", SortOrder: 1, IsDefault: true, IsSystem: true},
		{Name: "optimization", Label: "优化改进", Color: "success", SortOrder: 2, IsSystem: true},
		{Name: "bugfix", Label: "故障修复", Color: "danger", SortOrder: 3, IsSystem: true},
		{Name: "security", Label: "安全合规", Color: "warning", SortOrder: 4, IsSystem: true},
		{Name: "infrastructure", Label: "基础设施", Color: "info", SortOrder: 5, IsSystem: true},
		{Name: "other", Label: "其他", Color: "info", SortOrder: 6, IsSystem: true},
	}

	for _, cat := range categories {
		result := db.Where("name = ?", cat.Name).FirstOrCreate(&cat)
		if result.Error != nil {
			return fmt.Errorf("failed to seed requirement category %s: %w", cat.Name, result.Error)
		}
	}

	return nil
}

// MigrateBuiltinFieldSchemes 为已有项目的字段方案补充内置字段（一次性迁移）
func MigrateBuiltinFieldSchemes(db *gorm.DB) error {
	const migrationKey = "migration.builtin_field_schemes_v2"

	// 检查是否已执行
	var config SystemConfig
	if err := db.Where("config_key = ?", migrationKey).First(&config).Error; err == nil {
		return nil // 已执行过
	}

	logger.Info("migrating builtin field schemes for existing projects...")

	// 获取内置字段 key→ID 映射
	builtinKeys := []string{"description", "priority", "assignee", "planned_start_date", "planned_end_date"}
	var systemFields []FieldDefinition
	if err := db.Where("project_id IS NULL AND is_system = ? AND field_key IN ?", true, builtinKeys).Find(&systemFields).Error; err != nil {
		return err
	}
	if len(systemFields) == 0 {
		return nil // 内置字段还未创建
	}
	fieldMap := make(map[string]uint64)
	for _, f := range systemFields {
		fieldMap[f.FieldKey] = f.ID
	}

	// 内置字段默认配置
	type builtinCfg struct {
		FieldKey        string
		IsRequired      bool
		SortOrder       int
		IsVisibleCreate bool
		IsVisibleEdit   bool
		IsVisibleDetail bool
	}
	builtinDefaults := []builtinCfg{
		{FieldKey: "description", IsRequired: false, SortOrder: -5, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "priority", IsRequired: true, SortOrder: -4, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "assignee", IsRequired: false, SortOrder: -3, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "planned_start_date", IsRequired: false, SortOrder: -2, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		// 「开单必须评估什么时候交付」是团队的工单制度，落到系统就是这一条必填。
		// 告警类型除外（见 builtinFieldsAlert）—— 机器开的单评估不了排期。
		{FieldKey: "planned_end_date", IsRequired: true, SortOrder: -1, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
	}

	// 查询所有项目+工单类型组合（含已有字段方案的组合 + 所有项目×全局工单类型）
	type projectType struct {
		ProjectID   uint64
		IssueTypeID uint64
	}
	// 先从已有字段方案取组合
	var existingCombos []projectType
	if err := db.Model(&IssueTypeFieldScheme{}).
		Select("DISTINCT project_id, issue_type_id").
		Scan(&existingCombos).Error; err != nil {
		return err
	}
	// 再从所有项目×全局工单类型取组合（覆盖之前没有任何字段方案的工单类型）
	var projectIDs []uint64
	db.Model(&Project{}).Pluck("id", &projectIDs)
	var globalTypeIDs []uint64
	db.Model(&IssueType{}).Where("project_id IS NULL").Pluck("id", &globalTypeIDs)

	comboSet := make(map[[2]uint64]bool)
	var combos []projectType
	for _, c := range existingCombos {
		key := [2]uint64{c.ProjectID, c.IssueTypeID}
		if !comboSet[key] {
			comboSet[key] = true
			combos = append(combos, c)
		}
	}
	for _, pid := range projectIDs {
		for _, tid := range globalTypeIDs {
			key := [2]uint64{pid, tid}
			if !comboSet[key] {
				comboSet[key] = true
				combos = append(combos, projectType{ProjectID: pid, IssueTypeID: tid})
			}
		}
	}

	count := 0
	for _, combo := range combos {
		for _, bc := range builtinDefaults {
			fieldID, ok := fieldMap[bc.FieldKey]
			if !ok {
				continue
			}
			scheme := IssueTypeFieldScheme{
				ProjectID:       combo.ProjectID,
				IssueTypeID:     combo.IssueTypeID,
				FieldID:         fieldID,
				IsRequired:      bc.IsRequired,
				IsVisibleCreate: bc.IsVisibleCreate,
				IsVisibleEdit:   bc.IsVisibleEdit,
				IsVisibleDetail: bc.IsVisibleDetail,
				SortOrder:       bc.SortOrder,
			}
			result := db.Where("project_id = ? AND issue_type_id = ? AND field_id = ?",
				combo.ProjectID, combo.IssueTypeID, fieldID).FirstOrCreate(&scheme)
			if result.Error != nil {
				logger.Warn("failed to migrate builtin field",
					zap.Uint64("project_id", combo.ProjectID),
					zap.String("field_key", bc.FieldKey),
					zap.Error(result.Error))
			}
			if result.RowsAffected > 0 {
				count++
			}
		}
	}

	// 标记已完成
	db.Create(&SystemConfig{
		ConfigKey:   migrationKey,
		ConfigValue: "done",
		ConfigType:  "string",
		Category:    "migration",
		Description: "内置字段方案迁移标记",
	})

	logger.Info("builtin field schemes migration completed", zap.Int("fields_added", count))
	return nil
}

// MigrateAlertPriorityVisibility 修复 Alert 类型 priority 字段 is_visible_create 应为 false
func MigrateAlertPriorityVisibility(db *gorm.DB) error {
	const migrationKey = "migration.alert_priority_visibility_v1"

	var config SystemConfig
	if err := db.Where("config_key = ?", migrationKey).First(&config).Error; err == nil {
		return nil
	}

	// 查找 Alert 工单类型 ID
	var alertType IssueType
	if err := db.Where("name = ? AND project_id IS NULL", "Alert").First(&alertType).Error; err != nil {
		return nil // Alert 类型不存在，跳过
	}

	// 查找 priority 系统字段 ID
	var priorityField FieldDefinition
	if err := db.Where("field_key = ? AND project_id IS NULL AND is_system = ?", "priority", true).First(&priorityField).Error; err != nil {
		return nil // priority 字段不存在，跳过
	}

	// 更新所有项目中 Alert 类型的 priority 字段 is_visible_create = false
	result := db.Model(&IssueTypeFieldScheme{}).
		Where("issue_type_id = ? AND field_id = ?", alertType.ID, priorityField.ID).
		Update("is_visible_create", false)
	if result.Error != nil {
		return result.Error
	}

	// 标记已完成
	db.Create(&SystemConfig{
		ConfigKey:   migrationKey,
		ConfigValue: "done",
		ConfigType:  "string",
		Category:    "migration",
		Description: "Alert 类型 priority 可见性修复",
	})

	logger.Info("alert priority visibility migration completed", zap.Int64("rows_affected", result.RowsAffected))
	return nil
}

// MigratePlannedEndRequired 把存量项目的「预计交付时间」改成必填（一次性迁移）。
//
// 团队的工单制度是「开单必须评估什么时候开始、什么时候交付」，但字段方案里
// 这一条一直是选填，制度只能靠人自觉。默认值已经改成必填，可那只对新建的
// 方案生效（初始化走的是 FirstOrCreate，不会回头改已有行），所以存量要单独翻一次。
//
// 告警类型除外：那类单是告警规则自动开的，机器评估不了排期。
func MigratePlannedEndRequired(db *gorm.DB) error {
	const migrationKey = "migration.planned_end_required_v1"

	var config SystemConfig
	if err := db.Where("config_key = ?", migrationKey).First(&config).Error; err == nil {
		return nil
	}

	var plannedEnd FieldDefinition
	if err := db.Where("field_key = ? AND project_id IS NULL AND is_system = ?",
		"planned_end_date", true).First(&plannedEnd).Error; err != nil {
		return nil // 字段不存在，跳过
	}

	query := db.Model(&IssueTypeFieldScheme{}).Where("field_id = ?", plannedEnd.ID)

	// 排除告警类型
	var alertType IssueType
	if err := db.Where("name = ? AND project_id IS NULL", "Alert").First(&alertType).Error; err == nil {
		query = query.Where("issue_type_id <> ?", alertType.ID)
	}

	result := query.Updates(map[string]any{
		"is_required":       true,
		"is_visible_create": true,
	})
	if result.Error != nil {
		return result.Error
	}

	db.Create(&SystemConfig{
		ConfigKey:   migrationKey,
		ConfigValue: "done",
		ConfigType:  "string",
		Category:    "migration",
		Description: "预计交付时间改为必填",
	})

	logger.Info("planned end date required migration completed",
		zap.Int64("rows_affected", result.RowsAffected))
	return nil
}

// SeedDefaultFieldSchemeTemplates 初始化默认字段方案模板（从 typeFieldConfig 转化）
func SeedDefaultFieldSchemeTemplates(db *gorm.DB, createdBy uint64) error {
	logger.Info("seeding default field scheme templates...")

	// 获取系统字段 key→ID 映射
	var systemFields []FieldDefinition
	if err := db.Where("project_id IS NULL AND is_system = ?", true).Find(&systemFields).Error; err != nil {
		return err
	}
	fieldMap := make(map[string]uint64)
	for _, f := range systemFields {
		fieldMap[f.FieldKey] = f.ID
	}

	// 定义模板（与 InitProjectFieldSchemes 中的 typeFieldConfig 一致）
	type fieldCfg struct {
		FieldKey   string
		IsRequired bool
		SortOrder  int
	}

	// 内置字段（所有模板共享前缀）
	builtinTplFields := []fieldCfg{
		{FieldKey: "description", IsRequired: false, SortOrder: -5},
		{FieldKey: "priority", IsRequired: true, SortOrder: -4},
		{FieldKey: "assignee", IsRequired: false, SortOrder: -3},
		{FieldKey: "planned_start_date", IsRequired: false, SortOrder: -2},
		{FieldKey: "planned_end_date", IsRequired: true, SortOrder: -1},
	}

	templates := []struct {
		Name        string
		Description string
		Fields      []fieldCfg
	}{
		{
			Name:        "缺陷(Bug)默认方案",
			Description: "适用于缺陷(Bug)工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "severity", IsRequired: true, SortOrder: 1},
				{FieldKey: "environment", IsRequired: false, SortOrder: 2},
				{FieldKey: "steps_to_reproduce", IsRequired: false, SortOrder: 3},
				{FieldKey: "affects_version", IsRequired: false, SortOrder: 4},
				{FieldKey: "fix_version", IsRequired: false, SortOrder: 5},
				{FieldKey: "epic_link", IsRequired: false, SortOrder: 6},
				{FieldKey: "labels", IsRequired: false, SortOrder: 7},
				{FieldKey: "components", IsRequired: false, SortOrder: 8},
			}...),
		},
		{
			Name:        "Epic默认方案",
			Description: "适用于Epic工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "epic_name", IsRequired: true, SortOrder: 1},
				{FieldKey: "epic_color", IsRequired: false, SortOrder: 2},
				{FieldKey: "story_points", IsRequired: false, SortOrder: 3},
				{FieldKey: "labels", IsRequired: false, SortOrder: 4},
				{FieldKey: "components", IsRequired: false, SortOrder: 5},
			}...),
		},
		{
			Name:        "任务(Task)默认方案",
			Description: "适用于任务(Task)工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "epic_link", IsRequired: false, SortOrder: 1},
				{FieldKey: "story_points", IsRequired: false, SortOrder: 2},
				{FieldKey: "fix_version", IsRequired: false, SortOrder: 3},
				{FieldKey: "labels", IsRequired: false, SortOrder: 4},
				{FieldKey: "components", IsRequired: false, SortOrder: 5},
			}...),
		},
		{
			Name:        "故障(Fault)默认方案",
			Description: "适用于故障(Fault)工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "severity", IsRequired: true, SortOrder: 1},
				{FieldKey: "environment", IsRequired: false, SortOrder: 2},
				{FieldKey: "affects_version", IsRequired: false, SortOrder: 3},
				{FieldKey: "labels", IsRequired: false, SortOrder: 4},
				{FieldKey: "components", IsRequired: false, SortOrder: 5},
			}...),
		},
		{
			Name:        "变更(Change)默认方案",
			Description: "适用于变更(Change)工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "fix_version", IsRequired: false, SortOrder: 1},
				{FieldKey: "labels", IsRequired: false, SortOrder: 2},
				{FieldKey: "components", IsRequired: false, SortOrder: 3},
			}...),
		},
		{
			Name:        "服务请求(ServiceRequest)默认方案",
			Description: "适用于服务请求(ServiceRequest)工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "labels", IsRequired: false, SortOrder: 1},
				{FieldKey: "components", IsRequired: false, SortOrder: 2},
			}...),
		},
		{
			Name:        "子任务(Subtask)默认方案",
			Description: "适用于子任务(Subtask)工单类型的字段方案模板",
			Fields: append(append([]fieldCfg{}, builtinTplFields...), []fieldCfg{
				{FieldKey: "labels", IsRequired: false, SortOrder: 1},
			}...),
		},
		{
			Name:        "告警(Alert)默认方案",
			Description: "适用于告警(Alert)工单类型的字段方案模板",
			Fields:      append([]fieldCfg{}, builtinTplFields...),
		},
	}

	for _, tplDef := range templates {
		// 幂等：按名称检查
		var existing FieldSchemeTemplate
		result := db.Where("name = ?", tplDef.Name).First(&existing)
		if result.Error == nil {
			continue // 已存在，跳过
		}

		tpl := FieldSchemeTemplate{
			Name:        tplDef.Name,
			Description: tplDef.Description,
			CreatedBy:   createdBy,
			IsActive:    true,
		}
		if err := db.Create(&tpl).Error; err != nil {
			logger.Error("failed to create template", zap.String("name", tplDef.Name), zap.Error(err))
			return err
		}

		items := make([]FieldSchemeTemplateItem, 0, len(tplDef.Fields))
		for _, fc := range tplDef.Fields {
			fieldID, ok := fieldMap[fc.FieldKey]
			if !ok {
				continue
			}
			items = append(items, FieldSchemeTemplateItem{
				TemplateID:      tpl.ID,
				FieldID:         fieldID,
				IsRequired:      fc.IsRequired,
				IsVisibleCreate: true,
				IsVisibleEdit:   true,
				IsVisibleDetail: true,
				SortOrder:       fc.SortOrder,
			})
		}
		if len(items) > 0 {
			if err := db.Create(&items).Error; err != nil {
				logger.Error("failed to create template items", zap.String("name", tplDef.Name), zap.Error(err))
				return err
			}
		}
	}

	logger.Info("default field scheme templates seeded")
	return nil
}

// InitProjectRoles 初始化项目的预置角色
func InitProjectRoles(db *gorm.DB, projectID uint64) error {
	logger.Info("initializing project roles", zap.Uint64("project_id", projectID))

	// 系统预置角色
	presetRoles := []ProjectRole{
		{
			ProjectID:   projectID,
			RoleKey:     "administrators",
			RoleName:    "管理员",
			Description: "项目管理员，拥有项目所有权限",
			IsSystem:    true,
			SortOrder:   1,
		},
		{
			ProjectID:   projectID,
			RoleKey:     "developers",
			RoleName:    "开发人员",
			Description: "项目开发人员",
			IsSystem:    true,
			SortOrder:   2,
		},
		{
			ProjectID:   projectID,
			RoleKey:     "testers",
			RoleName:    "测试人员",
			Description: "项目测试人员",
			IsSystem:    true,
			SortOrder:   3,
		},
		{
			ProjectID:   projectID,
			RoleKey:     "viewers",
			RoleName:    "只读用户",
			Description: "只能查看项目信息，不能修改",
			IsSystem:    true,
			SortOrder:   4,
		},
	}

	for _, role := range presetRoles {
		result := db.Where("project_id = ? AND role_key = ?", projectID, role.RoleKey).FirstOrCreate(&role)
		if result.Error != nil {
			logger.Error("failed to init project role",
				zap.Uint64("project_id", projectID),
				zap.String("role_key", role.RoleKey),
				zap.Error(result.Error))
			return result.Error
		}
	}

	logger.Info("project roles initialized", zap.Uint64("project_id", projectID))

	// 初始化角色默认权限
	if err := InitProjectRolePermissions(db, projectID); err != nil {
		logger.Warn("failed to init project role permissions", zap.Uint64("project_id", projectID), zap.Error(err))
	}

	return nil
}

// 系统角色默认权限定义
var defaultRolePermissions = map[string][]string{
	"administrators": {
		"project:view", "project:manage",
		"issue:view", "issue:create", "issue:edit", "issue:delete", "issue:assign",
		"member:view", "member:manage",
		"role:view", "role:manage",
		"workflow:view", "workflow:manage",
		"alert:view", "alert:manage",
	},
	"developers": {
		"project:view",
		"issue:view", "issue:create", "issue:edit", "issue:assign",
		"member:view",
		"role:view",
		"workflow:view",
		"alert:view",
	},
	"testers": {
		"project:view",
		"issue:view", "issue:create", "issue:edit",
		"member:view",
		"role:view",
		"workflow:view",
		"alert:view",
	},
	"viewers": {
		"project:view",
		"issue:view",
		"member:view",
		"role:view",
		"alert:view",
	},
}

// InitProjectRolePermissions 初始化项目角色的默认权限
func InitProjectRolePermissions(db *gorm.DB, projectID uint64) error {
	logger.Info("initializing project role permissions", zap.Uint64("project_id", projectID))

	for roleKey, permissions := range defaultRolePermissions {
		var role ProjectRole
		if err := db.Where("project_id = ? AND role_key = ?", projectID, roleKey).First(&role).Error; err != nil {
			continue // 角色不存在，跳过
		}

		for _, permKey := range permissions {
			perm := ProjectRolePermission{
				RoleID:        role.ID,
				PermissionKey: permKey,
			}
			db.Where("role_id = ? AND permission_key = ?", role.ID, permKey).FirstOrCreate(&perm)
		}
	}

	logger.Info("project role permissions initialized", zap.Uint64("project_id", projectID))
	return nil
}

// SeedSystemFields 初始化系统字段定义
func SeedSystemFields(db *gorm.DB) error {
	logger.Info("seeding system fields...")

	systemFields := []FieldDefinition{
		// Bug 特有字段
		{
			FieldKey:    "severity",
			FieldName:   "严重程度",
			FieldType:   FieldTypeSelect,
			Description: "缺陷的严重程度",
			IsSystem:    true,
			IsActive:    true,
			Options:     `[{"value":"blocker","label":"阻塞"},{"value":"critical","label":"严重"},{"value":"major","label":"主要"},{"value":"minor","label":"次要"},{"value":"trivial","label":"轻微"}]`,
			SortOrder:   1,
		},
		{
			FieldKey:    "environment",
			FieldName:   "环境",
			FieldType:   FieldTypeTextarea,
			Description: "问题发生的环境信息",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   2,
		},
		{
			FieldKey:    "steps_to_reproduce",
			FieldName:   "复现步骤",
			FieldType:   FieldTypeTextarea,
			Description: "问题复现的详细步骤",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   3,
		},
		// 版本相关字段
		{
			FieldKey:    "affects_version",
			FieldName:   "受影响版本",
			FieldType:   FieldTypeVersion,
			Description: "受此问题影响的版本",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   4,
		},
		{
			FieldKey:    "fix_version",
			FieldName:   "修复版本",
			FieldType:   FieldTypeVersion,
			Description: "修复此问题的目标版本",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   5,
		},
		// Epic 特有字段
		{
			FieldKey:    "epic_name",
			FieldName:   "Epic名称",
			FieldType:   FieldTypeText,
			Description: "Epic的简短名称（用于看板显示）",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   6,
		},
		{
			FieldKey:    "epic_color",
			FieldName:   "Epic颜色",
			FieldType:   FieldTypeSelect,
			Description: "Epic的标识颜色",
			IsSystem:    true,
			IsActive:    true,
			Options:     `[{"value":"#6554C0","label":"紫色"},{"value":"#4FADE6","label":"蓝色"},{"value":"#36B37E","label":"绿色"},{"value":"#FFAB00","label":"黄色"},{"value":"#FF5630","label":"红色"},{"value":"#00B8D9","label":"青色"}]`,
			SortOrder:   7,
		},
		// 日期字段
		{
			FieldKey:    "start_date",
			FieldName:   "开始日期",
			FieldType:   FieldTypeDate,
			Description: "计划开始日期",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   8,
		},
		{
			FieldKey:    "end_date",
			FieldName:   "结束日期",
			FieldType:   FieldTypeDate,
			Description: "计划结束日期",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   9,
		},
		// 时间估算字段
		{
			FieldKey:    "original_estimate",
			FieldName:   "预估时间",
			FieldType:   FieldTypeTimeEstimate,
			Description: "原始预估工作时间",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   10,
		},
		{
			FieldKey:    "remaining_estimate",
			FieldName:   "剩余时间",
			FieldType:   FieldTypeTimeEstimate,
			Description: "剩余预估工作时间",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   11,
		},
		// Epic链接字段
		{
			FieldKey:    "epic_link",
			FieldName:   "Epic链接",
			FieldType:   FieldTypeEpicLink,
			Description: "关联的Epic",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   12,
		},
		// 故事点
		{
			FieldKey:    "story_points",
			FieldName:   "故事点",
			FieldType:   FieldTypeNumber,
			Description: "任务复杂度估算",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   13,
		},
		// 内置字段（对应 Issue 表列，通过字段方案控制可见性，不存 EAV）
		{
			FieldKey:    "description",
			FieldName:   "描述",
			FieldType:   FieldTypeTextarea,
			Description: "工单描述",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   100,
		},
		{
			FieldKey:     "priority",
			FieldName:    "优先级",
			FieldType:    FieldTypeSelect,
			Description:  "工单优先级",
			IsSystem:     true,
			IsActive:     true,
			Options:      `[{"value":"P0","label":"P0 - 紧急"},{"value":"P1","label":"P1 - 高"},{"value":"P2","label":"P2 - 中"},{"value":"P3","label":"P3 - 低"}]`,
			DefaultValue: "P2",
			SortOrder:    101,
		},
		{
			FieldKey:    "assignee",
			FieldName:   "指派人",
			FieldType:   FieldTypeUser,
			Description: "工单指派人",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   102,
		},
		{
			FieldKey:    "planned_start_date",
			FieldName:   "预计开始时间",
			FieldType:   FieldTypeDate,
			Description: "计划开始日期",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   103,
		},
		{
			FieldKey:    "planned_end_date",
			FieldName:   "预计交付时间",
			FieldType:   FieldTypeDate,
			Description: "计划结束日期",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   104,
		},
		// 通用字段
		{
			FieldKey:    "labels",
			FieldName:   "标签",
			FieldType:   FieldTypeLabel,
			Description: "工单标签",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   14,
		},
		{
			FieldKey:    "components",
			FieldName:   "组件",
			FieldType:   FieldTypeComponent,
			Description: "关联的项目组件",
			IsSystem:    true,
			IsActive:    true,
			SortOrder:   15,
		},
	}

	for _, field := range systemFields {
		// MySQL JSON 类型不接受空字符串，需要设置为有效 JSON
		if field.Options == "" {
			field.Options = "{}"
		}
		if field.Validation == "" {
			field.Validation = "{}"
		}
		result := db.Where("field_key = ? AND project_id IS NULL", field.FieldKey).FirstOrCreate(&field)
		if result.Error != nil {
			logger.Error("failed to seed system field", zap.String("field_key", field.FieldKey), zap.Error(result.Error))
			return result.Error
		}
	}

	logger.Info("system fields seeded")
	return nil
}

// InitProjectFieldSchemes 初始化项目的字段方案（为每个工单类型配置默认字段）
func InitProjectFieldSchemes(db *gorm.DB, projectID uint64) error {
	logger.Info("initializing project field schemes", zap.Uint64("project_id", projectID))

	// 获取系统字段
	var systemFields []FieldDefinition
	if err := db.Where("project_id IS NULL AND is_system = ?", true).Find(&systemFields).Error; err != nil {
		logger.Error("failed to get system fields", zap.Error(err))
		return err
	}

	// 创建字段key到ID的映射
	fieldMap := make(map[string]uint64)
	for _, field := range systemFields {
		fieldMap[field.FieldKey] = field.ID
	}

	// 获取全局工单类型
	var issueTypes []IssueType
	if err := db.Where("project_id IS NULL").Find(&issueTypes).Error; err != nil {
		logger.Error("failed to get issue types", zap.Error(err))
		return err
	}

	// 创建类型名到ID的映射
	typeMap := make(map[string]uint64)
	for _, it := range issueTypes {
		typeMap[it.Name] = it.ID
	}

	// 内置字段配置（所有工单类型共享，SortOrder 使用负数排在扩展字段前面）
	type fieldCfgItem struct {
		FieldKey        string
		IsRequired      bool
		SortOrder       int
		IsVisibleCreate bool
		IsVisibleEdit   bool
		IsVisibleDetail bool
	}
	builtinFields := []fieldCfgItem{
		{FieldKey: "description", IsRequired: false, SortOrder: -5, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "priority", IsRequired: true, SortOrder: -4, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "assignee", IsRequired: false, SortOrder: -3, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "planned_start_date", IsRequired: false, SortOrder: -2, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		// 「开单必须评估什么时候交付」是团队的工单制度，落到系统就是这一条必填。
		// 告警类型除外（见 builtinFieldsAlert）—— 机器开的单评估不了排期。
		{FieldKey: "planned_end_date", IsRequired: true, SortOrder: -1, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
	}
	// Alert 类型的内置字段（priority 创建时不可见，由规则决定）
	builtinFieldsAlert := []fieldCfgItem{
		{FieldKey: "description", IsRequired: false, SortOrder: -5, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "priority", IsRequired: true, SortOrder: -4, IsVisibleCreate: false, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "assignee", IsRequired: false, SortOrder: -3, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		{FieldKey: "planned_start_date", IsRequired: false, SortOrder: -2, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
		// 唯独告警类型的预计交付不必填：这类单是告警规则自动开的，
		// 机器没法评估排期。交付报表里它们单独归为「无排期」，不计入准时率。
		{FieldKey: "planned_end_date", IsRequired: false, SortOrder: -1, IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true},
	}

	// 定义每种工单类型的扩展字段配置
	type extFieldCfg struct {
		FieldKey   string
		IsRequired bool
		SortOrder  int
	}
	typeExtFieldConfig := map[string][]extFieldCfg{
		"Bug": {
			{FieldKey: "severity", IsRequired: true, SortOrder: 1},
			{FieldKey: "environment", IsRequired: false, SortOrder: 2},
			{FieldKey: "steps_to_reproduce", IsRequired: false, SortOrder: 3},
			{FieldKey: "affects_version", IsRequired: false, SortOrder: 4},
			{FieldKey: "fix_version", IsRequired: false, SortOrder: 5},
			{FieldKey: "epic_link", IsRequired: false, SortOrder: 6},
			{FieldKey: "labels", IsRequired: false, SortOrder: 7},
			{FieldKey: "components", IsRequired: false, SortOrder: 8},
		},
		"Epic": {
			{FieldKey: "epic_name", IsRequired: true, SortOrder: 1},
			{FieldKey: "epic_color", IsRequired: false, SortOrder: 2},
			{FieldKey: "story_points", IsRequired: false, SortOrder: 3},
			{FieldKey: "labels", IsRequired: false, SortOrder: 4},
			{FieldKey: "components", IsRequired: false, SortOrder: 5},
		},
		"Task": {
			{FieldKey: "epic_link", IsRequired: false, SortOrder: 1},
			{FieldKey: "story_points", IsRequired: false, SortOrder: 2},
			{FieldKey: "fix_version", IsRequired: false, SortOrder: 3},
			{FieldKey: "labels", IsRequired: false, SortOrder: 4},
			{FieldKey: "components", IsRequired: false, SortOrder: 5},
		},
		"Fault": {
			{FieldKey: "severity", IsRequired: true, SortOrder: 1},
			{FieldKey: "environment", IsRequired: false, SortOrder: 2},
			{FieldKey: "affects_version", IsRequired: false, SortOrder: 3},
			{FieldKey: "labels", IsRequired: false, SortOrder: 4},
			{FieldKey: "components", IsRequired: false, SortOrder: 5},
		},
		"Change": {
			{FieldKey: "fix_version", IsRequired: false, SortOrder: 1},
			{FieldKey: "labels", IsRequired: false, SortOrder: 2},
			{FieldKey: "components", IsRequired: false, SortOrder: 3},
		},
		"ServiceRequest": {
			{FieldKey: "labels", IsRequired: false, SortOrder: 1},
			{FieldKey: "components", IsRequired: false, SortOrder: 2},
		},
		"Subtask": {
			{FieldKey: "labels", IsRequired: false, SortOrder: 1},
		},
		"Alert": {},
	}

	// 合并内置字段和扩展字段，用旧类型结构兼容下方创建逻辑
	type fullFieldCfg struct {
		FieldKey        string
		IsRequired      bool
		SortOrder       int
		IsVisibleCreate bool
		IsVisibleEdit   bool
		IsVisibleDetail bool
	}
	typeFieldConfig := make(map[string][]fullFieldCfg)
	for typeName, extFields := range typeExtFieldConfig {
		var allFields []fullFieldCfg
		// 选择内置字段列表
		bi := builtinFields
		if typeName == "Alert" {
			bi = builtinFieldsAlert
		}
		for _, bf := range bi {
			allFields = append(allFields, fullFieldCfg(bf))
		}
		for _, ef := range extFields {
			allFields = append(allFields, fullFieldCfg{
				FieldKey: ef.FieldKey, IsRequired: ef.IsRequired, SortOrder: ef.SortOrder,
				IsVisibleCreate: true, IsVisibleEdit: true, IsVisibleDetail: true,
			})
		}
		typeFieldConfig[typeName] = allFields
	}

	// 为每个工单类型创建字段方案
	for typeName, fields := range typeFieldConfig {
		typeID, ok := typeMap[typeName]
		if !ok {
			continue
		}

		for _, fieldConfig := range fields {
			fieldID, ok := fieldMap[fieldConfig.FieldKey]
			if !ok {
				continue
			}

			scheme := IssueTypeFieldScheme{
				ProjectID:       projectID,
				IssueTypeID:     typeID,
				FieldID:         fieldID,
				IsRequired:      fieldConfig.IsRequired,
				IsVisibleCreate: fieldConfig.IsVisibleCreate,
				IsVisibleEdit:   fieldConfig.IsVisibleEdit,
				IsVisibleDetail: fieldConfig.IsVisibleDetail,
				SortOrder:       fieldConfig.SortOrder,
			}

			result := db.Where("project_id = ? AND issue_type_id = ? AND field_id = ?",
				projectID, typeID, fieldID).FirstOrCreate(&scheme)
			if result.Error != nil {
				logger.Error("failed to create field scheme",
					zap.Uint64("project_id", projectID),
					zap.String("type", typeName),
					zap.String("field", fieldConfig.FieldKey),
					zap.Error(result.Error))
				return result.Error
			}
		}
	}

	logger.Info("project field schemes initialized", zap.Uint64("project_id", projectID))
	return nil
}

// seedExistingProjectWorkflowSchemes 为已有项目初始化工作流方案
func seedExistingProjectWorkflowSchemes(db *gorm.DB) error {
	var projects []Project
	if err := db.Find(&projects).Error; err != nil {
		return err
	}

	for _, project := range projects {
		if err := InitProjectWorkflowSchemes(db, project.ID); err != nil {
			logger.Warn("failed to init workflow schemes for project",
				zap.Uint64("project_id", project.ID),
				zap.Error(err))
		}
	}
	return nil
}

// seedExistingProjectRolePermissions 为已有项目初始化角色权限
func seedExistingProjectRolePermissions(db *gorm.DB) error {
	var projects []Project
	if err := db.Find(&projects).Error; err != nil {
		return err
	}

	for _, project := range projects {
		if err := InitProjectRolePermissions(db, project.ID); err != nil {
			logger.Warn("failed to init role permissions for project",
				zap.Uint64("project_id", project.ID),
				zap.Error(err))
		}
	}
	return nil
}

// deduplicateGlobalRecords 去重全局工单类型和工作流（一次性迁移）
// 旧版软删除导致 FirstOrCreate 重复创建，移除软删除后重复记录全部可见
func deduplicateGlobalRecords(db *gorm.DB) error {
	const migrationKey = "migration.deduplicate_global_types_v1"

	var config SystemConfig
	if err := db.Where("config_key = ?", migrationKey).First(&config).Error; err == nil {
		return nil // 已执行过
	}

	logger.Info("deduplicating global issue_types and workflows...")

	return db.Transaction(func(tx *gorm.DB) error {
		// ==================== issue_types 去重 ====================
		type dupGroup struct {
			Name  string
			MinID uint64
		}
		var itDups []dupGroup
		if err := tx.Raw(`
			SELECT name, MIN(id) AS min_id
			FROM issue_types
			WHERE project_id IS NULL
			GROUP BY name
			HAVING COUNT(*) > 1
		`).Scan(&itDups).Error; err != nil {
			return err
		}

		for _, dup := range itDups {
			// 查出需要删除的重复 ID
			var dupIDs []uint64
			if err := tx.Raw("SELECT id FROM issue_types WHERE name = ? AND project_id IS NULL AND id != ?",
				dup.Name, dup.MinID).Scan(&dupIDs).Error; err != nil {
				return err
			}
			if len(dupIDs) == 0 {
				continue
			}

			keepID := dup.MinID

			// 迁移 issues.issue_type_id
			if err := tx.Exec("UPDATE issues SET issue_type_id = ? WHERE issue_type_id IN ?", keepID, dupIDs).Error; err != nil {
				return err
			}

			// 迁移 alert_rules.issue_type_id
			if err := tx.Exec("UPDATE alert_rules SET issue_type_id = ? WHERE issue_type_id IN ?", keepID, dupIDs).Error; err != nil {
				return err
			}

			// 迁移 workflow_schemes.issue_type_id（有 uk_project_issue_type 唯一索引，先删冲突行再更新）
			if err := tx.Exec(`
				DELETE FROM workflow_schemes
				WHERE issue_type_id IN ?
				  AND project_id IN (
				    SELECT project_id FROM (
				      SELECT project_id FROM workflow_schemes WHERE issue_type_id = ?
				    ) AS keep_rows
				  )
			`, dupIDs, keepID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE workflow_schemes SET issue_type_id = ? WHERE issue_type_id IN ?", keepID, dupIDs).Error; err != nil {
				return err
			}

			// 迁移 issue_type_field_schemes.issue_type_id（有 uk_type_field 唯一索引，先删冲突行再更新）
			if err := tx.Exec(`
				DELETE FROM issue_type_field_schemes
				WHERE issue_type_id IN ?
				  AND (project_id, field_id) IN (
				    SELECT project_id, field_id FROM (
				      SELECT project_id, field_id FROM issue_type_field_schemes WHERE issue_type_id = ?
				    ) AS keep_rows
				  )
			`, dupIDs, keepID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE issue_type_field_schemes SET issue_type_id = ? WHERE issue_type_id IN ?", keepID, dupIDs).Error; err != nil {
				return err
			}

			// 删除重复的 issue_types 记录
			if err := tx.Exec("DELETE FROM issue_types WHERE id IN ?", dupIDs).Error; err != nil {
				return err
			}

			logger.Info("deduplicated issue_type",
				zap.String("name", dup.Name),
				zap.Uint64("keep_id", keepID),
				zap.Int("removed", len(dupIDs)),
			)
		}

		// ==================== workflows 去重 ====================
		var wfDups []dupGroup
		if err := tx.Raw(`
			SELECT name, MIN(id) AS min_id
			FROM workflows
			WHERE project_id IS NULL
			GROUP BY name
			HAVING COUNT(*) > 1
		`).Scan(&wfDups).Error; err != nil {
			return err
		}

		for _, dup := range wfDups {
			var dupIDs []uint64
			if err := tx.Raw("SELECT id FROM workflows WHERE name = ? AND project_id IS NULL AND id != ?",
				dup.Name, dup.MinID).Scan(&dupIDs).Error; err != nil {
				return err
			}
			if len(dupIDs) == 0 {
				continue
			}

			keepID := dup.MinID

			// 删除重复工作流的节点和边
			if err := tx.Exec("DELETE FROM workflow_nodes WHERE workflow_id IN ?", dupIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM workflow_edges WHERE workflow_id IN ?", dupIDs).Error; err != nil {
				return err
			}

			// 迁移 workflow_schemes.workflow_id（有 uk_project_issue_type 唯一索引，先删冲突行再更新）
			if err := tx.Exec(`
				DELETE FROM workflow_schemes
				WHERE workflow_id IN ?
				  AND (project_id, issue_type_id) IN (
				    SELECT project_id, issue_type_id FROM (
				      SELECT project_id, issue_type_id FROM workflow_schemes WHERE workflow_id = ?
				    ) AS keep_rows
				  )
			`, dupIDs, keepID).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE workflow_schemes SET workflow_id = ? WHERE workflow_id IN ?", keepID, dupIDs).Error; err != nil {
				return err
			}

			// 迁移 workflow_instances.workflow_id
			if err := tx.Exec("UPDATE workflow_instances SET workflow_id = ? WHERE workflow_id IN ?", keepID, dupIDs).Error; err != nil {
				return err
			}

			// 删除重复的 workflows 记录
			if err := tx.Exec("DELETE FROM workflows WHERE id IN ?", dupIDs).Error; err != nil {
				return err
			}

			logger.Info("deduplicated workflow",
				zap.String("name", dup.Name),
				zap.Uint64("keep_id", keepID),
				zap.Int("removed", len(dupIDs)),
			)
		}

		// 标记迁移完成
		if err := tx.Create(&SystemConfig{
			ConfigKey:   migrationKey,
			ConfigValue: "done",
			ConfigType:  "string",
			Category:    "migration",
			Description: "全局工单类型和工作流去重迁移标记",
		}).Error; err != nil {
			return err
		}

		logger.Info("global deduplication completed")
		return nil
	})
}

// seedDefaultAlertSilences 初始化默认告警静默规则模板
func seedDefaultAlertSilences(db *gorm.DB, adminUserID uint64) error {
	logger.Info("seeding default alert silence templates...")

	// 使用固定的过去时间作为模板时间，状态为已取消(0)，用户使用时需修改时间并启用
	templateStart := time.Date(2025, 1, 1, 22, 0, 0, 0, time.Local)
	templateEnd := time.Date(2025, 1, 2, 6, 0, 0, 0, time.Local)
	templateStart2 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	templateEnd2 := time.Date(2025, 1, 2, 0, 0, 0, 0, time.Local)
	templateEnd7d := time.Date(2025, 1, 8, 0, 0, 0, 0, time.Local)

	silences := []AlertSilence{
		{
			Name:          "例行维护窗口 - 全局静默",
			Description:   "用于每周例行维护期间静默所有告警，避免维护操作触发大量无意义工单。使用时修改时间并启用。",
			LabelMatchers: `[{"key":"severity","value":"critical|warning|info","operator":"=~"}]`,
			StartsAt:      templateStart,
			SilenceType:   2,
			EndsAt:        &templateEnd,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：例行维护期间静默所有告警，使用前请修改时间并启用",
			Status:        0,
		},
		{
			Name:          "例行维护窗口 - 指定主机",
			Description:   "用于对特定主机进行维护时静默该主机的所有告警。使用时修改 instance 为目标主机IP，修改时间并启用。",
			LabelMatchers: `[{"key":"instance","value":"10.0.0.1.*","operator":"=~"}]`,
			StartsAt:      templateStart,
			SilenceType:   2,
			EndsAt:        &templateEnd,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：指定主机维护静默，使用前请修改主机IP、时间并启用",
			Status:        0,
		},
		{
			Name:          "例行维护窗口 - 指定业务组",
			Description:   "用于对特定业务组进行维护时静默该组的所有告警。使用时修改 group_name 为目标业务组，修改时间并启用。",
			LabelMatchers: `[{"key":"group_name","value":"prod-app","operator":"=="}]`,
			StartsAt:      templateStart,
			SilenceType:   2,
			EndsAt:        &templateEnd,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：指定业务组维护静默，使用前请修改业务组名、时间并启用",
			Status:        0,
		},
		{
			Name:          "临时静默 - 指定告警名称",
			Description:   "用于临时屏蔽某个已知告警（如误报、正在处理中的告警）。使用时修改 alertname 为目标告警名称，修改时间并启用。",
			LabelMatchers: `[{"key":"alertname","value":"HostHighCpuUsage-level-1","operator":"=="}]`,
			StartsAt:      templateStart2,
			SilenceType:   2,
			EndsAt:        &templateEnd2,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：临时屏蔽指定告警，使用前请修改告警名称、时间并启用",
			Status:        0,
		},
		{
			Name:          "临时静默 - Falco安全告警",
			Description:   "用于临时屏蔽 Falco 容器安全告警，例如在部署或调试期间容器行为可能触发大量安全告警。使用时修改时间并启用。",
			LabelMatchers: `[{"key":"alertname","value":".*Falco.*","operator":"=~"}]`,
			StartsAt:      templateStart2,
			SilenceType:   2,
			EndsAt:        &templateEnd2,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：临时屏蔽Falco安全告警，使用前请修改时间并启用",
			Status:        0,
		},
		{
			Name:          "临时静默 - 低级别告警",
			Description:   "用于故障高峰期只关注严重告警，临时屏蔽 warning 和 info 级别告警，减少噪音。使用时修改时间并启用。",
			LabelMatchers: `[{"key":"severity","value":"warning|info","operator":"=~"}]`,
			StartsAt:      templateStart2,
			SilenceType:   2,
			EndsAt:        &templateEnd7d,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：屏蔽低级别告警只保留critical，使用前请修改时间并启用",
			Status:        0,
		},
		{
			Name:          "临时静默 - 测试/UAT环境",
			Description:   "用于临时屏蔽测试和UAT环境的所有告警，例如在测试环境大规模变更期间。使用时修改时间并启用。",
			LabelMatchers: `[{"key":"group_name","value":".*uat.*|.*test.*","operator":"=~"}]`,
			StartsAt:      templateStart2,
			SilenceType:   2,
			EndsAt:        &templateEnd7d,
			CreatedBy:     adminUserID,
			Comment:       "模板规则：屏蔽测试/UAT环境告警，使用前请修改时间并启用",
			Status:        0,
		},
	}

	for _, silence := range silences {
		result := db.Where("name = ?", silence.Name).FirstOrCreate(&silence)
		if result.Error != nil {
			logger.Error("failed to seed alert silence", zap.String("name", silence.Name), zap.Error(result.Error))
			return result.Error
		}
	}

	logger.Info("default alert silence templates seeded")
	return nil
}

// 初始化状态相关常量
//
// 刻意不 import system-config 包：model 是最底层，被所有模块依赖，
// 反过来依赖上层服务会成环。
const (
	setupCompletedKey = "setup.completed"
	alertBotUsername  = "alert-bot"
)

// seedSetupState 判定并落定初始化状态
//
// 关键在于向后兼容：老部署升级上来时数据库里早就有用户了，
// 绝不能弹向导 —— 那会让一个已经在跑的实例变成"谁先访问谁是管理员"。
// 所以只在"配置行不存在（老库首次升级）"时做一次判定，之后这行就是唯一依据。
//
// alert-bot 是系统账号、不是人，判定时要排除掉，
// 否则新装的库因为有 alert-bot 会被误判成老部署，向导永远出不来。
func seedSetupState(db *gorm.DB) error {
	var existing SystemConfig
	err := db.Where("config_key = ?", setupCompletedKey).First(&existing).Error
	if err == nil {
		return nil // 已判定过，不再改动
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("failed to read setup state", zap.Error(err))
		return err
	}

	var humanUsers int64
	if err := db.Model(&User{}).Where("username <> ?", alertBotUsername).Count(&humanUsers).Error; err != nil {
		logger.Error("failed to count users for setup state", zap.Error(err))
		return err
	}

	completed := "0"
	if humanUsers > 0 {
		completed = "1" // 老部署：已经有人在用了
	}

	if err := db.Create(&SystemConfig{
		ConfigKey:   setupCompletedKey,
		ConfigValue: completed,
		ConfigType:  "string",
		Category:    "setup",
		Description: "是否已完成首次初始化",
	}).Error; err != nil {
		logger.Error("failed to seed setup state", zap.Error(err))
		return err
	}

	logger.Info("setup state initialized",
		zap.String("completed", completed),
		zap.Int64("existing_users", humanUsers),
	)
	return nil
}
