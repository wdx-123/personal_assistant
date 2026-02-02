package interfaces

import (
	"context"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
)

// APIRepository API仓储接口
type APIRepository interface {
	// GetByID 根据ID获取API
	GetByID(ctx context.Context, id uint) (*entity.API, error)
	// GetByPathAndMethod 根据路径和方法获取API
	GetByPathAndMethod(ctx context.Context, path, method string) (*entity.API, error)
	// Create 创建API
	Create(ctx context.Context, api *entity.API) error
	// Update 更新API
	Update(ctx context.Context, api *entity.API) error
	// Delete 删除API
	Delete(ctx context.Context, id uint) error

	// GetAPIList 获取API列表（分页，支持过滤）
	GetAPIList(ctx context.Context, filter *request.ApiListFilter) ([]*entity.API, int64, error)
	// GetAllAPIs 获取所有API
	GetAllAPIs(ctx context.Context) ([]*entity.API, error)
	// GetAPIsByGroup 根据分组获取API
	GetAPIsByGroup(ctx context.Context, groupID uint) ([]*entity.API, error)
	// GetActiveAPIs 获取所有启用的API
	GetActiveAPIs(ctx context.Context) ([]*entity.API, error)
	// ExistsByPathAndMethod 检查路径和方法组合是否存在
	ExistsByPathAndMethod(ctx context.Context, path, method string) (bool, error)

	// GetAPIsByUserID 获取用户在组织内的API权限列表
	GetAPIsByUserID(ctx context.Context, userID, orgID uint) ([]*entity.API, error)
	// GetAPIsByRoleID 获取角色的API权限列表
	GetAPIsByRoleID(ctx context.Context, roleID uint) ([]*entity.API, error)
	// CheckUserAPIPermission 检查用户在组织内是否有特定API权限
	CheckUserAPIPermission(ctx context.Context, userID, orgID uint, path, method string) (bool, error)
}
