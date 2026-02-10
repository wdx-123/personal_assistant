package middleware

import (
	"context"
	"errors"
	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/service"
	"personal_assistant/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// PermissionMiddleware 权限验证中间件
type PermissionMiddleware struct {
	serviceGroup *service.Group
	whiteList    map[string]bool
	timeout      time.Duration
}

// NewPermissionMiddleware 创建权限中间件
func NewPermissionMiddleware(serviceGroup *service.Group) *PermissionMiddleware {
	// 初始化白名单路由
	whiteList := map[string]bool{
		"POST:/api/v1/auth/login":    true,
		"POST:/api/v1/auth/register": true,
		"GET:/api/v1/health":         true,
		"GET:/api/v1/ping":           true,
		"GET:/api/v1/public/*":       true, // 公共资源
	}

	return &PermissionMiddleware{
		serviceGroup: serviceGroup,
		whiteList:    whiteList,
		timeout:      5 * time.Second, // 权限检查超时时间
	}
}

// CheckPermission 权限检查中间件函数
func (p *PermissionMiddleware) CheckPermission() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 创建超时的上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), p.timeout)
		defer cancel()

		auth := &permissionAuth{
			c:            c,
			ctx:          ctx,
			serviceGroup: p.serviceGroup,
			whiteList:    p.whiteList,
		}

		// 链式验证流程
		switch {
		// 白名单路由直接通过
		case !auth.checkWhiteList():
			return
			// 获取用户信息
		case !auth.extractUserInfo():
			return
			// 检查是否为超级管理员
		case !auth.checkSuperUser():
			return
			// 检查API权限
		case !auth.checkAPIPermission():
			return
		default:
			// 权限验证通过
			global.Log.Debug("权限验证通过",
				zap.Uint("userID", auth.userID),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method))
		}

		c.Next()

	}
}

// permissionAuth 权限验证辅助结构体
type permissionAuth struct {
	c            *gin.Context
	ctx          context.Context
	serviceGroup *service.Group
	whiteList    map[string]bool
	userID       uint
	userRoles    []string // 用户角色列表
	isSuperAdmin bool     // 是否为超级管理员
}

// checkWhiteList 检查白名单
func (a *permissionAuth) checkWhiteList() bool {
	routeKey := a.c.Request.Method + ":" + a.c.Request.URL.Path

	// 精确匹配
	if a.whiteList[routeKey] {
		a.c.Next()
		return false
	}
	// 通配符匹配
	for pattern := range a.whiteList {
		if matchPattern(pattern, routeKey) {
			a.c.Next()
			return false
		}
	}
	return true // 需要继续验证
}

// extractUserInfo 提取用户信息
func (a *permissionAuth) extractUserInfo() bool {
	// 从JWT中间件获取用户信息
	userClaims, exists := a.c.Get("claims")
	if !exists {
		global.Log.Warn("权限验证失败：未找到用户信息",
			zap.String("path", a.c.Request.URL.Path),
			zap.String("method", a.c.Request.Method))
		response.NewResponse[any, any](a.c).
			SetCode(global.StatusUnauthorized).
			Failed("用户未登录", errors.New("用户未登录"))

		a.c.Abort()
		return false
	}

	// 类型断言获取用户信息
	claims, ok := userClaims.(*request.JwtCustomClaims)
	if !ok {
		global.Log.Error("权限验证失败：用户信息类型错误")
		response.NewResponse[any, any](a.c).
			SetCode(global.StatusUnauthorized).
			Failed("用户无权限", errors.New("用户无权限"))
		a.c.Abort()
		return false
	}

	a.userID = claims.UserID
	// 获取用户角色信息
	return a.loadUserRoles()
}

