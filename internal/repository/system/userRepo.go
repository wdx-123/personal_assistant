package system

import (
	"context"
	"errors"

	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserGormRepository 用户仓储GORM实现
type UserGormRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓储实例，返回接口类型
func NewUserRepository(db *gorm.DB) interfaces.UserRepository {
	return &UserGormRepository{db: db}
}

// WithTx 启用事务
func (r *UserGormRepository) WithTx(tx any) interfaces.UserRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &UserGormRepository{db: transaction}
	}
	return r
}

// GetByID 根据ID获取用户
func (r *UserGormRepository) GetByID(
	ctx context.Context,
	id uint,
) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Preload("CurrentOrg").
		Where("id = ?", id).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (r *UserGormRepository) GetByUsername(
	ctx context.Context,
	username string,
) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Preload("CurrentOrg").
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *UserGormRepository) GetByEmail(
	ctx context.Context,
	email string,
) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Preload("CurrentOrg").
		Where("email = ?", email).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByPhone 根据手机号获取用户
func (r *UserGormRepository) GetByPhone(
	ctx context.Context,
	phone string,
) (*entity.User, error) {
	var user entity.User
	err := r.db.WithContext(ctx).
		Preload("CurrentOrg").
		Where("phone = ?", phone).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByIDs 批量获取用户
func (r *UserGormRepository) GetByIDs(
	ctx context.Context,
	ids []uint,
) ([]*entity.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []*entity.User
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// Create 创建用户
func (r *UserGormRepository) Create(
	ctx context.Context,
	user *entity.User,
) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// Update 更新用户
func (r *UserGormRepository) Update(
	ctx context.Context,
	user *entity.User,
) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete 删除用户（软删除）
func (r *UserGormRepository) Delete(
	ctx context.Context,
	id uint,
) error {
	return r.db.WithContext(ctx).Delete(&entity.User{}, id).Error
}

// GetUserList 获取用户列表（分页）
func (r *UserGormRepository) GetUserList(
	ctx context.Context,
	page,
	pageSize int,
) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64

	// 计算总数
	if err := r.db.WithContext(ctx).
		Model(&entity.User{}).
		Count(&total).
		Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	err := r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetUserListWithFilter 获取用户列表（支持过滤）
func (r *UserGormRepository) GetUserListWithFilter(
	ctx context.Context,
	req *request.UserListReq,
) ([]*entity.User, int64, error) {
	var users []*entity.User
	var total int64

	db := r.db.WithContext(ctx).Model(&entity.User{})

	// 关键词过滤
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		db = db.Where("username LIKE ? OR phone LIKE ?", keyword, keyword)
	}

	// 组织过滤
	if req.OrgID > 0 {
		// 过滤属于该组织的用户
		db = db.Where("id IN (SELECT user_id FROM user_org_roles WHERE org_id = ?)", req.OrgID)
	}

	// 统计总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页
	// 如果 Page 或 PageSize 为 0，设置默认值
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	if err := db.Preload("CurrentOrg").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *UserGormRepository) ExistsByUsername(
	ctx context.Context,
	username string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("username = ?", username).
		Count(&count).
		Error
	return count > 0, err
}

// ExistsByEmail 检查邮箱是否存在
func (r *UserGormRepository) ExistsByEmail(
	ctx context.Context,
	email string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("email = ?", email).
		Count(&count).
		Error
	return count > 0, err
}

// ExistsByPhone 检查手机号是否存在
func (r *UserGormRepository) ExistsByPhone(
	ctx context.Context,
	phone string,
) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("phone = ?", phone).
		Count(&count).
		Error
	return count > 0, err
}

// GetActiveUsers 获取活跃用户（未冻结）
func (r *UserGormRepository) GetActiveUsers(ctx context.Context) ([]*entity.User, error) {
	var users []*entity.User
	err := r.db.WithContext(ctx).
		Where("freeze = ?", false).
		Find(&users).
		Error
	return users, err
}

// ValidateUser 验证用户登录
func (r *UserGormRepository) ValidateUser(
	ctx context.Context,
	username, password string,
) (*entity.User, error) {
	user, err := r.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("用户不存在")
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, errors.New("密码错误")
	}

	// 检查用户是否被冻结
	if user.Freeze {
		return nil, errors.New("用户已被冻结")
	}

	return user, nil
}

// UpdateLastLogin 更新最后登录时间
func (r *UserGormRepository) UpdateLastLogin(
	ctx context.Context,
	id uint,
) error {
	return r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("id = ?", id).
		Update("updated_at", "NOW()").
		Error
}

// CheckEmailAddress 检查邮箱地址是否存在
func (r *UserGormRepository) CheckEmailAddress(
	ctx context.Context,
	email string,
) error {
	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&entity.User{}).
		Error
	return err
}

// UpdateCurrentOrgID 更新用户当前组织ID
func (r *UserGormRepository) UpdateCurrentOrgID(
	ctx context.Context,
	userID uint,
	orgID *uint,
) error {
	return r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("id = ?", userID).
		Update("current_org_id", orgID).
		Error
}
