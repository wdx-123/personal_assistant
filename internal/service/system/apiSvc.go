package system

import (
	"context"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"
)

// ApiService API接口管理服务
type ApiService struct {
	apiRepo  interfaces.APIRepository
	menuRepo interfaces.MenuRepository
}

// NewApiService 创建API服务实例
func NewApiService(repositoryGroup *repository.Group) *ApiService {
	return &ApiService{
		apiRepo:  repositoryGroup.SystemRepositorySupplier.GetAPIRepository(),
		menuRepo: repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
	}
}

// GetAPIList 获取API列表（分页，支持过滤）
func (s *ApiService) GetAPIList(ctx context.Context, filter *request.ApiListFilter) ([]*entity.API, int64, error) {
	if filter == nil {
		filter = &request.ApiListFilter{}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	filter.Method = strings.TrimSpace(filter.Method)
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	return s.apiRepo.GetAPIList(ctx, filter)
}

// GetAPIByID 根据ID获取API详情
func (s *ApiService) GetAPIByID(ctx context.Context, id uint) (*entity.API, error) {
	api, err := s.apiRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if api == nil {
		return nil, errors.New(errors.CodeAPINotFound)
	}
	return api, nil
}

// CreateAPI 创建API
func (s *ApiService) CreateAPI(ctx context.Context, req *entity.API) error {
	path := strings.TrimSpace(req.Path)
	method := strings.TrimSpace(strings.ToUpper(req.Method))
	if path == "" || method == "" {
		return errors.New(errors.CodeInvalidParams)
	}
	req.Path = path
	req.Method = method
	if req.Status == 0 {
		req.Status = 1
	}

	exists, err := s.apiRepo.ExistsByPathAndMethod(ctx, path, method)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if exists {
		return errors.New(errors.CodeAPIAlreadyExists)
	}
	if err := s.apiRepo.Create(ctx, req); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// UpdateAPI 更新API
func (s *ApiService) UpdateAPI(
	ctx context.Context,
	id uint,
	path *string,
	method *string,
	detail *string,
	groupID *uint,
	status *int,
) error {
	api, err := s.apiRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if api == nil {
		return errors.New(errors.CodeAPINotFound)
	}
	if path != nil {
		api.Path = strings.TrimSpace(*path)
	}
	if method != nil {
		api.Method = strings.TrimSpace(strings.ToUpper(*method))
	}
	if detail != nil {
		api.Detail = *detail
	}
	if groupID != nil {
		api.GroupID = *groupID
	}
	if status != nil {
		api.Status = *status
	}
	if api.Path == "" || api.Method == "" {
		return errors.New(errors.CodeInvalidParams)
	}
	existAPI, getErr := s.apiRepo.GetByPathAndMethod(ctx, api.Path, api.Method)
	if getErr == nil && existAPI != nil && existAPI.ID != id {
		return errors.New(errors.CodeAPIAlreadyExists)
	}
	if err := s.apiRepo.Update(ctx, api); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// DeleteAPI 删除API（先解绑菜单再删除）
func (s *ApiService) DeleteAPI(ctx context.Context, id uint) error {
	api, err := s.apiRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if api == nil {
		return errors.New(errors.CodeAPINotFound)
	}
	if err := s.menuRepo.RemoveAPIFromAllMenus(ctx, id); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if err := s.apiRepo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// SyncAPI 同步路由到API表
// 从 Gin 引擎扫描所有已注册路由，新增的插入，已移除的禁用或删除
// SyncAPI 同步路由与数据库API记录
// deleteRemoved: true-删除不存在的API, false-仅禁用
// 返回: 新增数、更新数、禁用数、总数、错误
func (s *ApiService) SyncAPI(
	ctx context.Context,
	deleteRemoved bool,
) (added, updated, disabled int, total int, err error) {
	if global.Router == nil {
		return 0, 0, 0, 0, errors.New(errors.CodeInternalError)
	}

	// 构建当前路由集合 (path:method 格式，与项目权限风格一致)
	routes := global.Router.Routes()
	pathMethodSet := make(map[string]bool)
	for _, r := range routes {
		if r.Method == "" || r.Path == "" {
			continue
		}
		pathMethodSet[r.Path+":"+r.Method] = true
	}

	// 获取数据库已有API，构建映射
	allAPIs, err := s.apiRepo.GetAllAPIs(ctx)
	if err != nil {
		return 0, 0, 0, 0, errors.Wrap(errors.CodeDBError, err)
	}
	existingMap := make(map[string]*entity.API)
	for _, api := range allAPIs {
		key := api.Path + ":" + api.Method
		existingMap[key] = api
	}

	// 遍历路由：已存在则恢复启用，不存在则新增
	for _, r := range routes {
		if r.Method == "" || r.Path == "" {
			continue
		}
		key := r.Path + ":" + r.Method
		if exist, ok := existingMap[key]; ok {
			// 已禁用的API重新启用
			if exist.Status == 0 {
				exist.Status = 1
				if err := s.apiRepo.Update(ctx, exist); err != nil {
					return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
				}
				updated++
			}
			continue
		}
		// 新路由，创建API记录
		newAPI := &entity.API{
			Path:   r.Path,
			Method: r.Method,
			Detail: "",
			Status: 1,
		}
		if err := s.apiRepo.Create(ctx, newAPI); err != nil {
			return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
		}
		added++
	}

	// 处理数据库中已不存在于路由的API
	for _, api := range allAPIs {
		key := api.Path + ":" + api.Method
		if pathMethodSet[key] {
			continue
		}
		if deleteRemoved {
			// 物理删除：先解除菜单关联，再删除API
			if err := s.menuRepo.RemoveAPIFromAllMenus(ctx, api.ID); err != nil {
				return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
			}
			if err := s.apiRepo.Delete(ctx, api.ID); err != nil {
				return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
			}
		} else if api.Status == 1 {
			// 逻辑禁用
			api.Status = 0
			if err := s.apiRepo.Update(ctx, api); err != nil {
				return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
			}
			disabled++
		}
	}

	// 重新获取总数
	allAPIs, _ = s.apiRepo.GetAllAPIs(ctx)
	total = len(allAPIs)
	return added, updated, disabled, total, nil
}
