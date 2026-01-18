package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// APIRepository API仓储接口
type APIRepository interface {
	// 基础CRUD操作
	GetByID(ctx context.Context, id uint) (*entity.API, error)
	GetByPathAndMethod(ctx context.Context, path, method string) (*entity.API, error)
	Create(ctx context.Context, api *entity.API) error
	Update(ctx context.Context, api *entity.API) error
	Delete(ctx context.Context, id uint) error

	// 业务相关查询
	GetAPIList(ctx context.Context, page, pageSize int) ([]*entity.API, int64, error)
	GetAllAPIs(ctx context.Context) ([]*entity.API, error)
	GetAPIsByGroup(ctx context.Context, groupID uint) ([]*entity.API, error)
	GetActiveAPIs(ctx context.Context) ([]*entity.API, error)
	ExistsByPathAndMethod(ctx context.Context, path, method string) (bool, error)

	// 权限查询
	GetAPIsByUserID(ctx context.Context, userID uint) ([]*entity.API, error)
	GetAPIsByRoleID(ctx context.Context, roleID uint) ([]*entity.API, error)
	CheckUserAPIPermission(ctx context.Context, userID uint, path, method string) (bool, error)
}
