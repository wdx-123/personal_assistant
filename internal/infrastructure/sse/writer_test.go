package sse

import (
	"strings"
	"testing"
)

// TestEncodeEvent_MultilineData 验证多行 data 在编码后会被拆成多条 `data:` 行。
// 参数：
//   - t：测试上下文。
//
// 返回值：无。
// 核心流程：
//  1. 构造包含重试、事件名和多行 data 的事件。
//  2. 调用 EncodeEvent 得到实际 SSE 文本。
//  3. 逐项断言关键片段都存在，确保编码格式符合预期。
//
// 注意事项：
//   - 这里不逐字节比较整段文本，而是按关键片段断言，能让测试在换行之外的轻微格式调整下更稳健。
func TestEncodeEvent_MultilineData(t *testing.T) {
	// 第一阶段：先处理入口参数、依赖或前置状态，尽早挡住不能继续推进的情况。
	// 把前置判断集中在这里，是为了避免后续主逻辑夹杂过多防御性分支。
	payload, err := EncodeEvent(&StreamEvent{
		EventID:   "evt-1",
		EventName: "assistant_token",
		RetryMS:   1500,
		Data:      []byte("line1\nline2"),
	})
	if err != nil {
		t.Fatalf("EncodeEvent() error = %v", err)
	}

	// 把结果转成文本后按关键片段核对，是为了明确覆盖 retry、id、event 和多行 data 的编码规则。
	// 第二阶段：进入当前函数的主体逻辑，逐步组装中间结果或推进状态。
	// 这里单独分段，是为了让阅读者更容易看清主要业务动作发生的位置。
	text := string(payload)
	// 第三阶段：统一收口结果、状态更新或返回动作，保证对外行为一致。
	// 把收尾逻辑显式标出来，可以降低后续维护时遗漏边界处理的风险。
	for _, want := range []string{
		"retry: 1500\n",
		"id: evt-1\n",
		"event: assistant_token\n",
		"data: line1\n",
		"data: line2\n\n",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("encoded payload missing %q, got %q", want, text)
		}
	}
}