// loadUserRoles 加载用户角色信息
func (a *permissionAuth) loadUserRoles() bool {
	permissionService := a.serviceGroup.SystemServiceSupplier.GetPermissionSvc()

	// 获取用户角色列表
	roles, err := permissionService.GetUserRoles(a.ctx, a.userID)
	if err != nil {
		global.Log.Error("获取用户角色失败",
			zap.Uint("userID", a.userID),
			zap.Error(err))
		response.NewResponse[any, any](a.c).
			SetCode(global.StatusInternalServerError).
			Failed("获取用户角色失败", errors.New("获取用户角色失败"))
		a.c.Abort()
		return false
	}

	// 提取角色代码并检查是否为超级管理员
	a.userRoles = make([]string, len(roles))
	for i, role := range roles {
		a.userRoles[i] = role.Code
		// 检查是否包含超级管理员角色
		if role.Code == consts.RoleCodeSuperAdmin || role.Code == "SuperAdmin" {
			a.isSuperAdmin = true
		}
	}

	global.Log.Debug("用户角色信息加载完成",
		zap.Uint("userID", a.userID),
		zap.Strings("roles", a.userRoles),
		zap.Bool("isSuperAdmin", a.isSuperAdmin))

	return true
}

// checkSuperUser 检查超级用户
func (a *permissionAuth) checkSuperUser() bool {
	if a.isSuperAdmin {
		global.Log.Debug("超级管理员访问",
			zap.Uint("userID", a.userID),
			zap.String("path", a.c.Request.URL.Path),
			zap.Strings("roles", a.userRoles))
		a.c.Next()
		return false // 超级管理员跳过后续检查
	}
	return true // 普通用户继续检查
}

// checkAPIPermission 检查API权限
func (a *permissionAuth) checkAPIPermission() bool {
	permissionService := a.serviceGroup.SystemServiceSupplier.GetPermissionSvc()

	// 使用FullPath获取路由模板
	apiPath := a.c.FullPath()
	if apiPath == "" {
		apiPath = a.c.Request.URL.Path
	}

	// 检查用户API权限
	hasPermission, err := permissionService.CheckUserAPIPermission(
		a.userID,
		apiPath,
		a.c.Request.Method,
	)

	if err != nil {
		global.Log.Error("API权限验证失败",
			zap.Uint("userID", a.userID),
			zap.String("path", apiPath),
			zap.String("method", a.c.Request.Method),
			zap.Strings("roles", a.userRoles),
			zap.Error(err))
		response.NewResponse[any, any](a.c).
			SetCode(global.StatusInternalServerError).
			Failed("API权限验证失败", errors.New("API权限验证失败"))
		a.c.Abort()
		return false
	}

	if !hasPermission {
		global.Log.Warn("用户无权限访问API",
			zap.Uint("userID", a.userID),
			zap.String("path", apiPath),
			zap.String("method", a.c.Request.Method),
			zap.Strings("roles", a.userRoles))
		response.NewResponse[any, any](a.c).
			SetCode(global.StatusUnauthorized).
			Failed("用户无权限访问API", errors.New("用户无权限访问API"))
		a.c.Abort()
		return false
	}

	return true
}

// matchPattern 通配符匹配
func matchPattern(pattern, target string) bool {
	// 简单的通配符匹配实现
	// 支持 /* 结尾的模式
	if len(pattern) > 2 && pattern[len(pattern)-2:] == "/*" {
		prefix := pattern[:len(pattern)-2]
		return len(target) >= len(prefix) && target[:len(prefix)] == prefix
	}
	return pattern == target
}

// 中间件配置方法

// AddWhiteListRoute 添加白名单路由
func (p *PermissionMiddleware) AddWhiteListRoute(method, path string) {
	routeKey := method + ":" + path
	p.whiteList[routeKey] = true
}

// RemoveWhiteListRoute 移除白名单路由
func (p *PermissionMiddleware) RemoveWhiteListRoute(method, path string) {
	routeKey := method + ":" + path
	delete(p.whiteList, routeKey)
}

// SetTimeout 设置权限检查超时时间
func (p *PermissionMiddleware) SetTimeout(timeout time.Duration) {
	p.timeout = timeout
}

// GetWhiteList 获取白名单列表（用于调试）
func (p *PermissionMiddleware) GetWhiteList() map[string]bool {
	result := make(map[string]bool)
	for k, v := range p.whiteList {
		result[k] = v
	}
	return result
}
