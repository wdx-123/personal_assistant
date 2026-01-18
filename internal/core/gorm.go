package core

import (
	"go.uber.org/zap"
	"gorm.io/driver/mysql"

	// gorm.io/driver/mysql 是一个 GORM 的 MySQL 驱动包。
	// 它提供了与 MySQL 数据库的连接支持，允许 GORM 使用 MySQL 作为后端数据库。
	// 使用这个包，你可以通过 GORM 的 API 执行 MySQL 数据库的查询、插入、更新等操作。
	// 依赖于底层的 MySQL Go 驱动（如 go-sql-driver/mysql）。
	"gorm.io/gorm"
	// gorm.io/gorm 是 GORM 的核心库，提供了对象关系映射（ORM）的功能。
	// 它允许开发者通过 Go 结构体与数据库表进行映射，简化数据库操作。
	// 提供了创建、查询、更新、删除（CRUD）等功能，并支持模型定义、关联关系、事务等。
	// 这个包是 GORM 的核心，任何使用 GORM 的项目都需要导入它。
	"os"
	"personal_assistant/global"

	"gorm.io/gorm/logger"
)

func InitGorm() *gorm.DB {
	mysqlCfg := global.Config.Mysql
	// 使用给定的 DSN（数据源名称）和日志级别打开 MySQL 数据库连接
	db, err := gorm.Open(mysql.Open(mysqlCfg.Dsn()), &gorm.Config{
		Logger: logger.Default.LogMode(mysqlCfg.LogLevel()), // 设置日志级别
	})
	if err != nil {
		global.Log.Error("Failed to connect to MySQL:", zap.Error(err))
		os.Exit(1)
	}
	// 获取底层的 SQL 数据库连接对象
	sqlDB, _ := db.DB()
	// 设置数据库连接池中的最大空闲连接数
	sqlDB.SetMaxIdleConns(mysqlCfg.MaxIdleConns)
	// 设置数据库的最大打开连接数
	sqlDB.SetMaxOpenConns(mysqlCfg.MaxOpenConns)
	return db
}
