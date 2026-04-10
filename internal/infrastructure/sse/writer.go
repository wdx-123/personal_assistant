package sse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ErrStreamingUnsupported 表示底层 ResponseWriter 不支持流式刷新。
// 在这种情况下继续走 SSE 会造成客户端长时间收不到数据，因此需要尽早报错。
var ErrStreamingUnsupported = errors.New("streaming unsupported")

// HTTPStreamWriter 负责把 StreamEvent 以 SSE 协议写入 HTTP 响应。
// 它通过互斥锁串行化写操作，避免多 goroutine 并发写同一条 HTTP 连接导致协议内容交叉。
type HTTPStreamWriter struct {
	w      http.ResponseWriter
	rc     *http.ResponseController
	policy ConnectionPolicy

	mu       sync.Mutex
	prepared bool
	started  bool
}

// NewHTTPStreamWriter 创建一个面向 HTTP 的 SSE 写出器。
// 参数：
//   - w：HTTP 响应写入器。
//   - policy：连接策略，主要用于写超时控制。
//
// 返回值：
//   - *HTTPStreamWriter：可复用的流写出器。
//
// 核心流程：
//  1. 创建 ResponseController，优先使用更现代的 flush 与 deadline 能力。
//  2. 归一化策略，确保写超时存在稳定默认值。
//
// 注意事项：
//   - 写出器本身不负责连接生命周期，仅负责协议层写入和 header 准备。
func NewHTTPStreamWriter(w http.ResponseWriter, policy ConnectionPolicy) *HTTPStreamWriter {
	return &HTTPStreamWriter{
		w:      w,
		rc:     http.NewResponseController(w),
		policy: policy.Normalize(),
	}
}

// Started 返回是否已经成功向客户端写出过任何内容。
// 参数：无。
// 返回值：
//   - bool：true 表示响应头或事件正文已经开始发送。
//
// 核心流程：
//  1. 通过互斥锁保护 started 标记，避免并发读写竞争。
//
// 注意事项：
//   - 控制器层会用它判断错误发生时能否继续回写标准 JSON 响应。
func (h *HTTPStreamWriter) Started() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started
}

// WriteEvent 负责把一个标准事件编码并写出到客户端。
// 参数：
//   - ctx：写出上下文，用于截止时间控制。
//   - evt：待发送事件。
//
// 返回值：
//   - error：编码、写入或 flush 失败时返回错误。
//
// 核心流程：
//  1. 空事件直接跳过，避免输出无意义帧。
//  2. 先按 SSE 规范编码，再走统一 write 路径。
//
// 注意事项：
//   - 所有正式事件都走同一个底层 write，可以保证 header 准备与超时策略一致。
func (h *HTTPStreamWriter) WriteEvent(ctx context.Context, evt *StreamEvent) error {
	if evt == nil {
		return nil
	}
	payload, err := EncodeEvent(evt)
	if err != nil {
		return err
	}
	return h.write(ctx, payload)
}

// WriteHeartbeat 负责发送注释型心跳帧。
// 参数：
//   - ctx：写出上下文。
//
// 返回值：
//   - error：写入失败时返回错误。
//
// 核心流程：
//  1. 复用统一 write 路径发送 `: keepalive` 注释帧。
//
// 注意事项：
//   - 选择注释帧而不是业务事件，是为了避免客户端把心跳误当成真实消息处理。
func (h *HTTPStreamWriter) WriteHeartbeat(ctx context.Context) error {
	return h.write(ctx, []byte(": keepalive\n\n"))
}

// WriteTerminal 负责写出终止事件。
// 参数与返回值与 WriteEvent 相同。
// 核心流程：
//  1. 当前实现直接复用普通事件写出逻辑。
//
// 注意事项：
//   - 单独保留该方法是为了将来在终止帧上附加特殊 header 或埋点时不改接口。
func (h *HTTPStreamWriter) WriteTerminal(ctx context.Context, evt *StreamEvent) error {
	return h.WriteEvent(ctx, evt)
}

// write 负责执行真正的 HTTP 写出。
// 参数：
//   - ctx：写出上下文。
//   - payload：已经编码好的 SSE 文本。
//
// 返回值：
//   - error：准备响应头、设置写超时、写正文或 flush 失败时返回错误。
//
// 核心流程：
//  1. 通过互斥锁保证单连接写操作串行。
//  2. 首次写入前准备 SSE 响应头并立即 flush。
//  3. 为当前写操作设置 deadline，避免网络挂死时无限阻塞。
//  4. 写入 payload 并再次 flush，让客户端尽快看到数据。
//
// 注意事项：
//   - started 只在真正写成功后置为 true，这样上层才能准确判断“是否还能退回普通响应”。
func (h *HTTPStreamWriter) write(ctx context.Context, payload []byte) error {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	h.mu.Lock()
	defer h.mu.Unlock()

	// 首次写入时必须先输出 SSE 所需响应头，否则客户端和代理可能不会按流式处理。
	if err := h.ensurePreparedLocked(); err != nil {
		return err
	}

	// 每次写入前都重置 deadline，避免长连接场景下复用过期的旧截止时间。
	if err := h.setWriteDeadlineLocked(ctx); err != nil {
		return err
	}
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	if _, err := h.w.Write(payload); err != nil {
		return err
	}
	if err := h.flushLocked(); err != nil {
		return err
	}
	h.started = true
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return nil
}

