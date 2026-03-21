package system

import (
	"context"
	stderrors "errors"
	"strings"
	"time"

	"personal_assistant/global"
	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	svccontract "personal_assistant/internal/service/contract"
	"personal_assistant/pkg/errors"

	"gorm.io/gorm"
)

// ApiService API接口管理服务
type ApiService struct {
	txRunner                repository.TxRunner
	apiRepo                 interfaces.APIRepository
	menuRepo                interfaces.MenuRepository
	roleRepo                interfaces.RoleRepository
	permissionProjectionSvc svccontract.PermissionProjectionServiceContract
}

// NewApiService 创建API服务实例
func NewApiService(
	repositoryGroup *repository.Group,
	permissionProjectionSvc svccontract.PermissionProjectionServiceContract,
) *ApiService {
	return &ApiService{
		txRunner:                repositoryGroup,
		apiRepo:                 repositoryGroup.SystemRepositorySupplier.GetAPIRepository(),
		menuRepo:                repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
		roleRepo:                repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		permissionProjectionSvc: permissionProjectionSvc,
	}
}

// GetAPIList 获取API列表（分页，支持过滤）
func (s *ApiService) GetAPIList(
	ctx context.Context,
	filter *request.ApiListFilter,
) ([]*entity.API, map[uint]*entity.Menu, int64, error) {
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
	filter.MenuName = strings.TrimSpace(filter.MenuName)
	filter.SyncState = strings.TrimSpace(filter.SyncState)

	if filter.MenuName != "" {
		matchedCount, err := s.menuRepo.CountByNameLike(ctx, filter.MenuName)
		if err != nil {
			return nil, nil, 0, errors.Wrap(errors.CodeDBError, err)
		}
		if matchedCount == 0 {
			return nil, nil, 0, errors.New(errors.CodeMenuNotFound)
		}
	}

	list, total, err := s.apiRepo.GetAPIList(ctx, filter)
	if err != nil {
		return nil, nil, 0, errors.Wrap(errors.CodeDBError, err)
	}

	menuMap, err := s.apiRepo.GetMenusByAPIIDs(ctx, collectAPIIDs(list))
	if err != nil {
		return nil, nil, 0, errors.Wrap(errors.CodeDBError, err)
	}
	return list, menuMap, total, nil
}

// GetAPIByID 根据ID获取API详情
func (s *ApiService) GetAPIByID(ctx context.Context, id uint) (*entity.API, *entity.Menu, error) {
	api, err := s.apiRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, errors.Wrap(errors.CodeDBError, err)
	}
	if api == nil {
		return nil, nil, errors.New(errors.CodeAPINotFound)
	}

	menu, err := s.apiRepo.GetMenuByAPIID(ctx, id)
	if err != nil {
		return nil, nil, errors.Wrap(errors.CodeDBError, err)
	}
	return api, menu, nil
}

// CreateAPI 创建API
func (s *ApiService) CreateAPI(ctx context.Context, req *request.CreateApiReq) error {
	path := strings.TrimSpace(req.Path)
	method := strings.TrimSpace(strings.ToUpper(req.Method))
	if path == "" || method == "" || req.MenuID == 0 {
		return errors.New(errors.CodeInvalidParams)
	}

	menu, err := s.menuRepo.GetByID(ctx, req.MenuID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if menu == nil || menu.ID == 0 {
		return errors.New(errors.CodeMenuNotFound)
	}

	status := req.Status
	if status == 0 {
		status = 1
	}

	exists, err := s.apiRepo.ExistsByPathAndMethod(ctx, path, method)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if exists {
		return errors.New(errors.CodeAPIAlreadyExists)
	}

	api := &entity.API{
		Path:      path,
		Method:    method,
		Detail:    req.Detail,
		Status:    status,
		SyncState: consts.APISyncStateRegistered,
	}
	// 创建API并关联菜单，发布权限变更事件
	return s.txRunner.InTx(ctx, func(tx any) error {
		txAPIRepo := s.apiRepo.WithTx(tx)
		if err := txAPIRepo.CreateWithMenu(ctx, api, req.MenuID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishPermissionGraphChangedInTx(ctx, tx, "api", api.ID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}
		return nil
	})
}

// UpdateAPI 更新API（支持部分更新）
func (s *ApiService) UpdateAPI(ctx context.Context, id uint, req *request.UpdateApiReq) error {
	api, err := s.apiRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if api == nil {
		return errors.New(errors.CodeAPINotFound)
	}
	if req.Path != nil {
		api.Path = strings.TrimSpace(*req.Path)
	}
	if req.Method != nil {
		api.Method = strings.TrimSpace(strings.ToUpper(*req.Method))
	}
	if req.Detail != nil {
		api.Detail = *req.Detail
	}
	if req.Status != nil {
		api.Status = *req.Status
	}
	if req.MenuID != nil && *req.MenuID > 0 {
		menu, getErr := s.menuRepo.GetByID(ctx, *req.MenuID)
		if getErr != nil {
			return errors.Wrap(errors.CodeDBError, getErr)
		}
		if menu == nil || menu.ID == 0 {
			return errors.New(errors.CodeMenuNotFound)
		}
	}
	if api.Path == "" || api.Method == "" {
		return errors.New(errors.CodeInvalidParams)
	}

	existAPI, getErr := s.apiRepo.GetByPathAndMethod(ctx, api.Path, api.Method)
	if getErr != nil && !stderrors.Is(getErr, gorm.ErrRecordNotFound) {
		return errors.Wrap(errors.CodeDBError, getErr)
	}
	if getErr == nil && existAPI != nil && existAPI.ID != id {
		return errors.New(errors.CodeAPIAlreadyExists)
	}
	return s.txRunner.InTx(ctx, func(tx any) error {
		txAPIRepo := s.apiRepo.WithTx(tx)
		if err := txAPIRepo.UpdateWithMenu(ctx, api, req.MenuID); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishPermissionGraphChangedInTx(ctx, tx, "api", api.ID); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}
		return nil
	})
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
	return s.txRunner.InTx(ctx, func(tx any) error {
		txMenuRepo := s.menuRepo.WithTx(tx)
		txRoleRepo := s.roleRepo.WithTx(tx)
		txAPIRepo := s.apiRepo.WithTx(tx)
		if err := txMenuRepo.RemoveAPIFromAllMenus(ctx, id); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := txRoleRepo.RemoveAPIFromAllRoles(ctx, id); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if err := txAPIRepo.Delete(ctx, id); err != nil {
			return errors.Wrap(errors.CodeDBError, err)
		}
		if s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishPermissionGraphChangedInTx(ctx, tx, "api", id); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}
		return nil
	})
}

