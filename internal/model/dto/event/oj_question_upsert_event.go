package event

// QuestionUpsertedEvent 表示本地权威题库新增或更新了一道题目。
// OJTask 通过该事件异步回填 pending_resolution 的任务项。
type QuestionUpsertedEvent struct {
	Platform     string `json:"platform"`
	QuestionID   uint   `json:"question_id"`
	QuestionCode string `json:"question_code"`
	Title        string `json:"title"`
}
