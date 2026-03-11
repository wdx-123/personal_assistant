package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/rediskey"
	"personal_assistant/pkg/util"

	"github.com/go-redis/redis/v8"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	userActiveStateValueActive   = "1"
	userActiveStateValueInactive = "0"
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

// GetByIDsActive 批量获取活跃用户
func (r *UserGormRepository) GetByIDsActive(
	ctx context.Context,
	ids []uint,
) ([]*entity.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var users []*entity.User
	err := r.db.WithContext(ctx).
		Where("id IN ? AND status = ? AND freeze = ?", ids, consts.UserStatusActive, false).
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
		// 过滤属于该组织且为 active 成员的用户
		db = db.Where(
			"id IN (SELECT user_id FROM org_members WHERE org_id = ? AND member_status = ?)",
			req.OrgID,
			consts.OrgMemberStatusActive,
		)
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
		Where("freeze = ? AND status = ?", false, consts.UserStatusActive).
		Find(&users).
		Error
	return users, err
}

// GetCachedActiveState 从 Redis 读取用户活跃态缓存。
func (r *UserGormRepository) GetCachedActiveState(
	ctx context.Context,
	userID uint,
) (active bool, found bool, err error) {
	if userID == 0 || global.Redis == nil {
		return false, false, nil
	}

	val, err := global.Redis.Get(ctx, rediskey.UserActiveStateKey(userID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, false, nil
		}
		return false, false, err
	}

	switch val {
	case userActiveStateValueActive:
		return true, true, nil
	case userActiveStateValueInactive:
		return false, true, nil
	default:
		return false, false, nil
	}
}

// CacheActiveState 写入用户活跃态缓存。
func (r *UserGormRepository) CacheActiveState(
	ctx context.Context,
	userID uint,
	active bool,
) error {
	if userID == 0 || global.Redis == nil {
		return nil
	}

	// 设置缓存值
	value := userActiveStateValueInactive
	if active {
		value = userActiveStateValueActive
	}

	// 写入 Redis，设置过期时间以支持自动失效
	return global.Redis.Set(
		ctx,
		rediskey.UserActiveStateKey(userID),
		value,
		r.activeStateTTL(),
	).Err()
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
	if user.Freeze || user.Status != consts.UserStatusActive {
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

func (r *UserGormRepository) ListIDsByCurrentOrgID(ctx context.Context, orgID uint) ([]uint, error) {
	if orgID == 0 {
		return nil, nil
	}
	var userIDs []uint
	err := r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("current_org_id = ?", orgID).
		Order("id ASC").
		Pluck("id", &userIDs).Error
	return userIDs, err
}

// ClearCurrentOrgByOrgID 将当前组织为指定 org 的用户置空
func (r *UserGormRepository) ClearCurrentOrgByOrgID(ctx context.Context, orgID uint) error {
	return r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("current_org_id = ?", orgID).
		Update("current_org_id", nil).
		Error
}

// UpdateUserStatus 更新账号状态及禁用元数据
func (r *UserGormRepository) UpdateUserStatus(
	ctx context.Context,
	userID uint,
	status consts.UserStatus,
	disabledBy *uint,
	disabledReason string,
) error {
	updates := map[string]any{
		"status": status,
		"freeze": status != consts.UserStatusActive,
	}

	if status == consts.UserStatusDisabled {
		now := time.Now()
		updates["disabled_at"] = now
		updates["disabled_by"] = disabledBy
		updates["disabled_reason"] = strings.TrimSpace(disabledReason)
	} else {
		updates["disabled_at"] = nil
		updates["disabled_by"] = nil
		updates["disabled_reason"] = ""
	}

	return r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("id = ?", userID).
		Updates(updates).Error
}

// ListDisabledUsersBefore 分页查询到期可清理的禁用用户
func (r *UserGormRepository) ListDisabledUsersBefore(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]*entity.User, error) {
	if limit <= 0 {
		limit = 100
	}
	var users []*entity.User
	err := r.db.WithContext(ctx).
		Where("status = ? AND disabled_at IS NOT NULL AND disabled_at < ?", consts.UserStatusDisabled, before).
		Order("disabled_at ASC").
		Limit(limit).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// SoftDeleteAndAnonymize 软删除并匿名化账号
func (r *UserGormRepository) SoftDeleteAndAnonymize(ctx context.Context, userID uint) error {
	now := time.Now()
	anon := fmt.Sprintf("d%x%x", userID, now.Unix()%0xFFFFFF)
	return r.db.WithContext(ctx).
		Model(&entity.User{}).
		Where("id = ?", userID).
		Updates(map[string]any{
			"status":         consts.UserStatusDeletedSoft,
			"freeze":         true,
			"username":       anon,
			"phone":          anon,
			"email":          "",
			"avatar":         "",
			"address":        "",
			"signature":      "",
			"current_org_id": nil,
			"deleted_at":     now,
		}).Error
}

func (r *UserGormRepository) activeStateTTL() time.Duration {
	if global.Config == nil {
		return util.ApplyTTLJitter(0, 0)
	}

	baseTTL := time.Duration(global.Config.Redis.ActiveUserStateTTLSeconds) * time.Second
	jitterTTL := time.Duration(global.Config.Redis.ActiveUserStateTTLJitterSeconds) * time.Second
	return util.ApplyTTLJitter(baseTTL, jitterTTL)
}
