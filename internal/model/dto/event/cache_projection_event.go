package event

// CacheProjectionEvent 表示用于刷新 Redis 排行榜/详情缓存的异步投影事件。
type CacheProjectionEvent struct {
	Kind            string `json:"kind"`
	UserID          uint   `json:"user_id"`
	Platform        string `json:"platform,omitempty"`
	OldCurrentOrgID *uint  `json:"old_current_org_id,omitempty"`
	NewCurrentOrgID *uint  `json:"new_current_org_id,omitempty"`
	AffectedOrgIDs  []uint `json:"affected_org_ids,omitempty"` // 受影响的组织 ID 列表，通常用于需要更新多个排行榜的情况
}

const (
	CacheProjectionKindUserSnapshotChanged = "user_snapshot_changed" // 用户快照（包含基本信息和所有平台资料）发生变化，通常用于全量更新缓存
	CacheProjectionKindCurrentOrgChanged   = "current_org_changed"   // 当前组织发生变化
	CacheProjectionKindOJProfileChanged    = "oj_profile_changed"    // OJ 资料发生变化
	CacheProjectionKindUserDeleted         = "user_deleted"          // 用户被删除
)