// SyncAPI 同步路由与数据库API记录。
// deleteRemoved 参数仅保留兼容性，当前不再执行物理删除。
func (s *ApiService) SyncAPI(
	ctx context.Context,
	deleteRemoved bool,
) (added, restored, markedMissing, archived int, total int, err error) {
	_ = deleteRemoved
	if global.Router == nil {
		return 0, 0, 0, 0, 0, errors.New(errors.CodeInternalError)
	}

	now := time.Now()
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
		return 0, 0, 0, 0, 0, errors.Wrap(errors.CodeDBError, err)
	}
	existingMap := make(map[string]*entity.API)
	for _, api := range allAPIs {
		key := api.Path + ":" + api.Method
		existingMap[key] = api
	}

	projectionChanged := false
	if err := s.txRunner.InTx(ctx, func(tx any) error {
		txAPIRepo := s.apiRepo.WithTx(tx)

		for _, r := range routes {
			if r.Method == "" || r.Path == "" {
				continue
			}
			key := r.Path + ":" + r.Method
			if api, ok := existingMap[key]; ok {
				if api.SyncState == consts.APISyncStateArchived {
					continue
				}
				needUpdate := false
				if api.SyncState == consts.APISyncStateMissing {
					api.SyncState = consts.APISyncStateRegistered
					restored++
					needUpdate = true
					projectionChanged = true
				} else if strings.TrimSpace(api.SyncState) == "" {
					api.SyncState = consts.APISyncStateRegistered
					needUpdate = true
				}
				if api.LastSeenAt == nil || !api.LastSeenAt.Equal(now) {
					api.LastSeenAt = &now
					needUpdate = true
				}
				if needUpdate {
					if err := txAPIRepo.Update(ctx, api); err != nil {
						return errors.Wrap(errors.CodeDBError, err)
					}
				}
				continue
			}

			existingAPI, getErr := txAPIRepo.GetByPathAndMethod(ctx, r.Path, r.Method)
			if getErr != nil && !stderrors.Is(getErr, gorm.ErrRecordNotFound) {
				return errors.Wrap(errors.CodeDBError, getErr)
			}
			if getErr == nil && existingAPI != nil {
				if existingAPI.DeletedAt.Valid || existingAPI.SyncState != consts.APISyncStateRegistered {
					existingAPI.SyncState = consts.APISyncStateRegistered
					restored++
					projectionChanged = true
				} else if strings.TrimSpace(existingAPI.SyncState) == "" {
					existingAPI.SyncState = consts.APISyncStateRegistered
				}
				existingAPI.LastSeenAt = &now
				existingAPI.DeletedAt = gorm.DeletedAt{}
				if err := txAPIRepo.Update(ctx, existingAPI); err != nil {
					return errors.Wrap(errors.CodeDBError, err)
				}
				existingMap[key] = existingAPI
				continue
			}

			newAPI := &entity.API{
				Path:       r.Path,
				Method:     r.Method,
				Detail:     "",
				Status:     1,
				SyncState:  consts.APISyncStateRegistered,
				LastSeenAt: &now,
			}
			if err := txAPIRepo.Create(ctx, newAPI); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			added++
			projectionChanged = true
		}

		for _, api := range allAPIs {
			key := api.Path + ":" + api.Method
			if pathMethodSet[key] {
				continue
			}
			if api.SyncState == consts.APISyncStateArchived || api.SyncState == consts.APISyncStateMissing {
				continue
			}
			api.SyncState = consts.APISyncStateMissing
			if err := txAPIRepo.Update(ctx, api); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			markedMissing++
			projectionChanged = true
		}

		if projectionChanged && s.permissionProjectionSvc != nil {
			if err := s.permissionProjectionSvc.PublishPermissionGraphChangedInTx(ctx, tx, "api", 0); err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
		}
		return nil
	}); err != nil {
		return added, restored, markedMissing, archived, 0, err
	}

	allAPIs, _ = s.apiRepo.GetAllAPIs(ctx)
	total = len(allAPIs)
	return added, restored, markedMissing, archived, total, nil
}

func collectAPIIDs(apis []*entity.API) []uint {
	apiIDs := make([]uint, 0, len(apis))
	for _, api := range apis {
		apiIDs = append(apiIDs, api.ID)
	}
	return apiIDs
}
