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
		&entity.User{},                   // 用户表
		&entity.Org{},                    // 组织表
		&entity.OrgMember{},              // 组织成员状态表 - 身份上的
		&entity.LeetcodeUserDetail{},     // 力扣用户详情表
		&entity.LuoguUserDetail{},        // 洛谷用户详情表
		&entity.LeetcodeQuestionBank{},   // 力扣题库题目表
		&entity.LuoguQuestionBank{},      // 洛谷题库题目表
		&entity.LeetcodeUserQuestion{},   // 力扣用户以做题目表
		&entity.LuoguUserQuestion{},      // 洛谷用户以做题目表
		&entity.OJUserDailyStat{},        // OJ 刷题曲线日聚合读模型
		&entity.Login{},                  // 登录日志表
		&entity.UserToken{},              // 用户Token记录表
		&entity.TokenBlacklist{},         // Token黑名单表
		&entity.JwtBlacklist{},           // JWT黑名单表（兼容现有代码）
		&entity.Role{},                   // 角色表
		&entity.Capability{},             // 业务能力表
		&entity.Menu{},                   // 菜单表
		&entity.UserOrgRole{},            // 用户组织角色关联表 - 权限上的（与 OrgMember 配合）
		&entity.RoleCapability{},         // 角色与业务能力关联表
		&entity.RoleAPI{},                // 角色API直绑关联表
		&entity.API{},                    // api表
		&entity.OutboxEvent{},            // Outbox事件表
		&entity.Image{},                  // 图片表
		&entity.ObservabilityMetric{},    // 指标聚合表
		&entity.ObservabilityTraceSpan{}, // 全链路追踪明细表
	); err != nil {
		return err
	}

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
	if err := seedBuiltinCapabilities(); err != nil {
		return err
	}
	if err := seedBuiltinRoleCapabilities(); err != nil {
		return err
	}

	return migrateOrgMemberLifecycleData(db)
}

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

// seedBuiltinCapabilities 初始化系统内置 capability（幂等）。
func seedBuiltinCapabilities() error {
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
	return nil
}

// seedBuiltinRoleCapabilities 确保组织管理员默认持有全部组织域 capability。
func seedBuiltinRoleCapabilities() error {
	var orgAdmin entity.Role
	if err := global.DB.Where("code = ?", consts.RoleCodeOrgAdmin).First(&orgAdmin).Error; err != nil {
		return err
	}

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
	return nil
}

// migrateOrgMemberLifecycleData 迁移组织成员生命周期相关的数据和结构，确保数据一致性和完整性
func migrateOrgMemberLifecycleData(db *gorm.DB) error {
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
	if err := ensureColumn(db, &entity.Org{}, "IsBuiltin"); err != nil {
		return err
	}
	if err := ensureColumn(db, &entity.Org{}, "BuiltinKey"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(db *gorm.DB, model any, field string) error {
	if db.Migrator().HasColumn(model, field) {
		return nil
	}
	return db.Migrator().AddColumn(model, field)
}

func normalizeOrgInviteCodes(db *gorm.DB) error {
	var orgs []entity.Org
	if err := db.Unscoped().Select("id", "code").Order("id ASC").Find(&orgs).Error; err != nil {
		return err
	}

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
	return nil
}

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

func normalizeUserStatusWithFreeze(db *gorm.DB) error {
	if err := ensureUsersStatusColumnsBeforeNormalize(db); err != nil {
		return fmt.Errorf("ensure users lifecycle columns failed: %w", err)
	}

	if err := db.Model(&entity.User{}).
		Where("status = 0").
		Update("status", consts.UserStatusActive).Error; err != nil {
		return err
	}

	hasFreeze, err := columnExists(db, "users", "freeze")
	if err != nil {
		return err
	}
	if !hasFreeze {
		return nil
	}

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
	return nil
}

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

func cleanupSoftDeletedUserOrgRoles(db *gorm.DB) error {
	return db.Exec("DELETE FROM user_org_roles WHERE deleted_at IS NOT NULL").Error
}

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

func indexExists(db *gorm.DB, tableName, indexName string) (bool, error) {
	type indexCount struct {
		Count int64 `gorm:"column:count"`
	}
	var result indexCount
	err := db.Raw(
		`SELECT COUNT(1) AS count
		   FROM information_schema.statistics
		  WHERE table_schema = DATABASE()
			AND table_name = ?
			AND index_name = ?`,
		tableName,
		indexName,
	).Scan(&result).Error
	if err != nil {
		return false, err
	}
	return result.Count > 0, nil
}

func columnExists(db *gorm.DB, tableName, columnName string) (bool, error) {
	type columnCount struct {
		Count int64 `gorm:"column:count"`
	}
	var result columnCount
	err := db.Raw(
		`SELECT COUNT(1) AS count
		   FROM information_schema.columns
		  WHERE table_schema = DATABASE()
			AND table_name = ?
			AND column_name = ?`,
		tableName,
		columnName,
	).Scan(&result).Error
	if err != nil {
		return false, err
	}
	return result.Count > 0, nil
}

func ensureAllMembersOrg(db *gorm.DB) (uint, error) {
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
	return org.ID, nil
}

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
