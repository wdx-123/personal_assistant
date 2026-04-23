package flag

import (
	"errors"
	"fmt"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"

	"github.com/gofrs/uuid"
	"gorm.io/gorm"
)

const (
	legacyCurrentOrgID     uint = 1 // 历史库中需要迁入全体成员组织的源组织
	legacyUserRoleMemberID uint = 1 // 历史 user_roles 中的默认成员角色
)

// SQL 表结构迁移，如果表不存在，它会创建新表；如果表已经存在，它会根据结构更新表
// 迁移完成后自动初始化内置角色（幂等）
func SQL() error {
	db := global.DB.Set("gorm:table_options", "ENGINE=InnoDB")

	if err := db.AutoMigrate(
		&entity.AIConversation{},          // AI 会话表
		&entity.AIMessage{},               // AI 消息表
		&entity.AIInterrupt{},             // AI 中断表
		&entity.User{},                    // 用户表
		&entity.Org{},                     // 组织表
		&entity.OrgMember{},               // 组织成员状态表 - 身份上的
		&entity.LeetcodeUserDetail{},      // 力扣用户详情表
		&entity.LuoguUserDetail{},         // 洛谷用户详情表
		&entity.LanqiaoUserDetail{},       // 蓝桥用户详情表
		&entity.LeetcodeQuestionBank{},    // 力扣题库题目表
		&entity.LuoguQuestionBank{},       // 洛谷题库题目表
		&entity.LanqiaoQuestionBank{},     // 蓝桥题库题目表
		&entity.LeetcodeUserQuestion{},    // 力扣用户以做题目表
		&entity.LuoguUserQuestion{},       // 洛谷用户以做题目表
		&entity.LanqiaoUserQuestion{},     // 蓝桥用户已通过题目表
		&entity.OJUserDailyStat{},         // OJ 刷题曲线日聚合读模型
		&entity.OJTask{},                  // OJ 任务版本表
		&entity.OJTaskOrg{},               // OJ 任务组织关联表
		&entity.OJTaskItem{},              // OJ 任务题单表
		&entity.OJQuestionIntake{},        // OJ 任务待解析题目表
		&entity.OJTaskExecution{},         // OJ 任务执行表
		&entity.OJTaskExecutionUser{},     // OJ 任务执行用户快照表
		&entity.OJTaskExecutionUserOrg{},  // OJ 任务执行用户组织快照表
		&entity.OJTaskExecutionUserItem{}, // OJ 任务执行用户题目快照表
		&entity.Login{},                   // 登录日志表
		&entity.UserToken{},               // 用户Token记录表
		&entity.TokenBlacklist{},          // Token黑名单表
		&entity.JwtBlacklist{},            // JWT黑名单表（兼容现有代码）
		&entity.Role{},                    // 角色表
		&entity.Capability{},              // 业务能力表
		&entity.Menu{},                    // 菜单表
		&entity.UserOrgRole{},             // 用户组织角色关联表 - 权限上的（与 OrgMember 配合）
		&entity.RoleCapability{},          // 角色与业务能力关联表
		&entity.RoleAPI{},                 // 角色API直绑关联表
		&entity.API{},                     // api表
		&entity.OutboxEvent{},             // Outbox事件表
		&entity.Image{},                   // 图片表
		&entity.ObservabilityMetric{},     // 指标聚合表
		&entity.ObservabilityTraceSpan{},  // 全链路追踪明细表
	); err != nil {
		return err
	}

	// AutoMigrate 会在当前 *gorm.DB 上留下 Statement 上下文。
	// 后续迁移步骤如果继续复用同一个实例，可能把历史表名串到新的查询/DDL 中。
	// 这里显式切换到全新 Session，避免后续辅助迁移误命中脏状态。
	db = global.DB.Session(&gorm.Session{NewDB: true})

	// menu_apis 加唯一索引前先按 api_id 清洗历史重复绑定，仅保留最小 menu_id。
	// 清晰数据用的，后期可删
	// if err := normalizeMenuAPISingleBinding(); err != nil {
	// 	return err
	// }

	// 显式迁移 menu_apis 中间表，补齐 api_id 唯一约束（单API归属单菜单）。
	// 迁移用的后期可删
	if err := db.AutoMigrate(&entity.MenuAPI{}); err != nil {
		return err
	}

	// 表结构就绪后，初始化内置角色
	if err := seedBuiltinRoles(); err != nil {
		return err
	}
	if err := seedBuiltinAssistantMenu(); err != nil {
		return err
	}
	if err := seedBuiltinAssistantRoleMenus(); err != nil {
		return err
	}
	if err := seedBuiltinCapabilities(); err != nil {
		return err
	}
	if err := seedBuiltinRoleCapabilities(); err != nil {
		return err
	}
	if err := migrateOJTaskSchema(db); err != nil {
		return err
	}
	if err := migrateAPILifecycleData(db); err != nil {
		return err
	}
	if err := dropLegacyAIMessageUIColumn(db); err != nil {
		return err
	}

	return migrateOrgMemberLifecycleData(db)
}

