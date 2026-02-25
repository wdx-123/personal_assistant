/**
 * @projectName: personal_assistant
 * @package: response
 * @className: roleResp
 * @author: lijunqi
 * @description: 角色管理响应DTO
 * @date: 2026-02-02
 * @Version: 1.0
 */
package response

import "time"

// RoleItem 角色项（列表/详情通用）
type RoleItem struct {
	// 角色ID
	ID uint `json:"id"`
	// 角色名称
	Name string `json:"name"`
	// 角色代码（唯一标识）
	Code string `json:"code"`
	// 角色描述
	Desc string `json:"desc"`
	// 状态：1启用 0禁用
	Status int `json:"status"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// RoleListResp 角色分页列表响应
type RoleListResp struct {
	// 角色列表
	List []*RoleItem `json:"list"`
	// 总数
	Total int64 `json:"total"`
	// 当前页码
	Page int `json:"page"`
	// 每页大小
	PageSize int `json:"page_size"`
}

// RoleSimpleItem 角色简要信息（用于下拉选择等场景）
type RoleSimpleItem struct {
	// 角色ID
	ID uint `json:"id"`
	// 角色名称
	Name string `json:"name"`
	// 角色代码
	Code string `json:"code"`
}

// RoleMenuAPIMappingItem 角色菜单/API映射（配置态）
type RoleMenuAPIMappingItem struct {
	MenuTree []*MenuItem `json:"menu_tree"` // 全量菜单树（节点包含已绑定APIs）
}
