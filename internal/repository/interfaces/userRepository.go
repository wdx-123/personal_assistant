package interfaces

import (
	"context"
	"personal_assistant/internal/model/entity"
)

// UserRepository 用户仓储接口
type UserRepository interface {
	// 基础CRUD操作
	GetByID(ctx context.Context, id uint) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	GetByPhone(ctx context.Context, phone string) (*entity.User, error)
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id uint) error

	// 业务相关查询
	GetUserList(ctx context.Context, page, pageSize int) ([]*entity.User, int64, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	ExistsByPhone(ctx context.Context, phone string) (bool, error)
	GetActiveUsers(ctx context.Context) ([]*entity.User, error)

	// 认证相关
	ValidateUser(ctx context.Context, username, password string) (*entity.User, error)
	UpdateLastLogin(ctx context.Context, id uint) error

	// CheckEmailAddress 检查邮箱是否会重复
	CheckEmailAddress(ctx context.Context, email string) error
}
