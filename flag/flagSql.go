package flag

import (
	"personal_assistant/global"
	"personal_assistant/internal/model/entity"
)

// SQL 表结构迁移，如果表不存在，它会创建新表；如果表已经存在，它会根据结构更新表
func SQL() error {
	return global.DB.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&entity.User{}, // 用户表
		&entity.Org{},
		&entity.LeetcodeUserDetail{},
		&entity.LuoguUserDetail{},
		&entity.LeetcodeQuestionBank{},
		&entity.LuoguQuestionBank{},
		&entity.LeetcodeUserQuestion{},
		&entity.LuoguUserQuestion{},
		&entity.Login{},          // 登录日志表
		&entity.UserToken{},      // 用户Token记录表
		&entity.TokenBlacklist{}, // Token黑名单表
		&entity.JwtBlacklist{},   // JWT黑名单表（兼容现有代码）
		&entity.Role{},           // 角色表
		&entity.Menu{},           // 菜单表
		&entity.UserOrgRole{},    // 用户组织角色关联表
		&entity.API{},            // api表
		&entity.OutboxEvent{},    // Outbox事件表
	)
}
