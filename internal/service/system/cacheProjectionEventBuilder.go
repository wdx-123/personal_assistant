package system

import (
	"strings"

	eventdto "personal_assistant/internal/model/dto/event"
	"personal_assistant/pkg/rankingcache"
)

// newUserSnapshotProjectionEvent 构建一个用户快照变更事件的缓存投影事件，
// 包含用户ID和受影响的组织ID列表等信息。
func newUserSnapshotProjectionEvent(
	userID uint,
	affectedOrgIDs []uint,
) *eventdto.CacheProjectionEvent {
	return newCacheProjectionEvent(
		eventdto.CacheProjectionKindUserSnapshotChanged,
		userID,
		"",
		nil,
		nil,
		affectedOrgIDs,
	)
}

// newCurrentOrgChangedProjectionEvent 构建一个当前组织变更事件的缓存投影事件，
// 包含用户ID、旧的当前组织ID、新的当前组织ID和受影响的组织ID列表等信息。
func newCurrentOrgChangedProjectionEvent(
	userID uint,
	oldCurrentOrgID *uint,
	newCurrentOrgID *uint,
	affectedOrgIDs []uint,
) *eventdto.CacheProjectionEvent {
	return newCacheProjectionEvent(
		eventdto.CacheProjectionKindCurrentOrgChanged,
		userID,
		"",
		oldCurrentOrgID,
		newCurrentOrgID,
		affectedOrgIDs,
	)
}

// newOJProfileChangedProjectionEvent 构建一个 OJ 账号信息变更事件的缓存投影事件，包含用户ID、平台等信息。
func newOJProfileChangedProjectionEvent(
	userID uint,
	platform string,
) *eventdto.CacheProjectionEvent {
	return newCacheProjectionEvent(
		eventdto.CacheProjectionKindOJProfileChanged,
		userID,
		platform,
		nil,
		nil,
		nil,
	)
}

// newUserDeletedProjectionEvent 构建一个用户删除事件的缓存投影事件，
// 包含用户ID、旧的当前组织ID和受影响的组织ID列表等信息。
func newUserDeletedProjectionEvent(
	userID uint,
	oldCurrentOrgID *uint,
	affectedOrgIDs []uint,
) *eventdto.CacheProjectionEvent {
	return newCacheProjectionEvent(
		eventdto.CacheProjectionKindUserDeleted,
		userID,
		"",
		oldCurrentOrgID,
		nil,
		affectedOrgIDs,
	)
}

// newCacheProjectionEvent 是一个通用的工厂函数，用于创建不同类型的缓存投影事件。它会根据传入的参数构建一个 CacheProjectionEvent 实例，并进行必要的规范化和去重处理。
func newCacheProjectionEvent(
	kind string,
	userID uint,
	platform string,
	oldCurrentOrgID *uint,
	newCurrentOrgID *uint,
	affectedOrgIDs []uint,
) *eventdto.CacheProjectionEvent {
	if userID == 0 {
		return nil
	}
	return &eventdto.CacheProjectionEvent{
		Kind:            strings.TrimSpace(kind),
		UserID:          userID,
		Platform:        normalizeProjectionPlatform(platform),
		OldCurrentOrgID: cloneUintPtr(oldCurrentOrgID),
		NewCurrentOrgID: cloneUintPtr(newCurrentOrgID),
		AffectedOrgIDs:  dedupeUintIDs(affectedOrgIDs),
	}
}

// normalizeProjectionPlatform 对平台字符串进行规范化处理，返回标准化后的平台名称
func normalizeProjectionPlatform(platform string) string {
	platform = strings.TrimSpace(platform)
	if platform == "" {
		return ""
	}
	return rankingcache.NormalizePlatform(platform)
}

// cloneUintPtr 克隆一个 *uint 指针，避免外部修改原始值
func cloneUintPtr(value *uint) *uint {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}

// dedupeUintIDs 去重并过滤掉零值的 uint ID 列表，返回新的切片
func dedupeUintIDs(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
