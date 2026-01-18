package core

import (
	"go.uber.org/zap"
	"personal_assistant/global"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
)

// InitCasbin 初始化casbin
func InitCasbin() {
	db := global.DB
	// 确保数据库连接有效
	if db == nil {
		global.Log.Panic("数据库连接失败")
		return
	}

	// 创建适配器
	a, err := gormadapter.NewAdapterByDB(db)
	if err != nil || a == nil {
		global.Log.Panic("适配器创建失败", zap.Error(err))
		return
	}

	// 加载模型配置
	m, err := model.NewModelFromFile("configs/model.conf")
	if err != nil || m == nil {
		global.Log.Panic("模型加载失败", zap.Error(err))
		return
	}

	// 创建执行器
	e, err := casbin.NewEnforcer(m, a)
	if err != nil {
		global.Log.Panic("执行器创建失败", zap.Error(err))
		return
	}

	// 启用自动保存策略
	e.EnableAutoSave(true)

	global.CasbinEnforcer = e
	global.Log.Info("Casbin 初始化成功")
}
