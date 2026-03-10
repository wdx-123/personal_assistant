package interfaces

import (
	"context"

	"personal_assistant/internal/model/entity"
)

// CapabilityRepository capability 仓储接口。
type CapabilityRepository interface {
	// GetAllActive 获取所有启用状态的 capability 列表。
	GetAllActive(ctx context.Context) ([]*entity.Capability, error)

	// GetByCodes 根据一组 capability code 获取对应的 capability 列表。
	GetByCodes(ctx context.Context, codes []string) ([]*entity.Capability, error)

	// GetRoleCapabilityCodes 获取角色关联的 capability codes。
	GetRoleCapabilityCodes(ctx context.Context, roleID uint) ([]string, error)

	// ReplaceRoleCapabilities 替换角色的 capability 关联。
	ReplaceRoleCapabilities(ctx context.Context, roleID uint, capabilityIDs []uint) error

	// GetAllRoleCapabilityRelations 获取所有角色 capability 关联。
	GetAllRoleCapabilityRelations(ctx context.Context) ([]map[string]interface{}, error)

	// WithTx 在事务上下文中返回一个新的 capability 仓储实例。
	WithTx(tx any) CapabilityRepository
}
