package consts

// CapabilitySeed 描述系统内置的 capability 定义。
type CapabilitySeed struct {
	Code      string
	Name      string
	Domain    string
	GroupCode string
	GroupName string
	Desc      string
}

const (
	CapabilityDomainOrgMember              = "org_member"
	CapabilityGroupCodeOrgMemberManagement = "org_member_management"
	CapabilityGroupNameOrgMemberManagement = "成员管理"
	CapabilityCodeOrgMemberKick            = "org.member.kick"
	CapabilityCodeOrgMemberRecover         = "org.member.recover"
	CapabilityCodeOrgMemberFreeze          = "org.member.freeze"
	CapabilityCodeOrgMemberDelete          = "org.member.delete"
	CapabilityCodeOrgMemberInvite          = "org.member.invite"
	CapabilityCodeOrgMemberAssignRole      = "org.member.assign_role"
	// 以下是一些常用的成员操作标识，可以在日志记录或事件追踪中使用
	OrgMemberActionKick       = "kick"        // 踢出成员
	OrgMemberActionRecover    = "recover"     // 恢复成员
	OrgMemberActionFreeze     = "freeze"      // 冻结成员
	OrgMemberActionDelete     = "delete"      // 删除成员
	OrgMemberActionInvite     = "invite"      // 邀请成员
	OrgMemberActionAssignRole = "assign_role" // 分配角色
)

var builtinCapabilitySeeds = []CapabilitySeed{
	{
		Code:      CapabilityCodeOrgMemberKick,
		Name:      "踢出成员",
		Domain:    CapabilityDomainOrgMember,
		GroupCode: CapabilityGroupCodeOrgMemberManagement,
		GroupName: CapabilityGroupNameOrgMemberManagement,
		Desc:      "允许将组织成员踢出组织",
	},
	{
		Code:      CapabilityCodeOrgMemberRecover,
		Name:      "恢复成员",
		Domain:    CapabilityDomainOrgMember,
		GroupCode: CapabilityGroupCodeOrgMemberManagement,
		GroupName: CapabilityGroupNameOrgMemberManagement,
		Desc:      "允许将 left 或 removed 成员恢复为 active",
	},
	{
		Code:      CapabilityCodeOrgMemberFreeze,
		Name:      "冻结成员",
		Domain:    CapabilityDomainOrgMember,
		GroupCode: CapabilityGroupCodeOrgMemberManagement,
		GroupName: CapabilityGroupNameOrgMemberManagement,
		Desc:      "允许冻结组织内成员相关账号或资格",
	},
	{
		Code:      CapabilityCodeOrgMemberDelete,
		Name:      "删除成员",
		Domain:    CapabilityDomainOrgMember,
		GroupCode: CapabilityGroupCodeOrgMemberManagement,
		GroupName: CapabilityGroupNameOrgMemberManagement,
		Desc:      "允许删除组织内成员关系或成员数据",
	},
	{
		Code:      CapabilityCodeOrgMemberInvite,
		Name:      "邀请成员",
		Domain:    CapabilityDomainOrgMember,
		GroupCode: CapabilityGroupCodeOrgMemberManagement,
		GroupName: CapabilityGroupNameOrgMemberManagement,
		Desc:      "允许邀请用户加入组织",
	},
	{
		Code:      CapabilityCodeOrgMemberAssignRole,
		Name:      "分配成员角色",
		Domain:    CapabilityDomainOrgMember,
		GroupCode: CapabilityGroupCodeOrgMemberManagement,
		GroupName: CapabilityGroupNameOrgMemberManagement,
		Desc:      "允许调整组织成员在本组织下的角色",
	},
}

// BuiltinCapabilitySeeds 返回 capability 种子定义副本。
func BuiltinCapabilitySeeds() []CapabilitySeed {
	dst := make([]CapabilitySeed, len(builtinCapabilitySeeds))
	copy(dst, builtinCapabilitySeeds)
	return dst
}

// OrgMemberCapabilityCodes 返回成员管理相关 capability code 列表副本。
func OrgMemberCapabilityCodes() []string {
	codes := []string{
		CapabilityCodeOrgMemberKick,
		CapabilityCodeOrgMemberRecover,
		CapabilityCodeOrgMemberFreeze,
		CapabilityCodeOrgMemberDelete,
		CapabilityCodeOrgMemberInvite,
		CapabilityCodeOrgMemberAssignRole,
	}
	dst := make([]string, len(codes))
	copy(dst, codes)
	return dst
}