const (
	builtinAssistantMenuCode          = "console:assistant"
	builtinAssistantMenuName          = "AI助手"
	builtinAssistantMenuIcon          = "RobotOutlined"
	builtinAssistantMenuRouteName     = "ConsoleAssistant"
	builtinAssistantMenuRoutePath     = "/console/assistant"
	builtinAssistantMenuComponentPath = "@/views/Console/Workbench/AssistantWorkbench.vue"
	builtinAssistantMenuDesc          = "AI 助手工作台"
	builtinAssistantMenuSort          = 15
)

// normalizeMenuAPISingleBinding 将同一 api_id 的多条菜单绑定裁剪为一条（保留最小 menu_id）。
// func normalizeMenuAPISingleBinding() error {
// 	if !global.DB.Migrator().HasTable("menu_apis") {
// 		return nil
// 	}

// 	return global.DB.Transaction(func(tx *gorm.DB) error {
// 		if err := tx.Exec(`
// 			CREATE TEMPORARY TABLE tmp_menu_apis_keep AS
// 			SELECT MIN(menu_id) AS menu_id, api_id
// 			FROM menu_apis
// 			GROUP BY api_id
// 		`).Error; err != nil {
// 			return err
// 		}

// 		if err := tx.Exec("DELETE FROM menu_apis").Error; err != nil {
// 			_ = tx.Exec("DROP TEMPORARY TABLE IF EXISTS tmp_menu_apis_keep").Error
// 			return err
// 		}

// 		if err := tx.Exec(`
// 			INSERT INTO menu_apis (menu_id, api_id)
// 			SELECT menu_id, api_id
// 			FROM tmp_menu_apis_keep
// 		`).Error; err != nil {
// 			_ = tx.Exec("DROP TEMPORARY TABLE IF EXISTS tmp_menu_apis_keep").Error
// 			return err
// 		}

// 		return tx.Exec("DROP TEMPORARY TABLE IF EXISTS tmp_menu_apis_keep").Error
// 	})
// }

// seedBuiltinRoles 初始化内置角色（幂等：已存在则跳过，不会重复创建）
func seedBuiltinRoles() error {
	roles := []entity.Role{
		{Code: consts.RoleCodeSuperAdmin, Name: "系统超级管理员", Status: 1, Desc: "系统最高权限，管理所有组织和用户"},
		{Code: consts.RoleCodeOrgAdmin, Name: "组织管理员", Status: 1, Desc: "管理单个组织内的一切"},
		{Code: consts.RoleCodeMember, Name: "普通成员", Status: 1, Desc: "组织普通成员"},
	}

	for _, r := range roles {
		var existing entity.Role
		// 使用 Unscoped() 忽略软删除标记，防止因记录被软删除而重复创建导致的唯一键冲突
		if err := global.DB.Unscoped().Where("code = ?", r.Code).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 角色不存在，创建
				if createErr := global.DB.Create(&r).Error; createErr != nil {
					return createErr
				}
			} else {
				return err
			}
		} else {
			// 如果角色存在但被软删除了，恢复它
			if existing.DeletedAt.Valid {
				if err := global.DB.Unscoped().Model(&existing).Update("deleted_at", nil).Error; err != nil {
					return err
				}
			}
		}
		// 角色已存在且正常，跳过
	}
	return nil
}

// seedBuiltinAssistantMenu 确保 AI 助手菜单存在，便于前端把工作台入口纳入 RBAC 菜单权限。
func seedBuiltinAssistantMenu() error {
	record := entity.Menu{
		ParentID:      0,
		Name:          builtinAssistantMenuName,
		Code:          builtinAssistantMenuCode,
		Icon:          builtinAssistantMenuIcon,
		Type:          2,
		RouteName:     builtinAssistantMenuRouteName,
		RoutePath:     builtinAssistantMenuRoutePath,
		ComponentPath: builtinAssistantMenuComponentPath,
		Status:        1,
		Sort:          builtinAssistantMenuSort,
		Desc:          builtinAssistantMenuDesc,
	}

	var existing entity.Menu
	query := global.DB.Unscoped().
		Where("code = ?", builtinAssistantMenuCode).
		Or("route_path = ?", builtinAssistantMenuRoutePath)
	if err := query.First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return global.DB.Create(&record).Error
		}
		return err
	}

	updates := map[string]any{
		"name":           builtinAssistantMenuName,
		"code":           builtinAssistantMenuCode,
		"icon":           builtinAssistantMenuIcon,
		"type":           2,
		"route_name":     builtinAssistantMenuRouteName,
		"route_path":     builtinAssistantMenuRoutePath,
		"component_path": builtinAssistantMenuComponentPath,
		"status":         1,
		"sort":           builtinAssistantMenuSort,
		"desc":           builtinAssistantMenuDesc,
	}
	if existing.DeletedAt.Valid {
		updates["deleted_at"] = nil
	}

	return global.DB.Unscoped().Model(&existing).Updates(updates).Error
}

