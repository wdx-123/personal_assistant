package system

import (
	"context"
	"sort"
	"strings"

	"personal_assistant/global"
	"personal_assistant/internal/model/dto/request"
	"personal_assistant/internal/model/dto/response"
	"personal_assistant/internal/model/entity"
	"personal_assistant/internal/repository"
	"personal_assistant/internal/repository/interfaces"
	"personal_assistant/pkg/errors"

	"gorm.io/gorm"
)

// MenuService 菜单管理服务
type MenuService struct {
	menuRepo interfaces.MenuRepository
	apiRepo  interfaces.APIRepository
}

// NewMenuService 创建菜单服务实例
func NewMenuService(repositoryGroup *repository.Group) *MenuService {
	return &MenuService{
		menuRepo: repositoryGroup.SystemRepositorySupplier.GetMenuRepository(),
		apiRepo:  repositoryGroup.SystemRepositorySupplier.GetAPIRepository(),
	}
}

// GetMenuTree 获取完整菜单树（管理端配置页）
func (s *MenuService) GetMenuTree(ctx context.Context) ([]*response.MenuItem, error) {
	menus, err := s.menuRepo.GetAllMenus(ctx)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	return s.buildTree(menus, 0), nil
}

// GetMyMenus 获取当前用户的菜单树（前端侧边栏）
func (s *MenuService) GetMyMenus(ctx context.Context, userID, orgID uint) ([]*response.MenuItem, error) {
	menus, err := s.menuRepo.GetMenusByUserID(ctx, userID, orgID)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	// 过滤只保留显示状态的菜单
	var activeMenus []*entity.Menu
	for _, m := range menus {
		if m.Status == 1 {
			activeMenus = append(activeMenus, m)
		}
	}
	return s.buildTree(activeMenus, 0), nil
}

// GetMenuList 获取菜单列表（分页，扁平）
func (s *MenuService) GetMenuList(ctx context.Context, filter *request.MenuListFilter) ([]*entity.Menu, int64, error) {
	if filter == nil {
		filter = &request.MenuListFilter{}
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 10
	}
	filter.Keyword = strings.TrimSpace(filter.Keyword)
	return s.menuRepo.GetMenuList(ctx, filter)
}

// GetMenuByID 根据ID获取菜单详情（含关联API）
func (s *MenuService) GetMenuByID(ctx context.Context, id uint) (*entity.Menu, error) {
	menu, err := s.menuRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Wrap(errors.CodeDBError, err)
	}
	if menu == nil || menu.ID == 0 {
		return nil, errors.New(errors.CodeMenuNotFound)
	}
	return menu, nil
}

// CreateMenu 创建菜单
func (s *MenuService) CreateMenu(ctx context.Context, req *request.CreateMenuReq) error {
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return errors.New(errors.CodeInvalidParams)
	}

	// 检查 code 唯一
	exists, err := s.menuRepo.ExistsByCode(ctx, code)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if exists {
		return errors.New(errors.CodeMenuCodeDuplicate)
	}

	// 若有父菜单，检查父菜单存在
	if req.ParentID > 0 {
		parent, err := s.menuRepo.GetByID(ctx, req.ParentID)
		if err != nil || parent == nil || parent.ID == 0 {
			return errors.NewWithMsg(errors.CodeInvalidParams, "父菜单不存在")
		}
	}

	// 状态默认值：仅当请求未显式传入 status 字段时使用默认值 1
	// 注：由于 int 零值为 0，无法区分"未传"与"传 0"，此处约定：传 0 表示禁用，不传则默认启用
	// 如需精确区分，应将 CreateMenuReq.Status 改为 *int 指针类型
	status := req.Status
	if status == 0 {
		status = 1 // 默认启用
	}
	menu := &entity.Menu{
		ParentID:      req.ParentID,
		Name:          name,
		Code:          code,
		Type:          req.Type,
		Icon:          req.Icon,
		RouteName:     req.RouteName,
		RoutePath:     req.RoutePath,
		RouteParam:    req.RouteParam,
		ComponentPath: req.ComponentPath,
		Status:        status,
		Sort:          req.Sort,
		Desc:          req.Desc,
	}

	if err := s.menuRepo.Create(ctx, menu); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// UpdateMenu 更新菜单
