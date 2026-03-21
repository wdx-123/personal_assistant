package system

import (
	"fmt"
	"strings"

	"personal_assistant/internal/model/consts"

	"gorm.io/gorm"
)

func applyRegisteredAPIFilter(query *gorm.DB, alias string) *gorm.DB {
	return query.Where(registeredAPIWhereClause(alias), consts.APISyncStateRegistered, 1)
}

func applyAPISyncStateFilter(query *gorm.DB, alias, syncState string) *gorm.DB {
	syncState = strings.TrimSpace(syncState)
	if syncState == "" {
		return query
	}
	if syncState == consts.APISyncStateRegistered {
		return query.Where(registeredSyncStateWhereClause(alias), consts.APISyncStateRegistered)
	}
	return query.Where(fmt.Sprintf("%ssync_state = ?", columnPrefix(alias)), syncState)
}

func registeredAPIWhereClause(alias string) string {
	prefix := columnPrefix(alias)
	return fmt.Sprintf(
		"(%ssync_state = ? OR %ssync_state = '' OR %ssync_state IS NULL) AND %sstatus = ?",
		prefix,
		prefix,
		prefix,
		prefix,
	)
}

func registeredSyncStateWhereClause(alias string) string {
	prefix := columnPrefix(alias)
	return fmt.Sprintf("(%ssync_state = ? OR %ssync_state = '' OR %ssync_state IS NULL)", prefix, prefix, prefix)
}

func columnPrefix(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return ""
	}
	return alias + "."
}