// seedBuiltinAssistantRoleMenus 把 AI 助手菜单补给内置组织角色，维持当前“登录后可用”的默认体验。
func seedBuiltinAssistantRoleMenus() error {
	var assistantMenu entity.Menu
	if err := global.DB.Where("code = ?", builtinAssistantMenuCode).First(&assistantMenu).Error; err != nil {
		return err
	}

	roleCodes := []string{consts.RoleCodeOrgAdmin, consts.RoleCodeMember}
	for _, roleCode := range roleCodes {
		var role entity.Role
		if err := global.DB.Where("code = ?", roleCode).First(&role).Error; err != nil {
			return err
		}
		if err := global.DB.Exec(
			"INSERT IGNORE INTO role_menus (role_id, menu_id) VALUES (?, ?)",
			role.ID,
			assistantMenu.ID,
		).Error; err != nil {
			return err
		}
	}

	return nil
}

// seedBuiltinCapabilities 初始化系统内置 capability（幂等）。
func seedBuiltinCapabilities() error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	for _, seed := range consts.BuiltinCapabilitySeeds() {
		record := entity.Capability{
			Code:      seed.Code,
			Name:      seed.Name,
			Domain:    seed.Domain,
			GroupCode: seed.GroupCode,
			GroupName: seed.GroupName,
			Desc:      seed.Desc,
			Status:    1,
		}

		var existing entity.Capability
		if err := global.DB.Unscoped().Where("code = ?", seed.Code).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if createErr := global.DB.Create(&record).Error; createErr != nil {
					return createErr
				}
				continue
			}
			return err
		}

		updates := map[string]any{
			"name":       seed.Name,
			"domain":     seed.Domain,
			"group_code": seed.GroupCode,
			"group_name": seed.GroupName,
			"desc":       seed.Desc,
			"status":     1,
		}
		if existing.DeletedAt.Valid {
			updates["deleted_at"] = nil
		}
		if err := global.DB.Unscoped().Model(&existing).Updates(updates).Error; err != nil {
			return err
		}
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	return nil
}

