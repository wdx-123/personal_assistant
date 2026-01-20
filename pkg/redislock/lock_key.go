package redislock

import "fmt"

const (
	LockKeyLuoguSyncAllUsers       = "luogu:sync:all_users"
	lockKeyLuoguBindUserFmt        = "luogu:bind:user:%d"
	lockKeyLuoguBindIdentifierFmt  = "luogu:bind:identifier:%s"
	lockKeyLuoguUserSyncSingleFmt  = "luogu:sync:user:%s"
	LockKeyLuoguProblemBankSyncKey = "luogu:sync:problem_bank"
	LockKeyLuoguProblemBankWarmup  = "luogu:warmup:problem_bank"
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
