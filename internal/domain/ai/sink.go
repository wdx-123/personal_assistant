package ai

import "context"

// Sink 表示 runtime 事件的承接端。
// 参数：
//   - ctx：调用链上下文。
//   - event：runtime 产生的领域事件。
//
// 核心流程：
//  1. Runtime 调用 Emit 输出事件。
//  2. Service 层具体实现 Sink，把事件写入 SSE。
//  3. Service 层同时把事件投影到 assistant message，并通过 Repository 落库。
//
// 注意事项：
//   - Runtime 不应该绕过 Sink 直接写 HTTP 或数据库。
type Sink interface {
	Emit(ctx context.Context, event Event) error
	Heartbeat(ctx context.Context) error
}
