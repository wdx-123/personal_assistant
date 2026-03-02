package config

import (
	"fmt"
)

// System 系统配置
type System struct {
	Host           string `json:"-" yaml:"host"`                          // 服务器绑定的主机地址，通常为 0.0.0.0 表示监听所有可用地址
	Port           int    `json:"-" yaml:"port"`                          // 服务器监听的端口号，通常用于 HTTP 服务
	Env            string `json:"-" yaml:"env"`                           // Gin 的环境类型，例如 "debug"、"release" 或 "test"
	RouterPrefix   string `json:"-" yaml:"router_prefix"`                 // API 路由前缀，用于构建 API 路径
	AutoMigrate    bool   `json:"auto_migrate" yaml:"auto_migrate"`       // 是否允许自动迁移（仅开发环境生效）
	UseMultipoint  bool   `json:"use_multipoint" yaml:"use_multipoint"`   // 是否启用多点登录拦截，防止同一账户在多个地方同时登录
	SessionsSecret string `json:"sessions_secret" yaml:"sessions_secret"` // 用于加密会话的密钥，确保会话数据的安全性

	// 角色配置相关
	DefaultRoleCode string `json:"default_role_code" yaml:"default_role_code"` // 新用户注册时的默认角色代码，如 "user"
	DefaultRoleName string `json:"default_role_name" yaml:"default_role_name"` // 默认角色的显示名称，用于日志和错误提示

	// 业务逻辑相关
	BindCoolDownHours int `json:"bind_cool_down_hours" yaml:"bind_cool_down_hours"` // 换绑冷却时间（小时），防止频繁换绑
}

// Addr 服务器监听地址（主机:端口号）
func (s System) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// 存储驱动由 storage.current 决定，这里不再提供映射方法。
