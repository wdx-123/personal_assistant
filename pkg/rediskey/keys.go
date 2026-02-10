package rediskey

import "fmt"

const (
	// 洛谷总题库
	LuoguProblemBankHashKey    = "luogu:problem_bank:pid_id"
	LeetcodeProblemBankHashKey = "leetcode:problem_bank:slug_id"
	// 洛谷排行榜（键为 orgID:platform）（通过组织与平台分开维护）
	rankingZSetKeyFmt = "ranking:%d:%s"
)

func RankingZSetKey(orgID uint, platform string) string {
	return fmt.Sprintf(rankingZSetKeyFmt, orgID, platform)
}
