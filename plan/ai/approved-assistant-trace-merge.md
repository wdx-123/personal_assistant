# 目标

将工具调用轨迹并入普通 assistant 消息显示，前端不再为 `trace_items` 单独渲染额外 UI。

# 范围

- 保持后端 `ai_messages.content` 与 `ai_messages.trace_items_json` 存储模型不变
- 保持历史消息接口继续返回 `content + trace_items`
- 保持 SSE 继续下发 `tool_call_started / tool_call_finished / assistant_token / message_completed / error / done`
- 前端改为把 `trace_items` 折叠成普通文本并与 `content` 合并显示

# 改动

- 后端校准 `AssistantMessageResp` 注释、SSE 协议说明、OpenAPI 示例与现行 `tool_call_*` 事件保持一致
- 前端在 `assistant` store 中消费 `tool_call_started` 与 `tool_call_finished` 并 upsert 到当前 assistant 消息的 `trace_items`
- 前端取消对 `trace_items` 的按 key 重排，保留服务端顺序
- 前端新增消息格式化逻辑，将 `trace_items` 折叠成简短普通文本并在渲染时与 `content` 合并

# 验证

- `go test ./internal/service/system -run "AIMessageProjector|AIStreamSink|AIService"`
- `go test ./internal/infrastructure/ai/eino -run Tool`
- `npx vue-tsc --noEmit`
- `npm run build`

# 风险

- OpenAPI 示例与前端真实消费路径存在旧描述残留，需要同步清理
- 历史消息与流式消息都依赖同一套 trace 折叠格式，需避免两端表现不一致
- 多工具场景必须保留服务端顺序，不能继续被前端排序逻辑打乱

# 执行顺序

1. 校准后端协议注释、OpenAPI 与相关测试
2. 调整前端类型与 store 的 SSE 消费
3. 调整 assistant 消息合并渲染
4. 跑后端测试与前端校验

# 待确认

- 默认展示顺序为工具过程文本在前、最终正文在后
- `detail_markdown` 仅在失败或摘要不足时做最小补充，不在成功场景全文内联
