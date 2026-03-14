package rediskey

import "fmt"

const (
	// 洛谷总题库
	LuoguProblemBankHashKey     = "luogu:problem_bank:pid_id"
	LeetcodeProblemBankHashKey  = "leetcode:problem_bank:slug_id"
	LanqiaoProblemBankHashKey   = "lanqiao:problem_bank:problem_id_id"
	rankingAllMembersZSetKeyFmt = "ranking:all_members:%s"
	rankingOrgZSetKeyFmt        = "ranking:org:%d:%s"
	rankingUserHashKeyFmt       = "ranking:user:%d"
	// 用户活跃态缓存 key。
	userActiveStateKeyFmt = "user:active_state:%d"
	lanqiaoSyncFailKeyFmt = "lanqiao:sync:fail:%d"
	lanqiaoSyncDisableFmt = "lanqiao:sync:disable:%d"
	lanqiaoSubmitSeenFmt  = "lanqiao:sync:submission_seen:%d:%s"
)

// RankingZSetKey 生成排行榜 zset key。
func RankingZSetKey(orgID uint, platform string) string {
	return RankingOrgZSetKey(orgID, platform)
}

// RankingAllMembersZSetKey 生成全站排行榜 zset key。
func RankingAllMembersZSetKey(platform string) string {
	return fmt.Sprintf(rankingAllMembersZSetKeyFmt, platform)
}

// RankingUserHashKey 生成用户详情 hash key。
func RankingOrgZSetKey(orgID uint, platform string) string {
	return fmt.Sprintf(rankingOrgZSetKeyFmt, orgID, platform)
}

// RankingUserHashKey 生成用户详情 hash key。
func RankingUserHashKey(userID uint) string {
	return fmt.Sprintf(rankingUserHashKeyFmt, userID)
}

// UserActiveStateKey 生成用户活跃态缓存 key。
func UserActiveStateKey(userID uint) string {
	return fmt.Sprintf(userActiveStateKeyFmt, userID)
}

func LanqiaoSyncFailKey(userID uint) string {
	return fmt.Sprintf(lanqiaoSyncFailKeyFmt, userID)
}

func LanqiaoSyncDisableKey(userID uint) string {
	return fmt.Sprintf(lanqiaoSyncDisableFmt, userID)
}

func LanqiaoSubmissionSeenKey(userID uint, fingerprint string) string {
	return fmt.Sprintf(lanqiaoSubmitSeenFmt, userID, fingerprint)
}