// ensurePreparedLocked 负责在首次写入前设置 SSE 所需响应头。
// 参数：无。
// 返回值：
//   - error：首次 flush 失败时返回错误。
//
// 核心流程：
//  1. 若已准备过则直接返回。
//  2. 设置 SSE 所需的 Content-Type、禁止缓存和关闭代理缓冲。
//  3. 发送 200 状态码并立即 flush，让客户端尽早进入流式读取状态。
//
// 注意事项：
//   - 这里要求调用方已持有互斥锁，避免 header 被并发重复写入。
func (h *HTTPStreamWriter) ensurePreparedLocked() error {
	if h.prepared {
		return nil
	}
	header := h.w.Header()
	header.Set("Content-Type", "text/event-stream; charset=utf-8")
	header.Set("Cache-Control", "no-cache")
	header.Set("X-Accel-Buffering", "no")
	header.Set("Connection", "keep-alive")
	h.w.WriteHeader(http.StatusOK)
	h.prepared = true
	return h.flushLocked()
}

// setWriteDeadlineLocked 根据连接策略和上下文设置本次写入截止时间。
// 参数：
//   - ctx：可能自带 Deadline 的上下文。
//
// 返回值：
//   - error：底层 ResponseController 不支持以外的错误。
//
// 核心流程：
//  1. 先以策略默认写超时作为基线。
//  2. 若上下文已有更早的截止时间，则优先采用更严格的那个。
//  3. 调用 ResponseController 设置 deadline。
//
// 注意事项：
//   - 忽略 `http.ErrNotSupported` 是为了兼容不支持 deadline 的 writer；这类场景仍可继续尝试流式输出。
func (h *HTTPStreamWriter) setWriteDeadlineLocked(ctx context.Context) error {
	deadline := time.Now().Add(h.policy.WriteTimeout)
	if ctx != nil {
		if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
			deadline = dl
		}
	}
	if err := h.rc.SetWriteDeadline(deadline); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	return nil
}

// flushLocked 负责把已写入的字节尽快刷到客户端。
// 参数：无。
// 返回值：
//   - error：当前 writer 完全不支持 flush 时返回 ErrStreamingUnsupported。
//
// 核心流程：
//  1. 优先尝试 ResponseController.Flush。
//  2. 若不支持，再退回传统的 http.Flusher。
//  3. 两者都不可用时明确报错。
//
// 注意事项：
//   - 这里同样要求调用方已持有互斥锁，避免 flush 与并发写混在一起。
func (h *HTTPStreamWriter) flushLocked() error {
	if err := h.rc.Flush(); err == nil {
		return nil
	}
	if flusher, ok := h.w.(http.Flusher); ok {
		flusher.Flush()
		return nil
	}
	return ErrStreamingUnsupported
}

// LastEventIDFromRequest 从 HTTP 请求头中提取 SSE 标准的 Last-Event-ID。
// 参数：
//   - r：HTTP 请求对象。
//
// 返回值：
//   - string：客户端声明的最后已确认事件 ID。
//
// 核心流程：
//  1. 对空请求做兜底。
//  2. 直接读取标准 Header。
//
// 注意事项：
//   - 该函数只负责取值，不做格式校验；真正的回放语义由 ReplayStore 决定。
func LastEventIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.Header.Get("Last-Event-ID")
}

// EncodeEvent 按 SSE 文本协议编码事件。
// 参数：
//   - evt：待编码事件。
//
// 返回值：
//   - []byte：符合 SSE 规范的文本负载。
//   - error：格式化写入失败时返回错误。
//
// 核心流程：
//  1. 依次输出 retry、id、event 等元信息字段。
//  2. 把 data 按行拆分成多个 `data:` 前缀，兼容多行文本。
//  3. 最后补一个空行，表示当前事件结束。
//
// 注意事项：
//   - 先去掉 `\r` 再按 `\n` 分行，是为了兼容不同平台换行符并避免客户端看到重复空行。
func EncodeEvent(evt *StreamEvent) ([]byte, error) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	if evt == nil {
		return nil, nil
	}

	var buf bytes.Buffer
	if evt.RetryMS > 0 {
		_, _ = fmt.Fprintf(&buf, "retry: %d\n", evt.RetryMS)
	}
	if evt.EventID != "" {
		_, _ = fmt.Fprintf(&buf, "id: %s\n", evt.EventID)
	}
	if evt.EventName != "" {
		_, _ = fmt.Fprintf(&buf, "event: %s\n", evt.EventName)
	}

	// 先统一换行格式，再逐行输出 `data:`，这是 SSE 处理多行 payload 的标准方式。
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	data := bytes.ReplaceAll(evt.Data, []byte("\r"), nil)
	lines := bytes.Split(data, []byte("\n"))
	for _, line := range lines {
		if _, err := fmt.Fprintf(&buf, "data: %s\n", line); err != nil {
			return nil, err
		}
	}

	// 事件之间必须用空行分隔，否则客户端无法正确识别边界。
	if _, err := io.WriteString(&buf, "\n"); err != nil {
		return nil, err
	}
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	return buf.Bytes(), nil
}
