package system

import (
	"strings"

	"personal_assistant/internal/model/consts"
	"personal_assistant/internal/model/entity"
)

func isProjectableAPI(api *entity.API) bool {
	if api == nil || api.Status != 1 {
		return false
	}
	syncState := strings.TrimSpace(api.SyncState)
	return syncState == "" || syncState == consts.APISyncStateRegistered
}