// seedBuiltinRoleCapabilities 确保组织管理员默认持有全部组织域 capability。
func seedBuiltinRoleCapabilities() error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	var orgAdmin entity.Role
	if err := global.DB.Where("code = ?", consts.RoleCodeOrgAdmin).First(&orgAdmin).Error; err != nil {
		return err
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	for _, code := range consts.BuiltinOrgAdminCapabilityCodes() {
		var capability entity.Capability
		if err := global.DB.Where("code = ?", code).First(&capability).Error; err != nil {
			return err
		}
		if err := global.DB.Exec(
			"INSERT IGNORE INTO role_capabilities (role_id, capability_id) VALUES (?, ?)",
			orgAdmin.ID,
			capability.ID,
		).Error; err != nil {
			return err
		}
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return nil
}

// migrateOrgMemberLifecycleData 迁移组织成员生命周期相关的数据和结构，确保数据一致性和完整性
func migrateOrgMemberLifecycleData(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if err := ensureLifecycleSchema(db); err != nil {
		return err
	}
	if err := normalizeOrgInviteCodes(db); err != nil {
		return err
	}
	if err := normalizeUserStatusWithFreeze(db); err != nil {
		return err
	}
	if err := cleanupSoftDeletedUserOrgRoles(db); err != nil {
		return err
	}
	if err := deduplicateUserOrgRoles(db); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "user_org_roles", "uk_user_org_role", "user_id", "org_id", "role_id"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "orgs", "uk_org_code", "code"); err != nil {
		return err
	}
	if err := deduplicateOrgMembers(db); err != nil {
		return err
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err := ensureUniqueIndex(db, "org_members", "uk_org_member", "org_id", "user_id"); err != nil {
		return err
	}

	allMembersOrgID, err := ensureAllMembersOrg(db)
	if err != nil {
		return err
	}
	memberRoleID, err := getMemberRoleID(db)
	if err != nil {
		return err
	}
	if err := migrateLegacyUserRoleToUserOrgRole(
		db,
		legacyUserRoleMemberID,
		allMembersOrgID,
		memberRoleID,
	); err != nil {
		return err
	}
	if err := backfillOrgMembersFromRoleRelations(db); err != nil {
		return err
	}
	// 历史 users.current_org_id=1 的用户仅补入全体成员组织，不再回写到 org_id=1。
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return backfillUsersByCurrentOrgToOrgMembers(
		db,
		legacyCurrentOrgID,
		allMembersOrgID,
		consts.OrgMemberJoinSourceLegacyBackfill,
	)
}

// ensureLifecycleSchema 为历史库补齐本次重构依赖的关键字段。
// 说明：理论上 AutoMigrate 会自动补列，但在部分历史环境中可能出现“列未就绪即被查询”的情况，
// 这里增加显式兜底，避免启动失败。
func ensureLifecycleSchema(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if err := ensureColumn(db, &entity.User{}, "Status"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.User{}, "DisabledAt"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.User{}, "DisabledBy"); err != nil {
		return err
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err := ensureColumn(db, &entity.User{}, "DisabledReason"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.Org{}, "IsBuiltin"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.Org{}, "BuiltinKey"); err != nil {
		return err
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return nil
}

// ensureColumn 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - model：当前函数需要消费的输入参数。
//   - field：当前函数需要消费的输入参数。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func ensureColumn(db *gorm.DB, model any, field string) error {
	// 使用全新 Session，避免复用链路上残留的 Statement/Table 状态，
	// 导致 GORM 在 AddColumn 时错误地把目标表解析成历史上下文中的临时表名。
	migrator := db.Session(&gorm.Session{NewDB: true}).Migrator()
	if migrator.HasColumn(model, field) {
		return nil
	}
	return migrator.AddColumn(model, field)
}

// normalizeOrgInviteCodes 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func normalizeOrgInviteCodes(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	var orgs []entity.Org
	if err := db.Unscoped().Select("id", "code").Order("id ASC").Find(&orgs).Error; err != nil {
		return err
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	usedCodes := make(map[string]struct{}, len(orgs))
	for _, org := range orgs {
		code := strings.TrimSpace(org.Code)
		_, exists := usedCodes[code]
		if code == "" || exists {
			newCode, err := generateUniqueOrgCode(usedCodes)
			if err != nil {
				return err
			}
			if err := db.Unscoped().Model(&entity.Org{}).Where("id = ?", org.ID).Update("code", newCode).Error; err != nil {
				return err
			}
			code = newCode
		}
		usedCodes[code] = struct{}{}
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return nil
}

// generateUniqueOrgCode 负责执行当前函数对应的核心逻辑。
// 参数：
//   - used：当前函数需要消费的输入参数。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func generateUniqueOrgCode(used map[string]struct{}) (string, error) {
	for i := 0; i < 10; i++ {
		raw := strings.ToUpper(strings.ReplaceAll(uuid.Must(uuid.NewV4()).String(), "-", ""))
		if len(raw) < 10 {
			continue
		}
		code := "ORG-" + raw[:10]
		if _, exists := used[code]; !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique org code")
}

// normalizeUserStatusWithFreeze 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func normalizeUserStatusWithFreeze(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if err := ensureUsersStatusColumnsBeforeNormalize(db); err != nil {
		return fmt.Errorf("ensure users lifecycle columns failed: %w", err)
	}

	if err := db.Model(&entity.User{}).
		Where("status = 0").
		Update("status", consts.UserStatusActive).Error; err != nil {
		return err
	}

	hasFreeze, err := columnExists(db, "users", "freeze")
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err != nil {
		return err
	}
	if !hasFreeze {
		return nil
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return db.Model(&entity.User{}).
		Where("freeze = ? AND status = ?", true, consts.UserStatusActive).
		Updates(map[string]any{
			"status":      consts.UserStatusDisabled,
			"disabled_at": gorm.Expr("COALESCE(disabled_at, updated_at)"),
		}).Error
}

// ensureUsersStatusColumnsBeforeNormalize 在执行 status 相关数据修复前，确保列已存在。
// 先走 GORM Migrator，再用 SQL 兜底，兼容历史库。
func ensureUsersStatusColumnsBeforeNormalize(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if err := ensureColumn(db, &entity.User{}, "Freeze"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.User{}, "Status"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.User{}, "DisabledAt"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.User{}, "DisabledBy"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.User{}, "DisabledReason"); err != nil {
		return err
	}

	// SQL 兜底：防止个别环境下 HasColumn/AddColumn 未按预期生效。
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err := ensureMySQLColumnWithDDL(
		db,
		"users",
		"freeze",
		"`freeze` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '用户冻结状态'",
	); err != nil {
		return err
	}
	if err := ensureMySQLColumnWithDDL(
		db,
		"users",
		"status",
		"`status` TINYINT NOT NULL DEFAULT 1 COMMENT '账号状态：1 active,2 disabled,3 deleted_soft'",
	); err != nil {
		return err
	}
	if err := ensureMySQLColumnWithDDL(
		db,
		"users",
		"disabled_at",
		"`disabled_at` DATETIME NULL COMMENT '禁用时间'",
	); err != nil {
		return err
	}
	if err := ensureMySQLColumnWithDDL(
		db,
		"users",
		"disabled_by",
		"`disabled_by` BIGINT UNSIGNED NULL COMMENT '禁用操作者ID'",
	); err != nil {
		return err
	}
	if err := ensureMySQLColumnWithDDL(
		db,
		"users",
		"disabled_reason",
		"`disabled_reason` VARCHAR(200) NOT NULL DEFAULT '' COMMENT '禁用原因'",
	); err != nil {
		return err
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return nil
}

// ensureMySQLColumnWithDDL 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - tableName：当前函数需要消费的输入参数。
//   - columnName：当前函数需要消费的输入参数。
//   - ddl：当前函数需要消费的输入参数。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func ensureMySQLColumnWithDDL(db *gorm.DB, tableName, columnName, ddl string) error {
	exists, err := columnExists(db, tableName, columnName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := db.Exec(fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s", tableName, ddl)).Error; err != nil {
		// 1060 Duplicate column name，视为幂等成功。
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return nil
		}
		return err
	}
	return nil
}

// cleanupSoftDeletedUserOrgRoles 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func cleanupSoftDeletedUserOrgRoles(db *gorm.DB) error {
	return db.Exec("DELETE FROM user_org_roles WHERE deleted_at IS NOT NULL").Error
}

// deduplicateUserOrgRoles 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func deduplicateUserOrgRoles(db *gorm.DB) error {
	return db.Exec(`
		DELETE u1
		FROM user_org_roles u1
		INNER JOIN user_org_roles u2
			ON u1.user_id = u2.user_id
			AND u1.org_id = u2.org_id
			AND u1.role_id = u2.role_id
			AND u1.id > u2.id
	`).Error
}

