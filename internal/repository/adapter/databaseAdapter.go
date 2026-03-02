package adapter

import (
	"fmt"

	"gorm.io/gorm"
)

// DatabaseType 数据库类型枚举
type DatabaseType string

const (
	MySQL   DatabaseType = "mysql"
	MongoDB DatabaseType = "mongodb"
	Redis   DatabaseType = "redis"
)

// FactoryConfig 工厂配置
type FactoryConfig struct {
	DatabaseType DatabaseType
	Connection   interface{} // 可以是 *gorm.DB, *mongo.Database, *redis.Client 等
}

// DatabaseConfig 数据库配置结构
type DatabaseConfig struct {
	Type     DatabaseType `yaml:"type" json:"type"`
	Host     string       `yaml:"host" json:"host"`
	Port     int          `yaml:"port" json:"port"`
	Username string       `yaml:"username" json:"username"`
	Password string       `yaml:"password" json:"password"`
	Database string       `yaml:"database" json:"database"`
	// 其他配置项
}

// DatabaseAdapter 数据库适配器接口
type DatabaseAdapter interface {
	Connect(config *DatabaseConfig) (interface{}, error)
	Close() error
	GetFactoryConfig() *FactoryConfig

	CreateUserRepository() interface{} // 返回 interfaces.UserRepository
	CreateJWTRepository() interface{}  // 返回 interfaces.JWTRepository
}

// CreateDatabaseAdapter 根据数据库类型创建适配器
func CreateDatabaseAdapter(dbType DatabaseType) DatabaseAdapter {
	switch dbType {
	case MySQL:
		return &MySQLAdapter{}
	case MongoDB:
		return nil // 后续需要就模仿MongoDB
	case Redis:
		return nil
	default:
		return &MySQLAdapter{} // 默认MySQL
	}
}

// MySQLAdapter MySQL数据库适配器
type MySQLAdapter struct {
	db *gorm.DB
}

func (a *MySQLAdapter) Connect(config *DatabaseConfig) (interface{}, error) {
	// 这里应该是实际的MySQL连接逻辑
	// 为了演示，我们假设已经有了连接
	if a.db != nil {
		return a.db, nil
	}
	return nil, fmt.Errorf("MySQL connection not initialized")
}

func (a *MySQLAdapter) Close() error {
	if a.db != nil {
		sqlDB, err := a.db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

func (a *MySQLAdapter) GetFactoryConfig() *FactoryConfig {
	return &FactoryConfig{
		DatabaseType: MySQL,
		Connection:   a.db,
	}
}

// SetConnection 设置已有的数据库连接（用于兼容现有代码）
func (a *MySQLAdapter) SetConnection(db *gorm.DB) {
	a.db = db
}

// CreateUserRepository 创建用户Repository
func (a *MySQLAdapter) CreateUserRepository() interface{} {
	// 这里需要导入system包，但为了避免循环依赖，我们返回nil
	// 实际使用时会在supplier中处理
	return nil
}

// CreateJWTRepository 创建JWT Repository
func (a *MySQLAdapter) CreateJWTRepository() interface{} {
	// 这里需要导入system包，但为了避免循环依赖，我们返回nil
	// 实际使用时会在supplier中处理
	return nil
}
