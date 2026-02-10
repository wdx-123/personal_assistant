package consts

// 内置角色Code常量（受保护：不可删除、不可修改code）
const (
	RoleCodeSuperAdmin = "super_admin" // 系统超级管理员（全局最高权限，管理所有组织和用户）
	RoleCodeOrgAdmin   = "org_admin"   // 组织管理员（管理单个组织内的一切）
	RoleCodeMember     = "member"      // 普通成员
)

// builtinRoleCodes 受保护的内置角色code列表
var builtinRoleCodes = []string{RoleCodeSuperAdmin, RoleCodeOrgAdmin}

// BuiltinRoleCodes 返回所有受保护的内置角色code
func BuiltinRoleCodes() []string {
	dst := make([]string, len(builtinRoleCodes))
	copy(dst, builtinRoleCodes)
	return dst
}

// IsBuiltinRole 判断是否为受保护的内置角色
func IsBuiltinRole(code string) bool {
	for _, c := range builtinRoleCodes {
		if c == code {
			return true
		}
	}
	return false
}
