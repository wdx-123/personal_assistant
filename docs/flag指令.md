# Flag 模块应用文档

## 📋 概述

Flag 模块是 Go Blog 项目的命令行工具集，提供了数据库管理、数据导入导出等功能。该模块基于 `github.com/urfave/cli` 构建，支持多种命令行操作。

## 🏗️ 模块结构

```
flag/
├── enter.go           # CLI 应用程序入口和路由
├── flag_sql.go        # 数据库表结构初始化
├── flag_sql_export.go # MySQL 数据导出
├── flag_sql_import.go # MySQL 数据导入
```

## 🚀 使用方法

### 🔨 构建项目（必须先执行）
```bash
# 构建可执行文件
go build -o blog-service.exe cmd/main.go
```

### 基本语法
```bash
# Windows
.\blog-service.exe [命令]

# Linux/macOS
./blog-service [命令]
```

### 可用命令

#### 1. 数据库管理

##### 🔧 初始化数据库表结构
```bash
.\blog-service.exe --sql
```
- **功能**: 自动创建/更新数据库表结构
- **说明**: 使用 GORM AutoMigrate 功能，根据实体模型创建表
- **支持的表**:
  - `users` - 用户基础信息表
  - `logins` - 登录日志表
  - `user_tokens` - 用户Token记录表
  - `token_blacklists` - Token黑名单表
  - `jwt_blacklists` - JWT黑名单表

##### 📤 导出数据库数据
```bash
.\blog-service.exe --sql-export
```
- **功能**: 导出 MySQL 数据库数据到 SQL 文件
- **输出文件**: `mysql_YYYYMMDD.sql` (按日期命名)
- **依赖**: 需要 Docker 环境，容器名为 `mysql`
- **导出内容**: 完整的数据库结构和数据

##### 📥 导入数据库数据
```bash
.\blog-service.exe --sql-import C:\path\to\file.sql
```
- **功能**: 从 SQL 文件导入数据到数据库
- **参数**: SQL 文件的完整路径
- **处理方式**: 逐条执行 SQL 语句
- **错误处理**: 收集所有执行错误并统一报告

#### 2. 管理员管理

##### 👤 创建管理员
```bash
.\blog-service.exe --admin
```
- **功能**: 创建系统管理员账户
- **配置**: 使用 `configs.yaml` 中的配置信息
- **状态**: 🚧 待实现 (标志已定义，功能待开发)

### 帮助信息
```bash
.\blog-service.exe -h
.\blog-service.exe --help
```

## ⚙️ 配置要求

### 数据库配置
确保 `configs.yaml` 中包含正确的数据库配置：
```yaml
mysql:
  host: 127.0.0.1
  port: 3306
  username: root
  password: root
  db_name: blog_db
```

### Docker 环境
数据导出功能需要 Docker 环境，确保：
- Docker 已安装并运行
- MySQL 容器名为 `mysql`
- 容器可以正常访问

## 🔒 安全注意事项

1. **权限控制**: 确保只有授权用户可以执行数据库操作
2. **备份策略**: 在执行导入操作前，建议先备份现有数据
3. **文件路径**: 导入时使用绝对路径，避免路径错误
4. **敏感信息**: 导出的 SQL 文件可能包含敏感数据，注意保护

## 🐛 错误处理

### 常见错误及解决方案

#### 1. 数据库连接失败
```
Failed to create table structure: dial tcp connection refused
```
**解决方案**: 检查数据库服务是否启动，配置是否正确

#### 2. Docker 容器不存在
```
Failed to export SQL data: No such container: mysql
```
**解决方案**: 确保 MySQL Docker 容器正在运行

#### 3. SQL 文件格式错误
```
Failed to import SQL data: syntax error
```
**解决方案**: 检查 SQL 文件格式，确保语法正确

#### 4. 多命令冲突
```
Only one command can be specified
```
**解决方案**: 一次只能执行一个命令，不要同时使用多个标志

## 📝 使用示例

### 完整的数据库初始化流程
```bash
# 0. 构建项目
go build -o blog-service.exe cmd/main.go

# 1. 初始化表结构
.\blog-service.exe --sql

# 2. 导出当前数据（备份）
.\blog-service.exe --sql-export

# 3. 导入历史数据（如需要）
.\blog-service.exe --sql-import backup_20240101.sql
```

### 开发环境快速设置
```bash
# 构建项目
go build -o blog-service.exe cmd/main.go

# 创建数据库表
.\blog-service.exe --sql

# 创建管理员账户（待实现）
.\blog-service.exe --admin
```

## 🔄 扩展开发

### 添加新命令
1. 在 `enter.go` 中定义新的 Flag
2. 在 `Run` 函数中添加处理逻辑
3. 创建对应的功能函数
4. 更新文档

### 示例：添加新的导出格式
```go
// 在 enter.go 中添加
jsonExportFlag = &cli.BoolFlag{
    Name:  "json-export",
    Usage: "Exports data to JSON format.",
}

// 在 Run 函数中添加处理
case c.Bool(jsonExportFlag.Name):
    if err := JSONExport(); err != nil {
        global.Log.Error("Failed to export JSON data:", zap.Error(err))
    }
```

## 📞 技术支持

如果在使用过程中遇到问题，请：
1. 检查日志文件 `log/go_blog.log`
2. 确认配置文件 `configs.yaml` 设置正确
3. 验证依赖服务（MySQL、Docker）状态
4. 查看错误信息并参考本文档的错误处理部分

---

*最后更新: 2025年10月*