// deduplicateOrgMembers 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func deduplicateOrgMembers(db *gorm.DB) error {
	return db.Exec(`
		DELETE m1
		FROM org_members m1
		INNER JOIN org_members m2
			ON m1.org_id = m2.org_id
			AND m1.user_id = m2.user_id
			AND m1.id > m2.id
	`).Error
}

// migrateOJTaskSchema 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func migrateOJTaskSchema(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if err := ensureUniqueIndex(db, "oj_tasks", "uk_oj_tasks_root_version", "root_task_id", "version_no"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_tasks", "idx_oj_tasks_created_by_created_at", "created_by", "created_at"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "oj_task_orgs", "uk_oj_task_orgs_task_org", "task_id", "org_id"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_orgs", "idx_oj_task_orgs_org_task", "org_id", "task_id"); err != nil {
		return err
	}
	if err := backfillOJTaskItemSchema(db); err != nil {
		return err
	}
	if err := relaxLegacyOJTaskItemColumns(db); err != nil {
		return err
	}
	if err := dropIndexIfExists(db, "oj_task_items", "uk_oj_task_items_task_platform_code"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_items", "idx_oj_task_items_task_resolved_question", "task_id", "resolved_question_id"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_items", "idx_oj_task_items_task_resolution_status", "task_id", "resolution_status"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_items", "idx_oj_task_items_platform_input_title", "platform", "input_title"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "oj_question_intakes", "uk_oj_question_intakes_task_item", "task_item_id"); err != nil {
		return err
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if err := ensureIndex(db, "oj_question_intakes", "idx_oj_question_intakes_platform_title_status", "platform", "input_title", "status"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_question_intakes", "idx_oj_question_intakes_task_status", "task_id", "status"); err != nil {
		return err
	}
	if err := syncOJQuestionIntakesFromTaskItems(db); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "oj_task_executions", "uk_oj_task_executions_task", "task_id"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_executions", "idx_oj_task_executions_status_planned", "status", "planned_at"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_executions", "idx_oj_task_executions_task_created", "task_id", "created_at"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "oj_task_execution_users", "uk_oj_task_execution_users_execution_user", "execution_id", "user_id"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_execution_users", "idx_oj_task_execution_users_execution_completed_user", "execution_id", "all_completed", "user_id"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "oj_task_execution_user_orgs", "uk_oj_task_execution_user_orgs_execution_user_org", "execution_user_id", "org_id"); err != nil {
		return err
	}
	if err := ensureUniqueIndex(db, "oj_task_execution_user_items", "uk_oj_task_execution_user_items_execution_user_task_item", "execution_user_id", "task_item_id"); err != nil {
		return err
	}
	if err := ensureIndex(db, "oj_task_execution_user_items", "idx_oj_task_execution_user_items_execution_user_status_reason", "execution_user_id", "result_status", "reason"); err != nil {
		return err
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return ensureIndex(db, "oj_task_execution_user_items", "idx_oj_task_execution_user_items_execution_user_result", "execution_id", "user_id", "result_status")
}

