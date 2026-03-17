package event

// OJTaskExecutionTriggerEvent 是 OJTask 即时执行的唤醒事件。
// 它只承载最小定位信息，真正业务执行仍回 DB 读取 execution 真相。
type OJTaskExecutionTriggerEvent struct {
	ExecutionID uint `json:"execution_id"`
	TaskID      uint `json:"task_id"`
}
