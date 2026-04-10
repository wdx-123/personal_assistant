# approved-ai-a2ui-hybrid-assistant

## 背景

本计划用于记录 AI 助手跨 `z_cur/UI` 与 `go/personal_assistant` 的已批准方案，避免后续开发再次回到“双段流 + 非 Checkpoint + 纯卡片协议”的旧结论。

## 已批准结论

1. V1 首版即采用 go-eino `Interrupt / Resume + Checkpoint`。
2. 顶层协议继续使用业务 SSE 事件流，不改成纯 A2UI 顶层协议。
3. A2UI 只进入消息内的声明式渲染块，不覆盖会话列表、主布局、权限状态机、SSE 协议和持久化。
4. 流交互固定为“单条 `stream` + `interrupt decision` 控制接口”。
5. `/tool-decisions/stream` 废弃。

## 本轮交付范围

1. 合并 AI 主文档与迁移说明。
2. 更新 `z_cur/UI` 的类型、store、mock、渲染与交互，支持 `ui_blocks`。
3. 更新 OpenAPI / Apifox，补齐 `ui_block`、A2UI 子集与 `interrupt_id`。

## 验收点

1. 前端可渲染 5 类 `ui_block`。
2. 等待确认时不能继续发送新问题。
3. decision 提交后不再开启第二条流。
4. 文档与 OpenAPI 中不再出现 `/tool-decisions/stream`。
