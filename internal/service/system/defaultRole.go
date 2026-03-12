package system

import (
	"context"
	stderrors "errors"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository/interfaces"
	bizerrors "personal_assistant/pkg/errors"

	"gorm.io/gorm"
)

// resolveDefaultOrgRole 解析组织内的新成员默认角色：
// 优先读取 system.default_role_code，未命中时回退到内置 member 角色。
func resolveDefaultOrgRole(
	ctx context.Context,
	roleRepo interfaces.RoleRepository,
) (*entity.Role, error) {
	roleCodes := []string{consts.RoleCodeMember}
	if global.Config != nil {
		if configured := strings.TrimSpace(global.Config.System.DefaultRoleCode); configured != "" && configured != consts.RoleCodeMember {
			roleCodes = append([]string{configured}, roleCodes...)
		}
	}

	for _, roleCode := range roleCodes {
		role, err := roleRepo.GetByCode(ctx, roleCode)
		switch {
		case err == nil && role != nil:
			return role, nil
		case err == nil && role == nil:
			continue
		case stderrors.Is(err, gorm.ErrRecordNotFound):
			continue
		default:
			return nil, bizerrors.Wrap(bizerrors.CodeDBError, err)
		}
	}

	return nil, bizerrors.New(bizerrors.CodeRoleNotFound)
}
