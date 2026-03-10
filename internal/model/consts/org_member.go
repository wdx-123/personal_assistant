package consts

// OrgMemberStatus 组织成员状态
type OrgMemberStatus int8

const (
	OrgMemberStatusActive  OrgMemberStatus = 1 // 正常成员
	OrgMemberStatusLeft    OrgMemberStatus = 2 // 主动退出
	OrgMemberStatusRemoved OrgMemberStatus = 3 // 被踢出
)

// OrgMemberJoinSource 成员加入来源
type OrgMemberJoinSource string

const (
	OrgMemberJoinSourceRegister       OrgMemberJoinSource = "register"        // 注册加入
	OrgMemberJoinSourceInvite         OrgMemberJoinSource = "invite"          // 邀请加入
	OrgMemberJoinSourceOrgCreate      OrgMemberJoinSource = "org_create"      // 创建组织时加入
	OrgMemberJoinSourceAdminRecover   OrgMemberJoinSource = "admin_recover"   // 管理员恢复
	OrgMemberJoinSourceLegacyBackfill OrgMemberJoinSource = "legacy_backfill" // 旧数据迁移
	OrgMemberJoinSourceSystemBackfill OrgMemberJoinSource = "system_backfill" // 系统自动补全
)
