package repository

import (
	"context"
	"errors"

	"personal_assistant/internal/repository/adapter"
	"personal_assistant/internal/repository/system"
)

type TxRunner interface {
	InTx(ctx context.Context, fn func(tx any) error) error
}

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

func (g *Group) InTx(ctx context.Context, fn func(tx any) error) error {
	if g == nil || g.SystemRepositorySupplier == nil {
		return errors.New("repository group is nil")
	}
	txRunner, ok := g.SystemRepositorySupplier.(TxRunner)
	if !ok {
		return errors.New("repository supplier does not support transactions")
	}
	return txRunner.InTx(ctx, fn)
}

func (g *Group) Ping(ctx context.Context) error {
	if g == nil || g.SystemRepositorySupplier == nil {
		return errors.New("repository group is nil")
	}
	pinger, ok := g.SystemRepositorySupplier.(interface {
		Ping(context.Context) error
	})
	if !ok {
		return errors.New("repository supplier does not support ping")
	}
	return pinger.Ping(ctx)
}

var _ TxRunner = (*Group)(nil)

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
