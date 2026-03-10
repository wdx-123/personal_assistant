package system

import (
	"context"
	"errors"
	"strings"
	"time"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// orgMemberRepository 组织成员状态仓储实现
type orgMemberRepository struct {
	db *gorm.DB
}

// NewOrgMemberRepository 创建组织成员状态仓储实例
func NewOrgMemberRepository(db *gorm.DB) interfaces.OrgMemberRepository {
	return &orgMemberRepository{db: db}
}

func (r *orgMemberRepository) WithTx(tx any) interfaces.OrgMemberRepository {
	if t, ok := tx.(*gorm.DB); ok {
		return &orgMemberRepository{db: t}
	}
	return r
}

// GetByOrgAndUser 根据组织和用户获取成员信息
func (r *orgMemberRepository) GetByOrgAndUser(ctx context.Context, orgID, userID uint) (*entity.OrgMember, error) {
	var member entity.OrgMember
	err := r.db.WithContext(ctx).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

// GetByOrgAndUserForUpdate 根据组织和用户获取成员信息（用于更新，带行锁）
func (r *orgMemberRepository) GetByOrgAndUserForUpdate(ctx context.Context, orgID, userID uint) (*entity.OrgMember, error) {
	var member entity.OrgMember
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &member, nil
}

func (r *orgMemberRepository) Create(ctx context.Context, member *entity.OrgMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *orgMemberRepository) Update(ctx context.Context, member *entity.OrgMember) error {
	return r.db.WithContext(ctx).Save(member).Error
}

// SetStatus 设置成员状态（加入、退出、踢出等）
func (r *orgMemberRepository) SetStatus(
	ctx context.Context,
	orgID, userID uint,
	status consts.OrgMemberStatus,
	operatorID *uint,
	reason string,
	joinSource string,
) error {
	reason = strings.TrimSpace(reason)
	updates := map[string]any{
		"member_status": status,
		"remove_reason": reason,
		"updated_at":    time.Now(),
	}

	switch status {
	case consts.OrgMemberStatusActive:
		updates["left_at"] = nil
		updates["removed_at"] = nil
		updates["removed_by"] = nil
		updates["remove_reason"] = ""
		if strings.TrimSpace(joinSource) != "" {
			updates["join_source"] = joinSource
		}
		updates["joined_at"] = time.Now()
	case consts.OrgMemberStatusLeft:
		now := time.Now()
		updates["left_at"] = now
		updates["removed_at"] = nil
		updates["removed_by"] = nil
	case consts.OrgMemberStatusRemoved:
		now := time.Now()
		updates["removed_at"] = now
		updates["left_at"] = nil
		updates["removed_by"] = operatorID
	}

	return r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("org_id = ? AND user_id = ?", orgID, userID).
		Updates(updates).Error
}

// SetAllRemovedByOrg 批量设置组织内所有成员为被踢出状态（如解散组织时）
func (r *orgMemberRepository) SetAllRemovedByOrg(ctx context.Context, orgID uint, operatorID *uint, reason string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("org_id = ?", orgID).
		Updates(map[string]any{
			"member_status": consts.OrgMemberStatusRemoved,
			"left_at":       nil,
			"removed_at":    now,
			"removed_by":    operatorID,
			"remove_reason": strings.TrimSpace(reason),
		}).Error
}

// IsUserActiveInOrg 获取组织内的活跃成员数量
func (r *orgMemberRepository) IsUserActiveInOrg(ctx context.Context, userID, orgID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("user_id = ? AND org_id = ? AND member_status = ?", userID, orgID, consts.OrgMemberStatusActive).
		Count(&count).Error
	return count > 0, err
}

// CountActiveMembersByOrgID 获取组织内的活跃成员数量
func (r *orgMemberRepository) CountActiveMembersByOrgID(ctx context.Context, orgID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("org_id = ? AND member_status = ?", orgID, consts.OrgMemberStatusActive).
		Count(&count).Error
	return count, err
}

// ListActiveOrgIDsByUser 获取用户加入的所有活跃组织ID列表
func (r *orgMemberRepository) ListActiveOrgIDsByUser(ctx context.Context, userID uint) ([]uint, error) {
	var orgIDs []uint
	err := r.db.WithContext(ctx).
		Model(&entity.OrgMember{}).
		Where("user_id = ? AND member_status = ?", userID, consts.OrgMemberStatusActive).
		Order("org_id ASC").
		Pluck("org_id", &orgIDs).Error
	return orgIDs, err
}
