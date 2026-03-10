package system

import (
	"context"

	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"

	"gorm.io/gorm"
)

type capabilityRepository struct {
	db *gorm.DB
}

func NewCapabilityRepository(db *gorm.DB) interfaces.CapabilityRepository {
	return &capabilityRepository{db: db}
}

func (r *capabilityRepository) WithTx(tx any) interfaces.CapabilityRepository {
	if transaction, ok := tx.(*gorm.DB); ok {
		return &capabilityRepository{db: transaction}
	}
	return r
}

// GetAllActive 获取所有启用状态的 capability 列表。
func (r *capabilityRepository) GetAllActive(ctx context.Context) ([]*entity.Capability, error) {
	var capabilities []*entity.Capability
	err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Order("group_code ASC, id ASC").
		Find(&capabilities).Error
	return capabilities, err
}

// GetByIDs 根据一组 capability ID 获取对应的 capability 列表。
func (r *capabilityRepository) GetByCodes(ctx context.Context, codes []string) ([]*entity.Capability, error) {
	if len(codes) == 0 {
		return nil, nil
	}
	var capabilities []*entity.Capability
	err := r.db.WithContext(ctx).
		Where("code IN ? AND status = ?", codes, 1).
		Find(&capabilities).Error
	return capabilities, err
}

// GetRoleCapabilityCodes 获取角色关联的 capability codes。
func (r *capabilityRepository) GetRoleCapabilityCodes(ctx context.Context, roleID uint) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("capabilities").
		Joins("JOIN role_capabilities ON capabilities.id = role_capabilities.capability_id").
		Where("role_capabilities.role_id = ? AND capabilities.deleted_at IS NULL AND capabilities.status = ?", roleID, 1).
		Order("capabilities.code ASC").
		Pluck("capabilities.code", &codes).Error
	return codes, err
}

// ReplaceRoleCapabilities 替换角色的 capability 关联。
func (r *capabilityRepository) ReplaceRoleCapabilities(ctx context.Context, roleID uint, capabilityIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM role_capabilities WHERE role_id = ?", roleID).Error; err != nil {
			return err
		}
		if len(capabilityIDs) == 0 {
			return nil
		}

		items := make([]entity.RoleCapability, 0, len(capabilityIDs))
		for _, capabilityID := range capabilityIDs {
			if capabilityID == 0 {
				continue
			}
			items = append(items, entity.RoleCapability{
				RoleID:       roleID,
				CapabilityID: capabilityID,
			})
		}
		if len(items) == 0 {
			return nil
		}
		return tx.Create(&items).Error
	})
}

// GetAllRoleCapabilityRelations 获取所有角色 capability 关联。
func (r *capabilityRepository) GetAllRoleCapabilityRelations(ctx context.Context) ([]map[string]interface{}, error) {
	var relations []map[string]interface{}
	err := r.db.WithContext(ctx).
		Table("role_capabilities").
		Select("roles.code as role_code, capabilities.code as capability_code").
		Joins("JOIN roles ON role_capabilities.role_id = roles.id").
		Joins("JOIN capabilities ON role_capabilities.capability_id = capabilities.id").
		Where("roles.deleted_at IS NULL AND capabilities.deleted_at IS NULL AND capabilities.status = ?", 1).
		Find(&relations).Error
	return relations, err
}
