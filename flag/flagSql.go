package flag

import (
	"errors"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"

	"gorm.io/gorm"
)

// SQL 表结构迁移，如果表不存在，它会创建新表；如果表已经存在，它会根据结构更新表
// 迁移完成后自动初始化内置角色（幂等）
func SQL() error {
	if err := global.DB.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&entity.User{},                 // 用户表
		&entity.Org{},                  // 组织表
		&entity.LeetcodeUserDetail{},   // 力扣用户详情表
		&entity.LuoguUserDetail{},      // 洛谷用户详情表
		&entity.LeetcodeQuestionBank{}, // 力扣题库题目表
		&entity.LuoguQuestionBank{},    // 洛谷题库题目表
		&entity.LeetcodeUserQuestion{}, // 力扣用户以做题目表
		&entity.LuoguUserQuestion{},    // 洛谷用户以做题目表
		&entity.Login{},                // 登录日志表
		&entity.UserToken{},            // 用户Token记录表
		&entity.TokenBlacklist{},       // Token黑名单表
		&entity.JwtBlacklist{},         // JWT黑名单表（兼容现有代码）
		&entity.Role{},                 // 角色表
		&entity.Menu{},                 // 菜单表
		&entity.UserOrgRole{},          // 用户组织角色关联表
		&entity.RoleAPI{},              // 角色API直绑关联表
		&entity.API{},                  // api表
		&entity.OutboxEvent{},          // Outbox事件表
		&entity.Image{},                // 图片表
	); err != nil {
		return err
	}

	// 表结构就绪后，初始化内置角色
	return seedBuiltinRoles()
}

// seedBuiltinRoles 初始化内置角色（幂等：已存在则跳过，不会重复创建）
func seedBuiltinRoles() error {
	roles := []entity.Role{
		{Code: consts.RoleCodeSuperAdmin, Name: "系统超级管理员", Status: 1, Desc: "系统最高权限，管理所有组织和用户"},
		{Code: consts.RoleCodeOrgAdmin, Name: "组织管理员", Status: 1, Desc: "管理单个组织内的一切"},
		{Code: consts.RoleCodeMember, Name: "普通成员", Status: 1, Desc: "组织普通成员"},
	}

	for _, r := range roles {
		var existing entity.Role
		if err := global.DB.Where("code = ?", r.Code).First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 角色不存在，创建
				if createErr := global.DB.Create(&r).Error; createErr != nil {
					return createErr
				}
			} else {
				return err
			}
		}
		// 角色已存在，跳过
	}
	return nil
}
