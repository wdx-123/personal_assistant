package casbin

import (
	"fmt"
	"github.com/casbin/casbin/v2"
	"personal_assistant/global"
	"strconv"
)

type Service struct {
	Enforcer *casbin.Enforcer
}

// NewCasbinService 创建casbin服务实例
func NewCasbinService() *Service {
	return &Service{
		Enforcer: global.CasbinEnforcer,
	}
}

// VerifySuper 验证用户是否是超级管理员
func (c *Service) VerifySuper(userID string, role string) (bool, error) {
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return false, fmt.Errorf("VerifySuper() 策略加载失败 err: %w", err)
	}
	// 验证用户是否是超级管理员
	ok, err := c.Enforcer.HasRoleForUser(userID, role)
	if err != nil {
		return false, fmt.Errorf("VerifySuper() 验证用户是否是超级管理员异常 err： %w", err)
	}
	return ok, nil
}

// GetPermByRole 获取角色对应的api ID
func (c *Service) GetPermByRole(role string) ([]uint, error) {
	var apiIDs []uint
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return nil, fmt.Errorf("GetPermByRole() 策略加载失败 err: %w", err)
	}
	permissions := c.Enforcer.GetPermissionsForUser(role)
	// 获取对应的api ID
	for _, p := range permissions {
		// 取出apiID
		id := p[1]
		// 转换为uint
		apiID, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("GetPermByRole() 字符串转int64失败 err: %w", err)
		}
		apiIDs = append(apiIDs, uint(apiID))
	}
	return apiIDs, nil
	/*
		GetPermissionsForUser 返回了一个二维数组
			角色     API_ID
			admin    1
			admin    2
			user     3
	*/
}

// ModifyPermByRole 修改角色权限
func (c *Service) ModifyPermByRole(apiIDs []uint, role string) error {
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return fmt.Errorf("ModifyPermByRole() 策略加载失败 err: %w", err)
	}
	// 删除该角色的权限
	_, err = c.Enforcer.DeletePermissionsForUser(role)
	if err != nil {
		return fmt.Errorf("ModifyPermByRole() 删除角色所属权限失败 err: %w", err)
	}
	for _, apiID := range apiIDs {
		// 给该角色添加权限
		_, err = c.Enforcer.AddPolicy(role, strconv.Itoa(int(apiID)))
		if err != nil {
			return fmt.Errorf("ModifyPermByRole() 添加角色权限失败 err: %w", err)
		}
	}
	// 存储新的api权限
	err = c.Enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("ModifyPermByRole() 策略存储失败 err: %w", err)
	}
	return nil
}

// DeletePermByRole 删除角色权限
// 删除角色的权限-影响所有拥有该角色的用户
func (c *Service) DeletePermByRole(roles []string) error {
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return fmt.Errorf("DeletePermByRole() 策略加载失败 err: %w", err)
	}
	for _, role := range roles {
		_, err = c.Enforcer.DeletePermissionsForUser(role)
		if err != nil {
			return fmt.Errorf("DeletePermByRole() 删除角色权限失败 err: %w", err)
		}
	}
	err = c.Enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("DeletePermByRole() 策略存储失败 err: %w", err)
	}
	return nil
}

// RemoveRoleByID 根据用户ID删除角色
// 删除用户的角色-只影响指定的用户
func (c *Service) RemoveRoleByID(userIDs []string) error {
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return fmt.Errorf("RemoveRoleByID() 策略加载失败 err: %w", err)
	}
	for _, userID := range userIDs {
		_, err = c.Enforcer.DeleteRolesForUser(userID)
		if err != nil {
			return fmt.Errorf("RemoveRoleByID() 删除用户角色失败 err: %w", err)
		}
	}
	err = c.Enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("RemoveRoleByID() 策略存储失败 err: %w", err)
	}
	return nil
}

// ModifyRoleByID 修改用户角色
func (c *Service) ModifyRoleByID(userID string, roles []string) error {
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return fmt.Errorf("ModifyRoleByID() 策略加载失败 err: %w", err)
	}
	// 删除用户的所有角色
	_, err = c.Enforcer.DeleteRolesForUser(userID)
	if err != nil {
		return fmt.Errorf("ModifyRoleByID() 删除用户角色失败 err: %w", err)
	}
	// 添加新角色
	for _, role := range roles {
		_, err = c.Enforcer.AddRoleForUser(userID, role)
		if err != nil {
			return fmt.Errorf("ModifyRoleByID() 添加用户角色失败 err: %w", err)
		}
	}
	err = c.Enforcer.SavePolicy()
	if err != nil {
		return fmt.Errorf("ModifyRoleByID() 策略存储失败 err: %w", err)
	}
	return nil
}

// CheckPermission 检查权限
func (c *Service) CheckPermission(userID string, apiID string) (bool, error) {
	err := c.Enforcer.LoadPolicy()
	if err != nil {
		return false, fmt.Errorf("CheckPermission() 策略加载失败 err: %w", err)
	}

	ok, err := c.Enforcer.Enforce(userID, apiID)
	if err != nil {
		return false, fmt.Errorf("CheckPermission() 权限检查失败 err: %w", err)
	}
	return ok, nil
}
