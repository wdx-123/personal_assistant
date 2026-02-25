/**
 * @projectName: personal_assistant
 * @package: request
 * @className: roleReq
 * @author: lijunqi
 * @description: 角色管理请求DTO
 * @date: 2026-02-02
 * @Version: 1.0
 */
package request

// RoleListFilter 角色列表查询过滤条件
type RoleListFilter struct {
	// 页码，默认1
	Page int `form:"page"`
	// 每页数量，默认10
	PageSize int `form:"page_size"`
	// 状态过滤：1启用 0禁用，nil表示不过滤
	Status *int `form:"status"`
	// 按名称或code模糊搜索
	Keyword string `form:"keyword"`
}

// CreateRoleReq 创建角色请求
type CreateRoleReq struct {
	// 角色名称，必填，最大20字符
	Name string `json:"name" binding:"required,max=20"`
	// 角色代码，必填，唯一标识，最大20字符
	Code string `json:"code" binding:"required,max=20"`
	// 角色描述，可选，最大200字符
	Desc string `json:"desc" binding:"max=200"`
}

// UpdateRoleReq 更新角色请求（全部可选，支持部分更新）
type UpdateRoleReq struct {
	// 角色名称
	Name *string `json:"name"`
	// 角色代码
	Code *string `json:"code"`
	// 角色描述
	Desc *string `json:"desc"`
	// 状态：1启用 0禁用
	Status *int `json:"status"`
}

// AssignRoleMenuReq 分配菜单权限请求
type AssignRoleMenuReq struct {
	// 角色ID，必填
	RoleID uint `json:"role_id" binding:"required"`
	// 菜单ID列表（全量替换），必填
	MenuIDs []uint `json:"menu_ids" binding:"required"`
}

// AssignRoleAPIReq 分配角色API权限请求（全量替换）
type AssignRoleAPIReq struct {
	// 角色ID，必填
	RoleID uint `json:"role_id" binding:"required"`
	// API ID列表（允许空数组，空表示清空角色直绑API权限）
	APIIDs []uint `json:"api_ids"`
}
