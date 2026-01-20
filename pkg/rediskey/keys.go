package rediskey

import "fmt"

const (
	LuoguProblemBankHashKey = "luogu:problem_bank:pid_id"
	rankingZSetKeyFmt       = "ranking:%d:%s"
)

func RankingZSetKey(orgID uint, platform string) string {
	return fmt.Sprintf(rankingZSetKeyFmt, orgID, platform)
}
