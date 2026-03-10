package consts

// UserStatus 账号状态
type UserStatus int8

const (
	UserStatusActive      UserStatus = 1 // 正常可用
	UserStatusDisabled    UserStatus = 2 // 禁用
	UserStatusDeletedSoft UserStatus = 3 // 软删除（清理后）
)
