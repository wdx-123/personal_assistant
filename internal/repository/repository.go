package repository

import (
	"personal_assistant/internal/repository/adapter"
	"personal_assistant/internal/repository/system"
)

type Group struct {
	SystemRepositorySupplier system.Supplier
}

var GroupApp *Group

// InitRepositoryGroupWithAdapter 直接使用适配器初始化Repository组
func InitRepositoryGroupWithAdapter(dbAdapter adapter.DatabaseAdapter) {
	factoryConfig := dbAdapter.GetFactoryConfig()
	GroupApp = &Group{
		SystemRepositorySupplier: system.SetUp(factoryConfig),
	}
}

/*// InitRepositoryGroup 初始化Repository组 - 兼容现有代码
func InitRepositoryGroup() {
	// 为了兼容现有代码，默认使用MySQL
	mysqlAdapter := &adapter.MySQLAdapter{}
	mysqlAdapter.SetConnection(global.DB) // 使用现有的全局DB连接

	factoryConfig := mysqlAdapter.GetFactoryConfig()

	GroupApp = &Group{
		SystemRepositorySupplier: system.SetUp(factoryConfig),
	}
}

// InitRepositoryGroupWithConfig 使用配置初始化Repository组
func InitRepositoryGroupWithConfig(dbConfig *adapter.DatabaseConfig) error {
	// 创建数据库适配器
	dbAdapter := adapter.CreateDatabaseAdapter(dbConfig.Type)

	// 连接数据库
	_, err := dbAdapter.Connect(dbConfig)
	if err != nil {
		return err
	}

	// 获取工厂配置
	factoryConfig := dbAdapter.GetFactoryConfig()

	// 创建Repository组
	GroupApp = &Group{
		SystemRepositorySupplier: system.SetUp(factoryConfig),
	}

	return nil
}*/
