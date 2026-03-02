package request

// MenuListFilter 菜单列表查询过滤条件
type MenuListFilter struct {
	Page     int    `form:"page"`      // 页码，默认1
	PageSize int    `form:"page_size"` // 每页数量，默认10
	Type     *int   `form:"type"`      // 类型过滤：1目录 2菜单 3按钮
	Status   *int   `form:"status"`    // 状态过滤：1显示 0隐藏
	ParentID *uint  `form:"parent_id"` // 父菜单ID
	Keyword  string `form:"keyword"`   // 按名称或code模糊搜索
}

// MyMenuReq 获取当前用户菜单请求
type MyMenuReq struct {
	OrgID *uint `form:"org_id"` // 可选：指定组织上下文，默认使用用户当前组织
}

// CreateMenuReq 创建菜单请求
type CreateMenuReq struct {
	ParentID      uint   `json:"parent_id"`                           // 父ID，0为顶级
	Name          string `json:"name" binding:"required"`             // 菜单名称
	Code          string `json:"code" binding:"required"`             // 权限标识（唯一）
	Type          int    `json:"type" binding:"required,oneof=1 2 3"` // 类型：1目录 2菜单 3按钮
	Icon          string `json:"icon"`                                // 菜单图标
	RouteName     string `json:"route_name"`                          // 路由名称
	RoutePath     string `json:"route_path"`                          // 路由路径
	RouteParam    string `json:"route_param"`                         // 路由参数
	ComponentPath string `json:"component_path"`                      // 组件路径
	Status        int    `json:"status"`                              // 状态：1显示 0隐藏，默认1
	Sort          int    `json:"sort"`                                // 排序
	Desc          string `json:"desc"`                                // 描述
}

// UpdateMenuReq 更新菜单请求（全部可选，支持部分更新）
type UpdateMenuReq struct {
	ParentID      *uint   `json:"parent_id"`      // 父ID
	Name          *string `json:"name"`           // 菜单名称
	Code          *string `json:"code"`           // 权限标识
	Type          *int    `json:"type"`           // 类型：1目录 2菜单 3按钮
	Icon          *string `json:"icon"`           // 菜单图标
	RouteName     *string `json:"route_name"`     // 路由名称
	RoutePath     *string `json:"route_path"`     // 路由路径
	RouteParam    *string `json:"route_param"`    // 路由参数
	ComponentPath *string `json:"component_path"` // 组件路径
	Status        *int    `json:"status"`         // 状态
	Sort          *int    `json:"sort"`           // 排序
	Desc          *string `json:"desc"`           // 描述
}

// BindMenuAPIReq 绑定API到菜单请求
type BindMenuAPIReq struct {
	MenuID uint   `json:"menu_id" binding:"required"` // 菜单ID
	APIIDs []uint `json:"api_ids" binding:"required"` // API ID列表（全量替换）
}