// backfillOJTaskItemSchema 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func backfillOJTaskItemSchema(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if !db.Migrator().HasTable("oj_task_items") {
		return nil
	}

	hasQuestionTitleSnapshot, err := columnExists(db, "oj_task_items", "question_title_snapshot")
	if err != nil {
		return err
	}
	hasQuestionCode, err := columnExists(db, "oj_task_items", "question_code")
	if err != nil {
		return err
	}
	hasPlatformQuestionID, err := columnExists(db, "oj_task_items", "platform_question_id")
	if err != nil {
		return err
	}

	inputTitleExpr := "COALESCE(NULLIF(input_title, ''), '')"
	if hasQuestionTitleSnapshot && hasQuestionCode {
		inputTitleExpr = "COALESCE(NULLIF(input_title, ''), NULLIF(question_title_snapshot, ''), NULLIF(question_code, ''), '')"
	} else if hasQuestionTitleSnapshot {
		inputTitleExpr = "COALESCE(NULLIF(input_title, ''), NULLIF(question_title_snapshot, ''), '')"
	} else if hasQuestionCode {
		inputTitleExpr = "COALESCE(NULLIF(input_title, ''), NULLIF(question_code, ''), '')"
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	resolvedQuestionIDExpr := "COALESCE(resolved_question_id, 0)"
	if hasPlatformQuestionID {
		resolvedQuestionIDExpr = "CASE WHEN COALESCE(resolved_question_id, 0) > 0 THEN resolved_question_id ELSE COALESCE(platform_question_id, 0) END"
	}

	resolvedQuestionCodeExpr := "COALESCE(NULLIF(resolved_question_code, ''), '')"
	if hasQuestionCode {
		resolvedQuestionCodeExpr = "COALESCE(NULLIF(resolved_question_code, ''), NULLIF(question_code, ''), '')"
	}

	resolvedTitleSnapshotExpr := "COALESCE(NULLIF(resolved_title_snapshot, ''), NULLIF(input_title, ''), '')"
	if hasQuestionTitleSnapshot && hasQuestionCode {
		resolvedTitleSnapshotExpr = "COALESCE(NULLIF(resolved_title_snapshot, ''), NULLIF(question_title_snapshot, ''), NULLIF(input_title, ''), NULLIF(question_code, ''), '')"
	} else if hasQuestionTitleSnapshot {
		resolvedTitleSnapshotExpr = "COALESCE(NULLIF(resolved_title_snapshot, ''), NULLIF(question_title_snapshot, ''), NULLIF(input_title, ''), '')"
	} else if hasQuestionCode {
		resolvedTitleSnapshotExpr = "COALESCE(NULLIF(resolved_title_snapshot, ''), NULLIF(input_title, ''), NULLIF(question_code, ''), '')"
	}

	resolutionStatusExpr := fmt.Sprintf(`CASE
		WHEN resolution_status IN ('resolved', 'shadow_resolved') THEN 'resolved'
		WHEN resolution_status = 'shadow_pending' THEN 'pending_resolution'
		WHEN resolution_status = 'missing' THEN 'invalid'
		WHEN (%s) > 0 THEN 'resolved'
		WHEN resolution_status = 'invalid' THEN 'invalid'
		ELSE 'pending_resolution'
	END`, resolvedQuestionIDExpr)

	updateSQL := fmt.Sprintf(`
		UPDATE oj_task_items
		SET
			input_title = %s,
			input_mode = 'title',
			resolved_question_id = %s,
			resolved_question_code = %s,
			resolved_title_snapshot = %s,
			resolution_status = %s
		WHERE deleted_at IS NULL
	`, inputTitleExpr, resolvedQuestionIDExpr, resolvedQuestionCodeExpr, resolvedTitleSnapshotExpr, resolutionStatusExpr)
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return db.Exec(updateSQL).Error
}

// relaxLegacyOJTaskItemColumns 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func relaxLegacyOJTaskItemColumns(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if !db.Migrator().HasTable("oj_task_items") {
		return nil
	}

	hasQuestionCode, err := columnExists(db, "oj_task_items", "question_code")
	if err != nil {
		return err
	}
	if hasQuestionCode {
		if err := db.Exec(`
			UPDATE oj_task_items
			SET question_code = COALESCE(NULLIF(question_code, ''), NULLIF(resolved_question_code, ''), '')
			WHERE question_code = ''
		`).Error; err != nil {
			return err
		}
		// 旧列仍会留在历史库里；补默认值避免新口径插入时再次被 MySQL 拦截。
		if err := db.Exec(`
			ALTER TABLE oj_task_items
			MODIFY COLUMN question_code varchar(64) NOT NULL DEFAULT '' COMMENT '平台题目编码'
		`).Error; err != nil {
			return err
		}
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	hasPlatformQuestionID, err := columnExists(db, "oj_task_items", "platform_question_id")
	if err != nil {
		return err
	}
	if hasPlatformQuestionID {
		if err := db.Exec(`
			UPDATE oj_task_items
			SET platform_question_id = CASE
				WHEN COALESCE(platform_question_id, 0) > 0 THEN platform_question_id
				ELSE COALESCE(resolved_question_id, 0)
			END
			WHERE COALESCE(platform_question_id, 0) = 0
		`).Error; err != nil {
			return err
		}
		if err := db.Exec(`
			ALTER TABLE oj_task_items
			MODIFY COLUMN platform_question_id bigint unsigned NOT NULL DEFAULT 0 COMMENT '本地题库主键ID'
		`).Error; err != nil {
			return err
		}
	}

	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return nil
}

// syncOJQuestionIntakesFromTaskItems 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func syncOJQuestionIntakesFromTaskItems(db *gorm.DB) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if !db.Migrator().HasTable("oj_task_items") || !db.Migrator().HasTable("oj_question_intakes") {
		return nil
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	return db.Exec(`
		INSERT INTO oj_question_intakes (
			task_id,
			task_item_id,
			platform,
			input_title,
			status,
			resolved_question_id,
			resolution_note,
			created_at,
			updated_at
		)
		SELECT
			task_id,
			id,
			platform,
			input_title,
			resolution_status,
			resolved_question_id,
			resolution_note,
			NOW(),
			NOW()
		FROM oj_task_items
		WHERE deleted_at IS NULL
			AND resolution_status = 'pending_resolution'
		ON DUPLICATE KEY UPDATE
			task_id = VALUES(task_id),
			platform = VALUES(platform),
			input_title = VALUES(input_title),
			status = VALUES(status),
			resolved_question_id = VALUES(resolved_question_id),
			resolution_note = VALUES(resolution_note),
			deleted_at = NULL,
			updated_at = VALUES(updated_at)
	`).Error
}

