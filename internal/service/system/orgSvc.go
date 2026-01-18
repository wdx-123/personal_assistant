package system

import (
	"context"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
)

type OrgService struct {
	orgRepo interfaces.OrgRepository
}

func NewOrgService(repositoryGroup *repository.Group) *OrgService {
	return &OrgService{
		orgRepo: repositoryGroup.SystemRepositorySupplier.GetOrgRepository(),
	}
}

// GetOrgList 获取组织列表（支持分页与不分页）
// 如果 page <= 0，则返回所有数据
func (s *OrgService) GetOrgList(ctx context.Context, page, pageSize int) ([]*entity.Org, int64, error) {
	if page <= 0 {
		// 不分页，获取所有
		list, err := s.orgRepo.GetAllOrgs(ctx)
		if err != nil {
			return nil, 0, err
		}
		return list, int64(len(list)), nil
	}
	// 分页查询
	return s.orgRepo.GetOrgList(ctx, page, pageSize)
}

// GetOrgByID 根据ID获取组织
func (s *OrgService) GetOrgByID(ctx context.Context, id uint) (*entity.Org, error) {
	return s.orgRepo.GetByID(ctx, id)
}
