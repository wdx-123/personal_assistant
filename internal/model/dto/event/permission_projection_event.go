package event

// PermissionProjectionEvent 表示权限投影链路中的异步事件。
type PermissionProjectionEvent struct {
	Kind          string `json:"kind"` // 事件类型，如 "subject_binding_changed" 或 "permission_graph_changed"
	UserID        uint   `json:"user_id,omitempty"`
	OrgID         uint   `json:"org_id,omitempty"` // 相关组织ID（如果适用）
	AggregateType string `json:"aggregate_type,omitempty"` // 聚合类型，如 "user"、"org" 等
	AggregateID   uint   `json:"aggregate_id,omitempty"` // 聚合ID，通常是用户ID或组织ID
}

const (
	
	// PermissionProjectionKindSubjectBindingChanged 表示用户的权限主体绑定发生了变化（如加入/退出组织、被踢出组织等）。
	PermissionProjectionKindSubjectBindingChanged = "subject_binding_changed"

	// PermissionProjectionKindPermissionGraphChanged 表示用户的权限图发生了变化（如权限分配、回收等）。
	PermissionProjectionKindPermissionGraphChanged = "permission_graph_changed"
)