func (s *MenuService) UpdateMenu(ctx context.Context, id uint, req *request.UpdateMenuReq) error {
	menu, err := s.menuRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if menu == nil || menu.ID == 0 {
		return errors.New(errors.CodeMenuNotFound)
	}

	// 部分更新
	if req.ParentID != nil {
		// 不能设自己为父
		if *req.ParentID == id {
			return errors.NewWithMsg(errors.CodeInvalidParams, "不能将菜单设为自己的子菜单")
		}
		menu.ParentID = *req.ParentID
	}
	if req.Name != nil {
		menu.Name = strings.TrimSpace(*req.Name)
	}
	if req.Code != nil {
		newCode := strings.TrimSpace(*req.Code)
		if newCode != menu.Code {
			// code 变更需校验唯一
			exists, err := s.menuRepo.ExistsByCode(ctx, newCode)
			if err != nil {
				return errors.Wrap(errors.CodeDBError, err)
			}
			if exists {
				return errors.New(errors.CodeMenuCodeDuplicate)
			}
			menu.Code = newCode
		}
	}
	if req.Type != nil {
		menu.Type = *req.Type
	}
	if req.Icon != nil {
		menu.Icon = *req.Icon
	}
	if req.RouteName != nil {
		menu.RouteName = *req.RouteName
	}
	if req.RoutePath != nil {
		menu.RoutePath = *req.RoutePath
	}
	if req.RouteParam != nil {
		menu.RouteParam = *req.RouteParam
	}
	if req.ComponentPath != nil {
		menu.ComponentPath = *req.ComponentPath
	}
	if req.Status != nil {
		menu.Status = *req.Status
	}
	if req.Sort != nil {
		menu.Sort = *req.Sort
	}
	if req.Desc != nil {
		menu.Desc = *req.Desc
	}

	if err := s.menuRepo.Update(ctx, menu); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// DeleteMenu 删除菜单（有子菜单禁止删除）
func (s *MenuService) DeleteMenu(ctx context.Context, id uint) error {
	menu, err := s.menuRepo.GetByID(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if menu == nil || menu.ID == 0 {
		return errors.New(errors.CodeMenuNotFound)
	}

	// 检查子菜单
	hasChildren, err := s.menuRepo.HasChildren(ctx, id)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if hasChildren {
		return errors.New(errors.CodeMenuHasChildren)
	}

	// 先清空 API 绑定
	if err := s.menuRepo.ClearMenuAPIs(ctx, id); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}

	if err := s.menuRepo.Delete(ctx, id); err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// BindAPIs 绑定API到菜单（覆盖式，事务保证原子性）
func (s *MenuService) BindAPIs(ctx context.Context, menuID uint, apiIDs []uint) error {
	// 检查菜单存在
	menu, err := s.menuRepo.GetByID(ctx, menuID)
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	if menu == nil || menu.ID == 0 {
		return errors.New(errors.CodeMenuNotFound)
	}

	// 过滤有效的 API ID（先查询存在的）
	var validAPIIDs []uint
	for _, apiID := range apiIDs {
		api, err := s.apiRepo.GetByID(ctx, apiID)
		if err == nil && api != nil && api.ID > 0 {
			validAPIIDs = append(validAPIIDs, apiID)
		}
	}

	// 使用事务保证原子性：先删后增
	err = global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 清空原有绑定
		if err := tx.Exec("DELETE FROM menu_apis WHERE menu_id = ?", menuID).Error; err != nil {
			return err
		}
		// 批量插入新绑定
		for _, apiID := range validAPIIDs {
			if err := tx.Exec("INSERT IGNORE INTO menu_apis (menu_id, api_id) VALUES (?, ?)", menuID, apiID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrap(errors.CodeDBError, err)
	}
	return nil
}

// buildTree 递归构建菜单树
func (s *MenuService) buildTree(menus []*entity.Menu, parentID uint) []*response.MenuItem {
	var result []*response.MenuItem
	for _, m := range menus {
		if m.ParentID == parentID {
			item := s.entityToMenuItem(m)
			item.Children = s.buildTree(menus, m.ID)
			result = append(result, item)
		}
	}
	// 按 Sort 排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Sort < result[j].Sort
	})
	return result
}

// entityToMenuItem 转换实体到响应DTO
func (s *MenuService) entityToMenuItem(m *entity.Menu) *response.MenuItem {
	item := &response.MenuItem{
		ID:            m.ID,
		ParentID:      m.ParentID,
		Name:          m.Name,
		Code:          m.Code,
		Type:          m.Type,
		Icon:          m.Icon,
		RouteName:     m.RouteName,
		RoutePath:     m.RoutePath,
		RouteParam:    m.RouteParam,
		ComponentPath: m.ComponentPath,
		Status:        m.Status,
		Sort:          m.Sort,
		Desc:          m.Desc,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
	// 转换关联的 APIs
	if len(m.APIs) > 0 {
		item.APIs = make([]*response.APIItemSimple, 0, len(m.APIs))
		for _, api := range m.APIs {
			item.APIs = append(item.APIs, &response.APIItemSimple{
				ID:     api.ID,
				Path:   api.Path,
				Method: api.Method,
			})
		}
	}
	return item
}
