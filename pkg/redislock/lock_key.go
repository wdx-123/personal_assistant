package redislock

import "fmt"

const (
	LockKeyLuoguSyncAllUsers         = "luogu:sync:all_users"         // 洛谷全量同步锁
	LockKeyLeetcodeSyncAllUsers      = "leetcode:sync:all_users"      // 力扣全量同步锁
	lockKeyLuoguBindUserFmt          = "luogu:bind:user:%d"           // 洛谷按用户绑定锁
	lockKeyLuoguBindIdentifierFmt    = "luogu:bind:identifier:%s"     // 洛谷按标识绑定锁
	lockKeyLuoguUserSyncSingleFmt    = "luogu:sync:user:%s"           // 洛谷单用户同步锁
	lockKeyLeetcodeUserSyncSingleFmt = "leetcode:sync:user:%s"        // 力扣单用户同步锁
	LockKeyLuoguProblemBankSyncKey   = "luogu:sync:problem_bank"      // 洛谷题库同步锁
	LockKeyLuoguProblemBankWarmup    = "luogu:warmup:problem_bank"    // 洛谷题库预热锁
	LockKeyLeetcodeProblemBankWarmup = "leetcode:warmup:problem_bank" // 力扣题库预热锁
	lockKeyRoleMenuAssignFmt         = "role:menu:assign:%d"          // 角色菜单分配锁
)

func LockKeyLuoguBindUser(userID uint) string {
	return fmt.Sprintf(lockKeyLuoguBindUserFmt, userID)
}

func LockKeyLuoguBindIdentifier(identifier string) string {
	return fmt.Sprintf(lockKeyLuoguBindIdentifierFmt, identifier)
}

func LockKeyLuoguSyncSingleUser(identifier string) string {
	return fmt.Sprintf(lockKeyLuoguUserSyncSingleFmt, identifier)
}

func LockKeyLeetcodeSyncSingleUser(identifier string) string {
	return fmt.Sprintf(lockKeyLeetcodeUserSyncSingleFmt, identifier)
}

// LockKeyRoleMenuAssign 生成角色菜单分配锁Key
func LockKeyRoleMenuAssign(roleID uint) string {
	return fmt.Sprintf(lockKeyRoleMenuAssignFmt, roleID)
}
