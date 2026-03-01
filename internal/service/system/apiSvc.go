package system

import (
	"context"
	stderrors "errors"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"

	"gorm.io/gorm"
)

// ApiService API接口管理服务
type ApiService struct {
	apiRepo           interfaces.APIRepository
	menuRepo          interfaces.MenuRepository
	roleRepo          interfaces.RoleRepository
	permissionService *PermissionService
}

// NewApiService 创建API服务实例
func NewApiService(repositoryGroup *repository.Group, permissionService *PermissionService) *ApiService {
	return &ApiService{
		apiRepo:           repositoryGroup.SystemRepositorySupplier.GetAPIRepository(),
		menuRepo:          repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
		roleRepo:          repositoryGroup.SystemRepositorySupplier.GetRoleRepository(),
		permissionService: permissionService,
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
		Path:   path,
		Method: method,
		Detail: req.Detail,
		Status: status,
	}
	if err := s.apiRepo.CreateWithMenu(ctx, api, req.MenuID); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if err := s.permissionService.RefreshAllPermissions(ctx); err != nil {
		return errors.Wrap(errors.CodeInternalError, err)
	}
	return nil
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
	if err := s.apiRepo.UpdateWithMenu(ctx, api, req.MenuID); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if err := s.permissionService.RefreshAllPermissions(ctx); err != nil {
		return errors.Wrap(errors.CodeInternalError, err)
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
	if err := s.roleRepo.RemoveAPIFromAllRoles(ctx, id); err != nil {
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

	// 遍历路由：数据库中不存在则新增，已存在则跳过（不自动恢复启用，启用由管理员手动操作）
	for _, r := range routes {
		if r.Method == "" || r.Path == "" {
			continue
		}
		key := r.Path + ":" + r.Method
		if _, ok := existingMap[key]; ok {
			// 已存在（无论启用还是禁用），跳过，不干预管理员的决定
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
		/*
			因为 apis 这个主体会被两张关系表引用：
			1、menu_apis（菜单绑定 API）
			2、role_apis（角色直绑 API）
		*/
		if deleteRemoved {
			// 物理删除：先解除菜单关联，再删除API
			if err := s.menuRepo.RemoveAPIFromAllMenus(ctx, api.ID); err != nil {
				return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
			}
			// 删除API前先解除角色关联，避免外键约束问题
			if err := s.roleRepo.RemoveAPIFromAllRoles(ctx, api.ID); err != nil {
				return added, updated, disabled, 0, errors.Wrap(errors.CodeDBError, err)
			}
			// 删除API记录
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

func collectAPIIDs(apis []*entity.API) []uint {
	apiIDs := make([]uint, 0, len(apis))
	for _, api := range apis {
		apiIDs = append(apiIDs, api.ID)
	}
	return apiIDs
}
