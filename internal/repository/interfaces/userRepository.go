package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	// GetByID 根据ID获取用户
	GetByID(ctx context.Context, id uint) (*entity.User, error)
	// GetByUsername 根据用户名获取用户
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	// GetByEmail 根据邮箱获取用户
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	// GetByPhone 根据手机号获取用户
	GetByPhone(ctx context.Context, phone string) (*entity.User, error)
	// GetByIDs 批量获取用户
	GetByIDs(ctx context.Context, ids []uint) ([]*entity.User, error)
	// Create 创建用户
	Create(ctx context.Context, user *entity.User) error
	// Update 更新用户
	Update(ctx context.Context, user *entity.User) error
	// Delete 删除用户
	Delete(ctx context.Context, id uint) error

	// GetUserList 获取用户列表（分页）
	GetUserList(ctx context.Context, page, pageSize int) ([]*entity.User, int64, error)
	// ExistsByUsername 检查用户名是否存在
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	// ExistsByEmail 检查邮箱是否存在
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	// ExistsByPhone 检查手机号是否存在
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
	// GetActiveUsers 获取所有活跃用户
	GetActiveUsers(ctx context.Context) ([]*entity.User, error)

	// ValidateUser 验证用户名和密码
	ValidateUser(ctx context.Context, username, password string) (*entity.User, error)
	// UpdateLastLogin 更新最后登录时间
	UpdateLastLogin(ctx context.Context, id uint) error

	// CheckEmailAddress 检查邮箱是否可用（排除自身）
	CheckEmailAddress(ctx context.Context, email string) error

	// UpdateCurrentOrgID 更新用户当前组织ID
	UpdateCurrentOrgID(ctx context.Context, userID uint, orgID *uint) error

	// WithTx 启用事务（返回支持事务的新实例）
	WithTx(tx any) UserRepository
}