// ensureUniqueIndex 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - tableName：当前函数需要消费的输入参数。
//   - indexName：当前函数需要消费的输入参数。
//   - columns：当前函数需要消费的输入参数。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func ensureUniqueIndex(db *gorm.DB, tableName, indexName string, columns ...string) error {
	exists, err := indexExists(db, tableName, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return db.Exec(
		fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", indexName, tableName, strings.Join(columns, ", ")),
	).Error
}

// ensureIndex 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - tableName：当前函数需要消费的输入参数。
//   - indexName：当前函数需要消费的输入参数。
//   - columns：当前函数需要消费的输入参数。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func ensureIndex(db *gorm.DB, tableName, indexName string, columns ...string) error {
	exists, err := indexExists(db, tableName, indexName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return db.Exec(
		fmt.Sprintf("CREATE INDEX %s ON %s (%s)", indexName, tableName, strings.Join(columns, ", ")),
	).Error
}

// dropIndexIfExists 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - tableName：当前函数需要消费的输入参数。
//   - indexName：当前函数需要消费的输入参数。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func dropIndexIfExists(db *gorm.DB, tableName, indexName string) error {
	exists, err := indexExists(db, tableName, indexName)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return db.Exec(fmt.Sprintf("DROP INDEX %s ON %s", indexName, tableName)).Error
}

// indexExists 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - tableName：当前函数需要消费的输入参数。
//   - indexName：当前函数需要消费的输入参数。
//
// 返回值：
//   - bool：表示当前操作是否成功、命中或可继续执行。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func indexExists(db *gorm.DB, tableName, indexName string) (bool, error) {
	var count int64
	row := db.Raw(
		`SELECT COUNT(1) AS count
		   FROM information_schema.statistics
		  WHERE table_schema = DATABASE()
			AND table_name = ?
			AND index_name = ?`,
		tableName,
		indexName,
	).Row()
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// columnExists 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//   - tableName：当前函数需要消费的输入参数。
//   - columnName：当前函数需要消费的输入参数。
//
// 返回值：
//   - bool：表示当前操作是否成功、命中或可继续执行。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func columnExists(db *gorm.DB, tableName, columnName string) (bool, error) {
	var count int64
	row := db.Raw(
		`SELECT COUNT(1) AS count
		   FROM information_schema.columns
		  WHERE table_schema = DATABASE()
			AND table_name = ?
			AND column_name = ?`,
		tableName,
		columnName,
	).Row()
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// migrateAPILifecycleData 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func migrateAPILifecycleData(db *gorm.DB) error {
	if !db.Migrator().HasTable(&entity.API{}) {
		return nil
	}
	return db.Session(&gorm.Session{NewDB: true}).
		Unscoped().
		Model(&entity.API{}).
		Where("sync_state = '' OR sync_state IS NULL").
		Update("sync_state", consts.APISyncStateRegistered).
		Error
}

// dropLegacyAIMessageUIColumn removes the legacy AI message UI column.
func dropLegacyAIMessageUIColumn(db *gorm.DB) error {
	if !db.Migrator().HasTable("ai_messages") {
		return nil
	}
	exists, err := columnExists(db, "ai_messages", "ui_blocks_json")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return db.Exec("ALTER TABLE ai_messages DROP COLUMN ui_blocks_json").Error
}

// ensureAllMembersOrg 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - uint：当前函数返回的处理结果。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func ensureAllMembersOrg(db *gorm.DB) (uint, error) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	var org entity.Org
	key := consts.OrgBuiltinKeyAllMembers
	queryDB := db.Session(&gorm.Session{NewDB: true}).Unscoped()

	if err := queryDB.
		Table("orgs").
		Select("id", "code", "deleted_at", "builtin_key").
		Where("builtin_key = ?", key).
		Order("id ASC").
		Limit(1).
		Scan(&org).Error; err != nil {
		return 0, err
	}
	if org.ID == 0 {
		if err := queryDB.
			Table("orgs").
			Select("id", "code", "deleted_at", "builtin_key").
			Where("name = ?", consts.OrgBuiltinNameAllMembers).
			Order("id ASC").
			Limit(1).
			Scan(&org).Error; err != nil {
			return 0, err
		}
	}
	if org.ID == 0 {
		// 未找到，创建内置组织
		code, genErr := generateAvailableOrgCodeFromDB(db)
		if genErr != nil {
			return 0, genErr
		}
		org = entity.Org{
			Name:        consts.OrgBuiltinNameAllMembers,
			Description: "系统内置组织：全体成员",
			Code:        code,
			OwnerID:     0,
			IsBuiltin:   true,
			BuiltinKey:  &key,
		}
		if createErr := db.Create(&org).Error; createErr != nil {
			return 0, createErr
		}
		return org.ID, nil
	}

	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	updates := map[string]any{
		"is_builtin": true,
	}
	if org.BuiltinKey == nil || strings.TrimSpace(*org.BuiltinKey) != key {
		updates["builtin_key"] = key
	}
	if org.Code == "" {
		code, genErr := generateAvailableOrgCodeFromDB(db)
		if genErr != nil {
			return 0, genErr
		}
		updates["code"] = code
	}
	if org.DeletedAt.Valid {
		updates["deleted_at"] = nil
	}
	if len(updates) > 0 {
		if err := queryDB.Model(&entity.Org{}).Where("id = ?", org.ID).Updates(updates).Error; err != nil {
			return 0, err
		}
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return org.ID, nil
}

