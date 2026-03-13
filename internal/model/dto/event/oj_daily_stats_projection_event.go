package event

type OJDailyStatsProjectionEvent struct {
	Kind       string `json:"kind"`
	UserID     uint   `json:"user_id"`
	Platform   string `json:"platform"`
	WindowDays int    `json:"window_days,omitempty"`
}

const (
	OJDailyStatsProjectionKindRefreshRecentWindow         = "refresh_recent_window"
	OJDailyStatsProjectionKindResetAndRebuildRecentWindow = "reset_and_rebuild_recent_window"
)