// generateAvailableOrgCodeFromDB 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - string：当前函数生成或返回的字符串结果。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func generateAvailableOrgCodeFromDB(db *gorm.DB) (string, error) {
	for i := 0; i < 10; i++ {
		code := "ORG-" + strings.ToUpper(strings.ReplaceAll(uuid.Must(uuid.NewV4()).String(), "-", ""))[:10]
		var count int64
		if err := db.Unscoped().Model(&entity.Org{}).Where("code = ?", code).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("failed to generate org code from db")
}

// backfillOrgMembersFromRoleRelations 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func backfillOrgMembersFromRoleRelations(db *gorm.DB) error {
	return db.Exec(`
		INSERT INTO org_members (
			org_id, user_id, member_status, joined_at, join_source, created_at, updated_at
		)
		SELECT src.org_id, src.user_id, ?, NOW(), ?, NOW(), NOW()
		FROM (
			SELECT DISTINCT org_id, user_id
			FROM user_org_roles
		) src
		LEFT JOIN org_members om
			ON om.org_id = src.org_id AND om.user_id = src.user_id
		WHERE om.id IS NULL
	`, consts.OrgMemberStatusActive, consts.OrgMemberJoinSourceLegacyBackfill).Error
}

// backfillUsersByCurrentOrgToOrgMembers 将历史 users.current_org_id 下的用户补齐到指定组织成员表。
// 该迁移使用唯一索引做幂等约束；若成员已存在，则仅恢复为 active 并清理离开/移除态。
func backfillUsersByCurrentOrgToOrgMembers(
	db *gorm.DB,
	sourceCurrentOrgID, targetOrgID uint,
	joinSource consts.OrgMemberJoinSource,
) error {
	return db.Exec(`
		INSERT INTO org_members (
			org_id, user_id, member_status, joined_at, join_source, created_at, updated_at
		)
		SELECT ?, u.id, ?, NOW(), ?, NOW(), NOW()
		FROM users u
		WHERE u.deleted_at IS NULL AND u.current_org_id = ?
		ON DUPLICATE KEY UPDATE
			member_status = VALUES(member_status),
			left_at = NULL,
			removed_at = NULL,
			removed_by = NULL,
			remove_reason = '',
			join_source = VALUES(join_source),
			updated_at = VALUES(updated_at)
	`, targetOrgID, consts.OrgMemberStatusActive, string(joinSource), sourceCurrentOrgID).Error
}

// getMemberRoleID 负责执行当前函数对应的核心逻辑。
// 参数：
//   - db：调用方传入的目标对象或配置实例。
//
// 返回值：
//   - uint：当前函数返回的处理结果。
//   - error：处理失败原因；成功时为 nil。
//
// 核心流程：
//  1. 根据当前输入整理本函数需要的上下文、默认值或依赖。
//  2. 执行该函数对应的核心职责，并把结果传递给下一层或调用方。
//
// 注意事项：
//   - 具体细节需结合函数体与调用方一起理解；当前注释基于函数命名和上下文整理。
func getMemberRoleID(db *gorm.DB) (uint, error) {
	var memberRole entity.Role
	if err := db.Session(&gorm.Session{NewDB: true}).
		Where("code = ?", consts.RoleCodeMember).
		First(&memberRole).Error; err != nil {
		return 0, err
	}
	return memberRole.ID, nil
}

// migrateLegacyUserRoleToUserOrgRole 将历史 user_roles 中的成员角色映射到 user_org_roles。
// 仅迁移当前仍存在且未软删除的用户，避免把脏历史数据继续带入新模型。
func migrateLegacyUserRoleToUserOrgRole(
	db *gorm.DB,
	sourceRoleID, targetOrgID, targetRoleID uint,
) error {
	return db.Exec(`
		INSERT IGNORE INTO user_org_roles (user_id, org_id, role_id, created_at, updated_at)
		SELECT DISTINCT ur.user_id, ?, ?, NOW(), NOW()
		FROM user_roles ur
		INNER JOIN users u ON u.id = ur.user_id
		WHERE ur.deleted_at IS NULL
			AND ur.role_id = ?
			AND u.deleted_at IS NULL
	`, targetOrgID, targetRoleID, sourceRoleID).Error
}
